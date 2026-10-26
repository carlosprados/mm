package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/carlosprados/mm/internal/client"
)

type downloadFileIn struct {
	FileID  string `json:"file_id,omitempty" jsonschema:"download this one attachment; file IDs come in read_channel's files"`
	PostID  string `json:"post_id,omitempty" jsonschema:"download every attachment of this post; accepts a post ID or a permalink"`
	Channel string `json:"channel,omitempty" jsonschema:"download the attachments of the most recent message with files in this channel"`
	User    string `json:"user,omitempty" jsonschema:"download the attachments of the most recent message with files in the DM with this username or alias"`
	Dir     string `json:"dir,omitempty" jsonschema:"directory to save into, on the host running this MCP server (default: the desktop download directory)"`
}

type savedFile struct {
	FileID string `json:"file_id"`
	Name   string `json:"name"`
	Path   string `json:"path"`
	Size   int64  `json:"size"`
}

type downloadFileOut struct {
	Files []savedFile `json:"files"`
}

func (s *Server) registerDownloadTool() {
	mcpsdk.AddTool(s.srv,
		&mcpsdk.Tool{
			Name:        "download_file",
			Description: "Download message attachments to the filesystem of the host running this MCP server and return where they were saved. Give exactly one of file_id, post_id, or channel/user (the latest message with attachments there). Existing files are never overwritten; a taken name gets a \" (1)\" suffix. Side effect: writes files to disk.",
		},
		func(ctx context.Context, _ *mcpsdk.CallToolRequest, in downloadFileIn) (*mcpsdk.CallToolResult, downloadFileOut, error) {
			files, err := s.resolveDownload(ctx, in)
			if err != nil {
				return nil, downloadFileOut{}, err
			}
			dir := in.Dir
			if dir == "" {
				dir = client.DefaultDownloadDir()
			}
			out := downloadFileOut{Files: make([]savedFile, 0, len(files))}
			for _, f := range files {
				path, err := s.mm.DownloadFile(ctx, f, dir)
				if err != nil {
					return nil, out, err
				}
				out.Files = append(out.Files, savedFile{FileID: f.ID, Name: f.Name, Path: path, Size: f.Size})
			}
			return nil, out, nil
		},
	)
}

func (s *Server) resolveDownload(ctx context.Context, in downloadFileIn) ([]client.Attachment, error) {
	given := 0
	for _, v := range []string{in.FileID, in.PostID, in.Channel + in.User} {
		if v != "" {
			given++
		}
	}
	if given != 1 {
		return nil, fmt.Errorf("give exactly one of file_id, post_id, or channel/user")
	}
	switch {
	case in.FileID != "":
		return s.mm.ResolveAttachments(ctx, in.FileID)
	case in.PostID != "":
		return s.mm.ResolveAttachments(ctx, in.PostID)
	default:
		return s.mm.LastAttachments(ctx, client.Target{Channel: in.Channel, User: in.User})
	}
}
