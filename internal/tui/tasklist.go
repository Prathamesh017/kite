package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"kite/internal/llm"
)

// Task statuses shown in the list.
const (
	taskPending = "pending"
	taskRunning = "running"
	taskPassed  = "passed"
	taskFailed  = "failed"
)

// TaskList shows the tasks of a task-mode reply while they run, each with its
// status. It sits below Activity and is cleared once the run's summary is
// appended to Activity.
type TaskList struct {
	rows []taskRow
}

type taskRow struct {
	task   llm.Task
	status string
	answer string
}

// Start fills the list with every task pending.
func (t *TaskList) Start(tasks []llm.Task) {
	t.rows = make([]taskRow, len(tasks))
	for i, task := range tasks {
		t.rows[i] = taskRow{task: task, status: taskPending}
	}
}

func (t *TaskList) Running(i int) {
	t.rows[i].status = taskRunning
}

func (t *TaskList) Finish(i int, answer string, passed bool) {
	t.rows[i].answer = answer
	t.rows[i].status = taskFailed
	if passed {
		t.rows[i].status = taskPassed
	}
}

func (t *TaskList) Clear() {
	t.rows = nil
}

func (t TaskList) Active() bool { return len(t.rows) > 0 }
func (t TaskList) Len() int     { return len(t.rows) }

func (t TaskList) Task(i int) llm.Task { return t.rows[i].task }

// Current is the index of the running task, or -1 when none is running.
func (t TaskList) Current() int {
	for i, row := range t.rows {
		if row.status == taskRunning {
			return i
		}
	}
	return -1
}

// Earlier is the description and answer of every finished task, sent with the
// next task so it can build on them.
func (t TaskList) Earlier() string {
	var b strings.Builder
	for _, row := range t.rows {
		if row.status == taskPassed || row.status == taskFailed {
			fmt.Fprintf(&b, "- %s\n%s\n", row.task.Description, row.answer)
		}
	}
	return b.String()
}

// Summary is the run's final answer: how many passed, then each task with its
// status and answer. Tasks after a failure show as not run.
func (t TaskList) Summary() string {
	passed := 0
	var b strings.Builder
	for i, row := range t.rows {
		status := row.status
		if status == taskPending {
			status = "not run"
		}
		if row.status == taskPassed {
			passed++
		}
		fmt.Fprintf(&b, "\n\n%d. %s (%s)", i+1, row.task.Description, status)
		if row.answer != "" {
			b.WriteString("\n" + row.answer)
		}
	}
	return fmt.Sprintf("%d of %d tasks passed", passed, len(t.rows)) + b.String()
}

// Height is the rows the pane needs: one per task, the title, and the border.
func (t TaskList) Height() int {
	return len(t.rows) + 1 + panelStyle.GetVerticalFrameSize()
}

// View draws one line per task. Long descriptions are cut to the width.
func (t TaskList) View(width, height int) string {
	textW, _ := textSize(width, height)
	line := lipgloss.NewStyle().MaxWidth(textW)

	out := []string{titleStyle.Render("Tasks")}
	for i, row := range t.rows {
		text := fmt.Sprintf("%s  %d. %s", taskStatusStyle(row.status).Render(fmt.Sprintf("%-7s", row.status)), i+1, row.task.Description)
		out = append(out, line.Render(text))
	}
	return box(strings.Join(out, "\n"), width, height)
}

func taskStatusStyle(status string) lipgloss.Style {
	switch status {
	case taskRunning:
		return titleStyle
	case taskPassed:
		return readyStyle
	case taskFailed:
		return brandStyle
	}
	return mutedStyle
}
