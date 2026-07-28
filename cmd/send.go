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
	sendFiles   []string
)

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a message to a channel or user (DM), optionally with file attachments",
	Example: `  mm send -c dev-backend -m "Deploy listo"
  mm send -c dev-backend -m "Logs del fallo" -f ./error.log
  mm send -u alex -f ./informe.pdf -f ./captura.png`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if sendMessage == "" && len(sendFiles) == 0 {
			return fmt.Errorf("provide a message with --message, a file with --file, or both")
		}

		ctx := cmd.Context()
		mm, err := client.New(ctx)
		if err != nil {
			return err
		}

		target := client.Target{Channel: sendChannel, User: sendUser}
		if _, _, err := mm.SendFiles(ctx, target, sendMessage, sendFiles); err != nil {
			return err
		}

		switch n := len(sendFiles); n {
		case 0:
			fmt.Println("Message sent.")
		case 1:
			fmt.Println("Message sent with 1 attachment.")
		default:
			fmt.Printf("Message sent with %d attachments.\n", n)
		}
		return nil
	},
}

func init() {
	sendCmd.Flags().StringVarP(&sendChannel, "channel", "c", "", "Target channel name")
	sendCmd.Flags().StringVarP(&sendUser, "user", "u", "", "Target username or alias (DM)")
	sendCmd.Flags().StringVarP(&sendMessage, "message", "m", "", "Message to send (optional if --file is given)")
	sendCmd.Flags().StringArrayVarP(&sendFiles, "file", "f", nil, "Path to a file to attach (repeatable)")
	rootCmd.AddCommand(sendCmd)
}
