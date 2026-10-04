// Package provider is the model boundary. OpenAI-compatible HTTP is the live
// path. Script is the offline path used when no API key is configured and in tests.
package provider

import "context"

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content,omitempty"`
	Name       string     `json:"name,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type ToolSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
	ReadOnly    bool           `json:"-"`
	Title       string         `json:"-"`
}

type Provider interface {
	Name() string
	Complete(ctx context.Context, model string, messages []Message, tools []ToolSpec) (Message, error)
}
