package tui

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"kite/internal/llm"
)

func (m *Model) onQuestion(question string) tea.Cmd {
	m.activity.AppendQuestion(question)
	m.history = append(m.history, llm.Message{Role: "user", Content: question})
	m.iterations, m.toolCalls = 0, 0
	m.runCtx, m.cancelRun = context.WithTimeout(context.Background(), runTimeout)
	return ask(m.runCtx, m.history, llm.ModeShape)
}

func (m *Model) showAnswer(answer string) tea.Cmd {
	m.releaseRun()
	m.activity.AppendAnswer(answer)
	m.history = append(m.history, llm.Message{Role: "assistant", Content: answer})
	m.chat.Show("")
	return nil
}

func (m *Model) cancel() {
	m.releaseRun()
	m.history = append(m.history, llm.Message{Role: "assistant", Content: "(cancelled by the user before answering)"})
	m.tasks.Clear()
	m.taskHistory = nil
	m.checking = false
	m.activity.SetStatus("")
	m.chat.Show("")
}

func (m *Model) onTimeout() tea.Cmd {
	m.releaseRun()
	m.tasks.Clear()
	m.taskHistory = nil
	m.checking = false
	question := m.activity.RemoveQuestion()
	m.history = dropLastQuestion(m.history)
	m.chat.Restore(question, "Request timed out, please try again.")
	return nil
}

func (m *Model) releaseRun() {
	if m.cancelRun != nil {
		m.cancelRun()
	}
	m.runCtx, m.cancelRun = nil, nil
}


//handles all 3 flows here: normal answer, tool calls, and tasks
func (m *Model) onReply(msg replyMsg) tea.Cmd {
	m.iterations++
	if msg.finish == "tool_calls" && len(msg.calls) > 0 {
		return m.onToolCalls(msg)
	}
	if m.checking {
		return m.onVerdict(msg)
	}
	if m.tasks.Active() {
		return m.onTaskAnswer(msg)
	}
	return m.onQuestionAnswer(msg)
}

func (m *Model) onToolCalls(msg replyMsg) tea.Cmd {
	if len(msg.calls) > toolCallLimit()-m.toolCalls {
		return m.stopLoop("Reached its tool-call limit. The task may be partially complete.")
	}
	m.activity.SetStatus(toolStatus(msg.calls))
	return runTools(m.runCtx, msg.response, msg.calls)
}

func (m *Model) onQuestionAnswer(msg replyMsg) tea.Cmd {
	reply, ok := llm.ParseReply(msg.response)
	if ok && reply.Mode == "task" && len(reply.Tasks) > 0 && msg.finish != "length" {
		return m.startTasks(reply.Tasks)
	}
	return m.showAnswer(markCutOff(readAnswer(msg.response), msg.finish))
}

func (m *Model) onTaskAnswer(msg replyMsg) tea.Cmd {
	return m.judgeTask(markCutOff(msg.response, msg.finish))
}

func markCutOff(answer, finish string) string {
	if finish == "length" {
		return answer + "\n\nThe reply was cut off because it hit the model's length limit."
	}
	return answer
}

func (m *Model) onReplyError(msg replyMsg) tea.Cmd {
	retryConfig := llm.RetryConfigFromEnv()
	if !msg.final && llm.Retryable(msg.err) && retryConfig.Count > 0 {
		m.activity.Retrying()
		return retry(m.runCtx, *m.loopHistory(), retryConfig, m.shape())
	}
	if m.tasks.Active() {
		return m.finishTask("The request failed: "+failText(msg.err), false)
	}
	m.releaseRun()
	question := m.activity.RemoveQuestion()
	m.history = dropLastQuestion(m.history)
	m.chat.Restore(question, failText(msg.err))
	return nil
}

func (m *Model) onToolsDone(msg toolsDoneMsg) tea.Cmd {
	history := m.loopHistory()
	*history = append(*history, llm.Message{Role: "assistant", Content: msg.response, ToolCalls: msg.calls})
	*history = append(*history, msg.results...)
	m.toolCalls += len(msg.calls)

	if m.tasks.Active() {
		m.recordToolErrors(msg.calls, msg.results)
	} else {
		for _, call := range msg.calls {
			m.activity.AppendToolCall(call.Function.Name)
		}
	}

	if m.iterations >= iterationLimit() {
		return m.stopLoop("Reached its execution-step limit before completing the task.")
	}
	m.activity.SetStatus("thinking")
	return ask(m.runCtx, *history, m.shape())
}

func (m *Model) stopLoop(note string) tea.Cmd {
	if m.tasks.Active() {
		return m.finishTask(note, false)
	}
	return m.showAnswer(note)
}

func (m *Model) loopHistory() *[]llm.Message {
	if m.tasks.Active() {
		return &m.taskHistory
	}
	return &m.history
}

func (m *Model) shape() llm.Shape {
	switch {
	case m.checking:
		return llm.VerdictShape
	case m.tasks.Active():
		return llm.NoShape
	default:
		return llm.ModeShape
	}
}

const runTimeout = time.Minute

func iterationLimit() int { return envInt("MAX_ITERATIONS", 10) }
func toolCallLimit() int  { return envInt("MAX_TOOL_CALLS", 20) }

func envInt(key string, fallback int) int {
	if n, err := strconv.Atoi(os.Getenv(key)); err == nil && n > 0 {
		return n
	}
	return fallback
}

func toolStatus(calls []llm.ToolCall) string {
	parts := make([]string, 0, len(calls))
	for _, call := range calls {
		var args struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal([]byte(call.Function.Arguments), &args)
		switch call.Function.Name {
		case "read_file":
			parts = append(parts, "reading "+args.Path)
		case "write_file":
			parts = append(parts, "writing "+args.Path)
		case "list_files":
			parts = append(parts, "listing files")
		default:
			parts = append(parts, "using "+call.Function.Name)
		}
	}
	return strings.Join(parts, ", ")
}

func readAnswer(response string) string {
	reply, ok := llm.ParseReply(response)
	if !ok {
		return response
	}
	if reply.Mode == "task" {
		if len(reply.Tasks) == 0 {
			return "The model chose task mode but listed no tasks."
		}
		lines := make([]string, len(reply.Tasks))
		for i, task := range reply.Tasks {
			lines[i] = strconv.Itoa(i+1) + ". " + task.Description
		}
		return strings.Join(lines, "\n")
	}
	if reply.Answer == "" {
		return "The model returned an empty answer."
	}
	return reply.Answer
}

func dropLastQuestion(history []llm.Message) []llm.Message {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "user" {
			return history[:i]
		}
	}
	return history
}

func failText(err error) string {
	var apiErr *llm.APIError
	if !errors.As(err, &apiErr) {
		return err.Error()
	}
	if apiErr.Status == 0 {
		return "something went wrong"
	}
	if apiErr.Message == "" {
		return "something went wrong, try again " + string(err.Error())
	}
	return apiErr.Message
}
