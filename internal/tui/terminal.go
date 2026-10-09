package tui

import "strings"

// Terminal is the right pane. It will take a few shell commands later.
// For now it only shows a prompt.
type Terminal struct{}

func (Terminal) View(width, height int) string {
	lines := []string{
		titleStyle.Render("Terminal"),
		mutedStyle.Render("$"),
	}
	return box(strings.Join(lines, "\n"), width, height)
}
