package tui

import "github.com/charmbracelet/bubbles/list"

// tea.Msg types produced by async commands. The Update loop never blocks on
// the network; every Mattermost call runs inside a tea.Cmd and reports back
// through one of these.

type channelsLoadedMsg struct {
	items []list.Item
}

// channelsReloadMsg requests a sidebar refresh (e.g. after marking a channel read).
type channelsReloadMsg struct{}

type postsLoadedMsg struct {
	channelID string
	markdown  string
	count     int
	ownPosts  []ownPost  // current user's posts, newest-first (for up-arrow editing)
	posts     []postLine // all displayed posts, chronological (for the copy picker)
}

// copiedMsg reports the result of copying a message to the clipboard.
type copiedMsg struct {
	err error
}

// attachmentsLoadedMsg carries the channel's image attachments for the picker.
type attachmentsLoadedMsg struct {
	images []imageAttachment
	err    error
}

// imageReadyMsg signals a downloaded image is on disk and ready to render.
type imageReadyMsg struct {
	path string
	err  error
}

// imageClosedMsg fires after the external image viewer (chafa) returns.
type imageClosedMsg struct {
	path string
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
