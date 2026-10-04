package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/StephenSHorton/rock/internal/harness"
	"github.com/StephenSHorton/rock/internal/jev"
	"github.com/StephenSHorton/rock/internal/perms"
	"github.com/StephenSHorton/rock/internal/provider"
	"github.com/StephenSHorton/rock/internal/tools"
)

func TestInitializeAndPromptV2(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	dir := t.TempDir()
	clientR, agentW := io.Pipe()
	agentR, clientW := io.Pipe()
	agent := &Agent{
		CWD: dir,
		In:  agentR,
		Out: agentW,
		Factory: func(cwd string) (*harness.Harness, error) {
			return harness.New(harness.Options{
				Provider:  &provider.Script{Replies: []provider.Message{{Role: provider.RoleAssistant, Content: "from acp"}}},
				FastModel: "fast",
				Policy:    perms.Policy{Mode: perms.ModeDefault},
				Gates:     jev.Gates{},
				Tools:     tools.New(tools.Env{Root: cwd}),
			}), nil
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- agent.Serve(ctx) }()

	mustWrite(t, clientW, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":2,"capabilities":{},"info":{"name":"test","version":"0"}}}`)
	initRes := mustRead(t, clientR)
	if !strings.Contains(initRes, `"protocolVersion":2`) || !strings.Contains(initRes, `"name":"rock"`) {
		t.Fatal(initRes)
	}
	mustWrite(t, clientW, `{"jsonrpc":"2.0","id":2,"method":"session/new","params":{"cwd":"`+dir+`"}}`)
	newRes := mustRead(t, clientR)
	var created struct {
		Result struct {
			SessionID string `json:"sessionId"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(newRes), &created); err != nil || created.Result.SessionID == "" {
		t.Fatal(err, newRes)
	}
	mustWrite(t, clientW, `{"jsonrpc":"2.0","id":3,"method":"session/prompt","params":{"sessionId":"`+created.Result.SessionID+`","prompt":[{"type":"text","text":"hi"}]}}`)
	var sawChunk, sawIdle, sawAck bool
	for i := 0; i < 8; i++ {
		line := mustRead(t, clientR)
		if strings.Contains(line, `"id":3`) {
			sawAck = true
		}
		if strings.Contains(line, "from acp") {
			sawChunk = true
		}
		if strings.Contains(line, `"state":"idle"`) {
			sawIdle = true
		}
		if sawChunk && sawIdle && sawAck {
			break
		}
	}
	if !sawChunk || !sawIdle || !sawAck {
		t.Fatalf("chunk=%v idle=%v ack=%v", sawChunk, sawIdle, sawAck)
	}
	_ = clientW.Close()
	cancel()
	<-done
}

func mustWrite(t *testing.T, w io.Writer, s string) {
	t.Helper()
	if _, err := io.WriteString(w, s+"\n"); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, r io.Reader) string {
	t.Helper()
	sc := bufio.NewScanner(r)
	if !sc.Scan() {
		t.Fatal(sc.Err())
	}
	return sc.Text()
}
