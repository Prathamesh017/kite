package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	core "kite/internal/core"
)

// kiteStepMsg is what comes back into Update after Ask or Step finished one step.
type kiteStepMsg struct {
	result core.Result
	err    error
}

// askKite starts a run in the background so the UI does not freeze while it works.
func askKite(ctx context.Context, kite core.Kite, question string) tea.Cmd {
	return func() (msg tea.Msg) {
		defer failOnPanic(&msg)
		result, err := kite.Ask(ctx, question)
		return kiteStepMsg{result: result, err: err}
	}
}

// runNextStep does the next step of the running question in the background.
func runNextStep(ctx context.Context, kite core.Kite) tea.Cmd {
	return func() (msg tea.Msg) {
		defer failOnPanic(&msg)
		result, err := kite.Step(ctx)
		return kiteStepMsg{result: result, err: err}
	}
}

// failOnPanic is deferred at the top of each background command. If anything
// inside panics, it turns the panic into a failed step, so Kite keeps running
// and the question goes back to the input.
func failOnPanic(msg *tea.Msg) {
	if r := recover(); r != nil {
		*msg = kiteStepMsg{err: fmt.Errorf("the step failed: %v", r)}
	}
}
