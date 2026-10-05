package grokcli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/StephenSHorton/rock/internal/config"
	"github.com/StephenSHorton/rock/internal/version"
	"sync"
	"sync/atomic"
	"time"
)

// StartFunc opens an ACP child. Tests inject a fake. Production starts
// the official binary with ChildArgs.
type StartFunc func(ctx context.Context, bin string, args []string) (io.WriteCloser, io.ReadCloser, func(), error)

// childProc tracks a real grok process so early exits surface stderr
// instead of hanging until the probe deadline. Lifetime is owned by the
// Provider — never bound to a per-call context (CommandContext would
// TerminateProcess on cancel and yield empty-stderr exit status 1 on Windows).
type childProc struct {
	cmd          *exec.Cmd
	stderr       *strings.Builder
	done         chan struct{}
	waitErr      error
	mu           sync.Mutex
	killedByRock bool
	killReason   string
}

func (p *childProc) pid() int {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
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

func (p *childProc) kill(reason string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.killedByRock = true
	if reason != "" {
		p.killReason = reason
	}
	p.mu.Unlock()
	if p.cmd != nil && p.cmd.Process != nil {
		debugGrok("kill pid=%d reason=%s", p.pid(), reason)
		_ = p.cmd.Process.Kill()
	}
}

func (p *childProc) exitErr() error {
	if p == nil || !p.finished() {
		return nil
	}
	p.mu.Lock()
	byRock, reason := p.killedByRock, p.killReason
	p.mu.Unlock()
	return formatChildExit(p.waitErr, p.stderr.String(), byRock, reason)
}

func (p *childProc) waitExit(timeout time.Duration) error {
	if p == nil {
		return nil
	}
	select {
	case <-p.done:
		p.mu.Lock()
		byRock, reason := p.killedByRock, p.killReason
		p.mu.Unlock()
		return formatChildExit(p.waitErr, p.stderr.String(), byRock, reason)
	case <-time.After(timeout):
		if s := strings.TrimSpace(p.stderr.String()); s != "" {
			return formatChildExit(nil, s, false, "")
		}
		return fmt.Errorf("grok closed stdout")
	}
}

func formatChildExit(waitErr error, stderr string, killedByRock bool, killReason string) error {
	if killedByRock {
		if killReason == "" {
			killReason = "closed by Rock"
		}
		return fmt.Errorf("grok stopped: %s", killReason)
	}
	stderr = strings.TrimSpace(stderr)
	for _, prefix := range []string{"error: ", "Error: "} {
		if strings.HasPrefix(stderr, prefix) {
			stderr = strings.TrimSpace(strings.TrimPrefix(stderr, prefix))
			break
		}
	}
	if stderr != "" {
		if i := strings.IndexAny(stderr, "\r\n"); i >= 0 {
			stderr = stderr[:i]
		}
		if waitErr != nil {
			return fmt.Errorf("grok exited: %v: %s", waitErr, stderr)
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
	if err := ctx.Err(); err != nil {
		return nil, nil, nil, err
	}
	// Provider-lifetime process: do NOT use CommandContext. Per-call
	// cancel must not TerminateProcess the child.
	cmd := exec.Command(bin, args...)
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
	debugGrok("spawn bin=%s args=%v pid=%d", bin, args, proc.pid())
	go func() {
		proc.waitErr = cmd.Wait()
		debugGrok("exit pid=%d err=%v stderr=%q", proc.pid(), proc.waitErr, truncateDebug(stderrBuf.String()))
		close(proc.done)
	}()
	stop := func() {
		_ = stdin.Close()
		proc.kill("provider closed")
		<-proc.done
	}
	return &procWriter{WriteCloser: stdin, proc: proc}, &procReader{ReadCloser: stdout, proc: proc}, stop, nil
}

func truncateDebug(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
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
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
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
	// activity is signaled (non-blocking) on each stdout line so prompt
	// waits can apply an idle timeout without a hard wall clock.
	activity  chan struct{}
	sessionID string

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
	if abs, err := filepath.Abs(cwd); err == nil && abs != "" {
		cwd = abs
	}
	c := &client{
		in:       in,
		out:      out,
		stop:     stop,
		wait:     map[int64]pending{},
		cwd:      cwd,
		activity: make(chan struct{}, 1),
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

func (c *client) alive() bool {
	if c == nil {
		return false
	}
	if c.proc != nil {
		return !c.proc.finished()
	}
	// Fake children have no proc; treat as alive until Close.
	c.mu.Lock()
	stopped := c.stop == nil
	c.mu.Unlock()
	return !stopped
}

func (c *client) pid() int {
	if c == nil || c.proc == nil {
		return 0
	}
	return c.proc.pid()
}

func (c *client) noteActivity() {
	if c.activity == nil {
		return
	}
	select {
	case c.activity <- struct{}{}:
	default:
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
		c.noteActivity()
		debugGrok("recv %s", truncateDebug(string(line)))
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
	return c.requestIdle(ctx, method, params, 0)
}

// requestIdle waits for a JSON-RPC response. idle>0 resets on each stdout
// line (agent streaming). On ctx cancel during a live session prompt,
// callers should send session/cancel separately — this returns ctx.Err()
// without killing the child.
func (c *client) requestIdle(ctx context.Context, method string, params any, idle time.Duration) (json.RawMessage, error) {
	id := c.next.Add(1)
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	done := make(chan rpc, 1)
	c.mu.Lock()
	c.wait[id] = pending{done: done}
	c.mu.Unlock()
	debugGrok("send %s %s", method, truncateDebug(string(raw)))
	if err := c.write(rpc{JSONRPC: "2.0", ID: id, Method: method, Params: raw}); err != nil {
		c.mu.Lock()
		delete(c.wait, id)
		c.mu.Unlock()
		return nil, err
	}
	var timer *time.Timer
	var timerC <-chan time.Time
	if idle > 0 {
		timer = time.NewTimer(idle)
		defer timer.Stop()
		timerC = timer.C
	}
	for {
		select {
		case <-ctx.Done():
			c.mu.Lock()
			delete(c.wait, id)
			c.mu.Unlock()
			return nil, fmt.Errorf("grok stopped: %v", ctx.Err())
		case msg := <-done:
			if msg.Error != nil {
				return nil, formatRPCError(method, msg.Error)
			}
			return msg.Result, nil
		case <-c.activity:
			if timer != nil {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(idle)
			}
		case <-timerC:
			c.mu.Lock()
			delete(c.wait, id)
			c.mu.Unlock()
			return nil, fmt.Errorf("grok stopped: no stdout activity for %s", idle)
		}
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

// InitializeParams is the ACP initialize body shared by the probe and
// the turn-path client. Real grok 1.0.41 rejects clientInfo without a
// version string (-32602 Invalid params); the working shape is
// protocolVersion integer 1 + clientCapabilities object + clientInfo
// {name, version} (title optional per ACP).
func InitializeParams() map[string]any {
	return map[string]any{
		"protocolVersion": 1,
		"clientCapabilities": map[string]any{
			"fs":       map[string]any{"readTextFile": false, "writeTextFile": false},
			"terminal": false,
		},
		"clientInfo": map[string]any{
			"name":    "rock",
			"title":   "Rock",
			"version": version.Version,
		},
	}
}

func formatRPCError(method string, e *rpcError) error {
	if e == nil {
		return fmt.Errorf("grok-cli %s: unknown error", method)
	}
	msg := strings.TrimSpace(e.Message)
	if len(e.Data) > 0 && string(e.Data) != "null" {
		msg = msg + " (" + string(e.Data) + ")"
	}
	return fmt.Errorf("grok-cli %s: %s", method, msg)
}

func (c *client) initialize(ctx context.Context) ([]string, error) {
	res, err := c.request(ctx, "initialize", InitializeParams())
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
		// Real grok 1.0.41 lists cached_token and defaultAuthMethodId
		// after initialize; no authenticate round-trip is required.
		return nil
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
	// ACP session/new requires absolute cwd + mcpServers array.
	// Extra fields (permission, _meta) are rejected by strict agents.
	res, err := c.request(ctx, "session/new", map[string]any{
		"cwd":        c.cwd,
		"mcpServers": []any{},
	})
	if err != nil {
		return "", err
	}
	var out struct {
		SessionID     string            `json:"sessionId"`
		ConfigOptions []json.RawMessage `json:"configOptions"`
		// Real grok 1.0.41 also returns models here (and via notifications).
		Models *struct {
			CurrentModelID string `json:"currentModelId"`
			Available      []struct {
				ModelID     string `json:"modelId"`
				Name        string `json:"name"`
				Description string `json:"description"`
				Meta        *struct {
					TotalContextTokens int `json:"totalContextTokens"`
				} `json:"_meta"`
			} `json:"availableModels"`
		} `json:"models"`
	}
	if err := json.Unmarshal(res, &out); err != nil {
		return "", err
	}
	if out.SessionID == "" {
		return "", fmt.Errorf("grok-cli: session/new returned no sessionId")
	}
	// Prefer configOptions (sets modelConfigID for set_config_option).
	// Then fill any gaps from the models object (real grok sends both).
	c.applyModelConfig(out.ConfigOptions)
	c.applySessionModels(out.Models)
	c.sessionID = out.SessionID
	return out.SessionID, nil
}

func (c *client) applySessionModels(models *struct {
	CurrentModelID string `json:"currentModelId"`
	Available      []struct {
		ModelID     string `json:"modelId"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Meta        *struct {
			TotalContextTokens int `json:"totalContextTokens"`
		} `json:"_meta"`
	} `json:"availableModels"`
}) {
	if models == nil {
		return
	}
	if cur := strings.TrimSpace(models.CurrentModelID); cur != "" {
		c.modelCurrent = cur
	}
	var list []ModelInfo
	for _, m := range models.Available {
		id := strings.TrimSpace(m.ModelID)
		if id == "" {
			continue
		}
		name := strings.TrimSpace(m.Name)
		if name == "" {
			name = id
		}
		info := ModelInfo{ID: id, Name: name, Description: strings.TrimSpace(m.Description)}
		if m.Meta != nil {
			info.ContextTokens = m.Meta.TotalContextTokens
		}
		list = append(list, info)
	}
	if len(list) > 0 {
		c.models = list
	}
}

func (c *client) applyModelConfig(options []json.RawMessage) {
	// Do not clear initialize _meta.modelState when session/new has no
	// model configOptions. Real grok 1.0.41 uses "id" (not only configId).
	for _, raw := range options {
		var opt struct {
			ID           string `json:"id"`
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
		cfgID := strings.TrimSpace(opt.ConfigID)
		if cfgID == "" {
			cfgID = strings.TrimSpace(opt.ID)
		}
		if strings.ToLower(opt.Category) != "model" && cfgID != "model" {
			continue
		}
		c.modelConfigID = cfgID
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
	return c.promptWithIdle(ctx, sessionID, text, PromptIdleTimeout)
}

func (c *client) promptWithIdle(ctx context.Context, sessionID, text string, idle time.Duration) error {
	c.mu.Lock()
	c.text.Reset()
	c.calls = nil
	c.sessionID = sessionID
	c.mu.Unlock()
	if idle <= 0 {
		idle = PromptIdleTimeout
	}
	// Watch for cancel while the prompt is in flight so the agent stops
	// work promptly; also send synchronously if the wait returns ctx.Err()
	// (covers the race where the wait returns first).
	doneWatch := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = c.sendCancel(sessionID)
		case <-doneWatch:
		}
	}()
	_, err := c.requestIdle(ctx, "session/prompt", map[string]any{
		"sessionId": sessionID,
		"prompt":    []map[string]string{{"type": "text", "text": text}},
	}, idle)
	close(doneWatch)
	if err != nil && ctx.Err() != nil {
		_ = c.sendCancel(sessionID)
	}
	return err
}

func (c *client) sendCancel(sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}
	debugGrok("send session/cancel sessionId=%s", sessionID)
	return c.write(rpc{
		JSONRPC: "2.0",
		Method:  "session/cancel",
		Params:  mustRaw(map[string]any{"sessionId": sessionID}),
	})
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
