// Package cli builds one process: config, Jev, tools, MCP, and the harness.
// The TUI, ACP server, and HTTP server are clients of that harness.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"charm.land/huh/v2"
	"charm.land/log/v2"

	"github.com/StephenSHorton/rock/internal/config"
	"github.com/StephenSHorton/rock/internal/jev"
	"github.com/StephenSHorton/rock/internal/mcp"
	"github.com/StephenSHorton/rock/internal/perms"
	"github.com/StephenSHorton/rock/internal/provider"
	"github.com/StephenSHorton/rock/internal/session"
	"github.com/StephenSHorton/rock/internal/skills"
	"github.com/StephenSHorton/rock/internal/suzuri"
	"github.com/StephenSHorton/rock/internal/tools"
	"github.com/StephenSHorton/rock/internal/version"
)

// App is the process-wide wiring. It does not own a session.
type App struct {
	CWD        string
	Loaded     config.Loaded
	Skills     []skills.Skill
	Gates      jev.Gates
	Provider   provider.Provider
	Extras     []tools.Extra
	Clients    []*mcp.Client
	MCPNotes   []string
	Log        *log.Logger
	Checkpoint bool
	mcpDone    bool
}

func Open(cwd string) (*App, error) {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return nil, err
	}
	loaded, err := config.Load(abs)
	if err != nil {
		return nil, err
	}
	sk, err := skills.Discover(abs)
	if err != nil {
		return nil, err
	}
	key, _ := config.JevKey()
	client := &jev.Client{
		APIKey:  key,
		BaseURL: loaded.File.Jev.BaseURL,
		Model:   loaded.File.Jev.Model,
	}
	gates := jev.Gates{
		Client:           client,
		MinConfidence:    loaded.File.Jev.MinConfidence,
		RiskBlock:        loaded.File.Jev.RiskBlock,
		AllowDestructive: loaded.File.Jev.AllowDestructive,
	}
	var model provider.Provider
	if k := config.APIKey(); k != "" {
		model = &provider.OpenAI{APIKey: k, BaseURL: loaded.File.BaseURL}
	} else {
		model = &provider.Script{}
	}
	lg := openLog()
	app := &App{
		CWD:        abs,
		Loaded:     loaded,
		Skills:     sk,
		Gates:      gates,
		Provider:   model,
		Log:        lg,
		Checkpoint: loaded.File.Git.Checkpoint,
	}
	lg.Info("open", "cwd", abs, "provider", model.Name(), "jev", gates.Mode(), "skills", len(sk))
	return app, nil
}

// ConnectMCP starts enabled stdio servers once. Later harnesses reuse the tools.
func (a *App) ConnectMCP(ctx context.Context) {
	if a.mcpDone {
		return
	}
	a.mcpDone = true
	a.attachMCP(ctx)
	a.Log.Info("mcp", "tools", len(a.Extras), "notes", len(a.MCPNotes))
}

func (a *App) Close() {
	for _, c := range a.Clients {
		_ = c.Close()
	}
}

func (a *App) Policy() perms.Policy {
	p := a.Loaded.Policy
	if a.Loaded.File.Git.ReviewOnly {
		p.ReviewOnly = true
	}
	return p
}

func (a *App) ApplyFlags(mode string, yolo bool) {
	if yolo {
		mode = string(perms.ModeYolo)
	}
	switch perms.Mode(mode) {
	case perms.ModePlan, perms.ModeYolo, perms.ModeDefault:
		a.Loaded.Policy.Mode = perms.Mode(mode)
		a.Loaded.File.Mode = mode
	}
}

// NewHarness builds a harness for cwd. MCP tools are copied onto its tool set.
func (a *App) NewHarness(cwd string) (*tools.Set, error) {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return nil, err
	}
	set := tools.New(tools.Env{Root: abs})
	for _, e := range a.Extras {
		set.Add(e)
	}
	return set, nil
}

func (a *App) FastModel() string {
	if a.Loaded.File.FastModel != "" {
		return a.Loaded.File.FastModel
	}
	return a.Loaded.File.Model
}

func (a *App) StrongModel() string {
	if a.Loaded.File.StrongModel != "" {
		return a.Loaded.File.StrongModel
	}
	return a.Loaded.File.Model
}

func (a *App) OpenSession(id string, create bool, title string) (*session.Session, error) {
	if id != "" && !create {
		return session.Load(a.CWD, id)
	}
	if id != "" {
		if s, err := session.Load(a.CWD, id); err == nil {
			return s, nil
		}
		return session.Create(a.CWD, id, title)
	}
	return session.Create(a.CWD, "", title)
}

func (a *App) Inspect(w io.Writer) {
	a.ConnectMCP(context.Background())
	fmt.Fprintf(w, "rock %s\n", version.Version)
	fmt.Fprintf(w, "cwd: %s\n", a.CWD)
	fmt.Fprintf(w, "config: %s\n", a.Loaded.Path)
	fmt.Fprintf(w, "mode: %s\n", a.Policy().Mode)
	fmt.Fprintf(w, "review_only: %v\n", a.Policy().ReviewOnly)
	fmt.Fprintf(w, "trusted: %v\n", a.Loaded.Trusted)
	if len(a.Loaded.Ignored) > 0 {
		fmt.Fprintf(w, "ignored project allow rules (folder is not trusted): %s\n", strings.Join(a.Loaded.Ignored, ", "))
	}
	fmt.Fprintf(w, "provider: %s\n", a.Provider.Name())
	if a.Provider.Name() == "offline" {
		fmt.Fprintf(w, "model key: unset (ROCK_API_KEY or OPENAI_API_KEY). Replies come from the offline provider.\n")
	} else {
		fmt.Fprintf(w, "base_url: %s\n", a.Loaded.File.BaseURL)
		fmt.Fprintf(w, "fast model: %s\n", a.FastModel())
		fmt.Fprintf(w, "strong model: %s\n", a.StrongModel())
	}
	fmt.Fprintf(w, "jev: %s\n", a.Gates.Mode())
	if a.Gates.Mode() == "offline" {
		fmt.Fprintf(w, "jev key: unset (JEV_API_KEY or TYPESAFE_API_KEY). Gates run a local policy. That policy is not Jev.\n")
	} else {
		_, src := config.JevKey()
		fmt.Fprintf(w, "jev key: %s\n", src)
	}
	if n := a.Loaded.File.Jev.NudgeInterval(); n < 0 {
		fmt.Fprintf(w, "jev.nudge: off\n")
	} else {
		fmt.Fprintf(w, "jev.nudge: every %d\n", n)
	}
	fmt.Fprintf(w, "git.checkpoint: %v\n", a.Checkpoint)
	fmt.Fprintf(w, "allow: %s\n", strings.Join(a.Policy().Allow, ", "))
	fmt.Fprintf(w, "ask: %s\n", strings.Join(a.Policy().Ask, ", "))
	fmt.Fprintf(w, "deny: %s\n", strings.Join(a.Policy().Deny, ", "))
	if len(a.Skills) == 0 {
		fmt.Fprintf(w, "skills: none\n")
	} else {
		fmt.Fprintf(w, "skills:\n")
		for _, s := range a.Skills {
			fmt.Fprintf(w, "  %s  %s\n", s.Name, s.Path)
		}
	}
	if len(a.MCPNotes) == 0 && len(a.Extras) == 0 {
		fmt.Fprintf(w, "mcp: none configured\n")
	} else {
		fmt.Fprintf(w, "mcp:\n")
		for _, n := range a.MCPNotes {
			fmt.Fprintf(w, "  %s\n", n)
		}
	}
	fmt.Fprintf(w, "sessions: %s\n", session.Home())
	fmt.Fprintf(w, "Permissions are not a sandbox. Plan mode blocks every shell command because redirections are not inspected.\n")
	fmt.Fprintf(w, "Suzuri: rock fork prints OSC 7880. The host allowlist must include the rock binary.\n")
}

func (a *App) PrintSessions(w io.Writer) error {
	list, err := session.List(a.CWD)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Fprintf(w, "no sessions in %s\n", a.CWD)
		return nil
	}
	for _, m := range list {
		fmt.Fprintf(w, "%s  %s  %s\n", m.ID, m.UpdatedAt.Format(time.RFC3339), m.Title)
	}
	return nil
}

func (a *App) PrintPermissions(w io.Writer) {
	p := a.Policy()
	fmt.Fprintf(w, "mode %s\n", p.Mode)
	for _, r := range p.Allow {
		fmt.Fprintf(w, "allow %s\n", r)
	}
	for _, r := range p.Ask {
		fmt.Fprintf(w, "ask   %s\n", r)
	}
	for _, r := range p.Deny {
		fmt.Fprintf(w, "deny  %s\n", r)
	}
	fmt.Fprintf(w, "Permissions are not a sandbox. Plan mode blocks every shell command because redirections are not inspected.\n")
}

func ForkSequence(newSession bool, id, prompt, title, cwd string) (string, error) {
	bin, err := os.Executable()
	if err != nil {
		return "", err
	}
	bin, err = filepath.Abs(bin)
	if err != nil {
		return "", err
	}
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}
	if id == "" {
		id = session.NewID()
	}
	return suzuri.Sequence(suzuri.Fork{
		NewSession: newSession,
		SessionID:  id,
		Bin:        bin,
		Prompt:     prompt,
		Title:      title,
		CWD:        abs,
	}), nil
}

// Headless runs one turn. format is "text" or "streaming-json".
// Ask is denied when yolo is off. A terminal can pass an ask function.
func (a *App) Headless(ctx context.Context, sess *session.Session, prompt, format string, stdout, stderr io.Writer, ask func(tool, detail string) perms.Decision) error {
	a.ConnectMCP(ctx)
	set, err := a.NewHarness(sess.Meta.CWD)
	if err != nil {
		return err
	}
	set.Env.PlanPath = sess.PlanPath()
	h := a.harness(set, func(ctx context.Context, tool, detail string) perms.Decision {
		if a.Policy().Mode == perms.ModeYolo {
			return perms.Allow
		}
		if ask == nil {
			return perms.Deny
		}
		return ask(tool, detail)
	})
	return h.Run(ctx, sess, prompt, func(ev harnessEvent) {
		if format == "streaming-json" {
			raw, _ := json.Marshal(ev)
			fmt.Fprintf(stdout, "%s\n", raw)
			return
		}
		switch ev.Kind {
		case "assistant":
			fmt.Fprintln(stdout, ev.Text)
		case "tool_call":
			fmt.Fprintf(stderr, "tool %s %s\n", ev.Name, ev.Text)
		case "tool_result":
			fmt.Fprintf(stderr, "result %s\n", oneLine(ev.Text))
		case "permission":
			fmt.Fprintf(stderr, "permission %s %s\n", ev.Name, ev.Text)
		case "jev":
			fmt.Fprintf(stderr, "jev %s %s\n", ev.Name, ev.Text)
		case "status":
			fmt.Fprintf(stderr, "%s\n", ev.Text)
		case "done":
			fmt.Fprintf(stderr, "done %s\n", ev.Text)
		}
	})
}

func (a *App) Setup(stdin io.Reader, stdout io.Writer) error {
	file := a.Loaded.File
	fast := a.FastModel()
	strong := a.StrongModel()
	base := file.BaseURL
	mode := string(a.Policy().Mode)
	checkpoint := file.Git.Checkpoint
	suzuriMCP := false
	if spec, ok := file.MCP["suzuri"]; ok && spec.Enabled {
		suzuriMCP = true
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Fast model").Value(&fast).Placeholder("gpt-4o-mini"),
			huh.NewInput().Title("Strong model").Value(&strong).Placeholder("gpt-4o"),
			huh.NewInput().Title("OpenAI-compatible base URL").Value(&base),
			huh.NewSelect[string]().Title("Mode").Options(
				huh.NewOption("default", "default"),
				huh.NewOption("plan", "plan"),
				huh.NewOption("yolo", "yolo"),
			).Value(&mode),
			huh.NewConfirm().Title("Git checkpoint after a mutating turn?").Value(&checkpoint),
			huh.NewConfirm().Title("Enable the suzuri mcp server? The Suzuri window has to be running.").Value(&suzuriMCP),
		),
	)
	if err := form.Run(); err != nil {
		return err
	}
	file.FastModel = strings.TrimSpace(fast)
	file.StrongModel = strings.TrimSpace(strong)
	file.Model = file.FastModel
	file.BaseURL = strings.TrimSpace(base)
	file.Mode = mode
	file.Git.Checkpoint = checkpoint
	if file.MCP == nil {
		file.MCP = map[string]config.MCPServer{}
	}
	if suzuriMCP {
		file.MCP["suzuri"] = config.MCPServer{Command: "suzuri", Args: []string{"mcp"}, Enabled: true}
	} else {
		delete(file.MCP, "suzuri")
	}
	path := config.ConfigPath()
	if err := config.Write(path, file); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "wrote %s\nAPI keys stay in the environment (ROCK_API_KEY, JEV_API_KEY, TYPESAFE_API_KEY).\n", path)
	return nil
}

func (a *App) attachMCP(ctx context.Context) {
	for name, spec := range a.Loaded.File.MCP {
		if spec.Command == "" || !spec.Enabled {
			a.MCPNotes = append(a.MCPNotes, name+": disabled")
			continue
		}
		if _, err := exec.LookPath(spec.Command); err != nil && !filepath.IsAbs(spec.Command) {
			a.MCPNotes = append(a.MCPNotes, fmt.Sprintf("%s: %s not on PATH", name, spec.Command))
			continue
		}
		client, err := mcp.Start(ctx, mcp.Server{Name: name, Command: spec.Command, Args: spec.Args})
		if err != nil {
			a.MCPNotes = append(a.MCPNotes, fmt.Sprintf("%s: %s", name, err.Error()))
			continue
		}
		listed, err := client.Tools(ctx, name)
		if err != nil {
			_ = client.Close()
			a.MCPNotes = append(a.MCPNotes, fmt.Sprintf("%s: tools/list: %s", name, err.Error()))
			continue
		}
		a.Clients = append(a.Clients, client)
		for _, t := range listed {
			toolName := mcpName(name, t.Name)
			schema := t.InputSchema
			callName := t.Name
			c := client
			a.Extras = append(a.Extras, tools.Extra{
				Name:        toolName,
				Description: t.Description + " (mcp " + name + ")",
				Schema:      schema,
				Call: func(ctx context.Context, args string) (string, error) {
					var raw map[string]any
					if strings.TrimSpace(args) != "" {
						if err := json.Unmarshal([]byte(args), &raw); err != nil {
							return "", err
						}
					}
					return c.Call(ctx, callName, raw)
				},
			})
		}
		a.MCPNotes = append(a.MCPNotes, fmt.Sprintf("%s: %d tools from %s", name, len(listed), spec.Command))
	}
}

func mcpName(server, tool string) string {
	return "mcp_" + sanitize(server) + "_" + sanitize(tool)
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "tool"
	}
	return out
}

func openLog() *log.Logger {
	path := filepath.Join(session.Home(), "rock.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return log.New(io.Discard)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return log.New(io.Discard)
	}
	return log.New(f)
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 180 {
		return s[:180] + "…"
	}
	return s
}
