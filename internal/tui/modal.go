package tui

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

// pickerCap is Grok's visible slash-dropdown budget: enough room to scan,
// not a full-screen list.
const pickerCap = 8

// pickerKind is a composer-attached modal. Help, permissions, agents, and
// the palette stay full-body overlays; these float above the prompt.
type pickerKind int

const (
	noPicker pickerKind = iota
	slashPicker
	modelPicker
	modePicker
	sessionsPicker
	filesPicker
)

type chipHit struct {
	id string
	x  int
	w  int
}

// picker is the shared modal/overlay used by the composer chips and the
// / @ triggers. One list, one selection, optional filter-as-you-type.
type picker struct {
	kind   pickerKind
	title  string
	items  []rowItem
	sel    int
	filter string
}

func (p *picker) open() bool { return p != nil && p.kind != noPicker }

func (p *picker) close() {
	if p == nil {
		return
	}
	*p = picker{}
}

func (p *picker) set(kind pickerKind, title string, items []rowItem) {
	p.kind = kind
	p.title = title
	p.items = items
	p.sel = 0
	p.filter = ""
	p.clamp()
}

func (p *picker) matches() []rowItem {
	if p == nil {
		return nil
	}
	q := strings.ToLower(strings.TrimSpace(p.filter))
	if q == "" {
		return p.items
	}
	var prefix, rest []rowItem
	for _, it := range p.items {
		name := strings.ToLower(it.title)
		desc := strings.ToLower(it.desc)
		bare := strings.TrimPrefix(name, "/")
		switch {
		case strings.HasPrefix(name, q) || strings.HasPrefix(bare, q):
			prefix = append(prefix, it)
		case strings.Contains(name, q) || strings.Contains(desc, q):
			rest = append(rest, it)
		}
	}
	return append(prefix, rest...)
}

func (p *picker) clamp() {
	n := len(p.matches())
	switch {
	case n == 0:
		p.sel = 0
	case p.sel >= n:
		p.sel = n - 1
	case p.sel < 0:
		p.sel = 0
	}
}

func (p *picker) move(delta int) {
	n := len(p.matches())
	if n == 0 {
		return
	}
	p.sel = (p.sel + delta%n + n) % n
}

func (p *picker) selected() (rowItem, bool) {
	items := p.matches()
	if p.sel < 0 || p.sel >= len(items) {
		return rowItem{}, false
	}
	return items[p.sel], true
}

func (p *picker) typeFilter(s string) {
	if s == "" {
		return
	}
	p.filter += s
	p.clamp()
}

func (p *picker) backspace() {
	if p.filter == "" {
		return
	}
	r := []rune(p.filter)
	p.filter = string(r[:len(r)-1])
	p.clamp()
}

func (p *picker) ownsTyping() bool {
	switch p.kind {
	case modelPicker, modePicker, sessionsPicker:
		return true
	}
	return false
}

// view paints the floating modal. Slash and @ stay title-less, like Grok's
// dropdown. Model, mode, and sessions put a short title on the top border.
func (p picker) view(t *theme, w int) string {
	items := p.matches()
	if p.kind == noPicker {
		return ""
	}
	if len(items) == 0 && p.title == "" {
		return ""
	}
	from := 0
	if p.sel >= pickerCap {
		from = p.sel - pickerCap + 1
	}
	to := min(len(items), from+pickerCap)
	var rows []string
	if p.filter != "" && p.ownsTyping() {
		rows = append(rows, t.faint.Render(clip(p.filter, max(1, w-4))))
	}
	if len(items) == 0 {
		rows = append(rows, t.faint.Render("no matches"))
	}
	for i := from; i < to; i++ {
		it := items[i]
		mark, titleSt := "  ", t.plain
		if i == p.sel {
			mark, titleSt = t.accent.Render("▸ "), t.strong
		}
		row := mark + titleSt.Render(it.title)
		if it.desc != "" {
			row += "  " + t.faint.Render(it.desc)
		}
		rows = append(rows, t.bar.Width(max(1, w-4)).Render(ansi.Truncate(row, max(1, w-4), "…")))
	}
	body := strings.Join(rows, "\n")
	box := t.boxFocus
	innerW := max(1, w-box.GetHorizontalFrameSize())
	innerH := max(1, lipglossHeight(body))
	if p.title != "" {
		top := t.accentBold.Render(clip(p.title, innerW))
		body = top + "\n" + body
		innerH++
	}
	return boxed(box, w, innerH+box.GetVerticalFrameSize(), body)
}

func lipglossHeight(s string) int {
	if s == "" {
		return 1
	}
	return strings.Count(s, "\n") + 1
}

// atToken is the trailing @mention in the composer, if the cursor is still
// inside that token. Grok opens file search from a leading @ on a word.
func atToken(v string) (start int, query string, ok bool) {
	start = strings.LastIndex(v, "@")
	if start < 0 {
		return 0, "", false
	}
	if start > 0 {
		r := rune(v[start-1])
		if !unicode.IsSpace(r) && r != '\n' && r != '\t' {
			return 0, "", false
		}
	}
	rest := v[start+1:]
	if strings.ContainsAny(rest, " \t\n") {
		return 0, "", false
	}
	return start, rest, true
}

var skipFileDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "target": true,
	"dist": true, ".rock": true, ".venv": true, "venv": true,
}

// listWorkspaceFiles is the @ picker source. Hidden names stay out (Grok's
// @! toggle is a skip). .git / node_modules / vendor match the glob tool.
func listWorkspaceFiles(root, query string, limit int) []rowItem {
	if root == "" || limit <= 0 {
		return nil
	}
	q := strings.ToLower(query)
	var items []rowItem
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if skipFileDirs[name] || strings.HasPrefix(name, ".") {
				if path != root {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if strings.HasPrefix(name, ".") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			return nil
		}
		if q != "" && !strings.Contains(strings.ToLower(rel), q) {
			return nil
		}
		items = append(items, rowItem{title: rel, desc: filepath.Dir(rel), id: rel})
		if len(items) >= limit {
			return os.ErrExist
		}
		return nil
	})
	return items
}
