package tui

import (
	"context"
	"errors"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"kite/internal/llm"
	"kite/internal/tools"
)

// replyMsg is one model reply: its text, why it stopped, and the tools it
// asked for, which have not run yet. final marks the result of the retries,
// so it is not retried again.
type replyMsg struct {
	response string
	finish   string
	calls    []llm.ToolCall
	err      error
	final    bool
}

// toolsDoneMsg carries the tools that ran and their results, one per call.
// response is the model's text that came with the calls, kept for the history.
type toolsDoneMsg struct {
	response string
	calls    []llm.ToolCall
	results  []llm.Message
}

// cancelledMsg is what a background command returns once its run was cancelled.
type cancelledMsg struct{}

// timedOutMsg is what a background command returns once its run ran out of time.
type timedOutMsg struct{}

// stopped is the message for a run whose context is done: timed out or cancelled.
func stopped(ctx context.Context) tea.Msg {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return timedOutMsg{}
	}
	return cancelledMsg{}
}

// ask sends the history, which ends with a question, a task, or tool results.
// The system prompt is added inside the client. shape is the reply shape asked for.
func ask(ctx context.Context, history []llm.Message, shape llm.Shape) tea.Cmd {
	sent := append([]llm.Message(nil), history...)
	return func() (msg tea.Msg) {
		step := "model request"
		defer failOnPanic(&step, &msg)

		client, err := llm.FromEnv()
		if err != nil {
			return replyMsg{err: err, final: true}
		}
		response, finish, calls, err := client.Complete(ctx, sent, shape)
		if ctx.Err() != nil {
			return stopped(ctx)
		}
		if err != nil {
			return replyMsg{err: err}
		}
		return replyMsg{response: response, finish: finish, calls: calls}
	}
}

// retry sends the same history up to settings.Count more times. Before each
// try it waits a random time up to the backoff cap. It stops early when an error
// cannot be retried.
func retry(ctx context.Context, history []llm.Message, retryConfig llm.Retry, shape llm.Shape) tea.Cmd {
	sent := append([]llm.Message(nil), history...)
	return func() (msg tea.Msg) {
		step := "model request"
		defer failOnPanic(&step, &msg)

		client, err := llm.FromEnv()
		if err != nil {
			return replyMsg{err: err, final: true}
		}
		for attempt := 0; attempt < retryConfig.Count; attempt++ {
			select {
			case <-ctx.Done():
				return stopped(ctx)
			case <-time.After(llm.RandomUpTo(llm.Backoff(attempt, retryConfig))):
			}

			response, finish, calls, err := client.Complete(ctx, sent, shape)
			if ctx.Err() != nil {
				return stopped(ctx)
			}
			//if the response is successful, return the response
			if err == nil {
				return replyMsg{response: response, finish: finish, calls: calls, final: true}
			}
			//if the new eror is retryable, and we have atrtemo
			if !llm.Retryable(err) || attempt == retryConfig.Count-1 {
				return replyMsg{err: err, final: true}
			}
		}
		return replyMsg{err: &llm.APIError{}, final: true}
	}
}

// runTools runs each call by looking its name up in the registry. Every call
// gets a result carrying its ID, because the model expects one answer per request.
// step is updated before each tool, so a panic can name the tool that crashed.
// A tool that already started finishes; the next one does not start after a cancel.
func runTools(ctx context.Context, response string, calls []llm.ToolCall) tea.Cmd {
	return func() (msg tea.Msg) {
		step := "tool call"
		defer failOnPanic(&step, &msg)

		results := make([]llm.Message, 0, len(calls))
		for _, call := range calls {
			if ctx.Err() != nil {
				return stopped(ctx)
			}
			step = "tool call " + call.Function.Name
			result := "error: unknown tool " + call.Function.Name
			if run, ok := tools.Registry[call.Function.Name]; ok {
				result = run(call.Function.Arguments)
			}
			results = append(results, llm.Message{Role: "tool", Content: result, ToolCallID: call.ID})
		}
		if ctx.Err() != nil {
			return stopped(ctx)
		}
		return toolsDoneMsg{response: response, calls: calls, results: results}
	}
}

// failOnPanic is deferred at the top of each background command. If anything
// inside panics, it turns the panic into a failed reply naming the step that
// was running, so Kite keeps running and the question goes back to the input.
// It is final because running the same broken step again would panic again.
func failOnPanic(step *string, msg *tea.Msg) {
	if r := recover(); r != nil {
		*msg = replyMsg{err: fmt.Errorf("%s failed: %v", *step, r), final: true}
	}
}
