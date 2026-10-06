// Package tools is the workspace tool set. Paths are confined to Root.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/StephenSHorton/rock/internal/childproc"
	"github.com/StephenSHorton/rock/internal/config"
	"github.com/StephenSHorton/rock/internal/jev"
	"github.com/StephenSHorton/rock/internal/provider"
)

type SubagentFunc func(ctx context.Context, prompt, kind string, worktree bool) (string, error)
type AskFunc func(ctx context.Context, state any, questions []jev.Query) (jev.Result, error)

type Env struct {
	Root           string
	PlanPath       string
	Subagent       SubagentFunc
	Ask            AskFunc
	KeepGrep       func(query, snippet string) bool
	FilterSnippets func(query string, snippets []string) []string
	ClipBytes      int
	MinConfidence  float64
}

// Extra is a tool supplied by an MCP server or another client. The name is
// what the model sees. Call receives the raw JSON arguments.
type Extra struct {
	Name        string
	Description string
	Schema      map[string]any
	Call        func(ctx context.Context, args string) (string, error)
}

type Set struct {
	Env   Env
	names []string
	extra []Extra
}

// Add registers a namespaced tool. Duplicate names replace the previous one.
func (s *Set) Add(e Extra) {
	if e.Name == "" || e.Call == nil {
		return
	}
	for i := range s.extra {
		if s.extra[i].Name == e.Name {
			s.extra[i] = e
			return
		}
	}
	s.extra = append(s.extra, e)
}

func New(env Env) *Set {
	return &Set{
		Env: env,
		names: []string{
			"read_file", "edit_file", "write_file", "grep", "glob",
			"shell", "web_fetch", "update_plan", "ask_jev", "spawn_subagent",
		},
	}
}

func (s *Set) Specs() []provider.ToolSpec {
	obj := func(props map[string]any, req []string) map[string]any {
		return map[string]any{"type": "object", "properties": props, "required": req}
	}
	str := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	out := []provider.ToolSpec{
		{Name: "read_file", Description: "Read a UTF-8 file in the workspace.", Parameters: obj(map[string]any{"path": str("relative path"), "limit": map[string]any{"type": "integer"}}, []string{"path"})},
		{Name: "edit_file", Description: "Replace one exact snippet in a file.", Parameters: obj(map[string]any{"path": str("relative path"), "old": str("exact text"), "new": str("replacement")}, []string{"path", "old", "new"})},
		{Name: "write_file", Description: "Create or overwrite a file.", Parameters: obj(map[string]any{"path": str("relative path"), "content": str("full contents")}, []string{"path", "content"})},
		{Name: "grep", Description: "Search file contents with a regular expression.", Parameters: obj(map[string]any{"pattern": str("Go regexp"), "path": str("directory, default workspace")}, []string{"pattern"})},
		{Name: "glob", Description: "List files whose relative path matches a glob.", Parameters: obj(map[string]any{"pattern": str("glob, * within a segment")}, []string{"pattern"})},
		{Name: "shell", Description: "Run a shell command in the workspace. Not a sandbox.", Parameters: obj(map[string]any{"command": str("command string")}, []string{"command"})},
		{Name: "web_fetch", Description: "HTTP GET a URL and return text.", Parameters: obj(map[string]any{"url": str("http(s) URL")}, []string{"url"})},
		{Name: "update_plan", Description: "Write the session plan. Allowed in plan mode.", Parameters: obj(map[string]any{"content": str("markdown plan")}, []string{"content"})},
		{Name: "ask_jev", Description: "Ask Jev to classify, verify a fix, filter or triage, or check risk. Batch questions. Do not draft code. Optional paths: Rock reads workspace clips into state — Jev has no filesystem. A failed call returns error and no invented answer.", Parameters: askJevSpec(obj, str), ReadOnly: true, Title: "Ask Jev"},
		{Name: "spawn_subagent", Description: "Run a depth-1 subagent: explore, plan, or general. Optional git worktree.", Parameters: obj(map[string]any{"prompt": str("task"), "kind": str("explore, plan, or general"), "worktree": map[string]any{"type": "boolean"}}, []string{"prompt"})},
	}
	for _, e := range s.extra {
		schema := e.Schema
		if schema == nil {
			schema = obj(map[string]any{}, nil)
		}
		out = append(out, provider.ToolSpec{Name: e.Name, Description: e.Description, Parameters: schema})
	}
	return out
}

type Outcome struct {
	Output  string
	Mutates bool
	Path    string
	Detail  string
	Filter  *jev.FilterStats
}

func (s *Set) Run(ctx context.Context, name, args string) (Outcome, error) {
	var raw map[string]any
	if strings.TrimSpace(args) != "" {
		if err := json.Unmarshal([]byte(args), &raw); err != nil {
			return Outcome{}, fmt.Errorf("arguments: %w", err)
		}
	}
	str := func(k string) string {
		v, _ := raw[k].(string)
		return v
	}
	switch name {
	case "read_file":
		p, err := s.within(str("path"))
		if err != nil {
			return Outcome{}, err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return Outcome{}, err
		}
		text := string(b)
		if len(text) > 32_000 {
			text = text[:32_000] + "\n…truncated"
		}
		return Outcome{Output: text, Path: str("path"), Detail: str("path")}, nil
	case "edit_file":
		p, err := s.within(str("path"))
		if err != nil {
			return Outcome{}, err
		}
		old, neu := str("old"), str("new")
		b, err := os.ReadFile(p)
		if err != nil {
			return Outcome{}, err
		}
		cur := string(b)
		if !strings.Contains(cur, old) {
			return Outcome{}, fmt.Errorf("old text not found in %s", str("path"))
		}
		if strings.Count(cur, old) != 1 {
			return Outcome{}, fmt.Errorf("old text is not unique in %s", str("path"))
		}
		if err := os.WriteFile(p, []byte(strings.Replace(cur, old, neu, 1)), 0o644); err != nil {
			return Outcome{}, err
		}
		return Outcome{Output: "edited " + str("path"), Mutates: true, Path: str("path"), Detail: str("path")}, nil
	case "write_file":
		p, err := s.within(str("path"))
		if err != nil {
			return Outcome{}, err
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return Outcome{}, err
		}
		if err := os.WriteFile(p, []byte(str("content")), 0o644); err != nil {
			return Outcome{}, err
		}
		return Outcome{Output: "wrote " + str("path"), Mutates: true, Path: str("path"), Detail: str("path")}, nil
	case "grep":
		out, stats, err := s.grep(str("pattern"), str("path"))
		return Outcome{Output: out, Detail: str("pattern"), Filter: stats}, err
	case "glob":
		out, err := s.glob(str("pattern"))
		return Outcome{Output: out, Detail: str("pattern")}, err
	case "shell":
		cmd := str("command")
		out, err := s.shell(ctx, cmd)
		return Outcome{Output: out, Mutates: true, Detail: cmd}, err
	case "web_fetch":
		out, err := fetch(ctx, str("url"))
		return Outcome{Output: out, Detail: str("url")}, err
	case "update_plan":
		if s.Env.PlanPath == "" {
			return Outcome{}, fmt.Errorf("no plan path")
		}
		if err := os.MkdirAll(filepath.Dir(s.Env.PlanPath), 0o755); err != nil {
			return Outcome{}, err
		}
		if err := os.WriteFile(s.Env.PlanPath, []byte(str("content")), 0o644); err != nil {
			return Outcome{}, err
		}
		return Outcome{Output: "updated plan", Mutates: true, Path: "plan.md", Detail: "plan.md"}, nil
	case "ask_jev":
		state, qs, err := parseAsk(raw)
		if err != nil {
			return Outcome{}, err
		}
		clips, stats, err := s.readClips(raw)
		if err != nil {
			return Outcome{}, err
		}
		if len(clips) > 0 {
			state = attachClips(state, clips)
			if len(qs) == 0 {
				qs = keepQueriesForClips(clips, strings.TrimSpace(asString(raw["question"])))
			}
		}
		if err := jev.ValidateQueries(qs); err != nil {
			return Outcome{}, err
		}
		var res jev.Result
		if s.Env.Ask == nil {
			res = jev.Failed(qs, "jev client is not wired")
		} else {
			res, err = s.Env.Ask(ctx, state, qs)
			if err != nil {
				res = jev.Failed(qs, err.Error())
			}
		}
		if len(clips) > 0 {
			res = applyClipFilter(res, clips, stats, s.keepAt(), filterFlag(raw))
		}
		body, err := json.Marshal(res)
		if err != nil {
			return Outcome{}, err
		}
		return Outcome{Output: string(body), Detail: AskDetail(raw), Filter: res.Filter}, nil
	case "spawn_subagent":
		if s.Env.Subagent == nil {
			return Outcome{}, fmt.Errorf("subagents are not wired")
		}
		wt, _ := raw["worktree"].(bool)
		kind := str("kind")
		out, err := s.Env.Subagent(ctx, str("prompt"), kind, wt)
		return Outcome{Output: out, Mutates: true, Detail: kind}, err
	default:
		for _, e := range s.extra {
			if e.Name == name {
				out, err := e.Call(ctx, args)
				return Outcome{Output: out, Detail: name}, err
			}
		}
		return Outcome{}, fmt.Errorf("unknown tool %s", name)
	}
}

func (s *Set) keepAt() float64 {
	if s.Env.MinConfidence > 0 {
		return s.Env.MinConfidence
	}
	return jev.DefaultKeepAt
}

func (s *Set) readClips(raw map[string]any) ([]jev.FileClip, jev.FilterStats, error) {
	paths, err := parsePathList(raw["paths"])
	if err != nil {
		return nil, jev.FilterStats{}, err
	}
	if len(paths) == 0 {
		return nil, jev.FilterStats{}, nil
	}
	if len(paths) > jev.MaxScanPaths {
		paths = paths[:jev.MaxScanPaths]
	}
	limit := jev.ClipSize(s.Env.ClipBytes)
	out := make([]jev.FileClip, 0, len(paths))
	for _, rel := range paths {
		p, err := s.within(rel)
		if err != nil {
			return nil, jev.FilterStats{}, err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, jev.FilterStats{}, err
		}
		text := string(b)
		if len(text) > limit {
			text = jev.Clip(text, limit)
		}
		out = append(out, jev.FileClip{Path: rel, Text: text})
	}
	return out, jev.FilterStats{}, nil
}

func (s *Set) within(p string) (string, error) {
	if strings.TrimSpace(p) == "" {
		return "", fmt.Errorf("missing path")
	}
	root := s.Env.Root
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	p = filepath.Clean(p)
	root = filepath.Clean(root)
	rel, err := filepath.Rel(root, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes workspace")
	}
	return p, nil
}

func (s *Set) grep(pattern, rel string) (string, *jev.FilterStats, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", nil, err
	}
	root := s.Env.Root
	if rel != "" {
		root, err = s.within(rel)
		if err != nil {
			return "", nil, err
		}
	}
	var found []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil || len(raw) > 500_000 {
			return nil
		}
		relPath, _ := filepath.Rel(s.Env.Root, path)
		for i, line := range strings.Split(string(raw), "\n") {
			if re.MatchString(line) {
				found = append(found, fmt.Sprintf("%s:%d:%s", relPath, i+1, strings.TrimSpace(line)))
				if len(found) >= 40 {
					return fmt.Errorf("stop")
				}
			}
		}
		return nil
	})
	if len(found) == 0 {
		return "no matches", nil, nil
	}
	kept := found
	var stats *jev.FilterStats
	if s.Env.FilterSnippets != nil || s.Env.KeepGrep != nil {
		before := snippetStats(found)
		if s.Env.FilterSnippets != nil {
			kept = s.Env.FilterSnippets(pattern, found)
		} else {
			kept = kept[:0]
			for _, sn := range found {
				if s.Env.KeepGrep(pattern, sn) {
					kept = append(kept, sn)
				}
			}
		}
		after := snippetStats(kept)
		stats = &jev.FilterStats{
			ClipsBefore: before.clips,
			ClipsAfter:  after.clips,
			BytesBefore: before.bytes,
			BytesAfter:  after.bytes,
		}
	}
	if len(kept) == 0 {
		return "no matches", stats, nil
	}
	var b strings.Builder
	for _, sn := range kept {
		b.WriteString(sn)
		b.WriteByte('\n')
	}
	return b.String(), stats, nil
}

func snippetStats(snips []string) struct{ clips, bytes int } {
	n := 0
	for _, s := range snips {
		n += len(s)
	}
	return struct{ clips, bytes int }{len(snips), n}
}

func (s *Set) glob(pattern string) (string, error) {
	var b strings.Builder
	n := 0
	err := filepath.WalkDir(s.Env.Root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(s.Env.Root, path)
		ok, err := filepath.Match(pattern, rel)
		if err != nil {
			return err
		}
		baseOK, _ := filepath.Match(pattern, filepath.Base(rel))
		if ok || baseOK {
			b.WriteString(rel)
			b.WriteByte('\n')
			n++
			if n >= 80 {
				return fmt.Errorf("stop")
			}
		}
		return nil
	})
	if err != nil && err.Error() != "stop" {
		return "", err
	}
	if b.Len() == 0 {
		return "no files", nil
	}
	return b.String(), nil
}

func (s *Set) shell(ctx context.Context, command string) (string, error) {
	if strings.TrimSpace(command) == "" {
		return "", fmt.Errorf("empty command")
	}
	cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, "bash", "-lc", command)
	cmd.Dir = s.Env.Root
	config.ScrubCmdEnv(cmd)
	childproc.Isolate(cmd)
	out, err := cmd.CombinedOutput()
	text := string(out)
	if len(text) > 32_000 {
		text = text[:32_000] + "\n…truncated"
	}
	if err != nil {
		if text == "" {
			text = err.Error()
		}
		return text, fmt.Errorf("shell: %w", err)
	}
	if strings.TrimSpace(text) == "" {
		text = "(no output)"
	}
	return text, nil
}

func fetch(ctx context.Context, rawURL string) (string, error) {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return "", fmt.Errorf("url must be http or https")
	}
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, 64_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("HTTP %d\n%s", res.StatusCode, string(b)), nil
}
