package core

import (
	"context"
	"fmt"
	"slices"
)

type Agent struct {
	llm     LLM       // any LLM adapter can be used here, directly replaceable
	tools   Tools     // any tools adapter can be used here, directly replaceable
	history []Message // history of messages to be sent to LLM
}

func NewAgent(llm LLM, tools Tools) *Agent {
	return &Agent{
		llm:     llm,
		history: []Message{},
		tools:   tools,
	}
}

func (a *Agent) Ask(ctx context.Context, question string) (string, error) {
	user := Message{Role: "user", Message: question}
	reply, err := a.llm.Complete(ctx, SystemPrompt, append(slices.Clone(a.history), user), a.tools.Specs())
	if err != nil {
		return "", err
	}

	switch {
	case len(reply.ToolCalls) > 0:

		return "", fmt.Errorf("the model asked for the tool %q, tool calls are not handled yet", reply.ToolCalls[0].Name)
	default:
		a.history = append(a.history, user, reply)
		return reply.Message, nil
	}
}
