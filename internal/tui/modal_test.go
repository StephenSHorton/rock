package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/StephenSHorton/rock/internal/perms"
)

func TestPickerOpensNavigatesSelectsAndCloses(t *testing.T) {
	p := picker{}
	p.set(modePicker, "Mode", []rowItem{
		{title: "default", desc: "asks", id: "default"},
		{title: "plan", desc: "blocked", id: "plan"},
		{title: "yolo", desc: "skip asks", id: "yolo"},
	})
	if !p.open() || p.kind != modePicker || p.selectedMust(t).id != "default" {
		t.Fatalf("open: %+v", p)
	}
	p.move(1)
	if p.selectedMust(t).id != "plan" {
		t.Fatal(p.selectedMust(t).id)
	}
	p.move(1)
	if p.selectedMust(t).id != "yolo" {
		t.Fatal(p.selectedMust(t).id)
	}
	p.move(1)
	if p.selectedMust(t).id != "default" {
		t.Fatal("move should wrap")
	}
	p.typeFilter("yo")
	if got := titles(p.matches()); len(got) != 1 || got[0] != "yolo" {
		t.Fatalf("filter: %v", got)
	}
	if p.selectedMust(t).id != "yolo" {
		t.Fatal("filter should clamp selection")
	}
	p.backspace()
	p.backspace()
	if len(p.matches()) != 3 {
		t.Fatalf("backspace should restore: %v", titles(p.matches()))
	}
	p.close()
	if p.open() {
		t.Fatal("close")
	}
}

func (p picker) selectedMust(t *testing.T) rowItem {
	t.Helper()
	it, ok := p.selected()
	if !ok {
		t.Fatal("no selection")
	}
	return it
}

func TestAtTokenAndWorkspaceFiles(t *testing.T) {
	start, q, ok := atToken("see @readme")
	if !ok || start != 4 || q != "readme" {
		t.Fatalf("atToken %d %q %v", start, q, ok)
	}
	if _, _, ok := atToken("email@x.com"); ok {
		t.Fatal("mid-word @ is not a mention")
	}
	if _, _, ok := atToken("@foo bar"); ok {
		t.Fatal("space ends the token")
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "keep.go"), []byte("package keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("ref"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".hidden"), []byte("no"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := titles(listWorkspaceFiles(root, "keep", 10))
	if len(got) != 1 || got[0] != "keep.go" {
		t.Fatalf("files: %v", got)
	}
	for _, it := range listWorkspaceFiles(root, "", 20) {
		if strings.Contains(it.title, ".git") || strings.HasPrefix(filepath.Base(it.title), ".") {
			t.Fatalf("hidden leaked: %+v", it)
		}
	}
}

func TestComposerFrameAndChipState(t *testing.T) {
	m := sized(t, 100, 24)
	m.deps.FastModel = "gpt-4o-mini"
	m.deps.Mode = perms.ModeDefault
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	rows := screen(m)
	g := m.geo
	band := strings.Join(rows[g.composerY():g.statusY()], "\n")
	if !strings.Contains(band, "╭") || !strings.Contains(band, "╰") || !strings.Contains(band, "❯") {
		t.Fatalf("framed composer missing:\n%s", band)
	}
	if !strings.Contains(band, "Ask Rock") || !strings.Contains(band, "/ for commands") {
		t.Fatalf("quiet placeholder missing:\n%s", band)
	}
	if !strings.Contains(band, "gpt-4o-mini") || !strings.Contains(band, "default") {
		t.Fatalf("chips missing:\n%s", band)
	}
	if m.geo.frameRows != 2 || m.geo.infoRows != 0 {
		t.Fatalf("chips live on the frame, not an extra info row: %+v", m.geo)
	}
	if len(m.chips) != 2 || m.chips[0].id != "model" || m.chips[1].id != "mode" {
		t.Fatalf("chip hits: %+v", m.chips)
	}

	m.openModelPicker()
	if m.picker.kind != modelPicker || m.input.Focused() {
		t.Fatal("model chip should steal focus")
	}
	view := flat(strings.Join(screen(m), "\n"))
	if !strings.Contains(view, "ChatGPT") || !strings.Contains(view, "SuperGrok") {
		t.Fatalf("model modal:\n%s", view)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.picker.open() || !m.input.Focused() {
		t.Fatal("esc should close the model modal")
	}

	m.openModePicker()
	if m.picker.kind != modePicker {
		t.Fatal(m.picker.kind)
	}
	m.Update(tea.KeyPressMsg{Text: "down"})
	if m.picker.selectedMust(t).id != "plan" {
		t.Fatal(m.picker.selectedMust(t).id)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	if m.deps.Mode != perms.ModePlan || m.picker.open() {
		t.Fatalf("mode select: mode=%s picker=%d", m.deps.Mode, m.picker.kind)
	}
	band = strings.Join(screen(m)[m.geo.composerY():m.geo.statusY()], "\n")
	if !strings.Contains(band, "plan") {
		t.Fatalf("mode chip should follow selection:\n%s", band)
	}
}

func TestSessionsAndAtFilePickers(t *testing.T) {
	m := sized(t, 100, 24)
	m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if m.picker.kind != sessionsPicker || m.input.Focused() {
		t.Fatalf("ctrl+s should open sessions: kind=%d focused=%v", m.picker.kind, m.input.Focused())
	}
	view := flat(strings.Join(screen(m), "\n"))
	if !strings.Contains(view, "Sessions") || !strings.Contains(view, "demo") {
		t.Fatalf("sessions modal:\n%s", view)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.picker.open() {
		t.Fatal("esc should close sessions")
	}

	root := m.deps.CWD
	if err := os.WriteFile(filepath.Join(root, "keep.go"), []byte("package keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skip.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.input.SetValue("@ke")
	m.syncPicker()
	if m.picker.kind != filesPicker {
		t.Fatalf("@ should open files: %d", m.picker.kind)
	}
	got := titles(m.picker.matches())
	if !contains(got, "keep.go") || contains(got, "skip.txt") {
		t.Fatalf("@ke filter: %v", got)
	}
	view = flat(strings.Join(screen(m), "\n"))
	if !strings.Contains(view, "keep.go") {
		t.Fatalf("file modal:\n%s", view)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	if m.input.Value() != "@keep.go " || m.picker.open() {
		t.Fatalf("select file: value=%q picker=%d", m.input.Value(), m.picker.kind)
	}
}

func TestChipClickOpensModal(t *testing.T) {
	m := sized(t, 100, 24)
	_ = m.View()
	if len(m.chips) < 2 {
		t.Fatal(m.chips)
	}
	model := m.chips[0]
	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: model.x, Y: m.chipY})
	if m.picker.kind != modelPicker {
		t.Fatalf("click model chip: kind=%d x=%d y=%d chips=%+v", m.picker.kind, model.x, m.chipY, m.chips)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	mode := m.chips[1]
	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: mode.x, Y: m.chipY})
	if m.picker.kind != modePicker {
		t.Fatalf("click mode chip: kind=%d", m.picker.kind)
	}
}
