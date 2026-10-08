package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/spf13/cobra"

	"github.com/carlosprados/mm/internal/client"
)

var (
	unreadLimit    int
	unreadMentions bool
	unreadCounts   bool
	unreadIDs      bool
	unreadJSON     bool
)

var unreadCmd = &cobra.Command{
	Use:   "unread",
	Short: "Show what you missed: unread messages across all channels and DMs",
	Long: "List every channel and DM with unread activity, those that mention you\n" +
		"first, with the messages you have not seen yet.\n\n" +
		"Read-only: nothing is marked as read, so the web and mobile clients keep\n" +
		"their unread state.",
	Example: `  mm unread
  mm unread --mentions          # only where you were mentioned
  mm unread --counts            # one line per channel, no messages
  mm unread --json | jq '.[] | select(.mentions > 0)'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		mm, err := client.New(ctx)
		if err != nil {
			return err
		}

		unread, err := mm.Unread(ctx, client.UnreadOptions{
			Limit:        unreadLimit,
			MentionsOnly: unreadMentions,
			CountsOnly:   unreadCounts,
		})
		if err != nil {
			return err
		}

		if unreadJSON {
			return writeUnreadJSON(os.Stdout, unread)
		}
		writeUnreadText(os.Stdout, unread, unreadIDs)
		return nil
	},
}

// jsonUnread is the stable shape of `mm unread --json` output. Channel is the
// bare slug (for -c) or "@username" (for -u), like the list_unread MCP tool.
type jsonUnread struct {
	Channel   string        `json:"channel"`
	Type      string        `json:"type"`
	Mentions  int           `json:"mentions"`
	Unread    int           `json:"unread"`
	Truncated bool          `json:"truncated,omitempty"`
	Messages  []jsonMessage `json:"messages,omitempty"`
}

func writeUnreadJSON(w io.Writer, unread []client.UnreadChannel) error {
	out := make([]jsonUnread, 0, len(unread))
	for _, u := range unread {
		ju := jsonUnread{
			Channel:   u.Name,
			Type:      client.ChannelTypeLabel(u.Type),
			Mentions:  u.Mentions,
			Unread:    u.Count,
			Truncated: len(u.Messages) > 0 && u.Truncated(),
		}
		for _, m := range u.Messages {
			ju.Messages = append(ju.Messages, toJSONMessage(m))
		}
		out = append(out, ju)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("could not write JSON: %w", err)
	}
	return nil
}

func writeUnreadText(w io.Writer, unread []client.UnreadChannel, withIDs bool) {
	if len(unread) == 0 {
		fmt.Fprintln(w, "Nothing unread.")
		return
	}
	for i, u := range unread {
		if i > 0 && len(u.Messages) > 0 {
			fmt.Fprintln(w)
		}
		header := fmt.Sprintf("%s — %d unread", unreadLabel(u), u.Count)
		if u.Mentions > 0 {
			header += fmt.Sprintf(", %d mention(s)", u.Mentions)
		}
		fmt.Fprintln(w, header)
		if len(u.Messages) > 0 && u.Truncated() {
			fmt.Fprintf(w, "  … %d older unread not shown (raise -n)\n", u.Count-len(u.Messages))
		}
		for _, m := range u.Messages {
			fmt.Fprintln(w, "  "+formatMessageLine(m, withIDs))
		}
	}
}

// unreadLabel names a channel the way the other commands target it: "#slug"
// for -c, "@user" for -u; group DMs keep their display name.
func unreadLabel(u client.UnreadChannel) string {
	switch u.Type {
	case model.ChannelTypeDirect, model.ChannelTypeGroup:
		return u.Name
	default:
		return "#" + u.Name
	}
}

func init() {
	unreadCmd.Flags().IntVarP(&unreadLimit, "limit", "n", client.DefaultUnreadLimit, "Max unread messages shown per channel")
	unreadCmd.Flags().BoolVar(&unreadMentions, "mentions", false, "Only channels and DMs where you were mentioned")
	unreadCmd.Flags().BoolVar(&unreadCounts, "counts", false, "Only the per-channel counts, without messages")
	unreadCmd.Flags().BoolVar(&unreadIDs, "ids", false, "Show each message's post ID")
	unreadCmd.Flags().BoolVar(&unreadJSON, "json", false, "Output JSON (always includes post IDs)")
	rootCmd.AddCommand(unreadCmd)
}
