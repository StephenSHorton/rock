package grokcli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const debugLineMax = 500

var debugMu sync.Mutex

func debugGrokOn() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ROCK_DEBUG_GROK"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func debugGrok(format string, args ...any) {
	if !debugGrokOn() {
		return
	}
	line := fmt.Sprintf(format, args...)
	line = redactDebug(line)
	if len(line) > debugLineMax {
		line = line[:debugLineMax] + "…"
	}
	path := filepath.Join(rockHome(), "rock.log")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	debugMu.Lock()
	defer debugMu.Unlock()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s grok %s\n", time.Now().Format(time.RFC3339Nano), line)
}

func redactDebug(s string) string {
	// Never log anything that looks like a bearer/token blob.
	lower := strings.ToLower(s)
	for _, key := range []string{"authorization", "bearer ", "api_key", "apikey", "access_token", "refresh_token", "\"token\""} {
		if i := strings.Index(lower, key); i >= 0 {
			return s[:i] + key + "=[redacted]"
		}
	}
	return s
}
