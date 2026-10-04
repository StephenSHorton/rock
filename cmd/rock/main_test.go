package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/StephenSHorton/rock/internal/cli"
)

func TestHelpReturnsNil(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"--help"}, {"help"}} {
		if err := run(args); err != nil {
			t.Fatalf("run(%q) = %v", args, err)
		}
	}
}

func isolateJev(t *testing.T) {
	t.Helper()
	t.Setenv("ROCK_HOME", t.TempDir())
	t.Setenv("ROCK_CONFIG", t.TempDir()+"/config.toml")
	t.Setenv("JEV_API_KEY", "")
	t.Setenv("TYPESAFE_API_KEY", "")
	t.Setenv("ROCK_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ROCK_TEST_FAKE_JEV", "")
}

func TestHeadlessPrintNeedsJev(t *testing.T) {
	isolateJev(t)
	err := run([]string{"-p", "hello", "--cwd", t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "rock setup jev") {
		t.Fatalf("got %v", err)
	}
	if !errors.Is(err, cli.ErrNeedJev) && !strings.Contains(err.Error(), "Jev is required") {
		t.Fatalf("got %v", err)
	}
}

func TestServeNeedsJev(t *testing.T) {
	isolateJev(t)
	err := run([]string{"serve", "--cwd", t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "rock setup jev") {
		t.Fatalf("got %v", err)
	}
}

func TestACPNeedsJev(t *testing.T) {
	isolateJev(t)
	err := run([]string{"acp", "--cwd", t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "rock setup jev") {
		t.Fatalf("got %v", err)
	}
}

func TestSetupJevValidAndInvalidFromCLI(t *testing.T) {
	isolateJev(t)
	t.Setenv("ROCK_TEST_FAKE_JEV", "reject")
	if err := run([]string{"setup", "jev", "--key", "bad-key"}); err == nil {
		t.Fatal("invalid key should fail")
	}
	t.Setenv("ROCK_TEST_FAKE_JEV", "1")
	if err := run([]string{"setup", "jev", "--key", "good-key"}); err != nil {
		t.Fatal(err)
	}
}
