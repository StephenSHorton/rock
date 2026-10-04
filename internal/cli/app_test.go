package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/StephenSHorton/rock/internal/perms"
	"github.com/StephenSHorton/rock/internal/provider"
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
	for _, want := range []string{"provider: offline", "jev: offline", "jev.nudge: every 2", "Permissions are not a sandbox", "redirections are not inspected"} {
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
