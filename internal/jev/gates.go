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
}

func (g Gates) Mode() string {
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
}

func (g Gates) BeforeTurn(ctx context.Context, prompt string, skillNames []string, recentTools []string, transcriptBytes int) Turn {
	out := Turn{Model: "fast", Source: g.Mode()}
	if g.Mode() == "live" {
		qs := map[string]Question{
			"model": ChoiceQ("Which model should complete this coding turn?", map[string]string{
				"fast":   "small edit, lookup, or a narrow question",
				"strong": "design, multi-file change, or ambiguous debugging",
			}),
			"stuck":  NoulQ("Are these recent tool calls repeating without progress?"),
			"weight": ScoreQ("How heavy is this transcript?", []string{"light", "fine", "tight", "compact now"}),
		}
		if len(skillNames) > 0 {
			crit := map[string]string{"none": "no skill applies"}
			for _, name := range skillNames {
				if len(crit) >= 12 {
					break
				}
				crit[name] = "a discovered skill"
			}
			qs["skill"] = ChoiceQ("Which single skill is most relevant? Prefer none when unsure.", crit)
		}
		state := map[string]any{
			"prompt":           clip(prompt, 2000),
			"recent_tools":     recentTools,
			"transcript_bytes": transcriptBytes,
		}
		res, err := g.Client.Decide(ctx, state, qs)
		if err == nil {
			if raw, ok := res.Answers["model"]; ok {
				if ch, err := DecodeChoice(raw); err == nil && ch.Confidence >= g.minConf() && (ch.Choice == "fast" || ch.Choice == "strong") {
					out.Model = ch.Choice
				}
			}
			if raw, ok := res.Answers["stuck"]; ok {
				if n, err := DecodeNoul(raw); err == nil {
					out.Stuck = n.Noul >= g.riskAt()
				}
			}
			if raw, ok := res.Answers["weight"]; ok {
				if s, err := DecodeScore(raw); err == nil {
					out.Compact = s.Score >= 2.5
				}
			}
			if raw, ok := res.Answers["skill"]; ok {
				if ch, err := DecodeChoice(raw); err == nil && ch.Choice != "" && ch.Choice != "none" && ch.Confidence >= g.minConf() {
					out.Skills = []string{ch.Choice}
				}
			}
			out.Detail = "jev " + res.Model
			return out
		}
		out.Detail = "jev error, offline policy: " + err.Error()
		out.Source = "offline"
	}
	out.Model, out.Skills, out.Stuck, out.Compact = offlineTurn(prompt, skillNames, recentTools, transcriptBytes)
	if out.Detail == "" {
		out.Detail = "offline policy"
	}
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

// Risk is the destructive-action gate. Yolo does not bypass a block.
func (g Gates) Risk(ctx context.Context, tool, args string) (block bool, p float64, source string) {
	source = g.Mode()
	if source == "live" {
		res, err := g.Client.Decide(ctx, map[string]string{"tool": tool, "args": clip(args, 1500)}, map[string]Question{
			"risk": NoulQ("Is this tool call destructive or hard to undo?"),
		})
		if err == nil {
			if raw, ok := res.Answers["risk"]; ok {
				if n, err := DecodeNoul(raw); err == nil {
					return !g.AllowDestructive && n.Noul >= g.riskAt(), n.Noul, "live"
				}
			}
		}
		source = "offline"
	}
	p = offlineRisk(tool, args)
	return !g.AllowDestructive && p >= g.riskAt(), p, source
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

// SubagentKind picks explore, plan, or general.
func (g Gates) SubagentKind(ctx context.Context, prompt, requested string) string {
	req := strings.ToLower(strings.TrimSpace(requested))
	if req == "explore" || req == "plan" || req == "general" {
		if g.Mode() != "live" {
			return req
		}
	}
	if g.Mode() == "live" {
		res, err := g.Client.Decide(ctx, map[string]string{"prompt": clip(prompt, 1500), "requested": req}, map[string]Question{
			"kind": ChoiceQ("What kind of subagent should run?", map[string]string{
				"explore": "read-only search",
				"plan":    "write a plan, do not edit the repo",
				"general": "may edit and run commands",
			}),
		})
		if err == nil {
			if raw, ok := res.Answers["kind"]; ok {
				if ch, err := DecodeChoice(raw); err == nil && ch.Confidence >= g.minConf() {
					return ch.Choice
				}
			}
		}
	}
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

// KeepSnippet drops weak search hits. Offline keeps them.
func (g Gates) KeepSnippet(ctx context.Context, query, snippet string) bool {
	if g.Mode() != "live" || strings.TrimSpace(snippet) == "" {
		return true
	}
	res, err := g.Client.Decide(ctx, map[string]string{"query": clip(query, 400), "snippet": clip(snippet, 800)}, map[string]Question{
		"keep": NoulQ("Does this snippet help answer the query?"),
	})
	if err != nil {
		return true
	}
	raw, ok := res.Answers["keep"]
	if !ok {
		return true
	}
	n, err := DecodeNoul(raw)
	if err != nil {
		return true
	}
	return n.Noul >= g.minConf()
}

// PlanReady is reported. It does not auto-approve.
func (g Gates) PlanReady(ctx context.Context, plan, request string) (bool, float64) {
	if g.Mode() == "live" {
		res, err := g.Client.Decide(ctx, map[string]string{"plan": clip(plan, 2000), "request": clip(request, 800)}, map[string]Question{
			"ready": NoulQ("Is this plan specific enough to implement?"),
		})
		if err == nil {
			if raw, ok := res.Answers["ready"]; ok {
				if n, err := DecodeNoul(raw); err == nil {
					return n.Noul >= g.minConf(), n.Noul
				}
			}
		}
	}
	ready := len(strings.TrimSpace(plan)) > 80 && strings.Contains(strings.ToLower(plan), "step")
	p := 0.3
	if ready {
		p = 0.8
	}
	return ready, p
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
