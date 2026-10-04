package config

import (
	"os"
	"os/exec"
	"strings"
)

// ChildEnv is the process environment with Jev keys removed so
// subprocesses cannot inherit JEV_API_KEY or TYPESAFE_API_KEY.
func ChildEnv() []string {
	return StripJevKeys(os.Environ())
}

// StripJevKeys drops JEV_API_KEY and TYPESAFE_API_KEY from an env slice.
func StripJevKeys(env []string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		k, _, _ := strings.Cut(e, "=")
		if k == "JEV_API_KEY" || k == "TYPESAFE_API_KEY" {
			continue
		}
		out = append(out, e)
	}
	return out
}

// ScrubCmdEnv sets cmd.Env so the child cannot see Jev keys.
func ScrubCmdEnv(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.Env == nil {
		cmd.Env = ChildEnv()
		return
	}
	cmd.Env = StripJevKeys(cmd.Env)
}
