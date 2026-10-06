package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

// editModel is a composer with "one two three" typed and the cursor at the
// end. ctrlHWord picks how ctrl+h is read.
func editModel(t *testing.T, ctrlHWord bool) *Model {
	t.Helper()
	m := sized(t, 100, 24)
	m.ctrlHWord = ctrlHWord
	m.input.KeyMap = composerKeyMap(m.keys.newline, ctrlHWord)
	m.input.Focus()
	m.input.SetValue("one two three")
	m.input.CursorEnd()
	return m
}

// decode runs raw terminal bytes through Bubble Tea's own input decoder, so
// the tests use the exact key a terminal would produce.
func decode(t *testing.T, raw string) tea.KeyPressMsg {
	t.Helper()
	var d uv.EventDecoder
	n, ev := d.Decode([]byte(raw))
	if n != len(raw) {
		t.Fatalf("%q: decoded %d of %d bytes (%T %v)", raw, n, len(raw), ev, ev)
	}
	k, ok := ev.(uv.KeyPressEvent)
	if !ok {
		t.Fatalf("%q: got %T %v, want a key press", raw, ev, ev)
	}
	return tea.KeyPressMsg(k)
}

func press(m *Model, k tea.KeyPressMsg) { m.Update(k) }

func TestCtrlBackspaceDeletesThePreviousWordInEveryEncoding(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string // String() of the decoded key
	}{
		{"Windows Terminal / conhost VT input, GNOME Terminal (BS)", "\x08", "ctrl+h"},
		{"win32 input record (VK_BACK + LEFT_CTRL)", "\x1b[8;14;127;1;8;1_", "ctrl+backspace"},
		{"win32 input record, BS char", "\x1b[8;14;8;1;8;1_", "ctrl+backspace"},
		{"Kitty keyboard protocol", "\x1b[127;5u", "ctrl+backspace"},
		{"xterm modifyOtherKeys", "\x1b[27;5;127~", "ctrl+backspace"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			k := decode(t, c.raw)
			if got := k.String(); got != c.want {
				t.Fatalf("decoded as %q, want %q", got, c.want)
			}
			m := editModel(t, true)
			press(m, k)
			if got := m.input.Value(); got != "one two " {
				t.Fatalf("after %s: %q, want %q", c.want, got, "one two ")
			}
			if m.overlay != noOverlay {
				t.Fatalf("%s opened overlay %d (it used to open help)", c.want, m.overlay)
			}
		})
	}
}

func TestPlainBackspaceStillDeletesOneCharacter(t *testing.T) {
	for _, raw := range []string{"\x7f", "\x1b[8;14;8;1;0;1_", "\x1b[127u"} {
		for _, ctrlHWord := range []bool{true, false} {
			k := decode(t, raw)
			if k.String() != "backspace" {
				t.Fatalf("%q decoded as %q", raw, k.String())
			}
			m := editModel(t, ctrlHWord)
			press(m, k)
			if got := m.input.Value(); got != "one two thre" {
				t.Fatalf("%q (ctrlHWord=%v): %q", raw, ctrlHWord, got)
			}
		}
	}
}

func TestCtrlHIsBackspaceWhenTheTerminalBackspaceSendsBS(t *testing.T) {
	m := editModel(t, false)
	press(m, decode(t, "\x08"))
	if got := m.input.Value(); got != "one two thre" {
		t.Fatalf("ctrl+h in char mode: %q", got)
	}
	if m.overlay != noOverlay {
		t.Fatalf("ctrl+h opened overlay %d", m.overlay)
	}
}

func TestCtrlHModeOverride(t *testing.T) {
	t.Setenv("WT_SESSION", "")
	t.Setenv("ROCK_CTRL_H", "char")
	if ctrlHDeletesWord() {
		t.Fatal("ROCK_CTRL_H=char should make ctrl+h a char delete")
	}
	t.Setenv("ROCK_CTRL_H", "word")
	if !ctrlHDeletesWord() {
		t.Fatal("ROCK_CTRL_H=word should make ctrl+h a word delete")
	}
	t.Setenv("ROCK_CTRL_H", "")
	t.Setenv("WT_SESSION", "1b0f-…")
	if !ctrlHDeletesWord() {
		t.Fatal("Windows Terminal (WT_SESSION) sends Ctrl+Backspace as BS")
	}
}

func TestComposerEditingKeys(t *testing.T) {
	type tc struct {
		name   string
		keys   []string // raw bytes, decoded like a terminal would
		start  int      // cursor column before the keys (-1 = end)
		want   string
		col    int // cursor column after (-1 = don't care)
		wantAs string
	}
	cases := []tc{
		{"ctrl+w", []string{"\x17"}, -1, "one two ", 8, "ctrl+w"},
		{"alt+backspace", []string{"\x1b\x7f"}, -1, "one two ", 8, "alt+backspace"},
		{"ctrl+alt+h", []string{"\x1b\x08"}, -1, "one two ", 8, "ctrl+alt+h"},
		{"ctrl+delete", []string{"\x1b[3;5~"}, 4, "one  three", 4, "ctrl+delete"},
		{"alt+delete", []string{"\x1b[3;3~"}, 4, "one  three", 4, "alt+delete"},
		{"alt+d", []string{"\x1bd"}, 4, "one  three", 4, "alt+d"},
		{"ctrl+left", []string{"\x1b[1;5D"}, -1, "one two three", 8, "ctrl+left"},
		{"ctrl+right", []string{"\x1b[1;5C"}, 0, "one two three", 3, "ctrl+right"},
		{"alt+left", []string{"\x1b[1;3D"}, -1, "one two three", 8, "alt+left"},
		{"alt+right", []string{"\x1b[1;3C"}, 0, "one two three", 3, "alt+right"},
		{"alt+b", []string{"\x1bb"}, -1, "one two three", 8, "alt+b"},
		{"alt+f", []string{"\x1bf"}, 0, "one two three", 3, "alt+f"},
		{"home", []string{"\x1b[H"}, -1, "one two three", 0, "home"},
		{"end", []string{"\x1b[F"}, 0, "one two three", 13, "end"},
		{"ctrl+a", []string{"\x01"}, -1, "one two three", 0, "ctrl+a"},
		{"ctrl+e", []string{"\x05"}, 0, "one two three", 13, "ctrl+e"},
		{"ctrl+u", []string{"\x15"}, 8, "three", 0, "ctrl+u"},
		{"ctrl+k", []string{"\x0b"}, 8, "one two ", 8, "ctrl+k"},
		{"ctrl+d", []string{"\x04"}, 0, "ne two three", 0, "ctrl+d"},
		{"delete", []string{"\x1b[3~"}, 0, "ne two three", 0, "delete"},
		{"ctrl+left twice then ctrl+backspace", []string{"\x1b[1;5D", "\x1b[1;5D", "\x7f\x08"[1:]}, -1, "two three", 0, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := editModel(t, true)
			if c.start >= 0 {
				m.input.SetCursorColumn(c.start)
			}
			for i, raw := range c.keys {
				k := decode(t, raw)
				if i == 0 && c.wantAs != "" && k.String() != c.wantAs {
					t.Fatalf("%q decoded as %q, want %q", raw, k.String(), c.wantAs)
				}
				press(m, k)
			}
			if got := m.input.Value(); got != c.want {
				t.Fatalf("value %q, want %q", got, c.want)
			}
			if c.col >= 0 && m.input.Column() != c.col {
				t.Fatalf("cursor col %d, want %d", m.input.Column(), c.col)
			}
			if m.overlay != noOverlay {
				t.Fatalf("opened overlay %d", m.overlay)
			}
		})
	}
}

func TestNewlineKeysStillInsertALine(t *testing.T) {
	for _, raw := range []string{"\x0a", "\x1b\r", "\x1b[13;2u"} {
		m := editModel(t, true)
		press(m, decode(t, raw))
		if got := m.input.Value(); got != "one two three\n" {
			t.Fatalf("%q: %q", raw, got)
		}
	}
}

func TestCtrlUAndCtrlDEditInTheComposerButScrollTheTranscript(t *testing.T) {
	m := editModel(t, true)
	for i := 0; i < 80; i++ {
		m.lines = append(m.lines, line{kind: "assistant", text: "line"})
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m.vp.GotoBottom()
	off := m.vp.YOffset()
	press(m, decode(t, "\x15")) // ctrl+u in the composer
	if m.input.Value() != "" || m.vp.YOffset() != off {
		t.Fatalf("ctrl+u in composer: value %q offset %d→%d", m.input.Value(), off, m.vp.YOffset())
	}
	m.focusTranscript = true
	m.input.SetValue("draft")
	press(m, decode(t, "\x15"))
	if m.input.Value() != "draft" {
		t.Fatalf("ctrl+u with the transcript focused edited the draft: %q", m.input.Value())
	}
	if off > 0 && m.vp.YOffset() >= off {
		t.Fatalf("ctrl+u with the transcript focused did not scroll: %d→%d", off, m.vp.YOffset())
	}
}

func TestHelpKeysNoLongerIncludeCtrlH(t *testing.T) {
	m := sized(t, 100, 24)
	for _, raw := range []string{"\x18"} { // ctrl+x
		m.overlay = noOverlay
		press(m, decode(t, raw))
		if m.overlay != helpOverlay {
			t.Fatalf("%q should open help", raw)
		}
	}
	for _, k := range m.keys.help.Keys() {
		if k == "ctrl+h" {
			t.Fatal("ctrl+h is still bound to help; it is Ctrl+Backspace on Windows")
		}
	}
}

func TestPickerFilterDeletesWords(t *testing.T) {
	m := sized(t, 100, 24)
	m.input.Focus()
	m.ctrlHWord = true
	m.openSessionsPicker()
	if !m.picker.ownsTyping() {
		t.Fatalf("sessions picker should own typing (kind %d)", m.picker.kind)
	}
	m.picker.filter = "fix the bug"
	press(m, decode(t, "\x08"))
	if m.picker.filter != "fix the " {
		t.Fatalf("ctrl+backspace in picker: %q", m.picker.filter)
	}
	press(m, decode(t, "\x7f"))
	if m.picker.filter != "fix the" {
		t.Fatalf("backspace in picker: %q", m.picker.filter)
	}
	press(m, decode(t, "\x17"))
	if m.picker.filter != "fix " {
		t.Fatalf("ctrl+w in picker: %q", m.picker.filter)
	}
}

func TestHelpOverlayListsEditingKeys(t *testing.T) {
	m := sized(t, 140, 50)
	submit(m, "/help")
	view := flat(strings.Join(screen(m), "\n"))
	for _, want := range []string{"Editing", "ctrl+backspace", "delete previous word", "ctrl+delete", "move by word", "ctrl+u / ctrl+k"} {
		if !strings.Contains(view, want) {
			t.Fatalf("help overlay missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "ctrl+h /") {
		t.Fatalf("help still advertises ctrl+h:\n%s", view)
	}
}
