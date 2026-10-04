package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResponsesHonorsPreviewLimitsAndStreams(t *testing.T) {
	var saw map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer plan-token" {
			http.Error(w, "auth", 401)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &saw); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"Hel\"}\n\n")
		_, _ = io.WriteString(w, "event: response.output_text.delta\ndata: {\"delta\":\"lo\"}\n\n")
		_, _ = io.WriteString(w, "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"Hello\"}]}]}}\n\n")
	}))
	defer srv.Close()

	p := &Responses{
		BaseURL: srv.URL + "/v1",
		Token:   func(context.Context) (string, error) { return "plan-token", nil },
	}
	msg, err := p.Complete(context.Background(), "gpt-4o", []Message{
		{Role: RoleSystem, Content: "be brief"},
		{Role: RoleUser, Content: "Say hello"},
	}, []ToolSpec{{Name: "read_file", Description: "read", Parameters: map[string]any{"type": "object"}}})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "Hello" {
		t.Fatalf("content %q", msg.Content)
	}
	if saw["store"] != false || saw["stream"] != true {
		t.Fatalf("preview limits: %#v", saw)
	}
	if _, ok := saw["temperature"]; ok {
		t.Fatal("temperature is rejected on this route")
	}
	if _, ok := saw["user"]; ok {
		t.Fatal("user is rejected on this route")
	}
	if saw["instructions"] != "be brief" {
		t.Fatalf("system must become instructions: %#v", saw)
	}
	input, _ := saw["input"].([]any)
	for _, item := range input {
		m := item.(map[string]any)
		if m["role"] == "system" {
			t.Fatal("system role item in input")
		}
	}
	tools, _ := saw["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools %#v", saw["tools"])
	}
}

func TestResponsesFunctionCallFromCompleted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		payload := `{"type":"response.completed","response":{"output":[{"type":"function_call","call_id":"call_1","name":"read_file","arguments":"{\"path\":\"a.go\"}"}]}}`
		_, _ = io.WriteString(w, "event: response.completed\ndata: "+payload+"\n\n")
	}))
	defer srv.Close()
	p := &Responses{BaseURL: srv.URL, Token: func(context.Context) (string, error) { return "t", nil }}
	msg, err := p.Complete(context.Background(), "gpt-4o", []Message{{Role: RoleUser, Content: "read"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].Name != "read_file" || !strings.Contains(msg.ToolCalls[0].Arguments, "a.go") {
		t.Fatalf("%+v", msg)
	}
}

func TestResponsesMapsToolLoop(t *testing.T) {
	var saw map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&saw)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"output\":[{\"type\":\"message\",\"content\":\"done\"}]}}\n\n")
	}))
	defer srv.Close()
	p := &Responses{BaseURL: srv.URL}
	_, err := p.Complete(context.Background(), "gpt-4o", []Message{
		{Role: RoleUser, Content: "edit"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "c1", Name: "read_file", Arguments: `{"path":"x"}`}}},
		{Role: RoleTool, ToolCallID: "c1", Content: "package x"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	input, _ := saw["input"].([]any)
	kinds := make([]string, 0, len(input))
	for _, item := range input {
		kinds = append(kinds, item.(map[string]any)["type"].(string))
	}
	want := []string{"message", "function_call", "function_call_output"}
	if len(kinds) != 3 || kinds[0] != want[0] || kinds[1] != want[1] || kinds[2] != want[2] {
		t.Fatalf("%v", kinds)
	}
}

func TestResponsesFailsWithoutCompleted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: response.output_text.delta\ndata: {\"delta\":\"hi\"}\n\n")
	}))
	defer srv.Close()
	p := &Responses{BaseURL: srv.URL}
	if _, err := p.Complete(context.Background(), "gpt-4o", []Message{{Role: RoleUser, Content: "x"}}, nil); err == nil {
		t.Fatal("expected incomplete stream error")
	}
}
