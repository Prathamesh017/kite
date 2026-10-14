package tui

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"kite/internal/llm"
)

func (m *Model) startTasks(tasks []llm.Task) tea.Cmd {
	m.tasks.Start(tasks)
	m.activity.SetStatus("creating todo list")
	return m.startTask(0)
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
	return ask(m.runCtx, m.taskHistory, false)
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
	return m.finishTask(answer, passed)
}

func (m *Model) finishTask(answer string, passed bool) tea.Cmd {
	i := m.tasks.Current()
	m.tasks.Finish(i, answer, passed)
	if next := i + 1; passed && next < m.tasks.Len() {
		m.activity.SetStatus(fmt.Sprintf("task %d completed", i+1))
		return m.startTask(next)
	}
	if passed {
		return m.summarizeRun()
	}
	return m.endRun()
}

func (m *Model) summarizeRun() tea.Cmd {
	content := "These tasks all completed. In one or two short lines, say what was done, " +
		"for example \"Both tasks completed. docs.md added.\" Do not repeat file contents. Do not call tools.\n\n" +
		m.tasks.Earlier()
	m.tasks.Clear()
	m.taskHistory = nil
	m.activity.SetStatus("summarizing")
	return ask(m.runCtx, []llm.Message{{Role: "user", Content: content}}, false)
}

func (m *Model) endRun() tea.Cmd {
	summary := m.tasks.Summary()
	m.tasks.Clear()
	m.taskHistory = nil
	return m.showAnswer(summary)
}
