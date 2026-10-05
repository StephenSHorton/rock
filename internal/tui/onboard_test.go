package tui

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/StephenSHorton/rock/internal/harness"
	"github.com/StephenSHorton/rock/internal/jev"
	"github.com/StephenSHorton/rock/internal/perms"
	"github.com/StephenSHorton/rock/internal/session"
)

func feed(m *Model, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if msg == nil {
		return
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			feed(m, c)
		}
		return
	}
	m.Update(msg)
}

func TestOnboardBlocksAgentUntilJevValidates(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	sess, err := session.Create(t.TempDir(), "", "gate")
	if err != nil {
		t.Fatal(err)
	}
	var runs atomic.Int32
	var saved string
	checkOK := false
	m := New(Deps{
		CWD:           sess.Meta.CWD,
		Session:       sess,
		Mode:          perms.ModeDefault,
		JevMode:       "offline",
		Gates:         jev.Gates{Client: &jev.Client{}},
		InitialPrompt: "do the work",
		JevGate:       true,
		CheckJev: func(ctx context.Context, key string) error {
			if !checkOK {
				return errors.New("jev http 401: invalid key")
			}
			return nil
		},
		SaveJev: func(key string) (string, error) {
			saved = key
			return "OS keychain", nil
		},
		Run: func(ctx context.Context, sess *session.Session, prompt string, ask harness.AskFunc, sink func(harness.Event)) error {
			runs.Add(1)
			sink(harness.Event{Kind: harness.EvAssistant, Text: "started: " + prompt})
			sink(harness.Event{Kind: harness.EvDone, Text: "end_turn"})
			return nil
		},
	})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	_ = m.Init()
	if runs.Load() != 0 {
		t.Fatal("agent loop started before Jev validated")
	}
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "System One") || !strings.Contains(view, "Jev API key") {
		t.Fatalf("onboard:\n%s", view)
	}
	if strings.Contains(view, "Ask Rock") {
		t.Fatal("composer must not show while gating")
	}

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	feed(m, cmd)
	if m.gate == nil || m.gate.phase != gateError {
		t.Fatal("empty key should error")
	}
	if runs.Load() != 0 {
		t.Fatal("empty enter started the agent")
	}

	m.gate.input.SetValue("bad-key")
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	feed(m, cmd)
	if m.gate.phase != gateError || !strings.Contains(m.gate.err, "rejected") {
		t.Fatalf("phase=%v err=%q", m.gate.phase, m.gate.err)
	}
	if runs.Load() != 0 {
		t.Fatal("invalid key started the agent")
	}
	view = ansi.Strip(m.View().Content)
	if !strings.Contains(view, "rejected") {
		t.Fatalf("error state:\n%s", view)
	}

	checkOK = true
	m.gate.input.SetValue("good-key")
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	feed(m, cmd)
	if m.gate.phase != gateSuccess || m.gate.store != "OS keychain" {
		t.Fatalf("phase=%v store=%q", m.gate.phase, m.gate.store)
	}
	if saved != "good-key" {
		t.Fatalf("saved %q", saved)
	}
	if runs.Load() != 0 {
		t.Fatal("success hold must not start the agent yet")
	}
	view = ansi.Strip(m.View().Content)
	if !strings.Contains(view, "OS keychain") {
		t.Fatalf("success:\n%s", view)
	}

	_, cmd = m.Update(jevGateClear{})
	feed(m, cmd)
	if m.gating() {
		t.Fatal("gate should clear after success")
	}
	if runs.Load() != 1 {
		t.Fatalf("agent should start after validation, runs=%d", runs.Load())
	}
}

func TestOnboardValidatingState(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	sess, err := session.Create(t.TempDir(), "", "gate")
	if err != nil {
		t.Fatal(err)
	}
	block := make(chan struct{})
	m := New(Deps{
		CWD:     sess.Meta.CWD,
		Session: sess,
		JevGate: true,
		CheckJev: func(ctx context.Context, key string) error {
			<-block
			return nil
		},
		SaveJev: func(string) (string, error) { return "memory", nil },
	})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.gate.input.SetValue("slow-key")
	cmd := m.beginJevCheck()
	if m.gate.phase != gateValidating {
		t.Fatalf("phase %v", m.gate.phase)
	}
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "Checking this key") && !strings.Contains(view, "Talking to Jev") {
		t.Fatalf("validating:\n%s", view)
	}
	close(block)
	feed(m, cmd)
	if m.gate.phase != gateSuccess {
		t.Fatalf("phase %v", m.gate.phase)
	}
}

func TestOnboardAcceptsPastedKey(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	sess, err := session.Create(t.TempDir(), "", "gate-paste")
	if err != nil {
		t.Fatal(err)
	}
	var checked, saved string
	m := New(Deps{
		CWD:     sess.Meta.CWD,
		Session: sess,
		Mode:    perms.ModeDefault,
		JevMode: "offline",
		Gates:   jev.Gates{Client: &jev.Client{}},
		JevGate: true,
		CheckJev: func(ctx context.Context, key string) error {
			checked = key
			return nil
		},
		SaveJev: func(key string) (string, error) {
			saved = key
			return "OS keychain", nil
		},
	})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	_ = m.Init()
	m.Update(tea.PasteMsg{Content: "  jev_live_abc123\r\n"})
	if got := m.gate.input.Value(); got != "jev_live_abc123" {
		t.Fatalf("pasted value = %q", got)
	}
	view := ansi.Strip(m.View().Content)
	if strings.Contains(view, "jev_live_abc123") {
		t.Fatal("pasted key must stay masked")
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	feed(m, cmd)
	if checked != "jev_live_abc123" || saved != "jev_live_abc123" {
		t.Fatalf("checked=%q saved=%q", checked, saved)
	}
}
