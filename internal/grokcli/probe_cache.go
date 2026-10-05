package grokcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	probeCacheName = "grok-probe.json"
	// probeCacheVersion bumps invalidate stale 1.0.5 entries that cached
	// "context deadline exceeded" from the bad --no-auto-update ChildArgs.
	probeCacheVersion = 2
	probeCacheTTL     = 5 * time.Minute
	// ProbeTimeout caps the ACP initialize used for auto-detect and
	// inspect. Keep it short so a missing or hung grok does not stall
	// Open() (and therefore the TUI's first frame).
	ProbeTimeout = 2 * time.Second
)

type probeCache struct {
	Version  int       `json:"version"`
	Bin      string    `json:"bin"`
	BinMtime int64     `json:"bin_mtime"`
	SignedIn bool      `json:"signed_in"`
	Detail   string    `json:"detail"`
	Checked  time.Time `json:"checked"`
}

func rockHome() string {
	if h := strings.TrimSpace(os.Getenv("ROCK_HOME")); h != "" {
		return h
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".rock"
	}
	return filepath.Join(home, ".rock")
}

func probeCachePath() string {
	return filepath.Join(rockHome(), probeCacheName)
}

func binMtime(bin string) int64 {
	fi, err := os.Stat(bin)
	if err != nil {
		return 0
	}
	return fi.ModTime().UnixNano()
}

func loadProbeCache(bin string) (Status, bool) {
	raw, err := os.ReadFile(probeCachePath())
	if err != nil {
		return Status{}, false
	}
	var c probeCache
	if json.Unmarshal(raw, &c) != nil {
		return Status{}, false
	}
	if c.Version != probeCacheVersion {
		return Status{}, false
	}
	if c.Bin != bin || !c.SignedIn || c.Checked.IsZero() || time.Since(c.Checked) > probeCacheTTL {
		return Status{}, false
	}
	if mt := binMtime(bin); mt == 0 || c.BinMtime != mt {
		return Status{}, false
	}
	return Status{Bin: c.Bin, Found: true, SignedIn: true, Detail: c.Detail}, true
}

func saveProbeCache(st Status) {
	if !st.Found || !st.SignedIn || st.Bin == "" {
		return
	}
	c := probeCache{
		Version:  probeCacheVersion,
		Bin:      st.Bin,
		BinMtime: binMtime(st.Bin),
		SignedIn: true,
		Detail:   st.Detail,
		Checked:  time.Now().UTC(),
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return
	}
	path := probeCachePath()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, raw, 0o600)
}

// ClearProbeCache drops the cached signed-in probe. Tests use it.
func ClearProbeCache() {
	_ = os.Remove(probeCachePath())
}
