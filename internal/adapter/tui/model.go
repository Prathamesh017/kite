package tui

import (
	"context"
	"errors"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	core "kite/internal/core"
)

// runTimeout is how long one question may take, from the question to the answer.
const runTimeout = time.Minute

type Model struct {
	width  int
	height int
	// theme is the index into themes.
	theme    int
	header   Header
	activity Activity
	chat     Chat
	tasks    TaskList
	kite     core.Kite

	// runCtx is the running question's context, sent with every step of that run.
	// cancelRun cancels it. Both are nil when nothing runs.
	runCtx    context.Context
	cancelRun context.CancelFunc

	// header   Header
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

// releaseRun cancels the running question's context, which frees it, and forgets it.
func (m *Model) releaseRun() {
	if m.cancelRun != nil {
		m.cancelRun()
	}
	m.runCtx, m.cancelRun = nil, nil
}

// cancel stops the running question. What the pane already showed stays, without an answer.
func (m *Model) cancel() {
	m.releaseRun()
	m.activity.conversations = append(m.activity.conversations, core.Message{Role: "user", Message: m.activity.pending})
	m.activity.conversations = append(m.activity.conversations, m.activity.steps...)
	m.activity.pending = ""
	m.activity.steps = nil
	m.activity.status = StatusIdle
	m.tasks.Clear()
	m.chat.busy = false
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
		case "esc":
			if m.cancelRun != nil {
				m.cancel()
			}
			return m, nil
		case "ctrl+t":
			m.setTheme((m.theme + 1) % len(themes))
			return m, nil
		}
		//checking and sending to core
		if question := m.chat.HandleKey(msg); question != "" {
			m.runCtx, m.cancelRun = context.WithTimeout(context.Background(), runTimeout)
			m.activity.pending = question
			m.activity.status = StatusThinking
			return m, askKite(m.runCtx, m.kite, question)
		}
	case kiteStepMsg:
		// A step that lands after Esc belongs to a run that is already gone.
		if m.cancelRun == nil {
			return m, nil
		}
		// The run is not finished yet, so show what happened and ask for its next step.
		if msg.err == nil && msg.result.Kind != core.Answered {
			switch msg.result.Kind {
			case core.AskedForTool:
				m.activity.status = StatusTool
				m.activity.detail = toolStatus(msg.result.Tools)
			case core.ToolRan:
				for _, call := range msg.result.Tools {
					m.activity.steps = append(m.activity.steps, core.Message{Role: "tool", Message: call.Name})
				}
				m.activity.status = StatusThinking
			case core.TasksCreated:
				m.tasks.Start(msg.result.Tasks)
				m.activity.status = StatusCreatingTasks
			case core.TaskStarted:
				m.tasks.Running(msg.result.Task)
				m.activity.status = StatusThinking
			case core.TaskFinished:
				m.tasks.Finish(msg.result.Task, msg.result.Passed)
				if msg.result.Passed {
					m.activity.steps = append(m.activity.steps, core.Message{Role: "note", Message: fmt.Sprintf("task %d completed", msg.result.Task+1)})
				}
			}
			return m, runNextStep(m.runCtx, m.kite)
		}
		question := m.activity.pending
		steps := m.activity.steps
		timedOut := errors.Is(m.runCtx.Err(), context.DeadlineExceeded)
		m.releaseRun()
		m.chat.busy = false
		m.activity.pending = ""
		m.activity.steps = nil
		m.activity.status = StatusIdle
		m.tasks.Clear()
		if timedOut {
			m.chat.Restore(question, "Request timed out, please try again.")
		} else if msg.err != nil {
			m.chat.Restore(question, msg.err.Error())
		} else {
			m.activity.conversations = append(m.activity.conversations, core.Message{Role: "user", Message: question})
			m.activity.conversations = append(m.activity.conversations, steps...)
			m.activity.conversations = append(m.activity.conversations, core.Message{Role: "assistant", Message: msg.result.Answer})
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
	activityH := m.activityHeight()
	body := m.activity.View(m.width, activityH)
	if m.tasks.Active() {
		body = lipgloss.JoinVertical(lipgloss.Left, body, m.tasks.View(m.width, bodyH-activityH))
	}

	footer := m.chat.View(m.width, footerH)
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}
