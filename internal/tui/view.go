package tui

import (
	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if !m.ready {
		return "Initializing…"
	}

	d := m.layout()

	sidebar := paneStyle(m.focus == focusSidebar).
		Width(d.sidebarInnerW).
		Height(d.sidebarInnerH).
		Render(m.list.View())

	messages := paneStyle(m.focus == focusMessages).
		Width(d.msgInnerW).
		Height(d.messagesInnerH).
		Render(m.viewport.View())

	composer := paneStyle(m.focus == focusComposer).
		Width(d.msgInnerW).
		Height(composerLines).
		Render(m.composer.View())

	right := lipgloss.JoinVertical(lipgloss.Left, messages, composer)
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, right)
	return lipgloss.JoinVertical(lipgloss.Left, body, m.footer())
}

func (m Model) footer() string {
	help := "tab switch · enter open · ctrl+s send · / filter · r refresh · q quit"
	status := statusStyle.Render(m.status)
	return footerStyle.Width(m.width).Render(status + "  —  " + help)
}
