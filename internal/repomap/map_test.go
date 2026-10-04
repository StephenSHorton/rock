package repomap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildRanksMentionedFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "alpha.go"), []byte("package alpha\nfunc Widget() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "node_modules", "nope.go"), []byte("package nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hits, err := Build(dir, "fix Widget in alpha", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].Path != "alpha.go" {
		t.Fatalf("%#v", hits)
	}
	text := Render(hits)
	if !strings.Contains(text, "Widget") {
		t.Fatal(text)
	}
}
