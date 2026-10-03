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

	"github.com/StephenSHorton/rock/internal/jev"
	"github.com/StephenSHorton/rock/internal/perms"
	"github.com/StephenSHorton/rock/internal/provider"
	"github.com/StephenSHorton/rock/internal/repomap"
	"github.com/StephenSHorton/rock/internal/session"
	"github.com/StephenSHorton/rock/internal/skills"
	"github.com/StephenSHorton/rock/internal/tools"
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
)

type Event struct {
	Kind EventKind `json:"kind"`
	Name string    `json:"name,omitempty"`
	Text string    `json:"text,omitempty"`
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
}

type Harness struct {
	Options
}

func New(opt Options) *Harness {
	if opt.MaxSteps <= 0 {
		opt.MaxSteps = 12
	}
	if opt.Tools != nil {
		opt.Tools.Env.Decide = func(ctx context.Context, state, question, typ string, criteria map[string]string) (string, error) {
			return decideText(ctx, opt.Gates, state, question, typ, criteria)
		}
		opt.Tools.Env.KeepGrep = func(query, snippet string) bool {
			return opt.Gates.KeepSnippet(ctxBackground(), query, snippet)
		}
	}
	h := &Harness{Options: opt}
	if h.Tools != nil {
		h.Tools.Env.Subagent = h.subagent
	}
	return h
}

func ctxBackground() context.Context { return context.Background() }

func (h *Harness) Run(ctx context.Context, sess *session.Session, prompt string, sink func(Event)) error {
	if sink == nil {
		sink = func(Event) {}
	}
	if strings.TrimSpace(prompt) != "" {
		sess.Append(provider.Message{Role: provider.RoleUser, Content: prompt})
	}
	turn := h.Gates.BeforeTurn(ctx, prompt, skillNames(h.Skills), sess.RecentTools(), sess.Bytes())
	sink(Event{Kind: EvJev, Name: "turn", Text: fmt.Sprintf("%s model=%s stuck=%v compact=%v skills=%s (%s)", turn.Source, turn.Model, turn.Stuck, turn.Compact, strings.Join(turn.Skills, ","), turn.Detail)})
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
	sess.ReplaceSystem(h.system(prompt, turn.Skills))
	mutated := false
	for step := 0; step < h.MaxSteps; step++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		msg, err := h.Provider.Complete(ctx, model, sess.Messages, h.Tools.Specs())
		if err != nil {
			sink(Event{Kind: EvStatus, Text: err.Error()})
			return err
		}
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
				block, p, source := h.Gates.Risk(ctx, call.Name, call.Arguments)
				sink(Event{Kind: EvJev, Name: "risk", Text: fmt.Sprintf("%s p=%.2f block=%v", source, p, block)})
				if block {
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
			sess.Append(provider.Message{Role: provider.RoleTool, ToolCallID: call.ID, Name: call.Name, Content: text})
			sink(Event{Kind: EvToolResult, Name: call.Name, Text: text})
		}
	}
	sink(Event{Kind: EvDone, Text: "max_steps"})
	return sess.Save()
}

func (h *Harness) system(prompt string, chosen []string) string {
	var b strings.Builder
	b.WriteString("You are Rock, a coding agent in this workspace. Use tools to read and change files. ")
	b.WriteString("Prefer a small patch. In plan mode, write the plan with update_plan and do not edit other files. ")
	b.WriteString("Shell is not a sandbox. Hard gates already judge risk, model size, and skills. ")
	b.WriteString("Call jev_decide for an extra bounded question. Do not use it to draft code.\n")
	b.WriteString("Mode: " + string(h.Policy.Mode) + "\n")
	if h.Tools != nil {
		b.WriteString("Workspace: " + h.Tools.Env.Root + "\n")
	}
	if hits, err := repomap.Build(h.Tools.Env.Root, prompt, 8); err == nil && len(hits) > 0 {
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
	kind = h.Gates.SubagentKind(ctx, prompt, kind)
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
	if err := add.Run(); err != nil {
		sink(Event{Kind: EvStatus, Text: "checkpoint add: " + err.Error()})
		return
	}
	commit := exec.CommandContext(ctx, "git", "commit", "-m", "rock checkpoint")
	commit.Dir = cwd
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
	case "spawn_subagent":
		if s, ok := raw["kind"].(string); ok {
			return s
		}
		return "general"
	}
	return call.Name
}

func decideText(ctx context.Context, g jev.Gates, state, question, typ string, criteria map[string]string) (string, error) {
	if g.Mode() != "live" {
		return "offline: no Jev key, so this question was not sent. " + question, nil
	}
	var q jev.Question
	switch typ {
	case "choice":
		if len(criteria) == 0 {
			criteria = map[string]string{"yes": "yes", "no": "no"}
		}
		q = jev.ChoiceQ(question, criteria)
	case "score":
		levels := []string{"low", "medium", "high"}
		if len(criteria) > 0 {
			levels = levels[:0]
			for k := range criteria {
				levels = append(levels, k)
			}
		}
		q = jev.ScoreQ(question, levels)
	default:
		q = jev.NoulQ(question)
	}
	res, err := g.Client.Decide(ctx, state, map[string]jev.Question{"q": q})
	if err != nil {
		return "", err
	}
	raw, _ := json.Marshal(res.Answers["q"])
	return string(raw), nil
}

func addWorktree(ctx context.Context, root string) (string, error) {
	id := session.NewID()
	dest := filepath.Join(root, ".rock", "worktrees", id)
	if err := os.MkdirAll(filepath.Join(root, ".rock", "worktrees"), 0o755); err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", "rock/"+id, dest, "HEAD")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("worktree: %s", strings.TrimSpace(string(out)))
	}
	return dest, nil
}

func removeWorktree(ctx context.Context, root, dest string) error {
	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", "--force", dest)
	cmd.Dir = root
	return cmd.Run()
}
