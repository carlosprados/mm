package mcp

import (
	"context"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/carlosprados/mm/internal/client"
)

// The unread digest is exposed three ways, like reading a channel: a tool
// (parameters), a resource (pull by URI) and the catch_up prompt (framing).
// All of them go through client.Unread, the same reader as `mm unread`.

type listUnreadIn struct {
	Limit        int  `json:"limit,omitempty" jsonschema:"max unread messages returned per channel (default 20)"`
	MentionsOnly bool `json:"mentions_only,omitempty" jsonschema:"only channels and DMs where the user was mentioned"`
	CountsOnly   bool `json:"counts_only,omitempty" jsonschema:"return the per-channel counts without the messages"`
}

type unreadInfo struct {
	// Channel is the channel slug (pass it as read_channel.channel) or
	// "@username" for a DM (pass it without the @ as read_channel.user).
	Channel   string        `json:"channel"`
	Type      string        `json:"type"`
	Mentions  int           `json:"mentions"`
	Unread    int           `json:"unread"`
	Truncated bool          `json:"truncated,omitempty"`
	Messages  []messageInfo `json:"messages,omitempty"`
}

type listUnreadOut struct {
	Channels []unreadInfo `json:"channels"`
}

func (s *Server) registerUnreadTool() {
	mcpsdk.AddTool(s.srv,
		&mcpsdk.Tool{
			Name:        "list_unread",
			Description: "What did I miss: every channel and DM with unread activity, those mentioning the user first, each with its unread messages (oldest to newest, with post_id). truncated means there are more unread messages than returned. Read-only: nothing is marked as read.",
		},
		func(ctx context.Context, _ *mcpsdk.CallToolRequest, in listUnreadIn) (*mcpsdk.CallToolResult, listUnreadOut, error) {
			out, err := s.fetchUnread(ctx, client.UnreadOptions{
				Limit:        in.Limit,
				MentionsOnly: in.MentionsOnly,
				CountsOnly:   in.CountsOnly,
			})
			return nil, out, err
		},
	)
}

func (s *Server) readUnreadResource(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
	out, err := s.fetchUnread(ctx, client.UnreadOptions{})
	if err != nil {
		return nil, err
	}
	return jsonResource(req.Params.URI, out)
}

func (s *Server) catchUpPrompt(ctx context.Context, req *mcpsdk.GetPromptRequest) (*mcpsdk.GetPromptResult, error) {
	out, err := s.fetchUnread(ctx, client.UnreadOptions{
		Limit:        argInt(req.Params.Arguments, "limit", 30),
		MentionsOnly: req.Params.Arguments["mentions_only"] == "true",
	})
	if err != nil {
		return nil, err
	}
	return promptResult("Catch up on everything unread.", catchUpText(s.mm.Username, out.Channels)), nil
}

func catchUpText(username string, chans []unreadInfo) string {
	var body strings.Builder
	if len(chans) == 0 {
		fmt.Fprintf(&body, "@%s has nothing unread. Say so in one line.\n", username)
		return body.String()
	}
	fmt.Fprintf(&body, "You are briefing @%s on what they missed. Below is everything unread, ", username)
	body.WriteString("channels that mention them first. Produce a catch-up that leads with what needs their action: ")
	body.WriteString("direct questions, requests and mentions, each with who asked and where. ")
	body.WriteString("Then 1-3 bullets per remaining channel with decisions and news; skip pure chatter. ")
	body.WriteString("Answer in the language the messages are written in.\n\n")
	for _, c := range chans {
		fmt.Fprintf(&body, "## %s (%s) — %d unread, %d mention(s)\n", c.Channel, c.Type, c.Unread, c.Mentions)
		if c.Truncated {
			fmt.Fprintf(&body, "(only the latest %d shown)\n", len(c.Messages))
		}
		for _, m := range c.Messages {
			fmt.Fprintf(&body, "[%s] %s: %s\n", m.Time, m.From, m.Text)
		}
		body.WriteString("\n")
	}
	return body.String()
}

func (s *Server) fetchUnread(ctx context.Context, opts client.UnreadOptions) (listUnreadOut, error) {
	unread, err := s.mm.Unread(ctx, opts)
	if err != nil {
		return listUnreadOut{}, err
	}
	out := listUnreadOut{Channels: make([]unreadInfo, 0, len(unread))}
	for _, u := range unread {
		info := unreadInfo{
			Channel:   u.Name,
			Type:      client.ChannelTypeLabel(u.Type),
			Mentions:  u.Mentions,
			Unread:    u.Count,
			Truncated: len(u.Messages) > 0 && u.Truncated(),
		}
		for _, m := range u.Messages {
			info.Messages = append(info.Messages, toMessageInfo(m))
		}
		out.Channels = append(out.Channels, info)
	}
	return out, nil
}
