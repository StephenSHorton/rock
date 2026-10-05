// Package grokcli talks to the official unmodified grok binary over ACP
// stdio. Rock never copies Grok's OAuth client id, never runs grok login,
// and never reads ~/.grok token files.
package grokcli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// AuthClass is the inspect / config pick.
	AuthClass = "grok-cli"
	// InstallURL is the official installer page.
	InstallURL = "https://x.ai/cli"
	// DefaultName is the official binary.
	DefaultName = "grok"
)

// Status is what inspect and the picker show. SignedIn is only set after
// an ACP initialize that lists cached_token (or grok's own XAI_API_KEY
// auth method). We do not read ~/.grok/auth.json to decide.
type Status struct {
	Bin      string
	Found    bool
	SignedIn bool
	Detail   string
}

// ResolveBin returns the configured path or grok on PATH.
// Order: ROCK_GROK_BIN, configured path, LookPath("grok").
func ResolveBin(configured string) string {
	if v := strings.TrimSpace(os.Getenv("ROCK_GROK_BIN")); v != "" {
		return v
	}
	if v := strings.TrimSpace(configured); v != "" {
		return v
	}
	if p, err := exec.LookPath(DefaultName); err == nil {
		return p
	}
	return ""
}

// Look reports whether a grok binary is configured or on PATH.
func Look(configured string) Status {
	bin := ResolveBin(configured)
	if bin == "" {
		return Status{Detail: missingBin()}
	}
	if filepath.IsAbs(bin) || strings.Contains(bin, string(filepath.Separator)) {
		if _, err := os.Stat(bin); err != nil {
			return Status{Bin: bin, Detail: missingBin()}
		}
	} else if _, err := exec.LookPath(bin); err != nil {
		return Status{Bin: bin, Detail: missingBin()}
	}
	return Status{Bin: bin, Found: true, Detail: "official grok binary"}
}

func missingBin() string {
	return fmt.Sprintf("grok is not on PATH. Install the official binary from %s then run grok login. Rock does not sign you in and does not read ~/.grok.", InstallURL)
}

func notSignedIn() string {
	return "grok is not signed in. Run grok login in the official binary. Rock never automates Grok sign-in and never reads Grok token files."
}

// ChildArgs is grok agent stdio. grok 1.0.41 rejected --no-auto-update;
// keep --no-leader. Never --always-approve, --yolo, or a bypass
// permission mode — those would let the child run tools.
func ChildArgs() []string {
	return []string{"agent", "--no-leader", "stdio"}
}

// ChildArgsModel is ChildArgs with -m <model> for agents that only
// accept the model on the command line (no ACP set_model / config option).
func ChildArgsModel(model string) []string {
	model = strings.TrimSpace(model)
	if model == "" {
		return ChildArgs()
	}
	return []string{"agent", "--no-leader", "-m", model, "stdio"}
}

func forbiddenFlag(arg string) bool {
	s := strings.ToLower(strings.TrimSpace(arg))
	s = strings.TrimLeft(s, "-")
	switch s {
	case "always-approve", "yolo", "reauth", "reauthenticate":
		return true
	}
	if strings.HasPrefix(s, "permission-mode") {
		return true
	}
	return false
}

func rejectForbidden(args []string) error {
	for _, a := range args {
		if forbiddenFlag(a) {
			return fmt.Errorf("grok-cli: refusing %s; the child must not auto-approve tools", a)
		}
	}
	return nil
}
