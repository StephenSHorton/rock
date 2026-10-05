package grokcli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/StephenSHorton/rock/internal/provider"
)

// Provider is the grok-cli model backend. Rock's harness still runs
// tools, ask_jev, and the Risk gate. The child only produces text (and
// optional proposed tool calls that Rock executes after denying the child).
type Provider struct {
	Bin     string
	CWD     string
	Start   StartFunc
	Timeout time.Duration

	mu   sync.Mutex
	sess *session
}

type session struct {
	client *client
	id     string
}

// ModelInfo is one subscription model advertised by the grok child via
// ACP session config options (category "model"). Names come from the
// child only — Rock never invents them.
type ModelInfo struct {
	ID   string
	Name string
}

func (p *Provider) Name() string { return AuthClass }

func (p *Provider) denied() (perm, fs int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sess == nil {
		return 0, 0
	}
	return p.sess.client.permN.Load(), p.sess.client.fsN.Load()
}

func (p *Provider) timeout() time.Duration {
	if p.Timeout > 0 {
		return p.Timeout
	}
	return 8 * time.Second
}

func (p *Provider) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sess != nil {
		p.sess.client.Close()
		p.sess = nil
	}
}

func (p *Provider) Complete(ctx context.Context, _ string, messages []provider.Message, tools []provider.ToolSpec) (provider.Message, error) {
	bin := p.Bin
	if p.Start == nil {
		st := Look(p.Bin)
		if !st.Found {
			return provider.Message{}, fmt.Errorf("%s", st.Detail)
		}
		bin = st.Bin
	}
	ctx, cancel := withTimeout(ctx, p.timeout())
	defer cancel()
	sess, err := p.ensure(ctx, bin)
	if err != nil {
		return provider.Message{}, err
	}
	if err := sess.client.prompt(ctx, sess.id, renderPrompt(messages, tools)); err != nil {
		return provider.Message{}, err
	}
	text, proposed := sess.client.snapshot()
	msg := provider.Message{Role: provider.RoleAssistant, Content: strings.TrimSpace(stripToolFences(text))}
	if calls := parseToolFences(text); len(calls) > 0 {
		msg.ToolCalls = calls
		if msg.Content == strings.TrimSpace(text) {
			msg.Content = ""
		}
	} else if calls := mapProposed(proposed); len(calls) > 0 {
		msg.ToolCalls = calls
	}
	return msg, nil
}

func (p *Provider) ensure(ctx context.Context, bin string) (*session, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sess != nil {
		return p.sess, nil
	}
	c, err := dial(ctx, bin, ChildArgs(), p.CWD, p.Start)
	if err != nil {
		return nil, err
	}
	methods, err := c.initialize(ctx)
	if err != nil {
		c.Close()
		return nil, err
	}
	if err := c.authenticate(ctx, methods); err != nil {
		c.Close()
		return nil, err
	}
	id, err := c.newSession(ctx)
	if err != nil {
		c.Close()
		return nil, err
	}
	p.sess = &session{client: c, id: id}
	return p.sess, nil
}

// ProbeSignedIn starts a short ACP initialize against bin and reports
// whether grok offered cached_token (or xai.api_key with XAI_API_KEY set).
// Timeout defaults to ProbeTimeout (2s) so auto-detect stays snappy.
func ProbeSignedIn(ctx context.Context, bin, cwd string, start StartFunc) Status {
	st := Look(bin)
	if !st.Found {
		return st
	}
	ctx, cancel := withTimeout(ctx, ProbeTimeout)
	defer cancel()
	c, err := dial(ctx, st.Bin, ChildArgs(), cwd, start)
	if err != nil {
		st.Detail = err.Error()
		return st
	}
	defer c.Close()
	methods, err := c.initialize(ctx)
	if err != nil {
		st.Detail = err.Error()
		return st
	}
	has := map[string]bool{}
	for _, m := range methods {
		has[m] = true
	}
	if has["cached_token"] || (has["xai.api_key"] && strings.TrimSpace(os.Getenv("XAI_API_KEY")) != "") {
		st.SignedIn = true
		st.Detail = "official grok binary (signed in)"
		return st
	}
	if len(methods) == 0 {
		// Some fakes and older agents skip authenticate.
		st.SignedIn = true
		st.Detail = "official grok binary"
		return st
	}
	st.Detail = notSignedIn()
	return st
}

// ProbeSignedInCached is ProbeSignedIn with a brief ROCK_HOME cache so
// repeated Open() calls do not re-spawn grok. Cache miss pays ProbeTimeout
// once; hits are instantaneous and keep the TUI's first frame snappy.
func ProbeSignedInCached(ctx context.Context, bin, cwd string, start StartFunc) Status {
	st := Look(bin)
	if !st.Found {
		return st
	}
	if cached, ok := loadProbeCache(st.Bin); ok {
		return cached
	}
	st = ProbeSignedIn(ctx, st.Bin, cwd, start)
	if st.Found {
		saveProbeCache(st)
	}
	return st
}

// Models returns the subscription models the child advertised on
// session/new, plus the current selection. Empty when the child did not
// expose a model config option — callers should fall back to showing
// whatever FastModel/current label they already have, not invent names.
func (p *Provider) Models(ctx context.Context) (current string, models []ModelInfo, err error) {
	bin := p.Bin
	if p.Start == nil {
		st := Look(p.Bin)
		if !st.Found {
			return "", nil, fmt.Errorf("%s", st.Detail)
		}
		bin = st.Bin
	}
	ctx, cancel := withTimeout(ctx, p.timeout())
	defer cancel()
	sess, err := p.ensure(ctx, bin)
	if err != nil {
		return "", nil, err
	}
	return sess.client.modelCurrent, append([]ModelInfo(nil), sess.client.models...), nil
}

// SetModel switches the child model through session/set_config_option.
// No-op with a clear error when the child never advertised a model option.
func (p *Provider) SetModel(ctx context.Context, modelID string) error {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return fmt.Errorf("grok-cli: empty model id")
	}
	bin := p.Bin
	if p.Start == nil {
		st := Look(p.Bin)
		if !st.Found {
			return fmt.Errorf("%s", st.Detail)
		}
		bin = st.Bin
	}
	ctx, cancel := withTimeout(ctx, p.timeout())
	defer cancel()
	sess, err := p.ensure(ctx, bin)
	if err != nil {
		return err
	}
	if sess.client.modelConfigID == "" {
		return fmt.Errorf("grok-cli: child did not advertise a model list over ACP")
	}
	return sess.client.setModel(ctx, sess.id, modelID)
}

func renderPrompt(messages []provider.Message, tools []provider.ToolSpec) string {
	var b strings.Builder
	b.WriteString("You are the SuperGrok model backend for Rock. ")
	b.WriteString("Rock owns the agent loop, tool execution, ask_jev, and the Jev Risk gate. ")
	b.WriteString("Do not execute tools yourself. If you need a Rock tool, reply with one or more fenced blocks:\n\n")
	b.WriteString("```tool\n{\"name\":\"read_file\",\"arguments\":{\"path\":\"README.md\"}}\n```\n\n")
	if len(tools) > 0 {
		b.WriteString("Rock tools:\n")
		for _, t := range tools {
			b.WriteString("- ")
			b.WriteString(t.Name)
			if t.Description != "" {
				b.WriteString(": ")
				b.WriteString(t.Description)
			}
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	for _, m := range messages {
		switch m.Role {
		case provider.RoleSystem:
			b.WriteString("System: ")
			b.WriteString(m.Content)
			b.WriteByte('\n')
		case provider.RoleUser:
			b.WriteString("User: ")
			b.WriteString(m.Content)
			b.WriteByte('\n')
		case provider.RoleAssistant:
			b.WriteString("Assistant: ")
			b.WriteString(m.Content)
			if len(m.ToolCalls) > 0 {
				raw, _ := json.Marshal(m.ToolCalls)
				b.WriteString("\n")
				b.Write(raw)
			}
			b.WriteByte('\n')
		case provider.RoleTool:
			b.WriteString("Tool ")
			b.WriteString(m.Name)
			b.WriteString(": ")
			b.WriteString(m.Content)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func parseToolFences(text string) []provider.ToolCall {
	var calls []provider.ToolCall
	rest := text
	for {
		i := strings.Index(rest, "```tool")
		if i < 0 {
			break
		}
		rest = rest[i+len("```tool"):]
		rest = strings.TrimLeft(rest, "\n")
		end := strings.Index(rest, "```")
		if end < 0 {
			break
		}
		body := strings.TrimSpace(rest[:end])
		rest = rest[end+3:]
		var raw struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal([]byte(body), &raw); err != nil || raw.Name == "" {
			continue
		}
		args := strings.TrimSpace(string(raw.Arguments))
		if args == "" {
			args = "{}"
		}
		if n := len(args); n >= 2 && args[0] == '"' && args[n-1] == '"' {
			var s string
			if json.Unmarshal(raw.Arguments, &s) == nil {
				args = s
			}
		}
		calls = append(calls, provider.ToolCall{
			ID:        fmt.Sprintf("grok-%d", len(calls)+1),
			Name:      mapToolName(raw.Name),
			Arguments: args,
		})
	}
	return calls
}

func stripToolFences(text string) string {
	var b strings.Builder
	rest := text
	for {
		i := strings.Index(rest, "```tool")
		if i < 0 {
			b.WriteString(rest)
			break
		}
		b.WriteString(rest[:i])
		rest = rest[i+len("```tool"):]
		end := strings.Index(rest, "```")
		if end < 0 {
			break
		}
		rest = rest[end+3:]
	}
	return strings.TrimSpace(b.String())
}

func mapProposed(in []proposed) []provider.ToolCall {
	var calls []provider.ToolCall
	for i, p := range in {
		name := mapToolName(p.Name)
		if name == "" {
			continue
		}
		id := p.ID
		if id == "" {
			id = fmt.Sprintf("grok-%d", i+1)
		}
		args := p.Args
		if args == "" {
			args = "{}"
		}
		calls = append(calls, provider.ToolCall{ID: id, Name: name, Arguments: args})
	}
	return calls
}

func mapToolName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.ReplaceAll(n, " ", "_")
	switch n {
	case "bash", "run", "terminal", "command":
		return "shell"
	case "read", "read_text", "readtextfile":
		return "read_file"
	case "write", "write_text", "writetextfile":
		return "write_file"
	case "edit", "str_replace", "apply_patch":
		return "edit_file"
	case "search":
		return "grep"
	default:
		return n
	}
}
