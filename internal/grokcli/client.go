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

// childProc tracks a real grok process so early exits surface stderr
// instead of hanging until the probe deadline.
type childProc struct {
	cmd     *exec.Cmd
	stderr  *strings.Builder
	done    chan struct{}
	waitErr error
}

func (p *childProc) finished() bool {
	if p == nil {
		return false
	}
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

func (p *childProc) exitErr() error {
	if p == nil || !p.finished() {
		return nil
	}
	return formatChildExit(p.waitErr, p.stderr.String())
}

func (p *childProc) waitExit(timeout time.Duration) error {
	if p == nil {
		return nil
	}
	select {
	case <-p.done:
		return formatChildExit(p.waitErr, p.stderr.String())
	case <-time.After(timeout):
		if s := strings.TrimSpace(p.stderr.String()); s != "" {
			return formatChildExit(nil, s)
		}
		return fmt.Errorf("grok closed stdout")
	}
}

func formatChildExit(waitErr error, stderr string) error {
	stderr = strings.TrimSpace(stderr)
	// Strip common CLI noise prefixes.
	for _, prefix := range []string{"error: ", "Error: "} {
		if strings.HasPrefix(stderr, prefix) {
			stderr = strings.TrimSpace(strings.TrimPrefix(stderr, prefix))
			break
		}
	}
	if stderr != "" {
		// First line is enough for the status bar / inspect.
		if i := strings.IndexAny(stderr, "\r\n"); i >= 0 {
			stderr = stderr[:i]
		}
		return fmt.Errorf("grok exited: %s", stderr)
	}
	if waitErr != nil {
		return fmt.Errorf("grok exited: %v", waitErr)
	}
	return fmt.Errorf("grok exited")
}

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
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf
	config.ScrubCmdEnv(cmd)
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, nil, nil, err
	}
	proc := &childProc{cmd: cmd, stderr: &stderrBuf, done: make(chan struct{})}
	go func() {
		proc.waitErr = cmd.Wait()
		close(proc.done)
	}()
	stop := func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		<-proc.done
	}
	return &procWriter{WriteCloser: stdin, proc: proc}, &procReader{ReadCloser: stdout, proc: proc}, stop, nil
}

type procWriter struct {
	io.WriteCloser
	proc *childProc
}

type procReader struct {
	io.ReadCloser
	proc *childProc
}

func (r *procReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if err != nil && r.proc != nil {
		if exit := r.proc.exitErr(); exit != nil {
			return n, exit
		}
	}
	return n, err
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
	proc    *childProc
	mu      sync.Mutex
	writeMu sync.Mutex
	next    atomic.Int64
	wait    map[int64]pending
	text    strings.Builder
	calls   []proposed
	cwd     string
	permN   atomic.Int64
	fsN     atomic.Int64

	modelConfigID string
	modelCurrent  string
	models        []ModelInfo
	modelArg      string // -m value used to start; empty = default ChildArgs
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
	if pr, ok := out.(*procReader); ok {
		c.proc = pr.proc
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
	// Stdout closed: fail any waiters with the child's exit reason when
	// we have one, so probes do not hang until the deadline.
	errMsg := "grok closed stdout"
	if c.proc != nil {
		errMsg = c.proc.waitExit(200 * time.Millisecond).Error()
	} else if err := sc.Err(); err != nil {
		errMsg = err.Error()
	}
	c.failPending(errMsg)
}

func (c *client) failPending(message string) {
	c.mu.Lock()
	waiters := c.wait
	c.wait = map[int64]pending{}
	c.mu.Unlock()
	msg := rpc{JSONRPC: "2.0", Error: &rpcError{Code: -32000, Message: message}}
	for _, p := range waiters {
		p.done <- msg
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
	if err != nil && c.proc != nil {
		if exit := c.proc.waitExit(200 * time.Millisecond); exit != nil {
			return exit
		}
	}
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
		Meta json.RawMessage `json:"_meta"`
	}
	_ = json.Unmarshal(res, &out)
	c.applyModelStateMeta(out.Meta)
	var ids []string
	for _, m := range out.AuthMethods {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	return ids, nil
}

// applyModelStateMeta reads grok's initialize _meta.modelState
// (currentModelId + availableModels). Real grok 1.0.41 puts the
// subscription catalog here rather than only on session/new.
func (c *client) applyModelStateMeta(meta json.RawMessage) {
	if len(meta) == 0 {
		return
	}
	var m struct {
		ModelState *struct {
			CurrentModelID string `json:"currentModelId"`
			Available      []struct {
				ModelID     string `json:"modelId"`
				Name        string `json:"name"`
				Description string `json:"description"`
				Meta        *struct {
					TotalContextTokens int `json:"totalContextTokens"`
				} `json:"_meta"`
			} `json:"availableModels"`
		} `json:"modelState"`
	}
	if json.Unmarshal(meta, &m) != nil || m.ModelState == nil {
		return
	}
	c.modelCurrent = strings.TrimSpace(m.ModelState.CurrentModelID)
	var models []ModelInfo
	for _, am := range m.ModelState.Available {
		id := strings.TrimSpace(am.ModelID)
		if id == "" {
			continue
		}
		name := strings.TrimSpace(am.Name)
		if name == "" {
			name = id
		}
		info := ModelInfo{ID: id, Name: name, Description: strings.TrimSpace(am.Description)}
		if am.Meta != nil {
			info.ContextTokens = am.Meta.TotalContextTokens
		}
		models = append(models, info)
	}
	if len(models) > 0 {
		c.models = models
	}
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
		SessionID     string            `json:"sessionId"`
		ConfigOptions []json.RawMessage `json:"configOptions"`
		// Legacy/unstable field some agents still send.
		Models *struct {
			CurrentModelID string `json:"currentModelId"`
			Available      []struct {
				ModelID string `json:"modelId"`
				Name    string `json:"name"`
			} `json:"availableModels"`
		} `json:"models"`
	}
	if err := json.Unmarshal(res, &out); err != nil {
		return "", err
	}
	if out.SessionID == "" {
		return "", fmt.Errorf("grok-cli: session/new returned no sessionId")
	}
	c.applyModelConfig(out.ConfigOptions)
	if len(c.models) == 0 && out.Models != nil {
		c.modelCurrent = strings.TrimSpace(out.Models.CurrentModelID)
		for _, m := range out.Models.Available {
			id := strings.TrimSpace(m.ModelID)
			if id == "" {
				continue
			}
			name := strings.TrimSpace(m.Name)
			if name == "" {
				name = id
			}
			c.models = append(c.models, ModelInfo{ID: id, Name: name})
		}
	}
	return out.SessionID, nil
}

func (c *client) applyModelConfig(options []json.RawMessage) {
	// Do not clear initialize _meta.modelState when session/new has no
	// model configOptions — real grok 1.0.41 advertises models only on
	// initialize.
	for _, raw := range options {
		var opt struct {
			ConfigID     string `json:"configId"`
			Category     string `json:"category"`
			Type         string `json:"type"`
			CurrentValue string `json:"currentValue"`
			Options      []struct {
				Value string `json:"value"`
				Name  string `json:"name"`
			} `json:"options"`
		}
		if json.Unmarshal(raw, &opt) != nil {
			continue
		}
		if strings.ToLower(opt.Category) != "model" && opt.ConfigID != "model" {
			continue
		}
		c.modelConfigID = opt.ConfigID
		if c.modelConfigID == "" {
			c.modelConfigID = "model"
		}
		c.modelCurrent = strings.TrimSpace(opt.CurrentValue)
		var models []ModelInfo
		for _, o := range opt.Options {
			id := strings.TrimSpace(o.Value)
			if id == "" {
				continue
			}
			name := strings.TrimSpace(o.Name)
			if name == "" {
				name = id
			}
			models = append(models, ModelInfo{ID: id, Name: name})
		}
		if len(models) > 0 {
			c.models = models
		}
		return
	}
}

func (c *client) setModel(ctx context.Context, sessionID, modelID string) error {
	if c.modelConfigID != "" {
		res, err := c.request(ctx, "session/set_config_option", map[string]any{
			"sessionId": sessionID,
			"configId":  c.modelConfigID,
			"value":     modelID,
		})
		if err == nil {
			var out struct {
				ConfigOptions []json.RawMessage `json:"configOptions"`
			}
			_ = json.Unmarshal(res, &out)
			if len(out.ConfigOptions) > 0 {
				c.applyModelConfig(out.ConfigOptions)
			} else {
				c.modelCurrent = modelID
			}
			return nil
		}
	}
	// Some agents still accept the older unstable method.
	if _, err := c.request(ctx, "session/set_model", map[string]any{
		"sessionId": sessionID,
		"modelId":   modelID,
	}); err == nil {
		c.modelCurrent = modelID
		return nil
	}
	return errNeedRestartModel
}

var errNeedRestartModel = fmt.Errorf("grok-cli: restart child with -m to switch models")

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
