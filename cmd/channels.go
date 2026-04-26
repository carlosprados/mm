package cmd

import (
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/spf13/cobra"
	"github.com/carlosprados/mm/internal/client"
)

var channelsCmd = &cobra.Command{
	Use:   "channels",
	Short: "List joined channels",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		mm, err := client.New(ctx)
		if err != nil {
			return err
		}

		channels, _, err := mm.Client.GetChannelsForTeamForUser(ctx, mm.TeamID, mm.UserID, false, "")
		if err != nil {
			return fmt.Errorf("could not fetch channels: %w", err)
		}

		for _, ch := range channels {
			typeLabel := "public"
			switch ch.Type {
			case model.ChannelTypePrivate:
				typeLabel = "private"
			case model.ChannelTypeDirect:
				typeLabel = "dm"
			}
			fmt.Printf("[%-7s] %s\n", typeLabel, ch.Name)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(channelsCmd)
}
