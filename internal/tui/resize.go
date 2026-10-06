package tui

import (
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/term"
)

// resizePollEvery is how often the Windows fallback re-reads the console
// size.
const resizePollEvery = 250 * time.Millisecond

// sizeSeen is the last size the model applied. The resize poller compares
// against it so a missed or wrong native resize event (Windows console
// input records) is corrected within one poll.
type sizeSeen struct{ w, h atomic.Int64 }

func (s *sizeSeen) set(w, h int) {
	if s == nil {
		return
	}
	s.w.Store(int64(w))
	s.h.Store(int64(h))
}

func (s *sizeSeen) get() (int, int) {
	if s == nil {
		return 0, 0
	}
	return int(s.w.Load()), int(s.h.Load())
}

// pollResize sends a WindowSizeMsg whenever get reports a size that differs
// from what the model last applied. It returns when done closes.
func pollResize(done <-chan struct{}, every time.Duration, seen *sizeSeen, get func() (int, int, error), send func(tea.Msg)) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-done:
			return
		case <-t.C:
		}
		w, h, err := get()
		if err != nil || w <= 0 || h <= 0 {
			continue
		}
		if sw, sh := seen.get(); sw == w && sh == h {
			continue
		}
		send(tea.WindowSizeMsg{Width: w, Height: h})
	}
}

// resizePollOn reports whether Run should start the console-size poller.
// Bubble Tea gets resizes from SIGWINCH on Unix, which is reliable. On
// Windows they come from console input records, which another process on
// the same console can consume, and some hosts only report the buffer
// size. ROCK_TUI_POLL_RESIZE=1/0 forces it on or off on any platform.
func resizePollOn() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ROCK_TUI_POLL_RESIZE"))) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return runtime.GOOS == "windows"
}

// startResizePoll starts the poller on the terminal Rock renders to.
func startResizePoll(seen *sizeSeen, send func(tea.Msg)) (stop func()) {
	if !resizePollOn() {
		return func() {}
	}
	fd := int(os.Stdout.Fd())
	if !term.IsTerminal(fd) {
		return func() {}
	}
	done := make(chan struct{})
	go pollResize(done, resizePollEvery, seen, func() (int, int, error) { return term.GetSize(fd) }, send)
	return func() { close(done) }
}
