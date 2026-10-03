package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectAllowWaitsOnTrust(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ROCK_CONFIG", filepath.Join(t.TempDir(), "none.toml"))
	if err := os.MkdirAll(filepath.Join(dir, ".rock"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "mode = \"yolo\"\n\n[permissions]\nallow = [\"shell\"]\ndeny = [\"shell:rm *\"]\n"
	if err := os.WriteFile(filepath.Join(dir, ".rock", "config.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Trusted {
		t.Fatal("trusted")
	}
	if len(got.Ignored) != 1 || got.Ignored[0] != "shell" {
		t.Fatalf("ignored %#v", got.Ignored)
	}
	foundDeny := false
	for _, d := range got.Policy.Deny {
		if d == "shell:rm *" {
			foundDeny = true
		}
	}
	if !foundDeny {
		t.Fatalf("deny %#v", got.Policy.Deny)
	}
	if err := os.WriteFile(filepath.Join(dir, ".rock", "trusted"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = Load(dir)
	if err != nil || !got.Trusted {
		t.Fatal(err, got.Trusted)
	}
	found := false
	for _, a := range got.Policy.Allow {
		if a == "shell" {
			found = true
		}
	}
	if !found {
		t.Fatalf("allow %#v", got.Policy.Allow)
	}
}
