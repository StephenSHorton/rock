package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"

	"github.com/StephenSHorton/rock/internal/session"
)

const (
	keyringService     = "rock"
	keyringUser        = "jev"
	KeyringUserChatGPT = "chatgpt"
)

var (
	keyringSet    = keyring.Set
	keyringGet    = keyring.Get
	keyringDelete = keyring.Delete
)

// Secrets is the shared OS keychain helper (service "rock"). Jev uses user
// "jev"; Sign in with ChatGPT uses user "chatgpt". Get returns
// os.ErrNotExist when the item is missing so callers can fall back to a file.
type Secrets struct{}

func (Secrets) Get(name string) ([]byte, error) {
	v, err := keyringGet(keyringService, name)
	if err != nil {
		return nil, os.ErrNotExist
	}
	if strings.TrimSpace(v) == "" {
		return nil, os.ErrNotExist
	}
	return []byte(v), nil
}

func (Secrets) Set(name string, value []byte) error {
	return keyringSet(keyringService, name, string(value))
}

func (Secrets) Delete(name string) error {
	err := keyringDelete(keyringService, name)
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

// SwapKeyring replaces the OS keychain hooks. Tests use it to force the
// file fallback or a fake keychain. The previous pair is returned.
func SwapKeyring(set func(service, user, password string) error, get func(service, user string) (string, error)) (oldSet func(string, string, string) error, oldGet func(string, string) (string, error)) {
	oldSet, oldGet = keyringSet, keyringGet
	if set != nil {
		keyringSet = set
	}
	if get != nil {
		keyringGet = get
	}
	return oldSet, oldGet
}

// JevKeyFile is the 0600 fallback when the OS keychain is unavailable.
func JevKeyFile() string {
	return filepath.Join(session.Home(), "jev.key")
}

// JevKey returns the first configured key and where it came from.
// Order: JEV_API_KEY, TYPESAFE_API_KEY, OS keychain, ROCK_HOME/jev.key.
// Env is read on every call. Empty and whitespace-only values count as
// unset. Tests that mutate those vars must clear both and must not use
// t.Parallel.
func JevKey() (key, source string) {
	if v := strings.TrimSpace(os.Getenv("JEV_API_KEY")); v != "" {
		return v, "JEV_API_KEY"
	}
	if v := strings.TrimSpace(os.Getenv("TYPESAFE_API_KEY")); v != "" {
		return v, "TYPESAFE_API_KEY"
	}
	if v, err := keyringGet(keyringService, keyringUser); err == nil {
		if k := strings.TrimSpace(v); k != "" {
			return k, "OS keychain"
		}
	}
	if raw, err := os.ReadFile(JevKeyFile()); err == nil {
		if k := strings.TrimSpace(string(raw)); k != "" {
			return k, JevKeyFile() + " (0600)"
		}
	}
	return "", ""
}

// SaveJevKey writes the key to the OS keychain, or a 0600 file in ROCK_HOME
// when the keychain is missing. The returned string is the store that won.
func SaveJevKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", errors.New("empty Jev key")
	}
	if err := keyringSet(keyringService, keyringUser, key); err == nil {
		return "OS keychain", nil
	}
	path := JevKeyFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(key+"\n"), 0o600); err != nil {
		return "", err
	}
	return path + " (0600)", nil
}
