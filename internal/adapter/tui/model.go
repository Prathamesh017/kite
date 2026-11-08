package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	width  int
	height int
	// theme is the index into themes.
	theme  int
	header Header

	// header   Header
	// activity Activity
	// tasks    TaskList
	// terminal Terminal
	// chat     Chat
}

func New() Model {
	m := Model{
		header: Header{cwd: workingDir()},
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
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() string {
	return m.header.View(m.width, m.height)
}
