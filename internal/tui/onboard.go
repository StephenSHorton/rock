package tui

import (
	"context"
	"errors"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/charmbracelet/x/ansi"
)

const jevOneLiner = "Jev is Rock's System One: fast, typed checks that make every task safer and cheaper."

type gatePhase int

const (
	gateEmpty gatePhase = iota
	gateValidating
	gateError
	gateSuccess
)

type jevGate struct {
	input textinput.Model
	spin  spinner.Model
	phase gatePhase
	err   string
	store string
}

type jevGateDone struct {
	err   error
	store string
	key   string
}

type jevGateClear struct{}

func newJevGate() *jevGate {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Placeholder = "paste your Jev key"
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'
	ti.CharLimit = 256
	ti.SetWidth(48)
	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	return &jevGate{input: ti, spin: sp, phase: gateEmpty}
}

func (m *Model) gating() bool { return m.gate != nil }

func (m *Model) gateKey(msg tea.KeyPressMsg) tea.Cmd {
	if msg.String() == "esc" || msg.String() == "ctrl+c" || msg.String() == "ctrl+q" {
		return tea.Quit
	}
	if m.gate.phase == gateValidating || m.gate.phase == gateSuccess {
		return nil
	}
	if msg.String() == "enter" {
		return m.beginJevCheck()
	}
	var cmd tea.Cmd
	m.gate.input, cmd = m.gate.input.Update(msg)
	if strings.TrimSpace(m.gate.input.Value()) == "" && m.gate.phase == gateError {
		m.gate.phase = gateEmpty
		m.gate.err = ""
	}
	return cmd
}

// gatePaste inserts a pasted Jev key. Terminals deliver Ctrl+V and
// right-click as a bracketed paste, not key presses, so without this the
// masked field silently ignored pastes. Surrounding whitespace and line
// breaks are dropped; a key never contains them.
func (m *Model) gatePaste(msg tea.PasteMsg) tea.Cmd {
	if m.gate.phase == gateValidating || m.gate.phase == gateSuccess {
		return nil
	}
	text := strings.Join(strings.Fields(msg.Content), "")
	if text == "" {
		return nil
	}
	m.gate.input.SetValue(m.gate.input.Value() + text)
	m.gate.input.CursorEnd()
	if m.gate.phase == gateError {
		m.gate.phase = gateEmpty
		m.gate.err = ""
	}
	return nil
}

func (m *Model) beginJevCheck() tea.Cmd {
	key := strings.TrimSpace(m.gate.input.Value())
	if key == "" {
		m.gate.phase = gateError
		m.gate.err = "Paste a Jev key, then press enter."
		return nil
	}
	m.gate.phase = gateValidating
	m.gate.err = ""
	check := m.deps.CheckJev
	save := m.deps.SaveJev
	return tea.Batch(m.gate.spin.Tick, func() tea.Msg {
		if check == nil {
			return jevGateDone{err: errors.New("Jev check is not wired"), key: key}
		}
		if err := check(context.Background(), key); err != nil {
			return jevGateDone{err: err, key: key}
		}
		if save == nil {
			return jevGateDone{store: "memory", key: key}
		}
		store, err := save(key)
		return jevGateDone{err: err, store: store, key: key}
	})
}

func (m *Model) finishJevGate(msg jevGateDone) tea.Cmd {
	if m.gate == nil {
		return nil
	}
	if msg.err != nil {
		m.gate.phase = gateError
		m.gate.err = "Jev rejected this key. " + strings.TrimSpace(msg.err.Error()) + " Enter retries."
		return nil
	}
	m.gate.phase = gateSuccess
	m.gate.store = msg.store
	m.deps.JevMode = "live"
	if m.deps.Gates.Client != nil {
		m.deps.Gates.Client.APIKey = msg.key
	}
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return jevGateClear{} })
}

func (m *Model) clearJevGate() tea.Cmd {
	m.gate = nil
	m.status = "ready"
	cmds := []tea.Cmd{m.input.Focus(), m.retarget()}
	if m.avail.Newer && !m.updateDismissed {
		m.openOverlay(updateOverlay)
	}
	if prompt := strings.TrimSpace(m.deps.InitialPrompt); prompt != "" {
		m.deps.InitialPrompt = ""
		m.lines = append(m.lines, line{kind: "user", text: prompt})
		cmds = append(cmds, m.start(prompt))
	}
	return tea.Batch(cmds...)
}

func (m *Model) onboardView() tea.View {
	t := m.th
	w, h := max(m.width, 20), max(m.height, 8)
	title := t.accentBold.Render("Rock")
	blurb := t.plain.Render(wrapText(jevOneLiner, min(64, w-8)))
	label := t.chrome.Render("Jev API key")
	field := m.gate.input.View()
	if m.gate.phase == gateValidating {
		field = t.faint.Render(m.gate.spin.View() + "  Checking this key with Jev…")
	}
	status := t.faint.Render("enter validate · esc quit")
	switch m.gate.phase {
	case gateError:
		status = t.alarm.Render(wrapText(m.gate.err, min(64, w-8)))
	case gateSuccess:
		where := m.gate.store
		if where == "" {
			where = "the key store"
		}
		status = t.plain.Render("Saved in " + where + ".")
	case gateValidating:
		status = t.faint.Render("Talking to Jev. The agent does not start until this works.")
	}
	innerW := min(64, max(36, w-10))
	body := strings.Join([]string{title, "", blurb, "", label, field, "", status}, "\n")
	box := boxed(t.boxFocus, innerW+4, lipgloss.Height(body)+2, body)
	pad := max(0, (h-lipgloss.Height(box))/2)
	left := max(0, (w-lipgloss.Width(box))/2)
	var lines []string
	for i := 0; i < pad; i++ {
		lines = append(lines, "")
	}
	for _, row := range strings.Split(box, "\n") {
		lines = append(lines, strings.Repeat(" ", left)+row)
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	for i, row := range lines {
		if ansi.StringWidth(row) < w {
			lines[i] = padRight(row, w)
		}
	}
	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	v.WindowTitle = "rock"
	return v
}
