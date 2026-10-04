package grokcli

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StephenSHorton/rock/internal/provider"
)

func TestLookMissing(t *testing.T) {
	t.Setenv("ROCK_GROK_BIN", "")
	t.Setenv("PATH", t.TempDir())
	st := Look("")
	if st.Found {
		t.Fatal(st)
	}
	if !strings.Contains(st.Detail, InstallURL) || !strings.Contains(st.Detail, "grok login") {
		t.Fatal(st.Detail)
	}
}

func TestLookConfiguredPath(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "grok")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	st := Look(bin)
	if !st.Found || st.Bin != bin {
		t.Fatalf("%#v", st)
	}
}

func TestChildArgsNeverAutoApprove(t *testing.T) {
	args := ChildArgs()
	if err := rejectForbidden(args); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, bad := range []string{"always-approve", "yolo", "permission-mode", "reauth"} {
		if strings.Contains(joined, bad) {
			t.Fatal(joined)
		}
	}
	if !strings.Contains(joined, "agent") || !strings.Contains(joined, "stdio") {
		t.Fatal(joined)
	}
}

func TestRejectForbidden(t *testing.T) {
	if err := rejectForbidden([]string{"agent", "--always-approve", "stdio"}); err == nil {
		t.Fatal("expected refuse")
	}
}

func TestCompleteTextAndDenyPermission(t *testing.T) {
	fake := &FakeScript{
		Reply:      "hello from SuperGrok",
		AskPerm:    true,
		AskFSWrite: true,
	}
	p := &Provider{CWD: t.TempDir(), Start: StartFake(fake), Timeout: 0}
	defer p.Close()
	msg, err := p.Complete(context.Background(), "", []provider.Message{
		{Role: provider.RoleUser, Content: "hi"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "hello from SuperGrok" {
		t.Fatalf("content %q", msg.Content)
	}
	perm, fs := p.denied()
	if perm == 0 {
		t.Fatal("child permission must be denied")
	}
	if fs == 0 {
		t.Fatal("child fs/write must be rejected")
	}
	if err := rejectForbidden(fake.Args()); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteParsesToolFence(t *testing.T) {
	fake := &FakeScript{Reply: "ok\n```tool\n{\"name\":\"read_file\",\"arguments\":{\"path\":\"README.md\"}}\n```\n"}
	p := &Provider{CWD: t.TempDir(), Start: StartFake(fake)}
	defer p.Close()
	msg, err := p.Complete(context.Background(), "", nil, []provider.ToolSpec{{Name: "read_file"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].Name != "read_file" {
		t.Fatalf("%#v", msg.ToolCalls)
	}
	if !strings.Contains(msg.ToolCalls[0].Arguments, "README.md") {
		t.Fatal(msg.ToolCalls[0].Arguments)
	}
}

func TestCompleteMapsDeniedChildTool(t *testing.T) {
	fake := &FakeScript{ToolName: "bash", ToolArgs: `{"command":"ls"}`}
	p := &Provider{CWD: t.TempDir(), Start: StartFake(fake)}
	defer p.Close()
	msg, err := p.Complete(context.Background(), "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].Name != "shell" {
		t.Fatalf("mapped %#v", msg.ToolCalls)
	}
}

func TestProbeSignedInAndMissingLogin(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "grok")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ok := &FakeScript{AuthMethods: []string{"cached_token"}}
	st := ProbeSignedIn(context.Background(), bin, dir, StartFake(ok))
	if !st.Found || !st.SignedIn {
		t.Fatalf("signed %#v", st)
	}
	no := &FakeScript{AuthMethods: []string{"device"}}
	st = ProbeSignedIn(context.Background(), bin, dir, StartFake(no))
	if st.SignedIn || !strings.Contains(st.Detail, "grok login") {
		t.Fatalf("unsigned %#v", st)
	}
}

func TestNeverReadsGrokAuthFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".grok"), 0o700); err != nil {
		t.Fatal(err)
	}
	auth := filepath.Join(home, ".grok", "auth.json")
	if err := os.WriteFile(auth, []byte(`{"secret":"nope"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	t.Setenv("ROCK_GROK_BIN", "")
	st := Look("")
	if st.Found {
		t.Fatal("must not treat ~/.grok as the binary")
	}
	raw, err := os.ReadFile(auth)
	if err != nil || !strings.Contains(string(raw), "nope") {
		t.Fatal("must not consume grok token files", err)
	}
}

func TestCompleteMissingBinary(t *testing.T) {
	t.Setenv("ROCK_GROK_BIN", "")
	t.Setenv("PATH", t.TempDir())
	p := &Provider{CWD: t.TempDir()}
	_, err := p.Complete(context.Background(), "", nil, nil)
	if err == nil || !strings.Contains(err.Error(), InstallURL) {
		t.Fatalf("%v", err)
	}
}

func TestStartExecScrubsJevKeys(t *testing.T) {
	t.Setenv("JEV_API_KEY", "secret-jev")
	t.Setenv("TYPESAFE_API_KEY", "secret-typesafe")
	bin := filepath.Join(t.TempDir(), "grok")
	script := "#!/bin/sh\nprintf 'JEV=%s\\n' \"$JEV_API_KEY\"\nprintf 'TYPESAFE=%s\\n' \"$TYPESAFE_API_KEY\"\ntest -n \"$PATH\" && printf 'PATH_OK\\n'\ncat >/dev/null\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	stdin, stdout, stop, err := startExec(context.Background(), bin, ChildArgs())
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	_ = stdin
	sc := bufio.NewScanner(stdout)
	var lines []string
	for sc.Scan() {
		lines = append(lines, sc.Text())
		if strings.Contains(sc.Text(), "PATH_OK") {
			break
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	out := strings.Join(lines, "\n")
	if strings.Contains(out, "secret-jev") || strings.Contains(out, "secret-typesafe") {
		t.Fatalf("leaked keys: %q", out)
	}
	if !strings.Contains(out, "JEV=") || !strings.Contains(out, "TYPESAFE=") {
		t.Fatalf("expected empty Jev keys: %q", out)
	}
	if strings.Contains(out, "JEV=secret") || strings.Contains(out, "TYPESAFE=secret") {
		t.Fatalf("leaked keys: %q", out)
	}
	if !strings.Contains(out, "PATH_OK") {
		t.Fatalf("PATH should still be inherited: %q", out)
	}
}
