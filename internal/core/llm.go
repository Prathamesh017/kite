// internal/core/llm.go
package core

import (
	"context"
	"encoding/json"
)

// ToolCall is the model asking to run one tool. Arguments is a JSON string.
// ExtraContent is provider-specific data, such as Gemini's thought signature.
// The core never reads it. It is kept with the call and given back unchanged.
type ToolCall struct {
	ID           string
	Name         string
	Arguments    string
	ExtraContent json.RawMessage
}

type Message struct {
	Role       string //  "user", "assistant", "tool"
	Message    string
	ToolCalls  []ToolCall // set when the model asks to run tools instead of answering
	ToolCallID string     // set on a "tool" message: the call it answers
}
type ReplyFormat struct {
	Name   string
	Schema json.RawMessage
}

/*
His
*/
type LLM interface {
	Complete(ctx context.Context, systemPrompt string, history []Message, tools []ToolSpec, format *ReplyFormat) (Message, error)
}
