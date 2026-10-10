package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"kite/internal/llm"
	"kite/internal/tools"
)

const (
	// headerHeight is one content row plus the top and bottom border.
	headerHeight = 3

	// minFooterHeight fits a title, two messages, the input line, and the border.
	minFooterHeight = 6

	// minBodyHeight keeps the middle band visible when the window is short.
	minBodyHeight = 5

	// leftShare is the activity pane's portion of the middle band. The terminal gets the rest.
	leftShare = 60
)

// Model is the parent view. It owns the window size and places the four panes.
// The panes do not know about each other, so a later step can give the terminal
// or the chat its own key handling without moving the frame.
type Model struct {
	width  int
	height int

	// theme is the index into themes.
	theme int

	header   Header
	activity Activity
	terminal Terminal
	chat     Chat

	// history is what the model is sent: questions, answers, and the tool
	// requests and results in between. Activity only shows questions and answers.
	history []llm.Message

	// iterations and toolCalls count the model replies and the tools run for
	// the current question. Both go back to 0 when a new question is sent.
	iterations int
	toolCalls  int
}

func New() Model {
	m := Model{
		header: Header{cwd: workingDir()},
		chat:   Chat{cursorOn: true},
	}
	m.setTheme(0)
	return m
}

func (m *Model) setTheme(i int) {
	m.theme = i
	applyTheme(themes[i])
	m.header.theme = themes[i].Name
}

func (m Model) Init() tea.Cmd {
	return tickCursor()
}

// cursorTickMsg flips the chat bar. View only runs again when a message arrives,
// so the bar needs a timer to blink.
type cursorTickMsg struct{}

func tickCursor() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return cursorTickMsg{}
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case cursorTickMsg:
		m.chat.cursorOn = !m.chat.cursorOn
		return m, tickCursor()
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case replyMsg:
		if msg.err == nil {
			m.iterations++
			// The reply asked for more tools than are left, so none of them ran.
			if msg.toolLimitHit {
				m.stopIncomplete("Reached its tool-call limit. The task may be partially complete.")
				return m, nil
			}
			// The tools already ran. Keep the request and the results for the
			// model, and ask it again so it can use them.
			if len(msg.toolCalls) > 0 {
				for _, call := range msg.toolCalls {
					m.activity.AppendToolCall(call.Function.Name)
				}
				m.history = append(m.history, llm.Message{Role: "assistant", Content: msg.response, ToolCalls: msg.toolCalls})
				m.history = append(m.history, msg.toolResults...)
				m.toolCalls += len(msg.toolCalls)
				if m.iterations >= iterationLimit() {
					m.stopIncomplete("Reached its execution-step limit before completing the task.")
					return m, nil
				}
				return m, ask(m.history, toolCallLimit()-m.toolCalls)
			}
			m.activity.AppendAnswer(msg.response)
			m.history = append(m.history, llm.Message{Role: "assistant", Content: msg.response})
			m.chat.Show("")
			return m, nil
		}
		// The first failure comes back here so the pane can switch to retrying.
		// Later retries run inside retry() and only report the final result.
		retryConfig := llm.RetryConfigFromEnv()
		if !msg.final && llm.Retryable(msg.err) && retryConfig.Count > 0 {
			m.activity.Retrying()
			return m, retry(m.history, retryConfig, toolCallLimit()-m.toolCalls)
		}
		question := m.activity.RemoveQuestion()
		m.history = dropLastQuestion(m.history)
		m.chat.Restore(question, failText(msg.err))
	case tea.MouseMsg:
		// The wheel only scrolls when the pointer is over the activity pane.
		if !m.overActivity(msg.X, msg.Y) {
			return m, nil
		}
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.scrollActivity(-wheelStep)
		case tea.MouseButtonWheelDown:
			m.scrollActivity(wheelStep)
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+t":
			// ctrl+t, not t, so the letter can be typed into the chat.
			m.setTheme((m.theme + 1) % len(themes))
			return m, nil
		case "up", "down":
			delta := 1
			if msg.String() == "up" {
				delta = -1
			}
			m.scrollActivity(delta)
			return m, nil
		}
		// HandleKey returns the question only on Enter. Other keys just edit the input.
		if question := m.chat.HandleKey(msg); question != "" {
			m.activity.AppendQuestion(question)
			m.history = append(m.history, llm.Message{Role: "user", Content: question})
			m.iterations, m.toolCalls = 0, 0
			return m, ask(m.history, toolCallLimit())
		}
	}
	return m, nil
}

// stopIncomplete ends the run when a limit is hit. It is not a failure: the
// question and the tool steps stay, and the note takes the place of the answer,
// in Activity and in the history the model sees next time.
func (m *Model) stopIncomplete(note string) {
	m.activity.AppendAnswer(note)
	m.history = append(m.history, llm.Message{Role: "assistant", Content: note})
	m.chat.Show("")
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

// replyMsg is the model's answer, or the tools it asked for along with their
// results, one per call. The question is already in the activity list.
// final marks the result of the retries, so it is not retried again.
// toolLimitHit means the reply asked for more tools than were left, so none ran.
type replyMsg struct {
	response     string
	toolCalls    []llm.ToolCall
	toolResults  []llm.Message
	toolLimitHit bool
	err          error
	final        bool
}

// ask sends the history, which ends with the question just asked or with tool
// results. The system prompt is added inside the client.
// toolsLeft is how many tool calls this question may still make.
func ask(history []llm.Message, toolsLeft int) tea.Cmd {
	sent := append([]llm.Message(nil), history...)
	return func() (msg tea.Msg) {
		step := "model request"
		defer failOnPanic(&step, &msg)

		client, err := llm.FromEnv()
		if err != nil {
			return replyMsg{err: err, final: true}
		}
		response, finish, calls, err := client.Complete(context.Background(), sent)
		if err != nil {
			return replyMsg{err: err}
		}
		return handleReply(response, finish, calls, toolsLeft, &step)
	}
}

// failOnPanic is deferred at the top of each background command. If anything
// inside panics, it turns the panic into a failed reply naming the step that
// was running, so Kite keeps running and the question goes back to the input.
// It is final because running the same broken step again would panic again.
func failOnPanic(step *string, msg *tea.Msg) {
	if r := recover(); r != nil {
		*msg = replyMsg{err: fmt.Errorf("%s failed: %v", *step, r), final: true}
	}
}

// handleReply checks why the model stopped. "tool_calls" means it wants tools
// run, so each one is looked up by name in the registry and called. Every call
// gets a result carrying its ID, because the model expects one answer per request.
// step is updated before each tool, so a panic can name the tool that crashed.
// If the reply asks for more tools than toolsLeft, none of them run.
func handleReply(response, finish string, calls []llm.ToolCall, toolsLeft int, step *string) replyMsg {
	// "length" means the model ran out of room mid-reply. Keep what it wrote,
	// and say it was cut off so it is not mistaken for a full answer.
	if finish == "length" {
		return replyMsg{response: response + "\n\nThe reply was cut off because it hit the model's length limit."}
	}
	if finish != "tool_calls" {
		return replyMsg{response: response}
	}
	if len(calls) > toolsLeft {
		return replyMsg{toolLimitHit: true}
	}
	results := make([]llm.Message, 0, len(calls))
	//tool call happening herew
	for _, call := range calls {
		*step = "tool call " + call.Function.Name
		result := "error: unknown tool " + call.Function.Name
		if run, ok := tools.Registry[call.Function.Name]; ok {
			result = run(call.Function.Arguments)
		}
		results = append(results, llm.Message{Role: "tool", Content: result, ToolCallID: call.ID})
	}
	return replyMsg{response: response, toolCalls: calls, toolResults: results}
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

// retry sends the same history up to settings.Count more times. Before each
// try it waits a random time up to the backoff cap. It stops early when an error
// cannot be retried.
func retry(history []llm.Message, retryConfig llm.Retry, toolsLeft int) tea.Cmd {
	sent := append([]llm.Message(nil), history...)
	return func() (msg tea.Msg) {
		step := "model request"
		defer failOnPanic(&step, &msg)

		client, err := llm.FromEnv()
		if err != nil {
			return replyMsg{err: err, final: true}
		}
		for attempt := 0; attempt < retryConfig.Count; attempt++ {
			time.Sleep(llm.RandomUpTo(llm.Backoff(attempt, retryConfig)))

			response, finish, calls, err := client.Complete(context.Background(), sent)
			//if the response is successful, return the response
			if err == nil {
				reply := handleReply(response, finish, calls, toolsLeft, &step)
				reply.final = true
				return reply
			}
			//if the new eror is retryable, and we have atrtemo
			if !llm.Retryable(err) || attempt == retryConfig.Count-1 {
				return replyMsg{err: err, final: true}
			}
		}
		return replyMsg{err: &llm.APIError{}, final: true}
	}
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

func (m Model) View() string {
	// Bubble Tea asks for a view before the first size message.
	if m.width == 0 || m.height == 0 {
		return ""
	}

	headerH, bodyH, footerH := splitHeight(m.height)
	// The terminal pane is hidden for phase 1, so activity takes the full width.
	// To bring it back: split with splitWidth and join m.terminal.View(rightW, bodyH).
	header := m.header.View(m.width, headerH)
	body := m.activity.View(m.width, bodyH)
	footer := m.chat.View(m.width, footerH)

	return fillScreen(lipgloss.JoinVertical(lipgloss.Left, header, body, footer), m.width)
}

// wheelStep is how many rows one wheel notch scrolls the activity pane.
const wheelStep = 3

func (m *Model) scrollActivity(delta int) {
	_, bodyH, _ := splitHeight(m.height)
	m.activity.Scroll(delta, m.width, bodyH)
}

// overActivity reports whether a mouse position falls inside the activity pane.
func (m Model) overActivity(x, y int) bool {
	headerH, bodyH, _ := splitHeight(m.height)
	return x < m.width && y >= headerH && y < headerH+bodyH
}

// splitHeight gives the header a fixed bar, the chat about a third of what
// remains, and the middle band everything left over.
func splitHeight(height int) (header, body, footer int) {
	header = headerHeight
	rest := height - header
	if rest < 0 {
		return height, 0, 0
	}

	footer = rest / 3
	if footer < minFooterHeight {
		footer = minFooterHeight
	}
	// Leave the activity and terminal panes a few rows when the window is short.
	if footer > rest-minBodyHeight {
		footer = rest - minBodyHeight
	}
	if footer < 0 {
		footer = 0
	}

	body = rest - footer
	return header, body, footer
}

// splitWidth gives the activity pane 60% and the terminal the remaining 40%.
func splitWidth(width int) (left, right int) {
	left = width * leftShare / 100
	right = width - left
	return left, right
}
