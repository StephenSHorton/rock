package grokcli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModelsFromSessionConfigOptions(t *testing.T) {
	fake := &FakeScript{
		Models: []ModelInfo{
			{ID: "grok-4", Name: "Grok 4"},
			{ID: "grok-3-mini", Name: "Grok 3 Mini"},
		},
		CurrentModel: "grok-4",
		Reply:        "hi",
	}
	p := &Provider{CWD: t.TempDir(), Start: StartFake(fake)}
	defer p.Close()
	cur, models, err := p.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cur != "grok-4" || len(models) != 2 {
		t.Fatalf("cur=%q models=%#v", cur, models)
	}
	if models[0].ID != "grok-4" || models[1].Name != "Grok 3 Mini" {
		t.Fatalf("%#v", models)
	}
	if err := p.SetModel(context.Background(), "grok-3-mini"); err != nil {
		t.Fatal(err)
	}
	if fake.SetModelSeen != "grok-3-mini" {
		t.Fatalf("set seen %q", fake.SetModelSeen)
	}
	cur, _, err = p.Models(context.Background())
	if err != nil || cur != "grok-3-mini" {
		t.Fatalf("cur=%q err=%v", cur, err)
	}
}

func TestModelsEmptyWhenChildOmitsList(t *testing.T) {
	fake := &FakeScript{Reply: "hi"}
	p := &Provider{CWD: t.TempDir(), Start: StartFake(fake)}
	defer p.Close()
	cur, models, err := p.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cur != "" || len(models) != 0 {
		t.Fatalf("expected empty list, got cur=%q models=%#v", cur, models)
	}
	if err := p.SetModel(context.Background(), "invented"); err == nil || !strings.Contains(err.Error(), "did not advertise") {
		t.Fatalf("expected advertise error, got %v", err)
	}
}

func TestProbeCacheRoundTrip(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	ClearProbeCache()
	st := Status{Bin: "/tmp/fake-grok", Found: true, SignedIn: true, Detail: "official grok binary (signed in)"}
	saveProbeCache(st)
	got, ok := loadProbeCache(st.Bin)
	if !ok || !got.SignedIn || got.Bin != st.Bin {
		t.Fatalf("cache %#v ok=%v", got, ok)
	}
	raw := []byte(`{"bin":"/tmp/fake-grok","signed_in":true,"detail":"x","checked":"2020-01-01T00:00:00Z"}`)
	if err := os.WriteFile(probeCachePath(), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := loadProbeCache(st.Bin); ok {
		t.Fatal("expired cache should miss")
	}
}

func TestProbeSignedInCachedUsesCache(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	ClearProbeCache()
	dir := t.TempDir()
	bin := filepath.Join(dir, "grok")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	saveProbeCache(Status{Bin: bin, Found: true, SignedIn: true, Detail: "cached"})
	called := false
	start := StartFunc(func(context.Context, string, []string) (io.WriteCloser, io.ReadCloser, func(), error) {
		called = true
		return nil, nil, nil, io.ErrClosedPipe
	})
	st := ProbeSignedInCached(context.Background(), bin, dir, start)
	if called {
		t.Fatal("cache hit must not dial grok")
	}
	if !st.SignedIn || st.Detail != "cached" {
		t.Fatalf("%#v", st)
	}
}
