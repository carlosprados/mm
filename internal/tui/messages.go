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
}

type errMsg struct {
	err error
}

// sentMsg reports that a message was posted to a channel.
type sentMsg struct {
	channelID string
}

// tickMsg drives the polling refresh of the active channel.
type tickMsg struct{}
