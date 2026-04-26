package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/carlosprados/mm/internal/client"
)

var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "List team members",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		mm, err := client.New(ctx)
		if err != nil {
			return err
		}

		users, _, err := mm.Client.GetUsersInTeam(ctx, mm.TeamID, 0, 100, "")
		if err != nil {
			return fmt.Errorf("could not fetch users: %w", err)
		}

		for _, u := range users {
			fmt.Printf("@%-20s %s %s\n", u.Username, u.FirstName, u.LastName)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(usersCmd)
}
