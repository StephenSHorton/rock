package tui

import (
	"strings"
	"time"
)

// LogFunc writes one structured line to ROCK_HOME/rock.log. level is
// "info", "warn" or "error". Never pass prompt text, replies or secrets.
type LogFunc func(level, msg string, keyvals ...any)

func (m *Model) log(level, msg string, keyvals ...any) {
	if m.deps.Log == nil {
		return
	}
	m.deps.Log(level, msg, keyvals...)
}

// logResize records size changes, at most every resizeLogEvery so a drag
// does not flood the log. Turn lines also carry the size.
const resizeLogEvery = 500 * time.Millisecond

func (m *Model) noteSize(w, h int) {
	if w == m.width && h == m.height {
		return
	}
	first := m.width == 0 && m.height == 0
	m.seen.set(w, h)
	now := time.Now()
	if first {
		m.log("info", "tui size", "w", w, "h", h)
		m.sizeLogAt = now
		return
	}
	m.resizes++
	if now.Sub(m.sizeLogAt) < resizeLogEvery {
		return
	}
	m.log("info", "tui resize", "w", w, "h", h, "events", m.resizes)
	m.sizeLogAt, m.resizes = now, 0
}

// visibleSince counts transcript lines added since index from that the
// user can see whatever the verbose setting (Jev diagnostics excluded).
func (m *Model) visibleSince(from int) int {
	n := 0
	for i := from; i >= 0 && i < len(m.lines); i++ {
		switch m.lines[i].kind {
		case "assistant", "tool", "result", "permission", "error":
			n++
		}
	}
	return n
}

func oneLineErr(err error) string {
	if err == nil {
		return ""
	}
	s := strings.Join(strings.Fields(err.Error()), " ")
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}

// emptyReplyText is shown when a turn finishes without any visible output,
// so Enter never looks like it did nothing.
func (m *Model) emptyReplyText() string {
	s := "No reply: the model returned an empty response. Try again"
	if m.deps.Auth == "grok-cli" {
		return s + "; if it keeps happening, start rock with ROCK_DEBUG_GROK=1 and check rock.log in ROCK_HOME."
	}
	return s + "; details are in rock.log in ROCK_HOME."
}
