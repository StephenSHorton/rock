package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/StephenSHorton/rock/internal/config"
	"github.com/StephenSHorton/rock/internal/jev"
)

// ErrNeedJev is the fail-fast message for headless clients.
var ErrNeedJev = errors.New("Jev is required to run Rock. Run rock setup jev")

// RequireJev validates the configured key with a real Jev call.
func (a *App) RequireJev(ctx context.Context) error {
	key, src := config.JevKey()
	if key == "" {
		return ErrNeedJev
	}
	c := a.Gates.Client
	if c == nil {
		c = &jev.Client{APIKey: key, BaseURL: a.Loaded.File.Jev.BaseURL, Model: a.Loaded.File.Jev.Model}
		a.Gates.Client = c
	} else {
		c.APIKey = key
	}
	if err := c.Ping(ctx); err != nil {
		if src == "" {
			src = "the configured key"
		}
		return fmt.Errorf("Jev key from %s failed: %w\nRun rock setup jev", src, err)
	}
	return nil
}

// CheckJevKey proves one key with a Decide call. Used by the TUI gate.
func (a *App) CheckJevKey(ctx context.Context, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("jev key is not set")
	}
	c := &jev.Client{APIKey: key, BaseURL: a.Loaded.File.Jev.BaseURL, Model: a.Loaded.File.Jev.Model}
	return c.Ping(ctx)
}

// SetupJev reads a key from flag/stdin/env, validates it, and stores it.
func SetupJev(ctx context.Context, key string, stdin io.Reader, stdout io.Writer) error {
	key = strings.TrimSpace(key)
	if key == "" {
		if v := strings.TrimSpace(os.Getenv("JEV_API_KEY")); v != "" {
			key = v
		} else if v := strings.TrimSpace(os.Getenv("TYPESAFE_API_KEY")); v != "" {
			key = v
		} else if stdin != nil && !stdinIsTerminal(stdin) {
			raw, err := io.ReadAll(io.LimitReader(stdin, 4096))
			if err != nil {
				return err
			}
			key = strings.TrimSpace(string(raw))
		}
	}
	if key == "" {
		return fmt.Errorf("no Jev key. Pass --key, pipe it on stdin, or set JEV_API_KEY")
	}
	c := &jev.Client{APIKey: key}
	if err := c.Ping(ctx); err != nil {
		return fmt.Errorf("Jev rejected this key: %w", err)
	}
	store, err := config.SaveJevKey(key)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "saved Jev key in %s\n", store)
	return nil
}

func stdinIsTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}
