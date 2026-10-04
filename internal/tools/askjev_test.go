package tools

import (
	"context"
	"encoding/json"
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

func TestJevDecideIsGone(t *testing.T) {
	set := New(Env{Root: t.TempDir(), Ask: fakeAsk(func(any, []jev.Query) jev.Result {
		return jev.Result{}
	})})
	if _, err := set.Run(context.Background(), "jev_decide", `{"question":"x","type":"noul","state":"y"}`); err == nil {
		t.Fatal("expected unknown tool")
	}
}
