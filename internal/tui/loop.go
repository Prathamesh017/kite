package tui

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"kite/internal/llm"
)

// onQuestion starts a new question: it shows it, keeps it for the model,
// resets the limits, and sends it with the JSON reply shape.
func (m *Model) onQuestion(question string) tea.Cmd {
	m.activity.AppendQuestion(question)
	m.history = append(m.history, llm.Message{Role: "user", Content: question})
	m.iterations, m.toolCalls = 0, 0
	return ask(m.history, true)
}

// onReply handles a model reply that arrived. There are three kinds:
//   - tool calls: show what is about to run, then run the tools as their own step
//   - an answer inside a task: judge the task by its policy
//   - an answer to the question: a task list starts the tasks, anything else is shown
func (m *Model) onReply(msg replyMsg) tea.Cmd {
	m.iterations++

	if msg.finish == "tool_calls" && len(msg.calls) > 0 {
		// More tools than are left: none of them run.
		if len(msg.calls) > toolCallLimit()-m.toolCalls {
			return m.stopLoop("Reached its tool-call limit. The task may be partially complete.")
		}
		m.activity.SetStatus(toolStatus(msg.calls))
		return runTools(msg.response, msg.calls)
	}

	answer := readAnswer(msg.response)
	// "length" means the model ran out of room mid-reply. Keep what it wrote,
	// and say it was cut off so it is not mistaken for a full answer.
	if msg.finish == "length" {
		answer += "\n\nThe reply was cut off because it hit the model's length limit."
	}

	if m.tasks.Active() {
		return m.judgeTask(answer)
	}

	if reply, ok := llm.ParseReply(msg.response); ok && msg.finish != "length" && reply.Mode == "task" && len(reply.Tasks) > 0 {
		m.tasks.Start(reply.Tasks)
		m.activity.SetStatus("creating todo list")
		return m.startTask(0)
	}

	m.activity.AppendAnswer(answer)
	m.history = append(m.history, llm.Message{Role: "assistant", Content: answer})
	m.chat.Show("")
	return nil
}

// onReplyError handles a failed model request. The first retryable failure
// switches the pane to retrying and starts retry(), which only reports its
// final result. A final failure fails the running task, or, for a question,
// gives the question back to the input with the reason.
func (m *Model) onReplyError(msg replyMsg) tea.Cmd {
	retryConfig := llm.RetryConfigFromEnv()
	if !msg.final && llm.Retryable(msg.err) && retryConfig.Count > 0 {
		m.activity.Retrying()
		return retry(*m.loopHistory(), retryConfig, !m.tasks.Active())
	}
	if m.tasks.Active() {
		return m.finishTask("The request failed: "+failText(msg.err), false)
	}
	question := m.activity.RemoveQuestion()
	m.history = dropLastQuestion(m.history)
	m.chat.Restore(question, failText(msg.err))
	return nil
}

// onToolsDone keeps the tool request and its results for the model, and asks
// it again so it can use them. Tool lines go to Activity only for a question;
// during tasks Activity is updated once, at the end, and the results are
// recorded for the task's policy instead.
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
	return ask(*history, !m.tasks.Active())
}

// stopLoop ends the loop when a limit is hit. For a question it is not a
// failure: the question and the tool steps stay, and the note takes the place
// of the answer, in Activity and in the history the model sees next time.
// For a task, the task fails with the note as its answer.
func (m *Model) stopLoop(note string) tea.Cmd {
	if m.tasks.Active() {
		return m.finishTask(note, false)
	}
	m.activity.AppendAnswer(note)
	m.history = append(m.history, llm.Message{Role: "assistant", Content: note})
	m.chat.Show("")
	return nil
}

// loopHistory is what the current loop sends: the running task's own messages
// while tasks run, otherwise the conversation.
func (m *Model) loopHistory() *[]llm.Message {
	if m.tasks.Active() {
		return &m.taskHistory
	}
	return &m.history
}

// iterationLimit and toolCallLimit read MAX_ITERATIONS and MAX_TOOL_CALLS, the
// limits for one question. A missing or unreadable value falls back to 10 and 20.
func iterationLimit() int { return envInt("MAX_ITERATIONS", 10) }
func toolCallLimit() int  { return envInt("MAX_TOOL_CALLS", 20) }

func envInt(key string, fallback int) int {
	if n, err := strconv.Atoi(os.Getenv(key)); err == nil && n > 0 {
		return n
	}
	return fallback
}

// toolStatus is the status line for the tools about to run, like
// "reading main.go". Several calls are joined with commas.
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

// readAnswer turns the model's JSON reply into the text shown as Kite's answer.
// "normal" and "unsupported" show the answer (for unsupported, the reason).
// A "task" reply with tasks is started instead; one without any is reported.
// Text that is not the JSON shape, like a task's plain answer, is shown as it is.
func readAnswer(response string) string {
	reply, ok := llm.ParseReply(response)
	if !ok {
		return response
	}
	if reply.Mode == "task" {
		return "The model chose task mode but listed no tasks."
	}
	if reply.Answer == "" {
		return "The model returned an empty answer."
	}
	return reply.Answer
}

// dropLastQuestion removes the last question and any tool steps after it,
// so a failed run leaves no trace in what the model is sent next time.
func dropLastQuestion(history []llm.Message) []llm.Message {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "user" {
			return history[:i]
		}
	}
	return history
}

// failText is what the chat shows when a request ends in failure.
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
