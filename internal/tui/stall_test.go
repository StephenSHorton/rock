package tui

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/StephenSHorton/rock/internal/grokcli"
	"github.com/StephenSHorton/rock/internal/harness"
	"github.com/StephenSHorton/rock/internal/session"
	"github.com/StephenSHorton/rock/internal/trace"
)

func TestStallWatchdogDumpsGoroutines(t *testing.T) {
	oldAfter, oldTick := stallAfter, stallTick
	stallAfter, stallTick = 150*time.Millisecond, 20*time.Millisecond
	defer func() { stallAfter, stallTick = oldAfter, oldTick }()
	log := &logRec{}
	m := testModel(t)
	m.deps.Log = log.fn
	m.deps.Run = func(ctx context.Context, _ *session.Session, _ string, _ harness.AskFunc, _ func(harness.Event)) error {
		trace.Info("grok prompt sent", "chars", 3)
		<-ctx.Done() // a hung provider
		return ctx.Err()
	}
	msgs := make(chan tea.Msg, 64)
	m.Send(func(msg tea.Msg) { msgs <- msg })
	go runCmd(submit(m, "Hey"))
	timeout := time.After(5 * time.Second)
	for stalled := false; !stalled; {
		select {
		case msg := <-msgs:
			m.Update(msg)
			_, stalled = msg.(stallMsg)
		case <-timeout:
			t.Fatal("watchdog never fired")
		}
	}
	if !log.has("warn tui turn stall") || !log.has("stage grok prompt sent") {
		t.Fatalf("stall line: %v", log.lines)
	}
	if !log.has("tui goroutines stacks goroutine ") || !log.has("watchTurn") {
		t.Fatal("goroutine dump missing")
	}
	if status := flat(strings.Join(screen(m), "\n")); !strings.Contains(status, "no progress for") {
		t.Fatalf("stall not visible:\n%s", strings.Join(screen(m), "\n"))
	}
	m.cancelTurn()
	for msg := range msgs {
		m.Update(msg)
		if _, ok := msg.(turnDone); ok {
			break
		}
	}
	if m.stall != "" || m.busy {
		t.Fatalf("stall %q busy %v after done", m.stall, m.busy)
	}
}

func TestNoStallWhileProgressing(t *testing.T) {
	oldAfter, oldTick := stallAfter, stallTick
	stallAfter, stallTick = 200*time.Millisecond, 20*time.Millisecond
	defer func() { stallAfter, stallTick = oldAfter, oldTick }()
	log := &logRec{}
	m := testModel(t)
	m.deps.Log = log.fn
	m.deps.Run = func(ctx context.Context, _ *session.Session, _ string, _ harness.AskFunc, sink func(harness.Event)) error {
		for i := 0; i < 8; i++ {
			time.Sleep(60 * time.Millisecond)
			trace.Mark("grok streaming")
		}
		sink(harness.Event{Kind: harness.EvAssistant, Text: "done"})
		return nil
	}
	pumpTurn(t, m, "Hey")
	if log.has("tui turn stall") {
		t.Fatalf("false stall: %v", log.lines)
	}
}

// Mirrors startTUI: Models() spawns the child before the program starts,
// then the first turn must reuse it, and a picker-style Models() call in
// the middle of a turn must not deadlock with Complete.
func TestModelsAtStartupThenSubmitAndModelsDuringTurn(t *testing.T) {
	log := &logRec{}
	trace.SetLogger(log.fn)
	defer trace.SetLogger(nil)
	fake := &grokcli.FakeScript{
		Reply:               "pong",
		Models:              []grokcli.ModelInfo{{ID: "grok-4.7", Name: "Grok 4.7"}, {ID: "grok-4.7-build-fast", Name: "Grok 4.7 Fast"}},
		CurrentModel:        "grok-4.7-build-fast",
		ConfigOptionsModels: true, SessionModels: true, StrictACP: true,
		PromptDelay: 300 * time.Millisecond,
	}
	m, p := grokModel(t, fake, log)
	if _, models, err := p.Models(context.Background()); err != nil || len(models) != 2 {
		t.Fatalf("startup models: %v %v", models, err)
	}
	if p.StartCount() != 1 {
		t.Fatalf("starts %d", p.StartCount())
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(100 * time.Millisecond)
		for i := 0; i < 5; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			if _, _, err := p.Models(ctx); err != nil {
				t.Errorf("models during turn: %v", err)
			}
			cancel()
		}
	}()
	pumpTurn(t, m, "Hey")
	wg.Wait()
	if !strings.Contains(transcriptText(m), "pong") {
		t.Fatalf("reply missing:\n%s", strings.Join(screen(m), "\n"))
	}
	if p.StartCount() != 1 {
		t.Fatalf("turn respawned grok: starts=%d", p.StartCount())
	}
	want := []string{
		"info turn jev route start", "info turn jev route done", "info turn context start",
		"info turn repo map", "info turn context done", "info turn model call start",
		"info grok ensure start", "info grok ensure done", "info grok prompt sent",
		"info grok first update", "info grok prompt done", "info turn model call done",
	}
	log.mu.Lock()
	lines := append([]string(nil), log.lines...)
	log.mu.Unlock()
	i := 0
	for _, l := range lines {
		if i < len(want) && strings.HasPrefix(l, want[i]) {
			i++
		}
	}
	if i != len(want) {
		t.Fatalf("stage lines out of order or missing (got through %d/%d, next %q):\n%s", i, len(want), want[min(i, len(want)-1)], strings.Join(lines, "\n"))
	}
	for _, l := range lines {
		if strings.Contains(l, "Hey") || strings.Contains(l, "pong") {
			t.Fatalf("prompt/reply text in log: %s", l)
		}
	}
}
