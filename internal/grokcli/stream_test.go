package grokcli

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/StephenSHorton/rock/internal/provider"
)

func TestCompleteStreamsThoughtsAndText(t *testing.T) {
	fake := &FakeScript{
		Thoughts:    []string{"think ", "more"},
		Reply:       "Sure.\n```tool\n{\"name\":\"read_file\",\"arguments\":{\"path\":\"a.go\"}}\n```",
		ReplyChunks: 5,
	}
	p := &Provider{CWD: t.TempDir(), Start: StartFake(fake)}
	defer p.Close()
	var mu sync.Mutex
	var thoughts, text strings.Builder
	ctx := provider.WithStream(context.Background(), func(k provider.StreamKind, d string) {
		mu.Lock()
		defer mu.Unlock()
		if k == provider.StreamThought {
			thoughts.WriteString(d)
		} else {
			text.WriteString(d)
		}
	})
	msg, err := p.Complete(ctx, "", []provider.Message{{Role: provider.RoleUser, Content: "read a.go"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if thoughts.String() != "think more" {
		t.Fatalf("thoughts %q", thoughts.String())
	}
	if text.String() != fake.Reply {
		t.Fatalf("streamed text %q", text.String())
	}
	if msg.Content != "Sure." || len(msg.ToolCalls) != 1 || msg.ToolCalls[0].Name != "read_file" {
		t.Fatalf("final message must still parse fences: %+v", msg)
	}
	// Without a stream callback nothing is required and the result is the same.
	msg2, err := p.Complete(context.Background(), "", []provider.Message{{Role: provider.RoleUser, Content: "again"}}, nil)
	if err != nil || len(msg2.ToolCalls) != 1 {
		t.Fatalf("no-stream: %+v %v", msg2, err)
	}
}
