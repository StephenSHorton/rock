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
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "jev-1.13.0",
			"answers": map[string]any{
				"model": map[string]any{"type": "choice", "choice": "strong", "confidence": 0.9, "probabilities": map[string]float64{"strong": 0.9, "fast": 0.1}},
				"stuck": map[string]any{"type": "noul", "noul": 0.1},
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
}

func TestEndpointFor(t *testing.T) {
	if !strings.Contains(EndpointFor("jv_live_abc"), "jevtypesafeai.com") {
		t.Fatal(EndpointFor("jv_live_abc"))
	}
	if !strings.Contains(EndpointFor("ts_live"), "api.typesafe.ai") {
		t.Fatal(EndpointFor("ts_live"))
	}
}
