// Package tui is the Charm client. It renders the transcript and sends prompts
// to the harness. Session state stays in the store.
package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"io"
	"math"
	"os"
	"sort"
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
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/harmonica"

	"github.com/StephenSHorton/rock/internal/harness"
	"github.com/StephenSHorton/rock/internal/jev"
	"github.com/StephenSHorton/rock/internal/perms"
	"github.com/StephenSHorton/rock/internal/provider"
	"github.com/StephenSHorton/rock/internal/session"
)

const (
	// contextLimit is the session size at which the offline Jev policy
	// compacts, so a full meter means compaction is next.
	contextLimit = 24000
	// wideAt is the width from which the plan column is always shown.
	wideAt = 100
	// splitMin is the narrowest width that still puts the plan beside the
	// transcript. Narrower screens stack the plan above it in plan mode.
	splitMin = 64
	// gutter is the speaker column of the transcript.
	gutter = 6
	fps    = 30

	readyText   = "Rock is ready. The transcript lives in the session store, not in this screen."
	placeholder = "Ask Rock. /help /plan /yolo /default /sessions /permissions /agents /ready /fork /quit"
)

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
	Provider      string
	FastModel     string
	StrongModel   string
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
	name string
	text string

	out     string
	outW    int
	outDark bool
}

type rowItem struct {
	title string
	desc  string
	id    string
	tag   string
}

func (r rowItem) FilterValue() string { return r.title + " " + r.desc }
func (r rowItem) Title() string       { return r.title }
func (r rowItem) Description() string { return r.desc }

type overlay int

const (
	noOverlay overlay = iota
	helpOverlay
	sessionsOverlay
	permsOverlay
	agentsOverlay
)

type askMsg struct {
	tool   string
	detail string
	reply  chan perms.Decision
}

type eventMsg struct{ ev harness.Event }

// turnEvent is an event from a running turn plus the session size the
// harness had reached, read on the harness goroutine.
type turnEvent struct {
	ev  harness.Event
	ctx int
}

type turnDone struct {
	err error
	ctx int
}

type tickMsg time.Time
type forkNote struct{ text string }

type keyMap struct {
	submit  key.Binding
	newline key.Binding
	help    key.Binding
	quit    key.Binding
	scroll  key.Binding
	up      key.Binding
	down    key.Binding
	allow   key.Binding
	deny    key.Binding
	close   key.Binding
	move    key.Binding
	pick    key.Binding
}

func newKeys() keyMap {
	return keyMap{
		submit:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "send")),
		newline: key.NewBinding(key.WithKeys("shift+enter", "ctrl+j", "alt+enter"), key.WithHelp("ctrl+j", "new line")),
		help:    key.NewBinding(key.WithKeys("ctrl+h"), key.WithHelp("ctrl+h", "help")),
		quit:    key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
		scroll:  key.NewBinding(key.WithKeys("pgup", "pgdown"), key.WithHelp("pgup/pgdn", "scroll")),
		up:      key.NewBinding(key.WithKeys("pgup", "ctrl+u"), key.WithHelp("pgup", "scroll up")),
		down:    key.NewBinding(key.WithKeys("pgdown", "ctrl+d"), key.WithHelp("pgdn", "scroll down")),
		allow:   key.NewBinding(key.WithKeys("y", "a"), key.WithHelp("y", "allow")),
		deny:    key.NewBinding(key.WithKeys("n", "d"), key.WithHelp("n", "deny")),
		close:   key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc", "close")),
		move:    key.NewBinding(key.WithKeys("up", "down", "k", "j"), key.WithHelp("↑/↓", "move")),
		pick:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "resume")),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.submit, k.newline, k.scroll, k.help, k.quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.submit, k.newline, k.up, k.down},
		{k.help, k.close, k.quit},
		{k.allow, k.deny},
	}
}

// geometry is where each part of the screen sits for the current size and
// state. layout computes it; View only reads it.
type geometry struct {
	w, h         int
	bodyY, bodyH int
	composerRows int
	helpRows     int
	transcriptW  int
	planW        int
	planH        int
	askH         int
	vpY, vpH     int
}

// Model is the Bubble Tea program.
type Model struct {
	deps Deps
	keys keyMap
	th   theme

	width  int
	height int
	geo    geometry

	lines    []line
	vp       viewport.Model
	planVP   viewport.Model
	sheet    viewport.Model
	input    textarea.Model
	spin     spinner.Model
	help     help.Model
	sessions list.Model
	choices  list.Model
	perms    list.Model
	agents   table.Model
	meter    progress.Model

	overlay  overlay
	showPlan bool

	pending   *askMsg
	lastCall  harness.Event
	busy      bool
	status    string
	alert     bool
	plan      string
	verdict   string
	follow    bool
	dragging  bool
	turnModel string

	spring   harmonica.Spring
	meterP   float64
	meterV   float64
	target   float64
	ctxBytes int
	ticking  bool

	send     func(tea.Msg)
	cancel   context.CancelFunc
	md       map[int]*glamour.TermRenderer
	contentW int
	planW    int
}

func New(deps Deps) *Model {
	if deps.Output == nil {
		deps.Output = os.Stdout
	}
	m := &Model{
		deps:   deps,
		keys:   newKeys(),
		follow: true,
		status: "ready",
		spring: harmonica.NewSpring(harmonica.FPS(fps), 7, 1),
		md:     map[int]*glamour.TermRenderer{},
	}

	ta := textarea.New()
	ta.Placeholder = placeholder
	ta.ShowLineNumbers = false
	ta.CharLimit = 8000
	ta.SetPromptFunc(2, func(info textarea.PromptInfo) string {
		if info.LineNumber == 0 {
			return "› "
		}
		return "  "
	})
	ta.KeyMap.InsertNewline = m.keys.newline
	ta.SetHeight(3)
	ta.SetWidth(40)
	m.input = ta

	m.vp = viewport.New(viewport.WithWidth(40), viewport.WithHeight(8))
	m.planVP = viewport.New(viewport.WithWidth(30), viewport.WithHeight(8))
	m.sheet = viewport.New(viewport.WithWidth(40), viewport.WithHeight(8))
	m.spin = spinner.New(spinner.WithSpinner(spinner.MiniDot))
	m.help = help.New()
	m.help.ShortSeparator = " · "

	m.sessions = newList(rowDelegate{th: &m.th, rows: 2})
	m.choices = newList(rowDelegate{th: &m.th, rows: 1})
	m.choices.SetShowPagination(false)
	m.perms = newList(rowDelegate{th: &m.th, rows: 1})
	m.agents = table.New(
		table.WithColumns(agentColumns(60)),
		table.WithHeight(6),
		table.WithFocused(true),
	)
	m.meter = progress.New(progress.WithWidth(16))

	m.applyTheme(true)
	m.seedTranscript()
	m.refreshPerms()
	m.refreshPlan()
	return m
}

func newList(d list.ItemDelegate) list.Model {
	l := list.New(nil, d, 40, 10)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()
	return l
}

func agentColumns(width int) []table.Column {
	detail := max(12, width-10-10-6)
	return []table.Column{{Title: "Kind", Width: 10}, {Title: "Status", Width: 10}, {Title: "Detail", Width: detail}}
}

// applyTheme rebuilds every style for a dark or light background.
func (m *Model) applyTheme(dark bool) {
	m.th = newTheme(dark)
	m.input.SetStyles(m.th.composer())
	m.help.Styles = m.th.keyHelp()
	m.spin.Style = m.th.accent
	m.agents.SetStyles(m.th.table())
	for _, l := range []*list.Model{&m.sessions, &m.choices, &m.perms} {
		l.Styles.ActivePaginationDot = m.th.accent.SetString("•")
		l.Styles.InactivePaginationDot = m.th.faint.SetString("·")
		l.Styles.NoItems = m.th.faint
	}
	warn, trim := m.th.attention, m.th.trim
	m.meter = progress.New(
		progress.WithWidth(m.meter.Width()),
		progress.WithoutPercentage(),
		progress.WithFillCharacters('━', '─'),
		progress.WithColorFunc(func(total, _ float64) color.Color {
			if total >= 0.8 {
				return warn
			}
			return trim
		}),
	)
	m.meter.EmptyColor = m.th.muted
	m.md = map[int]*glamour.TermRenderer{}
	m.contentW, m.planW = 0, 0
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
	cmds := []tea.Cmd{m.input.Focus(), tea.RequestBackgroundColor, m.retarget()}
	if prompt := strings.TrimSpace(m.deps.InitialPrompt); prompt != "" {
		m.deps.InitialPrompt = ""
		m.lines = append(m.lines, line{kind: "user", text: prompt})
		cmds = append(cmds, m.start(prompt))
	}
	return tea.Batch(cmds...)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.update(msg)
	switch msg.(type) {
	case tickMsg, spinner.TickMsg:
	default:
		m.layout()
	}
	return m, cmd
}

func (m *Model) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return nil
	case tea.BackgroundColorMsg:
		if msg.IsDark() != m.th.dark {
			m.applyTheme(msg.IsDark())
		}
		return nil
	case tickMsg:
		return m.stepMeter()
	case spinner.TickMsg:
		if !m.busy {
			return nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return cmd
	case turnEvent:
		m.ctxBytes = msg.ctx
		m.apply(msg.ev)
		return m.retarget()
	case eventMsg:
		m.ctxBytes += eventBytes(msg.ev)
		m.apply(msg.ev)
		return m.retarget()
	case turnDone:
		m.busy = false
		m.turnModel = ""
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
			m.setAlert(msg.err.Error())
			m.lines = append(m.lines, line{kind: "error", text: msg.err.Error()})
			m.syncView()
		}
		if msg.ctx > 0 {
			m.ctxBytes = msg.ctx
		}
		return m.retarget()
	case askMsg:
		m.openAsk(msg)
		return nil
	case forkNote:
		m.status, m.alert = msg.text, false
		return nil
	case tea.MouseMsg:
		m.onMouse(msg)
		return nil
	case tea.KeyPressMsg:
		return m.onKey(msg)
	}
	if m.pending != nil || m.overlay != noOverlay {
		return nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

func (m *Model) onKey(msg tea.KeyPressMsg) tea.Cmd {
	if key.Matches(msg, m.keys.quit) {
		return m.quit()
	}
	if m.pending != nil {
		return m.askKey(msg)
	}
	if m.overlay != noOverlay {
		return m.overlayKey(msg)
	}
	switch {
	case key.Matches(msg, m.keys.help):
		m.openOverlay(helpOverlay)
		return nil
	case key.Matches(msg, m.keys.up), key.Matches(msg, m.keys.down):
		m.scrollKey(msg)
		return nil
	case key.Matches(msg, m.keys.submit):
		return m.submit()
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

func (m *Model) scrollKey(msg tea.KeyPressMsg) {
	switch msg.String() {
	case "pgup":
		m.vp.PageUp()
	case "ctrl+u":
		m.vp.HalfPageUp()
	case "pgdown":
		m.vp.PageDown()
	case "ctrl+d":
		m.vp.HalfPageDown()
	}
	m.follow = m.vp.AtBottom()
}

func (m *Model) askKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, m.keys.allow):
		return m.answer(perms.Allow)
	case key.Matches(msg, m.keys.deny), msg.String() == "esc":
		return m.answer(perms.Deny)
	case msg.String() == "enter":
		if item, ok := m.choices.SelectedItem().(rowItem); ok && item.id == "allow" {
			return m.answer(perms.Allow)
		}
		return m.answer(perms.Deny)
	}
	var cmd tea.Cmd
	m.choices, cmd = m.choices.Update(msg)
	return cmd
}

func (m *Model) overlayKey(msg tea.KeyPressMsg) tea.Cmd {
	if key.Matches(msg, m.keys.close) || (m.overlay == helpOverlay && key.Matches(msg, m.keys.help)) {
		return m.closeOverlay()
	}
	enter := msg.String() == "enter"
	var cmd tea.Cmd
	switch m.overlay {
	case sessionsOverlay:
		if enter {
			m.resumeSelected()
			return m.closeOverlay()
		}
		m.sessions, cmd = m.sessions.Update(msg)
	case permsOverlay:
		if enter {
			return m.closeOverlay()
		}
		m.perms, cmd = m.perms.Update(msg)
	case agentsOverlay:
		if enter {
			return m.closeOverlay()
		}
		m.agents, cmd = m.agents.Update(msg)
	case helpOverlay:
		if enter {
			return m.closeOverlay()
		}
		switch msg.String() {
		case "up", "k":
			m.sheet.ScrollUp(1)
		case "down", "j":
			m.sheet.ScrollDown(1)
		case "pgup", "ctrl+u":
			m.sheet.PageUp()
		case "pgdown", "ctrl+d", "space":
			m.sheet.PageDown()
		}
	}
	return cmd
}

func (m *Model) openOverlay(o overlay) {
	m.overlay = o
	m.input.Blur()
	if o == helpOverlay {
		m.sheet.GotoTop()
	}
}

func (m *Model) closeOverlay() tea.Cmd {
	m.overlay = noOverlay
	return m.input.Focus()
}

func (m *Model) quit() tea.Cmd {
	if m.pending != nil {
		m.answer(perms.Deny)
	}
	if m.cancel != nil {
		m.cancel()
	}
	return tea.Quit
}

func (m *Model) submit() tea.Cmd {
	text := strings.TrimSpace(m.input.Value())
	if text == "" {
		return nil
	}
	if m.busy {
		m.status, m.alert = "a turn is running; the prompt stays in the composer", false
		return nil
	}
	m.input.Reset()
	if strings.HasPrefix(text, "/") {
		return m.slash(text)
	}
	m.lines = append(m.lines, line{kind: "user", text: text})
	m.follow = true
	m.syncView()
	return m.start(text)
}

func (m *Model) slash(text string) tea.Cmd {
	fields := strings.Fields(text)
	cmd := fields[0]
	rest := strings.TrimSpace(strings.TrimPrefix(text, cmd))
	m.alert = false
	switch cmd {
	case "/help":
		m.openOverlay(helpOverlay)
	case "/plan":
		m.setMode(perms.ModePlan)
		m.showPlan = true
		m.refreshPlan()
		m.status = "plan mode. Shell is blocked. Write the plan with update_plan."
	case "/yolo":
		m.reportReady()
		m.setMode(perms.ModeYolo)
		m.showPlan = false
		m.status = "yolo. Asks are skipped. The destructive gate still blocks."
	case "/default":
		m.reportReady()
		m.setMode(perms.ModeDefault)
		m.showPlan = false
		m.status = "default mode"
	case "/sessions":
		m.refreshSessions()
		m.openOverlay(sessionsOverlay)
	case "/permissions":
		m.refreshPerms()
		m.openOverlay(permsOverlay)
	case "/agents":
		m.openOverlay(agentsOverlay)
	case "/ready":
		m.refreshPlan()
		m.reportReady()
		m.status = m.verdict
	case "/fork":
		return m.emitFork(rest)
	case "/quit":
		return m.quit()
	default:
		m.setAlert("unknown command " + cmd + ". /help lists them.")
	}
	return nil
}

func (m *Model) setAlert(text string) {
	m.status, m.alert = text, true
}

func (m *Model) setMode(mode perms.Mode) {
	m.deps.Mode = mode
	if m.deps.SetMode != nil {
		m.deps.SetMode(mode)
	}
	if m.deps.Session != nil {
		m.deps.Session.Meta.Mode = string(mode)
	}
	m.refreshPerms()
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
	m.status, m.alert = "working", false
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	sess := m.deps.Session
	run := func() tea.Msg {
		err := m.deps.Run(ctx, sess, prompt, m.ask, func(ev harness.Event) {
			if m.send != nil {
				m.send(turnEvent{ev: ev, ctx: sessionBytes(sess)})
			}
		})
		if m.send != nil {
			m.send(turnDone{err: err, ctx: sessionBytes(sess)})
		}
		return nil
	}
	return tea.Batch(run, m.spin.Tick)
}

func sessionBytes(s *session.Session) int {
	if s == nil {
		return 0
	}
	return s.Bytes()
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

func (m *Model) openAsk(msg askMsg) {
	m.pending = &msg
	m.overlay = noOverlay
	_ = m.choices.SetItems([]list.Item{
		rowItem{title: "Allow", desc: "run this call once", id: "allow", tag: "choice"},
		rowItem{title: "Deny", desc: "block it; the model is told it was denied", id: "deny", tag: "choice"},
	})
	m.choices.Select(0)
	m.input.Blur()
	m.follow = true
	m.vp.GotoBottom()
}

func (m *Model) answer(d perms.Decision) tea.Cmd {
	if m.pending == nil {
		return nil
	}
	m.pending.reply <- d
	m.pending = nil
	m.status, m.alert = "you answered "+string(d), false
	return m.input.Focus()
}

func (m *Model) apply(ev harness.Event) {
	switch ev.Kind {
	case harness.EvAssistant:
		m.lines = append(m.lines, line{kind: "assistant", text: ev.Text})
	case harness.EvToolCall:
		m.lines = append(m.lines, line{kind: "tool", name: ev.Name, text: ev.Text})
		m.lastCall = ev
		if ev.Name == "spawn_subagent" {
			m.agentStarted(ev.Text)
		}
	case harness.EvToolResult:
		m.lines = append(m.lines, line{kind: "result", name: ev.Name, text: ev.Text})
		if ev.Name == "spawn_subagent" {
			m.agentFinished(ev.Text)
		}
		if ev.Name == "update_plan" {
			m.refreshPlan()
		}
	case harness.EvJev:
		m.lines = append(m.lines, line{kind: "jev", name: ev.Name, text: ev.Text})
		if ev.Name == "turn" {
			m.turnModel = ""
			if strings.Contains(" "+ev.Text+" ", " model=strong ") && m.deps.StrongModel != "" {
				m.turnModel = m.deps.StrongModel
			}
		}
	case harness.EvPermission:
		m.lines = append(m.lines, line{kind: "permission", name: ev.Name, text: ev.Text})
	case harness.EvStatus:
		m.status, m.alert = ev.Text, false
	case harness.EvDone:
		m.status, m.alert = doneText(ev.Text), ev.Text != "end_turn"
		m.refreshPlan()
	}
	m.syncView()
}

func doneText(reason string) string {
	switch reason {
	case "end_turn", "":
		return "ready"
	case "max_steps":
		return "stopped at the step limit"
	case "stuck":
		return "stopped: the last tool calls repeated"
	default:
		return reason
	}
}

func eventBytes(ev harness.Event) int {
	switch ev.Kind {
	case harness.EvAssistant, harness.EvToolCall, harness.EvToolResult:
		return len(ev.Text) + len(ev.Name)
	}
	return 0
}

// retarget points the context meter at the current session size and starts
// the spring if it has somewhere to go.
func (m *Model) retarget() tea.Cmd {
	m.target = math.Min(1, math.Max(0, float64(m.ctxBytes)/contextLimit))
	if m.ticking || m.settled() {
		return nil
	}
	m.ticking = true
	return m.tick()
}

func (m *Model) settled() bool {
	return math.Abs(m.meterP-m.target) < 0.0005 && math.Abs(m.meterV) < 0.005
}

func (m *Model) tick() tea.Cmd {
	return tea.Tick(time.Second/fps, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// stepMeter advances the Harmonica spring one frame. The loop stops when the
// meter settles so an idle screen does not redraw thirty times a second.
func (m *Model) stepMeter() tea.Cmd {
	m.meterP, m.meterV = m.spring.Update(m.meterP, m.meterV, m.target)
	if m.settled() {
		m.meterP, m.meterV = m.target, 0
		m.ticking = false
		return nil
	}
	return m.tick()
}

func (m *Model) agentStarted(args string) {
	var a struct {
		Kind   string `json:"kind"`
		Prompt string `json:"prompt"`
	}
	_ = json.Unmarshal([]byte(args), &a)
	kind := strings.ToLower(strings.TrimSpace(a.Kind))
	switch {
	case kind == "explore" || kind == "plan" || kind == "general":
	case strings.Contains(args, "explore"):
		kind = "explore"
	case strings.Contains(args, "plan"):
		kind = "plan"
	default:
		kind = "general"
	}
	m.setAgents(append(m.agents.Rows(), table.Row{kind, "running", clip(a.Prompt, 200)}))
}

func (m *Model) agentFinished(result string) {
	status := "done"
	if strings.HasPrefix(result, "denied") {
		status = "denied"
	}
	rows := m.agents.Rows()
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i][1] == "running" {
			rows[i] = table.Row{rows[i][0], status, clip(result, 200)}
			m.setAgents(rows)
			return
		}
	}
	m.setAgents(append(rows, table.Row{"general", status, clip(result, 200)}))
}

func (m *Model) setAgents(rows []table.Row) {
	if len(rows) > 8 {
		rows = rows[len(rows)-8:]
	}
	m.agents.SetRows(rows)
	m.agents.GotoBottom()
}

func (m *Model) seedTranscript() {
	m.lines = nil
	m.ctxBytes = 0
	if m.deps.Session != nil {
		for _, msg := range m.deps.Session.Messages {
			switch msg.Role {
			case provider.RoleUser:
				m.lines = append(m.lines, line{kind: "user", text: msg.Content})
			case provider.RoleAssistant:
				if msg.Content != "" {
					m.lines = append(m.lines, line{kind: "assistant", text: msg.Content})
				}
				for _, c := range msg.ToolCalls {
					m.lines = append(m.lines, line{kind: "tool", name: c.Name, text: c.Arguments})
				}
			case provider.RoleTool:
				m.lines = append(m.lines, line{kind: "result", name: msg.Name, text: msg.Content})
			}
		}
		m.ctxBytes = m.deps.Session.Bytes()
	}
	m.target = math.Min(1, float64(m.ctxBytes)/contextLimit)
	m.follow = true
	m.syncView()
}

func (m *Model) resumeSelected() {
	item, ok := m.sessions.SelectedItem().(rowItem)
	if !ok || item.id == "" || m.deps.LoadSession == nil {
		return
	}
	sess, err := m.deps.LoadSession(item.id)
	if err != nil {
		m.setAlert(err.Error())
		return
	}
	m.deps.Session = sess
	m.seedTranscript()
	m.refreshPlan()
	m.status, m.alert = "resumed "+item.id, false
}

func (m *Model) refreshSessions() {
	if m.deps.ListSessions == nil {
		return
	}
	metas := m.deps.ListSessions()
	sort.SliceStable(metas, func(i, j int) bool { return metas[i].UpdatedAt.After(metas[j].UpdatedAt) })
	current := ""
	if m.deps.Session != nil {
		current = m.deps.Session.Meta.ID
	}
	var items []list.Item
	selected := 0
	for _, meta := range metas {
		title := meta.Title
		if title == "" {
			title = meta.ID
		}
		desc := meta.ID + " · " + meta.UpdatedAt.Local().Format("2006-01-02 15:04")
		if meta.Mode != "" {
			desc += " · " + meta.Mode
		}
		tag := ""
		if meta.ID == current {
			tag = "current"
			selected = len(items)
		}
		items = append(items, rowItem{title: title, desc: desc, id: meta.ID, tag: tag})
	}
	if len(items) == 0 {
		items = append(items, rowItem{title: "No sessions yet", desc: "this folder has an empty store"})
	}
	_ = m.sessions.SetItems(items)
	m.sessions.Select(selected)
}

// refreshPerms lists the rules the process loaded. It only describes the
// policy; deciding stays in internal/perms.
func (m *Model) refreshPerms() {
	items := []list.Item{rowItem{title: string(m.deps.Mode), desc: modeMeaning(m.deps.Mode), tag: "mode"}}
	if m.deps.ReviewOnly {
		items = append(items, rowItem{title: "review-only", desc: "edits and shell are denied", tag: "deny"})
	}
	if len(m.deps.Rules) == 0 {
		items = append(items,
			rowItem{title: "reads", desc: "read_file, grep, glob, web_fetch run without asking", tag: "allow"},
			rowItem{title: "edits", desc: "edit_file, write_file, shell, spawn_subagent ask first", tag: "ask"},
		)
	}
	for _, r := range m.deps.Rules {
		tag, rule := "allow", r
		for _, p := range []string{"allow", "ask", "deny"} {
			if after, ok := strings.CutPrefix(r, p+" "); ok {
				tag, rule = p, after
			}
		}
		items = append(items, rowItem{title: rule, desc: ruleMeaning(tag), id: r, tag: tag})
	}
	_ = m.perms.SetItems(items)
}

func modeMeaning(mode perms.Mode) string {
	switch mode {
	case perms.ModePlan:
		return "edits and every shell command are blocked; update_plan still writes the plan"
	case perms.ModeYolo:
		return "asks are skipped; deny rules and the destructive-action gate still block"
	default:
		return "rules below decide; anything else that mutates asks first"
	}
}

func ruleMeaning(tag string) string {
	switch tag {
	case "ask":
		return "asks you first"
	case "deny":
		return "always blocked"
	default:
		return "runs without asking"
	}
}

func (m *Model) refreshPlan() {
	if m.deps.Session == nil {
		return
	}
	raw, err := os.ReadFile(m.deps.Session.PlanPath())
	if err != nil || strings.TrimSpace(string(raw)) == "" {
		m.plan = "No plan yet."
		m.verdict = "plan: empty"
	} else {
		m.plan = string(raw)
		m.reportReady()
	}
	m.planW = 0
}

func (m *Model) reportReady() {
	ready, p := m.deps.Gates.PlanReady(context.Background(), m.plan, "")
	word := "not ready"
	if ready {
		word = "ready"
	}
	m.verdict = fmt.Sprintf("plan %s (%.2f, %s). This does not approve the plan.", word, p, m.deps.Gates.Mode())
}

func (m *Model) onMouse(msg tea.MouseMsg) {
	mo := msg.Mouse()
	switch msg.(type) {
	case tea.MouseWheelMsg:
		up := mo.Button == tea.MouseWheelUp
		switch m.overlay {
		case sessionsOverlay:
			wheelList(&m.sessions, up)
		case permsOverlay:
			wheelList(&m.perms, up)
		case agentsOverlay:
			if up {
				m.agents.MoveUp(1)
			} else {
				m.agents.MoveDown(1)
			}
		case helpOverlay:
			m.sheet, _ = m.sheet.Update(msg)
		default:
			if m.inPlan(mo.X, mo.Y) {
				m.planVP, _ = m.planVP.Update(msg)
				return
			}
			m.vp, _ = m.vp.Update(msg)
			m.follow = m.vp.AtBottom()
		}
	case tea.MouseClickMsg:
		if mo.Button == tea.MouseLeft && m.overlay == noOverlay && m.onScrollbar(mo.X, mo.Y) {
			m.dragging = true
			m.scrollTo(mo.Y)
		}
	case tea.MouseMotionMsg:
		if m.dragging {
			m.scrollTo(mo.Y)
		}
	case tea.MouseReleaseMsg:
		m.dragging = false
	}
}

func wheelList(l *list.Model, up bool) {
	if up {
		l.CursorUp()
	} else {
		l.CursorDown()
	}
}

func (m *Model) inPlan(x, y int) bool {
	g := m.geo
	switch {
	case g.planW > 0:
		return x >= g.transcriptW && y >= g.vpY && y < g.vpY+g.vpH
	case g.planH > 0:
		return y >= g.bodyY && y < g.bodyY+g.planH
	}
	return false
}

func (m *Model) overflowing() bool {
	return m.vp.TotalLineCount() > m.vp.Height()
}

func (m *Model) onScrollbar(x, y int) bool {
	g := m.geo
	return m.overflowing() && x == g.transcriptW-1 && y >= g.vpY && y < g.vpY+g.vpH
}

// scrollTo maps a row on the scrollbar track to a transcript offset.
func (m *Model) scrollTo(y int) {
	g := m.geo
	maxOff := m.vp.TotalLineCount() - m.vp.Height()
	if maxOff <= 0 || g.vpH < 1 {
		return
	}
	rel := min(max(y-g.vpY, 0), g.vpH-1)
	off := maxOff
	if g.vpH > 1 {
		off = int(math.Round(float64(rel) / float64(g.vpH-1) * float64(maxOff)))
	}
	m.vp.SetYOffset(off)
	m.follow = m.vp.AtBottom()
}
