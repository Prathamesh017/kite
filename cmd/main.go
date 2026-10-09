package main

import (
	"fmt"
	"os"
	tea "github.com/charmbracelet/bubbletea"
	"kite/internal/tui"
)

func main() {
	// The alt screen lets the layout own the whole terminal.
	p := tea.NewProgram(tui.New(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "kite: %v\n", err)
		os.Exit(1)
	}
}
