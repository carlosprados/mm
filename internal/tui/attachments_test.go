package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/carlosprados/mm/internal/client"
)

func newFilePickModel(n int) Model {
	m := newTestModel()
	m.mm = &client.MM{Username: "me"}
	m.focus = focusMessages
	m.filePickMode = true
	for i := 0; i < n; i++ {
		f := client.Attachment{ID: fmt.Sprintf("f%d", i), Name: fmt.Sprintf("file-%02d.pdf", i), MimeType: "application/pdf"}
		m.fileAttachments = append(m.fileAttachments, fileAttachment{label: f.Name, file: f})
	}
	m.filePickCursor = n - 1
	return m
}

// The picker opens on the most recent attachment, the last row: with more
// attachments than rows it must still be on screen.
func TestFilePickerShowsCursor(t *testing.T) {
	m := newFilePickModel(40)
	u, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	got := u.(Model)

	v := got.View()
	if !strings.Contains(v, "file-39.pdf") {
		t.Errorf("selected attachment not visible:\n%s", v)
	}
	if !strings.Contains(v, "older") {
		t.Errorf("expected an \"older\" hint for the hidden rows:\n%s", v)
	}
}

func TestFilePickerViewRefusesNonImages(t *testing.T) {
	m := newFilePickModel(1)
	u, cmd := m.handleFileKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	got := u.(Model)
	if cmd != nil {
		t.Error("v on a PDF should not start a download")
	}
	if !got.filePickMode {
		t.Error("picker should stay open so another file can be chosen")
	}
}

func TestFilePickerEnterSaves(t *testing.T) {
	m := newFilePickModel(1)
	u, cmd := m.handleFileKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should start the download")
	}
	if u.(Model).filePickMode {
		t.Error("picker should close once the download starts")
	}
}
