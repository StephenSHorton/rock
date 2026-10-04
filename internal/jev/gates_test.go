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

func TestOfflineRiskBlocksDestructiveShell(t *testing.T) {
	g := Gates{}
	block, p, source := g.Risk(context.Background(), "shell", "rm -rf /tmp/x")
	if !block || p < 0.9 || source != "offline" {
		t.Fatalf("block=%v p=%v source=%s", block, p, source)
	}
	block, _, _ = g.Risk(context.Background(), "shell", "go test ./...")
	if block {
		t.Fatal("ordinary shell should not block")
	}
}

func TestAllowDestructiveOverridesBlock(t *testing.T) {
	g := Gates{AllowDestructive: true}
	block, _, _ := g.Risk(context.Background(), "shell", "sudo reboot")
	if block {
		t.Fatal("allow_destructive should skip the block")
	}
}

func TestBeforeTurnOffline(t *testing.T) {
	g := Gates{}
	turn := g.BeforeTurn(context.Background(), "Please refactor the architecture of the server", []string{"release"}, nil, 100)
	if turn.Model != "strong" {
		t.Fatal(turn.Model)
	}
	if turn.Source != "offline" {
		t.Fatal(turn.Source)
	}
	turn = g.BeforeTurn(context.Background(), "use the release skill", []string{"release"}, []string{"grep", "grep", "grep"}, 100)
	if len(turn.Skills) != 1 || turn.Skills[0] != "release" {
		t.Fatalf("skills %#v", turn.Skills)
	}
	if !turn.Stuck {
		t.Fatal("expected stuck")
	}
}

func TestLiveDecide(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth %s", r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "jev-latest") {
			t.Errorf("body %s", body)
		}
		for _, key := range []string{`"model"`, `"stuck"`, `"weight"`} {
			if !strings.Contains(string(body), key) {
				t.Errorf("missing question %s in %s", key, body)
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "jev-1.13.0",
			"answers": map[string]any{
				"model":  map[string]any{"type": "choice", "choice": "strong", "confidence": 0.9, "probabilities": map[string]float64{"strong": 0.9, "fast": 0.1}},
				"stuck":  map[string]any{"type": "noul", "noul": 0.1},
				"weight": map[string]any{"type": "score", "score": 3.0, "confidence": 0.8},
			},
		})
	}))
	defer srv.Close()
	g := Gates{Client: &Client{BaseURL: srv.URL, APIKey: "test-key", HTTP: srv.Client()}, RiskBlock: 0.72, MinConfidence: 0.55}
	turn := g.BeforeTurn(context.Background(), "hi", nil, nil, 10)
	if turn.Model != "strong" || turn.Source != "live" || !turn.Compact || turn.Stuck {
		t.Fatalf("%#v", turn)
	}
	if turn.Result.Error != "" || turn.Result.Answers["model"].Value != "strong" {
		t.Fatalf("result %#v", turn.Result)
	}
}

func TestBeforeTurnAskFailureDoesNotInvent(t *testing.T) {
	g := Gates{Client: fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	})}
	turn := g.BeforeTurn(context.Background(), "short", nil, nil, 10)
	if turn.Result.Error == "" {
		t.Fatal("expected error")
	}
	for _, a := range turn.Result.Answers {
		if a.Value != nil {
			t.Fatalf("invented %#v", a)
		}
	}
	if turn.Source != "offline" || turn.Model != "fast" {
		t.Fatalf("offline fallback %#v", turn)
	}
	if !strings.Contains(turn.Result.Line(), "error=") {
		t.Fatal(turn.Result.Line())
	}
}

func TestSubagentKindAndPlanReadyUseAsk(t *testing.T) {
	var bodies []string
	g := Gates{Client: fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		if strings.Contains(string(b), `"kind"`) {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"model": "fake",
				"answers": map[string]any{
					"kind": map[string]any{"type": "choice", "choice": "explore", "confidence": 0.9},
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "fake",
			"answers": map[string]any{
				"ready": map[string]any{"type": "noul", "noul": 0.8},
			},
		})
	}), MinConfidence: 0.55}
	kind := g.DecideSubagent(context.Background(), "find the bug", "general")
	if kind.Kind != "explore" || kind.Result.Error != "" {
		t.Fatalf("%#v", kind)
	}
	ready := g.DecideReady(context.Background(), "step 1 do it", "ship")
	if !ready.Ready || ready.P != 0.8 || ready.Result.Error != "" {
		t.Fatalf("%#v", ready)
	}
	joined := strings.Join(bodies, "\n")
	if !strings.Contains(joined, `"kind"`) || !strings.Contains(joined, `"ready"`) {
		t.Fatal(joined)
	}
}

func TestForceOfflineEvenWithKey(t *testing.T) {
	var posts int
	g := Gates{
		Client:       fakeClient(t, func(http.ResponseWriter, *http.Request) { posts++ }),
		ForceOffline: true,
	}
	if g.Mode() != "offline" {
		t.Fatal(g.Mode())
	}
	turn := g.BeforeTurn(context.Background(), "hi", nil, nil, 10)
	if posts != 0 || turn.Source != "offline" {
		t.Fatalf("posts=%d turn=%#v", posts, turn)
	}
	if g.SubagentKind(context.Background(), "plan the work", "") != "plan" {
		t.Fatal("offline kind")
	}
}

func TestLiveRiskUsesAsk(t *testing.T) {
	g := Gates{Client: fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"risk"`) || !strings.Contains(string(body), `"type":"noul"`) {
			t.Errorf("body %s", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "fake",
			"answers": map[string]any{
				"risk": map[string]any{"type": "noul", "noul": 0.95},
			},
		})
	}), RiskBlock: 0.72}
	d := g.DecideRisk(context.Background(), "shell", "echo hi")
	if !d.Block || d.Source != "live" || d.P != 0.95 || d.Result.Error != "" {
		t.Fatalf("%#v", d)
	}
}

func TestEndpointFor(t *testing.T) {
	if !strings.Contains(EndpointFor("jv_live_abc"), "jevtypesafeai.com") {
		t.Fatal(EndpointFor("jv_live_abc"))
	}
	if !strings.Contains(EndpointFor("ts_live"), "api.typesafe.ai") {
		t.Fatal(EndpointFor("ts_live"))
	}
}
