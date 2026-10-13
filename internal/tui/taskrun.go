package tui

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"kite/internal/llm"
)

// startTask marks task i running and sends it, without the JSON reply shape,
// with a fresh iteration and tool-call budget. The request carries the task's
// description and the results of the earlier tasks. Its steps then go through
// Update like a question's, using taskHistory.
func (m *Model) startTask(i int) tea.Cmd {
	m.tasks.Running(i)
	content := "Task: " + m.tasks.Task(i).Description
	if earlier := m.tasks.Earlier(); earlier != "" {
		content += "\n\nResults of the earlier tasks:\n" + earlier
	}
	m.taskHistory = []llm.Message{{Role: "user", Content: content}}
	m.taskErrors = map[string]string{}
	m.iterations, m.toolCalls = 0, 0
	return ask(m.taskHistory, false)
}

// recordToolErrors notes, per tool name, whether that tool's latest call in
// the running task failed. A later successful call of the same tool clears
// the note, so a mistake the model recovered from does not count.
func (m *Model) recordToolErrors(calls []llm.ToolCall, results []llm.Message) {
	for i, call := range calls {
		if result := results[i].Content; strings.HasPrefix(result, "error:") {
			m.taskErrors[call.Function.Name] = result
		} else {
			delete(m.taskErrors, call.Function.Name)
		}
	}
}

// judgeTask decides whether the running task passed once it answers.
// "none" passes once an answer arrives. Every other policy is checked like
// "tool_result" for now: the task fails if any tool's latest call in it still
// ended in an error.
func (m *Model) judgeTask(answer string) tea.Cmd {
	policy := m.tasks.Task(m.tasks.Current()).VerificationPolicy.Type
	if policy == "none" || len(m.taskErrors) == 0 {
		return m.finishTask(answer, true)
	}
	failed := strings.Join(slices.Sorted(maps.Values(m.taskErrors)), "; ")
	return m.finishTask("A tool failed: "+failed+"\n"+answer, false)
}

// finishTask records the running task's answer and whether it passed.
// A pass moves on to the next task, and the status says which one completed
// until the next reply arrives. A failure, or the last task, ends the run:
// the summary takes the list's place in Activity, and is the only trace of
// the run in the history the model sees next time.
func (m *Model) finishTask(answer string, passed bool) tea.Cmd {
	i := m.tasks.Current()
	m.tasks.Finish(i, answer, passed)
	if next := i + 1; passed && next < m.tasks.Len() {
		m.activity.SetStatus(fmt.Sprintf("task %d completed", i+1))
		return m.startTask(next)
	}
	summary := m.tasks.Summary()
	m.tasks.Clear()
	m.taskHistory = nil
	m.activity.AppendAnswer(summary)
	m.history = append(m.history, llm.Message{Role: "assistant", Content: summary})
	m.chat.Show("")
	return nil
}
