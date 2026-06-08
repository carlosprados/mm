package tui

import (
	"fmt"
	"strings"

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

	composerStyle := paneStyle(m.focus == focusComposer).
		Width(d.msgInnerW).
		Height(composerLines)
	if m.editing {
		composerStyle = composerStyle.BorderForeground(editingColor)
	}
	composer := composerStyle.Render(m.composer.View())

	rightParts := []string{messages}
	if d.popupRows > 0 {
		rightParts = append(rightParts, m.emojiPopupView(d.msgInnerW, d.popupRows))
	}
	rightParts = append(rightParts, composer)
	right := lipgloss.JoinVertical(lipgloss.Left, rightParts...)

	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, right)
	return lipgloss.JoinVertical(lipgloss.Left, body, m.footer())
}

func (m Model) emojiPopupView(width, rows int) string {
	var b strings.Builder
	for i := 0; i < rows; i++ {
		e := m.emojiMatches[i]
		line := fmt.Sprintf("%s  :%s:", e.glyph, e.short)
		if i == m.emojiIndex {
			line = emojiSelStyle.Render(line)
		}
		b.WriteString(line)
		if i < rows-1 {
			b.WriteByte('\n')
		}
	}
	return paneStyle(true).Width(width).Height(rows).Render(b.String())
}

func (m Model) footer() string {
	if m.aliasMode {
		return footerStyle.Width(m.width).
			Render("alias for @" + m.aliasUser + ": " + m.aliasInput.View() + "  (enter saves · esc cancels)")
	}
	if m.scheduleMode {
		return footerStyle.Width(m.width).
			Render("deliver at: " + m.scheduleInput.View() + "  (enter schedules · esc cancels)")
	}
	help := "tab · enter open · ctrl+s send · ctrl+t schedule · : emoji · a alias · q quit"
	status := statusStyle.Render(m.status)
	return footerStyle.Width(m.width).Render(status + "  —  " + help)
}
