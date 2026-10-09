package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Colors are filled in by applyTheme in theme.go.
var (
	// panelStyle is the shared frame. One cell of side padding keeps text off the border.
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			Padding(0, 1)

	brandStyle   lipgloss.Style
	versionStyle lipgloss.Style
	readyStyle   lipgloss.Style
	statusStyle  lipgloss.Style
	titleStyle   lipgloss.Style
	mutedStyle   lipgloss.Style
)

// textSize is the room left for text once the border and padding are removed.
func textSize(width, height int) (int, int) {
	return width - panelStyle.GetHorizontalFrameSize(),
		height - panelStyle.GetVerticalFrameSize()
}

// box paints content inside a bordered panel of the given outer size.
// Lipgloss Width includes padding and excludes the border, so those are added
// back separately. Extra lines are dropped so a short window cannot push the
// other panes out of place.
func box(content string, width, height int) string {
	textW, textH := textSize(width, height)
	if textW < 1 || textH < 1 {
		return ""
	}

	wrapped := lipgloss.NewStyle().Width(textW).Render(content)
	lines := strings.Split(wrapped, "\n")
	if len(lines) > textH {
		lines = lines[:textH]
	}

	return panelStyle.
		Width(textW + panelStyle.GetHorizontalPadding()).
		Height(textH + panelStyle.GetVerticalPadding()).
		Render(strings.Join(lines, "\n"))
}
