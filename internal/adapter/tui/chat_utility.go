package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"strings"
)

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
		c.errorText = ""
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

// Restore puts a failed question back in the input, with the reason above it.
func (c *Chat) Restore(question, reason string) {
	c.busy = false
	c.input = question
	c.errorText = reason
}
