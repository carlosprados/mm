package tui

import "github.com/charmbracelet/lipgloss"

var (
	focusedColor = lipgloss.Color("212") // pink — the focused pane
	blurredColor = lipgloss.Color("240") // grey — everything else

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Padding(0, 1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)
)

// paneStyle returns a bordered box whose border color reflects focus.
func paneStyle(focused bool) lipgloss.Style {
	color := blurredColor
	if focused {
		color = focusedColor
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color)
}
