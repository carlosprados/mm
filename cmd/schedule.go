package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/carlosprados/mm/internal/client"
	"github.com/carlosprados/mm/internal/schedule"
)

var (
	scheduleChannel string
	scheduleUser    string
	scheduleMessage string
	scheduleAt      string
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Schedule messages to be delivered later",
	Long: "Scheduled messages are stored locally and delivered by mm while `mm tui`\n" +
		"is running (this server has no scheduled-posts license, so delivery is\n" +
		"client-side). They are not sent if the TUI is not running at the due time;\n" +
		"overdue messages are delivered the next time the TUI starts.",
}

var scheduleAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Schedule a message for later delivery",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		at, err := schedule.ParseTime(scheduleAt)
		if err != nil {
			return err
		}
		mm, err := client.New(ctx)
		if err != nil {
			return err
		}
		channelID, err := mm.ResolveChannelID(ctx, client.Target{Channel: scheduleChannel, User: scheduleUser})
		if err != nil {
			return err
		}

		store, err := schedule.Load()
		if err != nil {
			return err
		}
		it, err := store.Add(channelID, scheduleTarget(), scheduleMessage, at)
		if err != nil {
			return err
		}
		if err := store.Save(); err != nil {
			return err
		}
		fmt.Printf("Scheduled for %s (id %s).\n", at.Format("2006-01-02 15:04"), it.ID)
		fmt.Println("Note: delivered while `mm tui` is running.")
		return nil
	},
}

var scheduleListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List your pending scheduled messages",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := schedule.Load()
		if err != nil {
			return err
		}
		items := store.Sorted()
		if len(items) == 0 {
			fmt.Println("No scheduled messages.")
			return nil
		}
		for _, it := range items {
			fmt.Printf("%s  [%s]  %s: %s\n",
				it.At.Format("2006-01-02 15:04"), it.ID, it.Label, firstLine(it.Message))
		}
		return nil
	},
}

var scheduleRmCmd = &cobra.Command{
	Use:     "rm <id>",
	Aliases: []string{"cancel", "del"},
	Short:   "Cancel a scheduled message",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := schedule.Load()
		if err != nil {
			return err
		}
		if err := store.Remove(args[0]); err != nil {
			return err
		}
		if err := store.Save(); err != nil {
			return err
		}
		fmt.Println("Scheduled message cancelled.")
		return nil
	},
}

func scheduleTarget() string {
	if scheduleUser != "" {
		return "@" + scheduleUser
	}
	return scheduleChannel
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i] + " …"
	}
	return s
}

func init() {
	scheduleAddCmd.Flags().StringVarP(&scheduleChannel, "channel", "c", "", "Target channel name")
	scheduleAddCmd.Flags().StringVarP(&scheduleUser, "user", "u", "", "Target username or alias (DM)")
	scheduleAddCmd.Flags().StringVarP(&scheduleMessage, "message", "m", "", "Message to send (required)")
	scheduleAddCmd.Flags().StringVar(&scheduleAt, "at", "", "When to deliver: \"2006-01-02 15:04\", RFC3339, or \"+2h\" (required)")
	scheduleAddCmd.MarkFlagRequired("message")
	scheduleAddCmd.MarkFlagRequired("at")

	scheduleCmd.AddCommand(scheduleAddCmd, scheduleListCmd, scheduleRmCmd)
	rootCmd.AddCommand(scheduleCmd)
}
