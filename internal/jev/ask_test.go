package jev

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeClient is a test-only Jev: httptest + a live-looking key. Process
// fail-fast without a real key lives in another PR.
func fakeClient(t *testing.T, fn http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(fn)
	t.Cleanup(srv.Close)
	return &Client{BaseURL: srv.URL, APIKey: "test-key", HTTP: srv.Client()}
}

func TestAskLiveBatchMixedModes(t *testing.T) {
	var sawBody []byte
	var posts int
	g := Gates{Client: fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		posts++
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth %s", r.Header.Get("Authorization"))
		}
		sawBody, _ = io.ReadAll(r.Body)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "jev-1.13.0",
			"answers": map[string]any{
				"resolved": map[string]any{"type": "noul", "noul": 0.91},
				"kind":     map[string]any{"type": "choice", "choice": "round", "confidence": 0.8, "probabilities": map[string]float64{"round": 0.8, "other": 0.2}},
				"risk":     map[string]any{"type": "score", "score": 1.2, "confidence": 0.7, "legend": map[string]string{"0": "safe", "3": "dangerous"}},
			},
			"usage": map[string]any{"input_tokens": 40, "output_tokens": 12},
		})
	})}
	res := g.Ask(context.Background(), map[string]any{"log": "expected 2 got 1.9"}, []Query{
		{Name: "resolved", Question: "Is the rounding failure gone?", Mode: "boolean"},
		{Name: "kind", Question: "What kind of failure?", Mode: "choice", Options: map[string]string{"round": "rounding", "other": "other"}},
		{Name: "risk", Question: "How risky is this change?", Mode: "score", Levels: []string{"safe", "caution", "dangerous", "stop"}},
	})
	if posts != 1 {
		t.Fatalf("batched posts: %d", posts)
	}
	if res.Source != "live" || res.Model != "jev-1.13.0" || res.Error != "" {
		t.Fatalf("%#v", res)
	}
	if res.Answers["resolved"].Value != 0.91 || res.Answers["resolved"].Mode != ModeBoolean {
		t.Fatalf("boolean %#v", res.Answers["resolved"])
	}
	if res.Answers["kind"].Value != "round" || res.Answers["kind"].Confidence != 0.8 {
		t.Fatalf("choice %#v", res.Answers["kind"])
	}
	if res.Answers["risk"].Value != 1.2 || res.Answers["risk"].Legend["0"] != "safe" {
		t.Fatalf("score %#v", res.Answers["risk"])
	}
	if res.Usage == nil || res.Usage.InputTokens != 40 {
		t.Fatalf("usage %#v", res.Usage)
	}
	body := string(sawBody)
	if !strings.Contains(body, `"resolved"`) || !strings.Contains(body, `"kind"`) || !strings.Contains(body, `"risk"`) {
		t.Fatalf("not batched: %s", body)
	}
	if !strings.Contains(body, `"type":"noul"`) || !strings.Contains(body, `"type":"choice"`) || !strings.Contains(body, `"type":"score"`) {
		t.Fatalf("wire types: %s", body)
	}
}

func TestAskRuntimeErrorDoesNotInvent(t *testing.T) {
	g := Gates{Client: fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	})}
	res := g.Ask(context.Background(), "x", []Query{
		{Name: "q", Question: "ok?", Mode: "boolean"},
	})
	if res.Error == "" || res.Answers["q"].Value != nil || res.Source == "live" {
		t.Fatalf("%#v", res)
	}
	if res.Answers["q"].Detail != FailedDetail {
		t.Fatal(res.Answers["q"].Detail)
	}
	if !strings.Contains(res.Error, "502") {
		t.Fatal(res.Error)
	}
}

func TestAskUnwiredClientIsRuntimeError(t *testing.T) {
	res := (Gates{}).Ask(context.Background(), "x", []Query{
		{Name: "q", Question: "ok?", Mode: "boolean"},
	})
	if res.Error == "" || res.Answers["q"].Value != nil {
		t.Fatalf("must not look like a successful stub: %#v", res)
	}
}

func TestValidateQueries(t *testing.T) {
	if err := ValidateQueries(nil); err == nil {
		t.Fatal("empty")
	}
	if err := ValidateQuery(Query{Name: "c", Question: "pick", Mode: "choice"}); err == nil {
		t.Fatal("choice without options")
	}
	if err := ValidateQuery(Query{Name: "s", Question: "score", Mode: "score", Levels: []string{"only"}}); err == nil {
		t.Fatal("one level")
	}
	if err := ValidateQuery(Query{Name: "x", Question: "?", Mode: "maybe"}); err == nil {
		t.Fatal("unknown mode")
	}
	if err := ValidateQueries([]Query{
		{Name: "a", Question: "one", Mode: "boolean"},
		{Name: "a", Question: "two", Mode: "boolean"},
	}); err == nil {
		t.Fatal("duplicate")
	}
}

func TestResultLineModesAndFailure(t *testing.T) {
	ok := Result{
		Source: "live",
		Answers: map[string]Answer{
			"resolved": {Mode: ModeBoolean, Question: "Is the rounding failure gone?", Value: 0.91},
			"kind":     {Mode: ModeChoice, Question: "What kind of failure?", Value: "round", Confidence: 0.8},
			"risk":     {Mode: ModeScore, Question: "How risky is this change?", Value: 1.2, Legend: map[string]string{"0": "safe", "1": "caution"}},
		},
	}
	line := ok.Line()
	if !strings.Contains(line, "live") || !strings.Contains(line, "resolved=0.91") {
		t.Fatal(line)
	}
	if !strings.Contains(line, "boolean") || !strings.Contains(line, "Is the rounding failure gone?") {
		t.Fatal(line)
	}
	if !strings.Contains(line, "kind=round") || !strings.Contains(line, "0.80") {
		t.Fatal(line)
	}
	if !strings.Contains(line, "risk=1.2") || !strings.Contains(line, "caution") {
		t.Fatal(line)
	}
	fail := Result{
		Error: "jev http 502: nope",
		Answers: map[string]Answer{
			"q": {Mode: ModeBoolean, Question: "ok?", Detail: FailedDetail},
		},
	}
	got := fail.Line()
	if !strings.Contains(got, "error=") || strings.Contains(got, "q=0") || !strings.Contains(got, FailedDetail) {
		t.Fatal(got)
	}
}

func TestAskKeepsReasonWhenPresent(t *testing.T) {
	g := Gates{Client: fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "jev-1.13.0",
			"answers": map[string]any{
				"q": map[string]any{"type": "noul", "noul": 0.2, "reason": "still failing the same assertion"},
			},
		})
	})}
	res := g.Ask(context.Background(), "x", []Query{{Name: "q", Question: "resolved?", Mode: "boolean"}})
	if res.Answers["q"].Reason != "still failing the same assertion" {
		t.Fatalf("%#v", res.Answers["q"])
	}
}
