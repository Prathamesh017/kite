// internal/core/llm.go
package core

import (
	"context"
)

// ToolCall is the model asking to run one tool. Arguments is a JSON string.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

type Message struct {
	Role      string //  "user", "assistant"
	Message   string
	ToolCalls []ToolCall // set when the model asks to run tools instead of answering
}

/*
His
*/
type LLM interface {
	Complete(ctx context.Context, systemPrompt string, history []Message, tools []ToolSpec) (Message, error)
}
