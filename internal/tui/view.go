package tui

import (
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/x/ansi"

	"github.com/StephenSHorton/rock/internal/perms"
)

const sandboxNote = "Permissions are not a sandbox. Plan mode blocks every shell command because redirections are not inspected."

// layout sizes every component for the window and the current state. Rows
// are handed out top-down from a fixed budget, so the body is what shrinks
// and the composer, status line, and shortcuts bar always stay on screen.
// The live session row sits under the composer, above the shortcuts, matching
// Grok's [ui.status_line] slot. There is no branded header.
func (m *Model) layout() {
	w, h := max(m.width, 20), max(m.height, 6)
	g := geometry{w: w, h: h, composerRows: 3, infoRows: 1, helpRows: 1}
	g.compact = h <= compactAt
	if !g.compact && h > shortAt {
		g.padT, g.padB, g.padL, g.padR = 1, 1, 1, 1
	}
	g.innerW = max(1, w-g.padL-g.padR)
	g.bodyY = g.padT

	chrome := func() int {
		return g.padT + 1 + g.composerRows + g.infoRows + g.helpRows + g.padB
	}
	for g.composerRows > 1 && h-chrome() < 4 {
		g.composerRows--
	}
	if g.infoRows > 0 && h-chrome() < 3 {
		g.infoRows = 0
	}
	if h-chrome() < 1 {
		g.helpRows = 0
	}
	g.bodyH = max(1, h-chrome())

	g.transcriptW = g.innerW
	switch {
	case m.wantPlan() && g.innerW >= splitMin:
		g.planW = min(max(g.innerW*32/100, 28), 46)
		g.transcriptW = g.innerW - g.planW - 1
	case m.wantPlan():
		inner := max(1, g.innerW-4)
		m.planContent(inner)
		want := 2 + lipgloss.Height(m.planHeader(inner, true)) + m.planVP.TotalLineCount()
		g.planH = min(max(want, 5), max(5, g.bodyH/2))
	}
	if m.pending != nil {
		g.askH = min(m.askHeight(g.innerW), g.bodyH)
	}
	if g.planH > 0 && g.bodyH-g.askH-g.planH < 3 {
		g.planH = 0
	}
	g.vpY = g.bodyY + g.planH
	g.vpH = max(0, g.bodyH-g.planH-g.askH)
	g.barX = g.padL + g.transcriptW - 1
	m.geo = g

	// Accent column + prompt leave innerW-2 for the textarea.
	m.input.SetWidth(max(1, g.innerW-2))
	m.input.SetHeight(g.composerRows)
	m.help.SetWidth(max(1, g.innerW-1))
	m.meter.SetWidth(meterWidth(g.innerW))

	// Reserved scrollbar gutter: last column of the transcript.
	contentW := max(1, g.transcriptW-2)
	m.vp.SetWidth(contentW)
	m.vp.SetHeight(g.vpH)
	if contentW != m.contentW {
		m.contentW = contentW
		m.syncView()
	} else if m.follow {
		m.vp.GotoBottom()
	} else {
		m.vp.SetYOffset(m.vp.YOffset())
	}

	if pw, ph, stacked := m.planBox(); pw > 0 {
		inner := max(1, pw-4)
		m.planContent(inner)
		headH := lipgloss.Height(m.planHeader(inner, stacked))
		footH := 0
		if !stacked {
			footH = lipgloss.Height(m.planFooter(inner))
		}
		m.planVP.SetWidth(inner)
		m.planVP.SetHeight(max(0, ph-2-headH-footH))
	}

	innerW, innerH := max(1, g.innerW-4), max(1, g.bodyH-2)
	listH := max(1, innerH-2)
	m.sessions.SetSize(innerW, listH)
	m.palette.SetSize(innerW, listH)
	m.perms.SetSize(innerW, max(1, listH-lipgloss.Height(wrapText(sandboxNote, innerW))-1))
	m.agents.SetColumns(agentColumns(innerW))
	m.agents.SetWidth(innerW)
	m.agents.SetHeight(max(2, listH))
	m.sheet.SetWidth(max(1, innerW-2))
	m.sheet.SetHeight(listH)
	if m.overlay == helpOverlay {
		m.sheet.SetContent(m.helpText(max(1, innerW-2)))
	}
	m.choices.SetSize(innerW, 2)
}

func meterWidth(w int) int {
	switch {
	case w >= 120:
		return 18
	case w >= 90:
		return 14
	default:
		return 8
	}
}

// planBox is the outer size of the plan pane, and whether it is stacked
// above the transcript instead of beside it.
func (m *Model) planBox() (w, h int, stacked bool) {
	g := m.geo
	switch {
	case g.planW > 0:
		return g.planW, g.vpH, false
	case g.planH > 0:
		return g.innerW, g.planH, true
	}
	return 0, 0, false
}

func (m *Model) View() tea.View {
	if m.width == 0 {
		v := tea.NewView("rock")
		v.AltScreen = true
		v.MouseMode = tea.MouseModeCellMotion
		return v
	}
	g := m.geo
	var parts []string
	if g.padT > 0 {
		parts = append(parts, strings.Repeat("\n", g.padT-1))
	}
	parts = append(parts,
		m.inset(m.bodyView(), g.bodyH),
		m.inset(m.composerView(), g.composerRows+g.infoRows),
		m.inset(m.statusView(), 1),
	)
	if g.helpRows > 0 {
		parts = append(parts, m.inset(m.helpLineView(), 1))
	}
	if g.padB > 0 {
		parts = append(parts, "")
	}
	v := tea.NewView(strings.Join(parts, "\n"))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "rock"
	return v
}

// inset places one band of content in the horizontal padding so the
// scrollback, composer, and shortcuts share one gutter.
func (m *Model) inset(s string, h int) string {
	g := m.geo
	inner := block(s, g.innerW, h)
	if g.padL == 0 && g.padR == 0 {
		return inner
	}
	left, right := strings.Repeat(" ", g.padL), strings.Repeat(" ", g.padR)
	rows := strings.Split(inner, "\n")
	for i := range rows {
		rows[i] = left + rows[i] + right
	}
	return strings.Join(rows, "\n")
}

func (m *Model) bodyView() string {
	g := m.geo
	if m.overlay != noOverlay {
		return m.overlayView(g.innerW, g.bodyH)
	}
	var rows []string
	if g.planH > 0 {
		rows = append(rows, m.planView(g.innerW, g.planH, true))
	}
	if g.vpH > 0 {
		t := m.transcriptView(g.transcriptW, g.vpH)
		if g.planW > 0 {
			t = joinColumns(t, " ", m.planView(g.planW, g.vpH, false))
		}
		rows = append(rows, t)
	}
	if g.askH > 0 {
		rows = append(rows, m.askView(g.innerW, g.askH))
	}
	return strings.Join(rows, "\n")
}

// transcriptView is the viewport with one column of air on the left and the
// scrollbar on the right. The last user prompt pins at the top when the
// viewport has scrolled past it.
func (m *Model) transcriptView(w, h int) string {
	lines := strings.Split(m.vp.View(), "\n")
	bar := m.scrollbar(h)
	out := make([]string, h)
	for i := range out {
		l := ""
		if i < len(lines) {
			l = lines[i]
		}
		out[i] = " " + padRight(l, w-2) + bar[i]
	}
	if stickyH, from := m.stickyUser(); stickyH > 0 && from >= 0 && m.contentW > 0 {
		pin := m.renderUser(clip(sanitize(m.lines[from].text), max(1, m.contentW-2)), m.contentW, m.selected == from, false)
		for i, row := range strings.Split(pin, "\n") {
			if i >= h {
				break
			}
			out[i] = " " + padRight(row, w-2) + bar[i]
		}
	}
	return strings.Join(out, "\n")
}

func (m *Model) scrollbar(h int) []string {
	return m.scrollbarFor(&m.vp, h)
}

func (m *Model) scrollbarFor(vp *viewport.Model, h int) []string {
	bar := make([]string, h)
	total := vp.TotalLineCount()
	if total <= h || h < 2 {
		for i := range bar {
			bar[i] = " "
		}
		return bar
	}
	thumb := max(1, h*h/total)
	pos := 0
	if maxOff := total - h; maxOff > 0 {
		pos = int(math.Round(float64(vp.YOffset()) / float64(maxOff) * float64(h-thumb)))
	}
	track := lipgloss.NewStyle().Foreground(mix(m.th.panel, m.th.muted, 0.45))
	for i := range bar {
		if i >= pos && i < pos+thumb {
			bar[i] = m.th.thumb.Render("┃")
		} else {
			bar[i] = track.Render("│")
		}
	}
	return bar
}

func (m *Model) planHeader(innerW int, stacked bool) string {
	t := m.th
	planMode := m.deps.Mode == perms.ModePlan
	if stacked {
		head := "Plan"
		if planMode {
			head += " · plan mode, shell is blocked"
		}
		return t.accentBold.Render(clip(head, innerW)) + "\n" + t.faint.Render(clip(m.verdict, innerW))
	}
	rows := []string{t.accentBold.Render("Plan")}
	if planMode {
		rows = append(rows, t.alarm.Render(wrapText("Plan mode. Shell is blocked, and so are edits outside the plan. /default leaves it.", innerW)))
	} else {
		rows = append(rows, t.faint.Render(wrapText("/plan blocks edits and shell while you plan.", innerW)))
	}
	rows = append(rows, t.faint.Render(wrapText(m.verdict, innerW)), "")
	return strings.Join(rows, "\n")
}

// planContent renders the plan into its viewport when the width or the plan
// changed since the last render.
func (m *Model) planContent(innerW int) {
	if innerW != m.planW {
		m.planW = innerW
		m.planVP.SetContent(m.planBody(innerW))
	}
}

func (m *Model) planBody(innerW int) string {
	if strings.TrimSpace(m.plan) == "" || m.plan == "No plan yet." {
		return m.th.plain.Render("No plan yet.")
	}
	return m.markdown(sanitize(m.plan), innerW)
}

func (m *Model) planFooter(innerW int) string {
	return m.th.faint.Render(clip(m.verdict, innerW))
}

func (m *Model) planView(w, h int, stacked bool) string {
	innerW := max(1, w-4)
	box := m.th.box
	if m.deps.Mode == perms.ModePlan {
		box = m.th.boxFocus
	}
	content := m.planHeader(innerW, stacked) + "\n" + m.planVP.View()
	if !stacked {
		content += "\n" + m.planFooter(innerW)
	}
	return boxed(box, w, h, content)
}

func (m *Model) overlayView(w, h int) string {
	t := m.th
	innerW := max(1, w-4)
	var title, sub, body, foot string
	switch m.overlay {
	case helpOverlay:
		title, sub = "Help", "keys and slash commands"
		lines := strings.Split(m.sheet.View(), "\n")
		bar := m.scrollbarFor(&m.sheet, len(lines))
		for i := range lines {
			lines[i] = padRight(lines[i], innerW-1) + bar[i]
		}
		body = strings.Join(lines, "\n")
	case sessionsOverlay:
		title, sub = "Sessions", "this folder, newest first"
		body = m.sessions.View()
	case permsOverlay:
		title, sub = "Permissions", "what this process loaded; mode first"
		body = m.perms.View()
		foot = t.alarm.Render(wrapText(sandboxNote, innerW))
	case agentsOverlay:
		title, sub = "Subagents", "depth 1; explore, plan, or general"
		if len(m.agents.Rows()) == 0 {
			body = t.faint.Render(wrapText("No subagents yet. The model starts one with spawn_subagent.", innerW))
		} else {
			body = m.agents.View()
		}
	case paletteOverlay:
		title, sub = "Commands", "slash commands and keys"
		body = m.palette.View()
	}
	head := t.accentBold.Render(title) + t.faint.Render("  "+clip(sub, max(0, innerW-ansi.StringWidth(title)-2)))
	content := head + "\n\n" + body
	if foot != "" {
		content = head + "\n\n" + block(body, innerW, m.perms.Height()) + "\n\n" + foot
	}
	return boxed(t.boxFocus, w, h, content)
}

func (m *Model) helpText(w int) string {
	t := m.th
	keys := [][2]string{
		{"enter", "send"},
		{"tab / shift+tab", "focus · cycle default/plan/yolo"},
		{"pgup/pgdn", "scroll the transcript"},
		{"ctrl+p", "command palette"},
		{"ctrl+s / ctrl+o", "sessions · toggle yolo"},
		{"←/→", "fold the selected block"},
		{"ctrl+h / ctrl+.", "this help"},
		{"esc", "close; deny; cancel a running turn"},
		{"ctrl+c twice", "quit"},
	}
	cmds := [][2]string{
		{"/help", "this help"},
		{"/plan", "plan mode: edits and shell blocked"},
		{"/yolo", "skip asks; destructive gate stays"},
		{"/default", "back to the default policy"},
		{"/sessions", "resume a session from this folder"},
		{"/permissions", "allow, ask, and deny rules"},
		{"/agents", "subagents spawned this session"},
		{"/ready", "is the plan ready? never approves"},
		{"/verbose", "show or hide Jev turn/risk diagnostics"},
		{"/fork", "new Rock pane in Suzuri (OSC 7880)"},
		{"/quit", "quit"},
	}
	section := func(title string, rows [][2]string, width int) string {
		keyW := 0
		for _, r := range rows {
			keyW = max(keyW, ansi.StringWidth(r[0]))
		}
		out := []string{t.accentBold.Render(title)}
		descW := max(8, width-keyW-2)
		for _, r := range rows {
			desc := strings.Split(wrapText(r[1], descW), "\n")
			for i, d := range desc {
				k := strings.Repeat(" ", keyW)
				if i == 0 {
					k = padRight(r[0], keyW)
				}
				out = append(out, t.chrome.Render(k)+"  "+t.plain.Render(d))
			}
		}
		return strings.Join(out, "\n")
	}
	left, right := section("Keys", keys, 1<<10), section("Slash commands", cmds, 1<<10)
	if lw := lipgloss.Width(left); lw+6+lipgloss.Width(right) <= w {
		return joinColumns(padBlock(left, lw), "      ", right)
	}
	return section("Keys", keys, w) + "\n\n" + section("Slash commands", cmds, w)
}

func (m *Model) askHeight(w int) int {
	return lipgloss.Height(m.askContent(max(1, w-4), 1<<10)) + 2
}

func (m *Model) askView(w, h int) string {
	return boxed(m.th.boxAlert, w, h, m.askContent(max(1, w-4), max(1, h-2)))
}

// askContent is the permission dialog. When rows are short the call preview
// gives way first, so the question, the choices, and the keys stay visible.
func (m *Model) askContent(w, maxH int) string {
	t := m.th
	p := m.pending
	if p == nil {
		return ""
	}
	title := t.alarmBold.Render(clip("Allow "+p.tool+"?", w))
	choices := m.choices.View()
	hint := t.faint.Render(wrapText("y allow · n deny · esc deny. An allowed call still passes the destructive-action gate.", w))
	fixed := 1 + 1 + lipgloss.Height(choices) + 1 + lipgloss.Height(hint)
	preview := m.preview(p.tool, p.detail, w)
	room := maxH - fixed
	if room < 1 {
		preview = nil
	} else if len(preview) > room {
		preview = append(preview[:room-1], t.faint.Render("…"))
	}
	rows := []string{title}
	if len(preview) > 0 {
		rows = append(rows, preview...)
	}
	rows = append(rows, "", choices, "", hint)
	if room < 1 {
		rows = []string{title, choices, hint}
	}
	return strings.Join(rows, "\n")
}

// preview shows what the pending call will do, from the tool call event the
// harness emitted just before it asked.
func (m *Model) preview(tool, detail string, w int) []string {
	t := m.th
	var args map[string]any
	if m.lastCall.Name == tool {
		_ = json.Unmarshal([]byte(m.lastCall.Text), &args)
	}
	str := func(k string) string {
		s, _ := args[k].(string)
		return sanitize(s)
	}
	field := func(label, value string) string {
		if value == "" {
			value = sanitize(detail)
		}
		return t.faint.Render(label+" ") + t.plain.Render(clip(value, max(1, w-len(label)-1)))
	}
	var out []string
	switch tool {
	case "shell":
		cmd := str("command")
		if cmd == "" {
			cmd = sanitize(detail)
		}
		for _, l := range strings.Split(wrapText("$ "+cmd, w), "\n") {
			out = append(out, t.plain.Render(l))
		}
	case "edit_file":
		out = append(out, field("path", str("path")))
		if args != nil {
			out = append(out, m.hunkRows(str("old"), str("new"), w, 6)...)
		}
	case "write_file":
		out = append(out, field("path", str("path")))
		if args != nil {
			out = append(out, m.snippetRows("  ", str("content"), t.faint, w, 6)...)
		}
	case "spawn_subagent":
		out = append(out, field("kind", str("kind")))
		if prompt := str("prompt"); prompt != "" {
			for _, l := range strings.Split(wrapText(prompt, w), "\n") {
				out = append(out, t.plain.Render(l))
			}
		}
	default:
		for _, l := range strings.Split(wrapText(sanitize(detail), w), "\n") {
			out = append(out, t.plain.Render(l))
		}
	}
	return out
}

func (m *Model) statusView() string {
	t, w := m.th, m.geo.innerW
	mode := string(m.deps.Mode)
	badges := t.badge(mode).Render(strings.ToUpper(mode))
	if m.deps.ReviewOnly {
		badges += " " + t.badgeReview.Render("REVIEW")
	}
	jevSeg := t.chrome.Render("jev:" + m.deps.JevMode)

	name := m.deps.FastModel
	if m.turnModel != "" {
		name = m.turnModel
	}
	if name == "" {
		name = "no model"
	}
	note := ""
	if m.deps.Provider == "offline" {
		note = " (offline)"
	}
	pct := fmt.Sprintf("%.0f%%", m.meterP*100)
	pctStyle := t.plain
	if m.meterP >= 0.8 {
		pctStyle = t.alarm
	}
	cwd := shortPath(m.deps.CWD)
	title := ""
	if m.deps.Session != nil {
		title = strings.TrimSpace(m.deps.Session.Meta.Title)
	}
	timer := ""
	if m.busy && !m.turnAt.IsZero() {
		timer = fmtDuration(time.Since(m.turnAt))
	}

	sep := t.faint.Render(" │ ")
	join := func(parts []string) string {
		var keep []string
		for _, p := range parts {
			if strings.TrimSpace(ansi.Strip(p)) != "" {
				keep = append(keep, p)
			}
		}
		return " " + strings.Join(keep, sep)
	}
	ctxSeg := func(barW int) string {
		s := pctStyle.Render(pct) + t.chrome.Render(" ctx")
		if barW > 0 {
			meter := m.meter
			meter.SetWidth(barW)
			fill := m.meterP
			if fill > 0 && fill*float64(barW) < 1 {
				fill = 1 / float64(barW)
			}
			s += " " + meter.ViewAs(fill)
		}
		return s
	}
	modelSeg := func(withNote bool) string {
		s := t.plain.Render(name)
		if withNote {
			s += t.faint.Render(note)
		}
		return s
	}

	build := func(barW int, withNote, withTitle, withTimer, withCwd bool, cwdW int) string {
		cwdSeg := ""
		if withCwd {
			path := cwd
			if cwdW > 0 {
				path = clipLeft(cwd, cwdW)
			}
			cwdSeg = t.faint.Render(path)
		}
		parts := []string{badges, jevSeg, cwdSeg, modelSeg(withNote), ctxSeg(barW)}
		if withTitle && title != "" {
			parts = append(parts, t.faint.Render(title))
		}
		if withTimer && timer != "" {
			parts = append(parts, t.chrome.Render(timer))
		}
		return join(parts)
	}

	bar := m.meter.Width()
	left := build(bar, true, true, true, true, 0)
	for _, try := range []struct {
		bar                         int
		note, title, timer, withCwd bool
		cwdW                        int
	}{
		{bar, true, true, true, true, 24},
		{bar, true, true, false, true, 16},
		{bar, true, false, false, true, 12},
		{bar, true, false, false, false, 0},
		{6, true, false, false, false, 0},
		{0, true, false, false, false, 0},
		{0, false, false, false, false, 0},
	} {
		if ansi.StringWidth(left) <= w {
			break
		}
		left = build(try.bar, try.note, try.title, try.timer, try.withCwd, try.cwdW)
	}
	left = ansi.Truncate(left, w, "…")

	status := sanitize(m.status)
	statusStyle := t.faint
	if m.alert {
		statusStyle = t.danger
	} else if m.busy {
		statusStyle = t.alarm
	}
	right := ""
	if m.busy {
		right = m.spin.View() + " "
	}
	room := w - ansi.StringWidth(left) - 3 - ansi.StringWidth(right)
	if room >= 6 {
		right += statusStyle.Render(clip(status, room))
	} else if right == "" {
		return left
	}
	gap := max(2, w-ansi.StringWidth(left)-ansi.StringWidth(right)-1)
	return left + strings.Repeat(" ", gap) + right
}

func fmtDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}

func (m *Model) composerView() string {
	g, t := m.geo, m.th
	accent := t.faint.Render("│")
	if m.input.Focused() {
		accent = t.accent.Render("┃")
	}
	rows := strings.Split(block(m.input.View(), max(1, g.innerW-2), g.composerRows), "\n")
	for i := range rows {
		rows[i] = accent + " " + rows[i]
	}
	if g.infoRows > 0 {
		rows = append(rows, "  "+m.composerInfo(max(1, g.innerW-2)))
	}
	return strings.Join(rows, "\n")
}

func (m *Model) composerInfo(w int) string {
	t := m.th
	name := m.deps.FastModel
	if m.turnModel != "" {
		name = m.turnModel
	}
	if name == "" {
		name = "no model"
	}
	mode := string(m.deps.Mode)
	return ansi.Truncate(t.plain.Render(name)+t.faint.Render(" · "+mode), w, "…")
}

func (m *Model) helpLineView() string {
	k := m.keys
	var bindings []key.Binding
	switch {
	case m.pending != nil:
		bindings = []key.Binding{
			k.allow, k.deny,
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "deny")),
			k.move,
		}
	case m.overlay == paletteOverlay:
		bindings = []key.Binding{k.pick, key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close"))}
	case m.overlay == sessionsOverlay:
		bindings = []key.Binding{k.move, k.pick, k.close}
	case m.overlay == helpOverlay:
		bindings = []key.Binding{key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("↑/↓ pgup pgdn", "scroll")), k.close}
	case m.overlay != noOverlay:
		bindings = []key.Binding{k.move, k.close}
	case m.focusTranscript:
		bindings = []key.Binding{k.focus, k.fold, k.back, k.scroll}
	case m.busy:
		bindings = []key.Binding{k.newline, k.scroll, key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")), k.quit}
	default:
		bindings = k.ShortHelp()
	}
	return " " + m.help.ShortHelpView(bindings)
}

func (m *Model) syncView() {
	if m.contentW <= 0 {
		return
	}
	m.vp.SetContent(m.renderTranscript(m.contentW))
	if m.follow {
		m.vp.GotoBottom()
	}
}

func (m *Model) renderTranscript(width int) string {
	m.spans = nil
	if len(m.lines) == 0 {
		return m.th.faint.Render(wrapText(readyText, width))
	}
	var b strings.Builder
	y := 0
	write := func(s string) (y0, y1 int) {
		if s == "" {
			return y, y
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
			y++
		}
		y0 = y
		b.WriteString(s)
		y += strings.Count(s, "\n")
		return y0, y
	}
	gap := func() {
		if b.Len() == 0 {
			return
		}
		b.WriteByte('\n')
		y++
	}
	i := 0
	for i < len(m.lines) {
		if !m.verbose && diagnosticJev(m.lines[i]) {
			i++
			continue
		}
		end, kind, n := verbRun(m.lines, i)
		if n > 1 {
			if i > 0 {
				gap()
			}
			folded := m.isFolded(i)
			sel := m.selected >= i && m.selected < end
			y0, y1 := write(m.renderGroup(kind, n, m.lines[i:end], width, sel, folded))
			m.spans = append(m.spans, entrySpan{from: i, to: end - 1, y0: y0, y1: y1, group: true})
			i = end
			continue
		}
		ln := &m.lines[i]
		if i > 0 && (ln.kind == "user" || ln.kind == "assistant" || ln.kind == "error") {
			gap()
		}
		sel := m.selected == i
		folded := m.isFolded(i)
		running := m.toolRunning(i)
		if ln.out == "" || ln.outW != width || ln.outDark != m.th.dark {
			ln.out, ln.outW, ln.outDark = m.renderLine(*ln, width, false, false, false), width, m.th.dark
		}
		painted := ln.out
		if sel || folded || running || ln.kind == "tool" || lineFoldable(*ln, width) {
			painted = m.renderLine(*ln, width, sel, folded, running)
		}
		y0, y1 := write(painted)
		m.spans = append(m.spans, entrySpan{from: i, to: i, y0: y0, y1: y1, group: false})
		i++
	}
	return b.String()
}

func (m *Model) renderLine(ln line, width int, selected, folded, running bool) string {
	t := m.th
	text := sanitize(ln.text)
	switch ln.kind {
	case "user":
		return m.renderUser(text, width, selected, folded)
	case "assistant":
		return m.renderAssistant(text, width, selected, folded, ln.raw)
	case "error":
		bodyW := max(8, width-blockPad)
		body := t.danger.Render(wrapText(text, bodyW))
		if folded && lineFoldable(ln, width) {
			body = t.danger.Render(clip(text, bodyW))
		}
		return m.markBlock(body, width, selected, folded)
	case "tool":
		return m.renderTool(ln, width, selected, folded, running)
	case "result":
		mark, st := t.chrome.Render("✓ "), t.faint
		if strings.HasPrefix(text, "denied") {
			mark, st = t.danger.Render("✗ "), t.danger
		}
		return m.eventRow(mark+t.plain.Render(ln.name), resultSummary(text), st, width, selected)
	case "permission":
		decision, why, _ := strings.Cut(text, ":")
		st := t.chrome
		if decision == string(perms.Deny) {
			st = t.danger
		}
		return m.eventRow(st.Render("⚑ ")+t.plain.Render(ln.name)+"  "+st.Render(decision), strings.TrimSpace(why), t.faint, width, selected)
	case "jev":
		return m.eventRow(t.faint.Render("◇ jev "+ln.name), text, t.faint, width, selected)
	default:
		return m.eventRow(t.faint.Render(ln.kind), text, t.faint, width, selected)
	}
}

func (m *Model) renderUser(text string, width int, selected, folded bool) string {
	t := m.th
	prefix := "❯ "
	if folded {
		prefix = "› "
	}
	bodyW := max(8, width-2)
	rows := strings.Split(wrapText(text, bodyW), "\n")
	if folded && len(rows) > 3 {
		rows = append(rows[:2], clip(rows[2], max(1, bodyW-2))+" …")
	}
	caret, body := t.accentBold, t.plain
	if selected {
		body = t.strong
	}
	band := t.bar
	out := make([]string, len(rows))
	for i, row := range rows {
		p := prefix
		if i > 0 {
			p = "  "
		}
		out[i] = band.Render(caret.Render(p) + body.Render(padRight(row, bodyW)))
	}
	return strings.Join(out, "\n")
}

func (m *Model) renderAssistant(text string, width int, selected, folded, raw bool) string {
	bodyW := max(8, width-blockPad)
	body := m.markdown(text, bodyW)
	if raw {
		body = m.th.plain.Render(wrapText(text, bodyW))
	}
	if folded && visualRows(body) > 6 {
		rows := strings.Split(body, "\n")
		body = strings.Join(rows[:5], "\n") + "\n" + m.th.faint.Render("› …")
	}
	return m.markBlock(body, width, selected, folded)
}

func (m *Model) renderGroup(kind string, n int, members []line, width int, selected, folded bool) string {
	t := m.th
	mark := ""
	if folded && !selected {
		mark = t.faint.Render("› ")
	}
	head := m.eventRow(mark+t.strong.Render(verbLabel(kind, n)), "", t.faint, width, selected)
	if folded {
		return head
	}
	var rows []string
	rows = append(rows, head)
	for _, ln := range members {
		memberFold := ln.kind == "tool" && toolFoldsByDefault(ln.name) && !ln.foldTouched || ln.folded
		rows = append(rows, m.renderLine(ln, width, false, memberFold, false))
	}
	return strings.Join(rows, "\n")
}

func (m *Model) markBlock(body string, width int, selected, folded bool) string {
	t := m.th
	lead := strings.Repeat(" ", blockPad)
	if selected {
		lead = t.accent.Render("▎") + strings.Repeat(" ", max(0, blockPad-1))
	} else if folded {
		lead = t.faint.Render("›") + strings.Repeat(" ", max(0, blockPad-1))
	}
	lines := strings.Split(body, "\n")
	for i := range lines {
		p := lead
		if i > 0 {
			p = strings.Repeat(" ", blockPad)
			if selected {
				p = t.accent.Render("▎") + strings.Repeat(" ", max(0, blockPad-1))
			}
		}
		lines[i] = p + lines[i]
	}
	return strings.Join(lines, "\n")
}

// eventRow is one tool, permission, or Jev row, inset by blockPad.
func (m *Model) eventRow(head, detail string, st lipgloss.Style, width int, selected bool) string {
	t := m.th
	inner := max(8, width-blockPad)
	row := ansi.Truncate(head, inner, "…")
	if room := inner - ansi.StringWidth(row) - 2; room > 0 && detail != "" {
		row += "  " + st.Render(clip(detail, room))
	}
	lead := strings.Repeat(" ", blockPad)
	if selected {
		lead = t.accent.Render("▎") + strings.Repeat(" ", max(0, blockPad-1))
	}
	return lead + row
}

func toolFoldsByDefault(name string) bool {
	switch name {
	case "read_file", "grep", "glob", "web_fetch":
		return true
	}
	return false
}

func (m *Model) renderTool(ln line, width int, selected, folded, running bool) string {
	t := m.th
	inner := max(8, width-blockPad)
	denied := strings.HasPrefix(ln.result, "denied")
	verb, detail := toolHeading(ln.name, ln.text, running && !ln.done)
	extra := toolDetail(ln)
	if d := toolElapsed(ln); d != "" {
		if extra != "" {
			extra += "  " + d
		} else {
			extra = d
		}
	}

	bulletSt, headSt, dimSt := t.chrome, t.strong, t.faint
	if folded {
		headSt, dimSt = t.faint, t.faint
	}
	if denied {
		bulletSt, headSt = t.danger, t.danger
	}
	if running {
		bulletSt = t.alarm
	}
	if selected {
		bulletSt = t.accent
	}

	head := bulletSt.Render("◆ ") + headSt.Render(verb)
	if detail != "" {
		head += "  " + headSt.Render(clip(detail, max(1, inner-ansi.StringWidth(verb)-4)))
	}
	row := ansi.Truncate(head, inner, "…")
	if room := inner - ansi.StringWidth(row) - 2; room > 0 && extra != "" {
		row += "  " + dimSt.Render(clip(extra, room))
	}
	lead := strings.Repeat(" ", blockPad)
	if selected {
		lead = t.accent.Render("▎") + strings.Repeat(" ", max(0, blockPad-1))
	} else if running {
		lead = t.alarm.Render("▎") + strings.Repeat(" ", max(0, blockPad-1))
	}
	out := lead + row
	if folded {
		return out
	}
	body := m.toolBody(ln, inner)
	if body == "" {
		return out
	}
	return out + "\n" + indentBlock(body, lead)
}

func toolHeading(name, args string, running bool) (verb, detail string) {
	detail = toolSummary(name, args)
	switch name {
	case "read_file":
		if running {
			return "Reading", detail
		}
		return "Read", detail
	case "grep", "glob":
		if running {
			return "Searching", detail
		}
		return "Searched", detail
	case "web_fetch":
		if running {
			return "Fetching", detail
		}
		return "Fetched", detail
	case "shell":
		if strings.HasPrefix(detail, "$ ") {
			return detail, ""
		}
		if running {
			return "Run", detail
		}
		return "Ran", detail
	case "edit_file":
		if running {
			return "Editing", detail
		}
		return "Edit", detail
	case "write_file":
		if running {
			return "Writing", detail
		}
		return "Write", detail
	case "update_plan":
		return "Update plan", ""
	case "spawn_subagent":
		return "Ran", detail
	default:
		return name, detail
	}
}

func toolDetail(ln line) string {
	if !ln.done {
		return ""
	}
	if strings.HasPrefix(ln.result, "denied") {
		return "denied"
	}
	switch ln.name {
	case "edit_file", "write_file":
		var raw map[string]any
		if json.Unmarshal([]byte(ln.text), &raw) != nil {
			return resultSummary(ln.result)
		}
		old, _ := raw["old"].(string)
		neu, _ := raw["new"].(string)
		if ln.name == "write_file" {
			neu, _ = raw["content"].(string)
		}
		add, del := countLines(neu), countLines(old)
		if add == 0 && del == 0 {
			return resultSummary(ln.result)
		}
		return fmt.Sprintf("+%d/-%d", add, del)
	case "read_file", "grep", "glob", "web_fetch", "shell":
		n := countLines(ln.result)
		if n == 0 {
			return "(no output)"
		}
		if n == 1 {
			return "1 line"
		}
		return fmt.Sprintf("%d lines", n)
	}
	return resultSummary(ln.result)
}

func toolElapsed(ln line) string {
	if ln.at.IsZero() || ln.finished.IsZero() || !ln.done {
		return ""
	}
	d := ln.finished.Sub(ln.at)
	if d < 10*time.Millisecond {
		return ""
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func countLines(s string) int {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func (m *Model) toolBody(ln line, width int) string {
	t := m.th
	if strings.HasPrefix(ln.result, "denied") {
		return t.danger.Render(wrapText(sanitize(ln.result), width))
	}
	switch ln.name {
	case "edit_file":
		var raw map[string]any
		if json.Unmarshal([]byte(ln.text), &raw) != nil {
			return t.faint.Render(wrapText(resultSummary(ln.result), width))
		}
		old, _ := raw["old"].(string)
		neu, _ := raw["new"].(string)
		return strings.Join(m.hunkRows(old, neu, width, 6), "\n")
	case "write_file":
		var raw map[string]any
		if json.Unmarshal([]byte(ln.text), &raw) != nil {
			return t.faint.Render(wrapText(resultSummary(ln.result), width))
		}
		content, _ := raw["content"].(string)
		return strings.Join(m.snippetRows("  ", content, t.faint, width, 6), "\n")
	case "shell":
		return strings.Join(m.shellRows(ln.result, width), "\n")
	default:
		if !ln.done {
			return ""
		}
		return t.faint.Render(wrapText(resultSummary(ln.result), width))
	}
}

func (m *Model) hunkRows(old, neu string, w, limit int) []string {
	var out []string
	out = append(out, m.snippetRows("- ", old, m.th.minus, w, limit)...)
	if old != "" && neu != "" {
		out = append(out, m.th.faint.Render("…"))
	}
	out = append(out, m.snippetRows("+ ", neu, m.th.plus, w, limit)...)
	return out
}

func (m *Model) snippetRows(prefix, text string, st lipgloss.Style, w, limit int) []string {
	var out []string
	ls := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if text == "" || (len(ls) == 1 && ls[0] == "") {
		return out
	}
	for i, l := range ls {
		if i == limit {
			out = append(out, m.th.faint.Render(fmt.Sprintf("  … %d more lines", len(ls)-limit)))
			break
		}
		out = append(out, st.Render(clip(prefix+l, w)))
	}
	return out
}

func (m *Model) shellRows(text string, w int) []string {
	t := m.th
	ls := nonemptyLines(text)
	if len(ls) == 0 {
		return []string{t.faint.Render("(no output)")}
	}
	keep := ls
	if len(ls) > 5 {
		keep = append(append([]string{}, ls[:2]...), ls[len(ls)-3:]...)
	}
	var out []string
	for i, l := range keep {
		if len(ls) > 5 && i == 2 {
			out = append(out, t.faint.Render("…"))
		}
		out = append(out, t.plain.Render(clip(l, w)))
	}
	return out
}

func nonemptyLines(text string) []string {
	var rows []string
	for _, l := range strings.Split(text, "\n") {
		if strings.TrimSpace(l) != "" {
			rows = append(rows, l)
		}
	}
	return rows
}

func indentBlock(s, lead string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = lead + lines[i]
	}
	return strings.Join(lines, "\n")
}

func groupKind(ln line) (string, bool) {
	if ln.kind != "tool" {
		return "", false
	}
	switch ln.name {
	case "read_file":
		return "file", true
	case "grep", "glob":
		return "search", true
	case "web_fetch":
		return "fetch", true
	}
	return "", false
}

// verbRun is a run of consecutive read/search/fetch tool rows, swallowing
// their matching results so interleaved ✓ rows do not split the group.
func verbRun(lines []line, i int) (end int, kind string, n int) {
	if i < 0 || i >= len(lines) {
		return i + 1, "", 1
	}
	kind, ok := groupKind(lines[i])
	if !ok {
		return i + 1, "", 1
	}
	n = 1
	j := i + 1
	for j < len(lines) {
		if lines[j].kind == "result" {
			if _, ok := groupKind(line{kind: "tool", name: lines[j].name}); ok {
				j++
				continue
			}
			break
		}
		k, ok := groupKind(lines[j])
		if !ok || k != kind {
			break
		}
		n++
		j++
	}
	return j, kind, n
}

func verbLabel(kind string, n int) string {
	noun := func(one, many string) string {
		if n == 1 {
			return one
		}
		return many
	}
	switch kind {
	case "search":
		return fmt.Sprintf("Searched %d %s", n, noun("pattern", "patterns"))
	case "fetch":
		return fmt.Sprintf("Fetched %d %s", n, noun("website", "websites"))
	default:
		return fmt.Sprintf("Read %d %s", n, noun("file", "files"))
	}
}

func lineFoldable(ln line, width int) bool {
	switch ln.kind {
	case "user":
		return visualRows(wrapText(sanitize(ln.text), max(8, width-2))) > 3
	case "assistant", "error":
		return visualRows(wrapText(sanitize(ln.text), max(8, width-blockPad))) > 6
	}
	return false
}

func visualRows(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func toolSummary(name, args string) string {
	var raw map[string]any
	if json.Unmarshal([]byte(args), &raw) != nil {
		return sanitize(args)
	}
	str := func(k string) string {
		s, _ := raw[k].(string)
		return sanitize(s)
	}
	switch name {
	case "read_file", "edit_file", "write_file":
		return str("path")
	case "shell":
		return "$ " + str("command")
	case "grep", "glob":
		return str("pattern")
	case "web_fetch":
		return str("url")
	case "update_plan":
		return "plan.md"
	case "ask_jev":
		return askJevSummary(raw)
	case "spawn_subagent":
		return strings.TrimSpace(str("kind") + "  " + str("prompt"))
	}
	return sanitize(args)
}

func askJevSummary(raw map[string]any) string {
	if list, ok := raw["questions"].([]any); ok && len(list) > 0 {
		m, _ := list[0].(map[string]any)
		mode, _ := m["mode"].(string)
		q, _ := m["question"].(string)
		sum := strings.TrimSpace(sanitize(mode) + "  " + sanitize(q))
		if len(list) > 1 {
			return fmt.Sprintf("%s  (+%d)", sum, len(list)-1)
		}
		return sum
	}
	mode, _ := raw["mode"].(string)
	q, _ := raw["question"].(string)
	return strings.TrimSpace(sanitize(mode) + "  " + sanitize(q))
}

func resultSummary(text string) string {
	var rows []string
	for _, l := range strings.Split(text, "\n") {
		if strings.TrimSpace(l) != "" {
			rows = append(rows, l)
		}
	}
	if len(rows) == 0 {
		return "(no output)"
	}
	if len(rows) == 1 {
		return rows[0]
	}
	return fmt.Sprintf("%s  (+%d lines)", rows[0], len(rows)-1)
}

// markdown renders assistant text with the Rock Glamour style at width.
// Renderers are cached per width and dropped when the theme changes.
func (m *Model) markdown(src string, width int) string {
	r := m.md[width]
	if r == nil {
		var err error
		r, err = glamour.NewTermRenderer(
			glamour.WithStyles(m.th.markdown()),
			glamour.WithWordWrap(width),
			glamour.WithChromaFormatter("terminal16m"),
		)
		if err != nil {
			return m.th.plain.Render(wrapText(src, width))
		}
		m.md[width] = r
	}
	out, err := r.Render(src)
	if err != nil {
		return m.th.plain.Render(wrapText(src, width))
	}
	lines := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
	for len(lines) > 0 && strings.TrimSpace(ansi.Strip(lines[0])) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(ansi.Strip(lines[len(lines)-1])) == "" {
		lines = lines[:len(lines)-1]
	}
	var fitted []string
	for _, l := range lines {
		if ansi.StringWidth(l) > width {
			fitted = append(fitted, strings.Split(ansi.Hardwrap(l, width, true), "\n")...)
			continue
		}
		fitted = append(fitted, l)
	}
	return strings.Join(fitted, "\n")
}

type rowDelegate struct {
	th   *theme
	rows int
}

func (d rowDelegate) Height() int                         { return d.rows }
func (d rowDelegate) Spacing() int                        { return 0 }
func (d rowDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d rowDelegate) Render(w io.Writer, l list.Model, index int, item list.Item) {
	it, ok := item.(rowItem)
	if !ok {
		return
	}
	t := d.th
	width := max(1, l.Width())
	selected := index == l.Index()
	mark, titleStyle := "  ", t.plain
	if selected {
		mark, titleStyle = t.accent.Render("▸ "), t.strong
	}
	switch {
	case d.rows == 2:
		note := ""
		if it.tag == "current" {
			note = t.accent.Render("  ● this session")
		}
		title := titleStyle.Render(clip(it.title, max(1, width-2-ansi.StringWidth(note))))
		fmt.Fprint(w, mark+title+note+"\n  "+t.faint.Render(clip(it.desc, max(1, width-2))))
	case it.tag == "choice":
		st := tagStyle(t, it.id).Bold(selected)
		fmt.Fprint(w, ansi.Truncate(mark+st.Render(padRight(it.title, 7))+t.faint.Render(it.desc), width, "…"))
	default:
		tag := tagStyle(t, it.tag).Render(padRight(it.tag, 6))
		title := titleStyle.Render(padRight(clip(it.title, 24), 24))
		fmt.Fprint(w, ansi.Truncate(mark+tag+" "+title+" "+t.faint.Render(it.desc), width, "…"))
	}
}

func tagStyle(t *theme, tag string) lipgloss.Style {
	switch tag {
	case "allow":
		return t.chrome
	case "deny":
		return t.danger
	case "mode":
		return t.strong
	default:
		return t.strong
	}
}

// sanitize drops escape sequences and control characters from text that
// came from a model, a tool, or the store, so it cannot drive the terminal.
func sanitize(s string) string {
	s = ansi.Strip(s)
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\t':
			return r
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0):
			return -1
		}
		return r
	}, s)
}

// clip flattens s to one line and cuts it to n cells with an ellipsis. It
// measures and cuts whole graphemes, so a rune is never split.
func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if n <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= n {
		return s
	}
	return ansi.Truncate(s, n, "…")
}

// clipLeft keeps the tail of s, which is the useful end of a path.
func clipLeft(s string, n int) string {
	w := ansi.StringWidth(s)
	switch {
	case w <= n:
		return s
	case n <= 0:
		return ""
	case n == 1:
		return "…"
	}
	return ansi.TruncateLeft(s, w-n+1, "…")
}

// wrapText wraps plain text to w cells on word boundaries and breaks words
// that do not fit. It works on graphemes, so it never splits a rune.
func wrapText(s string, w int) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\t", "    ")
	if w < 1 {
		return s
	}
	return ansi.Wrap(s, w, "")
}

func padRight(s string, w int) string {
	if n := w - ansi.StringWidth(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

func padBlock(s string, w int) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = padRight(lines[i], w)
	}
	return strings.Join(lines, "\n")
}

// block forces s to exactly h rows of at most w cells.
func block(s string, w, h int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for i, l := range lines {
		if ansi.StringWidth(l) > w {
			lines[i] = ansi.Truncate(l, w, "")
		}
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// boxed draws st around content at exactly w by h cells. Lip Gloss v2 counts
// border and padding inside Width and Height, so the content is cut to the
// inner size first.
func boxed(st lipgloss.Style, w, h int, content string) string {
	innerW := max(1, w-st.GetHorizontalFrameSize())
	innerH := max(1, h-st.GetVerticalFrameSize())
	return st.Width(w).Height(h).Render(padBlock(block(content, innerW, innerH), innerW))
}

func joinColumns(left, gap, right string) string {
	l, r := strings.Split(left, "\n"), strings.Split(right, "\n")
	lw := 0
	for _, s := range l {
		lw = max(lw, ansi.StringWidth(s))
	}
	n := max(len(l), len(r))
	out := make([]string, n)
	for i := range out {
		a, b := "", ""
		if i < len(l) {
			a = l[i]
		}
		if i < len(r) {
			b = r[i]
		}
		out[i] = padRight(a, lw) + gap + b
	}
	return strings.Join(out, "\n")
}

func shortPath(p string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if p == home {
			return "~"
		}
		if rel, ok := strings.CutPrefix(p, home+string(filepath.Separator)); ok {
			return "~" + string(filepath.Separator) + rel
		}
	}
	return p
}

func mix(a, b color.Color, f float64) color.Color {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	lerp := func(x, y uint32) uint8 { return uint8((float64(x>>8)*(1-f) + float64(y>>8)*f) + 0.5) }
	return color.RGBA{R: lerp(ar, br), G: lerp(ag, bg), B: lerp(ab, bb), A: 0xff}
}
