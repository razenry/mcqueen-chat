package ai

import (
	"context"
	"encoding/json"
)

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	Name       string     `json:"name,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolDefinition struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

type FunctionDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// Provider mempertahankan kemampuan chat biasa.
type Provider interface {
	Chat(
		ctx context.Context,
		messages []Message,
	) (string, error)
}

// ToolProvider mendukung AI yang dapat meminta eksekusi tools.
type ToolProvider interface {
	ChatWithTools(
		ctx context.Context,
		messages []Message,
		tools []ToolDefinition,
	) (Message, error)
}
