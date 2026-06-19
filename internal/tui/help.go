package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// helpBinding is a single key → action row in the help popup.
type helpBinding struct{ key, desc string }

// helpSection groups bindings under a heading.
type helpSection struct {
	title string
	binds []helpBinding
}

// The help popup is laid out in two columns so the whole keymap fits on one
// screen. Keep these in sync with the handlers in update.go.
var (
	helpSectionsLeft = []helpSection{
		{"Global", []helpBinding{
			{"tab", "switch pane"},
			{"esc", "back to sidebar"},
			{"r", "refresh"},
			{"?", "toggle this help"},
			{"q", "quit"},
			{"ctrl+c", "quit (always)"},
		}},
		{"Channel list", []helpBinding{
			{"↑/↓ · j/k", "move selection"},
			{"enter", "open channel / DM"},
			{"/", "filter channels"},
			{"a", "alias the DM's user"},
			{"s", "scheduled viewer"},
		}},
		{"Pickers / modals", []helpBinding{
			{"j/k · ↑/↓", "move"},
			{"enter", "confirm"},
			{"x", "cancel (scheduled)"},
			{"esc · q", "close"},
		}},
	}

	helpSectionsRight = []helpSection{
		{"Messages", []helpBinding{
			{"↑/↓ · j/k", "scroll"},
			{"↑/k at top", "load older history"},
			{"+", "react to a message"},
			{"y", "copy Markdown"},
			{"i", "view images (chafa)"},
		}},
		{"Composer", []helpBinding{
			{"ctrl+s", "send"},
			{"ctrl+t", "schedule for later"},
			{"↑", "edit previous message"},
			{"↓", "newer / restore draft"},
			{":query", "emoji picker"},
		}},
	}
)

// helpBody renders the two-column keyboard-shortcut reference shown by the
// help popup (toggled with '?').
func (m Model) helpBody() string {
	left := renderHelpColumn(helpSectionsLeft)
	right := renderHelpColumn(helpSectionsRight)
	cols := lipgloss.JoinHorizontal(lipgloss.Top, left, "    ", right)
	return statusStyle.Render("Keyboard shortcuts") + "\n\n" + cols
}

func renderHelpColumn(sections []helpSection) string {
	var b strings.Builder
	for si, s := range sections {
		if si > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(helpHeadingStyle.Render(s.title) + "\n")
		for _, bd := range s.binds {
			b.WriteString(helpKeyStyle.Render(bd.key) + bd.desc + "\n")
		}
	}
	return b.String()
}
