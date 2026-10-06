package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/StephenSHorton/rock/internal/grokcli"
)

func headlessGrok(t *testing.T, verbose bool) (string, string) {
	t.Helper()
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
	app.Provider = &grokcli.Provider{CWD: dir, Start: grokcli.StartFake(&grokcli.FakeScript{
		Thoughts: []string{"weighing ", "options"}, Reply: "pong", ReplyChunks: 2,
	})}
	app.Auth = grokcli.AuthClass
	app.Verbose = verbose
	sess, err := app.OpenSession("", true, "p")
	if err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if err := app.Headless(context.Background(), sess, "Reply with just the word pong.", "text", &out, &errb, nil); err != nil {
		t.Fatal(err)
	}
	return out.String(), errb.String()
}

func TestHeadlessOutputUnchangedByStreaming(t *testing.T) {
	out, errb := headlessGrok(t, false)
	if out != "pong\n" {
		t.Fatalf("stdout %q", out)
	}
	if strings.Contains(errb, "thinking") || strings.Contains(errb, "weighing") {
		t.Fatalf("thoughts leaked without --verbose: %q", errb)
	}
}

func TestHeadlessVerboseShowsThoughtsOnStderr(t *testing.T) {
	out, errb := headlessGrok(t, true)
	if out != "pong\n" {
		t.Fatalf("stdout %q", out)
	}
	if !strings.Contains(errb, "thinking: weighing options\n") {
		t.Fatalf("stderr %q", errb)
	}
}
