package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/carlosprados/mm/internal/client"
	"github.com/carlosprados/mm/internal/tui"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the interactive terminal UI",
	Long: "Opens a full-screen terminal client: a channel sidebar on the left and a\n" +
		"scrollable, Markdown-rendered message pane on the right. The active channel\n" +
		"is polled for new messages.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		mm, err := client.New(ctx)
		if err != nil {
			return err
		}
		p := tea.NewProgram(tui.New(ctx, mm), tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
