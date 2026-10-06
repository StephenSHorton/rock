// Package harness is the agent loop. The TUI, ACP, and HTTP server all call Run.
package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/StephenSHorton/rock/internal/childproc"
	"github.com/StephenSHorton/rock/internal/config"
	"github.com/StephenSHorton/rock/internal/jev"
	"github.com/StephenSHorton/rock/internal/perms"
	"github.com/StephenSHorton/rock/internal/provider"
	"github.com/StephenSHorton/rock/internal/repomap"
	"github.com/StephenSHorton/rock/internal/session"
	"github.com/StephenSHorton/rock/internal/skills"
	"github.com/StephenSHorton/rock/internal/tools"
	"github.com/StephenSHorton/rock/internal/trace"
)

type EventKind string

const (
	EvAssistant  EventKind = "assistant"
	EvToolCall   EventKind = "tool_call"
	EvToolResult EventKind = "tool_result"
	EvStatus     EventKind = "status"
	EvJev        EventKind = "jev"
	EvPermission EventKind = "permission"
	EvDone       EventKind = "done"
	// EvThought / EvDelta are live deltas (reasoning / reply text) while a
	// model call streams. Only sent when Options.Stream is set. The final
	// reply still arrives as EvAssistant and is authoritative.
	EvThought EventKind = "thought"
	EvDelta   EventKind = "delta"
)

type Event struct {
	Kind        EventKind `json:"kind"`
	Name        string    `json:"name,omitempty"`
	Text        string    `json:"text,omitempty"`
	ClipsBefore int       `json:"clips_before,omitempty"`
	ClipsAfter  int       `json:"clips_after,omitempty"`
	BytesBefore int       `json:"bytes_before,omitempty"`
	BytesAfter  int       `json:"bytes_after,omitempty"`
}

type AskFunc func(ctx context.Context, tool, detail string) perms.Decision

type Options struct {
	Provider    provider.Provider
	FastModel   string
	StrongModel string
	Policy      perms.Policy
	Gates       jev.Gates
	Tools       *tools.Set
	Skills      []skills.Skill
	MaxSteps    int
	Checkpoint  bool
	Ask         AskFunc
	Depth       int
	// NudgeEvery is how often a validation hint is appended after a
	// successful edit/write or an allowed shell. 0 means the product
	// default (2). Negative disables. Hints are text; they never call Jev.
	NudgeEvery int
	// TriageOff skips the failed-shell classification hint.
	TriageOff bool
	// FilterOff skips KeepSnippet / grep filtering.
	FilterOff bool
	ClipBytes int
	// Stream forwards provider deltas as EvThought / EvDelta events.
	Stream bool
}

type Harness struct {
	Options
	nudgeCount int
	emit       func(Event)
}

func New(opt Options) *Harness {
	if opt.MaxSteps <= 0 {
		opt.MaxSteps = 12
	}
	if opt.Tools != nil {
		opt.Tools.Env.Ask = func(ctx context.Context, state any, questions []jev.Query) (jev.Result, error) {
			return opt.Gates.Ask(ctx, state, questions), nil
		}
		opt.Tools.Env.ClipBytes = jev.ClipSize(opt.ClipBytes)
		opt.Tools.Env.MinConfidence = opt.Gates.MinConfidence
		if !opt.FilterOff {
			opt.Tools.Env.FilterSnippets = func(query string, snippets []string) []string {
				return opt.Gates.FilterSnippets(ctxBackground(), query, snippets)
			}
			opt.Tools.Env.KeepGrep = func(query, snippet string) bool {
				return opt.Gates.KeepSnippet(ctxBackground(), query, snippet)
			}
		}
	}
	h := &Harness{Options: opt}
	if h.Tools != nil {
		h.Tools.Env.Subagent = h.subagent
	}
	return h
}

func ctxBackground() context.Context { return context.Background() }

// streamCtx attaches a provider stream callback that turns deltas into
// EvThought / EvDelta events when Options.Stream is on.
func (h *Harness) streamCtx(ctx context.Context, sink func(Event)) context.Context {
	if !h.Stream || sink == nil {
		return ctx
	}
	return provider.WithStream(ctx, func(kind provider.StreamKind, delta string) {
		ev := Event{Kind: EvDelta, Text: delta}
		if kind == provider.StreamThought {
			ev.Kind = EvThought
		}
		sink(ev)
	})
}

func (h *Harness) Run(ctx context.Context, sess *session.Session, prompt string, sink func(Event)) error {
	if sink == nil {
		sink = func(Event) {}
	}
	h.emit = sink
	defer func() { h.emit = nil }()
	if strings.TrimSpace(prompt) != "" {
		sess.Append(provider.Message{Role: provider.RoleUser, Content: prompt})
	}
	routeAt := time.Now()
	trace.Info("turn jev route start", "jev", h.Gates.Mode())
	turn := h.Gates.BeforeTurn(ctx, prompt, skillNames(h.Skills), sess.RecentTools(), sess.Bytes())
	trace.Info("turn jev route done", "ms", trace.Since(routeAt), "source", turn.Source, "model", turn.Model)
	sink(jevEvent("turn", turn.Result, fmt.Sprintf("%s model=%s stuck=%v compact=%v skills=%s (%s)", turn.Source, turn.Model, turn.Stuck, turn.Compact, strings.Join(turn.Skills, ","), turn.Detail)))
	if turn.Stuck {
		sess.Append(provider.Message{Role: provider.RoleAssistant, Content: "Stopping. The last tool calls repeated without progress."})
		sink(Event{Kind: EvAssistant, Text: sess.LastAssistant()})
		sink(Event{Kind: EvDone, Text: "stuck"})
		return sess.Save()
	}
	if turn.Compact {
		sess.Compact("Earlier turns were folded to stay inside the window.", 6)
		sink(Event{Kind: EvStatus, Text: "compacted the transcript"})
	}
	model := h.FastModel
	if turn.Model == "strong" && h.StrongModel != "" {
		model = h.StrongModel
	}
	if model == "" {
		model = h.StrongModel
	}
	ctxAt := time.Now()
	trace.Info("turn context start")
	sess.ReplaceSystem(h.system(prompt, turn.Skills))
	trace.Info("turn context done", "ms", trace.Since(ctxAt))
	mutated := false
	for step := 0; step < h.MaxSteps; step++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		callAt := time.Now()
		trace.Info("turn model call start", "step", step, "model", model, "provider", h.Provider.Name())
		msg, err := h.Provider.Complete(h.streamCtx(ctx, sink), model, sess.Messages, h.Tools.Specs())
		if err != nil {
			trace.Error("turn model call error", "step", step, "ms", trace.Since(callAt), "err", err.Error())
			sink(Event{Kind: EvStatus, Text: err.Error()})
			return err
		}
		trace.Info("turn model call done", "step", step, "ms", trace.Since(callAt), "chars", len(msg.Content), "tools", len(msg.ToolCalls))
		sess.Append(msg)
		if msg.Content != "" {
			sink(Event{Kind: EvAssistant, Text: msg.Content})
		}
		if len(msg.ToolCalls) == 0 {
			sink(Event{Kind: EvDone, Text: "end_turn"})
			if mutated {
				h.maybeCheckpoint(ctx, sess.Meta.CWD, sink)
			}
			return sess.Save()
		}
		for _, call := range msg.ToolCalls {
			sink(Event{Kind: EvToolCall, Name: call.Name, Text: call.Arguments})
			detail := detailFor(call)
			decision, why := h.Policy.Decide(call.Name, detail)
			if decision == perms.Ask {
				if h.Ask == nil {
					decision = perms.Deny
					why = "nobody is available to approve"
				} else {
					decision = h.Ask(ctx, call.Name, detail)
					why = "you answered " + string(decision)
				}
			}
			sink(Event{Kind: EvPermission, Name: call.Name, Text: string(decision) + ": " + why})
			if decision != perms.Deny && riskyCall(call.Name) {
				risk := h.Gates.DecideRisk(ctx, call.Name, call.Arguments)
				sink(jevEvent("risk", risk.Result, fmt.Sprintf("%s p=%.2f block=%v", risk.Source, risk.P, risk.Block)))
				if risk.Block {
					decision = perms.Deny
					why = "destructive-action gate"
				}
			}
			if decision == perms.Deny {
				sess.Append(provider.Message{Role: provider.RoleTool, ToolCallID: call.ID, Name: call.Name, Content: "denied: " + why})
				sink(Event{Kind: EvToolResult, Name: call.Name, Text: "denied: " + why})
				continue
			}
			out, err := h.Tools.Run(ctx, call.Name, call.Arguments)
			text := out.Output
			if err != nil {
				if text == "" {
					text = err.Error()
				} else {
					text = text + "\n" + err.Error()
				}
			}
			if out.Mutates && err == nil {
				mutated = true
			}
			if hint := h.takeNudge(nudgeFor(call.Name, err)); hint != "" {
				text = appendHint(text, hint)
			}
			if call.Name == "shell" && err != nil {
				if line, ev := h.classifyFailure(ctx, text); line != "" {
					text = appendHint(text, line)
					sink(ev)
				}
			}
			if out.Filter != nil && !out.Filter.Zero() {
				text = appendHint(text, "Jev filter: "+out.Filter.Text())
				sink(filterEvent(call.Name, out.Filter))
			}
			sess.Append(provider.Message{Role: provider.RoleTool, ToolCallID: call.ID, Name: call.Name, Content: text})
			sink(Event{Kind: EvToolResult, Name: call.Name, Text: text})
			if call.Name == "ask_jev" && err == nil {
				sink(Event{Kind: EvJev, Name: "ask", Text: askEventText(text)})
			}
		}
	}
	sink(Event{Kind: EvDone, Text: "max_steps"})
	return sess.Save()
}

func (h *Harness) system(prompt string, chosen []string) string {
	var b strings.Builder
	b.WriteString("You are Rock, a coding agent in this workspace. Use tools to read and change files. ")
	b.WriteString("Prefer a small patch. In plan mode, write the plan with update_plan and do not edit other files. ")
	b.WriteString("Shell is not a sandbox. Hard gates still block destructive shell. ")
	b.WriteString("Call ask_jev to classify, verify a fix, filter or triage, or check risk. ")
	b.WriteString("After an edit or a proposed fix, and before a risky shell, consider ask_jev — is the failure type resolved? is the change too risky or too broad? You decide whether to call it. ")
	b.WriteString("When a shell or test fails, classify the failure with ask_jev (type, likely area, retry?) from a short excerpt before dumping the whole log into the next completion. ")
	b.WriteString("When grep, glob, or read returns many hits, pass paths to ask_jev so Rock clips the files into state — Jev does not open files — and filter against the task. Reason about the class or the kept clips, not the pile. ")
	b.WriteString("Batch questions. Do not draft code with it.\n")
	b.WriteString("Mode: " + string(h.Policy.Mode) + "\n")
	if h.Tools != nil {
		b.WriteString("Workspace: " + h.Tools.Env.Root + "\n")
	}
	hits, st, err := repomap.BuildStats(h.Tools.Env.Root, prompt, 8)
	trace.Info("turn repo map", "ms", st.Elapsed.Milliseconds(), "visited", st.Visited, "read", st.Read, "hits", len(hits), "stopped", st.Stopped)
	if err == nil && len(hits) > 0 {
		b.WriteString("Repo map:\n")
		b.WriteString(repomap.Render(hits))
	}
	loaded := skills.ByName(h.Skills, chosen)
	for _, sk := range loaded {
		b.WriteString("\nSkill " + sk.Name + ": " + sk.Description + "\n")
		b.WriteString(sk.Body)
		b.WriteString("\n")
	}
	return b.String()
}

func (h *Harness) subagent(ctx context.Context, prompt, kind string, worktree bool) (string, error) {
	if h.Depth >= 1 {
		return "", fmt.Errorf("subagent depth is 1")
	}
	picked := h.Gates.DecideSubagent(ctx, prompt, kind)
	kind = picked.Kind
	if h.emit != nil {
		h.emit(jevEvent("kind", picked.Result, "kind="+kind))
	}
	root := h.Tools.Env.Root
	cleanup := func() {}
	if worktree {
		wt, err := addWorktree(ctx, root)
		if err != nil {
			return "", err
		}
		root = wt
		cleanup = func() { _ = removeWorktree(context.Background(), h.Tools.Env.Root, wt) }
	}
	defer cleanup()
	policy := h.Policy
	switch kind {
	case "explore":
		policy.ReviewOnly = true
		policy.Mode = perms.ModeDefault
	case "plan":
		policy.Mode = perms.ModePlan
	}
	childTools := tools.New(tools.Env{Root: root, PlanPath: filepath.Join(root, ".rock", "plan.md")})
	child := New(Options{
		Provider:    h.Provider,
		FastModel:   h.FastModel,
		StrongModel: h.StrongModel,
		Policy:      policy,
		Gates:       h.Gates,
		Tools:       childTools,
		Skills:      h.Skills,
		MaxSteps:    h.MaxSteps,
		Ask:         func(context.Context, string, string) perms.Decision { return perms.Deny },
		Depth:       h.Depth + 1,
		NudgeEvery:  h.NudgeEvery,
		TriageOff:   h.TriageOff,
		FilterOff:   h.FilterOff,
		ClipBytes:   h.ClipBytes,
	})
	sess, err := session.Create(root, "", "subagent "+kind)
	if err != nil {
		return "", err
	}
	sess.Meta.Mode = string(policy.Mode)
	var last string
	err = child.Run(ctx, sess, prompt, func(ev Event) {
		if ev.Kind == EvAssistant {
			last = ev.Text
		}
	})
	if last == "" {
		last = sess.LastAssistant()
	}
	if err != nil {
		return last, err
	}
	return "[" + kind + "] " + last, nil
}

func (h *Harness) maybeCheckpoint(ctx context.Context, cwd string, sink func(Event)) {
	if !h.Checkpoint {
		return
	}
	if _, err := os.Stat(filepath.Join(cwd, ".git")); err != nil {
		return
	}
	add := exec.CommandContext(ctx, "git", "add", "-A")
	add.Dir = cwd
	config.ScrubCmdEnv(add)
	childproc.Isolate(add)
	if err := add.Run(); err != nil {
		sink(Event{Kind: EvStatus, Text: "checkpoint add: " + err.Error()})
		return
	}
	commit := exec.CommandContext(ctx, "git", "commit", "-m", "rock checkpoint")
	commit.Dir = cwd
	config.ScrubCmdEnv(commit)
	childproc.Isolate(commit)
	out, err := commit.CombinedOutput()
	if err != nil {
		sink(Event{Kind: EvStatus, Text: "checkpoint: " + strings.TrimSpace(string(out))})
		return
	}
	sink(Event{Kind: EvStatus, Text: "git checkpoint"})
}

func riskyCall(name string) bool {
	switch name {
	case "shell", "write_file", "edit_file":
		return true
	default:
		return strings.HasPrefix(name, "mcp_")
	}
}

func skillNames(all []skills.Skill) []string {
	out := make([]string, 0, len(all))
	for _, s := range all {
		out = append(out, s.Name)
	}
	return out
}

func detailFor(call provider.ToolCall) string {
	var raw map[string]any
	_ = json.Unmarshal([]byte(call.Arguments), &raw)
	switch call.Name {
	case "shell":
		if s, ok := raw["command"].(string); ok {
			return s
		}
	case "edit_file", "write_file", "read_file":
		if s, ok := raw["path"].(string); ok {
			return s
		}
	case "ask_jev":
		return tools.AskDetail(raw)
	case "spawn_subagent":
		if s, ok := raw["kind"].(string); ok {
			return s
		}
		return "general"
	}
	return call.Name
}

func jevEvent(name string, res jev.Result, extra string) Event {
	return Event{Kind: EvJev, Name: name, Text: attachJevExtra(res.Line(), extra)}
}

func attachJevExtra(text, extra string) string {
	text = strings.TrimSpace(text)
	extra = strings.TrimSpace(extra)
	if extra == "" {
		return text
	}
	if text == "" || text == "jev" {
		return extra
	}
	first, rest, ok := strings.Cut(text, "\n")
	first = strings.TrimSpace(first + " " + extra)
	if !ok {
		return first
	}
	return first + "\n" + rest
}

func askEventText(output string) string {
	var res jev.Result
	if err := json.Unmarshal([]byte(output), &res); err != nil {
		return strings.TrimSpace(output)
	}
	return res.Line()
}

func addWorktree(ctx context.Context, root string) (string, error) {
	id := session.NewID()
	dest := filepath.Join(root, ".rock", "worktrees", id)
	if err := os.MkdirAll(filepath.Join(root, ".rock", "worktrees"), 0o755); err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", "rock/"+id, dest, "HEAD")
	cmd.Dir = root
	config.ScrubCmdEnv(cmd)
	childproc.Isolate(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("worktree: %s", strings.TrimSpace(string(out)))
	}
	return dest, nil
}

func removeWorktree(ctx context.Context, root, dest string) error {
	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", "--force", dest)
	cmd.Dir = root
	config.ScrubCmdEnv(cmd)
	childproc.Isolate(cmd)
	return cmd.Run()
}
