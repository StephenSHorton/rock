package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// Live streaming state for the current model call. Grok sends
// agent_thought_chunk (reasoning) and agent_message_chunk (reply) many
// times a second; the harness forwards them as EvThought / EvDelta.
//
//   - Thoughts go into one dim "thinking" line: the last few rows while it
//     streams, folded to "Thought for Ns" once the reply starts or the step
//     ends (select it and press enter, or click, to expand).
//   - Reply text streams into a live assistant line. Rock tool fences
//     (```tool …```) are hidden while streaming; the final EvAssistant text
//     replaces the live text, so fence parsing still uses the final reply.

// thoughtTailRows is how many wrapped rows of a streaming thought show.
const thoughtTailRows = 4

func (m *Model) resetLive() {
	m.liveThought, m.liveText = -1, -1
	m.liveRaw = ""
}

func (m *Model) onThought(delta string) {
	if delta == "" {
		return
	}
	if m.liveThought < 0 || m.liveThought >= len(m.lines) {
		m.lines = append(m.lines, line{kind: "thinking", at: time.Now()})
		m.liveThought = len(m.lines) - 1
	}
	ln := &m.lines[m.liveThought]
	ln.text += delta
	ln.out = ""
}

func (m *Model) onDelta(delta string) {
	m.finishThought()
	m.liveRaw += delta
	vis := streamVisible(m.liveRaw)
	if m.liveText < 0 || m.liveText >= len(m.lines) {
		if strings.TrimSpace(vis) == "" {
			return
		}
		m.lines = append(m.lines, line{kind: "assistant", live: true, at: time.Now()})
		m.liveText = len(m.lines) - 1
	}
	ln := &m.lines[m.liveText]
	ln.text = vis
	ln.out = ""
}

// finishThought folds the streaming thought into its summary line.
func (m *Model) finishThought() {
	if m.liveThought >= 0 && m.liveThought < len(m.lines) {
		ln := &m.lines[m.liveThought]
		ln.done, ln.finished, ln.out = true, time.Now(), ""
	}
	m.liveThought = -1
}

// finalAssistant lands the authoritative reply for this step, replacing
// the live text when there is one.
func (m *Model) finalAssistant(text string) {
	m.finishThought()
	if m.liveText >= 0 && m.liveText < len(m.lines) {
		ln := &m.lines[m.liveText]
		ln.text, ln.live, ln.out = text, false, ""
		m.liveText = -1
		m.liveRaw = ""
		return
	}
	m.lines = append(m.lines, line{kind: "assistant", text: text})
	m.liveRaw = ""
}

// endStep closes a model call. A live line that never got its EvAssistant
// means the final content was empty (tool fences only): drop it. keep
// leaves partial text (the call failed mid-stream).
func (m *Model) endStep(keep bool) {
	m.finishThought()
	if i := m.liveText; i >= 0 && i < len(m.lines) {
		if keep && strings.TrimSpace(m.lines[i].text) != "" {
			m.lines[i].live, m.lines[i].out = false, ""
		} else {
			m.removeLine(i)
		}
	}
	m.liveText = -1
	m.liveRaw = ""
}

func (m *Model) removeLine(i int) {
	if i < 0 || i >= len(m.lines) {
		return
	}
	m.lines = append(m.lines[:i], m.lines[i+1:]...)
	if m.selected > i {
		m.selected--
	} else if m.selected == i {
		m.selected = -1
	}
	if m.liveThought > i {
		m.liveThought--
	}
	m.invalidatePaint()
}

// streamVisible hides Rock tool fences, finished or not, and a trailing
// partial "```tool" opener so raw tool JSON never flashes on screen.
func streamVisible(raw string) string {
	var b strings.Builder
	rest := raw
	for {
		i := strings.Index(rest, "```tool")
		if i < 0 {
			b.WriteString(rest)
			break
		}
		b.WriteString(rest[:i])
		rest = rest[i+len("```tool"):]
		end := strings.Index(rest, "```")
		if end < 0 {
			rest = ""
			break
		}
		rest = rest[end+3:]
	}
	out := b.String()
	const opener = "```tool"
	for n := len(opener) - 1; n > 0; n-- {
		if !strings.HasSuffix(out, opener[:n]) {
			continue
		}
		// A trailing ``` that closes an open generic code block stays.
		if n >= 3 && strings.Count(out[:len(out)-n], "```")%2 == 1 {
			break
		}
		out = out[:len(out)-n]
		break
	}
	return strings.TrimRight(out, " \t\n")
}

// renderThinking draws a thought line: streaming tail, folded summary, or
// the whole thought when expanded.
func (m *Model) renderThinking(ln line, width int, selected, folded bool) string {
	t := m.th
	bodyW := max(8, width-blockPad)
	if !ln.done {
		rows := strings.Split(wrapText(sanitize(strings.TrimSpace(ln.text)), bodyW), "\n")
		if len(rows) > thoughtTailRows {
			rows = rows[len(rows)-thoughtTailRows:]
		}
		head := t.faint.Render("✻ Thinking…")
		body := t.faint.Italic(true).Render(strings.Join(rows, "\n"))
		return m.markBlock(head+"\n"+body, width, selected, false)
	}
	took := ln.finished.Sub(ln.at)
	head := t.faint.Render(fmt.Sprintf("✻ Thought for %s", fmtDuration(took)))
	if folded {
		return m.markBlock(head, width, selected, true)
	}
	body := t.faint.Italic(true).Render(wrapText(sanitize(strings.TrimSpace(ln.text)), bodyW))
	return m.markBlock(head+"\n"+body, width, selected, false)
}

// activityView is the working indicator above the composer: spinner,
// phase, elapsed time and what is running.
func (m *Model) activityView() string {
	t := m.th
	w := max(1, m.geo.innerW)
	phase, detail := "Thinking…", ""
	switch {
	case m.pending != nil:
		phase, detail = "Waiting for you…", m.pending.tool
	case m.stall != "":
		phase, detail = "Waiting…", m.stall
	case m.runningTool() != "":
		phase, detail = "Working…", m.runningTool()
	case m.liveText >= 0:
		phase = "Responding…"
	}
	s := " " + m.spin.View() + " " + t.alarm.Render(phase)
	if !m.turnAt.IsZero() {
		s += t.faint.Render(" " + fmtDuration(time.Since(m.turnAt)))
	}
	if detail != "" {
		s += t.faint.Render(" · ") + t.chrome.Render(sanitize(detail))
	}
	hint := t.faint.Render("esc to interrupt")
	if gap := w - ansi.StringWidth(s) - ansi.StringWidth(hint) - 1; gap >= 2 {
		return s + strings.Repeat(" ", gap) + hint
	}
	return ansi.Truncate(s, w, "…")
}

// runningTool is the name of the tool call still waiting for its result.
func (m *Model) runningTool() string {
	for i := len(m.lines) - 1; i >= 0 && i >= m.turnFrom; i-- {
		if m.toolRunning(i) {
			return m.lines[i].name
		}
	}
	return ""
}
