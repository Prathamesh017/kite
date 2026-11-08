package core

import (
	"context"
	"slices"
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
	user := Message{Role: "user", Message: question}
	reply, err := a.llm.Complete(ctx, systemPrompt, append(slices.Clone(a.history), user))
	if err != nil {
		return "", err
	}

	// The exchange is stored only once it succeeded, so a failed question never enters the history.
	a.history = append(a.history, user, Message{Role: "assistant", Message: reply})
	return reply, nil
}
