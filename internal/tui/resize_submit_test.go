package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/StephenSHorton/rock/internal/grokcli"
	"github.com/StephenSHorton/rock/internal/harness"
	"github.com/StephenSHorton/rock/internal/jev"
	"github.com/StephenSHorton/rock/internal/perms"
	"github.com/StephenSHorton/rock/internal/session"
	"github.com/StephenSHorton/rock/internal/tools"
)

// composerBottomWidth is the drawn width of the composer's bottom border.
func composerBottomWidth(m *Model) int {
	for _, r := range screen(m) {
		if strings.Contains(r, "╰") && strings.Contains(r, "╯") {
			return ansi.StringWidth(strings.TrimRight(r, " "))
		}
	}
	return -1
}

func TestWindowSizeAppliesInEveryState(t *testing.T) {
	states := []struct {
		name string
		set  func(t *testing.T, m *Model)
	}{
		{"idle", func(*testing.T, *Model) {}},
		{"busy", func(_ *testing.T, m *Model) {
			m.busy, m.status, m.turnAt = true, "working", time.Now()
		}},
		{"help overlay", func(_ *testing.T, m *Model) { m.overlay = helpOverlay }},
		{"update overlay", func(_ *testing.T, m *Model) { m.overlay = updateOverlay }},
		{"slash picker", func(_ *testing.T, m *Model) {
			m.input.SetValue("/")
			m.Update(tea.KeyPressMsg{Code: 'm', Text: "m"})
		}},
		{"permission ask", func(_ *testing.T, m *Model) {
			m.openAsk(askMsg{tool: "shell", detail: "ls", reply: make(chan perms.Decision, 1)})
		}},
	}
	for _, st := range states {
		t.Run(st.name, func(t *testing.T) {
			m := sized(t, 100, 30)
			st.set(t, m)
			m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
			before := composerBottomWidth(m)
			m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
			if m.width != 140 || m.height != 40 {
				t.Fatalf("size not stored: %dx%d", m.width, m.height)
			}
			assertFrame(t, m, 140, 40)
			after := composerBottomWidth(m)
			if before > 0 && after-before != 40 {
				t.Fatalf("composer did not reflow: %d -> %d", before, after)
			}
			if w, h := m.seen.get(); w != 140 || h != 40 {
				t.Fatalf("poller baseline not updated: %dx%d", w, h)
			}
			// Shrink too.
			m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			assertFrame(t, m, 80, 24)
		})
	}
}

func TestWindowSizeAppliesWhileGating(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	sess, err := session.Create(t.TempDir(), "", "gate")
	if err != nil {
		t.Fatal(err)
	}
	m := New(Deps{
		CWD: sess.Meta.CWD, Session: sess, Mode: perms.ModeDefault, JevMode: "offline",
		Gates: jev.Gates{Client: &jev.Client{}}, JevGate: true,
		CheckJev: func(context.Context, string) error { return errors.New("no") },
		SaveJev:  func(string) (string, error) { return "", nil },
	})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if !m.gating() {
		t.Fatal("expected the Jev gate")
	}
	m.Update(tea.WindowSizeMsg{Width: 130, Height: 36})
	if m.width != 130 || m.height != 36 {
		t.Fatalf("gate dropped resize: %dx%d", m.width, m.height)
	}
	if rows := screen(m); len(rows) != 36 {
		t.Fatalf("onboard frame has %d rows, want 36", len(rows))
	}
}

func TestPollResizeSendsOnlyRealChanges(t *testing.T) {
	seen := &sizeSeen{}
	seen.set(100, 30)
	var mu sync.Mutex
	size := [2]int{100, 30}
	got := make(chan tea.WindowSizeMsg, 8)
	done := make(chan struct{})
	defer close(done)
	go pollResize(done, 5*time.Millisecond, seen, func() (int, int, error) {
		mu.Lock()
		defer mu.Unlock()
		return size[0], size[1], nil
	}, func(msg tea.Msg) {
		ws := msg.(tea.WindowSizeMsg)
		seen.set(ws.Width, ws.Height) // the model applies it
		got <- ws
	})
	select {
	case ws := <-got:
		t.Fatalf("unchanged size sent %v", ws)
	case <-time.After(40 * time.Millisecond):
	}
	mu.Lock()
	size = [2]int{140, 40}
	mu.Unlock()
	select {
	case ws := <-got:
		if ws.Width != 140 || ws.Height != 40 {
			t.Fatalf("got %v", ws)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("resize not detected")
	}
	select {
	case ws := <-got:
		t.Fatalf("duplicate send %v", ws)
	case <-time.After(40 * time.Millisecond):
	}
}

func TestPollResizeCorrectsAWrongNativeSize(t *testing.T) {
	// Legacy consoles can report the buffer height (e.g. 9001) on a
	// native resize. The poller must snap back to the window size.
	m := sized(t, 100, 30)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 9001})
	got := make(chan tea.Msg, 1)
	done := make(chan struct{})
	go pollResize(done, 5*time.Millisecond, m.seen, func() (int, int, error) { return 100, 30, nil }, func(msg tea.Msg) {
		select {
		case got <- msg:
		default:
		}
	})
	defer close(done)
	select {
	case msg := <-got:
		m.Update(msg)
	case <-time.After(2 * time.Second):
		t.Fatal("poller did not correct the size")
	}
	assertFrame(t, m, 100, 30)
}

type logRec struct {
	mu    sync.Mutex
	lines []string
}

func (l *logRec) fn(level, msg string, kv ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, strings.TrimSpace(level+" "+msg+" "+strings.TrimSuffix(fmt.Sprintln(kv...), "\n")))
}

func (l *logRec) has(sub string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, s := range l.lines {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// pumpTurn drives one submitted turn the way tea.Program would.
func pumpTurn(t *testing.T, m *Model, prompt string) {
	t.Helper()
	msgs := make(chan tea.Msg, 256)
	m.Send(func(msg tea.Msg) { msgs <- msg })
	go runCmd(submit(m, prompt))
	if !m.busy || m.status != "working" {
		t.Fatalf("no immediate feedback on submit: busy=%v status=%q", m.busy, m.status)
	}
	if !strings.Contains(transcriptText(m), prompt) {
		t.Fatalf("prompt not echoed:\n%s", strings.Join(screen(m), "\n"))
	}
	timeout := time.After(15 * time.Second)
	for {
		select {
		case msg := <-msgs:
			m.Update(msg)
			if _, ok := msg.(turnDone); ok {
				return
			}
		case <-timeout:
			t.Fatal("turn did not finish")
		}
	}
}

func grokModel(t *testing.T, fake *grokcli.FakeScript, log *logRec) (*Model, *grokcli.Provider) {
	t.Helper()
	t.Setenv("ROCK_HOME", t.TempDir())
	dir := t.TempDir()
	sess, err := session.Create(dir, "", "grok")
	if err != nil {
		t.Fatal(err)
	}
	p := &grokcli.Provider{CWD: dir, Start: grokcli.StartFake(fake)}
	t.Cleanup(p.Close)
	policy := perms.Policy{Mode: perms.ModeDefault}
	m := New(Deps{
		CWD: dir, Session: sess, Mode: perms.ModeDefault, JevMode: "offline",
		Provider: grokcli.AuthClass, Auth: "grok-cli", FastModel: "grok-4.7-build-fast",
		Log: log.fn,
		Run: func(ctx context.Context, s *session.Session, prompt string, ask harness.AskFunc, sink func(harness.Event)) error {
			h := harness.New(harness.Options{
				Provider: p, FastModel: "gpt-4o-mini", StrongModel: "gpt-4o",
				Policy: policy, Gates: jev.Gates{}, MaxSteps: 4, Ask: ask,
				Tools: tools.New(tools.Env{Root: dir, PlanPath: s.PlanPath()}),
			})
			return h.Run(ctx, s, prompt, sink)
		},
	})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m, p
}

func TestHeyThroughGrokCLIFakeShowsTheReply(t *testing.T) {
	log := &logRec{}
	fake := &grokcli.FakeScript{
		Reply:               "Hey Stephen! What are we building?",
		Models:              []grokcli.ModelInfo{{ID: "grok-4.7", Name: "Grok 4.7"}, {ID: "grok-4.7-build-fast", Name: "Grok 4.7 Fast"}},
		CurrentModel:        "grok-4.7-build-fast",
		ConfigOptionsModels: true, SessionModels: true, StrictACP: true,
		PromptDelay: 50 * time.Millisecond,
	}
	m, _ := grokModel(t, fake, log)
	pumpTurn(t, m, "Hey")
	text := transcriptText(m)
	if !strings.Contains(text, "Hey Stephen! What are we building?") {
		t.Fatalf("reply not rendered:\n%s", strings.Join(screen(m), "\n"))
	}
	if m.busy {
		t.Fatal("still busy after turnDone")
	}
	for _, want := range []string{"info tui turn start", "info tui turn done"} {
		if !log.has(want) {
			t.Fatalf("rock.log lacks %q: %v", want, log.lines)
		}
	}
	if log.has("Hey") {
		t.Fatalf("prompt text leaked into rock.log: %v", log.lines)
	}
}

func TestGrokCLIEmptyReplyIsVisible(t *testing.T) {
	log := &logRec{}
	m, _ := grokModel(t, &grokcli.FakeScript{Reply: ""}, log)
	pumpTurn(t, m, "Hey")
	text := flat(transcriptText(m))
	if !strings.Contains(text, "No reply") || !strings.Contains(text, "ROCK_DEBUG_GROK=1") {
		t.Fatalf("empty reply left no trace:\n%s", strings.Join(screen(m), "\n"))
	}
	if !log.has("warn tui turn empty") {
		t.Fatalf("log: %v", log.lines)
	}
}

func TestProviderErrorIsVisible(t *testing.T) {
	log := &logRec{}
	t.Setenv("ROCK_HOME", t.TempDir())
	m := testModel(t)
	m.deps.Log = log.fn
	m.deps.Run = func(ctx context.Context, s *session.Session, prompt string, ask harness.AskFunc, sink func(harness.Event)) error {
		err := errors.New("grok exited: exit status 1")
		sink(harness.Event{Kind: harness.EvStatus, Text: err.Error()})
		return err
	}
	pumpTurn(t, m, "Hey")
	if !strings.Contains(transcriptText(m), "grok exited: exit status 1") {
		t.Fatalf("error not in transcript:\n%s", strings.Join(screen(m), "\n"))
	}
	if !log.has("error tui turn error") || !log.has("grok exited") {
		t.Fatalf("log: %v", log.lines)
	}
}

func TestTurnPanicIsVisible(t *testing.T) {
	m := testModel(t)
	m.deps.Run = func(context.Context, *session.Session, string, harness.AskFunc, func(harness.Event)) error {
		panic("boom")
	}
	pumpTurn(t, m, "Hey")
	if !strings.Contains(transcriptText(m), "turn crashed: boom") || m.busy {
		t.Fatalf("panic not surfaced (busy=%v):\n%s", m.busy, strings.Join(screen(m), "\n"))
	}
}

func TestUserCancelStaysQuiet(t *testing.T) {
	m := testModel(t)
	started := make(chan struct{})
	m.deps.Run = func(ctx context.Context, _ *session.Session, _ string, _ harness.AskFunc, _ func(harness.Event)) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}
	msgs := make(chan tea.Msg, 16)
	m.Send(func(msg tea.Msg) { msgs <- msg })
	go runCmd(submit(m, "long task"))
	<-started
	m.cancelTurn()
	for msg := range msgs {
		m.Update(msg)
		if _, ok := msg.(turnDone); ok {
			break
		}
	}
	if strings.Contains(transcriptText(m), "No reply") {
		t.Fatalf("user cancel should not add an empty-reply error:\n%s", strings.Join(screen(m), "\n"))
	}
}
