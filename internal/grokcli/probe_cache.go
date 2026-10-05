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
	probeCacheTTL  = 5 * time.Minute
	// ProbeTimeout caps the ACP initialize used for auto-detect and
	// inspect. Keep it short so a missing or hung grok does not stall
	// Open() (and therefore the TUI's first frame).
	ProbeTimeout = 2 * time.Second
)

type probeCache struct {
	Bin      string    `json:"bin"`
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

func loadProbeCache(bin string) (Status, bool) {
	raw, err := os.ReadFile(probeCachePath())
	if err != nil {
		return Status{}, false
	}
	var c probeCache
	if json.Unmarshal(raw, &c) != nil {
		return Status{}, false
	}
	if c.Bin != bin || c.Checked.IsZero() || time.Since(c.Checked) > probeCacheTTL {
		return Status{}, false
	}
	return Status{Bin: c.Bin, Found: true, SignedIn: c.SignedIn, Detail: c.Detail}, true
}

func saveProbeCache(st Status) {
	if !st.Found || st.Bin == "" {
		return
	}
	c := probeCache{
		Bin:      st.Bin,
		SignedIn: st.SignedIn,
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
