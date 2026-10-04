package harness

import (
	"context"
	"encoding/json"
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
		Provider:    script,
		FastModel:   "fast",
		StrongModel: "strong",
		Policy:      perms.Policy{Mode: perms.ModeYolo, Allow: []string{"edit_file", "read_file"}},
		Gates:       jev.Gates{},
		Tools:       set,
		MaxSteps:    4,
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
		Provider:  script,
		FastModel: "fast",
		Policy:    perms.Policy{Mode: perms.ModePlan, Allow: []string{"*"}},
		Gates:     jev.Gates{},
		Tools:     tools.New(tools.Env{Root: dir, PlanPath: filepath.Join(t.TempDir(), "plan.md")}),
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
		Provider:  script,
		FastModel: "fast",
		Policy:    perms.Policy{Mode: perms.ModeYolo},
		Gates:     jev.Gates{},
		Tools:     tools.New(tools.Env{Root: dir}),
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

func TestAskJevBatchTurn(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	dir := t.TempDir()
	script := &provider.Script{Replies: []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "c1", Name: "ask_jev", Arguments: `{"state":"fail","questions":[{"name":"resolved","question":"fixed?","mode":"boolean"},{"name":"kind","question":"class","mode":"choice","options":["rounding","other"]}]}`}}},
		{Role: provider.RoleAssistant, Content: "it is rounding"},
	}}
	set := tools.New(tools.Env{Root: dir})
	h := New(Options{
		Provider:  script,
		FastModel: "fast",
		Policy:    perms.Policy{Mode: perms.ModeYolo, Allow: []string{"ask_jev"}},
		Gates:     jev.Gates{},
		Tools:     set,
		MaxSteps:  4,
	})
	var calls int
	var got []jev.Query
	// Fake Jev after New so BeforeTurn/Risk stay on the existing gates
	// and do not share an HTTP client with ask_jev.
	set.Env.Ask = func(_ context.Context, _ any, qs []jev.Query) (jev.Result, error) {
		calls++
		got = qs
		return jev.Result{
			Source: "live",
			Model:  "fake",
			Answers: map[string]jev.Answer{
				"resolved": {Mode: jev.ModeBoolean, Value: 0.11},
				"kind":     {Mode: jev.ModeChoice, Value: "rounding", Confidence: 0.7},
			},
		}, nil
	}
	sess, err := session.Create(dir, "ask-live", "t")
	if err != nil {
		t.Fatal(err)
	}
	var askText, done string
	if err := h.Run(context.Background(), sess, "classify", func(ev Event) {
		if ev.Kind == EvJev && ev.Name == "ask" {
			askText = ev.Text
		}
		if ev.Kind == EvDone {
			done = ev.Text
		}
	}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("batched Ask calls: %d", calls)
	}
	if len(got) != 2 || got[0].Name != "resolved" || got[1].Name != "kind" {
		t.Fatalf("%#v", got)
	}
	if done != "end_turn" {
		t.Fatal(done)
	}
	if !strings.Contains(askText, "live") || !strings.Contains(askText, "resolved=0.11") {
		t.Fatalf("ask event %q", askText)
	}
	if !strings.Contains(h.system("classify", nil), "ask_jev") || strings.Contains(h.system("classify", nil), "jev_decide") {
		t.Fatal(h.system("classify", nil))
	}
}

func TestAskJevRuntimeErrorTurnContinues(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	dir := t.TempDir()
	script := &provider.Script{Replies: []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "c1", Name: "ask_jev", Arguments: `{"state":"x","question":"ok?","mode":"boolean"}`}}},
		{Role: provider.RoleAssistant, Content: "jev failed; I will keep going"},
	}}
	set := tools.New(tools.Env{Root: dir})
	h := New(Options{
		Provider:  script,
		FastModel: "fast",
		Policy:    perms.Policy{Mode: perms.ModeYolo, Allow: []string{"ask_jev"}},
		Gates:     jev.Gates{},
		Tools:     set,
		MaxSteps:  4,
	})
	set.Env.Ask = func(context.Context, any, []jev.Query) (jev.Result, error) {
		return jev.Result{
			Error: "jev http 504: timeout",
			Answers: map[string]jev.Answer{
				"q": {Mode: jev.ModeBoolean, Detail: jev.FailedDetail},
			},
		}, nil
	}
	sess, err := session.Create(dir, "ask-err", "t")
	if err != nil {
		t.Fatal(err)
	}
	var askText, done string
	if err := h.Run(context.Background(), sess, "check", func(ev Event) {
		if ev.Kind == EvJev && ev.Name == "ask" {
			askText = ev.Text
		}
		if ev.Kind == EvDone {
			done = ev.Text
		}
	}); err != nil {
		t.Fatal(err)
	}
	if done != "end_turn" {
		t.Fatal(done)
	}
	if !strings.Contains(askText, "error=") || strings.Contains(askText, "q=0") {
		t.Fatalf("ask event %q", askText)
	}
	var res jev.Result
	if err := json.Unmarshal([]byte(toolResult(sess, "ask_jev")), &res); err != nil {
		t.Fatal(err)
	}
	if res.Error == "" || res.Answers["q"].Value != nil {
		t.Fatalf("%#v", res)
	}
}

func toolResult(sess *session.Session, name string) string {
	for i := len(sess.Messages) - 1; i >= 0; i-- {
		if sess.Messages[i].Role == provider.RoleTool && sess.Messages[i].Name == name {
			return sess.Messages[i].Content
		}
	}
	return ""
}
