package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestSplitPaths(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"a.txt", []string{"a.txt"}},
		{"a.txt b.png", []string{"a.txt", "b.png"}},
		{"  a.txt\tb.png ", []string{"a.txt", "b.png"}},
		{`"my file.txt"`, []string{"my file.txt"}},
		{`"my file.txt" other.png`, []string{"my file.txt", "other.png"}},
		{`my\ file.txt`, []string{"my file.txt"}},
		{"~/Dropbox/a.pdf", []string{"~/Dropbox/a.pdf"}},
		{"", nil},
		{"   ", nil},
	}
	for _, tc := range cases {
		if got := splitPaths(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("splitPaths(%q) = %#v, want %#v", tc.in, got, tc.want)
		}
	}
}

func TestExpandTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	cases := map[string]string{
		"~":          home,
		"~/a/b.txt":  filepath.Join(home, "a/b.txt"),
		"./rel.txt":  "./rel.txt",
		"/abs/x.txt": "/abs/x.txt",
		"~other/x":   "~other/x", // only the current user's home is expanded
		"a~b.txt":    "a~b.txt",
	}
	for in, want := range cases {
		got, err := expandTilde(in)
		if err != nil {
			t.Fatalf("expandTilde(%q): %v", in, err)
		}
		if got != want {
			t.Errorf("expandTilde(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExpandAttachmentsGlobAndValidation(t *testing.T) {
	dir := t.TempDir()
	one := filepath.Join(dir, "one.png")
	two := filepath.Join(dir, "two.png")
	empty := filepath.Join(dir, "empty.png")
	for _, p := range []string{one, two} {
		if err := os.WriteFile(p, []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	// A glob expands to every match, sorted for a stable attachment order.
	got, err := expandAttachments(filepath.Join(dir, "*o*.png"))
	if err != nil {
		t.Fatalf("expandAttachments: %v", err)
	}
	if want := []string{one, two}; !reflect.DeepEqual(got, want) {
		t.Errorf("glob = %v, want %v", got, want)
	}

	// Two explicit paths on one line, order preserved.
	got, err = expandAttachments(two + " " + one)
	if err != nil {
		t.Fatalf("expandAttachments: %v", err)
	}
	if want := []string{two, one}; !reflect.DeepEqual(got, want) {
		t.Errorf("two paths = %v, want %v", got, want)
	}

	bad := []struct {
		name, in, wantErr string
	}{
		{"missing", filepath.Join(dir, "nope.png"), "could not read"},
		{"directory", dir, "is a directory"},
		{"empty file", empty, "is empty"},
		{"glob matches nothing", filepath.Join(dir, "*.zzz"), "no files match"},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := expandAttachments(tc.in); err == nil {
				t.Fatal("expected an error")
			} else if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("got %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestAttachSummary(t *testing.T) {
	if got := attachSummary(nil); got != "" {
		t.Errorf("empty queue should render empty, got %q", got)
	}
	got := attachSummary([]string{"/tmp/a.txt"})
	if want := "[1 file: a.txt]"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	got = attachSummary([]string{"/tmp/a.txt", "/x/b.png"})
	if want := "[2 files: a.txt, b.png]"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// The footer is budgeted exactly one row, so neither the attach prompt nor a
// long queue summary may make View() outgrow the terminal.
func TestViewFitsTerminalWithAttachments(t *testing.T) {
	long := []string{
		"/very/long/path/to/a-report-with-a-long-name.pdf",
		"/another/quite/long/path/screenshot-2026-07-28-final.png",
		"/x/third-file-name-also-long.txt",
	}
	for _, sz := range [][2]int{{80, 24}, {40, 12}, {120, 40}} {
		w, h := sz[0], sz[1]
		for _, mode := range []string{"queued", "prompt"} {
			m := newTestModel()
			u, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
			got := u.(Model)
			got.pendingFiles = long
			got.status = "a fairly long status line about uploading things"
			if mode == "prompt" {
				got.attachMode = true
				got.attachInput.SetValue("/some/path/being/typed/right/now.png")
			}

			v := got.View()
			if gotH := lipgloss.Height(v); gotH > h-1 {
				t.Errorf("%dx%d %s: View is %d lines, want <= %d", w, h, mode, gotH, h-1)
			}
			if gotW := lipgloss.Width(v); gotW > w {
				t.Errorf("%dx%d %s: View width %d, want <= %d", w, h, mode, gotW, w)
			}
		}
	}
}

// newAttachModel returns a model with an open channel, ready to attach.
func newAttachModel(t *testing.T) (Model, string) {
	t.Helper()
	m := newTestModel()
	m.activeChannelID = "chan1"
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	return m, path
}

func TestCtrlOQueuesAttachment(t *testing.T) {
	m, path := newAttachModel(t)

	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	got := u.(Model)
	if !got.attachMode {
		t.Fatal("ctrl+o should open the attach prompt")
	}

	got.attachInput.SetValue(path)
	u, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = u.(Model)

	if got.attachMode {
		t.Error("prompt should close after a valid path")
	}
	if !reflect.DeepEqual(got.pendingFiles, []string{path}) {
		t.Errorf("pendingFiles = %v, want [%s]", got.pendingFiles, path)
	}
	if got.focus != focusComposer {
		t.Error("focus should move to the composer after attaching")
	}
}

// A bad path keeps the prompt open so it can be fixed in place.
func TestAttachBadPathKeepsPromptOpen(t *testing.T) {
	m, _ := newAttachModel(t)
	m.attachMode = true
	m.attachInput.SetValue(filepath.Join(t.TempDir(), "nope.txt"))

	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := u.(Model)

	if !got.attachMode {
		t.Error("prompt should stay open on error")
	}
	if len(got.pendingFiles) != 0 {
		t.Errorf("nothing should be queued, got %v", got.pendingFiles)
	}
	if !strings.Contains(got.status, "could not read") {
		t.Errorf("status should explain the failure, got %q", got.status)
	}
}

func TestAttachEmptyEnterClearsQueue(t *testing.T) {
	m, path := newAttachModel(t)
	m.pendingFiles = []string{path}
	m.attachMode = true
	m.attachInput.SetValue("")

	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := u.(Model)

	if len(got.pendingFiles) != 0 {
		t.Errorf("queue should be cleared, got %v", got.pendingFiles)
	}
	if got.attachMode {
		t.Error("prompt should close")
	}
}

func TestAttachEscCancelsWithoutClearing(t *testing.T) {
	m, path := newAttachModel(t)
	m.pendingFiles = []string{path}
	m.attachMode = true
	m.attachInput.SetValue("/whatever")

	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := u.(Model)

	if got.attachMode {
		t.Error("esc should close the prompt")
	}
	if !reflect.DeepEqual(got.pendingFiles, []string{path}) {
		t.Errorf("esc must not touch the queue, got %v", got.pendingFiles)
	}
}

// An attachment with no body is a valid send (same rule as `mm send -f`).
func TestSendWithAttachmentAndEmptyComposer(t *testing.T) {
	m, path := newAttachModel(t)
	m.pendingFiles = []string{path}

	u, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	got := u.(Model)

	if cmd == nil {
		t.Fatal("an attachment-only message should be sent")
	}
	if len(got.pendingFiles) != 0 {
		t.Errorf("queue should be consumed by the send, got %v", got.pendingFiles)
	}
}

// Without attachments an empty composer still sends nothing.
func TestSendEmptyWithNoAttachmentIsNoop(t *testing.T) {
	m, _ := newAttachModel(t)
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS}); cmd != nil {
		t.Error("empty message with no attachments should not send")
	}
}

func TestCtrlORefusedWhileEditing(t *testing.T) {
	m, _ := newAttachModel(t)
	m.editing = true

	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	got := u.(Model)

	if got.attachMode {
		t.Error("attaching to an edit is not possible, prompt should not open")
	}
	if !strings.Contains(got.status, "edit") {
		t.Errorf("status should explain why, got %q", got.status)
	}
}

func TestCtrlORefusedWithoutChannel(t *testing.T) {
	m := newTestModel()
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	if u.(Model).attachMode {
		t.Error("prompt should not open with no channel open")
	}
}

// Attachments must not survive a channel switch: they belong to the draft.
func TestChannelSwitchClearsAttachments(t *testing.T) {
	m, path := newAttachModel(t)
	m.pendingFiles = []string{path}
	m.list.SetItems([]list.Item{channelItem{id: "chan2", name: "other"}})

	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := u.(Model)

	if len(got.pendingFiles) != 0 {
		t.Errorf("attachments should be dropped on channel switch, got %v", got.pendingFiles)
	}
}
