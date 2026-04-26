package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/carlosprados/mm/internal/client"
)

var (
	readChannel string
	readLimit   int
)

var readCmd = &cobra.Command{
	Use:   "read",
	Short: "Read messages from a channel or DM",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		mm, err := client.New(ctx)
		if err != nil {
			return err
		}

		channel, _, err := mm.Client.GetChannelByName(ctx, readChannel, mm.TeamID, "")
		if err != nil {
			return fmt.Errorf("channel not found: %w", err)
		}

		posts, _, err := mm.Client.GetPostsForChannel(ctx, channel.Id, 0, readLimit, "", false, false)
		if err != nil {
			return fmt.Errorf("could not fetch posts: %w", err)
		}

		// Collect all unique user IDs from the posts.
		userIDSet := make(map[string]struct{})
		for _, id := range posts.Order {
			userIDSet[posts.Posts[id].UserId] = struct{}{}
		}
		uniqueIDs := make([]string, 0, len(userIDSet))
		for id := range userIDSet {
			uniqueIDs = append(uniqueIDs, id)
		}

		usernames, err := mm.ResolveUsernames(ctx, uniqueIDs)
		if err != nil {
			return fmt.Errorf("could not resolve usernames: %w", err)
		}

		// Print posts in chronological order (Order is newest-first, so reverse it).
		for i := len(posts.Order) - 1; i >= 0; i-- {
			post := posts.Posts[posts.Order[i]]
			ts := time.UnixMilli(post.CreateAt).Format("15:04")
			who := usernames[post.UserId]
			fmt.Printf("[%s] %s: %s\n", ts, who, post.Message)
		}
		return nil
	},
}

func init() {
	readCmd.Flags().StringVarP(&readChannel, "channel", "c", "", "Channel name (required)")
	readCmd.Flags().IntVarP(&readLimit, "limit", "n", 20, "Number of messages to fetch")
	readCmd.MarkFlagRequired("channel")
	rootCmd.AddCommand(readCmd)
}
