package client

import (
	"context"
	"fmt"
)

// DefaultReadLimit is how many posts are fetched when no limit is given.
const DefaultReadLimit = 20

// ownPostScanWindow is how many recent posts are scanned when looking for the
// current user's own messages (they may be interleaved with other people's).
const ownPostScanWindow = 50

// Message is one post as read from a channel or DM. It carries the post ID
// because that is what makes `mm edit --post` and the edit_message MCP tool
// usable: without it a caller can only ever reach its own last message.
type Message struct {
	ID       string
	CreateAt int64 // milliseconds since epoch
	UserID   string
	Author   string // "@username", or the raw user ID if it can't be resolved
	Text     string
	FileIDs  []string
	Own      bool // authored by the authenticated user
}

// ReadOptions tunes a read. Limit bounds how many posts are *fetched*; OnlyMine
// filters afterwards, so a mine-only read can return fewer than Limit messages.
type ReadOptions struct {
	Limit    int
	OnlyMine bool
}

// ReadMessages returns recent posts from a channel or DM, oldest first. Shared
// by the CLI and the MCP server so both see the same fields (parity rule).
func (mm *MM) ReadMessages(ctx context.Context, t Target, opts ReadOptions) ([]Message, error) {
	channelID, err := mm.ResolveChannelID(ctx, t)
	if err != nil {
		return nil, err
	}
	return mm.ReadMessagesFromChannelID(ctx, channelID, opts)
}

// ReadMessagesFromChannelID is ReadMessages for an already-resolved channel.
func (mm *MM) ReadMessagesFromChannelID(ctx context.Context, channelID string, opts ReadOptions) ([]Message, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = DefaultReadLimit
	}

	posts, _, err := mm.Client.GetPostsForChannel(ctx, channelID, 0, limit, "", false, false)
	if err != nil {
		return nil, fmt.Errorf("could not fetch posts: %w", err)
	}

	userIDs := make([]string, 0, len(posts.Order))
	for _, id := range posts.Order {
		userIDs = append(userIDs, posts.Posts[id].UserId)
	}
	usernames, err := mm.ResolveUsernames(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	// Order is newest-first; walk it backwards to return chronological output.
	out := make([]Message, 0, len(posts.Order))
	for i := len(posts.Order) - 1; i >= 0; i-- {
		p := posts.Posts[posts.Order[i]]
		own := p.UserId == mm.UserID
		if opts.OnlyMine && !own {
			continue
		}
		out = append(out, Message{
			ID:       p.Id,
			CreateAt: p.CreateAt,
			UserID:   p.UserId,
			Author:   usernames[p.UserId],
			Text:     p.Message,
			FileIDs:  p.FileIds,
			Own:      own,
		})
	}
	return out, nil
}

// GetMessage fetches a single post, with its author resolved. Used to show what
// is about to be deleted before doing it.
func (mm *MM) GetMessage(ctx context.Context, postID string) (Message, error) {
	p, _, err := mm.Client.GetPost(ctx, postID, "")
	if err != nil {
		return Message{}, fmt.Errorf("post not found: %w", err)
	}
	authors, err := mm.ResolveUsernames(ctx, []string{p.UserId})
	if err != nil {
		return Message{}, err
	}
	return Message{
		ID:       p.Id,
		CreateAt: p.CreateAt,
		UserID:   p.UserId,
		Author:   authors[p.UserId],
		Text:     p.Message,
		FileIDs:  p.FileIds,
		Own:      p.UserId == mm.UserID,
	}, nil
}

// DeletePost deletes a post. This is irreversible from the client's point of
// view: the server soft-deletes it and it disappears for everyone. Mattermost
// only allows deleting your own posts unless the account has the
// delete_others_posts permission.
func (mm *MM) DeletePost(ctx context.Context, postID string) error {
	if _, err := mm.Client.DeletePost(ctx, postID); err != nil {
		return fmt.Errorf("could not delete message: %w", err)
	}
	return nil
}

// OwnPostIDs returns the IDs of the current user's most recent posts in the
// channel, newest first, scanning a recent window. Used to target "your Nth
// message back" without needing a post ID.
func (mm *MM) OwnPostIDs(ctx context.Context, channelID string, max int) ([]string, error) {
	if max <= 0 {
		max = 1
	}
	posts, _, err := mm.Client.GetPostsForChannel(ctx, channelID, 0, ownPostScanWindow, "", false, false)
	if err != nil {
		return nil, fmt.Errorf("could not fetch posts: %w", err)
	}

	ids := make([]string, 0, max)
	for _, id := range posts.Order { // Order is newest-first
		if posts.Posts[id].UserId != mm.UserID {
			continue
		}
		ids = append(ids, posts.Posts[id].Id)
		if len(ids) == max {
			break
		}
	}
	return ids, nil
}

// NthOwnPostID returns the ID of the user's nth most recent post (n = 1 is the
// last message they sent).
func (mm *MM) NthOwnPostID(ctx context.Context, channelID string, n int) (string, error) {
	if n <= 0 {
		return "", fmt.Errorf("message number must be 1 or greater")
	}
	ids, err := mm.OwnPostIDs(ctx, channelID, n)
	if err != nil {
		return "", err
	}
	if len(ids) < n {
		if len(ids) == 0 {
			return "", fmt.Errorf("no message of yours found in this channel")
		}
		return "", fmt.Errorf("only found %d message(s) of yours in the last %d posts, cannot reach number %d",
			len(ids), ownPostScanWindow, n)
	}
	return ids[n-1], nil
}
