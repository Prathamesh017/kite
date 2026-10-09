package tui

import "strings"

// Activity is the left pane. It will list what the user asked and what the
// agent is doing (thinking, editing a file). These lines are placeholders.
type Activity struct{}

func (Activity) View(width, height int) string {
	lines := []string{
		titleStyle.Render("Activity"),
		"you    fix the failing tests",
		"think  looking at the test output",
		"edit   cmd/main.go",
	}
	return box(strings.Join(lines, "\n"), width, height)
}
