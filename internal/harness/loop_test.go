package harness

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StephenSHorton/rock/internal/jev"
	"github.com/StephenSHorton/rock/internal/perms"
	"github.com/StephenSHorton/rock/internal/provider"
	"github.com/StephenSHorton/rock/internal/session"
	"github.com/StephenSHorton/rock/internal/tools"
)

func TestTurnEditsFile(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := &provider.Script{Replies: []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "c1", Name: "edit_file", Arguments: `{"path":"a.txt","old":"world","new":"rock"}`}}},
		{Role: provider.RoleAssistant, Content: "renamed the greeting"},
	}}
	set := tools.New(tools.Env{Root: dir, PlanPath: filepath.Join(dir, "plan.md")})
	h := New(Options{
		Provider: script,
		FastModel: "fast",
		StrongModel: "strong",
		Policy: perms.Policy{Mode: perms.ModeYolo, Allow: []string{"edit_file", "read_file"}},
		Gates: jev.Gates{},
		Tools: set,
		MaxSteps: 4,
	})
	sess, err := session.Create(dir, "s1", "t")
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	err = h.Run(context.Background(), sess, "change world to rock", func(ev Event) {
		kinds = append(kinds, string(ev.Kind))
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "a.txt"))
	if string(got) != "hello rock" {
		t.Fatal(string(got))
	}
	if sess.LastAssistant() != "renamed the greeting" {
		t.Fatal(sess.LastAssistant())
	}
	if !strings.Contains(strings.Join(kinds, ","), "tool_result") {
		t.Fatal(kinds)
	}
}

func TestPlanModeDeniesEdit(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := &provider.Script{Replies: []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "c1", Name: "edit_file", Arguments: `{"path":"a.txt","old":"x","new":"y"}`}}},
		{Role: provider.RoleAssistant, Content: "could not edit"},
	}}
	h := New(Options{
		Provider: script,
		FastModel: "fast",
		Policy: perms.Policy{Mode: perms.ModePlan, Allow: []string{"*"}},
		Gates: jev.Gates{},
		Tools: tools.New(tools.Env{Root: dir, PlanPath: filepath.Join(t.TempDir(), "plan.md")}),
	})
	sess, err := session.Create(dir, "s2", "plan")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Run(context.Background(), sess, "edit it", nil); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "a.txt"))
	if string(got) != "x" {
		t.Fatal(string(got))
	}
}

func TestDestructiveShellBlockedUnderYolo(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	dir := t.TempDir()
	script := &provider.Script{Replies: []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "c1", Name: "shell", Arguments: `{"command":"rm -rf /tmp/nope"}`}}},
		{Role: provider.RoleAssistant, Content: "blocked"},
	}}
	h := New(Options{
		Provider: script,
		FastModel: "fast",
		Policy: perms.Policy{Mode: perms.ModeYolo},
		Gates: jev.Gates{},
		Tools: tools.New(tools.Env{Root: dir}),
	})
	sess, err := session.Create(dir, "s3", "risk")
	if err != nil {
		t.Fatal(err)
	}
	var saw string
	if err := h.Run(context.Background(), sess, "clean", func(ev Event) {
		if ev.Kind == EvPermission {
			saw = ev.Text
		}
		if ev.Kind == EvJev && ev.Name == "risk" {
			saw = ev.Text
		}
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saw, "block=true") && !strings.Contains(sess.Messages[len(sess.Messages)-2].Content, "denied") {
		t.Fatalf("saw %q msgs %#v", saw, sess.Messages)
	}
}
