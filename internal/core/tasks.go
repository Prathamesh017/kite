package core

import (
	"context"
	"fmt"
	"maps"
	"path"
	"path/filepath"
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

// existingFiles lists the files the tasks will create that already exist, so the
// run can be refused before it starts. It asks the tools for every file in the
// workspace once. If that fails, nothing is refused.
func (a *Agent) existingFiles(ctx context.Context, tasks []Task) []string {
	var wanted []string
	for _, task := range tasks {
		wanted = append(wanted, task.Creates...)
	}
	if len(wanted) == 0 {
		return nil
	}

	listing := a.tools.Run(ctx, "list_files", "{}")
	if strings.HasPrefix(listing, "error:") {
		return nil
	}
	present := map[string]bool{}
	for _, line := range strings.Split(listing, "\n") {
		present[path.Clean(filepath.ToSlash(line))] = true
	}

	var existing []string
	for _, file := range wanted {
		if present[path.Clean(filepath.ToSlash(file))] && !slices.Contains(existing, file) {
			existing = append(existing, file)
		}
	}
	return existing
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

	// A task answers in plain text, so there is no reply format. A check answers with a verdict.
	var format *ReplyFormat
	if r.checking {
		format = verdictFormat
	}
	if err := waitToRetry(ctx, r); err != nil {
		a.run = nil
		return Result{}, err
	}
	reply, err := a.llm.Complete(ctx, SystemPrompt, r.messages, a.tools.Specs(), format)
	if err != nil {
		if ctx.Err() != nil {
			// Cancel and timeouts are for the caller to handle.
			a.run = nil
			return Result{}, err
		}
		if result, ok := retryLater(ctx, r, err); ok {
			return result, nil
		}
		return a.finishTask(false, "The request failed: "+err.Error()), nil
	}
	r.retries = 0
	if len(reply.ToolCalls) > 0 {
		r.iterations++
		r.messages = append(r.messages, reply)
		r.toolCallsToRun = reply.ToolCalls
		return Result{Kind: AskedForTool, Tools: reply.ToolCalls}, nil
	}

	// The model's reply ends the task, or the check of it.
	if r.checking {
		return a.judgeCheck(r, reply.Message), nil
	}
	return a.judgeTask(r, markCutOff(reply.Message, reply.Truncated)), nil
}

// judgeTask decides whether a task passed, from the model's plain reply and the task's verification policy.
func (a *Agent) judgeTask(r *run, answer string) Result {
	policy := a.run.tasks[a.run.current].task.VerificationPolicy
	if policy != policyNone && len(r.toolErrors) > 0 {
		return a.finishTask(false, "A tool failed: "+failedTools(r)+"\n"+answer)
	}
	if policy == policyReadBack {
		return a.startCheck()
	}
	return a.finishTask(true, answer)
}

// startCheck begins the read_back check of the current task, in a run of its own.
func (a *Agent) startCheck() Result {
	i := a.run.current
	a.run.task = &run{
		messages:   []Message{{Role: "user", Message: a.checkPrompt(i)}},
		toolErrors: map[string]string{},
		checking:   true,
	}
	return Result{Kind: TaskChecking, Task: i}
}

// checkPrompt asks the model to read back what the task created and give a yes or no.
func (a *Agent) checkPrompt(i int) string {
	task := a.run.tasks[i].task
	files := "none listed"
	if len(task.Creates) > 0 {
		files = strings.Join(task.Creates, ", ")
	}
	return "Check the result of this task. Read the files it created with read_file and " +
		"compare them with the acceptance criteria. Do not write files.\n\n" +
		"Task: " + task.Description + "\n\n" +
		"Acceptance criteria:\n- " + strings.Join(task.AcceptanceCriteria, "\n- ") + "\n\n" +
		"Files it created: " + files + "\n\n" +
		"Reply with passed true or false. In message, say in one line what the files hold, or why the task fails."
}

// judgeCheck ends the task from the check's verdict.
func (a *Agent) judgeCheck(r *run, text string) Result {
	verdict, ok := parseVerdict(text)
	switch {
	case !ok:
		return a.finishTask(false, "The read_back check did not return a verdict.")
	case len(r.toolErrors) > 0:
		return a.finishTask(false, "A tool failed during the check: "+failedTools(r))
	}
	return a.finishTask(*verdict.Passed, verdict.Message)
}

// failedTools lists the remembered tool errors of a run.
func failedTools(r *run) string {
	return strings.Join(slices.Sorted(maps.Values(r.toolErrors)), "; ")
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
