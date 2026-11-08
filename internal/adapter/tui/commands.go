package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	core "kite/internal/core"
)

// kiteReplyMsg is what comes back into Update when Kite has finished replying.
type kiteReplyMsg struct {
	reply string
	err   error
}

// askKite asks the core in the background so the UI does not freeze while it works.
func askKite(kite core.Kite, question string) tea.Cmd {
	return func() tea.Msg {
		reply, err := kite.Ask(context.Background(), question)

		//Here to need if it tool call or regular message
		return kiteReplyMsg{reply: reply, err: err}
	}
}
