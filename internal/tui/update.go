package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/mattermost/mattermost/server/public/model"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.resize(msg.Width, msg.Height), nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case channelsLoadedMsg:
		m.list.SetItems(msg.items)
		m.status = fmt.Sprintf("%d channels", len(msg.items))
		return m, nil

	case postsLoadedMsg:
		// Ignore stale loads from a channel we've since navigated away from.
		if msg.channelID != m.activeChannelID {
			return m, nil
		}
		rendered, err := m.renderer.Render(msg.markdown)
		if err != nil {
			rendered = msg.markdown
		}
		m.viewport.SetContent(rendered)
		m.viewport.GotoBottom()
		m.status = fmt.Sprintf("%s · %d messages", m.activeChannelName, msg.count)
		return m, nil

	case sentMsg:
		if msg.channelID == m.activeChannelID {
			return m, m.loadPostsCmd(msg.channelID)
		}
		return m, nil

	case tickMsg:
		cmds := []tea.Cmd{tickCmd()}
		if m.activeChannelID != "" {
			cmds = append(cmds, m.loadPostsCmd(m.activeChannelID))
		}
		return m, tea.Batch(cmds...)

	case errMsg:
		m.err = msg.err
		m.status = "error: " + msg.err.Error()
		return m, nil
	}

	return m.delegateToFocused(msg)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Ctrl+C always quits, even while filtering or composing — never trap the user.
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	// While the sidebar filter is open or the composer is focused, single-letter
	// shortcuts (q, r) are text input, not commands.
	filtering := m.list.FilterState() == list.Filtering
	composing := m.focus == focusComposer

	switch {
	case key.Matches(msg, m.keys.Send):
		return m.sendMessage()

	case key.Matches(msg, m.keys.Tab):
		cmd := m.setFocus((m.focus + 1) % focusCount)
		return m, cmd

	case key.Matches(msg, m.keys.Back) && !filtering:
		cmd := m.setFocus(focusSidebar)
		return m, cmd

	case key.Matches(msg, m.keys.Quit) && !composing && !filtering:
		return m, tea.Quit

	case key.Matches(msg, m.keys.Refresh) && !composing && !filtering:
		if m.activeChannelID != "" {
			return m, m.loadPostsCmd(m.activeChannelID)
		}
		return m, m.loadChannelsCmd()

	case key.Matches(msg, m.keys.Enter) && m.focus == focusSidebar && !filtering:
		if it, ok := m.list.SelectedItem().(channelItem); ok {
			m.activeChannelID = it.id
			m.activeChannelName = it.name
			m.status = "loading " + it.name + "…"
			cmd := m.setFocus(focusComposer)
			return m, tea.Batch(cmd, m.loadPostsCmd(it.id))
		}
		return m, nil
	}

	return m.delegateToFocused(msg)
}

// sendMessage posts the composer's contents to the active channel.
func (m Model) sendMessage() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.composer.Value())
	if text == "" || m.activeChannelID == "" {
		return m, nil
	}
	ch := m.activeChannelID
	m.composer.Reset()
	m.status = "sending…"
	return m, m.sendCmd(ch, text)
}

// delegateToFocused forwards a message to whichever component holds focus.
func (m Model) delegateToFocused(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.focus {
	case focusSidebar:
		m.list, cmd = m.list.Update(msg)
	case focusMessages:
		m.viewport, cmd = m.viewport.Update(msg)
	case focusComposer:
		m.composer, cmd = m.composer.Update(msg)
	}
	return m, cmd
}

func (m Model) resize(w, h int) Model {
	m.width, m.height = w, h

	d := m.layout()

	m.list.SetSize(d.sidebarInnerW, d.sidebarInnerH)
	m.viewport.Width = d.msgInnerW
	m.viewport.Height = d.messagesInnerH
	m.composer.SetWidth(d.msgInnerW)
	m.composer.SetHeight(composerLines)
	if m.activeChannelID == "" {
		m.viewport.SetContent("\n  Pick a channel on the left and press enter to open it.")
	}

	// Re-create the renderer so Markdown wraps to the new message width. Use the
	// style resolved at startup — never WithAutoStyle here, as it would query the
	// TTY and deadlock against Bubble Tea's input reader.
	if r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(m.styleName),
		glamour.WithWordWrap(d.msgInnerW),
	); err == nil {
		m.renderer = r
	}

	m.ready = true
	return m
}

// dims holds the computed inner sizes of every pane for the current window.
// Both resize() and View() derive their geometry from here, so they never drift.
type dims struct {
	sidebarInnerW  int
	sidebarInnerH  int
	msgInnerW      int
	messagesInnerH int
}

func (m Model) layout() dims {
	const footerH = 1
	contentH := m.height - footerH

	msgInnerW := m.width - sidebarWidth - 2
	if msgInnerW < 10 {
		msgInnerW = 10
	}

	composerTotalH := composerLines + 2 // textarea rows + border
	messagesInnerH := contentH - composerTotalH - 2
	if messagesInnerH < 1 {
		messagesInnerH = 1
	}

	return dims{
		sidebarInnerW:  sidebarWidth - 2,
		sidebarInnerH:  contentH - 2,
		msgInnerW:      msgInnerW,
		messagesInnerH: messagesInnerH,
	}
}

// --- commands (async network calls) ---

func tickCmd() tea.Cmd {
	return tea.Tick(pollInterval*time.Second, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m Model) sendCmd(channelID, text string) tea.Cmd {
	return func() tea.Msg {
		if _, err := m.mm.SendToChannelID(m.ctx, channelID, text); err != nil {
			return errMsg{err}
		}
		return sentMsg{channelID: channelID}
	}
}

func (m Model) loadChannelsCmd() tea.Cmd {
	return func() tea.Msg {
		chans, _, err := m.mm.Client.GetChannelsForTeamForUser(m.ctx, m.mm.TeamID, m.mm.UserID, false, "")
		if err != nil {
			return errMsg{fmt.Errorf("could not fetch channels: %w", err)}
		}

		// DM channels have a machine name (myID__otherID); resolve the other
		// participant so we can label them by @username instead.
		var dmOtherIDs []string
		for _, ch := range chans {
			if ch.Type == model.ChannelTypeDirect {
				dmOtherIDs = append(dmOtherIDs, ch.GetOtherUserIdForDM(m.mm.UserID))
			}
		}
		names := map[string]string{}
		if len(dmOtherIDs) > 0 {
			if names, err = m.mm.ResolveUsernames(m.ctx, dmOtherIDs); err != nil {
				return errMsg{err}
			}
		}

		items := make([]channelItem, 0, len(chans))
		for _, ch := range chans {
			label := ch.DisplayName
			switch ch.Type {
			case model.ChannelTypeDirect:
				label = names[ch.GetOtherUserIdForDM(m.mm.UserID)]
			default:
				if label == "" {
					label = ch.Name
				}
			}
			items = append(items, channelItem{id: ch.Id, name: label, typ: channelTypeLabel(ch.Type)})
		}

		// Named channels first, DMs after; alphabetical within each group.
		sort.SliceStable(items, func(i, j int) bool {
			di, dj := items[i].typ == "dm", items[j].typ == "dm"
			if di != dj {
				return !di
			}
			return strings.ToLower(items[i].name) < strings.ToLower(items[j].name)
		})

		listItems := make([]list.Item, len(items))
		for i, it := range items {
			listItems[i] = it
		}
		return channelsLoadedMsg{items: listItems}
	}
}

func (m Model) loadPostsCmd(channelID string) tea.Cmd {
	return func() tea.Msg {
		posts, _, err := m.mm.Client.GetPostsForChannel(m.ctx, channelID, 0, m.limit, "", false, false)
		if err != nil {
			return errMsg{fmt.Errorf("could not fetch posts: %w", err)}
		}

		ids := make([]string, 0, len(posts.Order))
		for _, id := range posts.Order {
			ids = append(ids, posts.Posts[id].UserId)
		}
		usernames, err := m.mm.ResolveUsernames(m.ctx, ids)
		if err != nil {
			return errMsg{err}
		}

		var b strings.Builder
		for i := len(posts.Order) - 1; i >= 0; i-- {
			p := posts.Posts[posts.Order[i]]
			ts := time.UnixMilli(p.CreateAt).Format("15:04")
			fmt.Fprintf(&b, "**%s · %s**\n\n%s\n\n---\n\n", ts, usernames[p.UserId], p.Message)
		}

		return postsLoadedMsg{channelID: channelID, markdown: b.String(), count: len(posts.Order)}
	}
}

func channelTypeLabel(t model.ChannelType) string {
	switch t {
	case model.ChannelTypePrivate:
		return "private"
	case model.ChannelTypeDirect:
		return "dm"
	case model.ChannelTypeGroup:
		return "group"
	default:
		return "public"
	}
}
