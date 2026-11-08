package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	width  int
	height int
	// theme is the index into themes.
	theme  int
	header Header
	chat   Chat

	// header   Header
	// activity Activity
	// tasks    TaskList
	// terminal Terminal
	// chat     Chat
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

func (m Model) Init() tea.Cmd {
	return tea.Batch(tickCursor(), tickShimmer())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case cursorTickMsg:
		m.chat.cursorOn = !m.chat.cursorOn
		return m, tickCursor()
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		
		case "ctrl+t":
			m.setTheme((m.theme + 1) % len(themes))
			return m, nil
		}
		m.chat.HandleKey(msg);
	
		

	}
	return m, nil
}

func (m Model) View() string {
	// Bubble Tea asks for a view before the first size message.
	if m.width == 0 || m.height == 0 {
		return ""
	}

	//gets respective height for each section
	headerH, bodyH, footerH := splitHeight(m.height)
	header := m.header.View(m.width, headerH)
	// The middle band is empty until the activity pane exists. It keeps the chat at the bottom.
	body := lipgloss.NewStyle().Height(bodyH).Render("")
	
	footer := m.chat.View(m.width, footerH)
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}
