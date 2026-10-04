package jev

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestKeepSnippetOfflineKeeps(t *testing.T) {
	g := Gates{}
	if !g.KeepSnippet(context.Background(), "FindMe", "note.go:2:func FindMe()") {
		t.Fatal("offline must keep")
	}
	if got := g.FilterSnippets(context.Background(), "FindMe", []string{"a", "b"}); len(got) != 2 {
		t.Fatalf("%#v", got)
	}
}

func TestFilterSnippetsUsesAsk(t *testing.T) {
	var saw []byte
	g := Gates{Client: fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		saw, _ = io.ReadAll(r.Body)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "fake",
			"answers": map[string]any{
				"keep_0": map[string]any{"type": "noul", "noul": 0.9},
				"keep_1": map[string]any{"type": "noul", "noul": 0.1},
			},
		})
	}), MinConfidence: 0.55}
	got := g.FilterSnippets(context.Background(), "FindMe", []string{"keep this hit", "drop this hit"})
	if len(got) != 1 || got[0] != "keep this hit" {
		t.Fatalf("%#v", got)
	}
	body := string(saw)
	if !strings.Contains(body, `"keep_0"`) || !strings.Contains(body, `"keep_1"`) || !strings.Contains(body, `"type":"noul"`) {
		t.Fatal(body)
	}
	if !strings.Contains(body, "keep this hit") {
		t.Fatal(body)
	}
}

func TestKeepSnippetFailedAskKeeps(t *testing.T) {
	g := Gates{Client: fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	})}
	if !g.KeepSnippet(context.Background(), "q", "snippet") {
		t.Fatal("failed Ask must not invent a drop")
	}
}

func TestCompactClassifyEmptyOnError(t *testing.T) {
	if CompactClassify(Result{Error: "down", Answers: map[string]Answer{"kind": {Value: "test"}}}) != "" {
		t.Fatal("must not print a class when Error is set")
	}
	line := CompactClassify(Result{Answers: map[string]Answer{
		"kind":  {Value: "test"},
		"area":  {Value: "code"},
		"retry": {Value: 0.12},
	}})
	if !strings.Contains(line, "kind=test") || !strings.Contains(line, "area=code") || !strings.Contains(line, "retry=0.12") {
		t.Fatal(line)
	}
}

func TestClipSize(t *testing.T) {
	if ClipSize(0) != DefaultClipBytes || ClipSize(9000) != MaxClipBytes || ClipSize(400) != 400 {
		t.Fatalf("%d %d %d", ClipSize(0), ClipSize(9000), ClipSize(400))
	}
	if Clip("abcdef", 3) != "abc" {
		t.Fatal(Clip("abcdef", 3))
	}
}
