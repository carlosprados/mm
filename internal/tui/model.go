package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/list"
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
)

const (
	sidebarWidth  = 32 // total, including border
	defaultLimit  = 30
	pollInterval  = 5 // seconds
	defaultWrapAt = 80
)

// Model is the root Bubble Tea model. It owns the channel sidebar, the message
// viewport and the glamour renderer; all network access goes through mm.
type Model struct {
	ctx context.Context
	mm  *client.MM

	keys      keyMap
	list      list.Model
	viewport  viewport.Model
	renderer  *glamour.TermRenderer
	styleName string // glamour style resolved once at startup (no TTY query in the loop)

	focus  focusArea
	width  int
	height int
	ready  bool

	activeChannelID   string
	activeChannelName string
	limit             int

	status string
	err    error
}

// channelItem adapts a Mattermost channel to bubbles/list.Item.
type channelItem struct {
	id   string
	name string
	typ  string
}

func (c channelItem) Title() string       { return c.name }
func (c channelItem) Description() string  { return c.typ }
func (c channelItem) FilterValue() string { return c.name }

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

	return Model{
		ctx:       ctx,
		mm:        mm,
		keys:      defaultKeys(),
		list:      l,
		viewport:  viewport.New(0, 0),
		renderer:  r,
		styleName: styleName,
		focus:     focusSidebar,
		limit:     defaultLimit,
		status:    "Select a channel and press enter",
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadChannelsCmd(), tickCmd())
}
