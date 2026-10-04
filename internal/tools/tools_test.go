package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditAndEscape(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	set := New(Env{Root: dir})
	out, err := set.Run(context.Background(), "edit_file", `{"path":"a.txt","old":"world","new":"rock"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Mutates {
		t.Fatal("expected mutate")
	}
	got, _ := os.ReadFile(filepath.Join(dir, "a.txt"))
	if string(got) != "hello rock" {
		t.Fatal(string(got))
	}
	if _, err := set.Run(context.Background(), "read_file", `{"path":"../secret"}`); err == nil {
		t.Fatal("expected escape error")
	}
}

func TestGrepAndGlob(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.go"), []byte("package note\nfunc FindMe() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	set := New(Env{Root: dir})
	out, err := set.Run(context.Background(), "grep", `{"pattern":"FindMe"}`)
	if err != nil || !strings.Contains(out.Output, "FindMe") {
		t.Fatalf("%v %s", err, out.Output)
	}
	out, err = set.Run(context.Background(), "glob", `{"pattern":"*.go"}`)
	if err != nil || !strings.Contains(out.Output, "note.go") {
		t.Fatalf("%v %s", err, out.Output)
	}
}

func TestShell(t *testing.T) {
	dir := t.TempDir()
	set := New(Env{Root: dir})
	out, err := set.Run(context.Background(), "shell", `{"command":"printf hi"}`)
	if err != nil || strings.TrimSpace(out.Output) != "hi" {
		t.Fatalf("%v %q", err, out.Output)
	}
}
