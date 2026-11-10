package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	core "kite/internal/core"
)

type Model struct {
	width  int
	height int
	// theme is the index into themes.
	theme    int
	header   Header
	activity Activity
	chat     Chat
	kite     core.Kite

	// header   Header
	// tasks    TaskList
	// terminal Terminal
	// chat     Chat
}

func New(kite core.Kite) Model {
	m := Model{
		header: Header{cwd: workingDir()},
		chat:   Chat{cursorOn: true},
		kite:   kite,
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
	case shimmerTickMsg:
		m.activity.Shimmer()
		return m, tickShimmer()
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+t":
			m.setTheme((m.theme + 1) % len(themes))
			return m, nil
		}
		//checking and sending to core
		if question := m.chat.HandleKey(msg); question != "" {
			m.activity.pending = question
			m.activity.status = StatusThinking
			return m, askKite(m.kite, question)
		}
	case kiteStepMsg:
		// The run is not finished yet, so ask for its next step.
		if msg.err == nil && msg.result.Kind != core.Answered {
			return m, runNextStep(m.kite)
		}
		question := m.activity.pending
		m.chat.busy = false
		m.activity.pending = ""
		m.activity.status = StatusIdle
		if msg.err != nil {
			m.chat.Restore(question, msg.err.Error())
		} else {
			m.activity.conversations = append(m.activity.conversations,
				core.Message{Role: "user", Message: question},
				core.Message{Role: "assistant", Message: msg.result.Answer},
			)
		}
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
	body := m.activity.View(m.width, bodyH)

	footer := m.chat.View(m.width, footerH)
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}
