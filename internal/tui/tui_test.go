package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

func newTestModel() Model {
	return Model{
		keys:      defaultKeys(),
		list:      list.New(nil, list.NewDefaultDelegate(), 0, 0),
		viewport:  viewport.New(0, 0),
		composer:  textarea.New(),
		styleName: "dark",
	}
}

// TestResizeBecomesReady guards the deadlock regression: handling a
// WindowSizeMsg must complete synchronously (it must NOT query the TTY via
// glamour) and flip the model to ready.
func TestResizeBecomesReady(t *testing.T) {
	m := newTestModel()
	if m.ready {
		t.Fatal("model should start not ready")
	}

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	got := updated.(Model)

	if !got.ready {
		t.Error("model should be ready after WindowSizeMsg")
	}
	if got.renderer == nil {
		t.Error("renderer should be set after resize")
	}
	if got.width != 120 || got.height != 40 {
		t.Errorf("size not stored: got %dx%d", got.width, got.height)
	}
}

// TestCtrlCQuits ensures Ctrl+C always yields tea.Quit, even mid-filter.
func TestCtrlCQuits(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should return a command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("ctrl+c command should be tea.Quit")
	}
}

// TestTabCyclesFocus verifies tab rotates sidebar → messages → composer → sidebar.
func TestTabCyclesFocus(t *testing.T) {
	m := newTestModel()
	want := []focusArea{focusMessages, focusComposer, focusSidebar}
	for i, w := range want {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = updated.(Model)
		if m.focus != w {
			t.Errorf("tab #%d: focus = %d, want %d", i+1, m.focus, w)
		}
	}
}

// TestSendEmptyIsNoop guards against posting blank messages.
func TestSendEmptyIsNoop(t *testing.T) {
	m := newTestModel()
	m.activeChannelID = "chan1"
	_, cmd := m.sendMessage()
	if cmd != nil {
		t.Error("empty composer should not send")
	}
}
