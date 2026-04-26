package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/carlosprados/mm/internal/client"
	"github.com/carlosprados/mm/internal/config"
)

var (
	loginURL   string
	loginToken string
	loginTeam  string
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate against a Mattermost server and persist the session",
	Long: `Login validates your credentials against the Mattermost server and stores them
in $XDG_CONFIG_HOME/mm/config.json (mode 0600). Subsequent commands reuse the
saved session — no need to export MM_URL / MM_TOKEN every shell.

Flags are optional; missing values are prompted on stdin. The token is read
without echo.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		url := strings.TrimSpace(loginURL)
		team := strings.TrimSpace(loginTeam)
		token := loginToken

		reader := bufio.NewReader(os.Stdin)

		if url == "" {
			fmt.Print("Server URL (e.g. https://chat.example.com): ")
			line, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("could not read URL: %w", err)
			}
			url = strings.TrimSpace(line)
		}
		if url == "" {
			return fmt.Errorf("server URL is required")
		}

		if token == "" {
			fmt.Print("Personal Access Token: ")
			b, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				return fmt.Errorf("could not read token: %w", err)
			}
			token = strings.TrimSpace(string(b))
		}
		if token == "" {
			return fmt.Errorf("token is required")
		}

		if team == "" {
			fmt.Print("Team name (slug, optional, press enter to skip): ")
			line, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("could not read team: %w", err)
			}
			team = strings.TrimSpace(line)
		}

		mm, err := client.NewWithCredentials(ctx, url, token, team, "login")
		if err != nil {
			return err
		}

		if err := config.Save(&config.Config{URL: url, Token: token, Team: team}); err != nil {
			return err
		}
		path, _ := config.Path()
		fmt.Printf("✓ Logged in as @%s at %s", mm.Username, mm.URL)
		if mm.TeamName != "" {
			fmt.Printf(" (team: %s)", mm.TeamName)
		}
		fmt.Printf("\nConfig saved to %s\n", path)
		return nil
	},
}

func init() {
	loginCmd.Flags().StringVar(&loginURL, "url", "", "Server URL")
	loginCmd.Flags().StringVar(&loginToken, "token", "", "Personal Access Token (will be prompted if omitted)")
	loginCmd.Flags().StringVar(&loginTeam, "team", "", "Team name slug (optional)")
	rootCmd.AddCommand(loginCmd)
}
