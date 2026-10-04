package grokcli

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
)

// FakeScript drives an in-process ACP child for tests.
type FakeScript struct {
	AuthMethods []string
	Reply       string
	ToolName    string
	ToolArgs    string
	AskPerm     bool
	AskFSWrite  bool
	AuthSeen    string

	mu   sync.Mutex
	args []string
}

func (f *FakeScript) Args() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.args...)
}

func (f *FakeScript) setArgs(args []string) {
	f.mu.Lock()
	f.args = append([]string(nil), args...)
	f.mu.Unlock()
}

func (f *FakeScript) setAuth(id string) {
	f.mu.Lock()
	f.AuthSeen = id
	f.mu.Unlock()
}

// StartFake is a StartFunc that runs FakeACP on pipes. No network.
func StartFake(script *FakeScript) StartFunc {
	if script == nil {
		script = &FakeScript{Reply: "hello from fake grok"}
	}
	return func(_ context.Context, _ string, args []string) (io.WriteCloser, io.ReadCloser, func(), error) {
		script.setArgs(args)
		agentIn, clientOut := io.Pipe()
		clientIn, agentOut := io.Pipe()
		done := make(chan struct{})
		go func() {
			defer close(done)
			FakeACP(agentIn, agentOut, script)
			_ = agentOut.Close()
		}()
		stop := func() {
			_ = clientOut.Close()
			_ = agentIn.Close()
			_ = clientIn.Close()
			<-done
		}
		return clientOut, clientIn, stop, nil
	}
}

// FakeACP speaks enough ACP for grok-cli tests. No network.
func FakeACP(in io.Reader, out io.Writer, script *FakeScript) {
	if script == nil {
		script = &FakeScript{Reply: "hello from fake grok"}
	}
	write := func(v any) {
		raw, _ := json.Marshal(v)
		_, _ = out.Write(append(raw, '\n'))
	}
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	sessionID := "sess_fake"
	for sc.Scan() {
		var msg rpc
		if json.Unmarshal(sc.Bytes(), &msg) != nil || msg.Method == "" {
			continue
		}
		switch msg.Method {
		case "initialize":
			methods := script.AuthMethods
			if methods == nil {
				methods = []string{"cached_token"}
			}
			var listed []map[string]string
			for _, id := range methods {
				listed = append(listed, map[string]string{"id": id})
			}
			write(rpc{JSONRPC: "2.0", ID: msg.ID, Result: mustRaw(map[string]any{
				"protocolVersion": 1,
				"authMethods":     listed,
				"_meta":           map[string]any{"grokShell": true},
			})})
		case "authenticate":
			var p struct {
				MethodID string `json:"methodId"`
			}
			_ = json.Unmarshal(msg.Params, &p)
			script.setAuth(p.MethodID)
			write(rpc{JSONRPC: "2.0", ID: msg.ID, Result: mustRaw(map[string]any{})})
		case "session/new":
			write(rpc{JSONRPC: "2.0", ID: msg.ID, Result: mustRaw(map[string]any{"sessionId": sessionID})})
		case "session/prompt":
			if script.AskFSWrite {
				write(rpc{
					JSONRPC: "2.0",
					ID:      9001,
					Method:  "fs/write_text_file",
					Params:  mustRaw(map[string]any{"path": "/tmp/nope", "content": "x"}),
				})
			}
			if script.AskPerm {
				write(rpc{
					JSONRPC: "2.0",
					ID:      9002,
					Method:  "session/request_permission",
					Params: mustRaw(map[string]any{
						"sessionId": sessionID,
						"title":     "run shell",
						"options": []map[string]string{
							{"optionId": "allow-once", "kind": "allow_once", "name": "Allow"},
							{"optionId": "reject-once", "kind": "reject_once", "name": "Reject"},
						},
					}),
				})
			}
			if script.ToolName != "" {
				write(rpc{JSONRPC: "2.0", Method: "session/update", Params: mustRaw(map[string]any{
					"sessionId": sessionID,
					"update": map[string]any{
						"sessionUpdate": "tool_call",
						"toolCallId":    "call_1",
						"title":         script.ToolName,
						"rawInput":      json.RawMessage(orJSON(script.ToolArgs)),
					},
				})})
			}
			if script.Reply != "" {
				write(rpc{JSONRPC: "2.0", Method: "session/update", Params: mustRaw(map[string]any{
					"sessionId": sessionID,
					"update": map[string]any{
						"sessionUpdate": "agent_message_chunk",
						"content":       map[string]any{"type": "text", "text": script.Reply},
					},
				})})
			}
			write(rpc{JSONRPC: "2.0", ID: msg.ID, Result: mustRaw(map[string]any{"stopReason": "end_turn"})})
		default:
			write(rpc{JSONRPC: "2.0", ID: msg.ID, Error: &rpcError{Code: -32601, Message: "method not found"}})
		}
	}
}

func orJSON(s string) string {
	if strings.TrimSpace(s) == "" {
		return "{}"
	}
	return s
}
