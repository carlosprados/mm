package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/atotto/clipboard"
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
		// Clear the screen on resize: a resized frame can leave residue from the
		// previous (differently sized) render that a partial repaint won't cover.
		return m.resize(msg.Width, msg.Height), tea.ClearScreen

	case tea.KeyMsg:
		switch {
		case m.helpMode:
			return m.handleHelpKey(msg)
		case m.reactMode:
			return m.handleReactKey(msg)
		case m.imagePickMode:
			return m.handleImageKey(msg)
		case m.copyMode:
			return m.handleCopyKey(msg)
		case m.scheduleViewMode:
			return m.handleScheduleViewKey(msg)
		case m.aliasMode:
			return m.handleAliasKey(msg)
		case m.scheduleMode:
			return m.handleScheduleKey(msg)
		default:
			return m.handleKey(msg)
		}

	case channelsLoadedMsg:
		// Preserve the selected channel across reloads (the sort may reorder).
		var selID string
		if it, ok := m.list.SelectedItem().(channelItem); ok {
			selID = it.id
		}
		m.list.SetItems(msg.items)
		if selID != "" {
			for i, li := range msg.items {
				if ci, ok := li.(channelItem); ok && ci.id == selID {
					m.list.Select(i)
					break
				}
			}
		}
		return m, nil

	case channelsReloadMsg:
		return m, m.loadChannelsCmd()

	case postsLoadedMsg:
		// Ignore stale loads from a channel we've since navigated away from.
		if msg.channelID != m.activeChannelID {
			return m, nil
		}
		// Preserve the reader's scroll position: only jump to the bottom if they
		// were already there.
		atBottom := m.viewport.AtBottom()
		m.posts = msg.posts
		m.renderPosts()
		if atBottom {
			m.viewport.GotoBottom()
		}
		if !m.editing {
			m.ownPosts = m.ownPostsFrom(m.posts)
		}
		m.status = fmt.Sprintf("%s · %d messages", m.activeChannelName, len(m.posts))
		return m, nil

	case olderLoadedMsg:
		m.loadingOlder = false
		if msg.channelID != m.activeChannelID {
			return m, nil
		}
		if len(msg.posts) == 0 {
			m.status = "no older messages"
			return m, nil
		}
		// Prepend and keep the current message under the viewport by pushing the
		// y-offset down by the number of lines we added at the top.
		before := m.viewport.TotalLineCount()
		m.posts = append(msg.posts, m.posts...)
		m.renderPosts()
		added := m.viewport.TotalLineCount() - before
		m.viewport.SetYOffset(m.viewport.YOffset + added)
		// Sliding window: drop the newest beyond the cap (below the fold, so the
		// scroll position above is unaffected).
		if len(m.posts) > maxLoadedPosts {
			m.posts = keepOldest(m.posts, maxLoadedPosts)
			m.renderPosts()
		}
		if !m.editing {
			m.ownPosts = m.ownPostsFrom(m.posts)
		}
		m.status = fmt.Sprintf("%s · %d messages", m.activeChannelName, len(m.posts))
		return m, nil

	case newerLoadedMsg:
		if msg.channelID != m.activeChannelID || len(msg.posts) == 0 {
			return m, nil
		}
		atBottom := m.viewport.AtBottom()
		m.posts = append(m.posts, msg.posts...)
		// Sliding window: when at the bottom, drop the oldest beyond the cap. We
		// only trim while at the bottom so scrolling up to read history isn't
		// yanked out from under the reader.
		if atBottom && len(m.posts) > maxLoadedPosts {
			m.posts = keepNewest(m.posts, maxLoadedPosts)
		}
		m.renderPosts()
		if atBottom {
			m.viewport.GotoBottom()
		}
		if !m.editing {
			m.ownPosts = m.ownPostsFrom(m.posts)
		}
		m.status = fmt.Sprintf("%s · %d messages", m.activeChannelName, len(m.posts))
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

	case copiedMsg:
		if msg.err != nil {
			m.status = "copy failed: " + msg.err.Error()
		} else {
			m.status = "copied to clipboard"
		}
		return m, nil

	case reactedMsg:
		if msg.err != nil {
			m.status = "reaction failed: " + msg.err.Error()
			return m, nil
		}
		m.status = "reaction added"
		if msg.channelID == m.activeChannelID {
			return m, m.loadPostsCmd(msg.channelID) // refresh to show it
		}
		return m, nil

	case attachmentsLoadedMsg:
		if msg.err != nil {
			m.status = "attachments: " + msg.err.Error()
			return m, nil
		}
		if len(msg.images) == 0 {
			m.status = "no image attachments in this channel"
			return m, nil
		}
		m.imageAttachments = msg.images
		m.imagePickCursor = len(msg.images) - 1 // most recent
		m.imagePickMode = true
		m.status = "view an image"
		return m, nil

	case imageReadyMsg:
		if msg.err != nil {
			m.status = "image: " + msg.err.Error()
			return m, nil
		}
		return m, viewImageCmd(msg.path)

	case imageClosedMsg:
		if msg.path != "" {
			_ = os.Remove(msg.path)
		}
		m.status = "closed image"
		return m, nil

	case scheduleTickMsg:
		cmds := append(m.deliverDueCmds(), scheduleTickCmd())
		if m.idleForReload() {
			cmds = append(cmds, m.loadChannelsCmd()) // refresh unread state
		}
		// Safety net: if the WebSocket is down, fall back to refetching the
		// active channel so messages don't go stale during a reconnect.
		if !m.wsConnected && m.activeChannelID != "" {
			cmds = append(cmds, m.loadPostsCmd(m.activeChannelID))
		}
		return m, tea.Batch(cmds...)

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

	case wsConnectedMsg:
		m.ws = msg.ws
		m.wsConnected = true
		m.status = "● live"
		return m, waitWSEventCmd(m.ws.Events)

	case wsEventMsg:
		cmds := []tea.Cmd{waitWSEventCmd(m.ws.Events)} // keep listening
		switch msg.ev.EventType() {
		case model.WebsocketEventPosted, model.WebsocketEventPostEdited, model.WebsocketEventPostDeleted:
			var chID string
			if b := msg.ev.GetBroadcast(); b != nil {
				chID = b.ChannelId
			}
			if chID != "" && chID == m.activeChannelID {
				if msg.ev.EventType() == model.WebsocketEventPosted {
					cmds = append(cmds, m.loadNewerCmd(chID)) // append, keep history
				} else {
					cmds = append(cmds, m.loadPostsCmd(chID)) // edit/delete: refresh window
				}
			}
			if m.idleForReload() {
				cmds = append(cmds, m.loadChannelsCmd()) // bubble unread live
			}
		}
		return m, tea.Batch(cmds...)

	case wsClosedMsg:
		m.wsConnected = false
		m.ws = nil
		m.status = "reconnecting…"
		return m, reconnectCmd()

	case wsReconnectMsg:
		return m, m.connectWSCmd()

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

	// At the top of the message pane, scrolling up loads older history.
	case m.focus == focusMessages && !m.loadingOlder && m.activeChannelID != "" &&
		m.viewport.AtTop() && (msg.String() == "up" || msg.String() == "k" || msg.String() == "pgup"):
		m.loadingOlder = true
		m.status = "loading older…"
		return m, m.loadOlderCmd(m.activeChannelID)

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

	// React to a message: '+' from the messages pane.
	case msg.String() == "+" && m.focus == focusMessages && len(m.posts) > 0:
		m.reactMode = true
		m.reactPhase = 0
		m.reactCursor = len(m.posts) - 1 // default to most recent
		m.status = "react: pick a message"
		return m, nil

	// View image attachments from the messages pane.
	case msg.String() == "i" && m.focus == focusMessages:
		if !m.anyAttachments() {
			m.status = "no attachments in this channel"
			return m, nil
		}
		m.status = "loading attachments…"
		return m, m.loadAttachmentsCmd()

	// Open the copy picker from the messages pane.
	case msg.String() == "y" && m.focus == focusMessages && len(m.posts) > 0:
		m.copyMode = true
		m.copyCursor = len(m.posts) - 1 // default to the most recent message
		m.status = "copy a message"
		return m, nil

	// Toggle the keyboard-shortcut help popup. Not while composing or filtering,
	// where '?' is literal text.
	case msg.String() == "?" && !composing && !filtering:
		m.helpMode = true
		m.status = "keyboard shortcuts"
		return m, nil

	// Open the scheduled-messages viewer from the sidebar.
	case msg.String() == "s" && m.focus == focusSidebar && !filtering:
		store, _ := schedule.Load()
		m.scheduleView = store.Sorted()
		m.scheduleViewCursor = 0
		m.scheduleViewMode = true
		m.status = "scheduled messages"
		return m, nil

	case key.Matches(msg, m.keys.Enter) && m.focus == focusSidebar && !filtering:
		if it, ok := m.list.SelectedItem().(channelItem); ok {
			m.activeChannelID = it.id
			m.activeChannelName = it.name
			m.status = "loading " + it.name + "…"
			// Don't carry a draft or edit state across channels.
			m.exitEdit(false)
			m.composer.Reset()
			cmd := m.setFocus(focusComposer)
			// Mark read (clears unread everywhere) and refresh the sidebar.
			return m, tea.Batch(cmd, m.loadPostsCmd(it.id), m.markReadCmd(it.id))
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

// handleHelpKey closes the shortcut popup on any key (ctrl+c still quits).
func (m Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	m.helpMode = false
	m.status = ""
	return m, nil
}

// idleForReload reports whether a background sidebar reload is safe (no input
// mode or filter that a reordering would disrupt).
func (m Model) idleForReload() bool {
	return m.list.FilterState() != list.Filtering &&
		!m.aliasMode && !m.scheduleMode && !m.scheduleViewMode
}

// handleCopyKey drives the message copy picker; enter/y copies the selected
// message's Markdown source to the system clipboard.
func (m Model) handleCopyKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q":
		m.copyMode = false
		return m, nil
	case "down", "j":
		if m.copyCursor < len(m.posts)-1 {
			m.copyCursor++
		}
		return m, nil
	case "up", "k":
		if m.copyCursor > 0 {
			m.copyCursor--
		}
		return m, nil
	case "enter", "y":
		if m.copyCursor < 0 || m.copyCursor >= len(m.posts) {
			return m, nil
		}
		text := m.posts[m.copyCursor].message
		m.copyMode = false
		return m, copyCmd(text)
	}
	return m, nil
}

func copyCmd(text string) tea.Cmd {
	return func() tea.Msg {
		if err := clipboard.WriteAll(text); err != nil {
			return copiedMsg{err: err}
		}
		return copiedMsg{}
	}
}

// handleReactKey drives the two-phase react flow: pick a message, then an emoji.
func (m Model) handleReactKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	if m.reactPhase == 0 { // pick a message
		switch msg.String() {
		case "esc", "q":
			m.reactMode = false
		case "down", "j":
			if m.reactCursor < len(m.posts)-1 {
				m.reactCursor++
			}
		case "up", "k":
			if m.reactCursor > 0 {
				m.reactCursor--
			}
		case "enter":
			if m.reactCursor >= 0 && m.reactCursor < len(m.posts) {
				m.reactTarget = m.posts[m.reactCursor].postID
				m.reactPhase = 1
				m.reactInput.SetValue("")
				m.reactInput.Focus()
				m.reactMatches = nil
				m.reactEmojiCursor = 0
				m.status = "react: search an emoji"
			}
		}
		return m, nil
	}

	// phase 1: pick an emoji (textinput captures letters; navigate with arrows)
	switch msg.String() {
	case "esc":
		m.reactPhase = 0
		m.reactInput.Blur()
		m.status = "react: pick a message"
		return m, nil
	case "up":
		if m.reactEmojiCursor > 0 {
			m.reactEmojiCursor--
		}
		return m, nil
	case "down":
		if m.reactEmojiCursor < len(m.reactMatches)-1 {
			m.reactEmojiCursor++
		}
		return m, nil
	case "enter":
		if m.reactEmojiCursor < 0 || m.reactEmojiCursor >= len(m.reactMatches) {
			return m, nil
		}
		name := m.reactMatches[m.reactEmojiCursor].short
		target := m.reactTarget
		m.reactMode = false
		m.reactInput.Blur()
		m.status = "reacting…"
		return m, m.reactCmd(target, name)
	default:
		var cmd tea.Cmd
		m.reactInput, cmd = m.reactInput.Update(msg)
		if q := strings.ToLower(strings.TrimSpace(m.reactInput.Value())); len(q) >= 2 {
			m.reactMatches = searchEmoji(q)
		} else {
			m.reactMatches = nil
		}
		if m.reactEmojiCursor >= len(m.reactMatches) {
			m.reactEmojiCursor = 0
		}
		return m, cmd
	}
}

func (m Model) reactCmd(postID, emojiName string) tea.Cmd {
	return func() tea.Msg {
		if err := m.mm.React(m.ctx, postID, emojiName); err != nil {
			return reactedMsg{err: err}
		}
		return reactedMsg{channelID: m.activeChannelID}
	}
}

// handleImageKey drives the image-attachment picker; enter renders the selected
// image inline (via chafa, suspending the TUI).
func (m Model) handleImageKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q":
		m.imagePickMode = false
		return m, nil
	case "down", "j":
		if m.imagePickCursor < len(m.imageAttachments)-1 {
			m.imagePickCursor++
		}
		return m, nil
	case "up", "k":
		if m.imagePickCursor > 0 {
			m.imagePickCursor--
		}
		return m, nil
	case "enter":
		if m.imagePickCursor < 0 || m.imagePickCursor >= len(m.imageAttachments) {
			return m, nil
		}
		img := m.imageAttachments[m.imagePickCursor]
		m.imagePickMode = false
		m.status = "downloading " + img.name + "…"
		return m, m.downloadImageCmd(img)
	}
	return m, nil
}

func (m Model) anyAttachments() bool {
	for _, p := range m.posts {
		if len(p.fileIDs) > 0 {
			return true
		}
	}
	return false
}

// loadAttachmentsCmd fetches file infos for posts with files and keeps images.
func (m Model) loadAttachmentsCmd() tea.Cmd {
	return func() tea.Msg {
		var images []imageAttachment
		for _, p := range m.posts {
			if len(p.fileIDs) == 0 {
				continue
			}
			infos, _, err := m.mm.Client.GetFileInfosForPost(m.ctx, p.postID, "")
			if err != nil {
				return attachmentsLoadedMsg{err: err}
			}
			for _, fi := range infos {
				if strings.HasPrefix(fi.MimeType, "image/") {
					images = append(images, imageAttachment{
						label:  fmt.Sprintf("%s %s — %s", p.time, p.author, fi.Name),
						fileID: fi.Id,
						name:   fi.Name,
					})
				}
			}
		}
		return attachmentsLoadedMsg{images: images}
	}
}

func (m Model) downloadImageCmd(img imageAttachment) tea.Cmd {
	return func() tea.Msg {
		data, _, err := m.mm.Client.GetFile(m.ctx, img.fileID)
		if err != nil {
			return imageReadyMsg{err: err}
		}
		ext := filepath.Ext(img.name)
		if ext == "" {
			ext = ".img"
		}
		f, err := os.CreateTemp("", "mm-*"+ext)
		if err != nil {
			return imageReadyMsg{err: err}
		}
		if _, err := f.Write(data); err != nil {
			f.Close()
			return imageReadyMsg{err: err}
		}
		f.Close()
		return imageReadyMsg{path: f.Name()}
	}
}

// viewImageCmd suspends the TUI and renders the image with chafa (which
// auto-detects the best protocol: sixel/kitty/iterm/symbols), waiting for Enter.
func viewImageCmd(path string) tea.Cmd {
	script := fmt.Sprintf("clear; chafa %s; printf '\\n[enter] to close'; read _ < /dev/tty", shellQuote(path))
	c := exec.Command("sh", "-c", script)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return imageClosedMsg{path: path}
	})
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// handleScheduleViewKey drives the in-TUI scheduled-messages viewer.
func (m Model) handleScheduleViewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q":
		m.scheduleViewMode = false
		return m, nil
	case "down", "j":
		if m.scheduleViewCursor < len(m.scheduleView)-1 {
			m.scheduleViewCursor++
		}
		return m, nil
	case "up", "k":
		if m.scheduleViewCursor > 0 {
			m.scheduleViewCursor--
		}
		return m, nil
	case "x", "d":
		if m.scheduleViewCursor >= len(m.scheduleView) {
			return m, nil
		}
		id := m.scheduleView[m.scheduleViewCursor].ID
		store, err := schedule.Load()
		if err == nil {
			if err = store.Remove(id); err == nil {
				err = store.Save()
			}
		}
		if err != nil {
			m.status = "cancel error: " + err.Error()
			return m, nil
		}
		// Reload the view from disk and clamp the cursor.
		if store, err = schedule.Load(); err == nil {
			m.scheduleView = store.Sorted()
		}
		if m.scheduleViewCursor >= len(m.scheduleView) && m.scheduleViewCursor > 0 {
			m.scheduleViewCursor--
		}
		m.status = "scheduled message cancelled"
		return m, nil
	}
	return m, nil
}

func (m Model) markReadCmd(channelID string) tea.Cmd {
	return func() tea.Msg {
		_ = m.mm.MarkChannelRead(m.ctx, channelID)
		return channelsReloadMsg{}
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
	// Reserve the terminal's last row. A frame that fills the full height makes
	// the terminal scroll up one line when the bottom-right cell is written,
	// which eats the panes' top border. Leaving the last row untouched avoids it.
	const bottomReserve = 1
	contentH := m.height - footerH - bottomReserve

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

// connectWSCmd dials the WebSocket; returns wsConnectedMsg or wsClosedMsg.
func (m Model) connectWSCmd() tea.Cmd {
	return func() tea.Msg {
		conn, err := m.mm.ConnectWS()
		if err != nil {
			return wsClosedMsg{err: err}
		}
		return wsConnectedMsg{ws: conn}
	}
}

// waitWSEventCmd blocks on the next server event, re-armed after each one.
func waitWSEventCmd(events chan *model.WebSocketEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-events
		if !ok {
			return wsClosedMsg{}
		}
		return wsEventMsg{ev: ev}
	}
}

func reconnectCmd() tea.Cmd {
	return tea.Tick(reconnectInterval*time.Second, func(time.Time) tea.Msg {
		return wsReconnectMsg{}
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
		// Resolve DM participants. Track which are active so we can hide DMs with
		// deactivated users (DeleteAt > 0).
		names := map[string]string{}
		active := map[string]bool{}
		if len(dmOtherIDs) > 0 {
			users, _, uerr := m.mm.Client.GetUsersByIds(m.ctx, dmOtherIDs)
			if uerr != nil {
				return errMsg{fmt.Errorf("could not resolve users: %w", uerr)}
			}
			for _, u := range users {
				names[u.Id] = "@" + u.Username
				active[u.Id] = u.DeleteAt == 0
			}
		}

		// Alias store lets DMs show "luis" instead of "@luisdavid.francisco".
		aliases, _ := alias.Load()

		// Per-channel read state for unread prioritization (best-effort).
		members, _ := m.mm.ChannelMembers(m.ctx)

		// Favorited channels/DMs are pinned to the top.
		favs, _ := m.mm.FavoriteChannels(m.ctx)

		items := make([]channelItem, 0, len(chans))
		for _, ch := range chans {
			typ := channelTypeLabel(ch.Type)
			var name, desc, username string
			switch ch.Type {
			case model.ChannelTypeDirect:
				other := ch.GetOtherUserIdForDM(m.mm.UserID)
				if !active[other] {
					continue // hide DMs with deactivated (or unknown) users
				}
				username = strings.TrimPrefix(names[other], "@")
				name, desc = dmLabel(username, aliases)
			default:
				name = ch.DisplayName
				if name == "" {
					name = ch.Name
				}
				desc = "# " + typ
			}

			it := channelItem{id: ch.Id, name: name, desc: desc, typ: typ, username: username, lastPostAt: ch.LastPostAt, favorite: favs[ch.Id]}
			if mbr := members[ch.Id]; mbr != nil {
				it.unread = ch.LastPostAt > mbr.LastViewedAt
				it.mentions = int(mbr.MentionCount)
			}
			items = append(items, it)
		}

		// Unread first (most recent activity on top), then named channels, then
		// DMs; alphabetical within the read groups.
		sort.SliceStable(items, func(i, j int) bool { return channelLess(items[i], items[j]) })

		listItems := make([]list.Item, len(items))
		for i, it := range items {
			listItems[i] = it
		}
		return channelsLoadedMsg{items: listItems}
	}
}

// buildPostLines resolves usernames and converts a PostList (newest-first Order)
// into chronological postLines.
func (m Model) buildPostLines(posts *model.PostList) ([]postLine, error) {
	ids := make([]string, 0, len(posts.Order))
	for _, id := range posts.Order {
		ids = append(ids, posts.Posts[id].UserId)
	}
	usernames, err := m.mm.ResolveUsernames(m.ctx, ids)
	if err != nil {
		return nil, err
	}
	lines := make([]postLine, 0, len(posts.Order))
	for i := len(posts.Order) - 1; i >= 0; i-- {
		p := posts.Posts[posts.Order[i]]
		lines = append(lines, postLine{
			postID:    p.Id,
			time:      time.UnixMilli(p.CreateAt).Format("15:04"),
			author:    usernames[p.UserId],
			message:   p.Message,
			fileIDs:   p.FileIds,
			reactions: formatReactions(p),
		})
	}
	return lines, nil
}

// formatReactions aggregates a post's reactions into "👍 2  🎉 1" (from the
// metadata already attached to the post; no extra API calls).
func formatReactions(p *model.Post) string {
	if p.Metadata == nil || len(p.Metadata.Reactions) == 0 {
		return ""
	}
	counts := map[string]int{}
	var order []string
	for _, r := range p.Metadata.Reactions {
		if counts[r.EmojiName] == 0 {
			order = append(order, r.EmojiName)
		}
		counts[r.EmojiName]++
	}
	parts := make([]string, 0, len(order))
	for _, name := range order {
		parts = append(parts, fmt.Sprintf("%s %d", emojiGlyph(name), counts[name]))
	}
	return strings.Join(parts, "  ")
}

// keepNewest returns at most max posts, dropping the oldest (front).
func keepNewest(posts []postLine, max int) []postLine {
	if len(posts) > max {
		return posts[len(posts)-max:]
	}
	return posts
}

// keepOldest returns at most max posts, dropping the newest (tail).
func keepOldest(posts []postLine, max int) []postLine {
	if len(posts) > max {
		return posts[:max]
	}
	return posts
}

// markdownFor builds the rendered-input blob for a set of posts.
func markdownFor(lines []postLine) string {
	var b strings.Builder
	for _, p := range lines {
		fmt.Fprintf(&b, "**%s · %s**\n\n%s\n", p.time, p.author, p.message)
		if p.reactions != "" {
			fmt.Fprintf(&b, "\n%s\n", p.reactions)
		}
		b.WriteString("\n---\n\n")
	}
	return b.String()
}

// ownPostsFrom returns the current user's posts newest-first (for up-arrow edit).
func (m Model) ownPostsFrom(lines []postLine) []ownPost {
	me := "@" + m.mm.Username
	var own []ownPost
	for i := len(lines) - 1; i >= 0; i-- {
		if lines[i].author == me {
			own = append(own, ownPost{id: lines[i].postID, message: lines[i].message})
		}
	}
	return own
}

// renderPosts re-renders m.posts into the viewport via glamour.
func (m *Model) renderPosts() {
	md := markdownFor(m.posts)
	rendered, err := m.renderer.Render(md)
	if err != nil {
		rendered = md
	}
	m.viewport.SetContent(rendered)
}

func (m Model) loadPostsCmd(channelID string) tea.Cmd {
	return func() tea.Msg {
		posts, _, err := m.mm.Client.GetPostsForChannel(m.ctx, channelID, 0, m.limit, "", false, false)
		if err != nil {
			return errMsg{fmt.Errorf("could not fetch posts: %w", err)}
		}
		lines, err := m.buildPostLines(posts)
		if err != nil {
			return errMsg{err}
		}
		return postsLoadedMsg{channelID: channelID, posts: lines}
	}
}

// loadOlderCmd fetches the page of messages before the oldest one loaded.
func (m Model) loadOlderCmd(channelID string) tea.Cmd {
	if len(m.posts) == 0 {
		return nil
	}
	oldest := m.posts[0].postID
	return func() tea.Msg {
		posts, _, err := m.mm.Client.GetPostsBefore(m.ctx, channelID, oldest, 0, m.limit, "", false, false)
		if err != nil {
			return errMsg{fmt.Errorf("could not fetch older posts: %w", err)}
		}
		lines, err := m.buildPostLines(posts)
		if err != nil {
			return errMsg{err}
		}
		return olderLoadedMsg{channelID: channelID, posts: lines}
	}
}

// loadNewerCmd fetches messages after the newest one loaded (live updates).
func (m Model) loadNewerCmd(channelID string) tea.Cmd {
	if len(m.posts) == 0 {
		return m.loadPostsCmd(channelID)
	}
	newest := m.posts[len(m.posts)-1].postID
	return func() tea.Msg {
		posts, _, err := m.mm.Client.GetPostsAfter(m.ctx, channelID, newest, 0, m.limit, "", false, false)
		if err != nil {
			return errMsg{fmt.Errorf("could not fetch new posts: %w", err)}
		}
		lines, err := m.buildPostLines(posts)
		if err != nil {
			return errMsg{err}
		}
		return newerLoadedMsg{channelID: channelID, posts: lines}
	}
}

// channelLess orders the sidebar: favorites first, then unread (most recent
// activity on top), then named channels, then DMs, alphabetical within groups.
func channelLess(a, b channelItem) bool {
	if a.favorite != b.favorite {
		return a.favorite
	}
	if a.unread != b.unread {
		return a.unread
	}
	if a.unread {
		return a.lastPostAt > b.lastPostAt
	}
	if da, db := a.typ == "dm", b.typ == "dm"; da != db {
		return !da
	}
	return strings.ToLower(a.name) < strings.ToLower(b.name)
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
