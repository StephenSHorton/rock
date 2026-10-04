package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/StephenSHorton/rock/internal/harness"
	"github.com/StephenSHorton/rock/internal/jev"
	"github.com/StephenSHorton/rock/internal/perms"
	"github.com/StephenSHorton/rock/internal/provider"
	"github.com/StephenSHorton/rock/internal/session"
	"github.com/StephenSHorton/rock/internal/tools"
)

func testModel(t *testing.T) *Model {
	t.Helper()
	t.Setenv("ROCK_HOME", t.TempDir())
	sess, err := session.Create(t.TempDir(), "", "demo")
	if err != nil {
		t.Fatal(err)
	}
	m := New(Deps{
		CWD:     sess.Meta.CWD,
		Session: sess,
		Mode:    perms.ModeDefault,
		JevMode: "offline",
		Gates:   jev.Gates{},
		Run: func(ctx context.Context, sess *session.Session, prompt string, ask harness.AskFunc, sink func(harness.Event)) error {
			sink(harness.Event{Kind: harness.EvAssistant, Text: "from the harness: " + prompt})
			sink(harness.Event{Kind: harness.EvDone, Text: "end_turn"})
			return nil
		},
		ListSessions: func() []session.Meta { return []session.Meta{sess.Meta} },
		Rules:        []string{"allow read_file", "ask shell"},
		Fork:         func(string) (string, error) { return "\x1b]7880;brand=rock\a", nil },
	})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m
}

func TestViewRendersSession(t *testing.T) {
	m := testModel(t)
	view := m.View().Content
	if !strings.Contains(view, "Rock") || !strings.Contains(view, "jev:offline") {
		t.Fatalf("view:\n%s", view)
	}
	if !strings.Contains(view, "ready") {
		t.Fatal(view)
	}
}

func TestLayoutHasNoWordmarkHeader(t *testing.T) {
	m := sized(t, 120, 30)
	rows := screen(m)
	if strings.HasPrefix(strings.TrimSpace(rows[0]), "rock") {
		t.Fatalf("branded header wordmark still occupies row 0: %q", rows[0])
	}
	if m.geo.padT != 1 || m.geo.padL != 1 {
		t.Fatalf("tall screen should pad: %+v", m.geo)
	}
	compact := sized(t, 120, 20)
	if compact.geo.padT != 0 || !compact.geo.compact {
		t.Fatalf("height 20 should compact: %+v", compact.geo)
	}
	info := screen(m)[m.geo.composerY()+m.geo.composerRows]
	if !strings.Contains(info, "gpt-4o-mini") || !strings.Contains(info, "default") {
		t.Fatalf("composer info line: %q", info)
	}
}

func TestSlashPlanAndPermission(t *testing.T) {
	m := testModel(t)
	var mode perms.Mode
	m.deps.SetMode = func(next perms.Mode) { mode = next }
	m.input.SetValue("/plan")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	if m.deps.Mode != perms.ModePlan || mode != perms.ModePlan {
		t.Fatalf("mode %s set %s status %s", m.deps.Mode, mode, m.status)
	}
	if !strings.Contains(m.status, "Shell is blocked") {
		t.Fatal(m.status)
	}
	reply := make(chan perms.Decision, 1)
	m.Update(askMsg{tool: "shell", detail: "ls", reply: reply})
	if m.pending == nil {
		t.Fatal("expected a permission prompt")
	}
	view := m.View().Content
	if !strings.Contains(view, "Allow") {
		t.Fatal(view)
	}
	m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	select {
	case d := <-reply:
		if d != perms.Allow {
			t.Fatal(d)
		}
	default:
		t.Fatal("decision was not delivered")
	}
}

func TestTurnEventsLandInTheTranscript(t *testing.T) {
	m := testModel(t)
	m.Update(eventMsg{harness.Event{Kind: harness.EvAssistant, Text: "hello from jev's neighbor"}})
	m.Update(eventMsg{harness.Event{Kind: harness.EvToolCall, Name: "spawn_subagent", Text: `{"kind":"explore"}`}})
	m.Update(tickMsg{})
	if m.meterP == 0 && m.target == 0 {
		t.Fatal("context meter did not move")
	}
	if !strings.Contains(m.View().Content, "hello from jev") {
		t.Fatal(m.View().Content)
	}
	if len(m.agents.Rows()) == 0 {
		t.Fatal("subagent table empty")
	}
}

func TestForkCommandEmitsOSC(t *testing.T) {
	m := testModel(t)
	var got string
	m.deps.Output = writerFn(func(p []byte) (int, error) {
		got = string(p)
		return len(p), nil
	})
	m.input.SetValue("/fork look around")
	model, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	m = model.(*Model)
	if cmd == nil {
		t.Fatal("expected fork command")
	}
	msg := cmd()
	m.Update(msg)
	if !strings.Contains(got, "7880") {
		t.Fatalf("osc %q status %s", got, m.status)
	}
}

type writerFn func([]byte) (int, error)

func (f writerFn) Write(p []byte) (int, error) { return f(p) }

func sized(t *testing.T, w, h int) *Model {
	t.Helper()
	m := testModel(t)
	m.deps.FastModel = "gpt-4o-mini"
	m.deps.Provider = "offline"
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

func submit(m *Model, text string) tea.Cmd {
	m.input.SetValue(text)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	return cmd
}

// screen is the frame as a terminal would show it: one string per row.
func screen(m *Model) []string {
	return strings.Split(ansi.Strip(m.View().Content), "\n")
}

func flat(s string) string { return strings.Join(strings.Fields(s), " ") }

// transcriptText is the transcript column of the frame, read row by row.
func transcriptText(m *Model) string {
	rows := screen(m)
	g := m.geo
	var parts []string
	for y := g.vpY; y < g.vpY+g.vpH && y < len(rows); y++ {
		parts = append(parts, ansi.Cut(rows[y], g.padL, g.padL+g.transcriptW))
	}
	return flat(strings.Join(parts, " "))
}

// planText is the inside of the plan pane, read row by row.
func planText(m *Model) string {
	rows := screen(m)
	g := m.geo
	left, top, height := g.padL+g.transcriptW+1, g.vpY, g.vpH
	if g.planH > 0 {
		left, top, height = g.padL, g.bodyY, g.planH
	}
	right := g.padL + g.innerW - 2
	if g.planW > 0 {
		right = left + g.planW - 2
	}
	var parts []string
	for y := top + 1; y < top+height-1 && y < len(rows); y++ {
		parts = append(parts, ansi.Cut(rows[y], left+2, right))
	}
	return flat(strings.Join(parts, " "))
}

func assertFrame(t *testing.T, m *Model, w, h int) []string {
	t.Helper()
	rows := screen(m)
	if len(rows) != h {
		t.Fatalf("%dx%d: frame has %d rows", w, h, len(rows))
	}
	for i, r := range rows {
		if ansi.StringWidth(r) > w {
			t.Fatalf("%dx%d: row %d is %d cells wide:\n%s", w, h, i, ansi.StringWidth(r), r)
		}
	}
	return rows
}

func isQuit(cmd tea.Cmd) bool {
	return cmd != nil && reflect.ValueOf(cmd).Pointer() == reflect.ValueOf(tea.Quit).Pointer()
}

func TestReadySentenceIsWholeAndThePlanColumnIsHiddenByDefault(t *testing.T) {
	for _, tc := range []struct{ w, h int }{
		{60, 24}, {80, 24}, {99, 24},
		{100, 24}, {110, 30}, {120, 30}, {140, 36},
	} {
		m := sized(t, tc.w, tc.h)
		rows := assertFrame(t, m, tc.w, tc.h)
		if got := transcriptText(m); !strings.Contains(got, readyText) {
			t.Fatalf("%d cols: ready sentence is not whole: %q", tc.w, got)
		}
		g := m.geo
		if g.planW > 0 || g.planH > 0 {
			t.Fatalf("%d cols: plan pane should stay hidden by default (planW=%d planH=%d)", tc.w, g.planW, g.planH)
		}
		if g.transcriptW != g.innerW {
			t.Fatalf("%d cols: transcript %d should use the full inner width %d", tc.w, g.transcriptW, g.innerW)
		}
		if strings.Contains(strings.Join(rows[g.vpY:g.vpY+g.vpH], "\n"), "Plan") {
			t.Fatalf("%d cols: plan drawn while hidden", tc.w)
		}
	}
}

func TestPlanPaneShowsInPlanModeOrWhenAPlanExists(t *testing.T) {
	for _, w := range []int{80, 140} {
		m := sized(t, w, 30)
		if m.geo.planW != 0 {
			t.Fatalf("%d cols: idle should not open the plan column", w)
		}
		submit(m, "/plan")
		if m.geo.planW == 0 {
			t.Fatalf("%d cols: /plan did not open the plan column", w)
		}
		if m.geo.transcriptW+1+m.geo.planW != m.geo.innerW {
			t.Fatalf("%d cols: transcript %d + gap + plan %d does not fill the inner row", w, m.geo.transcriptW, m.geo.planW)
		}
		rows := assertFrame(t, m, w, 30)
		for y := m.geo.vpY; y < m.geo.vpY+m.geo.vpH; y++ {
			left := ansi.Cut(rows[y], m.geo.padL, m.geo.padL+m.geo.transcriptW)
			gap := ansi.Cut(rows[y], m.geo.padL+m.geo.transcriptW, m.geo.padL+m.geo.transcriptW+1)
			plan := ansi.Cut(rows[y], m.geo.padL+m.geo.transcriptW+1, m.geo.padL+m.geo.innerW)
			if strings.ContainsAny(left, "╭╰╮╯") {
				t.Fatalf("%d cols row %d: plan border inside the transcript column: %q", w, y, left)
			}
			if gap != " " || ansi.StringWidth(plan) != m.geo.planW {
				t.Fatalf("%d cols row %d: gap %q plan %d cells, want %d", w, y, gap, ansi.StringWidth(plan), m.geo.planW)
			}
		}
	}

	m := sized(t, 140, 30)
	if err := os.WriteFile(m.deps.Session.PlanPath(), []byte("## Steps\n\n1. Ship the polish\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.refreshPlan()
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	if !m.hasPlan() || m.geo.planW == 0 {
		t.Fatalf("a written plan should open the pane without plan mode: hasPlan=%v planW=%d mode=%s", m.hasPlan(), m.geo.planW, m.deps.Mode)
	}
	if !strings.Contains(planText(m), "Ship the polish") {
		t.Fatalf("plan body missing: %q", planText(m))
	}
	submit(m, "/default")
	if m.deps.Mode != perms.ModeDefault || m.geo.planW == 0 {
		t.Fatalf("leaving plan mode should keep the pane while a plan exists: mode=%s planW=%d", m.deps.Mode, m.geo.planW)
	}

	empty := sized(t, 140, 30)
	empty.deps.Mode = perms.ModePlan
	empty.showPlan = true
	empty.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	if empty.geo.planW == 0 {
		t.Fatal("plan mode should show the pane even with an empty plan")
	}
}

func TestShortHeightKeepsStatusComposerAndHelp(t *testing.T) {
	for _, h := range []int{8, 10, 12, 16} {
		for _, w := range []int{60, 120} {
			m := sized(t, w, h)
			rows := assertFrame(t, m, w, h)
			g := m.geo
			status := rows[g.statusY()]
			if !strings.Contains(status, "jev:offline") || !strings.Contains(status, "gpt-4o-mini") || !strings.Contains(status, "ctx") {
				t.Fatalf("%dx%d: status row %q", w, h, status)
			}
			top := strings.TrimLeft(rows[g.composerY()], " ")
			if !strings.Contains(top, "❯") {
				t.Fatalf("%dx%d: composer should use ❯, got %q", w, h, top)
			}
			if strings.Contains(rows[g.composerY()], "╭") {
				t.Fatalf("%dx%d: composer still has a rounded box: %q", w, h, rows[g.composerY()])
			}
			if g.helpRows > 0 && !strings.Contains(rows[h-1], "send") && !strings.Contains(rows[h-1], "allow") {
				t.Fatalf("%dx%d: help row %q", w, h, rows[h-1])
			}
		}
	}
}

func TestStatusLineCarriesModeJevModelAndMeter(t *testing.T) {
	for _, w := range []int{60, 80, 140} {
		m := sized(t, w, 24)
		status := screen(m)[m.geo.statusY()]
		for _, want := range []string{"DEFAULT", "jev:offline", "gpt-4o-mini", "(offline)", "ctx", "0%"} {
			if !strings.Contains(status, want) {
				t.Fatalf("%d cols: status %q lacks %q", w, status, want)
			}
		}
		if !strings.Contains(status, "│") {
			t.Fatalf("%d cols: status should use │ segments: %q", w, status)
		}
		if w >= 80 && !strings.Contains(status, "─") {
			t.Fatalf("%d cols: meter bar missing from %q", w, status)
		}
		if i, j := strings.Index(status, "0%"), strings.Index(status, "─"); w >= 80 && i >= 0 && j >= 0 && i > j {
			t.Fatalf("%d cols: percent should come before the meter: %q", w, status)
		}
	}
	m := sized(t, 120, 24)
	m.Update(eventMsg{harness.Event{Kind: harness.EvJev, Name: "turn", Text: "offline model=strong stuck=false"}})
	m.deps.StrongModel = "gpt-4o"
	m.Update(eventMsg{harness.Event{Kind: harness.EvJev, Name: "turn", Text: "offline model=strong stuck=false"}})
	if status := screen(m)[m.geo.statusY()]; !strings.Contains(status, "gpt-4o ") {
		t.Fatalf("strong turn should show the strong model: %q", status)
	}
	if strings.Contains(transcriptText(m), "◇ jev") {
		t.Fatalf("turn diagnostics should stay off the transcript by default:\n%s", transcriptText(m))
	}
}

func TestJevDiagnosticsStayHiddenUntilVerbose(t *testing.T) {
	m := sized(t, 100, 24)
	m.Update(eventMsg{harness.Event{Kind: harness.EvJev, Name: "turn", Text: "offline model=fast stuck=false compact=false skills="}})
	m.Update(eventMsg{harness.Event{Kind: harness.EvJev, Name: "risk", Text: "offline p=0.15 block=false"}})
	m.Update(eventMsg{harness.Event{Kind: harness.EvJev, Name: "ready", Text: "plan looks good"}})
	got := transcriptText(m)
	if strings.Contains(got, "stuck=") || strings.Contains(got, "p=0.15") || strings.Contains(got, "◇ jev turn") || strings.Contains(got, "◇ jev risk") {
		t.Fatalf("diagnostic jev lines should be hidden by default: %q", got)
	}
	if !strings.Contains(got, "◇ jev ready") || !strings.Contains(got, "plan looks good") {
		t.Fatalf("user-facing ◇ jev marks should stay visible: %q", got)
	}
	if status := screen(m)[m.geo.statusY()]; !strings.Contains(status, "jev:offline") {
		t.Fatalf("status jev mark missing: %q", status)
	}

	submit(m, "/verbose")
	if !m.verbose {
		t.Fatal("/verbose should turn diagnostics on")
	}
	got = transcriptText(m)
	for _, want := range []string{"◇ jev turn", "stuck=false", "◇ jev risk", "p=0.15", "◇ jev ready"} {
		if !strings.Contains(got, want) {
			t.Fatalf("verbose transcript lacks %q: %q", want, got)
		}
	}

	submit(m, "/verbose")
	if m.verbose {
		t.Fatal("/verbose again should hide diagnostics")
	}
	got = transcriptText(m)
	if strings.Contains(got, "◇ jev turn") || strings.Contains(got, "stuck=") || strings.Contains(got, "p=0.15") {
		t.Fatalf("diagnostics still visible after toggle off: %q", got)
	}
	if !strings.Contains(got, "◇ jev ready") {
		t.Fatalf("user-facing mark disappeared: %q", got)
	}

	on := sized(t, 100, 24)
	on.verbose = true
	on.Update(eventMsg{harness.Event{Kind: harness.EvJev, Name: "turn", Text: "offline model=fast stuck=false compact=false skills="}})
	if !strings.Contains(transcriptText(on), "◇ jev turn") {
		t.Fatalf("--verbose start should show diagnostics: %q", transcriptText(on))
	}

	keys := sized(t, 100, 24)
	keys.Update(eventMsg{harness.Event{Kind: harness.EvJev, Name: "risk", Text: "offline p=0.15 block=false"}})
	keys.Update(tea.KeyPressMsg{Code: tea.KeyTab, Text: "tab"})
	if !keys.focusTranscript {
		t.Fatal("tab should focus the transcript")
	}
	keys.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	if !keys.verbose || !strings.Contains(transcriptText(keys), "p=0.15") {
		t.Fatalf("v in the transcript should show diagnostics: verbose=%v %q", keys.verbose, transcriptText(keys))
	}
}

func TestStatusLineSitsUnderTheComposer(t *testing.T) {
	m := sized(t, 120, 24)
	g := m.geo
	if g.statusY() <= g.composerY() {
		t.Fatalf("status should sit under the composer: composerY=%d statusY=%d", g.composerY(), g.statusY())
	}
	rows := screen(m)
	if !strings.Contains(rows[g.composerY()], "❯") {
		t.Fatalf("composer: %q", rows[g.composerY()])
	}
	if !strings.Contains(rows[g.statusY()], "│") || !strings.Contains(rows[g.statusY()], "jev:offline") {
		t.Fatalf("status: %q", rows[g.statusY()])
	}
}

func TestStatusLineShowsSessionNameAndTurnTimer(t *testing.T) {
	m := sized(t, 140, 24)
	m.deps.CWD = "/tmp/rock"
	m.layout()
	if status := screen(m)[m.geo.statusY()]; !strings.Contains(status, "demo") {
		t.Fatalf("titled session should name itself: %q", status)
	}
	m.busy = true
	m.turnAt = time.Now().Add(-12 * time.Second)
	if status := screen(m)[m.geo.statusY()]; !strings.Contains(status, "12s") {
		t.Fatalf("busy turn should show the timer: %q", status)
	}
}

func TestContextMeterEasesWithoutJumping(t *testing.T) {
	m := sized(t, 120, 30)
	_, cmd := m.Update(turnEvent{ev: harness.Event{Kind: harness.EvAssistant, Text: "hi"}, ctx: 6000})
	if m.target != 0.25 || cmd == nil || !m.ticking {
		t.Fatalf("target %v ticking %v", m.target, m.ticking)
	}
	prev, steps := m.meterP, 0
	for m.ticking {
		_, cmd = m.Update(tickMsg{})
		step := m.meterP - prev
		if step < -1e-9 || m.meterP > m.target+1e-6 {
			t.Fatalf("step %d: meter went from %v to %v (target %v)", steps, prev, m.meterP, m.target)
		}
		if step > 0.15*m.target {
			t.Fatalf("step %d: meter jumped %v in one frame", steps, step)
		}
		prev, steps = m.meterP, steps+1
		if steps > 90 {
			t.Fatal("meter never settled")
		}
	}
	if steps < 10 || cmd != nil || m.meterP != m.target {
		t.Fatalf("settled after %d frames at %v, cmd %v", steps, m.meterP, cmd != nil)
	}
	if status := screen(m)[m.geo.statusY()]; !strings.Contains(status, "25%") || !strings.Contains(status, "━") {
		t.Fatalf("status %q", status)
	}
}

func TestOverlaysOpenReadableAndCloseBackToTheTranscript(t *testing.T) {
	for _, tc := range []struct {
		cmd, title string
		want       []string
	}{
		{"/help", "Help", []string{"Slash commands", "/default", "/quit", "/permissions"}},
		{"/sessions", "Sessions", []string{"demo", "this session"}},
		{"/permissions", "Permissions", []string{"Permissions are not a sandbox.", "read_file", "shell", "default"}},
		{"/agents", "Subagents", []string{"No subagents yet."}},
	} {
		for _, size := range [][2]int{{80, 24}, {140, 36}} {
			w, h := size[0], size[1]
			m := sized(t, w, h)
			submit(m, tc.cmd)
			if m.overlay == noOverlay || m.input.Focused() {
				t.Fatalf("%s at %d: overlay did not take focus", tc.cmd, w)
			}
			view := flat(strings.Join(assertFrame(t, m, w, h), "\n"))
			for _, want := range append([]string{tc.title}, tc.want...) {
				if !strings.Contains(view, want) {
					t.Fatalf("%s at %d cols lacks %q:\n%s", tc.cmd, w, want, strings.Join(screen(m), "\n"))
				}
			}
			m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
			if m.overlay != noOverlay || !m.input.Focused() {
				t.Fatalf("%s at %d: esc did not return to the composer", tc.cmd, w)
			}
			if !strings.Contains(transcriptText(m), readyText) {
				t.Fatalf("%s at %d: transcript not back", tc.cmd, w)
			}
		}
	}
}

func TestQInsidePanelsClosesInsteadOfQuitting(t *testing.T) {
	m := sized(t, 100, 24)
	submit(m, "/sessions")
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"}); isQuit(cmd) || m.overlay != noOverlay {
		t.Fatal("q in the session list must close it, not quit Rock")
	}
	reply := make(chan perms.Decision, 1)
	m.Update(askMsg{tool: "shell", detail: "ls", reply: reply})
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"}); isQuit(cmd) || m.pending == nil || len(reply) != 0 {
		t.Fatal("q in the allow dialog must neither quit nor answer")
	}
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); isQuit(cmd) || m.pending != nil || <-reply != perms.Deny {
		t.Fatal("first ctrl+c denies the pending call without quitting")
	}
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); !isQuit(cmd) {
		t.Fatal("second ctrl+c quits")
	}
}

func TestPlanPaneSaysShellIsBlockedBesideAReadableTranscript(t *testing.T) {
	for _, w := range []int{80, 140} {
		m := sized(t, w, 30)
		submit(m, "/plan")
		if m.geo.planW == 0 {
			t.Fatalf("%d cols: /plan did not open the plan column", w)
		}
		rows := assertFrame(t, m, w, 30)
		plan := planText(m)
		for _, want := range []string{"No plan yet.", "Shell is blocked", "plan: empty"} {
			if !strings.Contains(plan, want) {
				t.Fatalf("%d cols: plan pane lacks %q: %q", w, want, plan)
			}
		}
		if !strings.Contains(rows[m.geo.statusY()], "PLAN") {
			t.Fatalf("%d cols: status should show PLAN", w)
		}
		if !strings.Contains(transcriptText(m), readyText) {
			t.Fatalf("%d cols: transcript beside the plan: %q", w, transcriptText(m))
		}
	}

	m := sized(t, 60, 24)
	submit(m, "/plan")
	if plan := planText(m); m.geo.planH == 0 || !strings.Contains(plan, "No plan yet.") || !strings.Contains(plan, "shell is blocked") {
		t.Fatalf("60 cols: plan mode stacks the plan above the transcript: %q", plan)
	}
	if !strings.Contains(transcriptText(m), readyText) {
		t.Fatalf("60 cols: transcript under the plan: %q", transcriptText(m))
	}
	if err := os.WriteFile(m.deps.Session.PlanPath(), []byte("## Steps\n\n1. Read greeting.txt\n2. Fix the typo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.Update(eventMsg{harness.Event{Kind: harness.EvToolResult, Name: "update_plan", Text: "updated plan"}})
	if plan := planText(m); !strings.Contains(plan, "Read greeting.txt") {
		t.Fatalf("plan body not shown: %q\n%s", plan, strings.Join(screen(m), "\n"))
	}
	submit(m, "/default")
	if m.deps.Mode != perms.ModeDefault {
		t.Fatal("leaving plan mode should return to default")
	}
	if m.geo.planH == 0 {
		t.Fatal("a written plan should keep the stacked pane after leaving plan mode")
	}
}

func TestAllowDialogShowsTheCallAndKeepsTheChrome(t *testing.T) {
	for _, size := range [][2]int{{140, 36}, {80, 24}, {80, 14}} {
		w, h := size[0], size[1]
		m := sized(t, w, h)
		m.Update(eventMsg{harness.Event{Kind: harness.EvToolCall, Name: "edit_file", Text: `{"path":"greeting.txt","old":"helo wrold","new":"hello world"}`}})
		reply := make(chan perms.Decision, 1)
		m.Update(askMsg{tool: "edit_file", detail: "greeting.txt", reply: reply})
		rows := assertFrame(t, m, w, h)
		view := strings.Join(rows, "\n")
		want := []string{"Allow edit_file?", "Allow", "Deny", "y allow"}
		if h >= 24 {
			want = append(want, "path greeting.txt", "- helo wrold", "+ hello world")
		}
		for _, s := range want {
			if !strings.Contains(view, s) {
				t.Fatalf("%dx%d dialog lacks %q:\n%s", w, h, s, view)
			}
		}
		g := m.geo
		help := rows[h-1]
		if g.padB > 0 && h-2 >= 0 {
			help = rows[h-1-g.padB]
		}
		if !strings.Contains(rows[g.statusY()], "jev:offline") || !strings.Contains(help, "allow") {
			t.Fatalf("%dx%d: chrome pushed off:\n%s", w, h, view)
		}
		m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
		if d := <-reply; d != perms.Deny || m.pending != nil || !m.input.Focused() {
			t.Fatalf("%dx%d: answer %s", w, h, d)
		}
	}
}

func TestMouseWheelAndScrollbarScrollTheTranscript(t *testing.T) {
	m := sized(t, 120, 20)
	for i := range 40 {
		m.Update(eventMsg{harness.Event{Kind: harness.EvAssistant, Text: fmt.Sprintf("line %d", i)}})
	}
	if !m.vp.AtBottom() || !m.overflowing() {
		t.Fatal("transcript should start pinned to the bottom")
	}
	bottom := m.vp.YOffset()
	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp, X: 10, Y: 5})
	if m.vp.YOffset() >= bottom || m.follow {
		t.Fatalf("wheel up did not scroll: %d -> %d", bottom, m.vp.YOffset())
	}
	off := m.vp.YOffset()
	m.Update(eventMsg{harness.Event{Kind: harness.EvAssistant, Text: "late reply"}})
	if m.vp.YOffset() != off {
		t.Fatal("a new event yanked the transcript while the reader was scrolled up")
	}
	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 10, Y: 5})
	if m.vp.YOffset() <= off {
		t.Fatal("wheel down did not scroll")
	}
	g := m.geo
	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: g.barX, Y: g.vpY})
	if m.vp.YOffset() != 0 {
		t.Fatalf("click on the top of the scrollbar: offset %d", m.vp.YOffset())
	}
	if !strings.Contains(screen(m)[g.vpY], "┃") {
		t.Fatalf("scrollbar thumb not at the top: %q", screen(m)[g.vpY])
	}
	m.Update(tea.MouseMotionMsg{Button: tea.MouseLeft, X: g.barX, Y: g.vpY + g.vpH - 1})
	m.Update(tea.MouseReleaseMsg{Button: tea.MouseLeft, X: g.barX, Y: g.vpY + g.vpH - 1})
	if !m.vp.AtBottom() || !m.follow || m.dragging {
		t.Fatal("dragging the thumb to the bottom should pin the transcript again")
	}
	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 10, Y: g.vpY})
	if !m.vp.AtBottom() {
		t.Fatal("a click in the text is not a scrollbar click")
	}
}

func TestLightTerminalGetsThePalettePair(t *testing.T) {
	m := sized(t, 100, 24)
	_ = m.input.Focus()
	m.layout()
	if dark := m.View().Content; !strings.Contains(dark, "122;155;255") {
		t.Fatal("dark frame should draw with brand #7A9BFF")
	}
	m.Update(tea.BackgroundColorMsg{Color: color.White})
	if m.th.dark {
		t.Fatal("white background should select the light palette")
	}
	light := m.View().Content
	if !strings.Contains(light, "43;89;232") || strings.Contains(light, "122;155;255") {
		t.Fatal("light frame should use the light brand and nothing from the dark one")
	}
	assertFrame(t, m, 100, 24)
}

func TestAssistantMarkdownUsesTheRockPalette(t *testing.T) {
	m := sized(t, 120, 30)
	m.Update(eventMsg{harness.Event{Kind: harness.EvAssistant, Text: "## Edited\n\nChanged `greeting.txt`.\n\n- **one** item\n"}})
	text := transcriptText(m)
	for _, want := range []string{"Edited", "greeting.txt", "one item"} {
		if !strings.Contains(text, want) {
			t.Fatalf("markdown lost %q: %q", want, text)
		}
	}
	if strings.Contains(text, "##") || strings.Contains(text, "**") {
		t.Fatalf("markdown was not rendered: %q", text)
	}
	raw := m.View().Content
	// Glamour's truecolor heading is one count off #7A9BFF (121 vs 122).
	if !strings.Contains(raw, "38;2;122;155;") && !strings.Contains(raw, "38;2;121;155;") {
		t.Fatal("heading should be brand azurite")
	}
	if strings.Contains(raw, "38;5;") || strings.Contains(raw, "48;5;") {
		t.Fatal("a 256-color default style leaked into the frame")
	}
}

func TestCodeBlocksFollowTheThemeInTruecolor(t *testing.T) {
	m := sized(t, 120, 30)
	m.Update(eventMsg{harness.Event{Kind: harness.EvAssistant, Text: "```go\nfmt.Println(\"hi\") // say hi\n```"}})
	dark := m.View().Content
	if !strings.Contains(dark, "38;2;139;147;161") || strings.Contains(dark, "38;5;") {
		t.Fatal("dark code comment should be truecolor muted")
	}
	m.Update(tea.BackgroundColorMsg{Color: color.White})
	light := m.View().Content
	if !strings.Contains(light, "38;2;107;114;128") || strings.Contains(light, "38;2;139;147;") || strings.Contains(light, "38;5;") {
		t.Fatal("light code comment should switch to the light muted")
	}
}

func TestPlaceholderListsEverySlashCommandAndTheyAllWork(t *testing.T) {
	all := []string{"/help", "/plan", "/yolo", "/default", "/sessions", "/permissions", "/agents", "/ready", "/verbose", "/fork", "/quit"}
	m := sized(t, 80, 24)
	g := m.geo
	composer := flat(strings.Join(screen(m)[g.composerY():g.composerY()+g.composerRows+g.infoRows], " "))
	for _, c := range all {
		if !strings.Contains(placeholder, c) || !strings.Contains(composer, c) {
			t.Fatalf("placeholder lacks %s: %q", c, composer)
		}
	}
	for _, c := range all {
		m := sized(t, 100, 24)
		cmd := submit(m, c)
		if m.alert {
			t.Fatalf("%s: %s", c, m.status)
		}
		switch c {
		case "/quit":
			if !isQuit(cmd) {
				t.Fatal("/quit should quit")
			}
		case "/yolo":
			if m.deps.Mode != perms.ModeYolo || !strings.Contains(m.status, "destructive gate still blocks") {
				t.Fatal(m.status)
			}
		case "/ready":
			if !strings.Contains(m.status, "does not approve") {
				t.Fatal(m.status)
			}
		case "/verbose":
			if !m.verbose || !strings.Contains(m.status, "turn and risk") {
				t.Fatal(m.status)
			}
		}
	}
	m = sized(t, 100, 24)
	submit(m, "/nope")
	if !m.alert || !strings.Contains(m.status, "unknown command /nope") {
		t.Fatal(m.status)
	}
}

func TestClipAndWrapCutOnRunes(t *testing.T) {
	s := "héllo wörld — 日本語のテキスト 👩‍💻 done"
	for n := 0; n <= ansi.StringWidth(s)+2; n++ {
		got := clip(s, n)
		if !utf8.ValidString(got) || ansi.StringWidth(got) > n {
			t.Fatalf("clip(%d) = %q (%d cells)", n, got, ansi.StringWidth(got))
		}
	}
	if clip("a\nb", 10) != "a b" {
		t.Fatal(clip("a\nb", 10))
	}
	for _, w := range []int{3, 5, 7, 11} {
		for _, l := range strings.Split(wrapText(s, w), "\n") {
			if !utf8.ValidString(l) || ansi.StringWidth(l) > w {
				t.Fatalf("wrap(%d) row %q", w, l)
			}
		}
	}
	if got := clipLeft("/home/rock/projects/very/long/path", 12); ansi.StringWidth(got) != 12 || !strings.HasSuffix(got, "long/path") {
		t.Fatal(got)
	}
}

// stubLLM is an OpenAI-compatible server that asks for one edit, then
// answers in markdown once it sees the tool result.
func stubLLM() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		last := req.Messages[len(req.Messages)-1]
		msg := map[string]any{"role": "assistant", "content": "", "tool_calls": []any{map[string]any{
			"id": "call_1", "type": "function",
			"function": map[string]any{"name": "edit_file", "arguments": `{"path":"greeting.txt","old":"helo wrold","new":"hello world"}`},
		}}}
		if last.Role == "tool" {
			text := "## Edited\n\nChanged `greeting.txt`."
			if strings.HasPrefix(last.Content, "denied") {
				text = "## Skipped\n\nYou denied the edit, so `greeting.txt` is unchanged."
			}
			msg = map[string]any{"role": "assistant", "content": text}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": msg}}})
	})
}

func runCmd(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	if batch, ok := cmd().(tea.BatchMsg); ok {
		for _, c := range batch {
			go runCmd(c)
		}
	}
}

func TestDecisionFromTheDialogLandsInTheTranscript(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	srv := httptest.NewServer(stubLLM())
	defer srv.Close()
	for _, tc := range []struct {
		key      rune
		decision perms.Decision
		file     string
		reply    string
	}{
		{'n', perms.Deny, "helo wrold\n", "Skipped"},
		{'y', perms.Allow, "hello world\n", "Edited"},
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "greeting.txt"), []byte("helo wrold\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		sess, err := session.Create(dir, "", "stub")
		if err != nil {
			t.Fatal(err)
		}
		policy := perms.Policy{Mode: perms.ModeDefault, Allow: []string{"read_file", "grep", "glob"}, Ask: []string{"edit_file", "write_file", "shell", "spawn_subagent"}}
		m := New(Deps{
			CWD: dir, Session: sess, Mode: perms.ModeDefault, JevMode: "offline",
			Provider: "openai", FastModel: "stub-fast", StrongModel: "stub-strong",
			Run: func(ctx context.Context, s *session.Session, prompt string, ask harness.AskFunc, sink func(harness.Event)) error {
				h := harness.New(harness.Options{
					Provider:  &provider.OpenAI{BaseURL: srv.URL + "/v1", APIKey: "stub"},
					FastModel: "stub-fast", StrongModel: "stub-strong",
					Policy: policy, Gates: jev.Gates{}, MaxSteps: 4, Ask: ask,
					Tools: tools.New(tools.Env{Root: dir, PlanPath: s.PlanPath()}),
				})
				return h.Run(ctx, s, prompt, sink)
			},
		})
		msgs := make(chan tea.Msg, 64)
		m.Send(func(msg tea.Msg) { msgs <- msg })
		m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
		go runCmd(submit(m, "fix the greeting typo"))
		asked := false
		timeout := time.After(10 * time.Second)
	turn:
		for {
			select {
			case msg := <-msgs:
				m.Update(msg)
				switch msg.(type) {
				case askMsg:
					asked = true
					view := strings.Join(assertFrame(t, m, 120, 36), "\n")
					if !strings.Contains(view, "Allow edit_file?") || !strings.Contains(view, "- helo wrold") {
						t.Fatalf("dialog:\n%s", view)
					}
					m.Update(tea.KeyPressMsg{Code: tc.key, Text: string(tc.key)})
				case turnDone:
					break turn
				}
			case <-timeout:
				t.Fatal("turn did not finish")
			}
		}
		if !asked {
			t.Fatal("edit_file should ask in default mode")
		}
		text := transcriptText(m)
		for _, want := range []string{"fix the greeting typo", "greeting.txt", "edit_file " + string(tc.decision) + " you answered " + string(tc.decision), tc.reply} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s: transcript lacks %q:\n%s", tc.decision, want, strings.Join(screen(m), "\n"))
			}
		}
		if got, _ := os.ReadFile(filepath.Join(dir, "greeting.txt")); string(got) != tc.file {
			t.Fatalf("%s: file is %q", tc.decision, got)
		}
		if m.busy || m.ctxBytes == 0 || m.target == 0 {
			t.Fatalf("after the turn: busy %v ctx %d target %v", m.busy, m.ctxBytes, m.target)
		}
	}
}

func TestTranscriptHasNoSpeakerGutter(t *testing.T) {
	m := sized(t, 100, 24)
	m.Update(eventMsg{harness.Event{Kind: harness.EvAssistant, Text: "hello from the block"}})
	submit(m, "pin this prompt")
	text := transcriptText(m)
	if strings.Contains(text, "you pin") || strings.Contains(text, "rock hello") {
		t.Fatalf("speaker gutter still painted: %q", text)
	}
	if !strings.Contains(text, "❯") || !strings.Contains(text, "pin this prompt") {
		t.Fatalf("user turn should be a ❯ band: %q", text)
	}
	if !strings.Contains(text, "hello from the block") {
		t.Fatalf("assistant text missing: %q", text)
	}
	if !strings.Contains(transcriptText(sized(t, 80, 24)), readyText) {
		t.Fatal("empty state should invite the first prompt")
	}
}

func TestClickSelectsAndFoldsAUserPrompt(t *testing.T) {
	m := sized(t, 80, 24)
	long := "one\ntwo\nthree\nfour"
	m.Update(eventMsg{harness.Event{Kind: harness.EvAssistant, Text: "ack"}})
	m.lines = append(m.lines, line{kind: "user", text: long})
	m.follow = true
	m.syncView()
	g := m.geo
	from := len(m.lines) - 1
	if !m.foldableAt(from) {
		t.Fatal("a four-line user prompt should be foldable")
	}
	y := g.vpY
	for _, s := range m.spans {
		if s.from == from {
			y = g.vpY + s.y0 - m.vp.YOffset()
			break
		}
	}
	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: g.padL + 2, Y: y})
	if m.selected != from {
		t.Fatalf("click should select the user block, got %d", m.selected)
	}
	if m.isFolded(from) {
		t.Fatal("first click selects; it should not fold")
	}
	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: g.padL + 2, Y: y})
	if !m.isFolded(from) {
		t.Fatal("second click on the selection should fold")
	}
	if !strings.Contains(transcriptText(m), "›") {
		t.Fatalf("folded row should show ›: %q", transcriptText(m))
	}
}

func TestVerbGroupReadsConsecutiveFiles(t *testing.T) {
	m := sized(t, 100, 24)
	for _, path := range []string{"alpha.go", "beta.go", "gamma.go"} {
		m.Update(eventMsg{harness.Event{Kind: harness.EvToolCall, Name: "read_file", Text: `{"path":"` + path + `"}`}})
		m.Update(eventMsg{harness.Event{Kind: harness.EvToolResult, Name: "read_file", Text: "ok " + path}})
	}
	text := transcriptText(m)
	if !strings.Contains(text, "Read 3 files") {
		t.Fatalf("expected a verb group: %q", text)
	}
	if strings.Contains(text, "alpha.go") {
		t.Fatalf("folded group should hide member paths: %q", text)
	}
	g := m.geo
	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: g.padL + 2, Y: g.vpY})
	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: g.padL + 2, Y: g.vpY})
	if opened := transcriptText(m); !strings.Contains(opened, "alpha.go") || !strings.Contains(opened, "gamma.go") {
		t.Fatalf("expanded group should list the files: %q", opened)
	}
	m = sized(t, 100, 24)
	m.Update(eventMsg{harness.Event{Kind: harness.EvToolCall, Name: "read_file", Text: `{"path":"solo.go"}`}})
	if got := transcriptText(m); strings.Contains(got, "Read 1") || !strings.Contains(got, "solo.go") {
		t.Fatalf("a single read stays a row: %q", got)
	}
}

func TestStickyLastUserPrompt(t *testing.T) {
	m := sized(t, 80, 20)
	submit(m, "keep this prompt in view")
	for i := range 40 {
		m.Update(eventMsg{harness.Event{Kind: harness.EvAssistant, Text: fmt.Sprintf("line %d", i)}})
	}
	if !m.vp.AtBottom() {
		t.Fatal("should follow the tail")
	}
	g := m.geo
	top := flat(ansi.Cut(screen(m)[g.vpY], g.padL, g.padL+g.transcriptW))
	if !strings.Contains(top, "keep this prompt in view") {
		t.Fatalf("scrolled-past user prompt should stick: %q\n%s", top, strings.Join(screen(m), "\n"))
	}
	if h, from := m.stickyUser(); h == 0 || from < 0 {
		t.Fatal("stickyUser should report the pinned prompt")
	}
}

func TestToolCallMergesResultIntoOneBlock(t *testing.T) {
	m := sized(t, 100, 24)
	m.Update(eventMsg{harness.Event{Kind: harness.EvToolCall, Name: "edit_file", Text: `{"path":"greeting.txt","old":"helo wrold","new":"hello world"}`}})
	m.Update(eventMsg{harness.Event{Kind: harness.EvToolResult, Name: "edit_file", Text: "wrote 1 line"}})
	text := transcriptText(m)
	if !strings.Contains(text, "Edit") || !strings.Contains(text, "greeting.txt") {
		t.Fatalf("edit header: %q", text)
	}
	if strings.Contains(text, "▸") || strings.Contains(text, "✓ edit_file") {
		t.Fatalf("call and result should be one ◆ block: %q", text)
	}
	if !strings.Contains(text, "- helo wrold") || !strings.Contains(text, "+ hello world") || !strings.Contains(text, "…") {
		t.Fatalf("expanded edit should show the hunk: %q", text)
	}
	if n := countTools(m); n != 1 {
		t.Fatalf("want 1 tool line, got %d", n)
	}
}

func TestReadFoldsByDefaultAndShellShowsTruncatedOutput(t *testing.T) {
	m := sized(t, 100, 24)
	m.Update(eventMsg{harness.Event{Kind: harness.EvToolCall, Name: "read_file", Text: `{"path":"solo.go"}`}})
	m.Update(eventMsg{harness.Event{Kind: harness.EvToolResult, Name: "read_file", Text: "package main\n"}})
	if got := transcriptText(m); !strings.Contains(got, "Read") || !strings.Contains(got, "solo.go") || strings.Contains(got, "package main") {
		t.Fatalf("read should fold to the header: %q", got)
	}

	m = sized(t, 100, 30)
	var out strings.Builder
	for i := 1; i <= 8; i++ {
		fmt.Fprintf(&out, "line-%d\n", i)
	}
	m.Update(eventMsg{harness.Event{Kind: harness.EvToolCall, Name: "shell", Text: `{"command":"seq 8"}`}})
	m.Update(eventMsg{harness.Event{Kind: harness.EvToolResult, Name: "shell", Text: out.String()}})
	got := transcriptText(m)
	if !strings.Contains(got, "$ seq 8") {
		t.Fatalf("shell header: %q", got)
	}
	if !strings.Contains(got, "line-1") || !strings.Contains(got, "line-2") || !strings.Contains(got, "line-8") {
		t.Fatalf("shell should keep first 2 and last 3: %q", got)
	}
	if strings.Contains(got, "line-3") || strings.Contains(got, "line-4") || strings.Contains(got, "line-5") {
		t.Fatalf("middle shell lines should collapse: %q", got)
	}
}

func TestRunningToolPaintsAnAttentionBar(t *testing.T) {
	m := sized(t, 100, 24)
	m.busy = true
	m.Update(eventMsg{harness.Event{Kind: harness.EvToolCall, Name: "shell", Text: `{"command":"sleep 1"}`}})
	if !m.toolRunning(0) && !m.toolRunning(len(m.lines)-1) {
		t.Fatal("open shell while busy should count as running")
	}
	raw := m.View().Content
	if !strings.Contains(raw, "38;2;242;183;5") && !strings.Contains(ansi.Strip(raw), "▎") {
		t.Fatalf("running tool should use the attention accent:\n%s", ansi.Strip(raw))
	}
	if !strings.Contains(transcriptText(m), "sleep 1") {
		t.Fatal(transcriptText(m))
	}
}

func countTools(m *Model) int {
	n := 0
	for _, ln := range m.lines {
		if ln.kind == "tool" {
			n++
		}
	}
	return n
}

func TestTabMovesFocusAndSpaceReturns(t *testing.T) {
	m := sized(t, 100, 24)
	m.input.Focus()
	m.Update(eventMsg{harness.Event{Kind: harness.EvAssistant, Text: "hello from focus"}})
	if !m.input.Focused() || m.focusTranscript {
		t.Fatal("composer starts focused")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Text: "tab"})
	if !m.focusTranscript || m.input.Focused() {
		t.Fatal("tab should move focus to the transcript")
	}
	if m.selected < 0 {
		t.Fatal("transcript focus should select a block")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: "space"})
	if m.focusTranscript || !m.input.Focused() {
		t.Fatal("space should return to the composer")
	}
}

func TestShiftTabCyclesModesAndCtrlOTogglesYolo(t *testing.T) {
	m := sized(t, 100, 24)
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Text: "tab", Mod: tea.ModShift})
	// Some Bubble Tea builds report shift+tab as Text "shift+tab".
	if m.deps.Mode != perms.ModePlan {
		m.Update(tea.KeyPressMsg{Text: "shift+tab"})
	}
	if m.deps.Mode != perms.ModePlan {
		t.Fatalf("shift+tab should enter plan mode, got %s", m.deps.Mode)
	}
	m.Update(tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})
	if m.deps.Mode != perms.ModeYolo {
		t.Fatalf("ctrl+o should toggle yolo, got %s", m.deps.Mode)
	}
	m.Update(tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})
	if m.deps.Mode != perms.ModeDefault {
		t.Fatalf("ctrl+o again should leave yolo, got %s", m.deps.Mode)
	}
}

func TestCtrlSOpensSessionsAndCtrlPOpensPalette(t *testing.T) {
	m := sized(t, 100, 24)
	m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if m.overlay != sessionsOverlay {
		t.Fatalf("ctrl+s overlay %d", m.overlay)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	if m.overlay != paletteOverlay {
		t.Fatalf("ctrl+p overlay %d", m.overlay)
	}
	view := flat(strings.Join(screen(m), "\n"))
	if !strings.Contains(view, "Commands") || !strings.Contains(view, "/plan") {
		t.Fatalf("palette:\n%s", view)
	}
	// enter the first item (/help)
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	if m.overlay != helpOverlay {
		t.Fatalf("palette /help should open help, got %d", m.overlay)
	}
}

func TestEscCancelsARunningTurnAndKeepsTheDraft(t *testing.T) {
	m := sized(t, 100, 24)
	m.input.SetValue("keep me")
	cancelled := false
	m.busy = true
	m.cancel = func() { cancelled = true }
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !cancelled || !strings.Contains(m.status, "cancelled") {
		t.Fatalf("esc should cancel the turn: busy=%v status=%s", m.busy, m.status)
	}
	if m.input.Value() != "keep me" {
		t.Fatalf("draft should stay: %q", m.input.Value())
	}
}

func TestDoubleCtrlCQuitsWhenIdleAndEmpty(t *testing.T) {
	m := sized(t, 100, 24)
	m.input.SetValue("draft")
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); isQuit(cmd) || m.input.Value() != "" {
		t.Fatal("first ctrl+c should clear the draft")
	}
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); !isQuit(cmd) {
		t.Fatal("second ctrl+c should quit")
	}
}

func TestArrowKeysFoldTheSelectedBlock(t *testing.T) {
	m := sized(t, 80, 24)
	m.lines = append(m.lines, line{kind: "user", text: "one\ntwo\nthree\nfour"})
	m.syncView()
	from := len(m.lines) - 1
	m.setSelected(from)
	m.focusTranscript = true
	if !m.foldableAt(from) || m.isFolded(from) {
		t.Fatal("four-line user prompt should start expanded and foldable")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Text: "left"})
	if !m.isFolded(from) {
		t.Fatal("left should fold the selected block")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight, Text: "right"})
	if m.isFolded(from) {
		t.Fatal("right should expand it again")
	}
}
