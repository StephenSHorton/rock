package jev

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

const (
	// DefaultClipBytes is a Rock clip size. Stay well under Jev's
	// published ~32k state budget. Not a Jev API number.
	DefaultClipBytes = 1500
	MaxClipBytes     = 8000
	MaxScanPaths     = 8
	DefaultKeepAt    = 0.55
)

// FileClip is bytes Rock read. Jev has no filesystem verb.
type FileClip struct {
	Path string `json:"path"`
	Text string `json:"text"`
}

// FilterStats is measured, not estimated. Zero values mean we did not filter.
type FilterStats struct {
	ClipsBefore int `json:"clips_before"`
	ClipsAfter  int `json:"clips_after"`
	BytesBefore int `json:"bytes_before"`
	BytesAfter  int `json:"bytes_after"`
}

func (s FilterStats) Text() string {
	return fmt.Sprintf("clips %d→%d bytes %d→%d", s.ClipsBefore, s.ClipsAfter, s.BytesBefore, s.BytesAfter)
}

func (s FilterStats) Zero() bool {
	return s.ClipsBefore == 0 && s.ClipsAfter == 0 && s.BytesBefore == 0 && s.BytesAfter == 0
}

func Clip(s string, n int) string {
	if n <= 0 {
		n = DefaultClipBytes
	}
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func ClipSize(n int) int {
	if n <= 0 {
		return DefaultClipBytes
	}
	if n > MaxClipBytes {
		return MaxClipBytes
	}
	return n
}

// ClassifyQueries is the failed-shell set: type, likely area, retry.
func ClassifyQueries() []Query {
	return []Query{
		{
			Name:     "kind",
			Question: "What kind of failure is this log?",
			Mode:     ModeChoice,
			Options: map[string]string{
				"compile":    "compiler or type error",
				"test":       "failing test or assertion",
				"lint":       "linter or formatter",
				"timeout":    "deadline or hang",
				"permission": "access, sandbox, or auth",
				"runtime":    "crash or uncaught exception",
				"other":      "none of the above",
			},
		},
		{
			Name:     "area",
			Question: "Where should someone look first?",
			Mode:     ModeChoice,
			Options: map[string]string{
				"code":    "application code",
				"config":  "config or flags",
				"deps":    "dependencies or modules",
				"env":     "environment or machine",
				"test":    "the test itself",
				"unknown": "not enough signal",
			},
		},
		{
			Name:     "retry",
			Question: "Would retrying the same command likely succeed without a code change?",
			Mode:     ModeBoolean,
		},
	}
}

func KeepQuery(name, question string) Query {
	if name == "" {
		name = "keep"
	}
	if question == "" {
		question = "Does this snippet help answer the query?"
	}
	return Query{Name: name, Question: question, Mode: ModeBoolean}
}

func KeepName(i int) string { return "keep_" + strconv.Itoa(i) }

func KeepValue(a Answer, min float64) (keep bool, ok bool) {
	if a.Value == nil {
		return false, false
	}
	if min <= 0 {
		min = DefaultKeepAt
	}
	switch v := a.Value.(type) {
	case float64:
		return v >= min, true
	case float32:
		return float64(v) >= min, true
	case int:
		return float64(v) >= min, true
	case string:
		s := strings.ToLower(strings.TrimSpace(v))
		if s == "keep" || s == "yes" || s == "true" {
			return true, true
		}
		if s == "drop" || s == "no" || s == "false" {
			return false, true
		}
	}
	return false, false
}

// CompactClassify is a one-line observation. Empty when Jev did not answer.
func CompactClassify(res Result) string {
	if res.Error != "" {
		return ""
	}
	kind, area, retry := "", "", ""
	if a, ok := res.Answers["kind"]; ok && a.Value != nil {
		kind = fmt.Sprint(a.Value)
	}
	if a, ok := res.Answers["area"]; ok && a.Value != nil {
		area = fmt.Sprint(a.Value)
	}
	if a, ok := res.Answers["retry"]; ok && a.Value != nil {
		retry = fmt.Sprint(a.Value)
	}
	if kind == "" && area == "" && retry == "" {
		return ""
	}
	return strings.TrimSpace("Jev classify: kind=" + kind + " area=" + area + " retry=" + retry)
}

// FilterSnippets keeps search hits that help the query. Live-only; a failed
// Ask keeps every snippet (no invented drop). One Ask, shared state.
func (g Gates) FilterSnippets(ctx context.Context, query string, snippets []string) []string {
	if len(snippets) == 0 || g.Mode() != "live" {
		return snippets
	}
	clipped := make([]string, len(snippets))
	for i, s := range snippets {
		clipped[i] = Clip(s, 800)
	}
	qs := make([]Query, len(clipped))
	for i := range clipped {
		qs[i] = KeepQuery(KeepName(i), "Does snippet "+strconv.Itoa(i)+" help answer the query?")
	}
	res := g.Ask(ctx, map[string]any{"query": Clip(query, 400), "snippets": clipped}, qs)
	if res.Error != "" {
		return snippets
	}
	var kept []string
	for i, sn := range snippets {
		keep, ok := KeepValue(res.Answers[KeepName(i)], g.minConf())
		if !ok || keep {
			kept = append(kept, sn)
		}
	}
	return kept
}

// KeepSnippet is the one-clip form. Offline and failed Ask keep the clip.
func (g Gates) KeepSnippet(ctx context.Context, query, snippet string) bool {
	if g.Mode() != "live" || strings.TrimSpace(snippet) == "" {
		return true
	}
	res := g.Ask(ctx, map[string]string{"query": Clip(query, 400), "snippet": Clip(snippet, 800)}, []Query{
		KeepQuery("keep", "Does this snippet help answer the query?"),
	})
	if res.Error != "" {
		return true
	}
	keep, ok := KeepValue(res.Answers["keep"], g.minConf())
	if !ok {
		return true
	}
	return keep
}
