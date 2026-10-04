package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/StephenSHorton/rock/internal/config"
)

func TestSetupJevValidAndInvalid(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	t.Setenv("JEV_API_KEY", "")
	t.Setenv("TYPESAFE_API_KEY", "")
	origSet, origGet := config.SwapKeyring(
		func(string, string, string) error { return errors.New("no keychain") },
		func(string, string) (string, error) { return "", errors.New("no keychain") },
	)
	t.Cleanup(func() { config.SwapKeyring(origSet, origGet) })

	t.Setenv("ROCK_TEST_FAKE_JEV", "reject")
	var out bytes.Buffer
	if err := SetupJev(context.Background(), "bad-key", nil, &out); err == nil {
		t.Fatal("expected invalid key to fail")
	}
	if _, err := os.Stat(config.JevKeyFile()); !os.IsNotExist(err) {
		t.Fatal("invalid key must not be saved")
	}

	t.Setenv("ROCK_TEST_FAKE_JEV", "1")
	out.Reset()
	if err := SetupJev(context.Background(), "good-key", nil, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "0600") && !strings.Contains(out.String(), "keychain") {
		t.Fatalf("store: %s", out.String())
	}
	raw, err := os.ReadFile(config.JevKeyFile())
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(raw)) != "good-key" {
		t.Fatalf("file %q", raw)
	}
	info, err := os.Stat(config.JevKeyFile())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm %o", info.Mode().Perm())
	}
}

func TestRequireJevFailsWithoutKey(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	t.Setenv("ROCK_CONFIG", t.TempDir()+"/config.toml")
	t.Setenv("JEV_API_KEY", "")
	t.Setenv("TYPESAFE_API_KEY", "")
	t.Setenv("ROCK_TEST_FAKE_JEV", "")
	app, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	err = app.RequireJev(context.Background())
	if !errors.Is(err, ErrNeedJev) {
		t.Fatalf("got %v", err)
	}
}

func TestRequireJevRejectsInvalidEnvKey(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	t.Setenv("ROCK_CONFIG", t.TempDir()+"/config.toml")
	t.Setenv("JEV_API_KEY", "nope")
	t.Setenv("TYPESAFE_API_KEY", "")
	t.Setenv("ROCK_TEST_FAKE_JEV", "reject")
	app, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	err = app.RequireJev(context.Background())
	if err == nil || !strings.Contains(err.Error(), "rock setup jev") {
		t.Fatalf("got %v", err)
	}
}

func TestSetupJevReadsEnv(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	t.Setenv("JEV_API_KEY", "env-good")
	t.Setenv("TYPESAFE_API_KEY", "")
	t.Setenv("ROCK_TEST_FAKE_JEV", "1")
	origSet, origGet := config.SwapKeyring(
		func(string, string, string) error { return errors.New("no keychain") },
		func(string, string) (string, error) { return "", errors.New("no keychain") },
	)
	t.Cleanup(func() { config.SwapKeyring(origSet, origGet) })
	var out bytes.Buffer
	if err := SetupJev(context.Background(), "", nil, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "saved Jev key") {
		t.Fatal(out.String())
	}
}
