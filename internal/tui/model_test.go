package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/StephenSHorton/rock/internal/harness"
	"github.com/StephenSHorton/rock/internal/jev"
	"github.com/StephenSHorton/rock/internal/perms"
	"github.com/StephenSHorton/rock/internal/session"
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
	if !strings.Contains(view, "rock") || !strings.Contains(view, "jev:offline") {
		t.Fatalf("view:\n%s", view)
	}
	if !strings.Contains(view, "ready") {
		t.Fatal(view)
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
