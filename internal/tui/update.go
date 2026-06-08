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

	"github.com/carlosprados/mm/internal/alias"
	"github.com/carlosprados/mm/internal/schedule"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.resize(msg.Width, msg.Height), nil

	case tea.KeyMsg:
		switch {
		case m.aliasMode:
			return m.handleAliasKey(msg)
		case m.scheduleMode:
			return m.handleScheduleKey(msg)
		default:
			return m.handleKey(msg)
		}

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
		// Don't shift the edit history out from under an in-progress edit.
		if !m.editing {
			m.ownPosts = msg.ownPosts
		}
		m.status = fmt.Sprintf("%s · %d messages", m.activeChannelName, msg.count)
		return m, nil

	case sentMsg:
		if msg.channelID == m.activeChannelID {
			return m, m.loadPostsCmd(msg.channelID)
		}
		return m, nil

	case scheduledMsg:
		if msg.err != nil {
			m.status = "schedule error: " + msg.err.Error()
		} else {
			m.status = "scheduled for " + msg.when
		}
		return m, nil

	case scheduleTickMsg:
		return m, tea.Batch(append(m.deliverDueCmds(), scheduleTickCmd())...)

	case scheduledDeliveredMsg:
		delete(m.deliveringIDs, msg.id)
		if msg.err != nil {
			m.status = "scheduled delivery failed (" + msg.label + "): " + msg.err.Error()
			return m, nil // leave it in the store to retry next tick
		}
		m.status = "delivered scheduled message → " + msg.label
		cmds := []tea.Cmd{removeScheduledCmd(msg.id)}
		if msg.channelID == m.activeChannelID {
			cmds = append(cmds, m.loadPostsCmd(msg.channelID))
		}
		return m, tea.Batch(cmds...)

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
	composerEmpty := strings.TrimSpace(m.composer.Value()) == ""

	switch {
	// --- Emoji picker takes priority over everything below while open ---
	case m.emojiActive && msg.String() == "up":
		if m.emojiIndex > 0 {
			m.emojiIndex--
		}
		return m, nil
	case m.emojiActive && msg.String() == "down":
		if m.emojiIndex < len(m.emojiMatches)-1 {
			m.emojiIndex++
		}
		return m, nil
	case m.emojiActive && (msg.String() == "enter" || msg.String() == "tab"):
		m.acceptEmoji()
		return m.applyLayout(), nil
	case m.emojiActive && msg.String() == "esc":
		m.closeEmoji()
		return m.applyLayout(), nil

	case key.Matches(msg, m.keys.Send):
		return m.sendMessage()

	// Ctrl+T schedules the composed message for later delivery.
	case key.Matches(msg, m.keys.Schedule) && m.focus == focusComposer:
		if strings.TrimSpace(m.composer.Value()) == "" || m.activeChannelID == "" {
			return m, nil
		}
		m.scheduleMode = true
		m.scheduleInput.SetValue("")
		m.scheduleInput.Focus()
		m.status = "deliver when?"
		return m, nil

	// Press 'a' on a selected DM to assign it an alias.
	case msg.String() == "a" && m.focus == focusSidebar && !filtering:
		if it, ok := m.list.SelectedItem().(channelItem); ok && it.username != "" {
			m.aliasMode = true
			m.aliasUser = it.username
			m.aliasInput.SetValue("")
			m.aliasInput.Focus()
			m.status = "alias for @" + it.username
		}
		return m, nil

	// Esc while editing cancels the edit and restores the draft, staying put.
	case msg.String() == "esc" && m.editing:
		return m.cancelEdit(), nil

	// Up walks back through your own messages (shell/Slack style); only when the
	// composer is empty or already editing, otherwise it's a cursor move.
	case msg.String() == "up" && composing && (m.editing || composerEmpty):
		return m.historyOlder(), nil

	// Down walks forward and eventually restores the in-progress draft.
	case msg.String() == "down" && composing && m.editing:
		return m.historyNewer(), nil

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
			// Don't carry a draft or edit state across channels.
			m.exitEdit(false)
			m.composer.Reset()
			cmd := m.setFocus(focusComposer)
			return m, tea.Batch(cmd, m.loadPostsCmd(it.id))
		}
		return m, nil
	}

	return m.delegateToFocused(msg)
}

// sendMessage posts the composer's contents to the active channel, or saves the
// edit if the user is editing one of their previous messages.
func (m Model) sendMessage() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.composer.Value())
	if text == "" || m.activeChannelID == "" {
		return m, nil
	}
	ch := m.activeChannelID

	if m.editing {
		postID := m.ownPosts[m.editIndex].id
		m.composer.Reset()
		m.exitEdit(false)
		m.status = "saving edit…"
		return m, m.editCmd(ch, postID, text)
	}

	m.composer.Reset()
	m.status = "sending…"
	return m, m.sendCmd(ch, text)
}

// historyOlder steps to an older own message, entering edit mode the first time
// and saving the in-progress draft.
func (m Model) historyOlder() Model {
	if len(m.ownPosts) == 0 {
		return m
	}
	switch {
	case !m.editing:
		m.savedDraft = m.composer.Value()
		m.editing = true
		m.editIndex = 0
	case m.editIndex < len(m.ownPosts)-1:
		m.editIndex++
	default:
		return m // already at the oldest
	}
	m.loadEditTarget()
	return m
}

// historyNewer steps toward newer messages, restoring the draft past the newest.
func (m Model) historyNewer() Model {
	if !m.editing {
		return m
	}
	if m.editIndex > 0 {
		m.editIndex--
		m.loadEditTarget()
		return m
	}
	return m.cancelEdit() // stepped past the newest → back to the draft
}

// cancelEdit leaves edit mode and restores the saved draft.
func (m Model) cancelEdit() Model {
	m.exitEdit(true)
	return m
}

func (m *Model) loadEditTarget() {
	p := m.ownPosts[m.editIndex]
	m.composer.SetValue(p.message)
	m.status = fmt.Sprintf("editing your message %d/%d · ctrl+s saves · esc cancels",
		m.editIndex+1, len(m.ownPosts))
}

func (m *Model) exitEdit(restoreDraft bool) {
	if restoreDraft {
		m.composer.SetValue(m.savedDraft)
	}
	m.editing = false
	m.editIndex = 0
	m.savedDraft = ""
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
		m.refreshEmoji()
		m = m.applyLayout() // popup may have opened/closed → resize the message pane
	}
	return m, cmd
}

// refreshEmoji recomputes the picker state from the composer's trailing token.
func (m *Model) refreshEmoji() {
	if m.focus != focusComposer {
		m.closeEmoji()
		return
	}
	q := activeEmojiQuery(m.composer.Value())
	if q == "" {
		m.closeEmoji()
		return
	}
	matches := searchEmoji(q)
	if len(matches) == 0 {
		m.closeEmoji()
		return
	}
	if q != m.emojiQuery {
		m.emojiIndex = 0
	}
	m.emojiQuery = q
	m.emojiMatches = matches
	m.emojiActive = true
}

func (m *Model) closeEmoji() {
	m.emojiActive = false
	m.emojiMatches = nil
	m.emojiQuery = ""
	m.emojiIndex = 0
}

// acceptEmoji replaces the trailing ":query" with the selected glyph.
func (m *Model) acceptEmoji() {
	if !m.emojiActive || len(m.emojiMatches) == 0 {
		return
	}
	e := m.emojiMatches[m.emojiIndex]
	val := m.composer.Value()
	cut := len(val) - len(m.emojiQuery) - 1 // drop ":" + query
	if cut < 0 {
		cut = 0
	}
	m.composer.SetValue(val[:cut] + e.glyph + " ")
	m.closeEmoji()
}

// handleAliasKey captures text for the alias being assigned to a DM's user.
func (m Model) handleAliasKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "enter":
		text := strings.TrimSpace(m.aliasInput.Value())
		if text != "" {
			err := saveAlias(text, m.aliasUser)
			if err != nil {
				m.status = "alias error: " + err.Error()
			} else {
				m.status = "alias " + text + " → @" + m.aliasUser + " saved"
			}
		}
		m.aliasMode = false
		m.aliasInput.Blur()
		return m, m.loadChannelsCmd() // relabel the sidebar
	case "esc":
		m.aliasMode = false
		m.aliasInput.Blur()
		m.status = "alias cancelled"
		return m, nil
	default:
		var cmd tea.Cmd
		m.aliasInput, cmd = m.aliasInput.Update(msg)
		return m, cmd
	}
}

// handleScheduleKey captures the delivery time and schedules the composed message.
func (m Model) handleScheduleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "enter":
		at, err := schedule.ParseTime(m.scheduleInput.Value())
		if err != nil {
			m.status = "time: " + err.Error() // keep the prompt open to fix it
			return m, nil
		}
		text := strings.TrimSpace(m.composer.Value())
		m.scheduleMode = false
		m.scheduleInput.Blur()
		if text == "" || m.activeChannelID == "" {
			m.status = "nothing to schedule"
			return m, nil
		}
		ch := m.activeChannelID
		label := m.activeChannelName
		m.composer.Reset()
		m.status = "scheduling…"
		return m, m.scheduleCmd(ch, label, text, at)
	case "esc":
		m.scheduleMode = false
		m.scheduleInput.Blur()
		m.status = "schedule cancelled"
		return m, nil
	default:
		var cmd tea.Cmd
		m.scheduleInput, cmd = m.scheduleInput.Update(msg)
		return m, cmd
	}
}

func saveAlias(name, username string) error {
	store, err := alias.Load()
	if err != nil {
		return err
	}
	if err := store.Add(name, username); err != nil {
		return err
	}
	return store.Save()
}

func (m Model) resize(w, h int) Model {
	m.width, m.height = w, h

	// Re-create the renderer so Markdown wraps to the new message width. Use the
	// style resolved at startup — never WithAutoStyle here, as it would query the
	// TTY and deadlock against Bubble Tea's input reader.
	if r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(m.styleName),
		glamour.WithWordWrap(m.layout().msgInnerW),
	); err == nil {
		m.renderer = r
	}

	m.ready = true
	return m.applyLayout()
}

// applyLayout pushes the current geometry into the sub-components. It is cheap
// and runs whenever the layout changes (resize, emoji popup open/close).
func (m Model) applyLayout() Model {
	d := m.layout()
	m.list.SetSize(d.sidebarInnerW, d.sidebarInnerH)
	m.viewport.Width = d.msgInnerW
	m.viewport.Height = d.messagesInnerH
	m.composer.SetWidth(d.msgInnerW)
	m.composer.SetHeight(composerLines)
	m.aliasInput.Width = d.msgInnerW
	m.scheduleInput.Width = d.msgInnerW
	if m.activeChannelID == "" {
		m.viewport.SetContent("\n  Pick a channel on the left and press enter to open it.")
	}
	return m
}

// dims holds the computed inner sizes of every pane for the current window.
// Both applyLayout() and View() derive their geometry from here, so they never drift.
type dims struct {
	sidebarInnerW  int
	sidebarInnerH  int
	msgInnerW      int
	messagesInnerH int
	popupRows      int
}

func (m Model) layout() dims {
	const footerH = 1
	contentH := m.height - footerH

	msgInnerW := m.width - sidebarWidth - 2
	if msgInnerW < 10 {
		msgInnerW = 10
	}

	popupRows := 0
	if m.emojiActive {
		if popupRows = len(m.emojiMatches); popupRows > maxEmojiResults {
			popupRows = maxEmojiResults
		}
	}
	popupTotalH := 0
	if popupRows > 0 {
		popupTotalH = popupRows + 2 // popup border
	}

	composerTotalH := composerLines + 2 // textarea rows + border
	messagesInnerH := contentH - composerTotalH - popupTotalH - 2
	if messagesInnerH < 1 {
		messagesInnerH = 1
	}

	return dims{
		sidebarInnerW:  sidebarWidth - 2,
		sidebarInnerH:  contentH - 2,
		msgInnerW:      msgInnerW,
		messagesInnerH: messagesInnerH,
		popupRows:      popupRows,
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

func (m Model) editCmd(channelID, postID, text string) tea.Cmd {
	return func() tea.Msg {
		if err := m.mm.EditPost(m.ctx, postID, text); err != nil {
			return errMsg{err}
		}
		return sentMsg{channelID: channelID}
	}
}

// scheduleCmd stores a message for later delivery. Delivery happens in the
// scheduleTick loop while the TUI runs.
func (m Model) scheduleCmd(channelID, label, text string, at time.Time) tea.Cmd {
	return func() tea.Msg {
		store, err := schedule.Load()
		if err != nil {
			return scheduledMsg{err: err}
		}
		if _, err := store.Add(channelID, label, text, at); err != nil {
			return scheduledMsg{err: err}
		}
		if err := store.Save(); err != nil {
			return scheduledMsg{err: err}
		}
		return scheduledMsg{when: at.Format("2006-01-02 15:04")}
	}
}

func scheduleTickCmd() tea.Cmd {
	return tea.Tick(scheduleInterval*time.Second, func(time.Time) tea.Msg {
		return scheduleTickMsg{}
	})
}

// deliverDueCmds loads the store and returns a delivery command for each due
// item not already in flight, marking them so they aren't dispatched twice.
func (m *Model) deliverDueCmds() []tea.Cmd {
	store, err := schedule.Load()
	if err != nil {
		return nil
	}
	var cmds []tea.Cmd
	for _, it := range store.Due(time.Now()) {
		if m.deliveringIDs[it.ID] {
			continue
		}
		m.deliveringIDs[it.ID] = true
		cmds = append(cmds, m.deliverScheduledCmd(it))
	}
	return cmds
}

func (m Model) deliverScheduledCmd(it schedule.Item) tea.Cmd {
	return func() tea.Msg {
		err := error(nil)
		if _, e := m.mm.SendToChannelID(m.ctx, it.ChannelID, it.Message); e != nil {
			err = e
		}
		return scheduledDeliveredMsg{id: it.ID, label: it.Label, channelID: it.ChannelID, err: err}
	}
}

func removeScheduledCmd(id string) tea.Cmd {
	return func() tea.Msg {
		store, err := schedule.Load()
		if err != nil {
			return nil
		}
		_ = store.Remove(id)
		_ = store.Save()
		return nil
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

		// Alias store lets DMs show "luis" instead of "@luisdavid.francisco".
		aliases, _ := alias.Load()

		items := make([]channelItem, 0, len(chans))
		for _, ch := range chans {
			typ := channelTypeLabel(ch.Type)
			var name, desc, username string
			switch ch.Type {
			case model.ChannelTypeDirect:
				username = strings.TrimPrefix(names[ch.GetOtherUserIdForDM(m.mm.UserID)], "@")
				name, desc = dmLabel(username, aliases)
			default:
				name = ch.DisplayName
				if name == "" {
					name = ch.Name
				}
				desc = "# " + typ
			}
			items = append(items, channelItem{id: ch.Id, name: name, desc: desc, typ: typ, username: username})
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

		// posts.Order is newest-first, so this collects own posts newest-first too.
		var own []ownPost
		for _, id := range posts.Order {
			if p := posts.Posts[id]; p.UserId == m.mm.UserID {
				own = append(own, ownPost{id: p.Id, message: p.Message})
			}
		}

		return postsLoadedMsg{channelID: channelID, markdown: b.String(), count: len(posts.Order), ownPosts: own}
	}
}

// dmLabel returns the sidebar title and subtitle for a DM. When an alias points
// at the user, the alias is the title and the @handle the subtitle; otherwise
// the @handle is the title.
func dmLabel(bareUsername string, store *alias.Store) (title, desc string) {
	if store != nil {
		if a := store.AliasesFor(bareUsername); len(a) > 0 {
			return a[0], "@" + bareUsername
		}
	}
	return "@" + bareUsername, "dm"
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
