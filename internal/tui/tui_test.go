package tui

import (
	"sort"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/carlosprados/mm/internal/alias"
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

// newTestModelWithHistory returns a composing model with three own posts
// (newest-first) and an in-progress draft.
func newTestModelWithHistory() Model {
	m := newTestModel()
	m.focus = focusComposer
	m.activeChannelID = "chan1"
	m.ownPosts = []ownPost{
		{id: "p3", message: "third"},
		{id: "p2", message: "second"},
		{id: "p1", message: "first"},
	}
	m.composer.SetValue("draft")
	return m
}

// TestUpArrowEditWalk verifies up/down navigation through own messages and that
// the in-progress draft is preserved and restored.
func TestUpArrowEditWalk(t *testing.T) {
	m := newTestModelWithHistory()

	m = m.historyOlder()
	if !m.editing || m.editIndex != 0 || m.composer.Value() != "third" {
		t.Fatalf("first up: editing=%v idx=%d val=%q", m.editing, m.editIndex, m.composer.Value())
	}
	if m.savedDraft != "draft" {
		t.Errorf("draft not saved: %q", m.savedDraft)
	}

	m = m.historyOlder()
	m = m.historyOlder()
	if m.editIndex != 2 || m.composer.Value() != "first" {
		t.Fatalf("walk to oldest: idx=%d val=%q", m.editIndex, m.composer.Value())
	}
	// Already at the oldest — should not move further.
	m = m.historyOlder()
	if m.editIndex != 2 {
		t.Errorf("should stay at oldest, got idx=%d", m.editIndex)
	}

	// Walk back down to the draft.
	m = m.historyNewer()
	m = m.historyNewer()
	if m.editIndex != 0 || m.composer.Value() != "third" {
		t.Fatalf("walk back: idx=%d val=%q", m.editIndex, m.composer.Value())
	}
	m = m.historyNewer()
	if m.editing || m.composer.Value() != "draft" {
		t.Errorf("past newest should restore draft: editing=%v val=%q", m.editing, m.composer.Value())
	}
}

// TestChannelLess checks sidebar prioritization: unread first (recent on top),
// then channels, then DMs alphabetically.
func TestChannelLess(t *testing.T) {
	unreadOld := channelItem{name: "a", typ: "public", unread: true, lastPostAt: 100}
	unreadNew := channelItem{name: "z", typ: "dm", unread: true, lastPostAt: 200}
	readChan := channelItem{name: "dev", typ: "public"}
	readDM := channelItem{name: "@luis", typ: "dm"}

	// Unread before read.
	if !channelLess(unreadOld, readChan) {
		t.Error("unread should sort before read")
	}
	// Among unread, more recent first.
	if !channelLess(unreadNew, unreadOld) {
		t.Error("more recent unread should sort first")
	}
	// Among read, channels before DMs.
	if !channelLess(readChan, readDM) {
		t.Error("read channel should sort before read DM")
	}

	items := []channelItem{readDM, unreadOld, readChan, unreadNew}
	sort.SliceStable(items, func(i, j int) bool { return channelLess(items[i], items[j]) })
	order := []string{items[0].name, items[1].name, items[2].name, items[3].name}
	want := []string{"z", "a", "dev", "@luis"} // unreadNew, unreadOld, readChan, readDM
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order = %v, want %v", order, want)
			break
		}
	}
}

// TestShellQuote ensures paths are safely single-quoted for the chafa exec.
func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"/tmp/mm-123.png": "'/tmp/mm-123.png'",
		"/tmp/a b.png":    "'/tmp/a b.png'",
		"/tmp/it's.png":   `'/tmp/it'\''s.png'`,
		"; rm -rf ~":      `'; rm -rf ~'`,
	}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestDMLabel checks alias-aware DM labelling for the sidebar.
func TestDMLabel(t *testing.T) {
	store := &alias.Store{Aliases: map[string]string{"luis": "luisdavid.francisco"}}

	title, desc := dmLabel("luisdavid.francisco", store)
	if title != "luis" || desc != "@luisdavid.francisco" {
		t.Errorf("aliased DM: got (%q, %q)", title, desc)
	}

	title, desc = dmLabel("marta.gomez", store)
	if title != "@marta.gomez" || desc != "dm" {
		t.Errorf("plain DM: got (%q, %q)", title, desc)
	}

	title, _ = dmLabel("someone", nil)
	if title != "@someone" {
		t.Errorf("nil store: got %q", title)
	}
}

// TestActiveEmojiQuery checks the trigger heuristic.
func TestActiveEmojiQuery(t *testing.T) {
	cases := map[string]string{
		"hello :sm":    "sm",
		"see :smile":   "smile",
		"x\n:gr":       "gr",
		"hi":           "",    // no colon
		"10:30":        "",    // colon not at a word boundary
		":a":           "",    // too short (<2)
		"done :smile ": "",    // trailing space closes the token
		":SMI":         "smi", // lowercased
	}
	for in, want := range cases {
		if got := activeEmojiQuery(in); got != want {
			t.Errorf("activeEmojiQuery(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSearchEmoji checks matching and the result cap.
func TestSearchEmoji(t *testing.T) {
	res := searchEmoji("smile")
	if len(res) == 0 {
		t.Fatal("expected matches for 'smile'")
	}
	if len(res) > maxEmojiResults {
		t.Errorf("results not capped: %d", len(res))
	}
	if !strings.HasPrefix(res[0].short, "smile") {
		t.Errorf("prefix match should rank first, got %q", res[0].short)
	}
	for _, e := range res {
		if !strings.Contains(e.short, "smile") {
			t.Errorf("unexpected match %q", e.short)
		}
	}
}

// TestAcceptEmoji replaces the trailing token with the chosen glyph.
func TestAcceptEmoji(t *testing.T) {
	m := newTestModel()
	m.focus = focusComposer
	m.composer.SetValue("hello :smi")
	m.refreshEmoji()
	if !m.emojiActive {
		t.Fatal("picker should be active for ':smi'")
	}
	glyph := m.emojiMatches[m.emojiIndex].glyph
	m.acceptEmoji()

	got := m.composer.Value()
	if strings.Contains(got, ":smi") {
		t.Errorf("query not removed: %q", got)
	}
	if !strings.HasPrefix(got, "hello ") || !strings.Contains(got, glyph) {
		t.Errorf("glyph not inserted: %q", got)
	}
	if m.emojiActive {
		t.Error("picker should close after accept")
	}
}

// TestEscCancelsEdit restores the draft and leaves edit mode.
func TestEscCancelsEdit(t *testing.T) {
	m := newTestModelWithHistory()
	m = m.historyOlder() // enter edit, draft saved
	m = m.cancelEdit()
	if m.editing || m.composer.Value() != "draft" {
		t.Errorf("cancel should restore draft: editing=%v val=%q", m.editing, m.composer.Value())
	}
}

// TestSendInEditModeExits ensures saving an edit dispatches a command and leaves
// edit mode without keeping the draft.
func TestSendInEditModeExits(t *testing.T) {
	m := newTestModelWithHistory()
	m = m.historyOlder() // editing "third"
	m.composer.SetValue("edited text")
	updated, cmd := m.sendMessage()
	got := updated.(Model)
	if cmd == nil {
		t.Error("editing send should dispatch a command")
	}
	if got.editing {
		t.Error("should leave edit mode after saving")
	}
}
