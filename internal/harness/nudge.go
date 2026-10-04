package harness

// Soft validation hints. Text only — the agent decides whether to call
// ask_jev. Nothing here POSTs to Jev.

const (
	// DefaultNudgeEvery is the product default: hint on the first
	// qualifying event, then every Nth after that. Zero NudgeEvery
	// resolves to this so existing Options values stay on.
	DefaultNudgeEvery = 2

	nudgeEditHint = "Hint: call ask_jev if this was a fix — is the failure type resolved? is the change too risky or too broad? You decide."

	nudgeShellHint = "Hint: before more risky shell, call ask_jev — is this too risky or too broad? Hard gates still block destructive commands. You decide."
)

type nudgeKind int

const (
	nudgeNone nudgeKind = iota
	nudgeEdit
	nudgeShell
)

func resolveNudgeEvery(n int) int {
	if n < 0 {
		return -1
	}
	if n == 0 {
		return DefaultNudgeEvery
	}
	return n
}

func nudgeFor(name string, err error) nudgeKind {
	switch name {
	case "edit_file", "write_file":
		if err != nil {
			return nudgeNone
		}
		return nudgeEdit
	case "shell":
		return nudgeShell
	default:
		return nudgeNone
	}
}

func hintFor(k nudgeKind) string {
	switch k {
	case nudgeEdit:
		return nudgeEditHint
	case nudgeShell:
		return nudgeShellHint
	default:
		return ""
	}
}

// takeNudge rate-limits a qualifying event. The first event in a slot
// emits; the next (every-1) are silent. Negative every disables.
func (h *Harness) takeNudge(kind nudgeKind) string {
	if kind == nudgeNone {
		return ""
	}
	every := resolveNudgeEvery(h.NudgeEvery)
	if every < 0 {
		return ""
	}
	h.nudgeCount++
	if (h.nudgeCount-1)%every != 0 {
		return ""
	}
	return hintFor(kind)
}

func appendHint(text, hint string) string {
	if hint == "" {
		return text
	}
	if text == "" {
		return hint
	}
	return text + "\n\n" + hint
}
