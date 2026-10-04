// Command stubllm is a deterministic OpenAI-compatible chat completions
// server for driving the Rock TUI on a real screen.
//
//	go run ./internal/tui/testdata/stubllm -addr 127.0.0.1:18080
//
// Point Rock at it with base_url = "http://127.0.0.1:18080/v1" and any
// non-empty ROCK_API_KEY.
//
// Prompt routing (first match):
//   - tool result → a short markdown recap
//   - the canned explore task → text only (the child subagent)
//   - mentions "subagent" → spawn_subagent (explore)
//   - "write a plan" / "draft a plan" → update_plan
//   - mentions "shell" → shell ls
//   - anything else → edit_file on greeting.txt
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// childTask is the spawn_subagent prompt. The child harness sends it as
// the user message; respond returns text so the explore agent does not
// try a mutating tool that the child policy would deny.
const childTask = "look at greeting.txt and say what it says"

const planBody = "## Fix greeting.txt\n\n1. Read the file.\n2. Replace `helo wrold` with `hello world`.\n3. Re-read to confirm.\n"

func main() {
	addr := flag.String("addr", "127.0.0.1:18080", "listen address")
	delay := flag.Duration("delay", 600*time.Millisecond, "pause before each reply, so the spinner shows")
	flag.Parse()
	http.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model    string    `json:"model"`
			Messages []message `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Messages) == 0 {
			http.Error(w, `{"error":{"message":"bad request"}}`, http.StatusBadRequest)
			return
		}
		time.Sleep(*delay)
		reply := respond(req.Messages)
		log.Printf("model=%s last=%s -> %s", req.Model, req.Messages[len(req.Messages)-1].Role, summary(reply))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "stub",
			"object":  "chat.completion",
			"model":   req.Model,
			"choices": []any{map[string]any{"index": 0, "message": reply, "finish_reason": "stop"}},
		})
	})
	log.Printf("stubllm on http://%s/v1", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func respond(msgs []message) map[string]any {
	last := msgs[len(msgs)-1]
	if last.Role == "tool" {
		return map[string]any{"role": "assistant", "content": afterTool(last.Content)}
	}
	prompt := ""
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			prompt = strings.ToLower(msgs[i].Content)
			break
		}
	}
	if strings.Contains(prompt, childTask) {
		return map[string]any{"role": "assistant", "content": "greeting.txt is one line: `helo wrold`."}
	}
	name, args := "edit_file", `{"path":"greeting.txt","old":"helo wrold","new":"hello world"}`
	switch {
	case strings.Contains(prompt, "ask jev") || strings.Contains(prompt, "ask_jev") || strings.Contains(prompt, "rounding"):
		name = "ask_jev"
		args = `{
			"state":"rounding: expected 2 got 1.9",
			"questions":[
				{"name":"resolved","question":"Is the rounding failure gone?","mode":"boolean"},
				{"name":"kind","question":"What kind of failure?","mode":"choice","options":{"other":"something else","round":"rounding"}},
				{"name":"risk","question":"How risky is this change?","mode":"score","levels":["safe","caution","dangerous","stop"]},
				{"name":"fail","question":"Did this request reach a dead endpoint?","mode":"boolean"}
			]
		}`
	case strings.Contains(prompt, "subagent"):
		name = "spawn_subagent"
		args = `{"prompt":"` + childTask + `","kind":"explore"}`
	case strings.Contains(prompt, "write a plan") || strings.Contains(prompt, "draft a plan"):
		name = "update_plan"
		raw, _ := json.Marshal(map[string]string{"content": planBody})
		args = string(raw)
	case strings.Contains(prompt, "shell"):
		name, args = "shell", `{"command":"ls -la"}`
	}
	return map[string]any{
		"role":    "assistant",
		"content": "",
		"tool_calls": []any{map[string]any{
			"id":       fmt.Sprintf("call_%d", len(msgs)),
			"type":     "function",
			"function": map[string]any{"name": name, "arguments": args},
		}},
	}
}

func afterTool(result string) string {
	switch {
	case strings.HasPrefix(result, "denied"):
		return "## Skipped\n\nYou denied the call, so **nothing changed**. `greeting.txt` still says `helo wrold`.\n\n" +
			"1. `/permissions` shows why it asked.\n2. Ask again when you want the edit.\n\n" +
			"> Deny is final for this call. The model is told it was denied."
	case strings.HasPrefix(result, "edited"):
		return "## Edited `greeting.txt`\n\nThe typo is gone:\n\n- **before** `helo wrold`\n- **after** `hello world`\n\n" +
			"```go\nfmt.Println(\"hello world\") // what the file says now\n```\n\n" +
			"> Allowed once. The next edit asks again."
	case strings.Contains(result, `"source"`) && strings.Contains(result, `"answers"`):
		return "## Jev answered\n\nThe `ask_jev` batch came back. Read the `◇ jev` marks — boolean, choice, score, and the failed question."
	case strings.HasPrefix(result, "updated plan"):
		return "## Plan written\n\nThe plan is in the pane. `/ready` asks whether it is ready. That does not approve it."
	case strings.Contains(result, "[explore]") || strings.Contains(result, "[plan]") || strings.Contains(result, "[general]"):
		return "## Subagent finished\n\nThe explore agent returned:\n\n" + result
	default:
		return "## Ran it\n\nThe call returned:\n\n```\n" + firstLines(result, 6) + "\n```"
	}
}

func firstLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if n > 0 && len(lines) > n {
		lines = append(lines[:n], "…")
	}
	return strings.Join(lines, "\n")
}

func summary(reply map[string]any) string {
	if calls, ok := reply["tool_calls"].([]any); ok && len(calls) > 0 {
		fn := calls[0].(map[string]any)["function"].(map[string]any)
		return "tool " + fn["name"].(string)
	}
	text, _ := reply["content"].(string)
	first, _, _ := strings.Cut(text, "\n")
	return "text " + first
}
