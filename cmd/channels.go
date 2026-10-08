package cmd

import (
	"fmt"

	"github.com/carlosprados/mm/internal/client"
	"github.com/spf13/cobra"
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
			fmt.Printf("[%-7s] %s\n", client.ChannelTypeLabel(ch.Type), ch.Name)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(channelsCmd)
}
