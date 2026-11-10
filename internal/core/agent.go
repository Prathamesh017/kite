package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
)

type Agent struct {
	llm     LLM       // any LLM adapter can be used here, directly replaceable
	tools   Tools     // any tools adapter can be used here, directly replaceable
	history []Message // history of messages to be sent to LLM
	run     *run      // the question being answered, nil when there is none
}

// run is the state of one question, from Ask until the answer. Ask creates it
// fresh, so the counters always start at 0.
type run struct {
	messages       []Message  // the history, this question, and everything since
	toolCallsToRun []ToolCall // tool calls the model asked for that have not run yet
	tasks          []Task     // the tasks the model broke the request into
	iterations     int        // model replies that asked for tools in this run
	toolCalls      int        // tools run in this run
}

func NewAgent(llm LLM, tools Tools) *Agent {
	return &Agent{
		llm:     llm,
		history: []Message{},
		tools:   tools,
	}
}

// Ask starts a run for a question and does its first step.
func (a *Agent) Ask(ctx context.Context, question string) (Result, error) {
	a.run = &run{messages: append(slices.Clone(a.history), Message{Role: "user", Message: question})}
	return a.askModel(ctx)
}

// Step continues the run that Ask started, with its next step.
func (a *Agent) Step(ctx context.Context) (Result, error) {
	if a.run == nil {
		return Result{}, errors.New("no question is being answered")
	}
	if len(a.run.tasks) > 0 {
		// Running the tasks is not built yet, so the run ends here.
		return a.stop(fmt.Sprintf("The model split the request into %d tasks. Running tasks is not built yet.", len(a.run.tasks))), nil
	}
	if len(a.run.toolCallsToRun) > 0 {
		if len(a.run.toolCallsToRun) > toolCallLimit()-a.run.toolCalls {
			// The refused request has no results, so it is not kept. The model would reject a history with it.
			a.run.messages = a.run.messages[:len(a.run.messages)-1]
			return a.stop("Reached its tool-call limit. The task may be partially complete."), nil
		}
		return a.runTools(ctx), nil
	}
	if a.run.iterations >= iterationLimit() {
		return a.stop("Reached its execution-step limit before completing the task."), nil
	}
	return a.askModel(ctx)
}

// stop ends a run that hit a limit. The note is shown as the answer, and the work
// done so far stays in the history, so the user can ask to continue.
func (a *Agent) stop(note string) Result {
	a.history = append(a.run.messages, Message{Role: "assistant", Message: note})
	a.run = nil
	return Result{Kind: Answered, Answer: note}
}

func iterationLimit() int { return envInt("MAX_ITERATIONS", 10) }
func toolCallLimit() int  { return envInt("MAX_TOOL_CALLS", 20) }

// envInt reads a positive whole number from the environment. A missing or
// unreadable value falls back to fallback.
func envInt(key string, fallback int) int {
	if n, err := strconv.Atoi(os.Getenv(key)); err == nil && n > 0 {
		return n
	}
	return fallback
}

// askModel sends the run's messages to the model and reports what it replied.
func (a *Agent) askModel(ctx context.Context) (Result, error) {
	reply, err := a.llm.Complete(ctx, SystemPrompt+"\n\n"+ReplyPrompt, a.run.messages, a.tools.Specs(), replyFormat)
	if err != nil {
		// The run is dropped and the history is untouched, so a failed question never enters it.
		a.run = nil
		return Result{}, err
	}
	mode, parsed := parseReply(reply)

	switch mode {
	case ModeToolCall:
		a.run.iterations++
		a.run.messages = append(a.run.messages, reply)
		a.run.toolCallsToRun = reply.ToolCalls
		return Result{Kind: AskedForTool, Tools: reply.ToolCalls}, nil
	case ModeTask:
		a.run.tasks = parsed.Tasks
		return Result{Kind: TasksCreated, Tasks: parsed.Tasks}, nil
	case ModeAnswer:
		// The history keeps the answer, not the JSON around it.
		a.history = append(a.run.messages, Message{Role: "assistant", Message: parsed.Answer})
		a.run = nil
		return Result{Kind: Answered, Answer: parsed.Answer}, nil
	}
	return Result{}, fmt.Errorf("unknown reply mode %d", mode)
}

// runTools runs the tool calls the model asked for. Each call gets a result
// carrying its ID, because the model expects one answer per request.
func (a *Agent) runTools(ctx context.Context) Result {
	calls := a.run.toolCallsToRun
	for _, call := range calls {
		result := a.tools.Run(ctx, call.Name, call.Arguments)
		a.run.messages = append(a.run.messages, Message{Role: "tool", Message: result, ToolCallID: call.ID})
	}
	a.run.toolCalls += len(calls)
	a.run.toolCallsToRun = nil
	return Result{Kind: ToolRan, Tools: calls}
}
