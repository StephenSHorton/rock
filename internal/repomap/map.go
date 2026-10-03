// Package repomap ranks files against a prompt and skims top-level symbols.
// It is a cheap map, not a language server.
package repomap

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

type Hit struct {
	Path    string
	Symbols []string
	Score   int
}

var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "target": true,
	"dist": true, ".rock": true, ".venv": true, "venv": true,
}

func Build(root, prompt string, limit int) ([]Hit, error) {
	if limit <= 0 {
		limit = 12
	}
	words := tokens(prompt)
	var hits []Hit
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			return nil
		}
		if skipFile(rel) {
			return nil
		}
		score := 0
		low := strings.ToLower(rel)
		for w := range words {
			if strings.Contains(low, w) {
				score += 3
			}
		}
		info, err := d.Info()
		if err != nil || info.Size() > 200_000 {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil || looksBinary(raw) {
			return nil
		}
		syms := symbols(string(raw))
		for _, s := range syms {
			if words[strings.ToLower(s)] {
				score += 2
			}
		}
		if score == 0 && len(hits) > limit*4 {
			return nil
		}
		hits = append(hits, Hit{Path: rel, Symbols: syms, Score: score})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sortHits(hits)
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

func Render(hits []Hit) string {
	var b strings.Builder
	for _, h := range hits {
		b.WriteString(h.Path)
		if len(h.Symbols) > 0 {
			n := h.Symbols
			if len(n) > 6 {
				n = n[:6]
			}
			b.WriteString(" — ")
			b.WriteString(strings.Join(n, ", "))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func tokens(s string) map[string]bool {
	out := map[string]bool{}
	for _, f := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len(f) >= 3 {
			out[f] = true
		}
	}
	return out
}

func skipFile(rel string) bool {
	base := filepath.Base(rel)
	switch strings.ToLower(filepath.Ext(base)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".zip", ".gz", ".pdf", ".woff", ".woff2":
		return true
	}
	return base == "go.sum" || strings.HasSuffix(base, ".lock")
}

func looksBinary(b []byte) bool {
	n := len(b)
	if n > 800 {
		n = 800
	}
	for _, c := range b[:n] {
		if c == 0 {
			return true
		}
	}
	return false
}

func symbols(src string) []string {
	var out []string
	for _, line := range strings.Split(src, "\n") {
		trim := strings.TrimSpace(line)
		name := ""
		switch {
		case strings.HasPrefix(trim, "func "):
			name = identAfter(trim, "func ")
		case strings.HasPrefix(trim, "type "):
			name = identAfter(trim, "type ")
		case strings.HasPrefix(trim, "class "):
			name = identAfter(trim, "class ")
		case strings.HasPrefix(trim, "def "):
			name = identAfter(trim, "def ")
		case strings.HasPrefix(trim, "fn "):
			name = identAfter(trim, "fn ")
		}
		if name != "" {
			out = append(out, name)
		}
		if len(out) >= 12 {
			break
		}
	}
	return out
}

func identAfter(line, prefix string) string {
	rest := strings.TrimPrefix(line, prefix)
	rest = strings.TrimLeft(rest, "*(")
	if i := strings.IndexAny(rest, " (:[{"); i >= 0 {
		rest = rest[:i]
	}
	rest = strings.Trim(rest, " *")
	if rest == "" || strings.Contains(rest, ".") {
		// methods: func (t T) Name
		if prefix == "func " {
			if i := strings.Index(line, ")"); i >= 0 {
				rest = strings.TrimSpace(line[i+1:])
				if j := strings.IndexAny(rest, " ("); j >= 0 {
					rest = rest[:j]
				}
			}
		}
	}
	if rest == "" || strings.ContainsAny(rest, "{}") {
		return ""
	}
	return rest
}

func sortHits(hits []Hit) {
	for i := 1; i < len(hits); i++ {
		j := i
		for j > 0 && hits[j].Score > hits[j-1].Score {
			hits[j], hits[j-1] = hits[j-1], hits[j]
			j--
		}
	}
}
