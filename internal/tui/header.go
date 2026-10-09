package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	version = "v0.1.0"
	status  = "Ready"
)

// Header is the top bar: the name and theme on the left, then the working
// directory, version and status on the right. The (t) hint is the key that
// switches themes.
type Header struct {
	cwd   string
	theme string
}

func (h Header) View(width, height int) string {
	const sep = "  "
	left := brandStyle.Render("KITE") + sep + mutedStyle.Render(h.theme+" (t)")
	meta := versionStyle.Render(version) + sep +
		readyStyle.Render("●") + " " + statusStyle.Render(status)

	textW, _ := textSize(width, height)

	// The path gets whatever room is left, keeping one column of gap after the left side.
	room := textW - lipgloss.Width(left) - lipgloss.Width(meta) - len(sep) - 1
	right := meta
	if cwd := trimLeft(h.cwd, room); cwd != "" {
		right = mutedStyle.Render(cwd) + sep + meta
	}

	return box(spread(left, right, textW), width, height)
}

// spread puts left and right on one row, with the gap between them.
// A row that cannot fit both keeps the left side.
func spread(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return left
	}
	return left + strings.Repeat(" ", gap) + right
}
