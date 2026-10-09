package tui

import "strings"

// Chat is the footer. It will be the conversation with NAVI.
// The exchange below is a placeholder, and the last line is the input.
type Chat struct{}

func (Chat) View(width, height int) string {
	lines := []string{
		titleStyle.Render("Chat"),
		"you   how does the layout look?",
		"kite  the frame is up. nothing runs yet.",
		mutedStyle.Render("> Ask KITE anything..."),
	}
	return box(strings.Join(lines, "\n"), width, height)
}
