package jev

import (
	"context"
	"strings"
)

// Gates is the set of hard-wired decisions. Live Jev is used when a key is
// configured. Otherwise the same functions run a local policy and Mode reports
// "offline". Offline is not Jev.
type Gates struct {
	Client           *Client
	MinConfidence    float64
	RiskBlock        float64
	AllowDestructive bool
	// ForceOffline honors jev.enabled = false even when a key is set.
	ForceOffline bool
}

func (g Gates) Mode() string {
	if g.ForceOffline {
		return "offline"
	}
	if g.Client != nil && g.Client.Live() {
		return "live"
	}
	return "offline"
}

func (g Gates) minConf() float64 {
	if g.MinConfidence <= 0 {
		return 0.55
	}
	return g.MinConfidence
}

func (g Gates) riskAt() float64 {
	if g.RiskBlock <= 0 {
		return 0.72
	}
	return g.RiskBlock
}

// Turn is one round trip: model size, which skills to load, whether the loop
// is stuck, and how heavy the transcript feels.
type Turn struct {
	Model   string
	Skills  []string
	Stuck   bool
	Compact bool
	Source  string
	Detail  string
	Result  Result
}

func (g Gates) BeforeTurn(ctx context.Context, prompt string, skillNames []string, recentTools []string, transcriptBytes int) Turn {
	out := Turn{Model: "fast", Source: g.Mode()}
	state := map[string]any{
		"prompt":           Clip(prompt, 2000),
		"recent_tools":     recentTools,
		"transcript_bytes": transcriptBytes,
	}
	if g.Mode() == "live" {
		res := g.Ask(ctx, state, TurnQueries(skillNames))
		out.Result = res
		if res.Error != "" {
			out.Source = "offline"
			out.Detail = "jev error: " + res.Error
			out.Model, out.Skills, out.Stuck, out.Compact = offlineTurn(prompt, skillNames, recentTools, transcriptBytes)
			return out
		}
		if v, ok := AnswerString(res.Answers["model"]); ok && res.Answers["model"].Confidence >= g.minConf() && (v == "fast" || v == "strong") {
			out.Model = v
		}
		if n, ok := AnswerFloat(res.Answers["stuck"]); ok {
			out.Stuck = n >= g.riskAt()
		}
		if n, ok := AnswerFloat(res.Answers["weight"]); ok {
			out.Compact = n >= 2.5
		}
		if v, ok := AnswerString(res.Answers["skill"]); ok && v != "" && v != "none" && res.Answers["skill"].Confidence >= g.minConf() {
			out.Skills = []string{v}
		}
		out.Detail = "jev " + res.Model
		return out
	}
	out.Model, out.Skills, out.Stuck, out.Compact = offlineTurn(prompt, skillNames, recentTools, transcriptBytes)
	out.Detail = "offline policy"
	return out
}

func offlineTurn(prompt string, skills, recent []string, bytes int) (model string, chosen []string, stuck, compact bool) {
	low := strings.ToLower(prompt)
	model = "fast"
	for _, word := range []string{"architect", "refactor", "design", "migrate", "debug", "race"} {
		if strings.Contains(low, word) {
			model = "strong"
			break
		}
	}
	if len(prompt) > 800 {
		model = "strong"
	}
	for _, name := range skills {
		if strings.Contains(low, strings.ToLower(name)) {
			chosen = []string{name}
			break
		}
	}
	stuck = repeating(recent)
	compact = bytes > 24_000
	return model, chosen, stuck, compact
}

func repeating(tools []string) bool {
	if len(tools) < 3 {
		return false
	}
	last := tools[len(tools)-1]
	same := 1
	for i := len(tools) - 2; i >= 0; i-- {
		if tools[i] != last {
			break
		}
		same++
	}
	return same >= 3
}

type RiskDecision struct {
	Block  bool
	P      float64
	Source string
	Result Result
}

// Risk is the destructive-action gate. Yolo does not bypass a block.
// Go always calls this; the agent cannot skip it.
func (g Gates) Risk(ctx context.Context, tool, args string) (block bool, p float64, source string) {
	d := g.DecideRisk(ctx, tool, args)
	return d.Block, d.P, d.Source
}

func (g Gates) DecideRisk(ctx context.Context, tool, args string) RiskDecision {
	out := RiskDecision{Source: g.Mode()}
	if out.Source == "live" {
		out.Result = g.Ask(ctx, map[string]string{"tool": tool, "args": Clip(args, 1500)}, []Query{RiskQuery()})
		if out.Result.Error == "" {
			if n, ok := AnswerFloat(out.Result.Answers["risk"]); ok {
				out.P = n
				out.Block = !g.AllowDestructive && n >= g.riskAt()
				return out
			}
		}
		out.Source = "offline"
	}
	out.P = offlineRisk(tool, args)
	out.Block = !g.AllowDestructive && out.P >= g.riskAt()
	return out
}

func offlineRisk(tool, args string) float64 {
	low := strings.ToLower(args)
	for _, bad := range []string{"rm -rf", "sudo ", "curl |", "wget |", "chmod 777", "mkfs", "dd if=", ":(){", "> /dev/"} {
		if strings.Contains(low, bad) {
			return 0.95
		}
	}
	switch {
	case tool == "shell":
		return 0.2
	case tool == "write_file" || tool == "edit_file" || strings.HasPrefix(tool, "mcp_"):
		return 0.15
	default:
		return 0.05
	}
}

type KindDecision struct {
	Kind   string
	Result Result
}

// SubagentKind picks explore, plan, or general. Still called from Go.
func (g Gates) SubagentKind(ctx context.Context, prompt, requested string) string {
	return g.DecideSubagent(ctx, prompt, requested).Kind
}

func (g Gates) DecideSubagent(ctx context.Context, prompt, requested string) KindDecision {
	req := strings.ToLower(strings.TrimSpace(requested))
	if g.Mode() == "live" {
		res := g.Ask(ctx, map[string]string{"prompt": Clip(prompt, 1500), "requested": req}, []Query{SubagentQuery()})
		if res.Error == "" {
			if v, ok := AnswerString(res.Answers["kind"]); ok && res.Answers["kind"].Confidence >= g.minConf() {
				if v == "explore" || v == "plan" || v == "general" {
					return KindDecision{Kind: v, Result: res}
				}
			}
		} else {
			return KindDecision{Kind: offlineKind(req, prompt), Result: res}
		}
	}
	return KindDecision{Kind: offlineKind(req, prompt)}
}

func offlineKind(req, prompt string) string {
	if req == "explore" || req == "plan" || req == "general" {
		return req
	}
	low := strings.ToLower(prompt)
	switch {
	case strings.Contains(low, "plan"):
		return "plan"
	case strings.Contains(low, "find"), strings.Contains(low, "where"), strings.Contains(low, "search"):
		return "explore"
	default:
		return "general"
	}
}

type ReadyDecision struct {
	Ready  bool
	P      float64
	Result Result
}

// PlanReady is reported. It does not auto-approve.
func (g Gates) PlanReady(ctx context.Context, plan, request string) (bool, float64) {
	d := g.DecideReady(ctx, plan, request)
	return d.Ready, d.P
}

func (g Gates) DecideReady(ctx context.Context, plan, request string) ReadyDecision {
	if g.Mode() == "live" {
		res := g.Ask(ctx, map[string]string{"plan": Clip(plan, 2000), "request": Clip(request, 800)}, []Query{ReadyQuery()})
		if res.Error == "" {
			if n, ok := AnswerFloat(res.Answers["ready"]); ok {
				return ReadyDecision{Ready: n >= g.minConf(), P: n, Result: res}
			}
		} else {
			d := offlineReady(plan)
			d.Result = res
			return d
		}
	}
	return offlineReady(plan)
}

func offlineReady(plan string) ReadyDecision {
	ready := len(strings.TrimSpace(plan)) > 80 && strings.Contains(strings.ToLower(plan), "step")
	p := 0.3
	if ready {
		p = 0.8
	}
	return ReadyDecision{Ready: ready, P: p}
}
