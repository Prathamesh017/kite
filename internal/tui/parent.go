package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"kite/internal/llm"
)

// Model is the parent view. It owns the window size and places the four panes.
// The panes do not know about each other, so a later step can give the terminal
// or the chat its own key handling without moving the frame.
//
// Where the rest lives:
//   - loop.go: what happens on a question, a model reply, and finished tools
//   - commands.go: the background work (model requests, retries, running tools)
//   - taskrun.go: starting, judging, and finishing tasks
//   - layout.go: pane sizes, scrolling, and mouse hit-testing
type Model struct {
	width  int
	height int

	// theme is the index into themes.
	theme int

	header   Header
	activity Activity
	tasks    TaskList
	terminal Terminal
	chat     Chat

	// history is what the model is sent: questions, answers, and the tool
	// requests and results in between. Activity only shows questions and answers.
	history []llm.Message

	// taskHistory is what the running task sends: its description with the
	// earlier results, then its own tool steps. It starts over for each task.
	taskHistory []llm.Message

	// taskErrors holds, per tool name, the error from that tool's latest call
	// in the running task. A later successful call of the same tool removes
	// it, so a recovered mistake does not count. It starts over for each task.
	taskErrors map[string]string

	// iterations and toolCalls count the model replies and the tools run for
	// the current question, or the running task. Both go back to 0 when a new
	// question or task starts.
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
	return tea.Batch(tickCursor(), tickShimmer())
}

// shimmerTickMsg moves the status line's shimmer. It ticks faster than the
// cursor so the movement looks smooth.
type shimmerTickMsg struct{}

func tickShimmer() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return shimmerTickMsg{}
	})
}

// cursorTickMsg flips the chat bar. View only runs again when a message arrives,
// so the bar needs a timer to blink.
type cursorTickMsg struct{}

func tickCursor() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return cursorTickMsg{}
	})
}

// Update routes each message to its handler. The loop for one question, or one
// task, goes: onQuestion sends it, onReply reads the model's reply, onToolsDone
// takes the tool results and asks again, until a reply is a plain answer.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case cursorTickMsg:
		m.chat.cursorOn = !m.chat.cursorOn
		return m, tickCursor()
	case shimmerTickMsg:
		m.activity.Shimmer()
		return m, tickShimmer()
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case replyMsg:
		if msg.err != nil {
			return m, m.onReplyError(msg)
		}
		return m, m.onReply(msg)
	case toolsDoneMsg:
		return m, m.onToolsDone(msg)
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
			return m, m.onQuestion(question)
		}
	}
	return m, nil
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
	activityH := m.activityHeight()
	body := m.activity.View(m.width, activityH)
	if m.tasks.Active() {
		body = lipgloss.JoinVertical(lipgloss.Left, body, m.tasks.View(m.width, bodyH-activityH))
	}
	footer := m.chat.View(m.width, footerH)

	return fillScreen(lipgloss.JoinVertical(lipgloss.Left, header, body, footer), m.width)
}
