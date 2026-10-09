package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Chat is the footer. input is what the user is typing. output is the last reply.
type Chat struct {
	input  string
	output string
	busy   bool
}

// HandleKey edits the input. Enter returns the question and clears the line.
// Keys are ignored while a reply is in flight.
func (c *Chat) HandleKey(msg tea.KeyMsg) string {
	if c.busy {
		return ""
	}
	switch msg.Type {
	case tea.KeyEnter:
		text := strings.TrimSpace(c.input)
		c.input = ""
		if text == "" {
			return ""
		}
		c.busy = true
		return text
	case tea.KeyBackspace, tea.KeyDelete:
		r := []rune(c.input)
		if len(r) > 0 {
			c.input = string(r[:len(r)-1])
		}
	case tea.KeySpace:
		c.input += " "
	case tea.KeyRunes:
		c.input += string(msg.Runes)
	}
	return ""
}

// Show replaces the reply text and unlocks the input.
func (c *Chat) Show(text string) {
	c.busy = false
	c.output = text
}

func (c Chat) View(width, height int) string {
	textW, textH := textSize(width, height)

	input := mutedStyle.Render("> Ask KITE anything...")
	if c.busy {
		input = mutedStyle.Render("thinking...")
	} else if c.input != "" {
		input = "> " + c.input
	}

	room := textH - 2
	if room < 0 {
		room = 0
	}
	var wrapped []string
	if c.output != "" && room > 0 {
		rendered := lipgloss.NewStyle().Width(textW).Render(c.output)
		wrapped = strings.Split(rendered, "\n")
		if len(wrapped) > room {
			wrapped = wrapped[len(wrapped)-room:]
		}
	}

	out := make([]string, 0, 2+len(wrapped))
	out = append(out, titleStyle.Render("Chat"))
	out = append(out, wrapped...)
	out = append(out, input)
	return box(strings.Join(out, "\n"), width, height)
}
