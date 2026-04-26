// Package mcp exposes the same functionality as the CLI through the Model
// Context Protocol. The CLI and the MCP server are kept at functional parity:
// every CLI command has a tool/resource/prompt counterpart and vice versa.
package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/carlosprados/mm/internal/client"
)

// Server wraps an authenticated Mattermost client and the MCP SDK server.
type Server struct {
	mm  *client.MM
	srv *mcpsdk.Server
}

// New builds an MCP server bound to an authenticated MM client. Tools,
// resources and prompts are registered on construction.
func New(mm *client.MM, version string) *Server {
	srv := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "mm",
		Title:   "Mattermost CLI",
		Version: version,
	}, nil)

	s := &Server{mm: mm, srv: srv}
	s.registerTools()
	s.registerResources()
	s.registerPrompts()
	return s
}

// Run blocks serving the MCP protocol over stdio (the canonical transport for
// local Claude Desktop / Claude Code integrations).
func (s *Server) Run(ctx context.Context) error {
	return s.srv.Run(ctx, &mcpsdk.StdioTransport{})
}
