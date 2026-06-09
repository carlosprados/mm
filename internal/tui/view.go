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

	// Modal pickers take over the right column.
	if m.scheduleViewMode || m.copyMode || m.imagePickMode || m.reactMode {
		var bodyText string
		switch {
		case m.copyMode:
			bodyText = m.copyPickerBody()
		case m.imagePickMode:
			bodyText = m.imagePickerBody()
		case m.reactMode:
			bodyText = m.reactBody()
		default:
			bodyText = m.scheduleViewBody()
		}
		right := paneStyle(true).
			Width(d.msgInnerW).
			Height(d.sidebarInnerH).
			Render(bodyText)
		body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, right)
		return lipgloss.JoinVertical(lipgloss.Left, body, m.footer())
	}

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

func (m Model) reactBody() string {
	if m.reactPhase == 0 {
		title := statusStyle.Render("React — pick a message")
		if len(m.posts) == 0 {
			return title + "\n\n  No messages."
		}
		var b strings.Builder
		b.WriteString(title + "\n\n")
		for i, p := range m.posts {
			line := fmt.Sprintf("%s  %s: %s", p.time, p.author, firstLineTUI(p.message))
			if i == m.reactCursor {
				line = emojiSelStyle.Render(line)
			}
			b.WriteString("  " + line + "\n")
		}
		return b.String()
	}

	// phase 1: emoji search
	var b strings.Builder
	b.WriteString(statusStyle.Render("React — search emoji") + "\n\n")
	b.WriteString("  " + m.reactInput.View() + "\n\n")
	if len(m.reactMatches) == 0 {
		b.WriteString("  type at least 2 letters…")
		return b.String()
	}
	for i, e := range m.reactMatches {
		line := fmt.Sprintf("%s  :%s:", e.glyph, e.short)
		if i == m.reactEmojiCursor {
			line = emojiSelStyle.Render(line)
		}
		b.WriteString("  " + line + "\n")
	}
	return b.String()
}

func (m Model) imagePickerBody() string {
	title := statusStyle.Render("View an image (chafa)")
	if len(m.imageAttachments) == 0 {
		return title + "\n\n  No image attachments."
	}
	var b strings.Builder
	b.WriteString(title + "\n\n")
	for i, img := range m.imageAttachments {
		line := img.label
		if i == m.imagePickCursor {
			line = emojiSelStyle.Render(line)
		}
		b.WriteString("  " + line + "\n")
	}
	return b.String()
}

func (m Model) copyPickerBody() string {
	title := statusStyle.Render("Copy a message (Markdown)")
	if len(m.posts) == 0 {
		return title + "\n\n  No messages."
	}
	var b strings.Builder
	b.WriteString(title + "\n\n")
	for i, p := range m.posts {
		line := fmt.Sprintf("%s  %s: %s", p.time, p.author, firstLineTUI(p.message))
		if i == m.copyCursor {
			line = emojiSelStyle.Render(line)
		}
		b.WriteString("  " + line + "\n")
	}
	return b.String()
}

func (m Model) scheduleViewBody() string {
	title := statusStyle.Render("Scheduled messages")
	if len(m.scheduleView) == 0 {
		return title + "\n\n  None. Compose a message and press ctrl+t to schedule one."
	}
	var b strings.Builder
	b.WriteString(title + "\n\n")
	for i, it := range m.scheduleView {
		line := fmt.Sprintf("%s  →  %s: %s",
			it.At.Format("2006-01-02 15:04"), it.Label, firstLineTUI(it.Message))
		if i == m.scheduleViewCursor {
			line = emojiSelStyle.Render(line)
		}
		b.WriteString("  " + line + "\n")
	}
	return b.String()
}

func firstLineTUI(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i] + " …"
	}
	return s
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
	if m.scheduleViewMode {
		return footerStyle.Width(m.width).Render("scheduled · j/k move · x cancel · esc close")
	}
	if m.copyMode {
		return footerStyle.Width(m.width).Render("copy · j/k move · enter/y copy Markdown · esc close")
	}
	if m.imagePickMode {
		return footerStyle.Width(m.width).Render("images · j/k move · enter view · esc close")
	}
	if m.reactMode {
		if m.reactPhase == 0 {
			return footerStyle.Width(m.width).Render("react · j/k pick message · enter next · esc close")
		}
		return footerStyle.Width(m.width).Render("react · type emoji · ↑/↓ pick · enter apply · esc back")
	}
	help := "enter open · scroll-up=history · ctrl+s send · y copy · i images · + react · q quit"
	status := statusStyle.Render(m.status)
	return footerStyle.Width(m.width).Render(status + "  —  " + help)
}
