package tui

import "github.com/charmbracelet/lipgloss"

var (
	focusedColor = lipgloss.Color("212") // pink — the focused pane
	blurredColor = lipgloss.Color("240") // grey — everything else
	editingColor = lipgloss.Color("214") // orange — composer while editing

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Padding(0, 1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)

	// emojiSelStyle highlights the selected row in the emoji picker.
	emojiSelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("0")).
			Background(focusedColor)

	// help popup styles: bold pink section headings, fixed-width key column.
	helpHeadingStyle = lipgloss.NewStyle().
				Foreground(focusedColor).
				Bold(true)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250")).
			Width(14)
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
