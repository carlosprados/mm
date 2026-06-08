package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

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

		if _, _, err := mm.Send(ctx, client.Target{Channel: sendChannel, User: sendUser}, sendMessage); err != nil {
			return err
		}

		fmt.Println("Message sent.")
		return nil
	},
}

func init() {
	sendCmd.Flags().StringVarP(&sendChannel, "channel", "c", "", "Target channel name")
	sendCmd.Flags().StringVarP(&sendUser, "user", "u", "", "Target username or alias (DM)")
	sendCmd.Flags().StringVarP(&sendMessage, "message", "m", "", "Message to send (required)")
	sendCmd.MarkFlagRequired("message")
	rootCmd.AddCommand(sendCmd)
}
