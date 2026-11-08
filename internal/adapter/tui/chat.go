package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// fixedRows is what the title and the input line always take inside the chat pane.
const fixedRows = 2

// Chat is the footer: a title, the reason the last question failed (if it did),
// and the line the user types in.
type Chat struct {
	input     string // what the user is typing
	errorText string // why the last question failed
	cursorOn  bool   // the blinking bar, on or off
}

// View draws the pane, top to bottom: title, error rows, input line.
func (c Chat) View(width, height int) string {
	textW, textH := textSize(width, height)

	rows := []string{titleStyle.Render("Chat")}
	rows = append(rows, c.errorRows(textW, textH-fixedRows)...)
	rows = append(rows, c.inputLine())

	return box(strings.Join(rows, "\n"), width, height)
}

// inputLine is the prompt, then what the user typed, or a hint when it is empty.
// The bar sits at the end, where the next character will go. A space keeps the
// line from shifting when the bar blinks off.
func (c Chat) inputLine() string {
	cursor := " "
	if c.cursorOn {
		cursor = titleStyle.Render("|")
	}
	if c.input == "" {
		return "> " + cursor + mutedStyle.Render(" Ask KITE anything...")
	}
	return "> " + c.input + cursor
}

// errorRows wraps the error text to the pane width and keeps the last
// maxRows rows, so a long message cannot push the input out of the pane.
func (c Chat) errorRows(width, maxRows int) []string {
	if c.errorText == "" || maxRows <= 0 {
		return nil
	}
	wrapped := lipgloss.NewStyle().Width(width).Render(c.errorText)
	rows := strings.Split(wrapped, "\n")
	if len(rows) > maxRows {
		rows = rows[len(rows)-maxRows:]
	}
	return rows
}

