package session

import (
	"path/filepath"
	"testing"

	"github.com/StephenSHorton/rock/internal/provider"
)

func TestRoundTrip(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	dir := t.TempDir()
	s, err := Create(dir, "abc", "first")
	if err != nil {
		t.Fatal(err)
	}
	s.Append(provider.Message{Role: provider.RoleUser, Content: "hello"})
	s.Append(provider.Message{Role: provider.RoleAssistant, Content: "hi", ToolCalls: []provider.ToolCall{{ID: "1", Name: "read_file", Arguments: `{"path":"a.go"}`}}})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir, "abc")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 2 || got.Messages[1].ToolCalls[0].Name != "read_file" {
		t.Fatalf("%#v", got.Messages)
	}
	if got.PlanPath() != filepath.Join(got.Dir, "plan.md") {
		t.Fatal(got.PlanPath())
	}
	list, err := List(dir)
	if err != nil || len(list) != 1 || list[0].ID != "abc" {
		t.Fatalf("%v %#v", err, list)
	}
	got.Compact("did a thing", 1)
	if len(got.Messages) != 2 {
		t.Fatalf("compact %#v", got.Messages)
	}
}
