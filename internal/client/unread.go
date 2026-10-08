package client

import (
	"context"
	"fmt"
	"sort"

	"github.com/mattermost/mattermost/server/public/model"
)

// DefaultUnreadLimit is how many unread messages are returned per channel when
// no limit is given.
const DefaultUnreadLimit = 20

// UnreadChannel is one channel or DM with activity the user has not seen.
type UnreadChannel struct {
	ChannelID string
	// Name is what the other commands accept to target it: the channel slug,
	// or "@username" for a DM. Group DMs carry their display name and can only
	// be reached by ID.
	Name     string
	Type     model.ChannelType
	Mentions int
	// Count is the server's unread count (total minus viewed messages), which
	// can exceed len(Messages) when the per-channel limit cuts the list.
	Count      int
	LastPostAt int64
	Messages   []Message // unread posts, oldest first, system messages skipped
}

// Truncated reports whether there are more unread messages than were fetched.
func (u UnreadChannel) Truncated() bool { return u.Count > len(u.Messages) }

// UnreadOptions tunes Unread. Limit caps the messages fetched per channel;
// CountsOnly skips fetching them altogether.
type UnreadOptions struct {
	Limit        int
	MentionsOnly bool
	CountsOnly   bool
}

// Unread returns every channel and DM of the team with unread activity, those
// with mentions first, then by most recent activity. It is read-only on
// purpose: nothing is marked as read, so a summary taken by an agent does not
// clear the unread state on the web and mobile clients. Shared by the CLI and
// the MCP server (parity rule).
func (mm *MM) Unread(ctx context.Context, opts UnreadOptions) ([]UnreadChannel, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = DefaultUnreadLimit
	}

	chans, _, err := mm.Client.GetChannelsForTeamForUser(ctx, mm.TeamID, mm.UserID, false, "")
	if err != nil {
		return nil, fmt.Errorf("could not fetch channels: %w", err)
	}
	members, err := mm.ChannelMembers(ctx)
	if err != nil {
		return nil, err
	}

	var out []UnreadChannel
	var dmPeers []string
	for _, ch := range chans {
		mbr := members[ch.Id]
		if mbr == nil || ch.LastPostAt <= mbr.LastViewedAt {
			continue
		}
		if opts.MentionsOnly && mbr.MentionCount == 0 {
			continue
		}
		u := UnreadChannel{
			ChannelID:  ch.Id,
			Name:       ch.Name,
			Type:       ch.Type,
			Mentions:   int(mbr.MentionCount),
			Count:      int(max(ch.TotalMsgCount-mbr.MsgCount, 0)),
			LastPostAt: ch.LastPostAt,
		}
		switch ch.Type {
		case model.ChannelTypeDirect:
			peer := ch.GetOtherUserIdForDM(mm.UserID)
			dmPeers = append(dmPeers, peer)
			u.Name = peer // replaced by "@username" below
		case model.ChannelTypeGroup:
			u.Name = ch.DisplayName
		}
		if !opts.CountsOnly {
			msgs, err := mm.ReadMessagesFromChannelID(ctx, ch.Id, ReadOptions{
				Limit:      limit,
				Since:      mbr.LastViewedAt,
				SkipSystem: true,
			})
			if err != nil {
				return nil, fmt.Errorf("%s: %w", ch.Name, err)
			}
			u.Messages = msgs
		}
		// The server count can lag or include system posts; never report
		// fewer unread messages than were actually read.
		u.Count = max(u.Count, len(u.Messages))
		if u.Count == 0 {
			continue // LastPostAt moved for an edit, a deletion or a system post
		}
		out = append(out, u)
	}

	if len(dmPeers) > 0 {
		names, err := mm.ResolveUsernames(ctx, dmPeers)
		if err != nil {
			return nil, err
		}
		for i := range out {
			if out[i].Type == model.ChannelTypeDirect {
				out[i].Name = names[out[i].Name]
			}
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].Mentions > 0) != (out[j].Mentions > 0) {
			return out[i].Mentions > 0
		}
		return out[i].LastPostAt > out[j].LastPostAt
	})
	return out, nil
}
