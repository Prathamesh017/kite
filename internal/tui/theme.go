package tui

import "github.com/charmbracelet/lipgloss"

// Theme is the palette the whole frame is drawn with.
type Theme struct {
	Name   string
	Border lipgloss.Color // panel borders
	Brand  lipgloss.Color // the KITE name
	Title  lipgloss.Color // pane titles and status text
	Muted  lipgloss.Color // version, hints, placeholders
	Ready  lipgloss.Color // the status dot
}

// themes are cycled with the t key. The first one is used at startup.
var themes = []Theme{
	{
		Name:   "Sky Kite",
		Border: "#0EA5E9",
		Brand:  "#38BDF8",
		Title:  "#E0F2FE",
		Muted:  "#64748B",
		Ready:  "#4ADE80",
	},
	{
		Name:   "Deep Ocean",
		Border: "#1E40AF",
		Brand:  "#60A5FA",
		Title:  "#DBEAFE",
		Muted:  "#475569",
		Ready:  "#34D399",
	},
	{
		Name:   "Tokyo Night",
		Border: "#3B4261",
		Brand:  "#7AA2F7",
		Title:  "#C0CAF5",
		Muted:  "#565F89",
		Ready:  "#9ECE6A",
	},
	{
		Name:   "Nord Frost",
		Border: "#4C566A",
		Brand:  "#88C0D0",
		Title:  "#ECEFF4",
		Muted:  "#616E88",
		Ready:  "#A3BE8C",
	},
	{
		Name:   "Catppuccin",
		Border: "#45475A",
		Brand:  "#89B4FA",
		Title:  "#CDD6F4",
		Muted:  "#6C7086",
		Ready:  "#A6E3A1",
	},
	{
		Name:   "One Dark",
		Border: "#3E4451",
		Brand:  "#61AFEF",
		Title:  "#ABB2BF",
		Muted:  "#5C6370",
		Ready:  "#98C379",
	},
	{
		Name:   "GitHub Dark",
		Border: "#30363D",
		Brand:  "#58A6FF",
		Title:  "#E6EDF3",
		Muted:  "#7D8590",
		Ready:  "#3FB950",
	},
	{
		Name:   "Solarized",
		Border: "#073642",
		Brand:  "#268BD2",
		Title:  "#93A1A1",
		Muted:  "#586E75",
		Ready:  "#859900",
	},
}

// applyTheme rebuilds the shared styles from t. Panes read those styles on
// every render, so the next frame picks up the new colors.
func applyTheme(t Theme) {
	panelStyle = panelStyle.BorderForeground(t.Border)
	brandStyle = lipgloss.NewStyle().Foreground(t.Brand).Bold(true)
	versionStyle = lipgloss.NewStyle().Foreground(t.Muted)
	readyStyle = lipgloss.NewStyle().Foreground(t.Ready)
	statusStyle = lipgloss.NewStyle().Foreground(t.Title)
	titleStyle = lipgloss.NewStyle().Foreground(t.Title).Bold(true)
	mutedStyle = lipgloss.NewStyle().Foreground(t.Muted)
}
