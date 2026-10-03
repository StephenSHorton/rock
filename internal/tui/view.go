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
// and the status line, composer, and help line always stay on screen.
func (m *Model) layout() {
	w, h := max(m.width, 20), max(m.height, 6)
	g := geometry{w: w, h: h, bodyY: 1, composerRows: 3, helpRows: 1}
	chrome := func() int { return 1 + 1 + g.composerRows + 2 + g.helpRows }
	for g.composerRows > 1 && h-chrome() < 4 {
		g.composerRows--
	}
	if h-chrome() < 1 {
		g.helpRows = 0
	}
	g.bodyH = max(1, h-chrome())

	g.transcriptW = w
	switch {
	case (m.showPlan || w >= wideAt) && w >= splitMin:
		g.planW = min(max(w*32/100, 28), 46)
		g.transcriptW = w - g.planW - 1
	case m.showPlan:
		innerW := max(1, w-4)
		m.planContent(innerW)
		want := 2 + lipgloss.Height(m.planHeader(innerW, true)) + m.planVP.TotalLineCount()
		g.planH = min(max(want, 5), max(5, g.bodyH/2))
	}
	if m.pending != nil {
		g.askH = min(m.askHeight(w), g.bodyH)
	}
	if g.planH > 0 && g.bodyH-g.askH-g.planH < 3 {
		g.planH = 0
	}
	g.vpY = g.bodyY + g.planH
	g.vpH = max(0, g.bodyH-g.planH-g.askH)
	m.geo = g

	m.input.SetWidth(max(1, w-4))
	m.input.SetHeight(g.composerRows)
	m.help.SetWidth(max(1, w-1))
	m.meter.SetWidth(meterWidth(w))

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
		innerW := max(1, pw-4)
		m.planContent(innerW)
		m.planVP.SetWidth(innerW)
		m.planVP.SetHeight(max(0, ph-2-lipgloss.Height(m.planHeader(innerW, stacked))))
	}

	innerW, innerH := max(1, w-4), max(1, g.bodyH-2)
	listH := max(1, innerH-2)
	m.sessions.SetSize(innerW, listH)
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
		return g.w, g.planH, true
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
	parts := []string{
		block(m.headerView(), g.w, 1),
		block(m.bodyView(), g.w, g.bodyH),
		block(m.statusView(), g.w, 1),
		block(m.composerView(), g.w, g.composerRows+2),
	}
	if g.helpRows > 0 {
		parts = append(parts, block(m.helpLineView(), g.w, 1))
	}
	v := tea.NewView(strings.Join(parts, "\n"))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "rock"
	return v
}

func (m *Model) headerView() string {
	t, w := m.th, m.geo.w
	brand := " rock  "
	id, title := "", ""
	if s := m.deps.Session; s != nil {
		id, title = s.Meta.ID, s.Meta.Title
	}
	if title == "rock" || title == id {
		title = ""
	}
	cwd := shortPath(m.deps.CWD)
	avail := w - ansi.StringWidth(brand) - 1
	mid := id
	if title != "" {
		mid = title + " · " + id
	}
	if ansi.StringWidth(mid)+2+ansi.StringWidth(cwd) > avail {
		cwd = clipLeft(cwd, max(12, avail-ansi.StringWidth(mid)-2))
	}
	if ansi.StringWidth(mid)+2+ansi.StringWidth(cwd) > avail {
		mid = clip(mid, max(0, avail-2-ansi.StringWidth(cwd)))
	}
	midStyled := t.barMute.Render(mid)
	if title != "" && strings.HasPrefix(mid, title) {
		midStyled = t.barText.Render(title) + t.barMute.Render(strings.TrimPrefix(mid, title))
	}
	fill := max(0, avail-ansi.StringWidth(mid)-ansi.StringWidth(cwd))
	return t.barBrand.Render(brand) + midStyled + t.bar.Render(strings.Repeat(" ", fill)) +
		t.barMute.Render(cwd) + t.bar.Render(" ")
}

func (m *Model) bodyView() string {
	g := m.geo
	if m.overlay != noOverlay {
		return m.overlayView(g.w, g.bodyH)
	}
	var rows []string
	if g.planH > 0 {
		rows = append(rows, m.planView(g.w, g.planH, true))
	}
	if g.vpH > 0 {
		t := m.transcriptView(g.transcriptW, g.vpH)
		if g.planW > 0 {
			t = joinColumns(t, " ", m.planView(g.planW, g.vpH, false))
		}
		rows = append(rows, t)
	}
	if g.askH > 0 {
		rows = append(rows, m.askView(g.w, g.askH))
	}
	return strings.Join(rows, "\n")
}

// transcriptView is the viewport with one column of air on the left and the
// scrollbar on the right.
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
	track := lipgloss.NewStyle().Foreground(mix(m.th.ground, m.th.mute, 0.45))
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

func (m *Model) planView(w, h int, stacked bool) string {
	innerW := max(1, w-4)
	box := m.th.box
	if m.deps.Mode == perms.ModePlan {
		box = m.th.boxFocus
	}
	return boxed(box, w, h, m.planHeader(innerW, stacked)+"\n"+m.planVP.View())
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
		{"ctrl+j", "new line"},
		{"pgup/pgdn", "scroll the transcript"},
		{"ctrl+u/d", "half a page"},
		{"wheel", "scroll under the pointer"},
		{"click", "jump on the scrollbar"},
		{"ctrl+h", "this help"},
		{"esc", "close; deny a pending call"},
		{"ctrl+c", "quit"},
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
		{"/fork", "new Rock pane in Suzuri (OSC 7880)"},
		{"/quit", "quit"},
	}
	section := func(title string, rows [][2]string, width int) string {
		keyW := 0
		for _, r := range rows {
			keyW = max(keyW, ansi.StringWidth(r[0]))
		}
		out := []string{t.strong.Render(title)}
		descW := max(8, width-keyW-2)
		for _, r := range rows {
			desc := strings.Split(wrapText(r[1], descW), "\n")
			for i, d := range desc {
				k := strings.Repeat(" ", keyW)
				if i == 0 {
					k = padRight(r[0], keyW)
				}
				out = append(out, t.accent.Render(k)+"  "+t.plain.Render(d))
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
	snippet := func(prefix, text string, st lipgloss.Style, limit int) []string {
		var out []string
		ls := strings.Split(strings.TrimRight(text, "\n"), "\n")
		for i, l := range ls {
			if i == limit {
				out = append(out, t.faint.Render(fmt.Sprintf("  … %d more lines", len(ls)-limit)))
				break
			}
			out = append(out, st.Render(clip(prefix+l, w)))
		}
		return out
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
			out = append(out, snippet("- ", str("old"), t.alarm, 6)...)
			out = append(out, snippet("+ ", str("new"), t.accent, 6)...)
		}
	case "write_file":
		out = append(out, field("path", str("path")))
		if args != nil {
			out = append(out, snippet("  ", str("content"), t.faint, 6)...)
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
	t, w := m.th, m.geo.w
	mode := string(m.deps.Mode)
	badges := t.badge(mode).Render(strings.ToUpper(mode))
	if m.deps.ReviewOnly {
		badges += " " + t.badgeReview.Render("REVIEW")
	}
	jevStyle := t.faint
	if m.deps.JevMode == "live" {
		jevStyle = t.accent
	}
	jevSeg := jevStyle.Render("jev:" + m.deps.JevMode)

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
	pct := fmt.Sprintf("%3.0f%%", m.meterP*100)
	pctStyle := t.plain
	if m.meterP >= 0.8 {
		pctStyle = t.alarm
	}

	build := func(barW int, withNote bool) string {
		model := t.faint.Render("◆ ") + t.plain.Render(name)
		if withNote {
			model += t.faint.Render(note)
		}
		ctx := t.faint.Render("ctx ")
		if barW > 0 {
			meter := m.meter
			meter.SetWidth(barW)
			fill := m.meterP
			if fill > 0 && fill*float64(barW) < 1 {
				fill = 1 / float64(barW)
			}
			ctx += meter.ViewAs(fill) + " "
		}
		ctx += pctStyle.Render(pct)
		return " " + badges + "  " + jevSeg + "  " + model + "  " + ctx
	}
	left := build(m.meter.Width(), true)
	for _, try := range []struct {
		bar  int
		note bool
	}{{6, true}, {0, true}, {6, false}, {0, false}} {
		if ansi.StringWidth(left) <= w {
			break
		}
		left = build(try.bar, try.note)
	}
	left = ansi.Truncate(left, w, "…")

	status := sanitize(m.status)
	statusStyle := t.faint
	if m.alert {
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

func (m *Model) composerView() string {
	box := m.th.box
	if m.input.Focused() {
		box = m.th.boxFocus
	}
	return box.Width(m.geo.w).Render(block(m.input.View(), max(1, m.geo.w-4), m.geo.composerRows))
}

func (m *Model) helpLineView() string {
	k := m.keys
	var bindings []key.Binding
	switch {
	case m.pending != nil:
		bindings = []key.Binding{k.allow, k.deny, key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "deny")), k.move}
	case m.overlay == sessionsOverlay:
		bindings = []key.Binding{k.move, k.pick, k.close}
	case m.overlay == helpOverlay:
		bindings = []key.Binding{key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("↑/↓ pgup pgdn", "scroll")), k.close}
	case m.overlay != noOverlay:
		bindings = []key.Binding{k.move, k.close}
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
	if len(m.lines) == 0 {
		return m.th.faint.Render(wrapText(readyText, width))
	}
	var b strings.Builder
	for i := range m.lines {
		ln := &m.lines[i]
		if i > 0 && (ln.kind == "user" || ln.kind == "assistant" || ln.kind == "error") {
			b.WriteByte('\n')
		}
		if ln.out == "" || ln.outW != width || ln.outDark != m.th.dark {
			ln.out, ln.outW, ln.outDark = m.renderLine(*ln, width), width, m.th.dark
		}
		b.WriteString(ln.out)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) renderLine(ln line, width int) string {
	t := m.th
	bodyW := max(8, width-gutter)
	text := sanitize(ln.text)
	switch ln.kind {
	case "user":
		return speak(t.you.Render(padRight("you", gutter)), t.plain.Render(wrapText(text, bodyW)))
	case "assistant":
		return speak(t.rock.Render(padRight("rock", gutter)), m.markdown(text, bodyW))
	case "error":
		return speak(t.alarmBold.Render(padRight("error", gutter)), t.alarm.Render(wrapText(text, bodyW)))
	case "tool":
		return eventRow(t.accent.Render("▸ ")+t.strong.Render(ln.name), toolSummary(ln.name, ln.text), t.faint, bodyW)
	case "result":
		mark, st := t.accent.Render("✓ "), t.faint
		if strings.HasPrefix(text, "denied") {
			mark, st = t.alarm.Render("✗ "), t.alarm
		}
		return eventRow(mark+t.plain.Render(ln.name), resultSummary(text), st, bodyW)
	case "permission":
		decision, why, _ := strings.Cut(text, ":")
		st := t.accent
		if decision == string(perms.Deny) {
			st = t.alarm
		}
		return eventRow(st.Render("⚑ ")+t.plain.Render(ln.name)+"  "+st.Render(decision), strings.TrimSpace(why), t.faint, bodyW)
	case "jev":
		return eventRow(t.faint.Render("◇ jev "+ln.name), text, t.faint, bodyW)
	default:
		return strings.Repeat(" ", gutter) + t.faint.Render(clip(ln.kind+"  "+text, bodyW))
	}
}

// eventRow is one tool, permission, or Jev row under the speaker column.
func eventRow(head, detail string, st lipgloss.Style, w int) string {
	row := ansi.Truncate(head, w, "…")
	if room := w - ansi.StringWidth(row) - 2; room > 0 && detail != "" {
		row += "  " + st.Render(clip(detail, room))
	}
	return strings.Repeat(" ", gutter) + row
}

func speak(label, body string) string {
	lines := strings.Split(body, "\n")
	pad := strings.Repeat(" ", gutter)
	for i := range lines {
		if i == 0 {
			lines[i] = label + lines[i]
		} else {
			lines[i] = pad + lines[i]
		}
	}
	return strings.Join(lines, "\n")
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
	case "jev_decide":
		return str("question")
	case "spawn_subagent":
		return strings.TrimSpace(str("kind") + "  " + str("prompt"))
	}
	return sanitize(args)
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
		return t.accent
	case "deny":
		return t.alarm
	case "mode":
		return t.accentBold
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
