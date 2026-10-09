package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"kite/internal/llm"
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
}

func New() Model {
	m := Model{header: Header{cwd: workingDir()}}
	m.setTheme(0)
	return m
}

func (m *Model) setTheme(i int) {
	m.theme = i
	applyTheme(themes[i])
	m.header.theme = themes[i].Name
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case replyMsg:
		if msg.err != nil {
			m.chat.Show(msg.err.Error())
			return m, nil
		}
		m.chat.Show(msg.text)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+t":
			// ctrl+t, not t, so the letter can be typed into the chat.
			m.setTheme((m.theme + 1) % len(themes))
			return m, nil
		}
		// HandleKey returns the question only on Enter. Other keys just edit the input.
		if question := m.chat.HandleKey(msg); question != "" {
			return m, ask(question)
		}
	}
	return m, nil
}

// replyMsg is the model's answer, or the error text to show in the chat.
type replyMsg struct {
	text string
	err  error
}

// ask sends this one question. The system prompt is added inside the client.
func ask(question string) tea.Cmd {
	return func() tea.Msg {
		client, err := llm.FromEnv()
		if err != nil {
			return replyMsg{err: err}
		}
		text, err := client.Complete(context.Background(), []llm.Message{
			{Role: "user", Content: question},
		})
		if err != nil {
			return replyMsg{err: err}
		}
		return replyMsg{text: text}
	}
}

func (m Model) View() string {
	// Bubble Tea asks for a view before the first size message.
	if m.width == 0 || m.height == 0 {
		return ""
	}

	headerH, bodyH, footerH := splitHeight(m.height)
	leftW, rightW := splitWidth(m.width)

	header := m.header.View(m.width, headerH)
	body := lipgloss.JoinHorizontal(lipgloss.Top,
		m.activity.View(leftW, bodyH),
		m.terminal.View(rightW, bodyH),
	)
	footer := m.chat.View(m.width, footerH)

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
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
