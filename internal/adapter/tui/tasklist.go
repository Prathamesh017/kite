package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"kite/internal/core"
)

// Task statuses shown in the list.
const (
	taskPending = "pending"
	taskRunning = "running"
	taskPassed  = "passed"
	taskFailed  = "failed"
)

// TaskList shows the tasks the model broke the request into, each with its
// status. It sits below Activity and is cleared when the run ends.
type TaskList struct {
	rows []taskRow
}

type taskRow struct {
	task   core.Task
	status string
}

// Start fills the list with every task pending.
func (t *TaskList) Start(tasks []core.Task) {
	t.rows = make([]taskRow, len(tasks))
	for i, task := range tasks {
		t.rows[i] = taskRow{task: task, status: taskPending}
	}
}

func (t *TaskList) Running(i int) {
	t.rows[i].status = taskRunning
}

func (t *TaskList) Finish(i int, passed bool) {
	t.rows[i].status = taskFailed
	if passed {
		t.rows[i].status = taskPassed
	}
}

func (t *TaskList) Clear() {
	t.rows = nil
}

func (t TaskList) Active() bool { return len(t.rows) > 0 }

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
