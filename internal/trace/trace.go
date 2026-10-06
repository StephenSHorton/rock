// Package trace writes turn-stage lines to rock.log and tracks the last
// time a turn made progress, so the TUI stall watchdog can say where a
// turn is stuck. Never pass prompt text, replies or secrets.
package trace

import (
	"sync"
	"sync/atomic"
	"time"
)

// LogFunc matches the TUI logger: level is "info", "warn" or "error".
type LogFunc func(level, msg string, keyvals ...any)

var (
	mu    sync.RWMutex
	logf  LogFunc
	stage atomic.Value // string
	last  atomic.Int64 // unix nanos of the last progress mark
)

// SetLogger installs the rock.log writer. nil turns logging off.
func SetLogger(f LogFunc) {
	mu.Lock()
	logf = f
	mu.Unlock()
}

func emit(level, msg string, kv ...any) {
	mu.RLock()
	f := logf
	mu.RUnlock()
	if f != nil {
		f(level, msg, kv...)
	}
}

// Info logs a stage line and marks progress at that stage.
func Info(msg string, kv ...any) {
	Mark(msg)
	emit("info", msg, kv...)
}

// Warn logs without moving the stage.
func Warn(msg string, kv ...any) { emit("warn", msg, kv...) }

// Error logs without moving the stage.
func Error(msg string, kv ...any) { emit("error", msg, kv...) }

// Mark records progress at stage without logging (stream chunks).
func Mark(name string) {
	if name != "" {
		stage.Store(name)
	}
	last.Store(time.Now().UnixNano())
}

// Progress records progress without changing the stage name.
func Progress() { last.Store(time.Now().UnixNano()) }

// Last returns the most recent stage and when progress was last seen.
func Last() (string, time.Time) {
	s, _ := stage.Load().(string)
	n := last.Load()
	if n == 0 {
		return s, time.Time{}
	}
	return s, time.Unix(0, n)
}

// Since returns milliseconds since t, for stage timings.
func Since(t time.Time) int64 { return time.Since(t).Milliseconds() }
