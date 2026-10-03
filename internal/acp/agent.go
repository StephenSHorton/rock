// Package acp is a JSON-RPC agent on stdio. It speaks enough of ACP v1 and v2
// for an editor to initialize, open a session, and send a prompt.
package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/StephenSHorton/rock/internal/harness"
	"github.com/StephenSHorton/rock/internal/session"
	"github.com/StephenSHorton/rock/internal/version"
)

type Factory func(cwd string) (*harness.Harness, error)

type Agent struct {
	Factory Factory
	CWD     string
	In      io.Reader
	Out     io.Writer

	mu       sync.Mutex
	version  int
	sessions map[string]*session.Session
	harness  map[string]*harness.Harness
	cancel   map[string]context.CancelFunc
}

type rpc struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (a *Agent) Serve(ctx context.Context) error {
	a.sessions = map[string]*session.Session{}
	a.harness = map[string]*harness.Harness{}
	a.cancel = map[string]context.CancelFunc{}
	if a.version == 0 {
		a.version = 1
	}
	sc := bufio.NewScanner(a.In)
	sc.Buffer(make([]byte, 0, 64*1024), 4<<20)
	for sc.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var msg rpc
		if err := json.Unmarshal(line, &msg); err != nil {
			_ = a.write(rpc{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: err.Error()}})
			continue
		}
		if msg.Method == "" {
			continue
		}
		if err := a.handle(ctx, msg); err != nil {
			return err
		}
	}
	return sc.Err()
}

func (a *Agent) handle(ctx context.Context, msg rpc) error {
	switch msg.Method {
	case "initialize":
		var p struct {
			ProtocolVersion int `json:"protocolVersion"`
		}
		_ = json.Unmarshal(msg.Params, &p)
		if p.ProtocolVersion >= 2 {
			a.version = 2
			return a.reply(msg.ID, map[string]any{
				"protocolVersion": 2,
				"capabilities": map[string]any{
					"session": map[string]any{"prompt": map[string]any{"embeddedContext": map[string]any{}}},
				},
				"info":        map[string]any{"name": "rock", "title": "Rock", "version": version.Version},
				"authMethods": []any{},
			})
		}
		a.version = 1
		return a.reply(msg.ID, map[string]any{
			"protocolVersion": 1,
			"agentCapabilities": map[string]any{
				"loadSession": true,
				"promptCapabilities": map[string]any{
					"image":           false,
					"audio":           false,
					"embeddedContext": true,
				},
			},
			"agentInfo": map[string]any{"name": "rock", "title": "Rock", "version": version.Version},
		})
	case "session/new":
		var p struct {
			CWD string `json:"cwd"`
		}
		_ = json.Unmarshal(msg.Params, &p)
		cwd := p.CWD
		if cwd == "" {
			cwd = a.CWD
		}
		h, err := a.Factory(cwd)
		if err != nil {
			return a.fail(msg.ID, err)
		}
		sess, err := session.Create(cwd, "", "acp")
		if err != nil {
			return a.fail(msg.ID, err)
		}
		a.mu.Lock()
		a.sessions[sess.Meta.ID] = sess
		a.harness[sess.Meta.ID] = h
		a.mu.Unlock()
		return a.reply(msg.ID, map[string]any{"sessionId": sess.Meta.ID})
	case "session/prompt":
		var p struct {
			SessionID string `json:"sessionId"`
			Prompt    []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"prompt"`
		}
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return a.fail(msg.ID, err)
		}
		var text string
		for _, block := range p.Prompt {
			if block.Type == "text" || block.Type == "" {
				text += block.Text
			}
		}
		a.mu.Lock()
		sess := a.sessions[p.SessionID]
		h := a.harness[p.SessionID]
		a.mu.Unlock()
		if sess == nil || h == nil {
			return a.fail(msg.ID, fmt.Errorf("unknown session"))
		}
		runCtx, cancel := context.WithCancel(ctx)
		a.mu.Lock()
		if prev := a.cancel[p.SessionID]; prev != nil {
			prev()
		}
		a.cancel[p.SessionID] = cancel
		a.mu.Unlock()
		if a.version >= 2 {
			if err := a.reply(msg.ID, map[string]any{}); err != nil {
				cancel()
				return err
			}
			go a.run(runCtx, p.SessionID, h, sess, text)
			return nil
		}
		err := a.run(runCtx, p.SessionID, h, sess, text)
		cancel()
		if err != nil {
			return a.fail(msg.ID, err)
		}
		return a.reply(msg.ID, map[string]any{"stopReason": "end_turn"})
	case "session/cancel":
		var p struct {
			SessionID string `json:"sessionId"`
		}
		_ = json.Unmarshal(msg.Params, &p)
		a.mu.Lock()
		if c := a.cancel[p.SessionID]; c != nil {
			c()
		}
		a.mu.Unlock()
		if msg.ID != nil {
			return a.reply(msg.ID, map[string]any{})
		}
		return nil
	default:
		if msg.ID != nil {
			return a.fail(msg.ID, fmt.Errorf("method not found: %s", msg.Method))
		}
		return nil
	}
}

func (a *Agent) run(ctx context.Context, id string, h *harness.Harness, sess *session.Session, text string) error {
	_ = a.notify("session/update", map[string]any{
		"sessionId": id,
		"update":    map[string]any{"sessionUpdate": "state_update", "state": "running"},
	})
	err := h.Run(ctx, sess, text, func(ev harness.Event) {
		switch ev.Kind {
		case harness.EvAssistant:
			_ = a.notify("session/update", map[string]any{
				"sessionId": id,
				"update": map[string]any{
					"sessionUpdate": "agent_message_chunk",
					"content":       map[string]any{"type": "text", "text": ev.Text},
				},
			})
		case harness.EvToolCall:
			_ = a.notify("session/update", map[string]any{
				"sessionId": id,
				"update":    map[string]any{"sessionUpdate": "tool_call", "title": ev.Name, "rawInput": ev.Text},
			})
		}
	})
	reason := "end_turn"
	if err != nil {
		reason = "cancelled"
	}
	_ = a.notify("session/update", map[string]any{
		"sessionId": id,
		"update":    map[string]any{"sessionUpdate": "state_update", "state": "idle", "stopReason": reason},
	})
	return err
}

func (a *Agent) reply(id any, result any) error {
	return a.write(rpc{JSONRPC: "2.0", ID: id, Result: result})
}

func (a *Agent) fail(id any, err error) error {
	return a.write(rpc{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: -32000, Message: err.Error()}})
}

func (a *Agent) notify(method string, params any) error {
	return a.write(rpc{JSONRPC: "2.0", Method: method, Params: mustRaw(params)})
}

func (a *Agent) write(msg rpc) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	raw, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = a.Out.Write(append(raw, '\n'))
	return err
}

func mustRaw(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
