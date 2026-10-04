// Package perms is the allow / ask / deny gate. Plan mode blocks mutation
// even when yolo would otherwise allow it. Review-only denies mutation.
// Shell is blocked entirely in plan mode: we do not pretend to understand
// redirections.
package perms

import (
	"path/filepath"
	"strings"
)

type Mode string

const (
	ModeDefault Mode = "default"
	ModePlan    Mode = "plan"
	ModeYolo    Mode = "yolo"
)

type Decision string

const (
	Allow Decision = "allow"
	Ask   Decision = "ask"
	Deny  Decision = "deny"
)

type Policy struct {
	Mode       Mode
	ReviewOnly bool
	Allow      []string
	Ask        []string
	Deny       []string
}

func (p Policy) Decide(tool, detail string) (Decision, string) {
	if p.ReviewOnly && Mutates(tool) && tool != "update_plan" {
		return Deny, "review-only denies edits and shell"
	}
	if p.Mode == ModePlan && planBlocks(tool, detail) {
		return Deny, "plan mode blocks this until you leave the plan"
	}
	if matchAny(p.Deny, tool, detail) {
		return Deny, "matched a deny rule"
	}
	if tool == "ask_jev" {
		return Allow, "ask_jev is read-only"
	}
	if matchAny(p.Allow, tool, detail) {
		return Allow, "matched an allow rule"
	}
	if p.Mode == ModeYolo {
		return Allow, "yolo"
	}
	if matchAny(p.Ask, tool, detail) {
		return Ask, "matched an ask rule"
	}
	switch tool {
	case "read_file", "grep", "glob", "web_fetch", "ask_jev", "update_plan":
		return Allow, "read-only default"
	default:
		return Ask, "mutating tools ask by default"
	}
}

func Mutates(tool string) bool {
	switch tool {
	case "edit_file", "write_file", "shell", "spawn_subagent":
		return true
	default:
		return strings.HasPrefix(tool, "mcp_")
	}
}

func planBlocks(tool, detail string) bool {
	switch tool {
	case "update_plan", "read_file", "grep", "glob", "web_fetch", "ask_jev":
		return false
	case "edit_file", "write_file":
		return !strings.HasSuffix(filepath.Clean(detail), "plan.md")
	case "shell":
		return true
	case "spawn_subagent":
		kind := strings.ToLower(detail)
		return kind == "general" || kind == ""
	default:
		return Mutates(tool)
	}
}

func matchAny(rules []string, tool, detail string) bool {
	for _, rule := range rules {
		if match(rule, tool, detail) {
			return true
		}
	}
	return false
}

func match(rule, tool, detail string) bool {
	rule = strings.TrimSpace(rule)
	if rule == "" || rule == "*" {
		return rule == "*"
	}
	name, arg, ok := strings.Cut(rule, ":")
	if !ok {
		return rule == tool
	}
	if name != tool && name != "*" {
		return false
	}
	if arg == "*" {
		return true
	}
	if tool == "shell" {
		return strings.HasPrefix(strings.TrimSpace(detail), strings.TrimSuffix(arg, "*")) && (strings.HasSuffix(arg, "*") || strings.TrimSpace(detail) == arg)
	}
	okm, err := filepath.Match(arg, detail)
	if err != nil {
		return false
	}
	return okm
}
