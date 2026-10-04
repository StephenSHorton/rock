package config

import (
	"errors"
	"os"
	"testing"
)

func clearJevEnv(t *testing.T) {
	t.Helper()
	t.Setenv("JEV_API_KEY", "")
	t.Setenv("TYPESAFE_API_KEY", "")
}

func TestJevKeyPrefersEnvThenFile(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	clearJevEnv(t)
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

func TestJevKeyWhitespaceIsUnset(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	t.Setenv("JEV_API_KEY", "   ")
	t.Setenv("TYPESAFE_API_KEY", "\t\n")
	origSet, origGet := keyringSet, keyringGet
	t.Cleanup(func() { keyringSet, keyringGet = origSet, origGet })
	keyringGet = func(string, string) (string, error) { return "  ", nil }
	keyringSet = func(string, string, string) error { return errors.New("no keychain") }
	if k, src := JevKey(); k != "" || src != "" {
		t.Fatalf("whitespace: %q %q", k, src)
	}
}

func TestSaveJevKeyUsesKeychainWhenItWorks(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	clearJevEnv(t)
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

func TestSecretsChatGPTUserIsNotJev(t *testing.T) {
	mem := map[string]string{}
	origSet, origGet := SwapKeyring(
		func(_, user, pass string) error { mem[user] = pass; return nil },
		func(_, user string) (string, error) {
			if v, ok := mem[user]; ok {
				return v, nil
			}
			return "", errors.New("empty")
		},
	)
	origDel := keyringDelete
	keyringDelete = func(_, user string) error { delete(mem, user); return nil }
	t.Cleanup(func() {
		SwapKeyring(origSet, origGet)
		keyringDelete = origDel
	})
	s := Secrets{}
	if err := s.Set(KeyringUserChatGPT, []byte(`{"client_id":"oaiapp_x"}`)); err != nil {
		t.Fatal(err)
	}
	if _, ok := mem[keyringUser]; ok {
		t.Fatal("ChatGPT write must not use the Jev keyring user")
	}
	raw, err := s.Get(KeyringUserChatGPT)
	if err != nil || string(raw) != `{"client_id":"oaiapp_x"}` {
		t.Fatalf("%q %v", raw, err)
	}
	if err := s.Delete(KeyringUserChatGPT); err != nil {
		t.Fatal(err)
	}
}
