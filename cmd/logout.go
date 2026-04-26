package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/carlosprados/mm/internal/config"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Delete the saved session",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := config.Path()
		if err := config.Delete(); err != nil {
			return err
		}
		fmt.Printf("✓ Session removed (%s)\n", path)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(logoutCmd)
}
