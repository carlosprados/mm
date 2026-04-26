package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/carlosprados/mm/internal/client"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the active session",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		mm, err := client.New(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("@%s\n", mm.Username)
		fmt.Printf("  url:    %s\n", mm.URL)
		if mm.TeamName != "" {
			fmt.Printf("  team:   %s\n", mm.TeamName)
		}
		fmt.Printf("  source: %s\n", mm.Source)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}
