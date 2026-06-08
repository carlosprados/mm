package client

import (
	"context"
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/carlosprados/mm/internal/alias"
)

// Target identifies where a message goes. Exactly one of Channel (a channel
// slug) or User (a username or alias) must be set.
type Target struct {
	Channel string
	User    string
}

func (t Target) validate() error {
	if (t.Channel == "") == (t.User == "") {
		return fmt.Errorf("specify exactly one of channel or user")
	}
	return nil
}

// ResolveChannelID turns a Target into a channel ID. For a DM it resolves
// aliases to a canonical username, looks up the user and opens the direct
// channel. Centralizing this keeps the CLI, TUI and MCP send paths from
// drifting (see the CLI↔MCP parity rule).
func (mm *MM) ResolveChannelID(ctx context.Context, t Target) (string, error) {
	if err := t.validate(); err != nil {
		return "", err
	}
	if t.User != "" {
		store, err := alias.Load()
		if err != nil {
			return "", err
		}
		user, _, err := mm.Client.GetUserByUsername(ctx, store.Resolve(t.User), "")
		if err != nil {
			return "", fmt.Errorf("user not found: %w", err)
		}
		return mm.GetDirectChannelWith(ctx, user.Id)
	}
	ch, _, err := mm.Client.GetChannelByName(ctx, t.Channel, mm.TeamID, "")
	if err != nil {
		return "", fmt.Errorf("channel not found: %w", err)
	}
	return ch.Id, nil
}

// Send posts a message immediately. It returns the resolved channel ID and the
// created post ID.
func (mm *MM) Send(ctx context.Context, t Target, message string) (channelID, postID string, err error) {
	channelID, err = mm.ResolveChannelID(ctx, t)
	if err != nil {
		return "", "", err
	}
	postID, err = mm.SendToChannelID(ctx, channelID, message)
	return channelID, postID, err
}

// SendToChannelID posts a message to an already-resolved channel ID. The TUI
// uses this since it tracks the active channel by ID.
func (mm *MM) SendToChannelID(ctx context.Context, channelID, message string) (postID string, err error) {
	if message == "" {
		return "", fmt.Errorf("message is required")
	}
	post, _, err := mm.Client.CreatePost(ctx, &model.Post{ChannelId: channelID, Message: message})
	if err != nil {
		return "", fmt.Errorf("could not send message: %w", err)
	}
	return post.Id, nil
}

// LastOwnPostID returns the ID of the most recent post authored by the current
// user in the channel, scanning a recent window. Used by `mm edit` / the
// edit_message MCP tool to edit "your last message" without needing a post ID.
func (mm *MM) LastOwnPostID(ctx context.Context, channelID string) (string, error) {
	posts, _, err := mm.Client.GetPostsForChannel(ctx, channelID, 0, 50, "", false, false)
	if err != nil {
		return "", fmt.Errorf("could not fetch posts: %w", err)
	}
	for _, id := range posts.Order { // Order is newest-first
		if posts.Posts[id].UserId == mm.UserID {
			return posts.Posts[id].Id, nil
		}
	}
	return "", fmt.Errorf("no message of yours found in this channel")
}

// EditPost updates the body of an existing post. The server only allows editing
// the authenticated user's own posts (and may enforce an edit time limit).
func (mm *MM) EditPost(ctx context.Context, postID, message string) error {
	if message == "" {
		return fmt.Errorf("message is required")
	}
	_, _, err := mm.Client.PatchPost(ctx, postID, &model.PostPatch{Message: &message})
	if err != nil {
		return fmt.Errorf("could not edit message: %w", err)
	}
	return nil
}

// Scheduling is implemented client-side in internal/schedule (this server has
// no scheduled-posts license). The shared delivery primitive is SendToChannelID
// above; the TUI's delivery loop calls it when a scheduled item is due.

// ChannelMembers returns the current user's membership for each channel in the
// team (including DMs), keyed by channel ID. Used to compute unread state.
func (mm *MM) ChannelMembers(ctx context.Context) (map[string]*model.ChannelMember, error) {
	members, _, err := mm.Client.GetChannelMembersForUser(ctx, mm.UserID, mm.TeamID, "")
	if err != nil {
		return nil, fmt.Errorf("could not fetch channel members: %w", err)
	}
	out := make(map[string]*model.ChannelMember, len(members))
	for i := range members {
		out[members[i].ChannelId] = &members[i]
	}
	return out, nil
}

// MarkChannelRead marks a channel as read for the current user (server-side, so
// it also clears the unread state on the web/mobile clients).
func (mm *MM) MarkChannelRead(ctx context.Context, channelID string) error {
	_, _, err := mm.Client.ViewChannel(ctx, mm.UserID, &model.ChannelView{ChannelId: channelID})
	if err != nil {
		return fmt.Errorf("could not mark channel read: %w", err)
	}
	return nil
}
