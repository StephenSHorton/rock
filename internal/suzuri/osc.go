// Package suzuri speaks the host sequences Suzuri already understands.
// OSC 7880 asks the host to split the emitting pane and launch an allowlisted
// binary. Rock does not open its own window.
package suzuri

import "strings"

type Fork struct {
	NewSession bool
	SessionID  string
	Bin        string
	Prompt     string
	Title      string
	CWD        string
}

// Sequence is ESC ] 7880 ; … BEL. Values are percent-encoded so ';' and '='
// inside a field cannot break the host parser.
func Sequence(f Fork) string {
	var b strings.Builder
	b.WriteString("\x1b]7880;")
	if f.NewSession {
		b.WriteString("new=1;")
	} else {
		b.WriteString("fork=1;")
	}
	write := func(k, v string) {
		if v == "" {
			return
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(pct(v))
		b.WriteByte(';')
	}
	write("session", f.SessionID)
	write("bin", f.Bin)
	write("cwd", f.CWD)
	write("prompt", f.Prompt)
	write("title", f.Title)
	b.WriteString("brand=rock")
	b.WriteByte('\a')
	return b.String()
}

func pct(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0x0F])
	}
	return b.String()
}
