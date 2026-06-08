package cmd

import (
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/spf13/cobra"

	"github.com/carlosprados/mm/internal/alias"
	"github.com/carlosprados/mm/internal/client"
)

var (
	sendChannel string
	sendUser    string
	sendMessage string
)

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a message to a channel or user (DM)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		mm, err := client.New(ctx)
		if err != nil {
			return err
		}

		var channelID string

		if sendUser != "" {
			// DM: resolve alias → username → userID → DM channel
			store, err := alias.Load()
			if err != nil {
				return err
			}
			username := store.Resolve(sendUser)
			user, _, err := mm.Client.GetUserByUsername(ctx, username, "")
			if err != nil {
				return fmt.Errorf("user not found: %w", err)
			}
			channelID, err = mm.GetDirectChannelWith(ctx, user.Id)
			if err != nil {
				return err
			}
		} else if sendChannel != "" {
			ch, _, err := mm.Client.GetChannelByName(ctx, sendChannel, mm.TeamID, "")
			if err != nil {
				return fmt.Errorf("channel not found: %w", err)
			}
			channelID = ch.Id
		} else {
			return fmt.Errorf("specify --channel or --user")
		}

		post := &model.Post{
			ChannelId: channelID,
			Message:   sendMessage,
		}

		_, _, err = mm.Client.CreatePost(ctx, post)
		if err != nil {
			return fmt.Errorf("could not send message: %w", err)
		}

		fmt.Println("Message sent.")
		return nil
	},
}

func init() {
	sendCmd.Flags().StringVarP(&sendChannel, "channel", "c", "", "Target channel name")
	sendCmd.Flags().StringVarP(&sendUser, "user", "u", "", "Target username (DM)")
	sendCmd.Flags().StringVarP(&sendMessage, "message", "m", "", "Message to send (required)")
	sendCmd.MarkFlagRequired("message")
	rootCmd.AddCommand(sendCmd)
}
