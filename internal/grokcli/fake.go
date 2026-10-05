package grokcli

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"
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
	// Models, when non-nil, are advertised on initialize _meta.modelState
	// (matching real grok 1.0.41). ConfigOptionsModels also puts them on
	// session/new (with real "id" field). SessionModels puts the models
	// object on session/new (real grok sends both).
	Models              []ModelInfo
	CurrentModel        string
	ConfigOptionsModels bool
	SessionModels       bool // include models{} on session/new
	AcceptSetModel      bool // respond to session/set_model
	RejectConfigOption  bool // force set_config_option to fail
	SetModelSeen        string
	SetModelVia         string // "config" | "set_model" | ""
	// StrictACP rejects initialize / session/new / session/prompt that
	// do not match real grok 1.0.41 expectations (Invalid params -32602).
	StrictACP bool
	// EmitSetupNotes sends unknown _x.ai/* notifications before
	// session/new responds (client must ignore them).
	EmitSetupNotes bool
	// PromptDelay sleeps before answering session/prompt (lifetime tests).
	PromptDelay time.Duration
	CancelSeen  int

	mu         sync.Mutex
	args       []string
	initSeen   json.RawMessage
	newSeen    json.RawMessage
	promptSeen json.RawMessage
	authCalls  int
}

func (f *FakeScript) Args() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.args...)
}

func (f *FakeScript) AuthCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.authCalls
}

func (f *FakeScript) Cancels() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.CancelSeen
}

func (f *FakeScript) setArgs(args []string) {
	f.mu.Lock()
	f.args = append([]string(nil), args...)
	f.mu.Unlock()
}

func (f *FakeScript) setAuth(id string) {
	f.mu.Lock()
	f.AuthSeen = id
	f.authCalls++
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
	invalid := func(id any, detail string) {
		write(rpc{JSONRPC: "2.0", ID: id, Error: &rpcError{
			Code:    -32602,
			Message: "Invalid params",
			Data:    mustRaw(map[string]any{"detail": detail}),
		}})
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
			script.mu.Lock()
			script.initSeen = append(json.RawMessage(nil), msg.Params...)
			script.mu.Unlock()
			if script.StrictACP {
				if detail := validateInitialize(msg.Params); detail != "" {
					invalid(msg.ID, detail)
					break
				}
			}
			methods := script.AuthMethods
			if methods == nil {
				methods = []string{"cached_token", "grok.com"}
			}
			var listed []map[string]any
			for _, id := range methods {
				listed = append(listed, map[string]any{"id": id, "name": id})
			}
			meta := map[string]any{
				"grokShell":           true,
				"defaultAuthMethodId": "cached_token",
				"agentVersion":        "1.0.41",
			}
			if script.Models != nil {
				cur := script.current()
				var avail []map[string]any
				for _, m := range script.Models {
					entry := map[string]any{"modelId": m.ID, "name": m.Name, "description": m.Description}
					if m.ContextTokens > 0 {
						entry["_meta"] = map[string]any{"totalContextTokens": m.ContextTokens}
					}
					avail = append(avail, entry)
				}
				meta["modelState"] = map[string]any{
					"currentModelId":  cur,
					"availableModels": avail,
				}
			}
			write(rpc{JSONRPC: "2.0", ID: msg.ID, Result: mustRaw(map[string]any{
				"protocolVersion": 1,
				"agentCapabilities": map[string]any{
					"loadSession": true,
					"promptCapabilities": map[string]any{
						"image": false, "audio": false, "embeddedContext": true,
					},
					"mcpCapabilities": map[string]any{"http": true, "sse": true},
				},
				"authMethods": listed,
				"_meta":       meta,
			})})
		case "authenticate":
			var p struct {
				MethodID string `json:"methodId"`
			}
			_ = json.Unmarshal(msg.Params, &p)
			script.setAuth(p.MethodID)
			write(rpc{JSONRPC: "2.0", ID: msg.ID, Result: mustRaw(map[string]any{})})
		case "session/new":
			script.mu.Lock()
			script.newSeen = append(json.RawMessage(nil), msg.Params...)
			script.mu.Unlock()
			if script.StrictACP {
				if detail := validateSessionNew(msg.Params); detail != "" {
					invalid(msg.ID, detail)
					break
				}
			}
			if script.EmitSetupNotes {
				for _, method := range []string{
					"_x.ai/session/setup",
					"_x.ai/mcp/servers_updated",
					"_x.ai/models/update",
					"_x.ai/settings/update",
					"_x.ai/announcements/update",
				} {
					write(rpc{JSONRPC: "2.0", Method: method, Params: mustRaw(map[string]any{"ok": true})})
				}
			}
			result := map[string]any{"sessionId": sessionID}
			cur := script.current()
			if (script.SessionModels || script.ConfigOptionsModels) && script.Models != nil {
				var avail []map[string]any
				for _, m := range script.Models {
					entry := map[string]any{"modelId": m.ID, "name": m.Name, "description": m.Description}
					if m.ContextTokens > 0 {
						entry["_meta"] = map[string]any{"totalContextTokens": m.ContextTokens}
					}
					avail = append(avail, entry)
				}
				if script.SessionModels || script.ConfigOptionsModels {
					result["models"] = map[string]any{
						"currentModelId":  cur,
						"availableModels": avail,
					}
				}
			}
			if script.ConfigOptionsModels && script.Models != nil {
				var opts []map[string]string
				for _, m := range script.Models {
					opts = append(opts, map[string]string{"value": m.ID, "name": m.Name})
				}
				// Real grok 1.0.41 uses "id", not only "configId".
				result["configOptions"] = []map[string]any{{
					"id":           "model",
					"name":         "Model",
					"category":     "model",
					"type":         "select",
					"currentValue": cur,
					"options":      opts,
				}}
			}
			write(rpc{JSONRPC: "2.0", ID: msg.ID, Result: mustRaw(result)})
		case "session/set_config_option":
			if script.RejectConfigOption || !script.ConfigOptionsModels {
				write(rpc{JSONRPC: "2.0", ID: msg.ID, Error: &rpcError{Code: -32601, Message: "method not found"}})
				break
			}
			var p struct {
				ConfigID string `json:"configId"`
				Value    string `json:"value"`
			}
			_ = json.Unmarshal(msg.Params, &p)
			script.mu.Lock()
			script.SetModelSeen = p.Value
			script.SetModelVia = "config"
			script.CurrentModel = p.Value
			models := append([]ModelInfo(nil), script.Models...)
			script.mu.Unlock()
			var opts []map[string]string
			for _, m := range models {
				opts = append(opts, map[string]string{"value": m.ID, "name": m.Name})
			}
			write(rpc{JSONRPC: "2.0", ID: msg.ID, Result: mustRaw(map[string]any{
				"configOptions": []map[string]any{{
					"id":           "model",
					"name":         "Model",
					"category":     "model",
					"type":         "select",
					"currentValue": p.Value,
					"options":      opts,
				}},
			})})
		case "session/set_model":
			if !script.AcceptSetModel {
				write(rpc{JSONRPC: "2.0", ID: msg.ID, Error: &rpcError{Code: -32601, Message: "method not found"}})
				break
			}
			var p struct {
				ModelID string `json:"modelId"`
			}
			_ = json.Unmarshal(msg.Params, &p)
			script.mu.Lock()
			script.SetModelSeen = p.ModelID
			script.SetModelVia = "set_model"
			script.CurrentModel = p.ModelID
			script.mu.Unlock()
			write(rpc{JSONRPC: "2.0", ID: msg.ID, Result: mustRaw(map[string]any{})})
		case "session/prompt":
			script.mu.Lock()
			script.promptSeen = append(json.RawMessage(nil), msg.Params...)
			delay := script.PromptDelay
			reply := script.Reply
			askFS := script.AskFSWrite
			askPerm := script.AskPerm
			toolName := script.ToolName
			toolArgs := script.ToolArgs
			script.mu.Unlock()
			if script.StrictACP {
				if detail := validateSessionPrompt(msg.Params); detail != "" {
					invalid(msg.ID, detail)
					break
				}
			}
			reqID := msg.ID
			go func() {
				if delay > 0 {
					time.Sleep(delay)
				}
				if askFS {
					write(rpc{
						JSONRPC: "2.0",
						ID:      9001,
						Method:  "fs/write_text_file",
						Params:  mustRaw(map[string]any{"path": "/tmp/nope", "content": "x"}),
					})
				}
				if askPerm {
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
				if toolName != "" {
					write(rpc{JSONRPC: "2.0", Method: "session/update", Params: mustRaw(map[string]any{
						"sessionId": sessionID,
						"update": map[string]any{
							"sessionUpdate": "tool_call",
							"toolCallId":    "call_1",
							"title":         toolName,
							"rawInput":      json.RawMessage(orJSON(toolArgs)),
						},
					})})
				}
				if reply != "" {
					write(rpc{JSONRPC: "2.0", Method: "session/update", Params: mustRaw(map[string]any{
						"sessionId": sessionID,
						"update": map[string]any{
							"sessionUpdate": "agent_message_chunk",
							"content":       map[string]any{"type": "text", "text": reply},
						},
					})})
				}
				write(rpc{JSONRPC: "2.0", ID: reqID, Result: mustRaw(map[string]any{"stopReason": "end_turn"})})
			}()

		case "session/cancel":
			script.mu.Lock()
			script.CancelSeen++
			script.mu.Unlock()
			// notification — no response
		default:
			if msg.ID != nil {
				write(rpc{JSONRPC: "2.0", ID: msg.ID, Error: &rpcError{Code: -32601, Message: "method not found"}})
			}
		}
	}
}

func (f *FakeScript) current() string {
	cur := f.CurrentModel
	if cur == "" && len(f.Models) > 0 {
		cur = f.Models[0].ID
	}
	return cur
}

func validateInitialize(params json.RawMessage) string {
	var p struct {
		ProtocolVersion json.RawMessage `json:"protocolVersion"`
		Caps            json.RawMessage `json:"clientCapabilities"`
		Info            *struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"clientInfo"`
	}
	if json.Unmarshal(params, &p) != nil {
		return "unmarshal"
	}
	var ver any
	if json.Unmarshal(p.ProtocolVersion, &ver) != nil {
		return "protocolVersion"
	}
	switch v := ver.(type) {
	case float64:
		if v != 1 {
			return "protocolVersion not 1"
		}
	default:
		return "protocolVersion must be integer 1"
	}
	if len(p.Caps) == 0 || string(p.Caps) == "null" {
		return "clientCapabilities required object"
	}
	var capsObj map[string]any
	if json.Unmarshal(p.Caps, &capsObj) != nil {
		return "clientCapabilities must be object"
	}
	if p.Info == nil || strings.TrimSpace(p.Info.Name) == "" {
		return "clientInfo.name required"
	}
	if strings.TrimSpace(p.Info.Version) == "" {
		return "clientInfo.version required"
	}
	return ""
}

func validateSessionNew(params json.RawMessage) string {
	var p struct {
		CWD        string          `json:"cwd"`
		MCPServers json.RawMessage `json:"mcpServers"`
	}
	if json.Unmarshal(params, &p) != nil {
		return "unmarshal"
	}
	if p.CWD == "" || !filepath.IsAbs(p.CWD) {
		return "cwd must be absolute"
	}
	if len(p.MCPServers) == 0 || string(p.MCPServers) == "null" {
		return "mcpServers required array"
	}
	if p.MCPServers[0] != '[' {
		return "mcpServers must be array"
	}
	// Reject known-bad extras that older Rock sent.
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(params, &raw)
	if _, ok := raw["permission"]; ok {
		return "unexpected permission field"
	}
	return ""
}

func validateSessionPrompt(params json.RawMessage) string {
	var p struct {
		SessionID string `json:"sessionId"`
		Prompt    []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"prompt"`
	}
	if json.Unmarshal(params, &p) != nil {
		return "unmarshal"
	}
	if strings.TrimSpace(p.SessionID) == "" {
		return "sessionId required"
	}
	if len(p.Prompt) == 0 {
		return "prompt required array"
	}
	if p.Prompt[0].Type != "text" {
		return "prompt[0].type must be text"
	}
	return ""
}

func orJSON(s string) string {
	if strings.TrimSpace(s) == "" {
		return "{}"
	}
	return s
}
