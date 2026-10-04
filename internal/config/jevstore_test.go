package config

import (
	"errors"
	"os"
	"testing"
)

func TestJevKeyPrefersEnvThenFile(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	t.Setenv("JEV_API_KEY", "")
	t.Setenv("TYPESAFE_API_KEY", "")
	origSet, origGet := keyringSet, keyringGet
	t.Cleanup(func() { keyringSet, keyringGet = origSet, origGet })
	keyringGet = func(string, string) (string, error) { return "", errors.New("no keychain") }
	keyringSet = func(string, string, string) error { return errors.New("no keychain") }

	if k, src := JevKey(); k != "" || src != "" {
		t.Fatalf("empty: %q %q", k, src)
	}
	store, err := SaveJevKey("file-key")
	if err != nil {
		t.Fatal(err)
	}
	if store == "OS keychain" {
		t.Fatal("expected file fallback")
	}
	info, err := os.Stat(JevKeyFile())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm %o", info.Mode().Perm())
	}
	k, src := JevKey()
	if k != "file-key" || src == "" {
		t.Fatalf("file key %q %q", k, src)
	}
	t.Setenv("JEV_API_KEY", "env-key")
	if k, src = JevKey(); k != "env-key" || src != "JEV_API_KEY" {
		t.Fatalf("env %q %q", k, src)
	}
}

func TestSaveJevKeyUsesKeychainWhenItWorks(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	t.Setenv("JEV_API_KEY", "")
	t.Setenv("TYPESAFE_API_KEY", "")
	origSet, origGet := keyringSet, keyringGet
	t.Cleanup(func() { keyringSet, keyringGet = origSet, origGet })
	mem := ""
	keyringSet = func(_, _, pass string) error { mem = pass; return nil }
	keyringGet = func(_, _ string) (string, error) {
		if mem == "" {
			return "", errors.New("empty")
		}
		return mem, nil
	}
	store, err := SaveJevKey("chain-key")
	if err != nil || store != "OS keychain" {
		t.Fatalf("store %q err %v", store, err)
	}
	if _, err := os.Stat(JevKeyFile()); !os.IsNotExist(err) {
		t.Fatal("file should not be written when keychain works")
	}
	k, src := JevKey()
	if k != "chain-key" || src != "OS keychain" {
		t.Fatalf("got %q %q", k, src)
	}
}
