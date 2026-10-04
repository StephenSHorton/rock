package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StephenSHorton/rock/internal/jev"
)

func fakeAsk(fn func(state any, qs []jev.Query) jev.Result) AskFunc {
	return func(_ context.Context, state any, qs []jev.Query) (jev.Result, error) {
		return fn(state, qs), nil
	}
}

func TestAskJevReplacesDecideInSpecs(t *testing.T) {
	set := New(Env{Root: t.TempDir()})
	var names []string
	for _, spec := range set.Specs() {
		names = append(names, spec.Name)
		if spec.Name == "ask_jev" && spec.Description == "" {
			t.Fatal("empty description")
		}
	}
	joined := strings.Join(names, ",")
	if !strings.Contains(joined, "ask_jev") {
		t.Fatal(joined)
	}
	if strings.Contains(joined, "jev_decide") {
		t.Fatal("jev_decide should be gone")
	}
}

func TestParseAskSchemaErrors(t *testing.T) {
	cases := []string{
		`{}`,
		`{"state":"x","questions":[]}`,
		`{"state":"x","question":"pick one","mode":"choice"}`,
		`{"state":"x","question":"how bad","mode":"score"}`,
		`{"state":"x","question":"huh","mode":"maybe"}`,
	}
	set := New(Env{Root: t.TempDir(), Ask: fakeAsk(func(any, []jev.Query) jev.Result {
		t.Fatal("Ask should not run on bad schema")
		return jev.Result{}
	})})
	for _, args := range cases {
		if _, err := set.Run(context.Background(), "ask_jev", args); err == nil {
			t.Fatalf("expected error for %s", args)
		}
	}
}

func TestParseAskModesAndContext(t *testing.T) {
	var gotState any
	var got []jev.Query
	set := New(Env{Root: t.TempDir(), Ask: fakeAsk(func(state any, qs []jev.Query) jev.Result {
		gotState, got = state, qs
		return jev.Result{Source: "live", Answers: map[string]jev.Answer{}}
	})})
	args := `{
		"state": {"log": "fail"},
		"context": {"file": "math.go"},
		"questions": [
			{"name":"done","question":"Is the rounding failure gone?","mode":"boolean"},
			{"name":"kind","question":"What failed?","mode":"choice","options":["rounding","other"]},
			{"name":"risk","question":"How risky?","mode":"score","range":[0,3],"rubric":"0 safe, 3 stop"}
		]
	}`
	if _, err := set.Run(context.Background(), "ask_jev", args); err != nil {
		t.Fatal(err)
	}
	st, ok := gotState.(map[string]any)
	if !ok || st["log"] != "fail" || st["file"] != "math.go" {
		t.Fatalf("state %#v", gotState)
	}
	if len(got) != 3 {
		t.Fatalf("len %d", len(got))
	}
	if got[0].Mode != "boolean" || got[1].Options["rounding"] != "rounding" {
		t.Fatalf("%#v", got)
	}
	if strings.Join(got[2].Levels, ",") != "0,1,2,3" {
		t.Fatalf("levels %#v", got[2].Levels)
	}
	if !strings.Contains(got[2].Question, "Rubric:") {
		t.Fatalf("rubric not folded: %q", got[2].Question)
	}
}

func TestParseAskSingleQuestionShorthand(t *testing.T) {
	var got []jev.Query
	set := New(Env{Root: t.TempDir(), Ask: fakeAsk(func(_ any, qs []jev.Query) jev.Result {
		got = qs
		return jev.Result{Source: "live", Answers: map[string]jev.Answer{}}
	})})
	_, err := set.Run(context.Background(), "ask_jev", `{"state":"boom","question":"Is this a flaky test?","mode":"boolean"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "q" || got[0].Mode != "boolean" {
		t.Fatalf("%#v", got)
	}
}

func TestAskJevEachModeAgainstFake(t *testing.T) {
	set := New(Env{Root: t.TempDir(), Ask: fakeAsk(func(_ any, qs []jev.Query) jev.Result {
		answers := map[string]jev.Answer{}
		for _, q := range qs {
			switch queryMode(q) {
			case jev.ModeBoolean:
				answers[q.Name] = jev.Answer{Mode: jev.ModeBoolean, Value: 0.4}
			case jev.ModeChoice:
				answers[q.Name] = jev.Answer{Mode: jev.ModeChoice, Value: "rounding", Confidence: 0.9}
			case jev.ModeScore:
				answers[q.Name] = jev.Answer{Mode: jev.ModeScore, Value: 2.0, Confidence: 0.6}
			}
		}
		return jev.Result{Source: "live", Model: "fake", Answers: answers}
	})})
	out, err := set.Run(context.Background(), "ask_jev", `{
		"state": {"err": "want 2 got 1"},
		"questions": [
			{"name":"b","question":"resolved?","mode":"boolean"},
			{"name":"c","question":"class","mode":"choice","options":{"rounding":"float","logic":"wrong branch"}},
			{"name":"s","question":"risk","mode":"score","levels":["low","mid","high"]}
		]
	}`)
	if err != nil {
		t.Fatal(err)
	}
	var res jev.Result
	if err := json.Unmarshal([]byte(out.Output), &res); err != nil {
		t.Fatal(err)
	}
	if res.Source != "live" || res.Answers["b"].Value != 0.4 || res.Answers["c"].Value != "rounding" || res.Answers["s"].Value != 2.0 {
		t.Fatalf("%#v", res.Answers)
	}
}

func TestAskJevToolResultKeepsRawBooleanFloat(t *testing.T) {
	set := New(Env{Root: t.TempDir(), Ask: fakeAsk(func(any, []jev.Query) jev.Result {
		return jev.Result{
			Source: "live",
			Answers: map[string]jev.Answer{
				"resolved": {Mode: jev.ModeBoolean, Question: "gone?", Value: 0.88},
				"open":     {Mode: jev.ModeBoolean, Question: "still broken?", Value: 0.12},
			},
		}
	})})
	out, err := set.Run(context.Background(), "ask_jev", `{
		"state":"rounding",
		"questions":[
			{"name":"resolved","question":"gone?","mode":"boolean"},
			{"name":"open","question":"still broken?","mode":"boolean"}
		]
	}`)
	if err != nil {
		t.Fatal(err)
	}
	var res jev.Result
	if err := json.Unmarshal([]byte(out.Output), &res); err != nil {
		t.Fatal(err)
	}
	if res.Answers["resolved"].Value != 0.88 || res.Answers["open"].Value != 0.12 {
		t.Fatalf("tool result must keep the raw noul float: %#v", res.Answers)
	}
	if strings.Contains(out.Output, "yes") || strings.Contains(out.Output, "no 0") {
		t.Fatalf("yes/no display must not leak into the tool JSON:\n%s", out.Output)
	}
}

func TestAskJevRuntimeErrorIsStructured(t *testing.T) {
	set := New(Env{Root: t.TempDir(), Ask: fakeAsk(func(any, []jev.Query) jev.Result {
		return jev.Result{
			Error: "jev http 502: nope",
			Answers: map[string]jev.Answer{
				"q": {Mode: jev.ModeBoolean, Detail: jev.FailedDetail},
			},
		}
	})})
	out, err := set.Run(context.Background(), "ask_jev", `{"state":"log","question":"fixed?","mode":"boolean"}`)
	if err != nil {
		t.Fatal(err)
	}
	var res jev.Result
	if err := json.Unmarshal([]byte(out.Output), &res); err != nil {
		t.Fatal(err)
	}
	if res.Error == "" || res.Answers["q"].Value != nil {
		t.Fatalf("%#v", res)
	}
}

func TestAskJevPathsReadsWorkspaceClips(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "math.go"), []byte("package math\nfunc Add() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var gotState any
	var got []jev.Query
	set := New(Env{Root: dir, Ask: fakeAsk(func(state any, qs []jev.Query) jev.Result {
		gotState, got = state, qs
		return jev.Result{
			Source: "live",
			Answers: map[string]jev.Answer{
				"keep_0": {Mode: jev.ModeBoolean, Value: 0.9},
			},
		}
	})})
	out, err := set.Run(context.Background(), "ask_jev", `{"paths":["math.go"],"state":{"task":"find Add"}}`)
	if err != nil {
		t.Fatal(err)
	}
	st, ok := gotState.(map[string]any)
	if !ok {
		t.Fatalf("state %#v", gotState)
	}
	clips, _ := st["clips"].([]any)
	if len(clips) != 1 {
		t.Fatalf("clips %#v", st["clips"])
	}
	clip, _ := clips[0].(map[string]any)
	if clip["path"] != "math.go" || !strings.Contains(fmt.Sprint(clip["text"]), "func Add") {
		t.Fatalf("%#v", clip)
	}
	if len(got) != 1 || got[0].Name != "keep_0" {
		t.Fatalf("generated keep %#v", got)
	}
	var res jev.Result
	if err := json.Unmarshal([]byte(out.Output), &res); err != nil {
		t.Fatal(err)
	}
	if res.Filter == nil || res.Filter.ClipsBefore != 1 || res.Filter.ClipsAfter != 1 {
		t.Fatalf("stats %#v", res.Filter)
	}
	if len(res.Clips) != 1 || res.Clips[0].Path != "math.go" {
		t.Fatalf("kept %#v", res.Clips)
	}
}

func TestAskJevPathsRejectEscapeAndClip(t *testing.T) {
	dir := t.TempDir()
	big := strings.Repeat("x", 4000)
	if err := os.WriteFile(filepath.Join(dir, "big.go"), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	var gotState any
	set := New(Env{Root: dir, ClipBytes: 20, Ask: fakeAsk(func(state any, qs []jev.Query) jev.Result {
		gotState = state
		return jev.Result{Source: "live", Answers: map[string]jev.Answer{
			"q": {Mode: jev.ModeBoolean, Value: 0.2},
		}}
	})})
	if _, err := set.Run(context.Background(), "ask_jev", `{"paths":["../secret"],"question":"match?","mode":"boolean"}`); err == nil {
		t.Fatal("expected escape")
	}
	out, err := set.Run(context.Background(), "ask_jev", `{"paths":["big.go"],"state":"task","question":"relevant?","mode":"boolean"}`)
	if err != nil {
		t.Fatal(err)
	}
	st := gotState.(map[string]any)
	clips := st["clips"].([]any)
	text := fmt.Sprint(clips[0].(map[string]any)["text"])
	if len(text) != 20 {
		t.Fatalf("clipped %d %q", len(text), text)
	}
	var res jev.Result
	if err := json.Unmarshal([]byte(out.Output), &res); err != nil {
		t.Fatal(err)
	}
	if res.Filter == nil || res.Filter.ClipsBefore != 1 || res.Filter.ClipsAfter != 0 {
		t.Fatalf("custom question must not dump the clip back: %#v", res.Filter)
	}
	if len(res.Clips) != 0 {
		t.Fatalf("clips leaked %#v", res.Clips)
	}
}

func TestAskJevPathsOfflineDoesNotInventMatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	set := New(Env{Root: dir, Ask: fakeAsk(func(any, []jev.Query) jev.Result {
		return jev.Result{Error: "jev client is not wired", Answers: map[string]jev.Answer{
			"keep_0": {Mode: jev.ModeBoolean, Detail: jev.FailedDetail},
		}}
	})})
	out, err := set.Run(context.Background(), "ask_jev", `{"paths":["a.go"]}`)
	if err != nil {
		t.Fatal(err)
	}
	var res jev.Result
	if err := json.Unmarshal([]byte(out.Output), &res); err != nil {
		t.Fatal(err)
	}
	if res.Error == "" || len(res.Clips) != 0 || res.Answers["keep_0"].Value != nil {
		t.Fatalf("must not invent a match: %#v", res)
	}
}

func TestGrepFilterStatsAreMeasured(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("keep hit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("drop hit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	set := New(Env{Root: dir, FilterSnippets: func(_ string, snips []string) []string {
		var out []string
		for _, s := range snips {
			if strings.Contains(s, "keep") {
				out = append(out, s)
			}
		}
		return out
	}})
	out, err := set.Run(context.Background(), "grep", `{"pattern":"hit"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Output, "keep") || strings.Contains(out.Output, "drop") {
		t.Fatal(out.Output)
	}
	if out.Filter == nil || out.Filter.ClipsBefore != 2 || out.Filter.ClipsAfter != 1 {
		t.Fatalf("%#v", out.Filter)
	}
	if out.Filter.BytesBefore <= out.Filter.BytesAfter {
		t.Fatalf("bytes should shrink %#v", out.Filter)
	}
}

func TestJevDecideIsGone(t *testing.T) {
	set := New(Env{Root: t.TempDir(), Ask: fakeAsk(func(any, []jev.Query) jev.Result {
		return jev.Result{}
	})})
	if _, err := set.Run(context.Background(), "jev_decide", `{"question":"x","type":"noul","state":"y"}`); err == nil {
		t.Fatal("expected unknown tool")
	}
}
