package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"

	"github.com/StephenSHorton/rock/internal/acp"
	"github.com/StephenSHorton/rock/internal/cli"
	"github.com/StephenSHorton/rock/internal/config"
	"github.com/StephenSHorton/rock/internal/grokcli"
	"github.com/StephenSHorton/rock/internal/perms"
	"github.com/StephenSHorton/rock/internal/serve"
	"github.com/StephenSHorton/rock/internal/session"
	"github.com/StephenSHorton/rock/internal/siwc"
	"github.com/StephenSHorton/rock/internal/tui"
	"github.com/StephenSHorton/rock/internal/update"
	"github.com/StephenSHorton/rock/internal/version"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "help":
			fmt.Fprint(os.Stdout, usage)
			return nil
		case "version":
			fmt.Println(version.Version)
			return nil
		case "inspect":
			app, err := open(args[1:])
			if err != nil {
				return err
			}
			defer app.Close()
			app.Inspect(os.Stdout)
			return nil
		case "sessions":
			app, err := open(args[1:])
			if err != nil {
				return err
			}
			defer app.Close()
			return app.PrintSessions(os.Stdout)
		case "permissions":
			app, err := open(args[1:])
			if err != nil {
				return err
			}
			defer app.Close()
			app.PrintPermissions(os.Stdout)
			return nil
		case "setup":
			rest := args[1:]
			if len(rest) > 0 && rest[0] == "jev" {
				return setupJevCmd(rest[1:])
			}
			app, err := open(rest)
			if err != nil {
				return err
			}
			defer app.Close()
			return app.Setup(os.Stdin, os.Stdout)
		case "fork":
			return forkCmd(args[1:])
		case "serve":
			return serveCmd(args[1:])
		case "acp":
			return acpCmd(args[1:])
		case "update":
			return updateCmd(args[1:])
		case "login":
			return loginCmd(args[1:])
		case "logout":
			return logoutCmd(args[1:])
		default:
			return fmt.Errorf("unknown command %q\n%s", args[0], usage)
		}
	}
	return root(args)
}

func open(args []string) (*cli.App, error) {
	fs := flag.NewFlagSet("rock", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	cwd := fs.String("cwd", "", "workspace")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	dir := *cwd
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	return cli.Open(dir)
}

func root(args []string) error {
	fs := flag.NewFlagSet("rock", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	printPrompt := fs.String("p", "", "run one headless turn with this prompt")
	fs.StringVar(printPrompt, "print", "", "run one headless turn with this prompt")
	output := fs.String("output", "text", "text or streaming-json")
	resume := fs.String("resume", "", "resume a session id")
	sessionID := fs.String("session-id", "", "create or resume this session id")
	yolo := fs.Bool("yolo", false, "skip asks; the destructive gate still blocks")
	verbose := fs.Bool("verbose", false, "show Jev turn and risk diagnostics in the TUI")
	mode := fs.String("mode", "", "default, plan, or yolo")
	cwd := fs.String("cwd", "", "workspace")
	fs.Usage = func() { fmt.Fprint(fs.Output(), usage) }
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	rest := strings.TrimSpace(strings.Join(fs.Args(), " "))
	dir := *cwd
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	app, err := cli.Open(dir)
	if err != nil {
		return err
	}
	defer app.Close()
	app.ApplyFlags(*mode, *yolo)

	id := *resume
	create := false
	if id == "" && *sessionID != "" {
		id = *sessionID
		create = true
	}
	title := "rock"
	if t := strings.TrimSpace(os.Getenv("ROCK_SESSION_TITLE")); t != "" {
		title = t
	}
	prompt := strings.TrimSpace(*printPrompt)
	if rest != "" {
		if prompt != "" {
			prompt = strings.TrimSpace(prompt + " " + rest)
		} else {
			prompt = rest
		}
	}
	if prompt != "" && os.Getenv("ROCK_SESSION_TITLE") == "" {
		title = clipTitle(prompt)
	}
	sess, err := app.OpenSession(id, create || id == "", title)
	if err != nil {
		return err
	}
	sess.Meta.Mode = string(app.Policy().Mode)

	if *printPrompt != "" || (*output == "streaming-json" && prompt != "") {
		format := *output
		if format != "streaming-json" {
			format = "text"
		}
		if prompt == "" {
			return fmt.Errorf("headless mode needs a prompt")
		}
		if err := app.RequireJev(context.Background()); err != nil {
			return err
		}
		noticeIfQuiet(app.Loaded.File)
		return app.Headless(context.Background(), sess, prompt, format, os.Stdout, os.Stderr, askHeadless)
	}
	rules := append([]string{}, app.Policy().Allow...)
	for _, r := range app.Policy().Ask {
		rules = append(rules, "ask "+r)
	}
	for _, r := range app.Policy().Deny {
		rules = append(rules, "deny "+r)
	}
	if envOn("ROCK_VERBOSE") {
		*verbose = true
	}
	return startTUI(app, sess, prompt, rules, *verbose)
}

func envOn(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func startTUI(app *cli.App, sess *session.Session, initial string, rules []string, verbose bool) error {
	needGate := app.RequireJev(context.Background()) != nil
	model := tui.New(tui.Deps{
		CWD:           app.CWD,
		Session:       sess,
		Mode:          app.Policy().Mode,
		ReviewOnly:    app.Policy().ReviewOnly,
		JevMode:       app.Gates.Mode(),
		Gates:         app.Gates,
		Provider:      app.Provider.Name(),
		Auth:          app.Auth,
		FastModel:     app.FastModel(),
		StrongModel:   app.StrongModel(),
		InitialPrompt: initial,
		Run:           app.RunTurn,
		ListSessions: func() []session.Meta {
			list, _ := session.List(app.CWD)
			return list
		},
		LoadSession: func(id string) (*session.Session, error) {
			return session.Load(app.CWD, id)
		},
		Rules: rules,
		Fork: func(prompt string) (string, error) {
			return cli.ForkSequence(true, "", prompt, clipTitle(prompt), app.CWD)
		},
		SetMode: func(mode perms.Mode) {
			app.Loaded.Policy.Mode = mode
			app.Loaded.File.Mode = string(mode)
		},
		SetAuth: func(class string) (string, string, error) {
			if err := app.SetAuth(class); err != nil {
				return "", "", err
			}
			return app.Provider.Name(), app.Auth, nil
		},
		HasAPIKey:   config.APIKey() != "",
		HasSIWC:     siwc.LoggedIn(),
		HasGrokCLI:  grokcli.Look(app.Loaded.File.GrokBin).Found,
		Output:      os.Stdout,
		Verbose:     verbose,
		JevGate:     needGate,
		CheckJev:    app.CheckJevKey,
		SaveJev:     config.SaveJevKey,
		UpdateCheck: app.Loaded.File.Update.CheckOn() && update.AutoCheck(),
	})
	return tui.Run(model)
}

func askHeadless(tool, detail string) perms.Decision {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintf(os.Stderr, "denied %s (no terminal to approve): %s\n", tool, detail)
		return perms.Deny
	}
	fmt.Fprintf(os.Stderr, "allow %s?\n%s\n[y/N] ", tool, detail)
	var ans string
	_, _ = fmt.Fscanln(os.Stdin, &ans)
	if strings.EqualFold(ans, "y") || strings.EqualFold(ans, "yes") {
		return perms.Allow
	}
	return perms.Deny
}

func serveCmd(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	addr := fs.String("addr", "127.0.0.1:8787", "loopback listen address")
	cwd := fs.String("cwd", "", "workspace")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	dir := *cwd
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	app, err := cli.Open(dir)
	if err != nil {
		return err
	}
	defer app.Close()
	if err := app.RequireJev(context.Background()); err != nil {
		return err
	}
	app.ConnectMCP(context.Background())
	noticeIfQuiet(app.Loaded.File)
	fmt.Fprintf(os.Stderr, "rock serve %s\n", *addr)
	srv := serve.New(app.Factory)
	return serve.Listen(context.Background(), *addr, srv.Handler())
}

func acpCmd(args []string) error {
	fs := flag.NewFlagSet("acp", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	cwd := fs.String("cwd", "", "workspace")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	dir := *cwd
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	app, err := cli.Open(dir)
	if err != nil {
		return err
	}
	defer app.Close()
	if err := app.RequireJev(context.Background()); err != nil {
		return err
	}
	app.ConnectMCP(context.Background())
	noticeIfQuiet(app.Loaded.File)
	agent := &acp.Agent{Factory: app.Factory, CWD: app.CWD, In: os.Stdin, Out: os.Stdout}
	return agent.Serve(context.Background())
}

func setupJevCmd(args []string) error {
	fs := flag.NewFlagSet("setup jev", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	key := fs.String("key", "", "Jev API key")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	return cli.SetupJev(context.Background(), *key, os.Stdin, os.Stdout)
}

func forkCmd(args []string) error {
	fs := flag.NewFlagSet("fork", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fresh := fs.Bool("new", true, "ask the host for a new session")
	id := fs.String("session", "", "session id to put in the sequence")
	prompt := fs.String("prompt", "", "initial prompt")
	title := fs.String("title", "", "pane title")
	cwd := fs.String("cwd", "", "workspace")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	seq, err := cli.ForkSequence(*fresh, *id, *prompt, *title, *cwd)
	if err != nil {
		return err
	}
	_, err = os.Stdout.WriteString(seq)
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "emitted OSC 7880")
	return nil
}

// loginCmd stores ChatGPT tokens. It does not call RequireJev: this is
// credential plumbing, not running the agent.
func loginCmd(args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	cwd := fs.String("cwd", "", "workspace")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	rest := fs.Args()
	if len(rest) != 1 || rest[0] != "chatgpt" {
		return fmt.Errorf("usage: rock login chatgpt")
	}
	dir := *cwd
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	app, err := cli.Open(dir)
	if err != nil {
		return err
	}
	defer app.Close()
	return app.LoginChatGPT(context.Background(), os.Stdout)
}

func logoutCmd(args []string) error {
	fs := flag.NewFlagSet("logout", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	cwd := fs.String("cwd", "", "workspace")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	rest := fs.Args()
	if len(rest) != 1 || rest[0] != "chatgpt" {
		return fmt.Errorf("usage: rock logout chatgpt")
	}
	dir := *cwd
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	app, err := cli.Open(dir)
	if err != nil {
		return err
	}
	defer app.Close()
	return app.LogoutChatGPT(context.Background(), os.Stdout)
}

func noticeIfQuiet(file config.File) {
	if !file.Update.CheckOn() {
		return
	}
	update.MaybeStderr(os.Stderr, version.Version)
}

func runningExe() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved, nil
	}
	return exe, nil
}

func updateCmd(args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	check := fs.Bool("check", false, "print current and latest without installing")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	ctx := context.Background()
	c := &update.Client{}
	n, err := update.Refresh(ctx, version.Version, c)
	if err != nil {
		return err
	}
	fmt.Printf("%s -> %s\n", version.Version, n.Latest)
	if *check {
		return nil
	}
	if !n.Newer {
		return nil
	}
	exe, err := runningExe()
	if err != nil {
		return err
	}
	if update.InstalledByGo(exe) {
		fmt.Fprintln(os.Stderr, "this binary was installed with go install; it is not replaced")
		fmt.Println(update.GoInstallHint())
		return nil
	}
	got, err := c.Apply(ctx, exe)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "updated to v%s\n", got)
	return nil
}

func clipTitle(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) > 48 {
		return s[:48]
	}
	if s == "" {
		return "rock"
	}
	return s
}

const usage = `rock is a coding agent.

  rock                         fullscreen TUI
  rock -p "prompt"             one headless turn, text on stdout
  rock -p "prompt" --output streaming-json
  rock --resume ID             open a session in the TUI
  rock --session-id ID -- prompt
  rock --yolo -p "prompt"      skip asks; destructive commands still block
  rock --mode plan -p "prompt"
  rock --verbose               TUI shows Jev turn/risk diagnostic lines

  rock inspect                 config, skills, MCP, Jev mode, model auth class
  rock login chatgpt           Sign in with ChatGPT (OSS SIWC). Not Jev.
  rock logout chatgpt          revoke and clear the ChatGPT session
  rock setup                   Huh form; does not write API keys
  rock setup jev               validate and store a Jev key
  rock sessions                list sessions for this folder
  rock permissions             allow / ask / deny
  rock fork                    print a Suzuri OSC 7880 sequence
  rock serve                   loopback HTTP and SSE
  rock acp                     ACP v1 and v2 on stdio
  rock update                  install the latest GitHub release
  rock update --check          print current -> latest
  rock version

A working Jev key is required to run the agent. The TUI asks on first launch. Scripts use rock setup jev.
Headless -p, serve, and acp fail without a valid key.
rock login chatgpt and rock logout chatgpt store model credentials only; they work
before Jev setup and never start the agent.

Sessions live under ~/.rock (ROCK_HOME). Config is ~/.config/rock/config.toml (ROCK_CONFIG).
Model keys: ROCK_API_KEY or OPENAI_API_KEY. ChatGPT plan: rock login chatgpt.
SuperGrok: official grok on PATH, then grok login. Config: auth = "grok-cli" (or provider = "grok-cli").
Jev keys: JEV_API_KEY, TYPESAFE_API_KEY, the OS keychain, or ROCK_HOME/jev.key.
ROCK_VERBOSE=1 is the same as --verbose: Jev turn/risk lines stay in the TUI transcript.

rock update replaces a release binary after checksum verification. A go install
tree (GOPATH/bin, or a module-versioned build) prints:
  go install github.com/StephenSHorton/rock/cmd/rock@latest
and is not overwritten. Opt out of the once-a-day notice with ROCK_NO_UPDATE=1
or [update] check = false. The check is off in CI and under ROCK_TEST_FAKE_JEV.
`
