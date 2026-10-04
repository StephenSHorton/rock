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
	if m.gating() {
		return m.onboardView()
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
