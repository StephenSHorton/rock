package tui

import (
	"os"
	"runtime"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
)

// Composer editing keys, modeled on Grok Build's prompt editor
// (xai-ratatui-textarea editor_keys.rs): Ctrl/Alt+Backspace and Ctrl+W
// delete the previous word, Ctrl/Alt+Delete and Alt+D the next one,
// Ctrl/Alt+Left/Right move by word, Ctrl+U/K kill to the line start/end.
//
// Ctrl+Backspace has no single encoding:
//   - Kitty keyboard, xterm modifyOtherKeys and Windows console input
//     records report Backspace with the Ctrl modifier: "ctrl+backspace".
//   - Windows Terminal and conhost in VT input mode (what Bubble Tea uses on
//     Windows), and many Unix terminals, send a bare BS (0x08) while plain
//     Backspace is DEL (0x7f). Bubble Tea decodes 0x08 as "ctrl+h".
//
// So ctrl+h deletes a word unless the terminal's own Backspace sends ^H
// (the tty erase character is 0x08), in which case ctrl+h is plain
// Backspace and must delete one character. ROCK_CTRL_H=word|char forces it.

// ctrlHDeletesWord reports whether a ctrl+h key press should delete a word.
func ctrlHDeletesWord() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ROCK_CTRL_H"))) {
	case "word":
		return true
	case "char", "backspace":
		return false
	}
	if runtime.GOOS == "windows" || os.Getenv("WT_SESSION") != "" {
		return true
	}
	if erase, ok := ttyErase(); ok && erase == 0x08 {
		return false
	}
	return true
}

func bind(help, desc string, keys ...string) key.Binding {
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(help, desc))
}

// composerKeyMap is the textarea key map for the composer. newline is
// Rock's newline binding; ctrlHWord picks what ctrl+h does (see above).
func composerKeyMap(newline key.Binding, ctrlHWord bool) textarea.KeyMap {
	km := textarea.DefaultKeyMap()
	km.InsertNewline = newline

	wordBack := []string{"ctrl+backspace", "alt+backspace", "ctrl+w", "ctrl+alt+h", "ctrl+alt+backspace"}
	charBack := []string{"backspace", "shift+backspace"}
	if ctrlHWord {
		wordBack = append(wordBack, "ctrl+h")
	} else {
		charBack = append(charBack, "ctrl+h")
	}
	km.DeleteWordBackward = bind("ctrl+backspace", "delete previous word", wordBack...)
	km.DeleteCharacterBackward = bind("backspace", "delete character", charBack...)
	km.DeleteWordForward = bind("ctrl+delete", "delete next word", "ctrl+delete", "alt+delete", "alt+d")
	km.DeleteCharacterForward = bind("delete", "delete character forward", "delete", "shift+delete", "ctrl+d")
	km.WordBackward = bind("ctrl+left", "word left", "ctrl+left", "alt+left", "alt+b")
	km.WordForward = bind("ctrl+right", "word right", "ctrl+right", "alt+right", "alt+f")
	km.LineStart = bind("home", "line start", "home", "ctrl+a")
	km.LineEnd = bind("end", "line end", "end", "ctrl+e")
	km.DeleteBeforeCursor = bind("ctrl+u", "delete to line start", "ctrl+u")
	km.DeleteAfterCursor = bind("ctrl+k", "delete to line end", "ctrl+k")
	return km
}

// pickerEdit maps a key to a filter edit for pickers that own typing.
// It returns "word", "char" or "".
func pickerEdit(k string, ctrlHWord bool) string {
	switch k {
	case "backspace", "shift+backspace":
		return "char"
	case "ctrl+backspace", "alt+backspace", "ctrl+w", "ctrl+alt+h", "ctrl+alt+backspace":
		return "word"
	case "ctrl+h":
		if ctrlHWord {
			return "word"
		}
		return "char"
	}
	return ""
}

// editingHelp is the editing section of the help overlay.
func editingHelp() [][2]string {
	return [][2]string{
		{"ctrl+backspace / ctrl+w / alt+backspace", "delete previous word"},
		{"ctrl+delete / alt+d", "delete next word"},
		{"ctrl+←/→ / alt+←/→ (alt+b/f)", "move by word"},
		{"home / end · ctrl+a / ctrl+e", "line start · line end"},
		{"ctrl+u / ctrl+k", "delete to line start · line end"},
		{"shift+enter / ctrl+j / alt+enter", "new line"},
	}
}
