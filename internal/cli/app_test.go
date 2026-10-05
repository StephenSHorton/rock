package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StephenSHorton/rock/internal/grokcli"
	"github.com/StephenSHorton/rock/internal/perms"
	"github.com/StephenSHorton/rock/internal/provider"
	"github.com/StephenSHorton/rock/internal/siwc"
)

func TestHeadlessOfflineAndInspect(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	t.Setenv("ROCK_CONFIG", t.TempDir()+"/config.toml")
	t.Setenv("ROCK_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("JEV_API_KEY", "")
	t.Setenv("TYPESAFE_API_KEY", "")
	dir := t.TempDir()
	app, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	var inspect bytes.Buffer
	app.Inspect(&inspect)
	text := inspect.String()
	for _, want := range []string{"provider: offline", "auth: offline_model", "no model · /provider", "grok auto:", "jev: offline", "jev.nudge: every 2", "jev.triage: on", "jev.filter: on", "jev.clip_bytes: 1500", "Permissions are not a sandbox", "redirections are not inspected"} {
		if !strings.Contains(text, want) {
			t.Fatalf("inspect missing %q\n%s", want, text)
		}
	}
	sess, err := app.OpenSession("", true, "hello")
	if err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if err := app.Headless(context.Background(), sess, "Say hello", "text", &out, &errb, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "offline provider") {
		t.Fatalf("stdout %q stderr %q", out.String(), errb.String())
	}
	if !strings.Contains(errb.String(), "done end_turn") {
		t.Fatal(errb.String())
	}
}

func TestInspectNamesSIWCAuthClass(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	t.Setenv("ROCK_CONFIG", t.TempDir()+"/config.toml")
	t.Setenv("ROCK_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("JEV_API_KEY", "")
	t.Setenv("TYPESAFE_API_KEY", "")
	host, err := siwc.HostID()
	if err != nil {
		t.Fatal(err)
	}
	if err := siwc.OpenStore().Save(siwc.Record{
		ClientID:       "oaiapp_inspect",
		ExtAgentHostID: host,
		AccessToken:    "tok",
		Scopes:         []string{siwc.PlanUsageScope},
		Email:          "dev@example.com",
	}); err != nil {
		t.Fatal(err)
	}
	app, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	var buf bytes.Buffer
	app.Inspect(&buf)
	text := buf.String()
	for _, want := range []string{"provider: chatgpt", "auth: siwc", "Jev is a separate required key"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, "auth: api_key") {
		t.Fatal(text)
	}
}

func TestOfflineModelAuthDoesNotTouchJev(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	t.Setenv("ROCK_CONFIG", t.TempDir()+"/config.toml")
	t.Setenv("ROCK_API_KEY", "sk-test")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("JEV_API_KEY", "jv_live_test")
	t.Setenv("TYPESAFE_API_KEY", "")
	app, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if app.Gates.Mode() != "live" {
		t.Fatalf("jev before: %s", app.Gates.Mode())
	}
	if err := app.SetAuth(siwc.AuthOfflineModel); err != nil {
		t.Fatal(err)
	}
	if app.Auth != siwc.AuthOfflineModel || app.Provider.Name() != "offline" {
		t.Fatalf("auth %s provider %s", app.Auth, app.Provider.Name())
	}
	if app.Gates.Mode() != "live" {
		t.Fatal("offline model must not force Jev offline")
	}
	if app.Loaded.File.Jev.AllowDestructive {
		t.Fatal("offline model must not relax Jev gates")
	}
	var buf bytes.Buffer
	app.Inspect(&buf)
	if !strings.Contains(buf.String(), "auth: offline_model") || !strings.Contains(buf.String(), "jev: live") {
		t.Fatal(buf.String())
	}
	if strings.Contains(buf.String(), "jev key: unset") {
		t.Fatal("offline model must not clear the Jev key")
	}
}

func TestInspectGrokCLIAuthClass(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	t.Setenv("ROCK_CONFIG", t.TempDir()+"/config.toml")
	t.Setenv("ROCK_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("JEV_API_KEY", "jv_live_test")
	t.Setenv("TYPESAFE_API_KEY", "")
	t.Setenv("ROCK_GROK_BIN", "")
	t.Setenv("PATH", t.TempDir())
	app, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if err := app.SetAuth(grokcli.AuthClass); err != nil {
		t.Fatal(err)
	}
	if app.Gates.Mode() != "live" {
		t.Fatal("grok-cli must not force Jev offline")
	}
	var buf bytes.Buffer
	app.Inspect(&buf)
	text := buf.String()
	for _, want := range []string{
		"auth: grok-cli (subscription via official binary)",
		"https://x.ai/cli",
		"grok login",
		"jev: live",
		"Rock runs tools and Jev",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q\n%s", want, text)
		}
	}
}

func TestYoloStillBlocksDestructiveShell(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	t.Setenv("ROCK_CONFIG", t.TempDir()+"/config.toml")
	t.Setenv("ROCK_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	dir := t.TempDir()
	app, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.ApplyFlags("", true)
	app.Provider = &provider.Script{Replies: []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "1", Name: "shell", Arguments: `{"command":"rm -rf /tmp/nope"}`}}},
		{Role: provider.RoleAssistant, Content: "stopped"},
	}}
	sess, err := app.OpenSession("", true, "risk")
	if err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if err := app.Headless(context.Background(), sess, "clean the disk", "streaming-json", &out, &errb, func(string, string) perms.Decision {
		t.Fatal("ask should not run under yolo")
		return perms.Allow
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "destructive-action gate") && !strings.Contains(out.String(), `"block":true`) {
		t.Fatal(out.String())
	}
}

func TestForkSequenceNamesRock(t *testing.T) {
	seq, err := ForkSequence(true, "abc", "try this", "review", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seq, "\x1b]7880;") || !strings.Contains(seq, "brand=rock") || !strings.Contains(seq, "new=1;") || !strings.Contains(seq, "session=") {
		t.Fatal(seq)
	}
	if !strings.HasSuffix(seq, "\a") {
		t.Fatal("missing BEL")
	}
}

func TestAutoDetectsSignedInGrok(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	t.Setenv("ROCK_CONFIG", t.TempDir()+"/config.toml")
	t.Setenv("ROCK_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("JEV_API_KEY", "")
	t.Setenv("TYPESAFE_API_KEY", "")
	grokcli.ClearProbeCache()

	dir := t.TempDir()
	bin := filepath.Join(dir, "grok")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ROCK_GROK_BIN", bin)
	// Seed cache as signed-in so Open does not need a real ACP child.
	// ProbeSignedInCached reads this before dialing.
	home := os.Getenv("ROCK_HOME")
	raw := []byte(`{"bin":"` + bin + `","signed_in":true,"detail":"official grok binary (signed in)","checked":"2099-01-01T00:00:00Z"}`)
	if err := os.WriteFile(filepath.Join(home, "grok-probe.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	app, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if app.Auth != grokcli.AuthClass || !app.GrokAuto {
		t.Fatalf("auth=%q auto=%v provider=%s", app.Auth, app.GrokAuto, app.Provider.Name())
	}
	var buf bytes.Buffer
	app.Inspect(&buf)
	if !strings.Contains(buf.String(), "grok auto: yes") {
		t.Fatal(buf.String())
	}
}

func TestExplicitAuthBeatsGrokAuto(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	t.Setenv("ROCK_CONFIG", t.TempDir()+"/config.toml")
	t.Setenv("ROCK_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	grokcli.ClearProbeCache()
	app, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if err := app.SetAuth(siwc.AuthOfflineModel); err != nil {
		t.Fatal(err)
	}
	if app.GrokAuto || app.Auth != siwc.AuthOfflineModel {
		t.Fatalf("auth=%q auto=%v", app.Auth, app.GrokAuto)
	}
}
