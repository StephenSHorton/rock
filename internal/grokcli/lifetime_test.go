package grokcli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/StephenSHorton/rock/internal/provider"
)

func TestPromptLongerThanOldEightSecondBudget(t *testing.T) {
	fake := &FakeScript{
		Models:       realGrokModels(),
		CurrentModel: "grok-4.7-build-fast",
		StrictACP:    true,
		PromptDelay:  9 * time.Second,
		Reply:        "pong",
	}
	p := &Provider{CWD: t.TempDir(), Start: StartFake(fake), PromptIdle: time.Minute}
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	start := time.Now()
	msg, err := p.Complete(ctx, "gpt-4o-mini", []provider.Message{
		{Role: provider.RoleUser, Content: "Reply with just the word pong."},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "pong" {
		t.Fatalf("content %q", msg.Content)
	}
	if time.Since(start) < 8*time.Second {
		t.Fatalf("expected >8s delay path, took %s", time.Since(start))
	}
}

func TestCancelMidPromptSendsSessionCancelKeepsChild(t *testing.T) {
	fake := &FakeScript{
		Models:       realGrokModels(),
		CurrentModel: "grok-4.7-build-fast",
		StrictACP:    true,
		PromptDelay:  3 * time.Second,
		Reply:        "late",
	}
	p := &Provider{CWD: t.TempDir(), Start: StartFake(fake), PromptIdle: time.Minute}
	defer p.Close()
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := p.Complete(ctx, "", []provider.Message{
			{Role: provider.RoleUser, Content: "hi"},
		}, nil)
		errCh <- err
	}()
	time.Sleep(200 * time.Millisecond)
	cancel()
	err := <-errCh
	if err == nil || !strings.Contains(err.Error(), "grok stopped") {
		t.Fatalf("want grok stopped, got %v", err)
	}
	// Give cancel notification a moment to land.
	time.Sleep(50 * time.Millisecond)
	if fake.Cancels() < 1 {
		t.Fatalf("expected session/cancel, got %d", fake.Cancels())
	}
	starts := p.StartCount()
	// Next turn reuses the same child.
	fake.PromptDelay = 0
	fake.Reply = "ok"
	if _, err := p.Complete(context.Background(), "", []provider.Message{
		{Role: provider.RoleUser, Content: "again"},
	}, nil); err != nil {
		t.Fatal(err)
	}
	if p.StartCount() != starts {
		t.Fatalf("child respawned: before=%d after=%d", starts, p.StartCount())
	}
}

func TestSequentialCompleteReusesChild(t *testing.T) {
	fake := &FakeScript{
		Models:              realGrokModels(),
		CurrentModel:        "grok-4.7-build-fast",
		ConfigOptionsModels: true,
		StrictACP:           true,
		Reply:               "a",
	}
	p := &Provider{CWD: t.TempDir(), Start: StartFake(fake)}
	defer p.Close()
	if _, _, err := p.Models(context.Background()); err != nil {
		t.Fatal(err)
	}
	pid1 := p.ChildPID()
	starts := p.StartCount()
	if starts != 1 {
		t.Fatalf("starts=%d", starts)
	}
	fake.Reply = "b"
	if _, err := p.Complete(context.Background(), "gpt-4o-mini", []provider.Message{
		{Role: provider.RoleUser, Content: "one"},
	}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Complete(context.Background(), "gpt-4o-mini", []provider.Message{
		{Role: provider.RoleUser, Content: "one"},
		{Role: provider.RoleAssistant, Content: "b"},
		{Role: provider.RoleUser, Content: "two"},
	}, nil); err != nil {
		t.Fatal(err)
	}
	if p.StartCount() != 1 || p.ChildPID() != pid1 {
		t.Fatalf("reuse failed starts=%d pid %d→%d", p.StartCount(), pid1, p.ChildPID())
	}
	// Second prompt should be delta-only (no SuperGrok preamble).
	fake.mu.Lock()
	raw := string(append([]byte(nil), fake.promptSeen...))
	fake.mu.Unlock()
	if strings.Contains(raw, "SuperGrok model backend") {
		t.Fatalf("live session resent full preamble: %s", raw[:min(200, len(raw))])
	}
	if !strings.Contains(raw, "two") {
		t.Fatalf("delta missing new user turn: %s", raw)
	}
}

func TestModelsThenCompleteDoesNotKillChild(t *testing.T) {
	// Regression: FastModel()/Models used to wrap dial in CommandContext(8s)
	// and cancel after return, killing the cached child before Complete.
	fake := &FakeScript{
		Models:       realGrokModels(),
		CurrentModel: "grok-4.7-build-fast",
		StrictACP:    true,
		Reply:        "pong",
	}
	p := &Provider{CWD: t.TempDir(), Start: StartFake(fake)}
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	_, _, err := p.Models(ctx)
	cancel() // would have killed CommandContext child
	if err != nil {
		t.Fatal(err)
	}
	msg, err := p.Complete(context.Background(), "gpt-4o-mini", []provider.Message{
		{Role: provider.RoleUser, Content: "pong?"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "pong" {
		t.Fatal(msg.Content)
	}
	if p.StartCount() != 1 {
		t.Fatalf("starts=%d want 1", p.StartCount())
	}
}

func TestDebugGrokLog(t *testing.T) {
	home := t.TempDir()
	t.Setenv("ROCK_HOME", home)
	t.Setenv("ROCK_DEBUG_GROK", "1")
	fake := &FakeScript{
		Models:       realGrokModels(),
		CurrentModel: "grok-4.7-build-fast",
		Reply:        "x",
	}
	p := &Provider{CWD: t.TempDir(), Start: StartFake(fake)}
	defer p.Close()
	if _, err := p.Complete(context.Background(), "", []provider.Message{
		{Role: provider.RoleUser, Content: "hi"},
	}, nil); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(home, "rock.log"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "send initialize") || !strings.Contains(s, "recv") {
		t.Fatalf("log missing traffic: %s", s)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
