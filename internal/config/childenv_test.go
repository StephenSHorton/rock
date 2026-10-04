package config

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

func TestStripJevKeys(t *testing.T) {
	got := StripJevKeys([]string{
		"PATH=/usr/bin",
		"JEV_API_KEY=secret-jev",
		"TYPESAFE_API_KEY=secret-typesafe",
		"HOME=/tmp",
		"JEV_API_KEY=",
	})
	want := []string{"PATH=/usr/bin", "HOME=/tmp"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestScrubCmdEnv(t *testing.T) {
	t.Setenv("JEV_API_KEY", "secret-jev")
	t.Setenv("TYPESAFE_API_KEY", "secret-typesafe")
	cmd := exec.Command("true")
	ScrubCmdEnv(cmd)
	for _, e := range cmd.Env {
		k, _, _ := strings.Cut(e, "=")
		if k == "JEV_API_KEY" || k == "TYPESAFE_API_KEY" {
			t.Fatalf("child still has %s", e)
		}
	}
}
