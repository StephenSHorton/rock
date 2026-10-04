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

	"github.com/StephenSHorton/rock/internal/jev"
	"github.com/StephenSHorton/rock/internal/provider"
)

type SubagentFunc func(ctx context.Context, prompt, kind string, worktree bool) (string, error)
type AskFunc func(ctx context.Context, state any, questions []jev.Query) (jev.Result, error)

type Env struct {
	Root     string
	PlanPath string
	Subagent SubagentFunc
	Ask      AskFunc
	KeepGrep func(query, snippet string) bool
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
		{Name: "ask_jev", Description: "Ask Jev to classify, verify a fix, filter or triage, or check risk. Batch questions. Do not draft code. A failed call returns error and no invented answer.", Parameters: askJevSpec(obj, str)},
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
		out, err := s.grep(str("pattern"), str("path"))
		return Outcome{Output: out, Detail: str("pattern")}, err
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
		if s.Env.Ask == nil {
			return Outcome{}, fmt.Errorf("jev is not wired")
		}
		state, qs, err := parseAsk(raw)
		if err != nil {
			return Outcome{}, err
		}
		res, err := s.Env.Ask(ctx, state, qs)
		if err != nil {
			return Outcome{}, err
		}
		body, err := json.Marshal(res)
		if err != nil {
			return Outcome{}, err
		}
		detail := "ask"
		if len(qs) > 0 {
			detail = strings.TrimSpace(queryMode(qs[0]) + "  " + qs[0].Question)
		}
		return Outcome{Output: string(body), Detail: detail}, nil
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

func (s *Set) grep(pattern, rel string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}
	root := s.Env.Root
	if rel != "" {
		root, err = s.within(rel)
		if err != nil {
			return "", err
		}
	}
	var b strings.Builder
	n := 0
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
				snip := fmt.Sprintf("%s:%d:%s", relPath, i+1, strings.TrimSpace(line))
				if s.Env.KeepGrep != nil && !s.Env.KeepGrep(pattern, snip) {
					continue
				}
				b.WriteString(snip)
				b.WriteByte('\n')
				n++
				if n >= 40 {
					return fmt.Errorf("stop")
				}
			}
		}
		return nil
	})
	if b.Len() == 0 {
		return "no matches", nil
	}
	return b.String(), nil
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
