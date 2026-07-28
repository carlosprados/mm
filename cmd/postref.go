package cmd

import (
	"context"

	"github.com/carlosprados/mm/internal/client"
)

// resolvePost turns a command's post targeting flags into a post ID: --post
// (an ID or a permalink) wins, otherwise it is the user's Nth most recent
// message in the target channel or DM. Shared by `mm edit` and `mm delete`.
func resolvePost(ctx context.Context, mm *client.MM, postRef, channel, user string, nth int) (string, error) {
	if postRef != "" {
		return client.ParsePostRef(postRef)
	}
	channelID, err := mm.ResolveChannelID(ctx, client.Target{Channel: channel, User: user})
	if err != nil {
		return "", err
	}
	return mm.NthOwnPostID(ctx, channelID, nth)
}
