package tui

import (
	"encoding/json"
	"github.com/charmbracelet/lipgloss"
	"kite/internal/core"
	"strings"
)

// Activity is the middle pane. It lists the conversation.
// conversations is the UI's own list of what is shown. It is filled when a reply arrives.
// pending is the question sent but not answered yet. It is shown after the
// conversation, because a question joins the list only once it was answered.
// steps are the tool lines of the run in progress. They are shown after pending
// and join the conversation, between the question and the answer, when the answer arrives.
// status says what Kite is doing now, like "thinking". It is only a marker while
// waiting, not part of the conversation. detail is the text of StatusTool.
// frame moves the shimmer along the status.
type Activity struct {
	conversations []core.Message
	pending       string
	steps         []core.Message
	status        Status
	detail        string
	frame         int
}

// Status is what Kite is doing while it waits. Idle means nothing to show.
type Status int

const (
	StatusIdle Status = iota
	StatusThinking
	StatusTool          // a tool is about to run, and detail says which
	StatusCreatingTasks // the model broke the request into tasks
)

// statusText is the text shown for the status.
func (a Activity) statusText() string {
	switch a.status {
	case StatusThinking:
		return "thinking"
	case StatusTool:
		return a.detail
	case StatusCreatingTasks:
		return "creating todo list"
	}
	return ""
}

// toolStatus words what the tools are doing, like "reading main.go".
func toolStatus(calls []core.ToolCall) string {
	parts := make([]string, 0, len(calls))
	for _, call := range calls {
		var args struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal([]byte(call.Arguments), &args)
		switch call.Name {
		case "read_file":
			parts = append(parts, "reading "+args.Path)
		case "write_file":
			parts = append(parts, "writing "+args.Path)
		case "list_files":
			parts = append(parts, "listing files")
		default:
			parts = append(parts, "using "+call.Name)
		}
	}
	return strings.Join(parts, ", ")
}

// Shimmer moves the highlight one step, only while there is a status to show.
func (a *Activity) Shimmer() {
	if a.status != StatusIdle {
		a.frame++
	}
}

func (a Activity) View(width, height int) string {
	rows, room := a.rows(width, height)
	maxTop := max(0, len(rows)-room)

	top := maxTop
	visible := rows[top:min(top+room, len(rows))]

	out := make([]string, 0, 1+len(visible))
	out = append(out, titleStyle.Render("Activity"))
	out = append(out, visible...)
	return box(strings.Join(out, "\n"), width, height)
}

func (a Activity) rows(width, height int) (rows []string, room int) {
	textW, textH := textSize(width, height)
	room = max(textH-1, 0)

	for _, line := range a.conversations {
		rows = append(rows, row(line.Role, line.Message, textW)...)
	}
	if a.pending != "" {
		rows = append(rows, row("user", a.pending, textW)...)
	}
	for _, step := range a.steps {
		rows = append(rows, row(step.Role, step.Message, textW)...)
	}
	if a.status != StatusIdle {
		rows = append(rows, shimmer("> "+a.statusText()+"...", a.frame))
	}
	return rows, room
}

// shimmer draws text muted, with a few brighter letters around a point that
// moves one letter per frame and wraps around, so the line looks in progress.
func shimmer(text string, frame int) string {
	letters := []rune(text)
	const spread = 2
	cycle := len(letters) + 2*spread
	pos := frame%cycle - spread

	var b strings.Builder
	for i, r := range letters {
		if i >= pos-spread && i <= pos+spread {
			b.WriteString(titleStyle.Render(string(r)))
		} else {
			b.WriteString(mutedStyle.Render(string(r)))
		}
	}
	return b.String()
}

// row is one message wrapped to the pane width.
func row(role, text string, width int) []string {
	line := marker(role) + "  " + text
	if role == "tool" {
		line = mutedStyle.Render("> using tool call - " + text)
	}
	rendered := lipgloss.NewStyle().Width(width).Render(line)
	return strings.Split(rendered, "\n")
}

func marker(role string) string {
	if role == "assistant" {
		return readyStyle.Render("> kite")
	}
	return brandStyle.Render("> you")
}
