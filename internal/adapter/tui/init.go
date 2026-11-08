package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func Init() {
	var m Model = New()
	// A missing .env is fine. The chat reports it when a question is asked.
	// loadEnv(".env")

	// The alt screen lets the layout own the whole terminal.
	// Mouse events are on for the wheel. Selecting text then needs Option+drag on macOS.
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "kite: %v\n", err)
		os.Exit(1)
	}
}
