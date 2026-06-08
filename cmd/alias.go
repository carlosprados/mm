package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/carlosprados/mm/internal/alias"
)

var aliasCmd = &cobra.Command{
	Use:   "alias",
	Short: "Manage short handles that map to canonical usernames",
	Long: "Aliases let you DM a colleague by a short handle. For example, after\n" +
		"'mm alias add luis luisdavid.francisco' you can run 'mm send -u luis -m ...'.\n" +
		"Several aliases may point to the same username.",
}

var aliasAddCmd = &cobra.Command{
	Use:   "add <alias> <username>",
	Short: "Add or overwrite an alias",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := alias.Load()
		if err != nil {
			return err
		}
		if err := store.Add(args[0], args[1]); err != nil {
			return err
		}
		if err := store.Save(); err != nil {
			return err
		}
		fmt.Printf("Alias %q → %q saved.\n", args[0], args[1])
		return nil
	},
}

var aliasRmCmd = &cobra.Command{
	Use:     "rm <alias>",
	Aliases: []string{"remove", "del"},
	Short:   "Remove an alias",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := alias.Load()
		if err != nil {
			return err
		}
		if err := store.Remove(args[0]); err != nil {
			return err
		}
		if err := store.Save(); err != nil {
			return err
		}
		fmt.Printf("Alias %q removed.\n", args[0])
		return nil
	},
}

var aliasListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all aliases",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := alias.Load()
		if err != nil {
			return err
		}
		if len(store.Aliases) == 0 {
			fmt.Println("No aliases configured. Add one with 'mm alias add <alias> <username>'.")
			return nil
		}
		names := make([]string, 0, len(store.Aliases))
		for a := range store.Aliases {
			names = append(names, a)
		}
		sort.Strings(names)
		for _, a := range names {
			fmt.Printf("%-15s → %s\n", a, store.Aliases[a])
		}
		return nil
	},
}

func init() {
	aliasCmd.AddCommand(aliasAddCmd, aliasRmCmd, aliasListCmd)
	rootCmd.AddCommand(aliasCmd)
}
