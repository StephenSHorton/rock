// Package tui is the Charm client. It renders the transcript and sends prompts
// to the harness. Session state stays in the store.
package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/harmonica"

	"github.com/StephenSHorton/rock/internal/harness"
	"github.com/StephenSHorton/rock/internal/jev"
	"github.com/StephenSHorton/rock/internal/perms"
	"github.com/StephenSHorton/rock/internal/provider"
	"github.com/StephenSHorton/rock/internal/session"
)

const contextLimit = 24000

// RunFunc executes one turn against the session the screen is showing.
type RunFunc func(ctx context.Context, sess *session.Session, prompt string, ask harness.AskFunc, sink func(harness.Event)) error

// Deps is everything the screen needs from the process.
type Deps struct {
	CWD           string
	Session       *session.Session
	Mode          perms.Mode
	ReviewOnly    bool
	JevMode       string
	Gates         jev.Gates
	Run           RunFunc
	LoadSession   func(id string) (*session.Session, error)
	ListSessions  func() []session.Meta
	Rules         []string
	Fork          func(prompt string) (string, error)
	SetMode       func(perms.Mode)
	InitialPrompt string
	Output        io.Writer
}

type line struct {
	kind string
	text string
}

type rowItem struct {
	title string
	desc  string
	id    string
}

func (r rowItem) FilterValue() string { return r.title + " " + r.desc }
func (r rowItem) Title() string       { return r.title }
func (r rowItem) Description() string { return r.desc }

type askMsg struct {
	tool   string
	detail string
	reply  chan perms.Decision
}

type eventMsg struct{ ev harness.Event }
type turnDone struct{ err error }
type tickMsg time.Time
type forkNote struct{ text string }

type keyMap struct {
	submit key.Binding
	quit   key.Binding
	help   key.Binding
	allow  key.Binding
	deny   key.Binding
	up     key.Binding
	down   key.Binding
}

func newKeys() keyMap {
	return keyMap{
		submit: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "send")),
		quit:   key.NewBinding(key.WithKeys("ctrl+c", "esc"), key.WithHelp("ctrl+c", "quit")),
		help:   key.NewBinding(key.WithKeys("ctrl+h"), key.WithHelp("ctrl+h", "help")),
		allow:  key.NewBinding(key.WithKeys("y", "a"), key.WithHelp("y", "allow")),
		deny:   key.NewBinding(key.WithKeys("n", "d"), key.WithHelp("n", "deny")),
		up:     key.NewBinding(key.WithKeys("pgup", "ctrl+u"), key.WithHelp("pgup", "scroll")),
		down:   key.NewBinding(key.WithKeys("pgdown", "ctrl+d"), key.WithHelp("pgdn", "scroll")),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.submit, k.help, k.up, k.quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.submit, k.quit, k.help},
		{k.allow, k.deny, k.up, k.down},
	}
}

// Model is the Bubble Tea program.
type Model struct {
	deps Deps
	keys keyMap

	width  int
	height int

	lines    []line
	vp       viewport.Model
	input    textarea.Model
	spin     spinner.Model
	help     help.Model
	sessions list.Model
	choices  list.Model
	perms    list.Model
	agents   table.Model
	meter    progress.Model

	showHelp     bool
	showSessions bool
	showPerms    bool
	showAgents   bool
	showPlan     bool

	pending *askMsg
	busy    bool
	status  string
	plan    string
	verdict string
	follow  bool

	spring harmonica.Spring
	meterP float64
	meterV float64
	target float64
	bytes  int

	send   func(tea.Msg)
	cancel context.CancelFunc
	glam   *glamour.TermRenderer

	header lipgloss.Style
	dim    lipgloss.Style
	user   lipgloss.Style
	panel  lipgloss.Style
	alert  lipgloss.Style
}

func New(deps Deps) *Model {
	ta := textarea.New()
	ta.Placeholder = "Ask Rock. Slash commands: /plan /yolo /sessions /permissions /agents /fork /ready /help"
	ta.ShowLineNumbers = false
	ta.CharLimit = 8000
	ta.SetHeight(3)
	ta.SetWidth(40)
	ta.Prompt = "› "

	spin := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := help.New()
	h.Styles = help.DefaultDarkStyles()

	vp := viewport.New(viewport.WithWidth(40), viewport.WithHeight(8))
	meter := progress.New(progress.WithWidth(18), progress.WithoutPercentage())
	agents := table.New(
		table.WithColumns([]table.Column{{Title: "Kind", Width: 12}, {Title: "Status", Width: 12}, {Title: "Detail", Width: 32}}),
		table.WithHeight(6),
	)
	sessions := list.New(nil, list.NewDefaultDelegate(), 40, 10)
	sessions.Title = "Sessions"
	sessions.SetShowHelp(false)
	sessions.SetFilteringEnabled(false)
	sessions.SetShowStatusBar(false)
	choices := list.New(nil, list.NewDefaultDelegate(), 40, 6)
	choices.Title = "Allow this call?"
	choices.SetShowHelp(false)
	choices.SetFilteringEnabled(false)
	choices.SetShowStatusBar(false)
	permsList := list.New(nil, list.NewDefaultDelegate(), 40, 10)
	permsList.Title = "Permissions"
	permsList.SetShowHelp(false)
	permsList.SetFilteringEnabled(false)
	permsList.SetShowStatusBar(false)

	m := &Model{
		deps:     deps,
		keys:     newKeys(),
		vp:       vp,
		input:    ta,
		spin:     spin,
		help:     h,
		sessions: sessions,
		choices:  choices,
		perms:    permsList,
		agents:   agents,
		meter:    meter,
		follow:   true,
		status:   "ready",
		spring:   harmonica.NewSpring(0.032, 7, 0.65),
		header:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("111")),
		dim:      lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		user:     lipgloss.NewStyle().Foreground(lipgloss.Color("150")),
		panel:    lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1),
		alert:    lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("203")).Padding(0, 1),
	}
	if deps.Output == nil {
		m.deps.Output = os.Stdout
	}
	m.seedTranscript()
	m.refreshPerms()
	m.refreshPlan()
	return m
}

// Send is set by Run before the program starts so the harness can post events.
func (m *Model) Send(fn func(tea.Msg)) { m.send = fn }

func Run(m *Model) error {
	p := tea.NewProgram(m)
	m.send = p.Send
	_, err := p.Run()
	return err
}

func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.input.Focus(), m.tick()}
	if strings.TrimSpace(m.deps.InitialPrompt) != "" {
		prompt := m.deps.InitialPrompt
		m.deps.InitialPrompt = ""
		cmds = append(cmds, m.start(prompt))
	}
	return tea.Batch(cmds...)
}

func (m *Model) tick() tea.Cmd {
	return tea.Tick(32*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		m.syncView()
		return m, nil
	case tickMsg:
		m.meterP, m.meterV = m.spring.Update(m.meterP, m.meterV, m.target)
		if m.busy {
			m.spin, _ = m.spin.Update(m.spin.Tick())
		}
		return m, m.tick()
	case progress.FrameMsg:
		var cmd tea.Cmd
		m.meter, cmd = m.meter.Update(msg)
		return m, cmd
	case eventMsg:
		cmd := m.apply(msg.ev)
		m.syncView()
		return m, cmd
	case turnDone:
		m.busy = false
		if msg.err != nil {
			m.status = msg.err.Error()
		}
		m.syncView()
		return m, nil
	case askMsg:
		m.pending = &msg
		m.choices.Title = msg.tool
		items := []list.Item{
			rowItem{title: "Allow", desc: clip(msg.detail, 80), id: "allow"},
			rowItem{title: "Deny", desc: "block this call", id: "deny"},
		}
		cmd := m.choices.SetItems(items)
		m.input.Blur()
		return m, cmd
	case forkNote:
		m.status = msg.text
		return m, nil
	case tea.KeyPressMsg:
		return m.onKey(msg)
	}
	if m.pending != nil {
		var cmd tea.Cmd
		m.choices, cmd = m.choices.Update(msg)
		return m, cmd
	}
	if m.showSessions {
		var cmd tea.Cmd
		m.sessions, cmd = m.sessions.Update(msg)
		return m, cmd
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *Model) onKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.quit) && (msg.String() == "ctrl+c" || m.pending != nil || m.showSessions || m.showPerms || m.showAgents || m.showHelp) {
		if msg.String() == "ctrl+c" && m.pending == nil && !m.showSessions && !m.showPerms && !m.showAgents && !m.showHelp {
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
		if m.pending != nil && msg.String() == "esc" {
			m.answer(perms.Deny)
			return m, nil
		}
		if msg.String() == "esc" {
			m.showSessions, m.showPerms, m.showAgents, m.showHelp = false, false, false, false
			return m, m.input.Focus()
		}
		if msg.String() == "ctrl+c" {
			if m.cancel != nil {
				m.cancel()
			}
			if m.pending != nil {
				m.answer(perms.Deny)
			}
			return m, tea.Quit
		}
	}
	if m.pending != nil {
		if key.Matches(msg, m.keys.allow) {
			m.answer(perms.Allow)
			return m, nil
		}
		if key.Matches(msg, m.keys.deny) {
			m.answer(perms.Deny)
			return m, nil
		}
		if msg.String() == "enter" {
			if item, ok := m.choices.SelectedItem().(rowItem); ok && item.id == "allow" {
				m.answer(perms.Allow)
			} else {
				m.answer(perms.Deny)
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.choices, cmd = m.choices.Update(msg)
		return m, cmd
	}
	if m.showSessions {
		if msg.String() == "enter" {
			if item, ok := m.sessions.SelectedItem().(rowItem); ok && m.deps.LoadSession != nil {
				sess, err := m.deps.LoadSession(item.id)
				if err != nil {
					m.status = err.Error()
				} else {
					m.deps.Session = sess
					m.seedTranscript()
					m.refreshPlan()
					m.status = "resumed " + item.id
				}
			}
			m.showSessions = false
			m.syncView()
			return m, m.input.Focus()
		}
		var cmd tea.Cmd
		m.sessions, cmd = m.sessions.Update(msg)
		return m, cmd
	}
	if m.showPerms || m.showAgents {
		if msg.String() == "enter" || msg.String() == "esc" {
			m.showPerms, m.showAgents = false, false
			return m, m.input.Focus()
		}
		if m.showPerms {
			var cmd tea.Cmd
			m.perms, cmd = m.perms.Update(msg)
			return m, cmd
		}
		var cmd tea.Cmd
		m.agents, cmd = m.agents.Update(msg)
		return m, cmd
	}
	if key.Matches(msg, m.keys.up) || key.Matches(msg, m.keys.down) {
		m.follow = false
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}
	if msg.String() == "ctrl+h" {
		m.showHelp = !m.showHelp
		return m, nil
	}
	if msg.String() == "enter" && !m.busy {
		text := strings.TrimSpace(m.input.Value())
		if text == "" {
			return m, nil
		}
		m.input.Reset()
		if strings.HasPrefix(text, "/") {
			return m, m.slash(text)
		}
		m.lines = append(m.lines, line{kind: "user", text: text})
		m.follow = true
		m.syncView()
		return m, m.start(text)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *Model) slash(text string) tea.Cmd {
	fields := strings.Fields(text)
	cmd := fields[0]
	rest := strings.TrimSpace(strings.TrimPrefix(text, cmd))
	switch cmd {
	case "/help":
		m.showHelp = !m.showHelp
	case "/plan":
		m.setMode(perms.ModePlan)
		m.showPlan = true
		m.refreshPlan()
		m.status = "plan mode. Shell is blocked. Write the plan with update_plan."
	case "/yolo":
		m.reportReady()
		m.setMode(perms.ModeYolo)
		m.status = "yolo. Asks are skipped. The destructive gate still blocks."
	case "/default":
		m.reportReady()
		m.setMode(perms.ModeDefault)
		m.status = "default mode"
	case "/sessions":
		m.refreshSessions()
		m.showSessions = true
		m.input.Blur()
	case "/permissions":
		m.refreshPerms()
		m.showPerms = true
		m.input.Blur()
	case "/agents":
		m.showAgents = true
		m.input.Blur()
	case "/ready":
		m.refreshPlan()
		m.reportReady()
		m.status = m.verdict
	case "/fork":
		return m.emitFork(rest)
	case "/quit":
		if m.cancel != nil {
			m.cancel()
		}
		return tea.Quit
	default:
		m.status = "unknown command " + cmd
	}
	return nil
}

func (m *Model) setMode(mode perms.Mode) {
	m.deps.Mode = mode
	if m.deps.SetMode != nil {
		m.deps.SetMode(mode)
	}
	if m.deps.Session != nil {
		m.deps.Session.Meta.Mode = string(mode)
	}
}

func (m *Model) emitFork(prompt string) tea.Cmd {
	return func() tea.Msg {
		if m.deps.Fork == nil {
			return forkNote{text: "fork is not wired"}
		}
		seq, err := m.deps.Fork(prompt)
		if err != nil {
			return forkNote{text: err.Error()}
		}
		_, _ = io.WriteString(m.deps.Output, seq)
		return forkNote{text: "sent OSC 7880 to the host"}
	}
}

func (m *Model) start(prompt string) tea.Cmd {
	if m.deps.Run == nil || m.busy {
		return nil
	}
	m.busy = true
	m.status = "working"
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	return func() tea.Msg {
		err := m.deps.Run(ctx, m.deps.Session, prompt, m.ask, func(ev harness.Event) {
			if m.send != nil {
				m.send(eventMsg{ev})
			}
		})
		if m.send != nil {
			m.send(turnDone{err})
		}
		return nil
	}
}

func (m *Model) ask(ctx context.Context, tool, detail string) perms.Decision {
	if m.deps.Mode == perms.ModeYolo {
		return perms.Allow
	}
	if m.send == nil {
		return perms.Deny
	}
	reply := make(chan perms.Decision, 1)
	m.send(askMsg{tool: tool, detail: detail, reply: reply})
	select {
	case d := <-reply:
		return d
	case <-ctx.Done():
		return perms.Deny
	}
}

func (m *Model) answer(d perms.Decision) {
	if m.pending == nil {
		return
	}
	m.pending.reply <- d
	m.pending = nil
	m.status = "you answered " + string(d)
	_ = m.input.Focus()
}

func (m *Model) apply(ev harness.Event) tea.Cmd {
	switch ev.Kind {
	case harness.EvAssistant:
		m.lines = append(m.lines, line{kind: "assistant", text: ev.Text})
		m.bytes += len(ev.Text)
	case harness.EvToolCall:
		m.lines = append(m.lines, line{kind: "tool", text: ev.Name + " " + clip(ev.Text, 200)})
		if ev.Name == "spawn_subagent" {
			m.addAgent(ev.Text, "running", "")
		}
	case harness.EvToolResult:
		m.lines = append(m.lines, line{kind: "result", text: ev.Name + " " + clip(ev.Text, 240)})
		m.bytes += len(ev.Text)
		if ev.Name == "spawn_subagent" {
			m.addAgent("", "done", clip(ev.Text, 40))
		}
		if ev.Name == "update_plan" {
			m.refreshPlan()
		}
	case harness.EvJev:
		m.lines = append(m.lines, line{kind: "jev", text: ev.Name + " " + ev.Text})
	case harness.EvPermission:
		m.lines = append(m.lines, line{kind: "permission", text: ev.Name + " " + ev.Text})
	case harness.EvStatus:
		m.status = ev.Text
		if strings.Contains(ev.Text, "compact") {
			m.bytes = m.bytes / 4
		}
	case harness.EvDone:
		m.status = ev.Text
		m.busy = false
		m.refreshPlan()
	}
	if m.bytes < 0 {
		m.bytes = 0
	}
	m.target = float64(m.bytes) / contextLimit
	if m.target > 1 {
		m.target = 1
	}
	return m.meter.SetPercent(m.target)
}

func (m *Model) addAgent(args, status, detail string) {
	kind := "general"
	if strings.Contains(args, "explore") {
		kind = "explore"
	} else if strings.Contains(args, "plan") {
		kind = "plan"
	}
	rows := []table.Row{{kind, status, detail}}
	cur := m.agents.Rows()
	cur = append(cur, rows...)
	if len(cur) > 8 {
		cur = cur[len(cur)-8:]
	}
	m.agents.SetRows(cur)
}

func (m *Model) seedTranscript() {
	m.lines = nil
	m.bytes = 0
	if m.deps.Session == nil {
		return
	}
	for _, msg := range m.deps.Session.Messages {
		switch msg.Role {
		case provider.RoleUser:
			m.lines = append(m.lines, line{kind: "user", text: msg.Content})
			m.bytes += len(msg.Content)
		case provider.RoleAssistant:
			if msg.Content != "" {
				m.lines = append(m.lines, line{kind: "assistant", text: msg.Content})
				m.bytes += len(msg.Content)
			}
		case provider.RoleTool:
			m.lines = append(m.lines, line{kind: "result", text: msg.Name + " " + clip(msg.Content, 240)})
			m.bytes += len(msg.Content)
		}
	}
	m.target = float64(m.bytes) / contextLimit
}

func (m *Model) refreshSessions() {
	if m.deps.ListSessions == nil {
		return
	}
	var items []list.Item
	for _, meta := range m.deps.ListSessions() {
		title := meta.Title
		if title == "" {
			title = meta.ID
		}
		items = append(items, rowItem{title: title, desc: meta.ID + "  " + meta.UpdatedAt.Format(time.RFC3339), id: meta.ID})
	}
	if len(items) == 0 {
		items = append(items, rowItem{title: "No sessions yet", desc: "this folder has an empty store", id: ""})
	}
	_ = m.sessions.SetItems(items)
}

func (m *Model) refreshPerms() {
	var items []list.Item
	if len(m.deps.Rules) == 0 {
		items = append(items, rowItem{title: "defaults", desc: "reads allow, edits ask", id: ""})
	}
	for _, r := range m.deps.Rules {
		items = append(items, rowItem{title: r, desc: "rule", id: r})
	}
	items = append(items, rowItem{title: "not a sandbox", desc: "plan mode blocks shell because redirections are not inspected", id: ""})
	_ = m.perms.SetItems(items)
}

func (m *Model) refreshPlan() {
	if m.deps.Session == nil {
		return
	}
	raw, err := os.ReadFile(m.deps.Session.PlanPath())
	if err != nil {
		m.plan = "No plan yet."
		m.verdict = "plan: empty"
		return
	}
	m.plan = string(raw)
	m.reportReady()
}

func (m *Model) reportReady() {
	ready, p := m.deps.Gates.PlanReady(context.Background(), m.plan, "")
	word := "not ready"
	if ready {
		word = "ready"
	}
	m.verdict = fmt.Sprintf("plan %s (%.2f, %s). This does not approve the plan.", word, p, m.deps.Gates.Mode())
}

func (m *Model) layout() {
	w, h := m.width, m.height
	if w < 20 {
		w = 20
	}
	if h < 8 {
		h = 8
	}
	planW := 0
	if m.showPlan || w >= 100 {
		planW = 36
		if planW > w/3 {
			planW = w / 3
		}
	}
	bodyH := h - 8
	if bodyH < 3 {
		bodyH = 3
	}
	m.vp.SetWidth(w - planW - 2)
	m.vp.SetHeight(bodyH)
	m.input.SetWidth(w - 2)
	m.help.SetWidth(w)
	m.sessions.SetSize(w-4, bodyH)
	m.choices.SetSize(w-8, 6)
	m.perms.SetSize(w-4, bodyH)
	m.agents.SetWidth(w - 4)
	m.agents.SetHeight(bodyH)
	if m.glam == nil || m.vp.Width() > 0 {
		wrap := m.vp.Width()
		if wrap < 20 {
			wrap = 20
		}
		r, err := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(wrap))
		if err == nil {
			m.glam = r
		}
	}
}

func (m *Model) syncView() {
	wasBottom := m.vp.AtBottom() || m.follow
	m.vp.SetContent(m.renderTranscript())
	if wasBottom {
		m.vp.GotoBottom()
		m.follow = true
	}
}

func (m *Model) renderTranscript() string {
	if len(m.lines) == 0 {
		return m.dim.Render("Rock is ready. The transcript lives in the session store, not in this screen.")
	}
	var b strings.Builder
	for _, ln := range m.lines {
		switch ln.kind {
		case "user":
			b.WriteString(m.user.Render("you  " + ln.text))
		case "assistant":
			text := ln.text
			if m.glam != nil {
				if out, err := m.glam.Render(ln.text); err == nil {
					text = strings.TrimRight(out, "\n")
				}
			}
			b.WriteString(text)
		default:
			b.WriteString(m.dim.Render(ln.kind + "  " + ln.text))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func (m *Model) View() tea.View {
	if m.width == 0 {
		v := tea.NewView("rock")
		v.AltScreen = true
		v.MouseMode = tea.MouseModeCellMotion
		return v
	}
	id := ""
	if m.deps.Session != nil {
		id = m.deps.Session.Meta.ID
	}
	title := fmt.Sprintf("rock  %s  %s  jev:%s  %s", id, m.deps.Mode, m.deps.JevMode, m.deps.CWD)
	if m.deps.ReviewOnly {
		title += "  review"
	}
	header := m.header.Render(clip(title, m.width))
	body := m.vp.View()
	if m.showSessions {
		body = m.panel.Width(m.width - 2).Render(m.sessions.View())
	} else if m.showPerms {
		body = m.panel.Width(m.width - 2).Render(m.perms.View())
	} else if m.showAgents {
		body = m.panel.Width(m.width - 2).Render(m.agents.View())
	} else if m.showPlan || m.width >= 100 {
		planBody := m.verdict + "\n\n" + m.plan
		plan := m.panel.Width(36).Render(clip(planBody, 1200))
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.vp.View(), plan)
	}
	spin := " "
	if m.busy {
		spin = m.spin.View()
	}
	meter := fmt.Sprintf("context %3.0f%%  %s  %s", m.meterP*100, m.meter.ViewAs(m.meterP), m.status)
	status := m.dim.Render(spin + " " + meter)
	var overlay string
	if m.pending != nil {
		overlay = "\n" + m.alert.Render(m.choices.View())
	}
	helpView := m.help.View(m.keys)
	if m.showHelp {
		m.help.ShowAll = true
		helpView = m.help.View(m.keys) + "\n" + m.dim.Render("/plan  /yolo  /default  /sessions  /permissions  /agents  /fork  /ready  /quit")
	} else {
		m.help.ShowAll = false
	}
	content := lipgloss.JoinVertical(lipgloss.Left, header, body, status, m.input.View(), helpView)
	if overlay != "" {
		content += overlay
	}
	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "rock"
	return v
}

func clip(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
