package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/StephenSHorton/rock/internal/grokcli"
	"github.com/StephenSHorton/rock/internal/harness"
	"github.com/StephenSHorton/rock/internal/session"
)

func TestStreamVisibleHidesToolFences(t *testing.T) {
	for in, want := range map[string]string{
		"Hello": "Hello",
		"Reading it.\n```tool\n{\"name\":\"read_f": "Reading it.",
		"Reading it.\n```tool\n{}\n```\nDone.":     "Reading it.\n\nDone.",
		"Next ``":                                  "Next",
		"Next ```to":                               "Next",
		"```go\nfmt.Println()\n```":                "```go\nfmt.Println()\n```",
	} {
		if got := streamVisible(in); got != want {
			t.Errorf("streamVisible(%q) = %q, want %q", in, got, want)
		}
	}
}

// grokStreamModel builds a TUI whose turns go through the harness with
// streaming on, like cli.RunTurn.
func grokStreamModel(t *testing.T, fake *grokcli.FakeScript) *Model {
	t.Helper()
	m, p := grokModel(t, fake, &logRec{})
	inner := m.deps.Run
	_ = p
	m.deps.Run = func(ctx context.Context, s *session.Session, prompt string, ask harness.AskFunc, sink func(harness.Event)) error {
		return inner(ctx, s, prompt, ask, sink)
	}
	return m
}

func TestStreamingThoughtThenReplyRenders(t *testing.T) {
	fake := &grokcli.FakeScript{
		Thoughts:    []string{"The user greeted me. ", "I should greet back and ask what to build."},
		Reply:       "Hey Stephen! What are we building today?",
		ReplyChunks: 4,
		ChunkDelay:  40 * time.Millisecond,
	}
	m := grokStreamModel(t, fake)
	msgs := make(chan tea.Msg, 256)
	m.Send(func(msg tea.Msg) { msgs <- msg })
	go runCmd(submit(m, "Hey"))
	var sawThinking, sawPartial bool
	timeout := time.After(10 * time.Second)
loop:
	for {
		select {
		case msg := <-msgs:
			m.Update(msg)
			ev, ok := msg.(turnEvent)
			if ok && ev.ev.Kind == harness.EvThought && !sawThinking {
				text := flat(transcriptText(m))
				if strings.Contains(text, "Thinking…") && strings.Contains(text, "The user greeted me.") {
					sawThinking = true
				}
			}
			if ok && ev.ev.Kind == harness.EvDelta {
				text := flat(transcriptText(m))
				if strings.Contains(text, "Thought for") && !strings.Contains(text, "Thinking…") &&
					strings.Contains(text, "Hey") && !strings.Contains(text, "building today?") {
					sawPartial = true
				}
				if rows := screen(m); !strings.Contains(rows[m.geo.activityY()], "Responding…") {
					t.Fatalf("indicator should say Responding…: %q", rows[m.geo.activityY()])
				}
			}
			if _, done := msg.(turnDone); done {
				break loop
			}
		case <-timeout:
			t.Fatal("turn did not finish")
		}
	}
	if !sawThinking {
		t.Fatal("thoughts never streamed into the transcript")
	}
	if !sawPartial {
		t.Fatal("reply text never streamed (only appeared at the end)")
	}
	text := flat(transcriptText(m))
	if !strings.Contains(text, "Thought for") || !strings.Contains(text, "Hey Stephen! What are we building today?") {
		t.Fatalf("final:\n%s", strings.Join(screen(m), "\n"))
	}
	if strings.Contains(text, "I should greet back") {
		t.Fatal("finished thought should be collapsed")
	}
	if strings.Count(text, "What are we building today?") != 1 {
		t.Fatalf("reply duplicated:\n%s", text)
	}
	// Expand the thought with the existing fold toggle.
	for i, ln := range m.lines {
		if ln.kind == "thinking" {
			m.toggleFold(i)
		}
	}
	if text := flat(transcriptText(m)); !strings.Contains(text, "I should greet back and ask what to build.") {
		t.Fatalf("expanded thought:\n%s", text)
	}
	if rows := screen(m); m.geo.activityRows != 0 || strings.Contains(strings.Join(rows, "\n"), "esc to interrupt") {
		t.Fatal("indicator should be gone after the turn")
	}
}

func TestStreamingHidesToolFenceAndUsesFinalText(t *testing.T) {
	m := testModel(t)
	step := make(chan struct{})
	m.deps.Run = func(ctx context.Context, _ *session.Session, _ string, _ harness.AskFunc, sink func(harness.Event)) error {
		sink(harness.Event{Kind: harness.EvDelta, Text: "Let me read it.\n```tool\n{\"name\":\"read_file\",\"argu"})
		<-step
		sink(harness.Event{Kind: harness.EvDelta, Text: "ments\":{\"path\":\"a.go\"}}\n```"})
		sink(harness.Event{Kind: harness.EvAssistant, Text: "Let me read it."})
		sink(harness.Event{Kind: harness.EvToolCall, Name: "read_file", Text: `{"path":"a.go"}`})
		sink(harness.Event{Kind: harness.EvToolResult, Name: "read_file", Text: "package a"})
		sink(harness.Event{Kind: harness.EvDelta, Text: "```tool\n{\"name\":\"glob\"}\n```"}) // fence-only step
		sink(harness.Event{Kind: harness.EvToolCall, Name: "glob", Text: `{}`})
		sink(harness.Event{Kind: harness.EvToolResult, Name: "glob", Text: "a.go"})
		sink(harness.Event{Kind: harness.EvDelta, Text: "All good."})
		sink(harness.Event{Kind: harness.EvAssistant, Text: "All good."})
		sink(harness.Event{Kind: harness.EvDone, Text: "end_turn"})
		return nil
	}
	msgs := make(chan tea.Msg, 64)
	m.Send(func(msg tea.Msg) { msgs <- msg })
	go runCmd(submit(m, "check a.go"))
	m.Update(<-msgs) // first delta
	if text := transcriptText(m); strings.Contains(text, "read_file") || strings.Contains(text, "```") || !strings.Contains(text, "Let me read it.") {
		t.Fatalf("mid-stream:\n%s", text)
	}
	close(step)
	for msg := range msgs {
		m.Update(msg)
		if _, ok := msg.(turnDone); ok {
			break
		}
	}
	text := flat(transcriptText(m))
	if strings.Contains(text, "\"name\"") || strings.Contains(text, "```") {
		t.Fatalf("fence leaked:\n%s", text)
	}
	if strings.Count(text, "Let me read it.") != 1 || strings.Count(text, "All good.") != 1 {
		t.Fatalf("assistant text wrong:\n%s", text)
	}
	for _, ln := range m.lines {
		if ln.live {
			t.Fatalf("live line left behind: %+v", ln)
		}
	}
}

func TestIndicatorSitsAboveComposerOnTheLeft(t *testing.T) {
	m := sized(t, 100, 30)
	m.busy, m.turnAt = true, time.Now().Add(-4*time.Second)
	m.turnFrom = len(m.lines)
	m.layout()
	rows := screen(m)
	g := m.geo
	ind := rows[g.activityY()]
	if !strings.Contains(ind, "Thinking… 4s") || !strings.HasPrefix(strings.TrimLeft(ind, " "), m.spin.View()[:0]) {
		t.Fatalf("indicator row: %q", ind)
	}
	if idx := strings.Index(ind, "Thinking"); idx > 8 {
		t.Fatalf("indicator should be on the left: %q", ind)
	}
	if !strings.Contains(rows[g.composerY()], "╭") || g.composerY() != g.activityY()+1 {
		t.Fatalf("indicator must sit directly above the composer:\n%s", strings.Join(rows, "\n"))
	}
	if strings.Contains(rows[g.statusY()], "working") {
		t.Fatalf("status line still says working: %q", rows[g.statusY()])
	}
	// A running tool names itself.
	m.apply(harness.Event{Kind: harness.EvToolCall, Name: "read_file", Text: `{"path":"a.go"}`})
	m.layout()
	if ind := screen(m)[m.geo.activityY()]; !strings.Contains(ind, "Working…") || !strings.Contains(ind, "read_file") {
		t.Fatalf("tool indicator: %q", ind)
	}
	// Stall shows in the indicator.
	m.stall = "no progress for 21s (grok prompt sent)"
	if ind := screen(m)[m.geo.activityY()]; !strings.Contains(ind, "Waiting…") || !strings.Contains(ind, "no progress for 21s") {
		t.Fatalf("stall indicator: %q", ind)
	}
	assertFrame(t, m, 100, 30)
}
