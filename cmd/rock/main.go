package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/StephenSHorton/rock/internal/acp"
	"github.com/StephenSHorton/rock/internal/cli"
	"github.com/StephenSHorton/rock/internal/perms"
	"github.com/StephenSHorton/rock/internal/serve"
	"github.com/StephenSHorton/rock/internal/session"
	"github.com/StephenSHorton/rock/internal/tui"
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
			app, err := open(args[1:])
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
		return app.Headless(context.Background(), sess, prompt, format, os.Stdout, os.Stderr, askHeadless)
	}
	rules := append([]string{}, app.Policy().Allow...)
	for _, r := range app.Policy().Ask {
		rules = append(rules, "ask "+r)
	}
	for _, r := range app.Policy().Deny {
		rules = append(rules, "deny "+r)
	}
	return startTUI(app, sess, prompt, rules)
}

func startTUI(app *cli.App, sess *session.Session, initial string, rules []string) error {
	model := tui.New(tui.Deps{
		CWD:           app.CWD,
		Session:       sess,
		Mode:          app.Policy().Mode,
		ReviewOnly:    app.Policy().ReviewOnly,
		JevMode:       app.Gates.Mode(),
		Gates:         app.Gates,
		Provider:      app.Provider.Name(),
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
		Output: os.Stdout,
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
	app.ConnectMCP(context.Background())
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
	app.ConnectMCP(context.Background())
	agent := &acp.Agent{Factory: app.Factory, CWD: app.CWD, In: os.Stdin, Out: os.Stdout}
	return agent.Serve(context.Background())
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

  rock inspect                 config, skills, MCP, Jev mode
  rock setup                   Huh form; does not write API keys
  rock sessions                list sessions for this folder
  rock permissions             allow / ask / deny
  rock fork                    print a Suzuri OSC 7880 sequence
  rock serve                   loopback HTTP and SSE
  rock acp                     ACP v1 and v2 on stdio
  rock version

Sessions live under ~/.rock (ROCK_HOME). Config is ~/.config/rock/config.toml (ROCK_CONFIG).
Model keys: ROCK_API_KEY or OPENAI_API_KEY. Jev keys: JEV_API_KEY or TYPESAFE_API_KEY.
`
