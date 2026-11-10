package core

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Task statuses.
const (
	taskPending = "pending"
	taskRunning = "running"
	taskPassed  = "passed"
	taskFailed  = "failed"
)

// taskRow is a task with how it went.
type taskRow struct {
	task   Task
	status string
	answer string // the model's reply when the task ended
}

func newTaskRows(tasks []Task) []taskRow {
	rows := make([]taskRow, len(tasks))
	for i, task := range tasks {
		rows[i] = taskRow{task: task, status: taskPending}
	}
	return rows
}

// stepTasks does the next step of a question the model broke into tasks.
func (a *Agent) stepTasks(ctx context.Context) (Result, error) {
	switch {
	case a.run.task != nil:
		return a.stepTask(ctx)
	case a.run.finished:
		return a.endTasks(), nil
	default:
		return a.startTask(), nil
	}
}

// startTask begins the next task. It gets its own run, so its messages and counters start fresh.
func (a *Agent) startTask() Result {
	i := a.run.current
	a.run.tasks[i].status = taskRunning
	a.run.task = &run{
		messages:   []Message{{Role: "user", Message: a.taskPrompt(i)}},
		toolErrors: map[string]string{},
	}
	return Result{Kind: TaskStarted, Task: i}
}

// taskPrompt is all a task sees: its description, and the results of the tasks before it.
func (a *Agent) taskPrompt(i int) string {
	content := "Task: " + a.run.tasks[i].task.Description
	if earlier := a.earlier(); earlier != "" {
		content += "\n\nResults of the earlier tasks:\n" + earlier
	}
	return content
}

// stepTask does the next step of the task being run.
func (a *Agent) stepTask(ctx context.Context) (Result, error) {
	r := a.run.task
	if len(r.toolCallsToRun) > 0 {
		if len(r.toolCallsToRun) > toolCallLimit()-r.toolCalls {
			return a.finishTask(false, "Reached its tool-call limit. The task may be partially complete."), nil
		}
		return a.runTools(ctx, r), nil
	}
	if r.iterations >= iterationLimit() {
		return a.finishTask(false, "Reached its execution-step limit before completing the task."), nil
	}

	// A task answers in plain text, so there is no reply format.
	reply, err := a.llm.Complete(ctx, SystemPrompt, r.messages, a.tools.Specs(), nil)
	if err != nil {
		if ctx.Err() != nil {
			// Cancel and timeouts are for the caller to handle.
			a.run = nil
			return Result{}, err
		}
		return a.finishTask(false, "The request failed: "+err.Error()), nil
	}
	if len(reply.ToolCalls) > 0 {
		r.iterations++
		r.messages = append(r.messages, reply)
		r.toolCallsToRun = reply.ToolCalls
		return Result{Kind: AskedForTool, Tools: reply.ToolCalls}, nil
	}

	// The model's plain reply ends the task. It passed unless a tool failed.
	if len(r.toolErrors) > 0 {
		failed := strings.Join(slices.Sorted(maps.Values(r.toolErrors)), "; ")
		return a.finishTask(false, "A tool failed: "+failed+"\n"+reply.Message), nil
	}
	return a.finishTask(true, reply.Message), nil
}

// finishTask ends the current task. No task starts after a failure, and none is left after the last one.
func (a *Agent) finishTask(passed bool, answer string) Result {
	i := a.run.current
	a.run.tasks[i].answer = answer
	a.run.tasks[i].status = taskFailed
	if passed {
		a.run.tasks[i].status = taskPassed
	}
	a.run.task = nil
	a.run.current++
	a.run.finished = !passed || a.run.current == len(a.run.tasks)
	return Result{Kind: TaskFinished, Task: i, Passed: passed}
}

// endTasks ends the run with a summary of the tasks. The history keeps the
// question and the summary, not the work inside the tasks.
func (a *Agent) endTasks() Result {
	summary := a.summary()
	a.history = append(a.run.messages, Message{Role: "assistant", Message: summary})
	a.run = nil
	return Result{Kind: Answered, Answer: summary}
}

// recordToolError remembers a failed tool result under the tool's name, and
// forgets it when the same tool later succeeds.
func recordToolError(errors map[string]string, name, result string) {
	if strings.HasPrefix(result, "error:") {
		errors[name] = result
	} else {
		delete(errors, name)
	}
}

// earlier is the description and reply of every finished task, sent with the
// next task so it can build on them.
func (a *Agent) earlier() string {
	var b strings.Builder
	for _, row := range a.run.tasks {
		if row.status == taskPassed || row.status == taskFailed {
			fmt.Fprintf(&b, "- %s\n%s\n", row.task.Description, row.answer)
		}
	}
	return b.String()
}

// summary is the run's answer: how many tasks passed, then each task with its
// status and reply. Tasks after a failure show as not run.
func (a *Agent) summary() string {
	passed := 0
	var b strings.Builder
	for i, row := range a.run.tasks {
		status := row.status
		if status == taskPending {
			status = "not run"
		}
		if row.status == taskPassed {
			passed++
		}
		fmt.Fprintf(&b, "\n\n%d. %s (%s)", i+1, row.task.Description, status)
		if row.answer != "" {
			// Indented under its task, so the reply is easy to tell from the task line.
			b.WriteString("\n   " + strings.ReplaceAll(row.answer, "\n", "\n   "))
		}
	}
	return fmt.Sprintf("%d of %d tasks passed", passed, len(a.run.tasks)) + b.String()
}
