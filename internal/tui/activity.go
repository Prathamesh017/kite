package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"kite/internal/llm"
)

// Activity lists the conversation. The question is appended when it is sent.
// The answer is appended when it comes back. thinking and retrying are only
// markers while waiting; they are not part of the conversation.
//
// top is the first visible row while scrolledUp is true. When scrolledUp is
// false the pane follows the bottom, so new messages come into view.
type Activity struct {
	conversions []llm.Message
	thinking    bool
	retrying    bool
	top         int
	scrolledUp  bool
}

func (a *Activity) AppendQuestion(question string) {
	a.conversions = append(a.conversions, llm.Message{Role: "user", Content: question})
	a.thinking = true
	a.retrying = false
}

// AppendToolCall shows that a tool ran. It is only for display; the model's
// copy of the tool steps lives in the parent's history.
func (a *Activity) AppendToolCall(name string) {
	a.conversions = append(a.conversions, llm.Message{Role: "tool", Content: name})
}

func (a *Activity) AppendAnswer(answer string) {
	a.conversions = append(a.conversions, llm.Message{Role: "assistant", Content: answer})
	a.thinking = false
	a.retrying = false
}

// Retrying replaces thinking after the first failure.
func (a *Activity) Retrying() {
	a.retrying = true
}

// RemoveQuestion drops the unanswered question and any tool lines after it,
// and returns the question's text.
func (a *Activity) RemoveQuestion() string {
	a.thinking = false
	a.retrying = false
	for i := len(a.conversions) - 1; i >= 0; i-- {
		if a.conversions[i].Role == "user" {
			question := a.conversions[i].Content
			a.conversions = a.conversions[:i]
			return question
		}
	}
	return ""
}

// Scroll moves the view by delta rows. Negative is up. width and height are the
// pane's outer size, needed to know how many rows the conversation wraps into.
func (a *Activity) Scroll(delta, width, height int) {
	rows, room := a.rows(width, height)
	maxTop := max(0, len(rows)-room)

	top := a.top
	if !a.scrolledUp {
		top = maxTop
	}
	top = min(max(top+delta, 0), maxTop)

	a.top = top
	a.scrolledUp = top < maxTop
}

func (a Activity) View(width, height int) string {
	rows, room := a.rows(width, height)
	maxTop := max(0, len(rows)-room)

	top := maxTop
	if a.scrolledUp {
		top = min(a.top, maxTop)
	}
	visible := rows[top:min(top+room, len(rows))]

	out := make([]string, 0, 1+len(visible))
	out = append(out, titleStyle.Render("Activity"))
	out = append(out, visible...)
	return box(strings.Join(out, "\n"), width, height)
}

// rows is the conversation wrapped to the pane width, plus the waiting marker.
// room is how many of those rows fit under the title.
func (a Activity) rows(width, height int) (rows []string, room int) {
	textW, textH := textSize(width, height)
	room = max(textH-1, 0)

	for _, line := range a.conversions {
		text := marker(line.Role) + "  " + line.Content
		if line.Role == "tool" {
			text = mutedStyle.Render("> using tool call - " + line.Content)
		}
		rendered := lipgloss.NewStyle().Width(textW).Render(text)
		rows = append(rows, strings.Split(rendered, "\n")...)
	}
	if a.retrying {
		rows = append(rows, mutedStyle.Render("> retrying..."))
	} else if a.thinking {
		rows = append(rows, mutedStyle.Render("> thinking..."))
	}
	return rows, room
}

func marker(role string) string {
	if role == "assistant" {
		return readyStyle.Render("> kite")
	}
	return brandStyle.Render("> you")
}
