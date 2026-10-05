package grokcli

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StephenSHorton/rock/internal/provider"
)

func realGrokModels() []ModelInfo {
	return []ModelInfo{
		{ID: "grok-4.7", Name: "Grok 4.7", Description: "SpaceXAI's latest frontier model", ContextTokens: 256000},
		{ID: "grok-4.7-build-fast", Name: "Grok 4.7 Fast", Description: "Fast variant. 2x the price.", ContextTokens: 256000},
		{ID: "grok-4.6", Name: "Grok 4.6", ContextTokens: 500000},
		{ID: "grok-4.5", Name: "Grok 4.5", ContextTokens: 500000},
	}
}

func TestInitializeParamsShape(t *testing.T) {
	p := InitializeParams()
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	var ver any
	if err := json.Unmarshal(decoded["protocolVersion"], &ver); err != nil {
		t.Fatal(err)
	}
	if ver != float64(1) {
		t.Fatalf("protocolVersion=%v (%T), want integer 1", ver, ver)
	}
	var info struct {
		Name, Title, Version string
	}
	if err := json.Unmarshal(decoded["clientInfo"], &info); err != nil {
		t.Fatal(err)
	}
	if info.Name != "rock" || info.Version == "" {
		t.Fatalf("clientInfo=%+v (version required; was the Invalid params root cause)", info)
	}
	var caps map[string]any
	if err := json.Unmarshal(decoded["clientCapabilities"], &caps); err != nil || caps == nil {
		t.Fatal("clientCapabilities must be object")
	}
}

func TestStrictFakeFullTurn(t *testing.T) {
	fake := &FakeScript{
		Models:              realGrokModels(),
		CurrentModel:        "grok-4.7-build-fast",
		ConfigOptionsModels: true,
		SessionModels:       true,
		StrictACP:           true,
		EmitSetupNotes:      true,
		Reply:               "pong",
	}
	cwd := t.TempDir()
	p := &Provider{CWD: cwd, Start: StartFake(fake), Timeout: 0}
	defer p.Close()

	msg, err := p.Complete(context.Background(), "gpt-4o-mini", []provider.Message{
		{Role: provider.RoleUser, Content: "Reply with just the word pong."},
	}, nil)
	if err != nil {
		t.Fatalf("turn failed: %v", err)
	}
	if msg.Content != "pong" {
		t.Fatalf("content %q", msg.Content)
	}
	if fake.AuthCalls() != 0 {
		t.Fatalf("authenticate must not be called for cached_token; got %d", fake.AuthCalls())
	}
	args := fake.Args()
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "-m") {
		t.Fatalf("must not restart with -m for gpt-4o-mini: %v", args)
	}
	if strings.Contains(joined, "gpt-") {
		t.Fatalf("must never pass OpenAI names to grok: %v", args)
	}

	// session/new must be absolute cwd + mcpServers array only.
	var newParams map[string]json.RawMessage
	fake.mu.Lock()
	rawNew := append(json.RawMessage(nil), fake.newSeen...)
	fake.mu.Unlock()
	if err := json.Unmarshal(rawNew, &newParams); err != nil {
		t.Fatal(err)
	}
	var cwdParam string
	_ = json.Unmarshal(newParams["cwd"], &cwdParam)
	if !filepath.IsAbs(cwdParam) {
		t.Fatalf("cwd not absolute: %q", cwdParam)
	}
	if _, ok := newParams["permission"]; ok {
		t.Fatal("session/new must not send permission")
	}
	if string(newParams["mcpServers"])[0] != '[' {
		t.Fatal(newParams["mcpServers"])
	}

	cur, models, err := p.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cur != "grok-4.7-build-fast" || len(models) != 4 {
		t.Fatalf("cur=%q models=%#v", cur, models)
	}
}

func TestStrictRejectsOldInitializeWithoutVersion(t *testing.T) {
	fake := &FakeScript{StrictACP: true, Reply: "x"}
	// Dial raw and send the broken shape Rock 1.0.6 used.
	ctx := context.Background()
	in, out, stop, err := StartFake(fake)(ctx, "grok", ChildArgs())
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	c := &client{in: in, out: out, stop: stop, wait: map[int64]pending{}, cwd: t.TempDir()}
	go c.readLoop()
	_, err = c.request(ctx, "initialize", map[string]any{
		"protocolVersion": 1,
		"clientCapabilities": map[string]any{
			"fs":       map[string]any{"readTextFile": false, "writeTextFile": false},
			"terminal": false,
		},
		"clientInfo": map[string]any{"name": "rock", "title": "Rock"}, // no version
	})
	if err == nil || !strings.Contains(err.Error(), "Invalid params") {
		t.Fatalf("expected Invalid params, got %v", err)
	}
	if !strings.Contains(err.Error(), "version") && !strings.Contains(err.Error(), "detail") {
		// error.data should surface
		t.Logf("err=%v (data preferred)", err)
	}
}

func TestMapGrokTargetNeverOpenAI(t *testing.T) {
	avail := realGrokModels()
	cur := "grok-4.7-build-fast"
	got := MapGrokTarget("gpt-4o-mini", avail, cur, "")
	if got != "grok-4.7-build-fast" {
		t.Fatalf("fast map got %q", got)
	}
	got = MapGrokTarget("gpt-4o", avail, cur, "")
	if got != "grok-4.7" {
		t.Fatalf("strong map got %q", got)
	}
	got = MapGrokTarget("strong", avail, cur, "")
	if got != "grok-4.7" {
		t.Fatalf("tier strong got %q", got)
	}
	got = MapGrokTarget("gpt-4o-mini", avail, cur, "grok-4.6")
	if got != "grok-4.6" {
		t.Fatalf("override got %q", got)
	}
	if LooksLikeForeignModel("gpt-4o-mini") != true || LooksLikeForeignModel("grok-4.7") != false {
		t.Fatal("LooksLikeForeignModel")
	}
}

func TestSetModelUsesConfigOptionNotDashM(t *testing.T) {
	fake := &FakeScript{
		Models:              realGrokModels(),
		CurrentModel:        "grok-4.7-build-fast",
		ConfigOptionsModels: true,
		SessionModels:       true,
		StrictACP:           true,
		Reply:               "hi",
	}
	p := &Provider{CWD: t.TempDir(), Start: StartFake(fake)}
	defer p.Close()
	if err := p.SetModel(context.Background(), "grok-4.7"); err != nil {
		t.Fatal(err)
	}
	if fake.SetModelVia != "config" || fake.SetModelSeen != "grok-4.7" {
		t.Fatalf("via=%q seen=%q", fake.SetModelVia, fake.SetModelSeen)
	}
	if strings.Contains(strings.Join(fake.Args(), " "), "-m") {
		t.Fatal(fake.Args())
	}
}

func TestCompleteDoesNotRestartEveryTurn(t *testing.T) {
	fake := &FakeScript{
		Models:              realGrokModels(),
		CurrentModel:        "grok-4.7-build-fast",
		ConfigOptionsModels: true,
		StrictACP:           true,
		Reply:               "ok",
	}
	startCount := 0
	base := StartFake(fake)
	p := &Provider{CWD: t.TempDir(), Start: StartFunc(func(ctx context.Context, bin string, args []string) (io.WriteCloser, io.ReadCloser, func(), error) {
		startCount++
		return base(ctx, bin, args)
	})}
	defer p.Close()
	for i := 0; i < 3; i++ {
		if _, err := p.Complete(context.Background(), "gpt-4o-mini", []provider.Message{
			{Role: provider.RoleUser, Content: "hi"},
		}, nil); err != nil {
			t.Fatal(err)
		}
	}
	if startCount != 1 {
		t.Fatalf("started child %d times; want 1 (no per-turn restart)", startCount)
	}
}
