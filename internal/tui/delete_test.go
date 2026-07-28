package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/carlosprados/mm/internal/client"
)

// newDeleteModel returns a model focused on the message pane with two of the
// user's own posts and one from somebody else.
func newDeleteModel() Model {
	m := newTestModel()
	m.mm = &client.MM{Username: "me"}
	m.activeChannelID = "chan1"
	m.focus = focusMessages
	m.posts = []postLine{
		{postID: "p1", time: "09:00", author: "@me", message: "mine one"},
		{postID: "p2", time: "09:01", author: "@other", message: "theirs"},
		{postID: "p3", time: "09:02", author: "@me", message: "mine two", fileIDs: []string{"f1"}},
	}
	return m
}

func TestDeletePickerOffersOnlyOwnPosts(t *testing.T) {
	m := newDeleteModel()

	u, _ := m.Update(tea.KeyMsg{Runes: []rune{'d'}, Type: tea.KeyRunes})
	got := u.(Model)

	if !got.deleteMode {
		t.Fatal("'d' should open the delete picker")
	}
	if len(got.deleteCandidates) != 2 {
		t.Fatalf("got %d candidates, want 2 (only own posts)", len(got.deleteCandidates))
	}
	for _, idx := range got.deleteCandidates {
		if got.posts[idx].author != "@me" {
			t.Errorf("candidate %d is %q, not the user's own", idx, got.posts[idx].author)
		}
	}
	// Defaults to the most recent of your own messages.
	if got.posts[got.deleteCandidates[got.deleteCursor]].postID != "p3" {
		t.Error("cursor should start on your most recent message")
	}
}

func TestDeleteRefusedWithoutOwnPosts(t *testing.T) {
	m := newDeleteModel()
	m.posts = []postLine{{postID: "x", author: "@other", message: "theirs"}}

	u, _ := m.Update(tea.KeyMsg{Runes: []rune{'d'}, Type: tea.KeyRunes})
	got := u.(Model)

	if got.deleteMode {
		t.Error("picker should not open with nothing of yours to delete")
	}
	if !strings.Contains(got.status, "no messages of yours") {
		t.Errorf("status should explain why, got %q", got.status)
	}
}

// Deleting takes two deliberate keys: enter to pick, y to confirm.
func TestDeleteNeedsConfirmation(t *testing.T) {
	m := newDeleteModel()
	m.deleteMode = true
	m.deleteCandidates = m.ownPostIndices()
	m.deleteCursor = len(m.deleteCandidates) - 1

	u, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := u.(Model)
	if cmd != nil {
		t.Fatal("enter must only ask for confirmation, never delete")
	}
	if !got.deleteConfirm {
		t.Fatal("enter should enter the confirmation state")
	}

	u, cmd = got.Update(tea.KeyMsg{Runes: []rune{'y'}, Type: tea.KeyRunes})
	got = u.(Model)
	if cmd == nil {
		t.Error("y should issue the delete")
	}
	if got.deleteMode || got.deleteConfirm {
		t.Error("the picker should close after confirming")
	}
}

func TestDeleteConfirmationBacksOutOnAnyOtherKey(t *testing.T) {
	m := newDeleteModel()
	m.deleteMode = true
	m.deleteConfirm = true
	m.deleteCandidates = m.ownPostIndices()

	u, cmd := m.Update(tea.KeyMsg{Runes: []rune{'n'}, Type: tea.KeyRunes})
	got := u.(Model)

	if cmd != nil {
		t.Error("only 'y' may delete")
	}
	if got.deleteConfirm {
		t.Error("confirmation should be dismissed")
	}
	if !got.deleteMode {
		t.Error("backing out of the confirmation returns to the picker")
	}
}

func TestDeleteEscClosesPicker(t *testing.T) {
	m := newDeleteModel()
	m.deleteMode = true
	m.deleteCandidates = m.ownPostIndices()

	u, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Error("esc must not delete")
	}
	if u.(Model).deleteMode {
		t.Error("esc should close the picker")
	}
}

// Regression: EVERY modal with content must respect the layout invariant. Before
// clampBody, a body with more rows than the pane — or with lines wider than it,
// which wrap — grew the frame into the reserved bottom row.
func TestViewFitsTerminalWithModalContent(t *testing.T) {
	modals := map[string]func(*Model){
		"copy":  func(m *Model) { m.copyMode = true },
		"react": func(m *Model) { m.reactMode = true },
		"help":  func(m *Model) { m.helpMode = true },
		"images": func(m *Model) {
			m.imagePickMode = true
			m.imageAttachments = []imageAttachment{{label: "09:00 @me — a-very-long-screenshot-name.png"}}
		},
		"delete": func(m *Model) { m.deleteMode = true; m.deleteCandidates = m.ownPostIndices() },
	}
	for name, open := range modals {
		for _, sz := range [][2]int{{80, 24}, {40, 12}, {60, 10}} {
			w, h := sz[0], sz[1]
			m := newDeleteModel()
			u, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
			got := u.(Model)
			open(&got)

			v := got.View()
			if gotH := lipgloss.Height(v); gotH > h-1 {
				t.Errorf("%s at %dx%d: View is %d lines, want <= %d", name, w, h, gotH, h-1)
			}
			if gotW := lipgloss.Width(v); gotW > w {
				t.Errorf("%s at %dx%d: View width %d, want <= %d", name, w, h, gotW, w)
			}
		}
	}
}

func TestClampBody(t *testing.T) {
	body := "one\ntwo\nthree\nfour"
	if got := clampBody(body, 2, 10); got != "one\ntwo" {
		t.Errorf("rows: got %q", got)
	}
	if got := clampBody(body, 10, 10); got != body {
		t.Errorf("short body should be untouched, got %q", got)
	}
	if got := clampBody("aaaaaaaaaa", 1, 4); lipgloss.Width(got) > 4 {
		t.Errorf("width not clamped: %q is %d wide", got, lipgloss.Width(got))
	}
	// Styled text must stay intact enough to render at the clamped width.
	styled := emojiSelStyle.Render("selected row that is far too wide")
	if got := clampBody(styled, 1, 8); lipgloss.Width(got) > 8 {
		t.Errorf("styled line not clamped: %d wide", lipgloss.Width(got))
	}
	if got := clampBody(body, 0, 10); got != "" {
		t.Errorf("no rows means no body, got %q", got)
	}
}

func TestVisibleWindow(t *testing.T) {
	cases := []struct{ n, cursor, rows, wantStart, wantEnd int }{
		{3, 0, 10, 0, 3},  // everything fits
		{10, 0, 4, 0, 4},  // cursor at the top
		{10, 9, 4, 6, 10}, // cursor at the bottom
		{10, 5, 4, 3, 7},  // cursor centred
		{10, 5, 0, 0, 10}, // no room reported: don't hide anything
	}
	for _, c := range cases {
		start, end := visibleWindow(c.n, c.cursor, c.rows)
		if start != c.wantStart || end != c.wantEnd {
			t.Errorf("visibleWindow(%d,%d,%d) = (%d,%d), want (%d,%d)",
				c.n, c.cursor, c.rows, start, end, c.wantStart, c.wantEnd)
		}
		if c.rows > 0 && c.n > c.rows && (c.cursor < start || c.cursor >= end) {
			t.Errorf("cursor %d fell outside the window [%d,%d)", c.cursor, start, end)
		}
	}
}

// The picker and its confirmation are modals: they must respect the layout
// invariant like every other pane.
func TestViewFitsTerminalWhileDeleting(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {40, 12}, {120, 40}} {
		w, h := sz[0], sz[1]
		for _, confirm := range []bool{false, true} {
			m := newDeleteModel()
			u, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
			got := u.(Model)
			got.deleteMode = true
			got.deleteCandidates = got.ownPostIndices()
			got.deleteCursor = 0
			got.deleteConfirm = confirm

			v := got.View()
			if gotH := lipgloss.Height(v); gotH > h-1 {
				t.Errorf("%dx%d confirm=%v: View is %d lines, want <= %d", w, h, confirm, gotH, h-1)
			}
			if gotW := lipgloss.Width(v); gotW > w {
				t.Errorf("%dx%d confirm=%v: View width %d, want <= %d", w, h, confirm, gotW, w)
			}
		}
	}
}
