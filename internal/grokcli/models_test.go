package grokcli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChildArgsNeverNoAutoUpdate(t *testing.T) {
	args := ChildArgs()
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "no-auto-update") {
		t.Fatalf("grok 1.0.41 rejects --no-auto-update: %v", args)
	}
	if err := rejectForbidden(args); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(joined, "agent") || !strings.Contains(joined, "--no-leader") || !strings.Contains(joined, "stdio") {
		t.Fatal(joined)
	}
	withModel := ChildArgsModel("grok-4.7")
	if strings.Join(withModel, " ") != "agent --no-leader -m grok-4.7 stdio" {
		t.Fatal(withModel)
	}
}

func TestParseInitializeModelStateFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/initialize_1.0.41.json")
	if err != nil {
		t.Fatal(err)
	}
	c := &client{}
	var out struct {
		AuthMethods []struct {
			ID string `json:"id"`
		} `json:"authMethods"`
		Meta json.RawMessage `json:"_meta"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	c.applyModelStateMeta(out.Meta)
	want := []string{"grok-4.7", "grok-4.7-build-fast", "grok-4.6", "grok-4.5"}
	if c.modelCurrent != "grok-4.7-build-fast" {
		t.Fatalf("current %q", c.modelCurrent)
	}
	if len(c.models) != len(want) {
		t.Fatalf("models %#v", c.models)
	}
	for i, id := range want {
		if c.models[i].ID != id {
			t.Fatalf("models[%d]=%q want %q", i, c.models[i].ID, id)
		}
	}
	if c.models[0].ContextTokens != 256000 || c.models[3].ContextTokens != 128000 {
		t.Fatalf("context tokens %#v", c.models)
	}
	has := map[string]bool{}
	for _, m := range out.AuthMethods {
		has[m.ID] = true
	}
	if !has["cached_token"] {
		t.Fatal("fixture must list cached_token")
	}
}

func TestModelsFromInitializeModelState(t *testing.T) {
	fake := &FakeScript{
		Models: []ModelInfo{
			{ID: "grok-4.7", Name: "Grok 4.7", ContextTokens: 256000},
			{ID: "grok-4.7-build-fast", Name: "Grok 4.7 Fast", ContextTokens: 256000},
			{ID: "grok-4.6", Name: "Grok 4.6", ContextTokens: 256000},
			{ID: "grok-4.5", Name: "Grok 4.5", ContextTokens: 128000},
		},
		CurrentModel: "grok-4.7-build-fast",
		Reply:        "hi",
	}
	p := &Provider{CWD: t.TempDir(), Start: StartFake(fake)}
	defer p.Close()
	cur, models, err := p.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cur != "grok-4.7-build-fast" || len(models) != 4 {
		t.Fatalf("cur=%q models=%#v", cur, models)
	}
	got := make([]string, len(models))
	for i, m := range models {
		got[i] = m.ID
	}
	want := "grok-4.7,grok-4.7-build-fast,grok-4.6,grok-4.5"
	if strings.Join(got, ",") != want {
		t.Fatalf("got %v", got)
	}
	if p.ContextLimit(context.Background()) != 256000 {
		t.Fatal(p.ContextLimit(context.Background()))
	}
}

func TestSetModelRestartsWithDashM(t *testing.T) {
	fake := &FakeScript{
		Models: []ModelInfo{
			{ID: "grok-4.7", Name: "Grok 4.7"},
			{ID: "grok-4.7-build-fast", Name: "Grok 4.7 Fast"},
		},
		CurrentModel: "grok-4.7-build-fast",
		Reply:        "hi",
		// No config options, no set_model → restart with -m.
	}
	p := &Provider{CWD: t.TempDir(), Start: StartFake(fake)}
	defer p.Close()
	if _, _, err := p.Models(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := p.SetModel(context.Background(), "grok-4.7"); err != nil {
		t.Fatal(err)
	}
	args := fake.Args()
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-m") || !strings.Contains(joined, "grok-4.7") {
		t.Fatalf("expected -m restart, got %v", args)
	}
	if strings.Contains(joined, "no-auto-update") {
		t.Fatal(joined)
	}
}

func TestSetModelViaConfigOption(t *testing.T) {
	fake := &FakeScript{
		Models: []ModelInfo{
			{ID: "grok-4.7", Name: "Grok 4.7"},
			{ID: "grok-4.7-build-fast", Name: "Fast"},
		},
		CurrentModel:        "grok-4.7-build-fast",
		ConfigOptionsModels: true,
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
}

func TestProbeCacheRoundTrip(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	ClearProbeCache()
	dir := t.TempDir()
	bin := filepath.Join(dir, "grok")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	st := Status{Bin: bin, Found: true, SignedIn: true, Detail: "official grok binary (signed in)"}
	saveProbeCache(st)
	got, ok := loadProbeCache(bin)
	if !ok || !got.SignedIn || got.Bin != bin {
		t.Fatalf("cache %#v ok=%v", got, ok)
	}
	// Old version ignored.
	raw := []byte(`{"version":1,"bin":"` + bin + `","signed_in":true,"detail":"x","checked":"2099-01-01T00:00:00Z","bin_mtime":1}`)
	if err := os.WriteFile(probeCachePath(), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := loadProbeCache(bin); ok {
		t.Fatal("v1 cache must miss")
	}
	// Failure must not be cached.
	ClearProbeCache()
	saveProbeCache(Status{Bin: bin, Found: true, SignedIn: false, Detail: "grok exited: unexpected argument"})
	if _, ok := loadProbeCache(bin); ok {
		t.Fatal("unsigned/failure must not cache")
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

func TestStartExecSurfacesBadFlag(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "grok")
	// Mimic grok 1.0.41 rejecting --no-auto-update.
	script := "#!/bin/sh\necho \"error: unexpected argument '--no-auto-update' found\" >&2\nexit 2\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	st := ProbeSignedIn(context.Background(), bin, dir, nil)
	if st.SignedIn {
		t.Fatal(st)
	}
	if !strings.Contains(st.Detail, "grok exited:") || !strings.Contains(st.Detail, "no-auto-update") {
		t.Fatalf("want child stderr, got %q", st.Detail)
	}
	if strings.Contains(st.Detail, "context deadline") {
		t.Fatalf("must not report deadline: %q", st.Detail)
	}
}
