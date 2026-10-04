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

func TestNudgeInterval(t *testing.T) {
	if Default().Jev.NudgeInterval() != 2 {
		t.Fatalf("default %d", Default().Jev.NudgeInterval())
	}
	off := false
	if (Jev{Nudge: &off}).NudgeInterval() != -1 {
		t.Fatal("nudge=false")
	}
	if (Jev{NudgeEvery: -1}).NudgeInterval() != -1 {
		t.Fatal("nudge_every=-1")
	}
	if (Jev{NudgeEvery: 4}).NudgeInterval() != 4 {
		t.Fatal("nudge_every=4")
	}
	on := true
	if (Jev{Nudge: &on, NudgeEvery: 3}).NudgeInterval() != 3 {
		t.Fatal("nudge=true keeps every")
	}
}

func TestNudgeConfigMerge(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ROCK_CONFIG", filepath.Join(t.TempDir(), "none.toml"))
	if err := os.MkdirAll(filepath.Join(dir, ".rock"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".rock", "trusted"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".rock", "config.toml"), []byte("[jev]\nnudge = false\nnudge_every = 5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.File.Jev.NudgeInterval() != -1 {
		t.Fatalf("false wins over every: %d", got.File.Jev.NudgeInterval())
	}

	if err := os.WriteFile(filepath.Join(dir, ".rock", "config.toml"), []byte("[jev]\nnudge_every = 5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.File.Jev.NudgeInterval() != 5 {
		t.Fatalf("every 5: %d", got.File.Jev.NudgeInterval())
	}
}

func TestTriageFilterConfig(t *testing.T) {
	if !Default().Jev.TriageOn() || !Default().Jev.FilterOn() || Default().Jev.ClipSize() != 1500 {
		t.Fatal("defaults")
	}
	off := false
	if (Jev{Triage: &off}).TriageOn() || (Jev{Filter: &off}).FilterOn() {
		t.Fatal("explicit off")
	}
	if (Jev{ClipBytes: 400}).ClipSize() != 400 {
		t.Fatal("clip")
	}

	dir := t.TempDir()
	t.Setenv("ROCK_CONFIG", filepath.Join(t.TempDir(), "none.toml"))
	if err := os.MkdirAll(filepath.Join(dir, ".rock"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".rock", "trusted"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".rock", "config.toml"), []byte("[jev]\ntriage = false\nfilter = false\nclip_bytes = 800\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.File.Jev.TriageOn() || got.File.Jev.FilterOn() || got.File.Jev.ClipSize() != 800 {
		t.Fatalf("%#v", got.File.Jev)
	}
}
