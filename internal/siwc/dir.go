package siwc

import (
	"os"
	"path/filepath"
	"strings"
)

// Dir is where SIWC files live: host id and ChatGPT credentials.
// Preference: ROCK_SIWC_HOME, then the directory of ROCK_CONFIG,
// then ROCK_HOME, then ~/.config/rock. Never ~/.codex.
func Dir() string {
	if d := strings.TrimSpace(os.Getenv("ROCK_SIWC_HOME")); d != "" {
		return d
	}
	if p := strings.TrimSpace(os.Getenv("ROCK_CONFIG")); p != "" {
		return filepath.Dir(p)
	}
	if h := strings.TrimSpace(os.Getenv("ROCK_HOME")); h != "" {
		return h
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "rock"
	}
	return filepath.Join(dir, "rock")
}

func hostPath() string {
	return filepath.Join(Dir(), "siwc-host.json")
}

func credPath() string {
	return filepath.Join(Dir(), "chatgpt.json")
}

func lockPath() string {
	return filepath.Join(Dir(), "chatgpt.lock")
}

func rejectsCodexPath(p string) bool {
	return strings.Contains(filepath.ToSlash(strings.ToLower(p)), "/.codex") ||
		strings.Contains(strings.ToLower(filepath.Base(p)), "codex")
}
