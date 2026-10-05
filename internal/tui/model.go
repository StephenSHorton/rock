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
	"path/filepath"
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
	"github.com/StephenSHorton/rock/internal/update"
	"github.com/StephenSHorton/rock/internal/version"
)

const (
	// contextLimit is the session size at which the offline Jev policy
	// compacts, so a full meter means compaction is next.
	contextLimit = 24000
	// splitMin is the narrowest width that still puts the plan beside the
	// transcript. Narrower screens stack the plan above it when the pane
	// is showing.
	splitMin = 64
	// blockPad is the left inset of transcript blocks, Grok's
	// scrollback.layout.block_pad_left default.
	blockPad = 2
	// compactAt is the height at which outer padding drops and the
	// composer shrinks, matching Grok's auto-compact cut.
	compactAt = 20
	// maxComposerRows caps multi-line paste / shift+enter growth.
	maxComposerRows = 8
	// shortAt is the height that also drops outer padding so the
	// transcript keeps a floor.
	shortAt = 16
	fps     = 30

	readyText   = "Ask Rock to start."
	placeholder = "Ask Rock"
	slashHint   = "/ for commands"
)

// RunFunc executes one turn against the session the screen is showing.
type RunFunc func(ctx context.Context, sess *session.Session, prompt string, ask harness.AskFunc, sink func(harness.Event)) error

// GrokModel is one subscription model from the official grok child.
type GrokModel struct {
	ID, Name      string
	ContextTokens int
}

// Deps is everything the screen needs from the process.
type Deps struct {
	CWD          string
	Session      *session.Session
	Mode         perms.Mode
	ReviewOnly   bool
	JevMode      string
	Gates        jev.Gates
	Provider     string
	Auth         string
	FastModel    string
	StrongModel  string
	ContextLimit int // bytes; 0 = use default contextLimit
	Run          RunFunc
	LoadSession  func(id string) (*session.Session, error)
	ListSessions func() []session.Meta
	Rules        []string
	Fork         func(prompt string) (string, error)
	SetMode      func(perms.Mode)
	SetAuth      func(class string) (provider, auth string, err error)
	HasAPIKey    bool
	HasSIWC      bool
	HasGrokCLI   bool
	// ListGrokModels / SetGrokModel are set when auth is grok-cli so the
	// model chip can show subscription models from the child ACP session.
	ListGrokModels func() (current string, models []GrokModel, err error)
	SetGrokModel   func(id string) error
	InitialPrompt  string
	Output         io.Writer
	// Verbose starts the TUI with Jev turn/risk diagnostics in the
	// transcript. /verbose toggles it after that.
	Verbose bool
	// JevGate blocks the agent until CheckJev succeeds and SaveJev stores
	// the key. Existing view tests leave this false.
	JevGate  bool
	CheckJev func(ctx context.Context, key string) error
	SaveJev  func(key string) (store string, err error)
	// UpdateCheck starts the 24h background release check. Tests leave this false.
	UpdateCheck bool
	// Log writes TUI lifecycle lines (size, turn start/finish/error) to
	// rock.log. Nil in tests that do not care.
	Log LogFunc
}

type line struct {
	kind string
	name string
	text string
	at   time.Time

	result   string
	done     bool
	finished time.Time

	folded      bool
	foldTouched bool
	raw         bool

	out     string
	outW    int
	outDark bool
}

// entrySpan maps a painted transcript range back to one or more log lines.
type entrySpan struct {
	from, to int
	y0, y1   int
	group    bool
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
	paletteOverlay
	providerOverlay
	updateOverlay
)

type updateNoticeMsg struct {
	notice update.Notice
	err    error
	asked  bool
}

type updateDoneMsg struct {
	ver string
	err error
}

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
	at  time.Time // turn start, for the rock.log duration
}

type tickMsg time.Time
type forkNote struct{ text string }

type keyMap struct {
	submit   key.Binding
	newline  key.Binding
	help     key.Binding
	quit     key.Binding
	scroll   key.Binding
	up       key.Binding
	down     key.Binding
	allow    key.Binding
	deny     key.Binding
	close    key.Binding
	move     key.Binding
	pick     key.Binding
	focus    key.Binding
	cycle    key.Binding
	palette  key.Binding
	sessions key.Binding
	yolo     key.Binding
	fold     key.Binding
	back     key.Binding
}

func newKeys() keyMap {
	return keyMap{
		submit:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "send")),
		newline:  key.NewBinding(key.WithKeys("shift+enter", "ctrl+j", "alt+enter"), key.WithHelp("ctrl+j", "new line")),
		help:     key.NewBinding(key.WithKeys("ctrl+h", "ctrl+.", "ctrl+x"), key.WithHelp("ctrl+.", "help")),
		quit:     key.NewBinding(key.WithKeys("ctrl+c", "ctrl+q"), key.WithHelp("ctrl+c", "quit")),
		scroll:   key.NewBinding(key.WithKeys("pgup", "pgdown"), key.WithHelp("pgup/pgdn", "scroll")),
		up:       key.NewBinding(key.WithKeys("pgup", "ctrl+u"), key.WithHelp("pgup", "scroll up")),
		down:     key.NewBinding(key.WithKeys("pgdown", "ctrl+d"), key.WithHelp("pgdn", "scroll down")),
		allow:    key.NewBinding(key.WithKeys("y", "a"), key.WithHelp("y", "allow")),
		deny:     key.NewBinding(key.WithKeys("n", "d"), key.WithHelp("n", "deny")),
		close:    key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc", "close")),
		move:     key.NewBinding(key.WithKeys("up", "down", "k", "j"), key.WithHelp("↑/↓", "move")),
		pick:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "resume")),
		focus:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "focus")),
		cycle:    key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "cycle mode")),
		palette:  key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "palette")),
		sessions: key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "sessions")),
		yolo:     key.NewBinding(key.WithKeys("ctrl+o"), key.WithHelp("ctrl+o", "yolo")),
		fold:     key.NewBinding(key.WithKeys("left", "right"), key.WithHelp("←/→", "fold")),
		back:     key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "composer")),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.submit, k.focus, k.palette, k.help, k.quit}
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
	padT, padB   int
	padL, padR   int
	innerW       int
	bodyY, bodyH int
	composerRows int
	frameRows    int
	noticeRows   int
	infoRows     int
	helpRows     int
	transcriptW  int
	planW        int
	planH        int
	askH         int
	vpY, vpH     int
	barX         int
	compact      bool
}

func (g geometry) noticeY() int { return g.bodyY + g.bodyH }
func (g geometry) composerY() int {
	return g.noticeY() + g.noticeRows
}
func (g geometry) statusY() int {
	return g.composerY() + g.composerRows + g.frameRows + g.infoRows
}
func (g geometry) contentLeft() int { return g.padL }

// Model is the Bubble Tea program.
type Model struct {
	deps Deps
	keys keyMap
	th   theme

	width  int
	height int
	geo    geometry

	lines     []line
	selected  int
	spans     []entrySpan
	vp        viewport.Model
	planVP    viewport.Model
	sheet     viewport.Model
	input     textarea.Model
	spin      spinner.Model
	help      help.Model
	sessions  list.Model
	choices   list.Model
	perms     list.Model
	providers list.Model
	agents    table.Model
	meter     progress.Model

	overlay         overlay
	showPlan        bool
	verbose         bool
	focusTranscript bool
	quitArmed       time.Time
	gate            *jevGate

	palette         list.Model
	picker          picker
	chips           []chipHit
	chipY           int
	slashSel        int
	slashHide       bool
	avail           update.Notice
	updateDismissed bool
	updating        bool

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
	turnAt   time.Time
	// turnFrom is len(lines) when the turn started; turnDone uses it to
	// make sure every turn leaves something visible.
	turnFrom   int
	userCancel bool
	seen       *sizeSeen
	sizeLogAt  time.Time
	resizes    int

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
		deps:     deps,
		seen:     &sizeSeen{},
		keys:     newKeys(),
		follow:   true,
		status:   "ready",
		selected: -1,
		verbose:  deps.Verbose,
		showPlan: deps.Mode == perms.ModePlan,
		spring:   harmonica.NewSpring(harmonica.FPS(fps), 7, 1),
		md:       map[int]*glamour.TermRenderer{},
	}

	ta := textarea.New()
	ta.Placeholder = placeholder
	ta.ShowLineNumbers = false
	ta.CharLimit = 8000
	ta.SetPromptFunc(2, func(info textarea.PromptInfo) string {
		if info.LineNumber == 0 {
			return "❯ "
		}
		return "  "
	})
	ta.KeyMap.InsertNewline = m.keys.newline
	ta.DynamicHeight = true
	ta.MinHeight = 1
	ta.MaxHeight = maxComposerRows
	ta.SetHeight(1)
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
	m.providers = newList(rowDelegate{th: &m.th, rows: 2})
	m.palette = newList(rowDelegate{th: &m.th, rows: 1})
	m.palette.SetFilteringEnabled(true)
	m.palette.SetShowFilter(true)
	m.agents = table.New(
		table.WithColumns(agentColumns(60)),
		table.WithHeight(6),
		table.WithFocused(true),
	)
	m.meter = progress.New(progress.WithWidth(16))

	if deps.JevGate {
		m.gate = newJevGate()
	}
	m.applyTheme(true)
	m.seedTranscript()
	m.refreshPerms()
	m.refreshPlan()
	if n := forceUpdateNotice(); n.Newer {
		m.avail = n
	}
	return m
}

// forceUpdateNotice is the visual / test hook. ROCK_TEST_UPDATE_NOTICE=1.0.2
// paints a muted line above the composer without opening the update overlay
// and without hitting GitHub.
func forceUpdateNotice() update.Notice {
	v := strings.TrimSpace(os.Getenv("ROCK_TEST_UPDATE_NOTICE"))
	if v == "" {
		return update.Notice{}
	}
	return update.Notice{Current: version.Version, Latest: v, Newer: true}
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
	for _, l := range []*list.Model{&m.sessions, &m.choices, &m.perms, &m.providers, &m.palette} {
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
	m.invalidatePaint()
	if m.gate != nil {
		m.gate.spin.Style = m.th.accent
		m.gate.input.SetStyles(m.th.jevField())
	}
}

// Send is set by Run before the program starts so the harness can post events.
func (m *Model) Send(fn func(tea.Msg)) { m.send = fn }

func Run(m *Model) error {
	p := tea.NewProgram(m)
	m.send = p.Send
	stop := startResizePoll(m.seen, p.Send)
	defer stop()
	m.log("info", "tui open", "auth", m.deps.Auth, "provider", m.deps.Provider, "model", m.deps.FastModel, "poll_resize", resizePollOn())
	_, err := p.Run()
	if err != nil {
		m.log("error", "tui exit", "err", oneLineErr(err))
	} else {
		m.log("info", "tui exit")
	}
	return err
}

func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tea.RequestBackgroundColor}
	if m.deps.UpdateCheck {
		if n, ok := update.CachedNotice(version.Version); ok {
			if n.Newer {
				m.avail = n
				if !m.gating() {
					m.overlay = updateOverlay
					m.input.Blur()
				}
			}
		} else {
			cmds = append(cmds, m.pollUpdate(false))
		}
	}
	if m.gating() {
		return tea.Batch(append(cmds, m.gate.input.Focus(), m.gate.spin.Tick)...)
	}
	cmds = append(cmds, m.input.Focus(), m.retarget())
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
		// Always applied, whatever state the UI is in (gate, overlay,
		// picker, busy). Update re-runs layout() after this.
		m.noteSize(msg.Width, msg.Height)
		m.width, m.height = msg.Width, msg.Height
		return nil
	case tea.BackgroundColorMsg:
		if msg.IsDark() != m.th.dark {
			m.applyTheme(msg.IsDark())
		}
		return nil
	case jevGateDone:
		return m.finishJevGate(msg)
	case jevGateClear:
		return m.clearJevGate()
	case tickMsg:
		return m.stepMeter()
	case spinner.TickMsg:
		if m.gating() {
			var cmd tea.Cmd
			m.gate.spin, cmd = m.gate.spin.Update(msg)
			return cmd
		}
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
		m.turnAt = time.Time{}
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		var dur time.Duration
		if !msg.at.IsZero() {
			dur = time.Since(msg.at).Round(time.Millisecond)
		}
		userCancel := m.userCancel
		m.userCancel = false
		switch {
		case msg.err != nil && errors.Is(msg.err, context.Canceled) && userCancel:
			m.log("info", "tui turn cancelled", "dur", dur)
		case msg.err != nil:
			m.log("error", "tui turn error", "dur", dur, "err", oneLineErr(msg.err))
			m.setAlert(msg.err.Error())
			m.lines = append(m.lines, line{kind: "error", text: msg.err.Error()})
			m.syncView()
		case m.turnFrom <= len(m.lines) && m.visibleSince(m.turnFrom) == 0:
			m.log("warn", "tui turn empty", "dur", dur)
			m.setAlert("empty reply")
			m.lines = append(m.lines, line{kind: "error", text: m.emptyReplyText()})
			m.syncView()
		default:
			m.log("info", "tui turn done", "dur", dur, "shown", m.visibleSince(m.turnFrom))
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
	case updateNoticeMsg:
		return m.onUpdateNotice(msg)
	case updateDoneMsg:
		return m.onUpdateDone(msg)
	case tea.MouseMsg:
		m.onMouse(msg)
		return nil
	case tea.KeyPressMsg:
		return m.onKey(msg)
	case tea.PasteMsg:
		if m.gating() {
			return m.gatePaste(msg)
		}
	}
	if m.gating() || m.pending != nil || m.overlay != noOverlay {
		return nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

func (m *Model) onKey(msg tea.KeyPressMsg) tea.Cmd {
	if m.gating() {
		return m.gateKey(msg)
	}
	if key.Matches(msg, m.keys.quit) {
		return m.quitKey()
	}
	if m.pending != nil {
		return m.askKey(msg)
	}
	if m.overlay != noOverlay {
		return m.overlayKey(msg)
	}
	m.syncPicker()
	if m.picker.open() {
		return m.pickerKey(msg)
	}
	switch {
	case key.Matches(msg, m.keys.help):
		m.openOverlay(helpOverlay)
		return nil
	case key.Matches(msg, m.keys.palette):
		return m.openPalette()
	case key.Matches(msg, m.keys.sessions):
		m.openSessionsPicker()
		return nil
	case key.Matches(msg, m.keys.yolo):
		return m.toggleYolo()
	case key.Matches(msg, m.keys.cycle):
		return m.cycleMode()
	case key.Matches(msg, m.keys.focus):
		if m.slashVisible() {
			m.slashComplete()
			return nil
		}
		return m.toggleFocus()
	case msg.String() == "esc" && m.busy:
		return m.cancelTurn()
	case key.Matches(msg, m.keys.up), key.Matches(msg, m.keys.down):
		m.scrollKey(msg)
		return nil
	case m.focusTranscript:
		return m.scrollbackKey(msg)
	case msg.String() == "?" && strings.TrimSpace(m.input.Value()) == "":
		m.openOverlay(helpOverlay)
		return nil
	case key.Matches(msg, m.keys.submit):
		return m.submit()
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.syncPicker()
	return cmd
}

func (m *Model) scrollbackKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, m.keys.back), msg.String() == "space":
		return m.focusComposer()
	case key.Matches(msg, m.keys.fold), msg.String() == "left", msg.String() == "right":
		if m.foldableAt(m.selected) {
			m.toggleFold(m.selected)
		}
		return nil
	case msg.String() == "up":
		m.moveSelect(-1)
		return nil
	case msg.String() == "down":
		m.moveSelect(1)
		return nil
	case msg.String() == "r":
		m.toggleRaw()
		return nil
	case msg.String() == "v":
		return m.toggleVerbose()
	case msg.String() == "?":
		return m.openPalette()
	case msg.String() == "/":
		m.input.SetValue("/")
		return m.focusComposer()
	}
	return nil
}

func (m *Model) toggleFocus() tea.Cmd {
	if m.focusTranscript {
		return m.focusComposer()
	}
	m.focusTranscript = true
	m.input.Blur()
	if m.selected < 0 && len(m.spans) > 0 {
		m.setSelected(m.spans[len(m.spans)-1].from)
	}
	return nil
}

func (m *Model) focusComposer() tea.Cmd {
	m.focusTranscript = false
	return m.input.Focus()
}

func (m *Model) setSelected(from int) {
	if from < 0 || from >= len(m.lines) || m.selected == from {
		return
	}
	m.selected = from
	m.invalidatePaint()
	m.syncView()
}

func (m *Model) moveSelect(delta int) {
	if len(m.spans) == 0 {
		return
	}
	idx := 0
	for i, s := range m.spans {
		if m.selected >= s.from && m.selected <= s.to {
			idx = i
			break
		}
	}
	idx = min(len(m.spans)-1, max(0, idx+delta))
	m.setSelected(m.spans[idx].from)
}

func (m *Model) toggleRaw() {
	if m.selected < 0 || m.selected >= len(m.lines) {
		return
	}
	if m.lines[m.selected].kind != "assistant" {
		return
	}
	m.lines[m.selected].raw = !m.lines[m.selected].raw
	m.lines[m.selected].out = ""
	m.syncView()
}

func (m *Model) cycleMode() tea.Cmd {
	switch m.deps.Mode {
	case perms.ModeDefault:
		m.input.SetValue("/plan")
	case perms.ModePlan:
		m.input.SetValue("/yolo")
	default:
		m.input.SetValue("/default")
	}
	return m.submit()
}

func (m *Model) toggleYolo() tea.Cmd {
	if m.deps.Mode == perms.ModeYolo {
		m.input.SetValue("/default")
	} else {
		m.input.SetValue("/yolo")
	}
	return m.submit()
}

func (m *Model) toggleVerbose() tea.Cmd {
	m.verbose = !m.verbose
	m.alert = false
	if m.verbose {
		m.status = "verbose. Jev turn and risk lines are in the transcript."
	} else {
		m.status = "quiet. Jev diagnostics are hidden. /verbose shows them."
	}
	m.syncView()
	return nil
}

// wantPlan is whether the plan pane should take space: plan mode, or a
// plan file that already has text.
func (m *Model) wantPlan() bool {
	return m.showPlan || m.hasPlan()
}

func (m *Model) hasPlan() bool {
	p := strings.TrimSpace(m.plan)
	return p != "" && p != "No plan yet."
}

// diagnosticJev is a harness gate trace (BeforeTurn / Risk). Those rows
// stay off the transcript until /verbose. Other ◇ jev marks still paint.
func diagnosticJev(ln line) bool {
	if ln.kind != "jev" {
		return false
	}
	switch ln.name {
	case "turn", "risk", "kind":
		return true
	}
	return false
}

func (m *Model) cancelTurn() tea.Cmd {
	if m.cancel != nil {
		m.userCancel = true
		m.cancel()
	}
	m.status, m.alert = "cancelled. the draft stays in the composer", false
	return nil
}

func (m *Model) quitKey() tea.Cmd {
	if m.pending != nil {
		m.answer(perms.Deny)
		m.quitArmed = time.Now()
		m.status, m.alert = "denied. press ctrl+c again to quit", false
		return m.input.Focus()
	}
	if m.busy {
		return m.cancelTurn()
	}
	if m.overlay != noOverlay {
		return m.closeOverlay()
	}
	if strings.TrimSpace(m.input.Value()) != "" {
		m.input.Reset()
		m.quitArmed = time.Now()
		m.status, m.alert = "cleared. press ctrl+c again to quit", false
		return nil
	}
	if !m.quitArmed.IsZero() && time.Since(m.quitArmed) < 2*time.Second {
		return tea.Quit
	}
	m.quitArmed = time.Now()
	m.status, m.alert = "press ctrl+c again to quit", false
	return nil
}

func (m *Model) openPalette() tea.Cmd {
	m.refreshPalette()
	m.openOverlay(paletteOverlay)
	return nil
}

func slashCommands() []rowItem {
	return []rowItem{
		{title: "/help", desc: "keys and slash commands", id: "cmd:/help"},
		{title: "/plan", desc: "plan mode; edits and shell blocked", id: "cmd:/plan"},
		{title: "/yolo", desc: "skip asks; destructive gate stays", id: "cmd:/yolo"},
		{title: "/default", desc: "back to the default policy", id: "cmd:/default"},
		{title: "/sessions", desc: "resume a session from this folder", id: "cmd:/sessions"},
		{title: "/permissions", desc: "allow, ask, and deny rules", id: "cmd:/permissions"},
		{title: "/agents", desc: "subagents spawned this session", id: "cmd:/agents"},
		{title: "/ready", desc: "is the plan ready? never approves", id: "cmd:/ready"},
		{title: "/verbose", desc: "show or hide Jev turn/risk diagnostics", id: "cmd:/verbose"},
		{title: "/fork", desc: "new Rock pane in Suzuri (OSC 7880)", id: "cmd:/fork"},
		{title: "/provider", desc: "ChatGPT, SuperGrok, API key, or offline model", id: "cmd:/provider"},
		{title: "/update", desc: "install the latest Rock release", id: "cmd:/update"},
		{title: "/quit", desc: "quit", id: "cmd:/quit"},
	}
}

func (m *Model) refreshPalette() {
	items := make([]list.Item, 0, 20)
	for _, it := range slashCommands() {
		items = append(items, it)
	}
	items = append(items,
		rowItem{title: "tab", desc: "focus composer ↔ transcript", id: "key:focus"},
		rowItem{title: "shift+tab", desc: "cycle default / plan / yolo", id: "key:cycle"},
		rowItem{title: "ctrl+s", desc: "sessions", id: "key:sessions"},
		rowItem{title: "ctrl+o", desc: "toggle yolo", id: "key:yolo"},
		rowItem{title: "v", desc: "toggle Jev diagnostics (transcript focus)", id: "key:verbose"},
		rowItem{title: "ctrl+.", desc: "help", id: "key:help"},
	)
	_ = m.palette.SetItems(items)
	m.palette.Select(0)
}

func (m *Model) slashVisible() bool {
	if m.slashHide || m.focusTranscript || m.overlay != noOverlay || m.pending != nil {
		return false
	}
	if m.picker.open() && m.picker.kind != slashPicker && m.picker.kind != filesPicker {
		return false
	}
	v := m.input.Value()
	return strings.HasPrefix(v, "/") && !strings.ContainsAny(v, " \t\n")
}

func (m *Model) slashMatches() []rowItem {
	q := strings.ToLower(strings.TrimPrefix(m.input.Value(), "/"))
	all := slashCommands()
	if q == "" {
		return all
	}
	var prefix, rest []rowItem
	for _, it := range all {
		name := strings.ToLower(strings.TrimPrefix(it.title, "/"))
		switch {
		case strings.HasPrefix(name, q):
			prefix = append(prefix, it)
		case len(q) >= 2 && strings.Contains(name, q):
			rest = append(rest, it)
		}
	}
	return append(prefix, rest...)
}

func (m *Model) syncSlash() {
	m.syncPicker()
}

func (m *Model) slashMove(delta int) {
	n := len(m.slashMatches())
	if n == 0 {
		return
	}
	m.slashSel = (m.slashSel + delta%n + n) % n
	if m.picker.kind == slashPicker {
		m.picker.sel = m.slashSel
	}
}

func (m *Model) slashComplete() {
	items := m.slashMatches()
	if m.slashSel < 0 || m.slashSel >= len(items) {
		return
	}
	m.input.SetValue(items[m.slashSel].title)
	m.slashHide = true
	m.picker.close()
}

func (m *Model) syncPicker() {
	if m.overlay != noOverlay || m.pending != nil || m.focusTranscript {
		if m.picker.kind == slashPicker || m.picker.kind == filesPicker {
			m.picker.close()
		}
		return
	}
	v := m.input.Value()
	if !strings.HasPrefix(v, "/") {
		m.slashHide = false
	}
	if _, query, ok := atToken(v); ok && !m.slashHide {
		if m.picker.kind != filesPicker {
			m.openFilesPicker(query)
			return
		}
		m.picker.items = listWorkspaceFiles(m.deps.CWD, query, 40)
		m.picker.filter = query
		m.picker.clamp()
		return
	}
	if m.slashVisible() {
		items := m.slashMatches()
		if m.picker.kind != slashPicker {
			m.picker.set(slashPicker, "", items)
			m.picker.sel = m.slashSel
			m.picker.clamp()
			m.slashSel = m.picker.sel
			return
		}
		m.picker.items = items
		m.picker.sel = m.slashSel
		m.picker.clamp()
		m.slashSel = m.picker.sel
		return
	}
	if m.picker.kind == slashPicker || m.picker.kind == filesPicker {
		m.picker.close()
	}
}

func (m *Model) openModelPicker() {
	if m.deps.Auth == "grok-cli" && m.deps.ListGrokModels != nil {
		cur, models, err := m.deps.ListGrokModels()
		if err == nil && len(models) > 0 {
			items := make([]rowItem, 0, len(models)+1)
			for _, gm := range models {
				title := gm.Name
				if title == "" {
					title = gm.ID
				}
				tag := ""
				if gm.ID == cur {
					tag = "current"
				}
				items = append(items, rowItem{title: title, desc: gm.ID, id: "grok-model:" + gm.ID, tag: tag})
			}
			items = append(items, rowItem{title: "Change provider…", desc: "/provider", id: "providers"})
			m.picker.set(modelPicker, "Model", items)
			for i, it := range items {
				if it.tag == "current" {
					m.picker.sel = i
				}
			}
			m.input.Blur()
			return
		}
	}
	m.refreshProviders()
	m.picker.set(modelPicker, "Model", listItems(m.providers))
	m.input.Blur()
}

func (m *Model) openModePicker() {
	cur := string(m.deps.Mode)
	items := []rowItem{
		{title: "default", desc: "asks when the policy says so", id: "default"},
		{title: "plan", desc: "edits and shell blocked", id: "plan"},
		{title: "yolo", desc: "skip asks; destructive gate stays", id: "yolo"},
	}
	m.picker.set(modePicker, "Mode", items)
	for i, it := range items {
		if it.id == cur {
			m.picker.sel = i
		}
	}
	m.input.Blur()
}

func (m *Model) openSessionsPicker() {
	m.refreshSessions()
	m.picker.set(sessionsPicker, "Sessions", listItems(m.sessions))
	m.input.Blur()
}

func (m *Model) openFilesPicker(query string) {
	items := listWorkspaceFiles(m.deps.CWD, query, 40)
	m.picker.set(filesPicker, "", items)
	m.picker.filter = query
}

func (m *Model) closePicker() tea.Cmd {
	m.picker.close()
	m.slashHide = true
	m.focusTranscript = false
	return m.input.Focus()
}

func (m *Model) pickerKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case msg.String() == "esc":
		if m.picker.kind == slashPicker || m.picker.kind == filesPicker {
			m.slashHide = true
			m.picker.close()
			return nil
		}
		return m.closePicker()
	case msg.String() == "up":
		m.picker.move(-1)
		if m.picker.kind == slashPicker {
			m.slashSel = m.picker.sel
		}
		return nil
	case msg.String() == "down":
		m.picker.move(1)
		if m.picker.kind == slashPicker {
			m.slashSel = m.picker.sel
		}
		return nil
	case msg.String() == "tab" && (m.picker.kind == slashPicker || m.picker.kind == filesPicker):
		if m.picker.kind == filesPicker {
			m.acceptFile()
			return nil
		}
		m.slashComplete()
		return nil
	case key.Matches(msg, m.keys.submit) || msg.String() == "enter":
		return m.pickCurrent()
	}
	if m.picker.ownsTyping() {
		switch msg.String() {
		case "backspace", "ctrl+h":
			m.picker.backspace()
			return nil
		}
		if text := msg.Text; text != "" && !strings.ContainsAny(text, "\n\t") {
			m.picker.typeFilter(text)
			return nil
		}
		return nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.syncPicker()
	return cmd
}

func (m *Model) pickCurrent() tea.Cmd {
	switch m.picker.kind {
	case slashPicker:
		m.slashComplete()
		return m.submit()
	case modelPicker:
		if it, ok := m.picker.selected(); ok {
			if it.id == "providers" {
				m.refreshProviders()
				m.picker.set(modelPicker, "Model", listItems(m.providers))
				return nil
			}
			if id, ok := strings.CutPrefix(it.id, "grok-model:"); ok {
				if m.deps.SetGrokModel != nil {
					if err := m.deps.SetGrokModel(id); err != nil {
						m.setAlert(err.Error())
					} else {
						m.deps.FastModel = id
						m.status, m.alert = "model "+id, false
						if m.deps.ListGrokModels != nil {
							if cur, models, err := m.deps.ListGrokModels(); err == nil {
								for _, gm := range models {
									if gm.ID == id || gm.ID == cur {
										if gm.ContextTokens > 0 {
											m.deps.ContextLimit = gm.ContextTokens
										}
										break
									}
								}
							}
						}
					}
				}
				return m.closePicker()
			}
			m.selectProviderItem(it)
		}
		return m.closePicker()
	case modePicker:
		if it, ok := m.picker.selected(); ok {
			m.applyModeID(it.id)
		}
		return m.closePicker()
	case sessionsPicker:
		if it, ok := m.picker.selected(); ok {
			m.resumeID(it.id)
		}
		return m.closePicker()
	case filesPicker:
		m.acceptFile()
		return nil
	}
	return m.closePicker()
}

func (m *Model) applyModeID(id string) {
	switch id {
	case "plan":
		m.setMode(perms.ModePlan)
		m.showPlan = true
		m.refreshPlan()
		m.status = "plan mode. Shell is blocked. Write the plan with update_plan."
	case "yolo":
		m.reportReady()
		m.setMode(perms.ModeYolo)
		m.showPlan = false
		m.status = "yolo. Asks are skipped. The destructive gate still blocks."
	default:
		m.reportReady()
		m.setMode(perms.ModeDefault)
		m.showPlan = false
		m.status = "default mode"
	}
	m.alert = false
}

func (m *Model) acceptFile() {
	it, ok := m.picker.selected()
	if !ok {
		m.picker.close()
		return
	}
	v := m.input.Value()
	start, _, found := atToken(v)
	if !found {
		m.picker.close()
		return
	}
	m.input.SetValue(v[:start] + "@" + it.id + " ")
	m.picker.close()
}

func listItems(l list.Model) []rowItem {
	var items []rowItem
	for _, it := range l.Items() {
		if row, ok := it.(rowItem); ok {
			items = append(items, row)
		}
	}
	return items
}

func (m *Model) runPalette() tea.Cmd {
	item, ok := m.palette.SelectedItem().(rowItem)
	if !ok {
		return m.closeOverlay()
	}
	if cmd := m.closeOverlay(); cmd != nil {
		_ = cmd
	}
	switch {
	case strings.HasPrefix(item.id, "cmd:"):
		m.input.SetValue(strings.TrimPrefix(item.id, "cmd:"))
		return m.submit()
	case item.id == "key:focus":
		return m.toggleFocus()
	case item.id == "key:cycle":
		return m.cycleMode()
	case item.id == "key:sessions":
		m.openSessionsPicker()
	case item.id == "key:yolo":
		return m.toggleYolo()
	case item.id == "key:verbose":
		return m.toggleVerbose()
	case item.id == "key:help":
		m.openOverlay(helpOverlay)
	}
	return nil
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
	if m.overlay == updateOverlay {
		return m.updateKey(msg)
	}
	if m.overlay == paletteOverlay {
		if msg.String() == "esc" {
			return m.closeOverlay()
		}
		if msg.String() == "enter" {
			return m.runPalette()
		}
		var cmd tea.Cmd
		m.palette, cmd = m.palette.Update(msg)
		return cmd
	}
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
	case providerOverlay:
		if enter {
			return m.pickProvider()
		}
		m.providers, cmd = m.providers.Update(msg)
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
	m.picker.close()
	m.overlay = o
	m.input.Blur()
	if o == helpOverlay {
		m.sheet.GotoTop()
	}
}

func (m *Model) closeOverlay() tea.Cmd {
	m.overlay = noOverlay
	m.focusTranscript = false
	return m.input.Focus()
}

func (m *Model) quit() tea.Cmd {
	if m.pending != nil {
		m.answer(perms.Deny)
	}
	if m.cancel != nil {
		m.userCancel = true
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
		m.openSessionsPicker()
	case "/permissions":
		m.refreshPerms()
		m.openOverlay(permsOverlay)
	case "/agents":
		m.openOverlay(agentsOverlay)
	case "/ready":
		m.refreshPlan()
		res := m.reportReady()
		m.status = m.verdict
		m.apply(harness.Event{Kind: harness.EvJev, Name: "ready", Text: readyEventText(res, m.verdict)})
	case "/verbose":
		return m.toggleVerbose()
	case "/fork":
		return m.emitFork(rest)
	case "/provider":
		m.openModelPicker()
	case "/update":
		return m.askUpdate()
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

func (m *Model) pollUpdate(asked bool) tea.Cmd {
	return func() tea.Msg {
		n, err := update.Refresh(context.Background(), version.Version, nil)
		return updateNoticeMsg{notice: n, err: err, asked: asked}
	}
}

func (m *Model) askUpdate() tea.Cmd {
	if m.avail.Newer {
		m.openOverlay(updateOverlay)
		return nil
	}
	m.status, m.alert = "checking for a Rock release…", false
	return m.pollUpdate(true)
}

func (m *Model) onUpdateNotice(msg updateNoticeMsg) tea.Cmd {
	if msg.err != nil {
		if msg.asked {
			m.setAlert(msg.err.Error())
		}
		return nil
	}
	m.avail = msg.notice
	if msg.notice.Newer {
		if !m.gating() && !m.updateDismissed && m.overlay == noOverlay {
			m.openOverlay(updateOverlay)
		} else if msg.asked && !m.gating() {
			m.openOverlay(updateOverlay)
		} else if m.status == "ready" || strings.HasPrefix(m.status, "checking for a Rock") {
			m.status, m.alert = msg.notice.Line(), false
		}
		return nil
	}
	if msg.asked {
		m.status, m.alert = "already on "+version.Version, false
	}
	return nil
}

func (m *Model) updateKey(msg tea.KeyPressMsg) tea.Cmd {
	if m.updating {
		return nil
	}
	switch msg.String() {
	case "y", "enter":
		return m.applyUpdate()
	case "n", "esc":
		m.updateDismissed = true
		if m.avail.Newer {
			m.status, m.alert = m.avail.Line(), false
		}
		return m.closeOverlay()
	}
	return nil
}

func (m *Model) applyUpdate() tea.Cmd {
	exe, err := os.Executable()
	if err != nil {
		m.setAlert(err.Error())
		return m.closeOverlay()
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	if update.InstalledByGo(exe) {
		m.status, m.alert = "this binary is from go install. run: "+update.GoInstallHint(), false
		m.updateDismissed = true
		return m.closeOverlay()
	}
	m.updating = true
	m.status, m.alert = "updating…", false
	return func() tea.Msg {
		ver, err := (&update.Client{}).Apply(context.Background(), exe)
		return updateDoneMsg{ver: ver, err: err}
	}
}

func (m *Model) onUpdateDone(msg updateDoneMsg) tea.Cmd {
	m.updating = false
	if msg.err != nil {
		m.setAlert(msg.err.Error())
		return m.closeOverlay()
	}
	m.avail = update.Notice{}
	m.status, m.alert = "updated to v"+msg.ver+". Restart Rock.", false
	return m.closeOverlay()
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
	if m.gating() || m.deps.Run == nil || m.busy {
		return nil
	}
	m.busy = true
	m.turnAt = time.Now()
	m.turnFrom = len(m.lines)
	m.userCancel = false
	m.status, m.alert = "working", false
	m.log("info", "tui turn start", "chars", len(prompt), "auth", m.deps.Auth, "model", m.deps.FastModel, "w", m.width, "h", m.height)
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	sess := m.deps.Session
	at := m.turnAt
	run := func() (out tea.Msg) {
		var err error
		defer func() {
			// A panic in the harness or provider must still end the
			// turn with a visible error instead of a frozen spinner.
			if r := recover(); r != nil {
				err = fmt.Errorf("turn crashed: %v", r)
			}
			done := turnDone{err: err, ctx: sessionBytes(sess), at: at}
			if m.send != nil {
				m.send(done)
				out = nil
				return
			}
			out = done
		}()
		err = m.deps.Run(ctx, sess, prompt, m.ask, func(ev harness.Event) {
			if m.send != nil {
				m.send(turnEvent{ev: ev, ctx: sessionBytes(sess)})
			}
		})
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
		m.lines = append(m.lines, line{kind: "tool", name: ev.Name, text: ev.Text, at: time.Now()})
		m.lastCall = ev
		if ev.Name == "spawn_subagent" {
			m.agentStarted(ev.Text)
		}
	case harness.EvToolResult:
		m.attachResult(ev.Name, ev.Text)
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

func (m *Model) contextCap() float64 {
	if m.deps.ContextLimit > 0 {
		return float64(m.deps.ContextLimit)
	}
	return float64(contextLimit)
}

// retarget points the context meter at the current session size and starts
// the spring if it has somewhere to go.
func (m *Model) retarget() tea.Cmd {
	m.target = math.Min(1, math.Max(0, float64(m.ctxBytes)/float64(m.contextCap())))
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
	m.selected = -1
	m.spans = nil
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
				m.attachResult(msg.Name, msg.Content)
			}
		}
		m.ctxBytes = m.deps.Session.Bytes()
	}
	m.target = math.Min(1, float64(m.ctxBytes)/float64(m.contextCap()))
	m.follow = true
	m.syncView()
}

func (m *Model) resumeSelected() {
	item, ok := m.sessions.SelectedItem().(rowItem)
	if !ok {
		return
	}
	m.resumeID(item.id)
}

func (m *Model) resumeID(id string) {
	if id == "" || m.deps.LoadSession == nil {
		return
	}
	sess, err := m.deps.LoadSession(id)
	if err != nil {
		m.setAlert(err.Error())
		return
	}
	m.deps.Session = sess
	m.seedTranscript()
	m.refreshPlan()
	m.status, m.alert = "resumed "+id, false
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
func (m *Model) refreshProviders() {
	current := m.deps.Auth
	if current == "" {
		current = m.deps.Provider
	}
	items := []list.Item{
		rowItem{
			title: "ChatGPT",
			desc:  providerDesc("siwc", m.deps.HasSIWC, "Sign in with ChatGPT. Run rock login chatgpt if this is disabled."),
			id:    "siwc",
			tag:   providerTag("siwc", current, m.deps.HasSIWC),
		},
		rowItem{
			title: "SuperGrok",
			desc:  providerDesc("grok-cli", m.deps.HasGrokCLI, "Official grok binary over ACP. Run grok login yourself. Rock still runs tools and Jev."),
			id:    "grok-cli",
			tag:   providerTag("grok-cli", current, m.deps.HasGrokCLI),
		},
		rowItem{
			title: "API key",
			desc:  providerDesc("api_key", m.deps.HasAPIKey, "ROCK_API_KEY or OPENAI_API_KEY. The fallback when ChatGPT is not signed in."),
			id:    "api_key",
			tag:   providerTag("api_key", current, m.deps.HasAPIKey),
		},
		rowItem{
			title: "Offline model",
			desc:  "Model-provider stub only. Completions stay local. Jev still runs and still requires its own key.",
			id:    "offline_model",
			tag:   providerTag("offline_model", current, true),
		},
	}
	selected := 0
	for i, it := range items {
		if it.(rowItem).id == current {
			selected = i
		}
	}
	_ = m.providers.SetItems(items)
	m.providers.Select(selected)
}

func providerDesc(id string, ready bool, readyText string) string {
	if ready {
		return readyText
	}
	switch id {
	case "siwc":
		return "not signed in. rock login chatgpt. API keys stay the fallback."
	case "grok-cli":
		return "grok is not on PATH. Install from https://x.ai/cli then run grok login. Rock does not sign you in."
	case "api_key":
		return "no ROCK_API_KEY / OPENAI_API_KEY. Offline model is the fallback."
	}
	return readyText
}

func providerTag(id, current string, ready bool) string {
	if id == current {
		return "current"
	}
	if !ready && id != "offline_model" {
		return "ask"
	}
	return ""
}

func (m *Model) pickProvider() tea.Cmd {
	item, ok := m.providers.SelectedItem().(rowItem)
	if !ok {
		return m.closeOverlay()
	}
	m.selectProviderItem(item)
	return m.closeOverlay()
}

func (m *Model) selectProviderItem(item rowItem) {
	switch {
	case item.id == "siwc" && !m.deps.HasSIWC:
		m.setAlert("ChatGPT: run rock login chatgpt first. API keys stay the fallback.")
		return
	case item.id == "grok-cli" && !m.deps.HasGrokCLI:
		m.setAlert("grok is not on PATH. Install from https://x.ai/cli then run grok login. Rock does not sign you in.")
		return
	case item.id == "api_key" && !m.deps.HasAPIKey:
		m.setAlert("no ROCK_API_KEY / OPENAI_API_KEY; offline model is the fallback")
		return
	}
	if m.deps.SetAuth != nil {
		name, auth, err := m.deps.SetAuth(item.id)
		if err != nil {
			m.setAlert(err.Error())
			return
		}
		m.deps.Provider = name
		m.deps.Auth = auth
	} else {
		m.deps.Auth = item.id
		switch item.id {
		case "siwc":
			m.deps.Provider = "chatgpt"
		case "grok-cli":
			m.deps.Provider = "grok-cli"
		case "api_key":
			m.deps.Provider = "openai"
		default:
			m.deps.Provider = "offline"
		}
	}
	m.status, m.alert = "model auth "+m.deps.Auth, false
}

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

func (m *Model) reportReady() jev.Result {
	d := m.deps.Gates.DecideReady(context.Background(), m.plan, "")
	word := "not ready"
	if d.Ready {
		word = "ready"
	}
	m.verdict = fmt.Sprintf("plan %s (%.2f, %s). This does not approve the plan.", word, d.P, m.deps.Gates.Mode())
	return d.Result
}

func readyEventText(res jev.Result, verdict string) string {
	line := res.Line()
	if line == "" || line == "jev" {
		return verdict
	}
	first, rest, ok := strings.Cut(line, "\n")
	first = strings.TrimSpace(first + " " + verdict)
	if !ok {
		return first
	}
	return first + "\n" + rest
}

func (m *Model) onMouse(msg tea.MouseMsg) {
	mo := msg.Mouse()
	switch msg.(type) {
	case tea.MouseWheelMsg:
		up := mo.Button == tea.MouseWheelUp
		switch m.overlay {
		case sessionsOverlay:
			wheelList(&m.sessions, up)
		case paletteOverlay:
			wheelList(&m.palette, up)
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
			if m.picker.open() {
				if up {
					m.picker.move(-1)
				} else {
					m.picker.move(1)
				}
				if m.picker.kind == slashPicker {
					m.slashSel = m.picker.sel
				}
				return
			}
			if m.inPlan(mo.X, mo.Y) {
				m.planVP, _ = m.planVP.Update(msg)
				return
			}
			m.vp, _ = m.vp.Update(msg)
			m.follow = m.vp.AtBottom()
		}
	case tea.MouseClickMsg:
		if mo.Button != tea.MouseLeft {
			return
		}
		if m.clickChip(mo.X, mo.Y) {
			return
		}
		if m.overlay != noOverlay {
			return
		}
		if m.onScrollbar(mo.X, mo.Y) {
			m.dragging = true
			m.scrollTo(mo.Y)
			return
		}
		m.clickTranscript(mo.X, mo.Y)
	case tea.MouseMotionMsg:
		if m.dragging {
			m.scrollTo(mo.Y)
		}
	case tea.MouseReleaseMsg:
		m.dragging = false
	}
}

func (m *Model) clickChip(x, y int) bool {
	if y != m.chipY || m.pending != nil || m.overlay != noOverlay {
		return false
	}
	for _, c := range m.chips {
		if x >= c.x && x < c.x+c.w {
			switch c.id {
			case "model":
				if m.picker.kind == modelPicker {
					_ = m.closePicker()
					return true
				}
				m.openModelPicker()
			case "mode":
				if m.picker.kind == modePicker {
					_ = m.closePicker()
					return true
				}
				m.openModePicker()
			}
			return true
		}
	}
	return false
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
		return x >= g.padL+g.transcriptW && y >= g.vpY && y < g.vpY+g.vpH
	case g.planH > 0:
		return y >= g.bodyY && y < g.bodyY+g.planH && x >= g.padL
	}
	return false
}

func (m *Model) overflowing() bool {
	return m.vp.TotalLineCount() > m.vp.Height()
}

func (m *Model) onScrollbar(x, y int) bool {
	g := m.geo
	return m.overflowing() && x == g.barX && y >= g.vpY && y < g.vpY+g.vpH
}

func (m *Model) invalidatePaint() {
	for i := range m.lines {
		m.lines[i].out = ""
	}
}

func (m *Model) clickTranscript(x, y int) {
	g := m.geo
	if m.pending != nil || y < g.vpY || y >= g.vpY+g.vpH || x < g.padL || x >= g.barX || m.inPlan(x, y) {
		return
	}
	local := y - g.vpY
	if h, from := m.stickyUser(); h > 0 && local < h && from >= 0 {
		m.selectEntry(from)
		return
	}
	contentY := m.vp.YOffset() + local
	for i := range m.spans {
		s := m.spans[i]
		if contentY >= s.y0 && contentY <= s.y1 {
			m.selectEntry(s.from)
			return
		}
	}
}

func (m *Model) selectEntry(from int) {
	if from < 0 || from >= len(m.lines) {
		return
	}
	if m.selected == from {
		if m.foldableAt(from) {
			m.toggleFold(from)
		}
		return
	}
	m.selected = from
	m.invalidatePaint()
	m.syncView()
}

func (m *Model) toggleFold(from int) {
	if from < 0 || from >= len(m.lines) {
		return
	}
	was := m.isFolded(from)
	m.lines[from].foldTouched = true
	m.lines[from].folded = !was
	m.invalidatePaint()
	m.syncView()
}

func (m *Model) foldableAt(from int) bool {
	if from < 0 || from >= len(m.lines) {
		return false
	}
	if _, _, n := verbRun(m.lines, from); n > 1 {
		return true
	}
	if m.lines[from].kind == "tool" {
		return true
	}
	return lineFoldable(m.lines[from], m.contentW)
}

func (m *Model) isFolded(from int) bool {
	if from < 0 || from >= len(m.lines) {
		return false
	}
	ln := m.lines[from]
	if ln.foldTouched {
		return ln.folded
	}
	if _, _, n := verbRun(m.lines, from); n > 1 {
		return true
	}
	return ln.kind == "tool" && toolFoldsByDefault(ln.name)
}

func (m *Model) attachResult(name, text string) {
	for i := range m.lines {
		if m.lines[i].kind == "tool" && m.lines[i].name == name && !m.lines[i].done {
			m.lines[i].result = text
			m.lines[i].done = true
			m.lines[i].finished = time.Now()
			m.lines[i].out = ""
			return
		}
	}
	m.lines = append(m.lines, line{kind: "result", name: name, text: text, done: true})
}

func (m *Model) toolRunning(i int) bool {
	if !m.busy || i < 0 || i >= len(m.lines) {
		return false
	}
	ln := m.lines[i]
	if ln.kind != "tool" || ln.done {
		return false
	}
	for j := i + 1; j < len(m.lines); j++ {
		if m.lines[j].kind == "tool" && !m.lines[j].done {
			return false
		}
	}
	return true
}

func (m *Model) lastUserSpan() *entrySpan {
	for i := len(m.spans) - 1; i >= 0; i-- {
		s := &m.spans[i]
		if s.from >= 0 && s.from < len(m.lines) && m.lines[s.from].kind == "user" {
			return s
		}
	}
	return nil
}

// stickyUser is the last user prompt when the viewport has scrolled past it.
// Height is 0 when nothing should pin.
func (m *Model) stickyUser() (height, from int) {
	u := m.lastUserSpan()
	if u == nil || m.vp.YOffset() <= u.y0 {
		return 0, -1
	}
	return 1, u.from
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
