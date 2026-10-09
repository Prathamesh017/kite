package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"kite/internal/llm"
)

// Activity lists the conversation. The question is appended when it is sent.
// The answer is appended when it comes back. thinking is only a marker while waiting.
type Activity struct {
	conversions []llm.Message
	thinking    bool
}

func (a *Activity) AppendQuestion(question string) {
	a.conversions = append(a.conversions, llm.Message{Role: "user", Content: question})
	a.thinking = true
}

func (a *Activity) AppendAnswer(answer string) {
	a.conversions = append(a.conversions, llm.Message{Role: "assistant", Content: answer})
	a.thinking = false
}

func (a *Activity) StopThinking() {
	a.thinking = false
}

func (a Activity) View(width, height int) string {
	textW, textH := textSize(width, height)
	room := textH - 1
	if room < 0 {
		room = 0
	}

	var wrapped []string
	for _, line := range a.conversions {
		text := marker(line.Role) + "  " + line.Content
		rendered := lipgloss.NewStyle().Width(textW).Render(text)
		wrapped = append(wrapped, strings.Split(rendered, "\n")...)
	}
	if a.thinking {
		wrapped = append(wrapped, mutedStyle.Render("> thinking..."))
	}
	if len(wrapped) > room {
		wrapped = wrapped[len(wrapped)-room:]
	}

	out := make([]string, 0, 1+len(wrapped))
	out = append(out, titleStyle.Render("Activity"))
	out = append(out, wrapped...)
	return box(strings.Join(out, "\n"), width, height)
}

func marker(role string) string {
	if role == "assistant" {
		return readyStyle.Render("> kite")
	}
	return brandStyle.Render("> you")
}
