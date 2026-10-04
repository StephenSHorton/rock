package harness

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestAskJevAndRiskSameTurn(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	dir := t.TempDir()
	script := &provider.Script{Replies: []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{
			{ID: "c1", Name: "ask_jev", Arguments: `{"state":"rm -rf /tmp/nope","question":"too risky?","mode":"boolean"}`},
			{ID: "c2", Name: "shell", Arguments: `{"command":"rm -rf /tmp/nope"}`},
		}},
		{Role: provider.RoleAssistant, Content: "stopped"},
	}}
	set := tools.New(tools.Env{Root: dir})
	h := New(Options{
		Provider:  script,
		FastModel: "fast",
		Policy:    perms.Policy{Mode: perms.ModeYolo, Allow: []string{"ask_jev", "shell"}},
		Gates:     jev.Gates{},
		Tools:     set,
		MaxSteps:  6,
	})
	set.Env.Ask = func(context.Context, any, []jev.Query) (jev.Result, error) {
		return jev.Result{
			Source:  "live",
			Answers: map[string]jev.Answer{"q": {Mode: jev.ModeBoolean, Value: 0.2}},
		}, nil
	}
	sess, err := session.Create(dir, "ask-risk", "t")
	if err != nil {
		t.Fatal(err)
	}
	var askText, riskText string
	if err := h.Run(context.Background(), sess, "clean", func(ev Event) {
		if ev.Kind == EvJev && ev.Name == "ask" {
			askText = ev.Text
		}
		if ev.Kind == EvJev && ev.Name == "risk" {
			riskText = ev.Text
		}
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(askText, "q=0.2") {
		t.Fatalf("ask %q", askText)
	}
	if !strings.Contains(riskText, "block=true") {
		t.Fatalf("risk %q", riskText)
	}
	if strings.Contains(riskText, "q=0.2") {
		t.Fatalf("risk mixed ask answers: %q", riskText)
	}
	if !strings.Contains(toolResult(sess, "shell"), "denied") {
		t.Fatal(toolResult(sess, "shell"))
	}
}

func TestNudgeAfterEditWithoutAskJev(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := &provider.Script{Replies: []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "c1", Name: "edit_file", Arguments: `{"path":"a.txt","old":"world","new":"rock"}`}}},
		{Role: provider.RoleAssistant, Content: "renamed the greeting"},
	}}
	cap := &captureProvider{Script: script}
	var asks int
	set := tools.New(tools.Env{Root: dir, PlanPath: filepath.Join(dir, "plan.md")})
	h := New(Options{
		Provider:  cap,
		FastModel: "fast",
		Policy:    perms.Policy{Mode: perms.ModeYolo, Allow: []string{"edit_file"}},
		Gates:     jev.Gates{},
		Tools:     set,
		MaxSteps:  4,
	})
	set.Env.Ask = func(context.Context, any, []jev.Query) (jev.Result, error) {
		asks++
		return jev.Result{Error: "should not run"}, nil
	}
	sess, err := session.Create(dir, "nudge-edit", "t")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Run(context.Background(), sess, "change world to rock", nil); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "a.txt"))
	if string(got) != "hello rock" {
		t.Fatal(string(got))
	}
	if asks != 0 {
		t.Fatalf("ask_jev ran %d times", asks)
	}
	edit := toolResult(sess, "edit_file")
	if !strings.Contains(edit, "edited a.txt") || !strings.Contains(edit, nudgeEditHint) {
		t.Fatalf("edit result %q", edit)
	}
	if !sawHintOnComplete(cap.seen, nudgeEditHint) {
		t.Fatal("nudge missing from the next complete")
	}
	if !strings.Contains(h.system("fix", nil), "You decide whether to call it") {
		t.Fatal(h.system("fix", nil))
	}
}

func TestNudgeThenAskJev(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := &provider.Script{Replies: []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "c1", Name: "edit_file", Arguments: `{"path":"a.txt","old":"broken","new":"fixed"}`}}},
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "c2", Name: "ask_jev", Arguments: `{"state":"edited a.txt broken→fixed","question":"is the failure type resolved?","mode":"boolean"}`}}},
		{Role: provider.RoleAssistant, Content: "verified"},
	}}
	cap := &captureProvider{Script: script}
	var asks int
	set := tools.New(tools.Env{Root: dir})
	h := New(Options{
		Provider:  cap,
		FastModel: "fast",
		Policy:    perms.Policy{Mode: perms.ModeYolo, Allow: []string{"edit_file", "ask_jev"}},
		Gates:     jev.Gates{},
		Tools:     set,
		MaxSteps:  6,
	})
	set.Env.Ask = func(_ context.Context, _ any, qs []jev.Query) (jev.Result, error) {
		asks++
		return jev.Result{
			Source: "live",
			Model:  "fake",
			Answers: map[string]jev.Answer{
				"q": {Mode: jev.ModeBoolean, Value: 0.88},
			},
		}, nil
	}
	sess, err := session.Create(dir, "nudge-ask", "t")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Run(context.Background(), sess, "fix it", nil); err != nil {
		t.Fatal(err)
	}
	if asks != 1 {
		t.Fatalf("ask_jev calls %d", asks)
	}
	if !strings.Contains(toolResult(sess, "edit_file"), nudgeEditHint) {
		t.Fatal(toolResult(sess, "edit_file"))
	}
	var res jev.Result
	if err := json.Unmarshal([]byte(toolResult(sess, "ask_jev")), &res); err != nil {
		t.Fatal(err)
	}
	if res.Error != "" || res.Answers["q"].Value != 0.88 {
		t.Fatalf("%#v", res)
	}
	if !sawHintOnComplete(cap.seen, `"value":0.88`) && !sawHintOnComplete(cap.seen, `"value": 0.88`) {
		// The model must see the ask_jev payload on the complete after it.
		found := false
		for _, msgs := range cap.seen {
			blob := messagesBlob(msgs)
			if strings.Contains(blob, "0.88") && strings.Contains(blob, "ask_jev") {
				found = true
			}
		}
		if !found {
			t.Fatalf("ask_jev result not on a later complete: %#v", cap.seen)
		}
	}
	if sess.LastAssistant() != "verified" {
		t.Fatal(sess.LastAssistant())
	}
}

func TestNudgeDoesNotBypassRisk(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	dir := t.TempDir()
	script := &provider.Script{Replies: []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "c1", Name: "ask_jev", Arguments: `{"state":"rm -rf /tmp/nope","question":"too risky?","mode":"boolean"}`}}},
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "c2", Name: "shell", Arguments: `{"command":"rm -rf /tmp/nope"}`}}},
		{Role: provider.RoleAssistant, Content: "stopped"},
	}}
	set := tools.New(tools.Env{Root: dir})
	h := New(Options{
		Provider:  script,
		FastModel: "fast",
		Policy:    perms.Policy{Mode: perms.ModeYolo, Allow: []string{"ask_jev", "shell"}},
		Gates:     jev.Gates{},
		Tools:     set,
		MaxSteps:  6,
	})
	set.Env.Ask = func(context.Context, any, []jev.Query) (jev.Result, error) {
		return jev.Result{
			Source:  "live",
			Answers: map[string]jev.Answer{"q": {Mode: jev.ModeBoolean, Value: 0.2}},
		}, nil
	}
	sess, err := session.Create(dir, "nudge-risk", "t")
	if err != nil {
		t.Fatal(err)
	}
	var risk string
	if err := h.Run(context.Background(), sess, "clean", func(ev Event) {
		if ev.Kind == EvJev && ev.Name == "risk" {
			risk = ev.Text
		}
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(risk, "block=true") {
		t.Fatalf("risk %q", risk)
	}
	if !strings.Contains(toolResult(sess, "shell"), "denied") {
		t.Fatal(toolResult(sess, "shell"))
	}
	if strings.Contains(toolResult(sess, "shell"), "Hint:") {
		t.Fatal("denied shell must not carry a nudge")
	}
}

func TestNudgeWhenAskJevUnwired(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := &provider.Script{Replies: []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "c1", Name: "edit_file", Arguments: `{"path":"a.txt","old":"x","new":"y"}`}}},
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "c2", Name: "ask_jev", Arguments: `{"state":"y","question":"resolved?","mode":"boolean"}`}}},
		{Role: provider.RoleAssistant, Content: "kept going"},
	}}
	h := New(Options{
		Provider:  script,
		FastModel: "fast",
		Policy:    perms.Policy{Mode: perms.ModeYolo, Allow: []string{"edit_file", "ask_jev"}},
		Gates:     jev.Gates{},
		Tools:     tools.New(tools.Env{Root: dir}),
		MaxSteps:  6,
	})
	sess, err := session.Create(dir, "nudge-offline", "t")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Run(context.Background(), sess, "fix", nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(toolResult(sess, "edit_file"), nudgeEditHint) {
		t.Fatal(toolResult(sess, "edit_file"))
	}
	var res jev.Result
	if err := json.Unmarshal([]byte(toolResult(sess, "ask_jev")), &res); err != nil {
		t.Fatal(err)
	}
	if res.Error == "" || res.Answers["q"].Value != nil {
		t.Fatalf("unwired ask_jev must not fabricate %#v", res)
	}
}

func TestNudgeRateLimitAndOff(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTwoEdits := func(every int, id string) (first, second string) {
		script := &provider.Script{Replies: []provider.Message{
			{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{
				{ID: "c1", Name: "edit_file", Arguments: `{"path":"a.txt","old":"one","new":"alpha"}`},
				{ID: "c2", Name: "edit_file", Arguments: `{"path":"b.txt","old":"two","new":"beta"}`},
			}},
			{Role: provider.RoleAssistant, Content: "done"},
		}}
		h := New(Options{
			Provider:   script,
			FastModel:  "fast",
			Policy:     perms.Policy{Mode: perms.ModeYolo, Allow: []string{"edit_file"}},
			Gates:      jev.Gates{},
			Tools:      tools.New(tools.Env{Root: dir}),
			MaxSteps:   4,
			NudgeEvery: every,
		})
		sess, err := session.Create(dir, id, "t")
		if err != nil {
			t.Fatal(err)
		}
		if err := h.Run(context.Background(), sess, "edit both", nil); err != nil {
			t.Fatal(err)
		}
		var edits []string
		for _, m := range sess.Messages {
			if m.Role == provider.RoleTool && m.Name == "edit_file" {
				edits = append(edits, m.Content)
			}
		}
		if len(edits) != 2 {
			t.Fatalf("edits %#v", edits)
		}
		return edits[0], edits[1]
	}

	first, second := runTwoEdits(2, "nudge-rate-on")
	if !strings.Contains(first, nudgeEditHint) {
		t.Fatalf("first should hint: %q", first)
	}
	if strings.Contains(second, "Hint:") {
		t.Fatalf("second should be silent at every=2: %q", second)
	}

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, second = runTwoEdits(-1, "nudge-rate-off")
	if strings.Contains(first, "Hint:") || strings.Contains(second, "Hint:") {
		t.Fatalf("disabled: %q %q", first, second)
	}
}

func TestNudgeSkipsFailedEditAndDenied(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := &provider.Script{Replies: []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "c1", Name: "edit_file", Arguments: `{"path":"a.txt","old":"missing","new":"x"}`}}},
		{Role: provider.RoleAssistant, Content: "missed"},
	}}
	h := New(Options{
		Provider:   script,
		FastModel:  "fast",
		Policy:     perms.Policy{Mode: perms.ModeYolo, Allow: []string{"edit_file"}},
		Gates:      jev.Gates{},
		Tools:      tools.New(tools.Env{Root: dir}),
		NudgeEvery: 1,
	})
	sess, err := session.Create(dir, "nudge-fail", "t")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Run(context.Background(), sess, "edit", nil); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(toolResult(sess, "edit_file"), "Hint:") {
		t.Fatal(toolResult(sess, "edit_file"))
	}
}

func TestNudgeAfterAllowedShell(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	dir := t.TempDir()
	script := &provider.Script{Replies: []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "c1", Name: "shell", Arguments: `{"command":"echo ok"}`}}},
		{Role: provider.RoleAssistant, Content: "ran"},
	}}
	h := New(Options{
		Provider:  script,
		FastModel: "fast",
		Policy:    perms.Policy{Mode: perms.ModeYolo, Allow: []string{"shell"}},
		Gates:     jev.Gates{},
		Tools:     tools.New(tools.Env{Root: dir}),
		MaxSteps:  4,
	})
	sess, err := session.Create(dir, "nudge-shell", "t")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Run(context.Background(), sess, "echo", nil); err != nil {
		t.Fatal(err)
	}
	got := toolResult(sess, "shell")
	if !strings.Contains(got, "ok") || !strings.Contains(got, nudgeShellHint) {
		t.Fatalf("shell result %q", got)
	}
}

type captureProvider struct {
	*provider.Script
	seen [][]provider.Message
}

func (c *captureProvider) Complete(ctx context.Context, model string, msgs []provider.Message, specs []provider.ToolSpec) (provider.Message, error) {
	cp := make([]provider.Message, len(msgs))
	copy(cp, msgs)
	c.seen = append(c.seen, cp)
	return c.Script.Complete(ctx, model, msgs, specs)
}

func sawHintOnComplete(seen [][]provider.Message, hint string) bool {
	for _, msgs := range seen {
		if strings.Contains(messagesBlob(msgs), hint) {
			return true
		}
	}
	return false
}

func messagesBlob(msgs []provider.Message) string {
	var b strings.Builder
	for _, m := range msgs {
		b.WriteString(m.Content)
		b.WriteByte('\n')
	}
	return b.String()
}

func toolResult(sess *session.Session, name string) string {
	for i := len(sess.Messages) - 1; i >= 0; i-- {
		if sess.Messages[i].Role == provider.RoleTool && sess.Messages[i].Name == name {
			return sess.Messages[i].Content
		}
	}
	return ""
}

func TestClassifyFailedShell(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	dir := t.TempDir()
	script := &provider.Script{Replies: []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "c1", Name: "shell", Arguments: `{"command":"printf 'FAIL: TestAdd\\nassert want 2 got 1\\n'; exit 1"}`}}},
		{Role: provider.RoleAssistant, Content: "classified"},
	}}
	set := tools.New(tools.Env{Root: dir})
	h := New(Options{
		Provider:  script,
		FastModel: "fast",
		Policy:    perms.Policy{Mode: perms.ModeYolo, Allow: []string{"shell"}},
		Gates:     jev.Gates{},
		Tools:     set,
		MaxSteps:  4,
	})
	var state any
	var qs []jev.Query
	set.Env.Ask = func(_ context.Context, st any, questions []jev.Query) (jev.Result, error) {
		state, qs = st, questions
		return jev.Result{
			Source: "live",
			Answers: map[string]jev.Answer{
				"kind":  {Mode: jev.ModeChoice, Value: "test"},
				"area":  {Mode: jev.ModeChoice, Value: "code"},
				"retry": {Mode: jev.ModeBoolean, Value: 0.12},
			},
		}, nil
	}
	sess, err := session.Create(dir, "classify", "t")
	if err != nil {
		t.Fatal(err)
	}
	var class Event
	if err := h.Run(context.Background(), sess, "run tests", func(ev Event) {
		if ev.Kind == EvClassify {
			class = ev
		}
	}); err != nil {
		t.Fatal(err)
	}
	st, _ := state.(map[string]any)
	if !strings.Contains(fmt.Sprint(st["log"]), "FAIL: TestAdd") {
		t.Fatalf("log not in state %#v", state)
	}
	if len(qs) != 3 || qs[0].Name != "kind" || qs[1].Name != "area" || qs[2].Name != "retry" {
		t.Fatalf("%#v", qs)
	}
	got := toolResult(sess, "shell")
	if !strings.Contains(got, "kind=test") || !strings.Contains(got, "area=code") {
		t.Fatal(got)
	}
	if class.Text == "" || !strings.Contains(class.Text, "kind=test") {
		t.Fatalf("event %#v", class)
	}
}

func TestClassifyOfflineDoesNotInvent(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	dir := t.TempDir()
	script := &provider.Script{Replies: []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "c1", Name: "shell", Arguments: `{"command":"printf fail; exit 1"}`}}},
		{Role: provider.RoleAssistant, Content: "kept going"},
	}}
	h := New(Options{
		Provider:  script,
		FastModel: "fast",
		Policy:    perms.Policy{Mode: perms.ModeYolo, Allow: []string{"shell"}},
		Gates:     jev.Gates{},
		Tools:     tools.New(tools.Env{Root: dir}),
		MaxSteps:  4,
	})
	sess, err := session.Create(dir, "classify-off", "t")
	if err != nil {
		t.Fatal(err)
	}
	var class Event
	if err := h.Run(context.Background(), sess, "run", func(ev Event) {
		if ev.Kind == EvClassify {
			class = ev
		}
	}); err != nil {
		t.Fatal(err)
	}
	if class.Text != "" {
		t.Fatalf("offline invented a class: %#v", class)
	}
	if strings.Contains(toolResult(sess, "shell"), "Jev classify:") {
		t.Fatal(toolResult(sess, "shell"))
	}
}

func TestTriageOffSkipsClassify(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	dir := t.TempDir()
	script := &provider.Script{Replies: []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "c1", Name: "shell", Arguments: `{"command":"printf fail; exit 1"}`}}},
		{Role: provider.RoleAssistant, Content: "ok"},
	}}
	set := tools.New(tools.Env{Root: dir})
	h := New(Options{
		Provider:  script,
		FastModel: "fast",
		Policy:    perms.Policy{Mode: perms.ModeYolo, Allow: []string{"shell"}},
		Gates:     jev.Gates{},
		Tools:     set,
		TriageOff: true,
		MaxSteps:  4,
	})
	var asks int
	set.Env.Ask = func(context.Context, any, []jev.Query) (jev.Result, error) {
		asks++
		return jev.Result{}, nil
	}
	sess, err := session.Create(dir, "triage-off", "t")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Run(context.Background(), sess, "run", nil); err != nil {
		t.Fatal(err)
	}
	if asks != 0 {
		t.Fatalf("triage off still asked %d", asks)
	}
}

func TestGrepFilterEventCounts(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("keep hit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("drop hit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := &provider.Script{Replies: []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "c1", Name: "grep", Arguments: `{"pattern":"hit"}`}}},
		{Role: provider.RoleAssistant, Content: "filtered"},
	}}
	set := tools.New(tools.Env{Root: dir})
	h := New(Options{
		Provider:  script,
		FastModel: "fast",
		Policy:    perms.Policy{Mode: perms.ModeYolo, Allow: []string{"grep"}},
		Gates:     jev.Gates{},
		Tools:     set,
		MaxSteps:  4,
	})
	set.Env.FilterSnippets = func(_ string, snips []string) []string {
		var out []string
		for _, s := range snips {
			if strings.Contains(s, "keep") {
				out = append(out, s)
			}
		}
		return out
	}
	sess, err := session.Create(dir, "filter-ev", "t")
	if err != nil {
		t.Fatal(err)
	}
	var fil Event
	if err := h.Run(context.Background(), sess, "search", func(ev Event) {
		if ev.Kind == EvFilter {
			fil = ev
		}
	}); err != nil {
		t.Fatal(err)
	}
	if fil.ClipsBefore != 2 || fil.ClipsAfter != 1 || fil.BytesBefore <= fil.BytesAfter {
		t.Fatalf("%#v", fil)
	}
	got := toolResult(sess, "grep")
	if !strings.Contains(got, "Jev filter: clips 2→1") {
		t.Fatal(got)
	}
	if !strings.Contains(h.system("x", nil), "Jev does not open files") {
		t.Fatal(h.system("x", nil))
	}
}

func TestAskJevPathsInPlanMode(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := &provider.Script{Replies: []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "c1", Name: "ask_jev", Arguments: `{"paths":["a.go"],"question":"keep?","mode":"boolean"}`}}},
		{Role: provider.RoleAssistant, Content: "ok"},
	}}
	set := tools.New(tools.Env{Root: dir})
	h := New(Options{
		Provider:  script,
		FastModel: "fast",
		Policy:    perms.Policy{Mode: perms.ModePlan, Allow: []string{"ask_jev"}},
		Gates:     jev.Gates{},
		Tools:     set,
		MaxSteps:  4,
	})
	set.Env.Ask = func(context.Context, any, []jev.Query) (jev.Result, error) {
		return jev.Result{Source: "live", Answers: map[string]jev.Answer{"q": {Mode: jev.ModeBoolean, Value: 0.4}}}, nil
	}
	sess, err := session.Create(dir, "plan-paths", "t")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Run(context.Background(), sess, "scan", nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(toolResult(sess, "ask_jev"), `"value":0.4`) && !strings.Contains(toolResult(sess, "ask_jev"), "0.4") {
		t.Fatal(toolResult(sess, "ask_jev"))
	}
}
