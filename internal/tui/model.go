package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/glamour"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"

	"github.com/carlosprados/mm/internal/client"
	"github.com/carlosprados/mm/internal/schedule"
)

type focusArea int

const (
	focusSidebar focusArea = iota
	focusMessages
	focusComposer
	focusCount = 3
)

const (
	sidebarWidth      = 32 // total, including border
	composerLines     = 3  // textarea visible rows
	defaultLimit      = 30
	maxLoadedPosts    = 400 // sliding-window cap on messages held in memory per channel
	scheduleInterval  = 20  // seconds — scheduled-message delivery + safety refresh
	reconnectInterval = 3  // seconds — WebSocket reconnect backoff
	defaultWrapAt     = 80
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

	// helpMode shows the keyboard-shortcut popup (toggled with '?').
	helpMode bool

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

	// WebSocket real-time updates (replaces message polling).
	ws          *client.WSConn
	wsConnected bool

	// scheduleView is the in-TUI list of pending scheduled messages (toggled
	// with 's' from the sidebar).
	scheduleViewMode   bool
	scheduleView       []schedule.Item
	scheduleViewCursor int

	// posts holds the active channel's loaded messages (chronological). The set
	// grows upward via loadOlder and downward via live newer events.
	posts        []postLine
	loadingOlder bool
	copyMode     bool
	copyCursor   int

	// image picker: 'i' lists image attachments in the channel; selecting one
	// downloads it and renders it inline via chafa (suspending the TUI).
	imagePickMode    bool
	imageAttachments []imageAttachment
	imagePickCursor  int

	// react flow: '+' picks a message (phase 0) then an emoji (phase 1).
	reactMode        bool
	reactPhase       int // 0 = pick message, 1 = pick emoji
	reactCursor      int // message index in m.posts
	reactTarget      string
	reactInput       textinput.Model
	reactMatches     []emojiEntry
	reactEmojiCursor int

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

// postLine is a displayed message in the active channel, kept so it can be
// copied as Markdown source and so its attachments can be located.
type postLine struct {
	postID    string
	time      string
	author    string
	message   string
	fileIDs   []string
	reactions string // pre-formatted "👍 2  🎉 1" (empty if none)
}

// imageAttachment is an image file attached to a message in the active channel.
type imageAttachment struct {
	label  string // "HH:MM @user — name.png"
	fileID string
	name   string
}

// channelItem adapts a Mattermost channel to bubbles/list.Item.
type channelItem struct {
	id         string
	name       string
	desc       string // shown under the title (handle for DMs, type for channels)
	typ        string // "public"/"private"/"group"/"dm" — drives sorting
	username   string // bare username for DMs (empty for channels), used to set aliases
	unread     bool   // there are messages the user hasn't read
	mentions   int    // unread @-mentions
	lastPostAt int64  // for ordering unread items by recency
	favorite   bool   // user-favorited channel/DM
}

// Title renders a favorite star and unread bullet (fixed 3-char prefix for
// alignment), keeping `name` clean for sorting and filtering.
func (c channelItem) Title() string {
	star, bullet := " ", " "
	if c.favorite {
		star = "★"
	}
	if c.unread {
		bullet = "●"
	}
	t := star + bullet + " " + c.name
	if c.unread && c.mentions > 0 {
		t += fmt.Sprintf(" (%d)", c.mentions)
	}
	return t
}

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

	ri := textinput.New()
	ri.Placeholder = "emoji"

	return Model{
		ctx:           ctx,
		mm:            mm,
		keys:          defaultKeys(),
		list:          l,
		viewport:      viewport.New(0, 0),
		composer:      ta,
		aliasInput:    ai,
		scheduleInput: si,
		reactInput:    ri,
		renderer:      r,
		styleName:     styleName,
		focus:         focusSidebar,
		limit:         defaultLimit,
		deliveringIDs: map[string]bool{},
		status:        "Select a channel · enter opens · ? for help",
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
	return tea.Batch(m.loadChannelsCmd(), scheduleTickCmd(), m.connectWSCmd())
}
