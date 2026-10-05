package grokcli

import (
	"context"
	"encoding/json"
	"errors"
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
	// OverrideModel is config grok_model — a user pick from the child's
	// advertised list. It overrides Jev fast/strong mapping.
	OverrideModel string

	mu        sync.Mutex
	sess      *session
	wantModel string // pending -m restart target (last resort only)
}

type session struct {
	client *client
	id     string
}

// ModelInfo is one subscription model advertised by the grok child.
// Names and ids come from the child only — Rock never invents them.
// Prefer initialize _meta.modelState; fall back to session/new
// configOptions (category "model").
type ModelInfo struct {
	ID            string
	Name          string
	Description   string
	ContextTokens int
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

func (p *Provider) Complete(ctx context.Context, model string, messages []provider.Message, tools []provider.ToolSpec) (provider.Message, error) {
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
	target := MapGrokTarget(model, sess.client.models, sess.client.modelCurrent, p.OverrideModel)
	if target != "" && target != sess.client.modelCurrent {
		if err := p.switchModel(ctx, bin, sess, target); err != nil {
			return provider.Message{}, err
		}
		sess, err = p.ensure(ctx, bin)
		if err != nil {
			return provider.Message{}, err
		}
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

// switchModel prefers session/set_config_option, then session/set_model.
// Restarts with -m only when both fail and target is a child-advertised id
// (never OpenAI / Jev tier labels).
func (p *Provider) switchModel(ctx context.Context, bin string, sess *session, target string) error {
	err := sess.client.setModel(ctx, sess.id, target)
	if err == nil {
		return nil
	}
	if !errors.Is(err, errNeedRestartModel) {
		return err
	}
	if LooksLikeForeignModel(target) || target == "" {
		// Keep the live session on the child's current model.
		return nil
	}
	p.mu.Lock()
	if p.sess != nil {
		p.sess.client.Close()
		p.sess = nil
	}
	p.wantModel = target
	p.mu.Unlock()
	_, err = p.ensure(ctx, bin)
	return err
}

func (p *Provider) ensure(ctx context.Context, bin string) (*session, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sess != nil {
		return p.sess, nil
	}
	args := ChildArgsModel(p.wantModel)
	c, err := dial(ctx, bin, args, p.CWD, p.Start)
	if err != nil {
		return nil, err
	}
	c.modelArg = p.wantModel
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
	// -m may be the only switch path; trust it when initialize did not
	// echo the selection yet.
	if p.wantModel != "" && c.modelCurrent == "" {
		c.modelCurrent = p.wantModel
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
	// Only cache successful signed-in probes. Child crashes (bad flags,
	// unexpected args) must not stick for five minutes.
	if st.Found && st.SignedIn {
		saveProbeCache(st)
	}
	return st
}

// Models returns the subscription models the child advertised on
// initialize _meta.modelState and/or session/new (models + configOptions),
// plus the current selection. Empty when the child did not expose any —
// callers must not invent names.
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

// SetModel switches the subscription model. Order: session/set_config_option
// (id/configId "model"), then session/set_model, then restart with
// grok agent -m <model> --no-leader stdio (last resort only; never for
// OpenAI / Jev tier labels).
func (p *Provider) SetModel(ctx context.Context, modelID string) error {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return fmt.Errorf("grok-cli: empty model id")
	}
	if LooksLikeForeignModel(modelID) {
		return fmt.Errorf("grok-cli: refusing to set OpenAI/tier model %q on grok child", modelID)
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
	return p.switchModel(ctx, bin, sess, modelID)
}

// ContextLimit returns totalContextTokens for the current model, or 0.
func (p *Provider) ContextLimit(ctx context.Context) int {
	cur, models, err := p.Models(ctx)
	if err != nil {
		return 0
	}
	for _, m := range models {
		if m.ID == cur && m.ContextTokens > 0 {
			return m.ContextTokens
		}
	}
	if len(models) > 0 && models[0].ContextTokens > 0 {
		return models[0].ContextTokens
	}
	return 0
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
