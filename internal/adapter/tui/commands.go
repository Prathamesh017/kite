package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	core "kite/internal/core"
)

// kiteStepMsg is what comes back into Update after Ask or Step finished one step.
type kiteStepMsg struct {
	result core.Result
	err    error
}

// askKite starts a run in the background so the UI does not freeze while it works.
func askKite(kite core.Kite, question string) tea.Cmd {
	return func() tea.Msg {
		result, err := kite.Ask(context.Background(), question)
		return kiteStepMsg{result: result, err: err}
	}
}

// runNextStep does the next step of the running question in the background.
func runNextStep(kite core.Kite) tea.Cmd {
	return func() tea.Msg {
		result, err := kite.Step(context.Background())
		return kiteStepMsg{result: result, err: err}
	}
}
