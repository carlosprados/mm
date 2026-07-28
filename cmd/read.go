package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/carlosprados/mm/internal/client"
)

var (
	readChannel string
	readUser    string
	readLimit   int
	readIDs     bool
	readMine    bool
	readJSON    bool
)

var readCmd = &cobra.Command{
	Use:   "read",
	Short: "Read messages from a channel or a DM",
	Long: "Read recent messages from a channel (-c) or a DM (-u, username or alias).\n\n" +
		"Use --ids to print each message's post ID, or --json for machine-readable\n" +
		"output: those IDs are what 'mm edit --post <id>' needs.",
	Example: `  mm read -c dev-backend -n 10
  mm read -u alex --mine --ids       # your own messages, with post IDs
  mm read -c dev-backend --json | jq '.[] | select(.own)'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		mm, err := client.New(ctx)
		if err != nil {
			return err
		}

		target := client.Target{Channel: readChannel, User: readUser}
		msgs, err := mm.ReadMessages(ctx, target, client.ReadOptions{
			Limit:    readLimit,
			OnlyMine: readMine,
		})
		if err != nil {
			return err
		}

		if readJSON {
			return writeMessagesJSON(msgs)
		}
		for _, m := range msgs {
			fmt.Println(formatMessageLine(m, readIDs))
		}
		return nil
	},
}

// jsonMessage is the stable shape of `mm read --json` output.
type jsonMessage struct {
	ID      string   `json:"id"`
	Time    string   `json:"time"` // RFC3339, local zone
	From    string   `json:"from"`
	Text    string   `json:"text"`
	Own     bool     `json:"own"`
	FileIDs []string `json:"file_ids,omitempty"`
}

func writeMessagesJSON(msgs []client.Message) error {
	out := make([]jsonMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, jsonMessage{
			ID:      m.ID,
			Time:    time.UnixMilli(m.CreateAt).Format(time.RFC3339),
			From:    m.Author,
			Text:    m.Text,
			Own:     m.Own,
			FileIDs: m.FileIDs,
		})
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("could not write JSON: %w", err)
	}
	return nil
}

// formatMessageLine renders one message for humans, optionally with its post ID
// so it can be fed straight to `mm edit --post`.
func formatMessageLine(m client.Message, withID bool) string {
	ts := time.UnixMilli(m.CreateAt).Format("15:04")
	if withID {
		return fmt.Sprintf("[%s] %s %s: %s", ts, m.ID, m.Author, m.Text)
	}
	return fmt.Sprintf("[%s] %s: %s", ts, m.Author, m.Text)
}

func init() {
	readCmd.Flags().StringVarP(&readChannel, "channel", "c", "", "Channel name (mutually exclusive with --user)")
	readCmd.Flags().StringVarP(&readUser, "user", "u", "", "Username or alias to read the DM with (mutually exclusive with --channel)")
	readCmd.Flags().IntVarP(&readLimit, "limit", "n", client.DefaultReadLimit, "Number of messages to fetch")
	readCmd.Flags().BoolVar(&readIDs, "ids", false, "Show each message's post ID (use it with 'mm edit --post')")
	readCmd.Flags().BoolVar(&readMine, "mine", false, "Only your own messages (the ones you can edit); filters the fetched window")
	readCmd.Flags().BoolVar(&readJSON, "json", false, "Output JSON (always includes post IDs)")
	rootCmd.AddCommand(readCmd)
}
