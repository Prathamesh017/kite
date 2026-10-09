package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	case tea.KeyMsg:
		// The chat and the terminal do not take keys yet, so single letters are free.
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "t":
			m.setTheme((m.theme + 1) % len(themes))
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
