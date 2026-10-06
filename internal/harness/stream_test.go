package harness

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StephenSHorton/rock/internal/jev"
	"github.com/StephenSHorton/rock/internal/perms"
	"github.com/StephenSHorton/rock/internal/provider"
	"github.com/StephenSHorton/rock/internal/session"
	"github.com/StephenSHorton/rock/internal/tools"
)

// streamer emits deltas through the context callback, like grok-cli.
type streamer struct{}

func (streamer) Name() string { return "streamer" }
func (streamer) Complete(ctx context.Context, _ string, _ []provider.Message, _ []provider.ToolSpec) (provider.Message, error) {
	if f := provider.StreamFrom(ctx); f != nil {
		f(provider.StreamThought, "hmm ")
		f(provider.StreamText, "Hel")
		f(provider.StreamText, "lo")
	}
	return provider.Message{Role: provider.RoleAssistant, Content: "Hello"}, nil
}

func runStream(t *testing.T, stream bool) []string {
	t.Helper()
	t.Setenv("ROCK_HOME", t.TempDir())
	dir := t.TempDir()
	h := New(Options{
		Provider: streamer{}, FastModel: "fast", StrongModel: "strong",
		Policy: perms.Policy{Mode: perms.ModeDefault}, Gates: jev.Gates{},
		Tools:  tools.New(tools.Env{Root: dir, PlanPath: filepath.Join(dir, "plan.md")}),
		Stream: stream,
	})
	sess, err := session.Create(dir, "s", "t")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	if err := h.Run(context.Background(), sess, "hi", func(ev Event) {
		switch ev.Kind {
		case EvThought, EvDelta, EvAssistant, EvDone:
			got = append(got, string(ev.Kind)+":"+ev.Text)
		}
	}); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestStreamEventsOnlyWhenEnabled(t *testing.T) {
	on := strings.Join(runStream(t, true), "|")
	if on != "thought:hmm |delta:Hel|delta:lo|assistant:Hello|done:end_turn" {
		t.Fatalf("stream on: %s", on)
	}
	off := strings.Join(runStream(t, false), "|")
	if off != "assistant:Hello|done:end_turn" {
		t.Fatalf("stream off: %s", off)
	}
}
