package tui

const (
	// headerHeight is one content row plus the top and bottom border.
	headerHeight = 3

	// minFooterHeight fits a title, two messages, the input line, and the border.
	minFooterHeight = 6

	// minBodyHeight keeps the middle band visible when the window is short.
	minBodyHeight = 5

	// leftShare is the activity pane's portion of the middle band. The terminal gets the rest.
	leftShare = 60

	// wheelStep is how many rows one wheel notch scrolls the activity pane.
	wheelStep = 3
)

func (m *Model) scrollActivity(delta int) {
	m.activity.Scroll(delta, m.width, m.activityHeight())
}

// overActivity reports whether a mouse position falls inside the activity pane.
func (m Model) overActivity(x, y int) bool {
	headerH, _, _ := splitHeight(m.height)
	return x < m.width && y >= headerH && y < headerH+m.activityHeight()
}

// activityHeight is the middle band, less the task list while tasks run.
// The list takes at most half the band, so Activity stays readable.
func (m Model) activityHeight() int {
	_, bodyH, _ := splitHeight(m.height)
	if !m.tasks.Active() {
		return bodyH
	}
	return bodyH - min(m.tasks.Height(), bodyH/2)
}

// splitHeight gives the header a fixed bar, the chat about a third of what
// remains, and the middle band everything left over.
func splitHeight(height int) (header, body, footer int) {
	header = headerHeight
	rest := height - header
	if rest < 0 {
		return height, 0, 0
	}

	footer = rest / 3
	if footer < minFooterHeight {
		footer = minFooterHeight
	}
	// Leave the activity and terminal panes a few rows when the window is short.
	if footer > rest-minBodyHeight {
		footer = rest - minBodyHeight
	}
	if footer < 0 {
		footer = 0
	}

	body = rest - footer
	return header, body, footer
}

// splitWidth gives the activity pane 60% and the terminal the remaining 40%.
func splitWidth(width int) (left, right int) {
	left = width * leftShare / 100
	right = width - left
	return left, right
}
