package core

import (
	"context"
)

type Agent struct {
	llm     LLM       // any LLM adapter can be used here, directly replaceable
	history []Message // history of messages to be sent to LLM
}

func NewAgent(llm LLM) *Agent {
	return &Agent{
		llm:     llm,
		history: []Message{},
	}
}

const systemPrompt = "You are a helpful assistant. Answer the questions as best as you can."

func (a *Agent) Ask(ctx context.Context, question string) (string, error) {
	a.history = append(a.history, Message{Role: "user", Message: question})
	reply, err := a.llm.Complete(ctx, systemPrompt, a.history)
	if err != nil {
		return "", err
	}

	a.history = append(a.history, Message{Role: "assistant", Message: reply})
	return reply, nil
}
