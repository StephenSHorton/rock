package repomap

import (
	"fmt"
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

func TestBuildStopsAtBudgets(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 50; i++ {
		dir := filepath.Join(root, fmt.Sprintf("d%02d", i))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for j := 0; j < 20; j++ {
			_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%02d.go", j)), []byte("package x\nfunc Hello() {}\n"), 0o644)
		}
	}
	oldV, oldR := MaxVisit, MaxRead
	defer func() { MaxVisit, MaxRead = oldV, oldR }()
	MaxRead = 30
	_, st, err := BuildStats(root, "hello", 8)
	if err != nil || st.Stopped != "read cap" || st.Read != 30 {
		t.Fatalf("read cap: %+v %v", st, err)
	}
	MaxRead, MaxVisit = 10000, 100
	_, st, _ = BuildStats(root, "hello", 8)
	if st.Stopped != "visit cap" || st.Visited != 101 {
		t.Fatalf("visit cap: %+v", st)
	}
}

func TestBuildSkipsHomeAndRoot(t *testing.T) {
	home := t.TempDir()
	_ = os.WriteFile(filepath.Join(home, "a.go"), []byte("package a\n"), 0o644)
	old := homeDir
	homeDir = func() (string, error) { return home, nil }
	defer func() { homeDir = old }()
	hits, st, err := BuildStats(home, "a", 8)
	if err != nil || len(hits) != 0 || st.Stopped != "home dir" || st.Visited != 0 {
		t.Fatalf("home: %v %+v %v", hits, st, err)
	}
	if _, st, _ := BuildStats(string(filepath.Separator), "a", 8); st.Stopped != "volume root" {
		t.Fatalf("root: %+v", st)
	}
	// A project under home is still mapped.
	proj := filepath.Join(home, "proj")
	_ = os.MkdirAll(proj, 0o755)
	_ = os.WriteFile(filepath.Join(proj, "main.go"), []byte("package main\nfunc Hey() {}\n"), 0o644)
	if hits, _, _ := BuildStats(proj, "hey", 8); len(hits) != 1 {
		t.Fatalf("project hits %v", hits)
	}
}

func TestBuildSkipsProfileDirs(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"AppData/Local/x", "OneDrive/docs", "src"} {
		_ = os.MkdirAll(filepath.Join(root, d), 0o755)
		_ = os.WriteFile(filepath.Join(root, d, "f.go"), []byte("package f\n"), 0o644)
	}
	hits, _, _ := BuildStats(root, "f", 8)
	if len(hits) != 1 || !strings.HasPrefix(filepath.ToSlash(hits[0].Path), "src/") {
		t.Fatalf("hits %v", hits)
	}
}
