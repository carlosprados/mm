package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/carlosprados/mm/internal/client"
)

var (
	editChannel string
	editUser    string
	editMessage string
	editPostID  string
)

var editCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit one of your messages",
	Long: "Edits your most recent message in a channel or DM, or a specific post when\n" +
		"--post is given. You can only edit your own messages.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		mm, err := client.New(ctx)
		if err != nil {
			return err
		}

		postID := editPostID
		if postID == "" {
			channelID, err := mm.ResolveChannelID(ctx, client.Target{Channel: editChannel, User: editUser})
			if err != nil {
				return err
			}
			postID, err = mm.LastOwnPostID(ctx, channelID)
			if err != nil {
				return err
			}
		}

		if err := mm.EditPost(ctx, postID, editMessage); err != nil {
			return err
		}

		fmt.Println("Message edited.")
		return nil
	},
}

func init() {
	editCmd.Flags().StringVarP(&editChannel, "channel", "c", "", "Target channel name")
	editCmd.Flags().StringVarP(&editUser, "user", "u", "", "Target username or alias (DM)")
	editCmd.Flags().StringVarP(&editMessage, "message", "m", "", "New message body (required)")
	editCmd.Flags().StringVar(&editPostID, "post", "", "Edit a specific post by ID instead of your last message")
	editCmd.MarkFlagRequired("message")
	rootCmd.AddCommand(editCmd)
}
