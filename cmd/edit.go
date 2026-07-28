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
	editNth     int
)

var editCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit one of your messages",
	Long: "Edits your most recent message in a channel or DM, an earlier one with --nth,\n" +
		"or a specific post with --post (a post ID or a Mattermost permalink).\n" +
		"You can only edit your own messages.\n\n" +
		"Find post IDs with 'mm read --ids' or 'mm read --json'.",
	Example: `  mm edit -c dev-backend -m "Deploy listo (corregido)"
  mm edit -u alex --nth 3 -m "Corrijo el tercero por detrás"
  mm edit --post 4rmsfuwfafyuiq9qkbcgzjg73y -m "…"
  mm edit --post https://chat.acme.com/acme/pl/4rmsfuwfafyuiq9qkbcgzjg73y -m "…"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		mm, err := client.New(ctx)
		if err != nil {
			return err
		}

		var postID string
		switch {
		case editPostID != "":
			// Accepts a bare ID or the permalink from Mattermost's "Copy link".
			if postID, err = client.ParsePostRef(editPostID); err != nil {
				return err
			}
		default:
			channelID, err := mm.ResolveChannelID(ctx, client.Target{Channel: editChannel, User: editUser})
			if err != nil {
				return err
			}
			if postID, err = mm.NthOwnPostID(ctx, channelID, editNth); err != nil {
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
	editCmd.Flags().StringVar(&editPostID, "post", "", "Edit a specific post by ID or permalink instead of your last message")
	editCmd.Flags().IntVar(&editNth, "nth", 1, "Edit your Nth most recent message (1 = the last one)")
	editCmd.MarkFlagRequired("message")
	editCmd.MarkFlagsMutuallyExclusive("post", "nth")
	rootCmd.AddCommand(editCmd)
}
