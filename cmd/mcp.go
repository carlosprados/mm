package cmd

import (
	"github.com/spf13/cobra"

	"github.com/carlosprados/mm/internal/client"
	mcpserver "github.com/carlosprados/mm/internal/mcp"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run the MCP server over stdio",
	Long: `Starts mm as a Model Context Protocol server on stdio. Use this in your
MCP client (Claude Desktop, Claude Code, MCP Inspector) to expose the same
functionality as the CLI to AI assistants.

Tools, resources and prompts are kept at functional parity with the CLI.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		mm, err := client.New(ctx)
		if err != nil {
			return err
		}
		return mcpserver.New(mm, version).Run(ctx)
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
