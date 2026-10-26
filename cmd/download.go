package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/carlosprados/mm/internal/client"
)

var (
	downloadChannel string
	downloadUser    string
	downloadDir     string
)

var downloadCmd = &cobra.Command{
	Use:     "download [post|permalink|file-id]...",
	Aliases: []string{"dl"},
	Short:   "Download message attachments",
	Long: "Saves attachments to disk. Each argument is a post ID or permalink (all its\n" +
		"files) or a file ID (that file). With -c or -u and no arguments, it saves\n" +
		"the attachments of the most recent message there that has any.\n\n" +
		"Files go to the desktop download directory (XDG_DOWNLOAD_DIR, e.g.\n" +
		"~/Downloads) unless -o is given. Existing files are never overwritten: a\n" +
		"name already taken becomes \"name (1).ext\".\n\n" +
		"Find post and file IDs with 'mm read --json'.",
	Example: `  mm download -u alex                       # what alex just sent you
  mm download -c dev-backend -o ~/tmp
  mm download https://chat.acme.com/acme/pl/4rmsfuwfafyuiq9qkbcgzjg73y
  mm dl 8xk3wq1hsbbcpm6qsy7e9i5w1a`,
	RunE: func(cmd *cobra.Command, args []string) error {
		hasTarget := downloadChannel != "" || downloadUser != ""
		if len(args) == 0 && !hasTarget {
			return fmt.Errorf("give a post, permalink or file ID, or -c/-u to take the latest attachments there")
		}
		if len(args) > 0 && hasTarget {
			return fmt.Errorf("give either IDs or -c/-u, not both")
		}

		ctx := cmd.Context()
		mm, err := client.New(ctx)
		if err != nil {
			return err
		}

		var files []client.Attachment
		if len(args) == 0 {
			files, err = mm.LastAttachments(ctx, client.Target{Channel: downloadChannel, User: downloadUser})
			if err != nil {
				return err
			}
		}
		for _, ref := range args {
			found, err := mm.ResolveAttachments(ctx, ref)
			if err != nil {
				return err
			}
			files = append(files, found...)
		}

		dir := downloadDir
		if dir == "" {
			dir = client.DefaultDownloadDir()
		}
		for _, f := range files {
			path, err := mm.DownloadFile(ctx, f, dir)
			if err != nil {
				return err
			}
			fmt.Printf("saved %s (%s)\n", path, client.FormatSize(f.Size))
		}
		return nil
	},
}

func init() {
	downloadCmd.Flags().StringVarP(&downloadChannel, "channel", "c", "", "Take the latest attachments in this channel")
	downloadCmd.Flags().StringVarP(&downloadUser, "user", "u", "", "Take the latest attachments in the DM with this username or alias")
	downloadCmd.Flags().StringVarP(&downloadDir, "output", "o", "", "Directory to save into (default: the desktop download directory)")
	downloadCmd.MarkFlagsMutuallyExclusive("channel", "user")
	rootCmd.AddCommand(downloadCmd)
}
