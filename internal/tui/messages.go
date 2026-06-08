package tui

import "github.com/charmbracelet/bubbles/list"

// tea.Msg types produced by async commands. The Update loop never blocks on
// the network; every Mattermost call runs inside a tea.Cmd and reports back
// through one of these.

type channelsLoadedMsg struct {
	items []list.Item
}

type postsLoadedMsg struct {
	channelID string
	markdown  string
	count     int
	ownPosts  []ownPost // current user's posts, newest-first (for up-arrow editing)
}

type errMsg struct {
	err error
}

// sentMsg reports that a message was posted to a channel.
type sentMsg struct {
	channelID string
}

// scheduledMsg reports a message was stored for later delivery.
type scheduledMsg struct {
	when string
	err  error
}

// scheduledDeliveredMsg reports the outcome of delivering a due scheduled item.
type scheduledDeliveredMsg struct {
	id        string
	label     string
	channelID string
	err       error
}

// tickMsg drives the polling refresh of the active channel.
type tickMsg struct{}

// scheduleTickMsg drives the scheduled-message delivery loop.
type scheduleTickMsg struct{}
