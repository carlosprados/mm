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

	// The list has a minimum height of its own (title + pagination), so on short
	// terminals it does not shrink to the budget and would grow the frame past
	// the reserved bottom row. Clamp it like the modal bodies.
	sidebar := paneStyle(m.focus == focusSidebar).
		Width(d.sidebarInnerW).
		Height(d.sidebarInnerH).
		Render(clampBody(m.list.View(), d.sidebarInnerH, d.sidebarInnerW))

	var body string

	// Modal pickers take over the right column.
	if m.helpMode || m.scheduleViewMode || m.copyMode || m.imagePickMode || m.reactMode || m.deleteMode {
		var bodyText string
		switch {
		case m.helpMode:
			bodyText = m.helpBody()
		case m.deleteMode:
			bodyText = m.deletePickerBody(d.sidebarInnerH)
		case m.copyMode:
			bodyText = m.copyPickerBody()
		case m.imagePickMode:
			bodyText = m.imagePickerBody()
		case m.reactMode:
			bodyText = m.reactBody()
		default:
			bodyText = m.scheduleViewBody()
		}
		// Clamp the body to the pane's rows: a taller body grows the pane and
		// pushes the frame into the reserved bottom row, which scrolls the
		// terminal and eats the top border (see the layout invariants).
		right := paneStyle(true).
			Width(d.msgInnerW).
			Height(d.sidebarInnerH).
			Render(clampBody(bodyText, d.sidebarInnerH, d.msgInnerW))
		body = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, right)
	} else {
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

		body = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, right)
	}

	// Last line of defence: panes carry minimum sizes of their own (the sidebar
	// list, the composer), so on a very short terminal the assembled body can
	// still exceed its budget. Clamping here keeps the reserved bottom row free
	// whatever the panes do — at the cost of a chopped bottom border below ~10
	// rows, which beats a frame that scrolls the terminal.
	body = clampBody(body, d.contentH, m.width)

	out := lipgloss.JoinVertical(lipgloss.Left, body, m.footer())
	// Hard clamp to the real terminal size. A frame one line too tall scrolls
	// the terminal and desyncs Bubble Tea's cursor, which is what leaves residue
	// and eats the panes' top borders on resize.
	return lipgloss.NewStyle().MaxWidth(m.width).MaxHeight(m.height).Render(out)
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

// clampBody fits a modal body into the rows and columns its pane was budgeted.
// Both axes matter: extra lines grow the pane directly, and an over-wide line
// wraps, which grows it just the same. Truncation goes through lipgloss so ANSI
// styling is never cut mid-escape.
func clampBody(s string, rows, width int) string {
	if rows <= 0 || width <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > rows {
		lines = lines[:rows]
	}
	trunc := lipgloss.NewStyle().MaxWidth(width)
	for i, line := range lines {
		lines[i] = trunc.Render(line)
	}
	return strings.Join(lines, "\n")
}

// visibleWindow returns the [start, end) slice of n rows to show in a picker
// that has room for `rows`, keeping the cursor inside the window.
func visibleWindow(n, cursor, rows int) (int, int) {
	if rows <= 0 || n <= rows {
		return 0, n
	}
	start := cursor - rows/2
	if start < 0 {
		start = 0
	}
	if start+rows > n {
		start = n - rows
	}
	return start, start + rows
}

// deletePickerBody lists only your own messages, and switches to a confirmation
// once one is picked. maxRows is how many terminal rows the modal pane has: the
// list is windowed around the cursor so the message you are about to delete is
// always the one you can see.
func (m Model) deletePickerBody(maxRows int) string {
	title := statusStyle.Render("Delete one of your messages")
	if m.deleteConfirm {
		i := m.deleteCandidates[m.deleteCursor]
		p := m.posts[i]
		body := title + "\n\n  " + deleteWarnStyle.Render("This cannot be undone. It disappears for everyone.") + "\n\n"
		body += fmt.Sprintf("  %s  %s: %s\n", p.time, p.author, firstLineTUI(p.message))
		if len(p.fileIDs) > 0 {
			body += fmt.Sprintf("  %s\n", deleteWarnStyle.Render(
				fmt.Sprintf("⚠ %d attachment(s) go with it", len(p.fileIDs))))
		}
		return body + "\n  press y to delete · any other key cancels"
	}

	var b strings.Builder
	b.WriteString(title + "\n\n")
	start, end := visibleWindow(len(m.deleteCandidates), m.deleteCursor, maxRows-2) // 2 = title + blank
	if start > 0 {
		b.WriteString(fmt.Sprintf("  … %d older\n", start))
		start++ // the hint occupies a row
	}
	for i := start; i < end; i++ {
		p := m.posts[m.deleteCandidates[i]]
		line := fmt.Sprintf("%s  %s: %s", p.time, p.author, firstLineTUI(p.message))
		if i == m.deleteCursor {
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
	if m.helpMode {
		return footerStyle.Width(m.width).Render("help · any key closes")
	}
	if m.aliasMode {
		return footerStyle.Width(m.width).
			Render("alias for @" + m.aliasUser + ": " + m.aliasInput.View() + "  (enter saves · esc cancels)")
	}
	if m.scheduleMode {
		return footerStyle.Width(m.width).
			Render("deliver at: " + m.scheduleInput.View() + "  (enter schedules · esc cancels)")
	}
	if m.attachMode {
		return footerStyle.Width(m.width).
			Render("attach: " + m.attachInput.View() + "  (enter adds · empty enter clears · esc cancels)")
	}
	if m.scheduleViewMode {
		return footerStyle.Width(m.width).Render("scheduled · j/k move · x cancel · esc close")
	}
	if m.deleteMode {
		if m.deleteConfirm {
			return footerStyle.Width(m.width).Render("delete · y confirms (irreversible) · any other key cancels")
		}
		return footerStyle.Width(m.width).Render("delete · j/k move · enter picks · esc close")
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
	// Queued attachments displace the generic help: what's about to be sent
	// matters more than the keymap, which '?' shows in full anyway.
	if len(m.pendingFiles) > 0 {
		return footerStyle.Width(m.width).Render(
			statusStyle.Render(attachSummary(m.pendingFiles)) +
				"  —  ctrl+s sends · ctrl+o adds more · " + m.status)
	}
	help := "enter open · ctrl+s send · ctrl+o attach · + react · y copy · i images · ? help · q quit"
	status := statusStyle.Render(m.status)
	return footerStyle.Width(m.width).Render(status + "  —  " + help)
}
