package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/carlosprados/mm/internal/client"
)

var (
	deleteChannel string
	deleteUser    string
	deletePostID  string
	deleteNth     int
	deleteYes     bool
)

var deleteCmd = &cobra.Command{
	Use:     "delete",
	Aliases: []string{"rm"},
	Short:   "Delete one of your messages",
	Long: "Deletes your most recent message in a channel or DM, an earlier one with\n" +
		"--nth, or a specific post with --post (a post ID or a Mattermost permalink).\n\n" +
		"Deleting is irreversible and removes the message for everyone, so the\n" +
		"message is shown and confirmed first. Use --yes to skip the prompt.\n\n" +
		"Find post IDs with 'mm read --ids' or 'mm read --json'.",
	Example: `  mm delete -c dev-backend                 # your last message there, with confirmation
  mm delete -u alex --nth 2
  mm delete --post 4rmsfuwfafyuiq9qkbcgzjg73y --yes
  mm rm --post https://chat.acme.com/acme/pl/4rmsfuwfafyuiq9qkbcgzjg73y`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		mm, err := client.New(ctx)
		if err != nil {
			return err
		}

		postID, err := resolvePost(ctx, mm, deletePostID, deleteChannel, deleteUser, deleteNth)
		if err != nil {
			return err
		}

		msg, err := mm.GetMessage(ctx, postID)
		if err != nil {
			return err
		}

		if !deleteYes {
			ok, err := confirmDelete(msg)
			if err != nil {
				return err
			}
			if !ok {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		if err := mm.DeletePost(ctx, postID); err != nil {
			return err
		}
		fmt.Println("Message deleted.")
		return nil
	},
}

// confirmDelete shows the message and asks for a yes/no. It refuses to guess
// when stdin is not a terminal: a piped or scripted run must pass --yes rather
// than have a destructive default applied to it.
func confirmDelete(msg client.Message) (bool, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false, fmt.Errorf("stdin is not a terminal: pass --yes to delete without confirmation")
	}

	fmt.Println("About to delete this message:")
	fmt.Println("  " + deletePreview(msg))
	if !msg.Own {
		fmt.Println("  ⚠ this message is not yours")
	}
	if len(msg.FileIDs) > 0 {
		fmt.Printf("  ⚠ it carries %d attachment(s), which go with it\n", len(msg.FileIDs))
	}
	fmt.Print("Delete it? [y/N]: ")

	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("could not read confirmation: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

// deletePreview renders a one-line summary of the message being deleted.
func deletePreview(msg client.Message) string {
	text := strings.ReplaceAll(msg.Text, "\n", " ")
	if text == "" {
		text = "(no text)"
	}
	const maxText = 80
	if r := []rune(text); len(r) > maxText { // runes, so accents/emoji aren't split
		text = string(r[:maxText]) + "…"
	}
	return fmt.Sprintf("[%s] %s: %s",
		time.UnixMilli(msg.CreateAt).Format("2006-01-02 15:04"), msg.Author, text)
}

func init() {
	deleteCmd.Flags().StringVarP(&deleteChannel, "channel", "c", "", "Target channel name")
	deleteCmd.Flags().StringVarP(&deleteUser, "user", "u", "", "Target username or alias (DM)")
	deleteCmd.Flags().StringVar(&deletePostID, "post", "", "Delete a specific post by ID or permalink instead of your last message")
	deleteCmd.Flags().IntVar(&deleteNth, "nth", 1, "Delete your Nth most recent message (1 = the last one)")
	deleteCmd.Flags().BoolVarP(&deleteYes, "yes", "y", false, "Skip the confirmation prompt")
	deleteCmd.MarkFlagsMutuallyExclusive("post", "nth")
	rootCmd.AddCommand(deleteCmd)
}
