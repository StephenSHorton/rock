package update

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/StephenSHorton/rock/internal/session"
)

const cacheName = "update-check.json"

// Notice is a newer release the user can install.
type Notice struct {
	Current string
	Latest  string
	Newer   bool
}

func (n Notice) Line() string {
	if !n.Newer || n.Latest == "" {
		return ""
	}
	return "Rock v" + Normalize(n.Latest) + " available, run rock update"
}

type cacheFile struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
	Current   string    `json:"current"`
}

// DisabledReason is a non-empty string when auto-update must stay off.
func DisabledReason() string {
	if on(os.Getenv("ROCK_NO_UPDATE")) {
		return "ROCK_NO_UPDATE"
	}
	if on(os.Getenv("ROCK_DISABLE_AUTOUPDATE")) {
		return "ROCK_DISABLE_AUTOUPDATE"
	}
	if on(os.Getenv("CI")) {
		return "CI"
	}
	if strings.TrimSpace(os.Getenv("ROCK_TEST_FAKE_JEV")) != "" {
		return "ROCK_TEST_FAKE_JEV"
	}
	return ""
}

func on(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// AutoCheck is the launch-time background check. Tests never start it.
func AutoCheck() bool {
	if testing.Testing() {
		return false
	}
	return DisabledReason() == ""
}

func cachePath() string {
	return filepath.Join(session.Home(), cacheName)
}

// CachedNotice returns a stored notice if the cache is still fresh.
func CachedNotice(current string) (Notice, bool) {
	raw, err := os.ReadFile(cachePath())
	if err != nil {
		return Notice{}, false
	}
	var c cacheFile
	if json.Unmarshal(raw, &c) != nil {
		return Notice{}, false
	}
	if time.Since(c.CheckedAt) > 24*time.Hour {
		return Notice{}, false
	}
	n := Notice{Current: current, Latest: c.Latest, Newer: Compare(current, c.Latest) < 0}
	return n, true
}

func writeCache(current, latest string) {
	c := cacheFile{CheckedAt: time.Now().UTC(), Latest: latest, Current: current}
	raw, err := json.Marshal(c)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(cachePath()), 0o755)
	_ = os.WriteFile(cachePath(), raw, 0o600)
}

// Check looks up the latest release, using a 24h cache.
func Check(ctx context.Context, current string, c *Client) (Notice, error) {
	if n, ok := CachedNotice(current); ok {
		return n, nil
	}
	return Refresh(ctx, current, c)
}

// Refresh hits the API and rewrites the cache.
func Refresh(ctx context.Context, current string, c *Client) (Notice, error) {
	if c == nil {
		c = &Client{}
	}
	rel, err := c.Latest(ctx)
	if err != nil {
		return Notice{}, err
	}
	latest := rel.Version()
	writeCache(current, latest)
	return Notice{Current: current, Latest: latest, Newer: Compare(current, latest) < 0}, nil
}

// MaybeStderr prints at most one line if a fresh cache already knows a
// newer release. The API refresh runs in the background.
func MaybeStderr(w interface{ WriteString(string) (int, error) }, current string) {
	if w == nil || !AutoCheck() {
		return
	}
	if n, ok := CachedNotice(current); ok && n.Newer {
		_, _ = w.WriteString(n.Line() + "\n")
	}
	go func() {
		_, _ = Refresh(context.Background(), current, nil)
	}()
}
