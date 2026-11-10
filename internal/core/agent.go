package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Agent struct {
	llm     LLM       // any LLM adapter can be used here, directly replaceable
	tools   Tools     // any tools adapter can be used here, directly replaceable
	history []Message // history of messages to be sent to LLM
	run     *run      // the question being answered, nil when there is none
}

// run is the state of one stretch of back and forth with the model: a question,
// or one task of a question. It is created fresh, so the counters always start at 0.
type run struct {
	messages       []Message     // what is sent to the model: for a question the history and everything since, for a task its own messages
	toolCallsToRun []ToolCall    // tool calls the model asked for that have not run yet
	iterations     int           // model replies that asked for tools in this run
	toolCalls      int           // tools run in this run
	retries        int           // times the current request failed and was sent again
	retryWait      time.Duration // how long the next request waits before it is sent again

	// Only in a question the model broke into tasks.
	tasks    []taskRow // every task, with its status and reply
	current  int       // the task being run, or the next one to start
	task     *run      // the current task's own run, nil between tasks
	finished bool      // no task is left to start: the last one ran, or one failed

	// Only in a task's own run.
	toolErrors map[string]string // the last error result of each tool, cleared by a later success
	checking   bool              // this run is the read_back check of the task, not the task itself
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
	if a.run.tasks != nil {
		return a.stepTasks(ctx)
	}
	if len(a.run.toolCallsToRun) > 0 {
		if len(a.run.toolCallsToRun) > toolCallLimit()-a.run.toolCalls {
			// The refused request has no results, so it is not kept. The model would reject a history with it.
			a.run.messages = a.run.messages[:len(a.run.messages)-1]
			return a.stop("Reached its tool-call limit. The task may be partially complete."), nil
		}
		return a.runTools(ctx, a.run), nil
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

// askModel sends the question's messages to the model and reports what it replied.
func (a *Agent) askModel(ctx context.Context) (Result, error) {
	if err := waitToRetry(ctx, a.run); err != nil {
		a.run = nil
		return Result{}, err
	}
	reply, err := a.llm.Complete(ctx, SystemPrompt+"\n\n"+ReplyPrompt, a.run.messages, a.tools.Specs(), replyFormat)
	if err != nil {
		if result, ok := retryLater(ctx, a.run, err); ok {
			return result, nil
		}
		// The run is dropped and the history is untouched, so a failed question never enters it.
		a.run = nil
		return Result{}, err
	}
	a.run.retries = 0
	mode, parsed := parseReply(reply)

	switch mode {
	case ModeToolCall:
		a.run.iterations++
		a.run.messages = append(a.run.messages, reply)
		a.run.toolCallsToRun = reply.ToolCalls
		return Result{Kind: AskedForTool, Tools: reply.ToolCalls}, nil
	case ModeTask:
		if existing := a.existingFiles(ctx, parsed.Tasks); len(existing) > 0 {
			return a.stop("Not started: " + strings.Join(existing, ", ") +
				" already exists, and Kite can only create new files."), nil
		}
		a.run.tasks = newTaskRows(parsed.Tasks)
		return Result{Kind: TasksCreated, Tasks: parsed.Tasks}, nil
	case ModeAnswer:
		// The history keeps the answer, not the JSON around it.
		answer := markCutOff(parsed.Answer, reply.Truncated)
		a.history = append(a.run.messages, Message{Role: "assistant", Message: answer})
		a.run = nil
		return Result{Kind: Answered, Answer: answer}, nil
	}
	return Result{}, fmt.Errorf("unknown reply mode %d", mode)
}

// runTools runs the tool calls the model asked for in r. Each call gets a result
// carrying its ID, because the model expects one answer per request.
func (a *Agent) runTools(ctx context.Context, r *run) Result {
	calls := r.toolCallsToRun
	for _, call := range calls {
		result := a.tools.Run(ctx, call.Name, call.Arguments)
		r.messages = append(r.messages, Message{Role: "tool", Message: result, ToolCallID: call.ID})
		if r.toolErrors != nil {
			recordToolError(r.toolErrors, call.Name, result)
		}
	}
	r.toolCalls += len(calls)
	r.toolCallsToRun = nil
	return Result{Kind: ToolRan, Tools: calls}
}
