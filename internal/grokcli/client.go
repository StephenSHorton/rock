package grokcli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/StephenSHorton/rock/internal/config"
	"sync"
	"sync/atomic"
	"time"
)

// StartFunc opens an ACP child. Tests inject a fake. Production starts
// the official binary with ChildArgs.
type StartFunc func(ctx context.Context, bin string, args []string) (io.WriteCloser, io.ReadCloser, func(), error)

func startExec(ctx context.Context, bin string, args []string) (io.WriteCloser, io.ReadCloser, func(), error) {
	if err := rejectForbidden(args); err != nil {
		return nil, nil, nil, err
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, nil, nil, err
	}
	cmd.Stderr = io.Discard
	config.ScrubCmdEnv(cmd)
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, nil, nil, err
	}
	stop := func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
	return stdin, stdout, stop, nil
}

type rpc struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type pending struct {
	done chan rpc
}

type client struct {
	in      io.WriteCloser
	out     io.ReadCloser
	stop    func()
	mu      sync.Mutex
	writeMu sync.Mutex
	next    atomic.Int64
	wait    map[int64]pending
	text    strings.Builder
	calls   []proposed
	cwd     string
	permN   atomic.Int64
	fsN     atomic.Int64
}

type proposed struct {
	ID   string
	Name string
	Args string
}

func dial(ctx context.Context, bin string, args []string, cwd string, start StartFunc) (*client, error) {
	if start == nil {
		start = startExec
	}
	if err := rejectForbidden(args); err != nil {
		return nil, err
	}
	in, out, stop, err := start(ctx, bin, args)
	if err != nil {
		return nil, err
	}
	c := &client{
		in:   in,
		out:  out,
		stop: stop,
		wait: map[int64]pending{},
		cwd:  cwd,
	}
	go c.readLoop()
	return c, nil
}

func (c *client) Close() {
	c.mu.Lock()
	stop := c.stop
	c.stop = nil
	c.mu.Unlock()
	if stop != nil {
		stop()
	}
}

func (c *client) readLoop() {
	sc := bufio.NewScanner(c.out)
	sc.Buffer(make([]byte, 0, 64*1024), 4<<20)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var msg rpc
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}
		if msg.Method != "" && msg.ID != nil {
			go c.handleAgentRequest(msg)
			continue
		}
		if msg.Method != "" {
			c.handleNote(msg)
			continue
		}
		if id, ok := asInt(msg.ID); ok {
			c.mu.Lock()
			p, found := c.wait[id]
			if found {
				delete(c.wait, id)
			}
			c.mu.Unlock()
			if found {
				p.done <- msg
			}
		}
	}
}

func (c *client) handleNote(msg rpc) {
	if msg.Method != "session/update" {
		return
	}
	var p struct {
		Update struct {
			SessionUpdate string `json:"sessionUpdate"`
			Title         string `json:"title"`
			RawInput      any    `json:"rawInput"`
			Content       struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			ToolCallID string `json:"toolCallId"`
			Kind       string `json:"kind"`
		} `json:"update"`
	}
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return
	}
	switch p.Update.SessionUpdate {
	case "agent_message_chunk", "agent_message":
		if p.Update.Content.Text != "" {
			c.mu.Lock()
			c.text.WriteString(p.Update.Content.Text)
			c.mu.Unlock()
		}
	case "tool_call", "tool_call_update":
		args := rawToJSON(p.Update.RawInput)
		name := p.Update.Title
		if name == "" {
			name = p.Update.Kind
		}
		c.mu.Lock()
		c.calls = append(c.calls, proposed{ID: p.Update.ToolCallID, Name: name, Args: args})
		c.mu.Unlock()
	}
}

func (c *client) handleAgentRequest(msg rpc) {
	switch msg.Method {
	case "session/request_permission":
		c.permN.Add(1)
		_ = c.reply(msg.ID, denyPermission(msg.Params))
	case "fs/read_text_file", "fs/write_text_file", "terminal/create", "terminal/output", "terminal/release", "terminal/wait_for_exit", "terminal/kill":
		c.fsN.Add(1)
		_ = c.fail(msg.ID, fmt.Errorf("Rock denies child %s; tools stay in Rock behind Jev Risk", msg.Method))
	default:
		_ = c.fail(msg.ID, fmt.Errorf("unsupported client method %s", msg.Method))
	}
}

func denyPermission(params json.RawMessage) map[string]any {
	var p struct {
		Options []struct {
			OptionID string `json:"optionId"`
			Kind     string `json:"kind"`
		} `json:"options"`
	}
	_ = json.Unmarshal(params, &p)
	for _, o := range p.Options {
		k := strings.ToLower(o.Kind)
		id := strings.ToLower(o.OptionID)
		if strings.Contains(k, "reject") || strings.Contains(id, "reject") || strings.Contains(id, "deny") {
			return map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": o.OptionID}}
		}
	}
	return map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}
}

func (c *client) request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.next.Add(1)
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	done := make(chan rpc, 1)
	c.mu.Lock()
	c.wait[id] = pending{done: done}
	c.mu.Unlock()
	if err := c.write(rpc{JSONRPC: "2.0", ID: id, Method: method, Params: raw}); err != nil {
		c.mu.Lock()
		delete(c.wait, id)
		c.mu.Unlock()
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg := <-done:
		if msg.Error != nil {
			return nil, fmt.Errorf("grok-cli %s: %s", method, msg.Error.Message)
		}
		return msg.Result, nil
	}
}

func (c *client) write(msg rpc) error {
	raw, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	c.mu.Lock()
	w := c.in
	c.mu.Unlock()
	if w == nil {
		return io.ErrClosedPipe
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err = w.Write(append(raw, '\n'))
	return err
}

func (c *client) reply(id any, result any) error {
	return c.write(rpc{JSONRPC: "2.0", ID: id, Result: mustRaw(result)})
}

func (c *client) fail(id any, err error) error {
	return c.write(rpc{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: -32000, Message: err.Error()}})
}

func (c *client) initialize(ctx context.Context) ([]string, error) {
	// Empty fs/terminal caps: a well-behaved child should not ask us to
	// execute files or a PTY. We still reject those methods if it does.
	res, err := c.request(ctx, "initialize", map[string]any{
		"protocolVersion": 1,
		"clientCapabilities": map[string]any{
			"fs":       map[string]any{"readTextFile": false, "writeTextFile": false},
			"terminal": false,
		},
		"clientInfo": map[string]any{"name": "rock", "title": "Rock"},
	})
	if err != nil {
		return nil, err
	}
	var out struct {
		AuthMethods []struct {
			ID string `json:"id"`
		} `json:"authMethods"`
	}
	_ = json.Unmarshal(res, &out)
	var ids []string
	for _, m := range out.AuthMethods {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	return ids, nil
}

func (c *client) authenticate(ctx context.Context, methods []string) error {
	has := map[string]bool{}
	for _, m := range methods {
		has[m] = true
	}
	switch {
	case has["cached_token"]:
		_, err := c.request(ctx, "authenticate", map[string]any{"methodId": "cached_token"})
		return err
	case has["xai.api_key"] && strings.TrimSpace(os.Getenv("XAI_API_KEY")) != "":
		// Grok's own API-key path. Rock does not mint or store this key.
		_, err := c.request(ctx, "authenticate", map[string]any{"methodId": "xai.api_key"})
		return err
	case len(methods) == 0:
		return nil
	default:
		return fmt.Errorf("%s", notSignedIn())
	}
}

func (c *client) newSession(ctx context.Context) (string, error) {
	res, err := c.request(ctx, "session/new", map[string]any{
		"cwd":        c.cwd,
		"mcpServers": []any{},
		"permission": "default",
		"_meta":      map[string]any{"yoloMode": false},
	})
	if err != nil {
		return "", err
	}
	var out struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(res, &out); err != nil {
		return "", err
	}
	if out.SessionID == "" {
		return "", fmt.Errorf("grok-cli: session/new returned no sessionId")
	}
	return out.SessionID, nil
}

func (c *client) prompt(ctx context.Context, sessionID, text string) error {
	c.mu.Lock()
	c.text.Reset()
	c.calls = nil
	c.mu.Unlock()
	_, err := c.request(ctx, "session/prompt", map[string]any{
		"sessionId": sessionID,
		"prompt":    []map[string]string{{"type": "text", "text": text}},
	})
	return err
}

func (c *client) snapshot() (string, []proposed) {
	c.mu.Lock()
	defer c.mu.Unlock()
	calls := append([]proposed(nil), c.calls...)
	return c.text.String(), calls
}

func asInt(id any) (int64, bool) {
	switch v := id.(type) {
	case float64:
		return int64(v), true
	case int64:
		return v, true
	case int:
		return int64(v), true
	case json.Number:
		n, err := v.Int64()
		return n, err == nil
	}
	return 0, false
}

func rawToJSON(v any) string {
	if v == nil {
		return "{}"
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func mustRaw(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func withTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, d)
}
