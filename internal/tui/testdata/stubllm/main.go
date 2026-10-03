// Command stubllm is a deterministic OpenAI-compatible chat completions
// server for driving the Rock TUI on a real screen. A fresh prompt gets one
// tool call that asks by default. The tool result gets a short markdown
// reply that says whether the call ran.
//
//	go run ./internal/tui/testdata/stubllm -addr 127.0.0.1:18080
//
// Point Rock at it with base_url = "http://127.0.0.1:18080/v1" and any
// non-empty ROCK_API_KEY. A prompt that mentions "shell" gets a shell call.
// Anything else gets an edit_file call that fixes greeting.txt.
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
	name, args := "edit_file", `{"path":"greeting.txt","old":"helo wrold","new":"hello world"}`
	if strings.Contains(prompt, "shell") {
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
	default:
		return "## Ran it\n\nThe call returned:\n\n```\n" + firstLines(result, 6) + "\n```"
	}
}

func firstLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
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
