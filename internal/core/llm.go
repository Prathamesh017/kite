// internal/core/llm.go
package core

import (
	"context"
)

type Message struct {
	Role    string //  "user", "assistant"
	Message string
}

/*
His
*/
type LLM interface {
	Complete(ctx context.Context, systemPrompt string, history []Message) (string, error)
}
