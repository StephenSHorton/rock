package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAI speaks chat completions, including tool calls. Any host that matches
// that shape works: set BaseURL.
type OpenAI struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

func (o *OpenAI) Name() string { return "openai" }

type oaMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	Name       string     `json:"name,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []oaCall   `json:"tool_calls,omitempty"`
}

type oaCall struct {
	ID       string     `json:"id"`
	Type     string     `json:"type"`
	Function oaFunction `json:"function"`
}

type oaFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type oaTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

func (o *OpenAI) Complete(ctx context.Context, model string, messages []Message, tools []ToolSpec) (Message, error) {
	if o.HTTP == nil {
		o.HTTP = &http.Client{Timeout: 120 * time.Second}
	}
	base := strings.TrimRight(o.BaseURL, "/")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	body := map[string]any{
		"model":    model,
		"messages": toOA(messages),
	}
	if len(tools) > 0 {
		ot := make([]oaTool, 0, len(tools))
		for _, t := range tools {
			params := t.Parameters
			if params == nil {
				params = map[string]any{"type": "object", "properties": map[string]any{}}
			}
			var item oaTool
			item.Type = "function"
			item.Function.Name = t.Name
			item.Function.Description = t.Description
			item.Function.Parameters = params
			ot = append(ot, item)
		}
		body["tools"] = ot
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return Message{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return Message{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if o.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.APIKey)
	}
	res, err := o.HTTP.Do(req)
	if err != nil {
		return Message{}, err
	}
	defer res.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return Message{}, err
	}
	if res.StatusCode >= 300 {
		return Message{}, fmt.Errorf("model http %d: %s", res.StatusCode, truncate(string(payload), 400))
	}
	var parsed struct {
		Choices []struct {
			Message oaMessage `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return Message{}, err
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return Message{}, fmt.Errorf("model: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return Message{}, fmt.Errorf("model returned no choices")
	}
	return fromOA(parsed.Choices[0].Message), nil
}

func toOA(in []Message) []oaMessage {
	out := make([]oaMessage, 0, len(in))
	for _, m := range in {
		msg := oaMessage{Role: string(m.Role), Content: m.Content, Name: m.Name, ToolCallID: m.ToolCallID}
		for _, c := range m.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, oaCall{
				ID:   c.ID,
				Type: "function",
				Function: oaFunction{
					Name:      c.Name,
					Arguments: c.Arguments,
				},
			})
		}
		out = append(out, msg)
	}
	return out
}

func fromOA(m oaMessage) Message {
	msg := Message{Role: Role(m.Role), Content: m.Content, Name: m.Name, ToolCallID: m.ToolCallID}
	for _, c := range m.ToolCalls {
		msg.ToolCalls = append(msg.ToolCalls, ToolCall{
			ID:        c.ID,
			Name:      c.Function.Name,
			Arguments: c.Function.Arguments,
		})
	}
	return msg
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
