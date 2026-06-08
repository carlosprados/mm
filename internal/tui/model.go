package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/glamour"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"

	"github.com/carlosprados/mm/internal/client"
)

type focusArea int

const (
	focusSidebar focusArea = iota
	focusMessages
	focusComposer
	focusCount = 3
)

const (
	sidebarWidth     = 32 // total, including border
	composerLines    = 3  // textarea visible rows
	defaultLimit     = 30
	pollInterval     = 5  // seconds — active channel refresh
	scheduleInterval = 20 // seconds — scheduled-message delivery check
	defaultWrapAt    = 80
)

// Model is the root Bubble Tea model. It owns the channel sidebar, the message
// viewport and the glamour renderer; all network access goes through mm.
type Model struct {
	ctx context.Context
	mm  *client.MM

	keys       keyMap
	list       list.Model
	viewport   viewport.Model
	composer   textarea.Model
	aliasInput textinput.Model
	renderer   *glamour.TermRenderer
	styleName  string // glamour style resolved once at startup (no TTY query in the loop)

	// aliasMode captures an alias for the selected DM's user (aliasUser).
	aliasMode bool
	aliasUser string

	// scheduleMode captures a delivery time for the composed message.
	scheduleMode  bool
	scheduleInput textinput.Model

	// Emoji picker: opens on a trailing ":query" in the composer.
	emojiActive  bool
	emojiMatches []emojiEntry
	emojiIndex   int
	emojiQuery   string

	// deliveringIDs tracks scheduled items currently being sent, so the delivery
	// loop doesn't dispatch the same item twice.
	deliveringIDs map[string]bool

	focus  focusArea
	width  int
	height int
	ready  bool

	activeChannelID   string
	activeChannelName string
	limit             int

	// Up-arrow editing (shell/Slack style): ownPosts is newest-first, editIndex
	// walks back through it, savedDraft preserves the in-progress message.
	ownPosts   []ownPost
	editing    bool
	editIndex  int
	savedDraft string

	status string
	err    error
}

// ownPost is one of the current user's editable posts in the active channel.
type ownPost struct {
	id      string
	message string
}

// channelItem adapts a Mattermost channel to bubbles/list.Item.
type channelItem struct {
	id       string
	name     string
	desc     string // shown under the title (handle for DMs, type for channels)
	typ      string // "public"/"private"/"group"/"dm" — drives sorting
	username string // bare username for DMs (empty for channels), used to set aliases
}

func (c channelItem) Title() string       { return c.name }
func (c channelItem) Description() string  { return c.desc }
func (c channelItem) FilterValue() string { return c.name + " " + c.desc }

// New builds the initial model bound to an authenticated client.
func New(ctx context.Context, mm *client.MM) Model {
	delegate := list.NewDefaultDelegate()
	l := list.New(nil, delegate, 0, 0)
	l.Title = "Channels"
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)

	// Resolve the Markdown style ONCE here, before tea.Program owns the TTY.
	// glamour's "auto" style queries the terminal background over the TTY; doing
	// that inside the Update loop deadlocks against Bubble Tea's input reader.
	styleName := "dark"
	if !termenv.HasDarkBackground() {
		styleName = "light"
	}

	r, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle(styleName),
		glamour.WithWordWrap(defaultWrapAt),
	)

	ta := textarea.New()
	ta.Placeholder = "Write a message… (ctrl+s to send, : for emoji)"
	ta.ShowLineNumbers = false
	ta.SetHeight(composerLines)
	ta.CharLimit = 0 // unlimited

	ai := textinput.New()
	ai.Placeholder = "alias"

	si := textinput.New()
	si.Placeholder = "+2h  ·  2006-01-02 15:04"

	return Model{
		ctx:           ctx,
		mm:            mm,
		keys:          defaultKeys(),
		list:          l,
		viewport:      viewport.New(0, 0),
		composer:      ta,
		aliasInput:    ai,
		scheduleInput: si,
		renderer:      r,
		styleName:     styleName,
		focus:         focusSidebar,
		limit:         defaultLimit,
		deliveringIDs: map[string]bool{},
		status:        "Select a channel · enter opens · a aliases a DM",
	}
}

// setFocus moves focus and toggles the composer cursor accordingly. It returns
// the blink command when the composer gains focus.
func (m *Model) setFocus(area focusArea) tea.Cmd {
	m.focus = area
	if area == focusComposer {
		return m.composer.Focus()
	}
	m.composer.Blur()
	return nil
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadChannelsCmd(), tickCmd(), scheduleTickCmd())
}
