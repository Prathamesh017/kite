package tui

// Activity is the left pane. What it lists is still open.
type Activity struct{}

func (Activity) View(width, height int) string {
	return box(titleStyle.Render("Activity"), width, height)
}
