package tui

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"kite/internal/llm"
)

func (m *Model) startTasks(tasks []llm.Task) tea.Cmd {
	if existing := existingFiles(tasks); len(existing) > 0 {
		return m.showAnswer("Not started: " + strings.Join(existing, ", ") +
			" already exists, and Kite can only create new files.")
	}
	m.tasks.Start(tasks)
	m.activity.SetStatus("creating todo list")
	return m.startTask(0)
}

//checks if a task has to write a file that already exists
func existingFiles(tasks []llm.Task) []string {
	var existing []string
	for _, task := range tasks {
		for _, path := range task.Creates {
			if _, err := os.Stat(path); err == nil {
				existing = append(existing, path)
			}
		}
	}
	return existing
}

func (m *Model) startTask(i int) tea.Cmd {
	m.tasks.Running(i)
	content := "Task: " + m.tasks.Task(i).Description
	if earlier := m.tasks.Earlier(); earlier != "" {
		content += "\n\nResults of the earlier tasks:\n" + earlier
	}
	m.taskHistory = []llm.Message{{Role: "user", Content: content}}
	m.taskErrors = map[string]string{}
	m.iterations, m.toolCalls = 0, 0
	return ask(m.runCtx, m.taskHistory, llm.NoShape)
}

func (m *Model) recordToolErrors(calls []llm.ToolCall, results []llm.Message) {
	for i, call := range calls {
		if result := results[i].Content; strings.HasPrefix(result, "error:") {
			m.taskErrors[call.Function.Name] = result
		} else {
			delete(m.taskErrors, call.Function.Name)
		}
	}
}

func (m *Model) judgeTask(answer string) tea.Cmd {
	policy := m.tasks.Task(m.tasks.Current()).VerificationPolicy.Type
	passed := policy == "none" || len(m.taskErrors) == 0
	if !passed {
		failed := strings.Join(slices.Sorted(maps.Values(m.taskErrors)), "; ")
		answer = "A tool failed: " + failed + "\n" + answer
	}
	if passed && policy == "read_back" {
		return m.startCheck()
	}
	return m.finishTask(answer, passed)
}

func (m *Model) startCheck() tea.Cmd {
	task := m.tasks.Task(m.tasks.Current())
	files := "none listed"
	if len(task.Creates) > 0 {
		files = strings.Join(task.Creates, ", ")
	}
	content := "Check the result of this task. Read the files it created with read_file and " +
		"compare them with the acceptance criteria. Do not write files.\n\n" +
		"Task: " + task.Description + "\n\n" +
		"Acceptance criteria:\n- " + strings.Join(task.AcceptanceCriteria, "\n- ") + "\n\n" +
		"Files it created: " + files + "\n\n" +
		"Reply with verdict PASS or FAIL. In message, say in one line what the files hold, or why the task fails."
	m.checking = true
	m.taskHistory = []llm.Message{{Role: "user", Content: content}}
	m.taskErrors = map[string]string{}
	m.iterations, m.toolCalls = 0, 0
	return ask(m.runCtx, m.taskHistory, llm.VerdictShape)
}

func (m *Model) onVerdict(msg replyMsg) tea.Cmd {
	verdict, ok := llm.ParseVerdict(msg.response)
	if !ok {
		return m.finishTask("The read_back check did not return a verdict.", false)
	}
	if len(m.taskErrors) > 0 {
		failed := strings.Join(slices.Sorted(maps.Values(m.taskErrors)), "; ")
		return m.finishTask("A tool failed during the check: "+failed, false)
	}
	return m.finishTask(verdict.Message, verdict.Verdict == "PASS")
}

func (m *Model) finishTask(answer string, passed bool) tea.Cmd {
	m.checking = false
	i := m.tasks.Current()
	m.tasks.Finish(i, answer, passed)
	if passed {
		m.activity.AppendNote(fmt.Sprintf("task %d completed", i+1))
	}
	if next := i + 1; passed && next < m.tasks.Len() {
		m.activity.SetStatus("thinking")
		return m.startTask(next)
	}
	if passed {
		return m.summarizeRun()
	}
	return m.endRun()
}

func (m *Model) summarizeRun() tea.Cmd {
	content := "These tasks all completed. In one or two short lines, say what was done, " +
		"for example \"All tasks completed. docs.md added.\" Do not repeat file contents. Do not call tools.\n\n" +
		m.tasks.Earlier()
	m.tasks.Clear()
	m.taskHistory = nil
	m.activity.SetStatus("summarizing")
	return ask(m.runCtx, []llm.Message{{Role: "user", Content: content}}, llm.NoShape)
}

func (m *Model) endRun() tea.Cmd {
	summary := m.tasks.Summary()
	m.tasks.Clear()
	m.taskHistory = nil
	return m.showAnswer(summary)
}
