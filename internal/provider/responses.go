package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// TokenSource returns a bearer access token. SIWC supplies a refresher.
type TokenSource func(ctx context.Context) (string, error)

// Responses is the Sign in with ChatGPT inference adapter. Preview limits:
// store false, stream true, no system-role input items, no rejected fields.
type Responses struct {
	BaseURL string
	Token   TokenSource
	HTTP    *http.Client
}

func (r *Responses) Name() string { return "chatgpt" }

func (r *Responses) Complete(ctx context.Context, model string, messages []Message, tools []ToolSpec) (Message, error) {
	if r.HTTP == nil {
		r.HTTP = &http.Client{Timeout: 120 * time.Second}
	}
	base := strings.TrimRight(r.BaseURL, "/")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	token := ""
	if r.Token != nil {
		var err error
		token, err = r.Token(ctx)
		if err != nil {
			return Message{}, err
		}
	}
	instructions, input := toResponsesInput(messages)
	body := map[string]any{
		"model":  model,
		"input":  input,
		"store":  false,
		"stream": true,
	}
	if instructions != "" {
		body["instructions"] = instructions
	}
	if len(tools) > 0 {
		body["tools"] = toResponsesTools(tools)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return Message{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/responses", bytes.NewReader(raw))
	if err != nil {
		return Message{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := r.HTTP.Do(req)
	if err != nil {
		return Message{}, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(res.Body, 2<<20))
		return Message{}, fmt.Errorf("model http %d: %s", res.StatusCode, truncate(string(payload), 400))
	}
	return readResponsesStream(res.Body)
}

func toResponsesTools(tools []ToolSpec) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		params := t.Parameters
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out = append(out, map[string]any{
			"type":        "function",
			"name":        t.Name,
			"description": t.Description,
			"parameters":  params,
		})
	}
	return out
}

func toResponsesInput(messages []Message) (instructions string, input []map[string]any) {
	var sys []string
	for _, m := range messages {
		switch m.Role {
		case RoleSystem:
			if strings.TrimSpace(m.Content) != "" {
				sys = append(sys, m.Content)
			}
		case RoleUser:
			input = append(input, map[string]any{
				"type":    "message",
				"role":    "user",
				"content": m.Content,
			})
		case RoleAssistant:
			if len(m.ToolCalls) > 0 {
				for _, c := range m.ToolCalls {
					input = append(input, map[string]any{
						"type":      "function_call",
						"call_id":   c.ID,
						"name":      c.Name,
						"arguments": c.Arguments,
					})
				}
			}
			if strings.TrimSpace(m.Content) != "" {
				input = append(input, map[string]any{
					"type":    "message",
					"role":    "assistant",
					"content": m.Content,
				})
			}
		case RoleTool:
			input = append(input, map[string]any{
				"type":    "function_call_output",
				"call_id": m.ToolCallID,
				"output":  m.Content,
			})
		}
	}
	return strings.Join(sys, "\n\n"), input
}

func readResponsesStream(r io.Reader) (Message, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 2<<20)
	var text strings.Builder
	var calls []ToolCall
	var failed string
	completed := false
	var event string
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "event:") {
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			continue
		}
		typ := event
		if typ == "" {
			if s, _ := ev["type"].(string); s != "" {
				typ = s
			}
		}
		switch typ {
		case "response.output_text.delta":
			if d, ok := ev["delta"].(string); ok {
				text.WriteString(d)
			}
		case "response.failed":
			failed = streamError(ev)
		case "response.completed":
			completed = true
			if msg, ok := messageFromCompleted(ev); ok {
				if msg.Content != "" || len(msg.ToolCalls) > 0 {
					return msg, nil
				}
			}
		}
		event = ""
	}
	if err := sc.Err(); err != nil {
		return Message{}, err
	}
	if failed != "" {
		return Message{}, fmt.Errorf("model: %s", failed)
	}
	if !completed {
		return Message{}, fmt.Errorf("model: stream ended without response.completed")
	}
	return Message{Role: RoleAssistant, Content: text.String(), ToolCalls: calls}, nil
}

func streamError(ev map[string]any) string {
	if resp, ok := ev["response"].(map[string]any); ok {
		if errObj, ok := resp["error"].(map[string]any); ok {
			if c, _ := errObj["code"].(string); c != "" {
				return c
			}
			if m, _ := errObj["message"].(string); m != "" {
				return m
			}
		}
	}
	if errObj, ok := ev["error"].(map[string]any); ok {
		if c, _ := errObj["code"].(string); c != "" {
			return c
		}
	}
	return "response.failed"
}

func messageFromCompleted(ev map[string]any) (Message, bool) {
	resp, _ := ev["response"].(map[string]any)
	if resp == nil {
		return Message{}, false
	}
	out, _ := resp["output"].([]any)
	if out == nil {
		return Message{}, false
	}
	msg := Message{Role: RoleAssistant}
	var text strings.Builder
	for _, item := range out {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		switch m["type"] {
		case "function_call":
			callID, _ := m["call_id"].(string)
			if callID == "" {
				callID, _ = m["id"].(string)
			}
			name, _ := m["name"].(string)
			args, _ := m["arguments"].(string)
			msg.ToolCalls = append(msg.ToolCalls, ToolCall{ID: callID, Name: name, Arguments: args})
		case "message":
			text.WriteString(extractOutputText(m["content"]))
		}
	}
	msg.Content = text.String()
	return msg, true
}

func extractOutputText(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var b strings.Builder
		for _, part := range v {
			m, ok := part.(map[string]any)
			if !ok {
				continue
			}
			if t, _ := m["text"].(string); t != "" {
				b.WriteString(t)
			}
		}
		return b.String()
	}
	return ""
}
