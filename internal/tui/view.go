package tui

import (
	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if !m.ready {
		return "Initializing…"
	}

	paneInnerH := m.height - 1 - 2 // footer + pane border
	sidebarInner := sidebarWidth - 2
	msgInner := m.width - sidebarWidth - 2
	if msgInner < 10 {
		msgInner = 10
	}

	sidebar := paneStyle(m.focus == focusSidebar).
		Width(sidebarInner).
		Height(paneInnerH).
		Render(m.list.View())

	messages := paneStyle(m.focus == focusMessages).
		Width(msgInner).
		Height(paneInnerH).
		Render(m.viewport.View())

	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, messages)
	return lipgloss.JoinVertical(lipgloss.Left, body, m.footer())
}

func (m Model) footer() string {
	help := "tab switch · j/k move · enter open · / filter · r refresh · q quit"
	status := statusStyle.Render(m.status)
	return footerStyle.Width(m.width).Render(status + "  —  " + help)
}
