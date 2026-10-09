package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"kite/internal/tui"
)

func main() {
	// A missing .env is fine. The chat reports it when a question is asked.
	loadEnv(".env")

	// The alt screen lets the layout own the whole terminal.
	p := tea.NewProgram(tui.New(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "kite: %v\n", err)
		os.Exit(1)
	}
}

// loadEnv copies KEY=VALUE lines from path into the environment.
// Variables already set are left alone.
func loadEnv(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		os.Setenv(key, strings.TrimSpace(val))
	}
}
