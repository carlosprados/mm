package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Resources are the "browsable" face of the same data offered by tools.
// Some MCP clients prefer pulling resources by URI over invoking tools, so
// exposing both maximizes compatibility at no extra cost.

func (s *Server) registerResources() {
	s.srv.AddResource(&mcpsdk.Resource{
		URI:         "mm://team/channels",
		Name:        "Channels",
		Description: "List of channels the authenticated user has joined, as JSON.",
		MIMEType:    "application/json",
	}, s.readChannelsResource)

	s.srv.AddResource(&mcpsdk.Resource{
		URI:         "mm://team/users",
		Name:        "Team users",
		Description: "Members of the configured Mattermost team, as JSON.",
		MIMEType:    "application/json",
	}, s.readUsersResource)

	s.srv.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		URITemplate: "mm://channel/{name}/messages{?limit}",
		Name:        "Channel messages",
		Description: "Most recent messages of a channel. Use ?limit=N to control how many (default 20).",
		MIMEType:    "application/json",
	}, s.readChannelMessagesResource)
}

func (s *Server) readChannelsResource(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
	channels, _, err := s.mm.Client.GetChannelsForTeamForUser(ctx, s.mm.TeamID, s.mm.UserID, false, "")
	if err != nil {
		return nil, fmt.Errorf("could not fetch channels: %w", err)
	}
	out := listChannelsOut{Channels: make([]channelInfo, 0, len(channels))}
	for _, ch := range channels {
		out.Channels = append(out.Channels, channelInfo{Name: ch.Name, Type: channelTypeLabel(ch.Type)})
	}
	return jsonResource(req.Params.URI, out)
}

func (s *Server) readUsersResource(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
	users, _, err := s.mm.Client.GetUsersInTeam(ctx, s.mm.TeamID, 0, 200, "")
	if err != nil {
		return nil, fmt.Errorf("could not fetch users: %w", err)
	}
	out := listUsersOut{Users: make([]userInfo, 0, len(users))}
	for _, u := range users {
		out.Users = append(out.Users, userInfo{
			Username: u.Username,
			Name:     strings.TrimSpace(u.FirstName + " " + u.LastName),
		})
	}
	return jsonResource(req.Params.URI, out)
}

func (s *Server) readChannelMessagesResource(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
	name, limit, err := parseChannelMessagesURI(req.Params.URI)
	if err != nil {
		return nil, err
	}
	msgs, err := s.fetchMessages(ctx, name, limit)
	if err != nil {
		return nil, err
	}
	return jsonResource(req.Params.URI, readChannelOut{Channel: name, Messages: msgs})
}

func parseChannelMessagesURI(raw string) (name string, limit int, err error) {
	// raw looks like mm://channel/<name>/messages?limit=N
	u, perr := url.Parse(raw)
	if perr != nil {
		return "", 0, fmt.Errorf("invalid resource URI: %w", perr)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	// With scheme mm://, host = "channel", path = "/<name>/messages" → parts = [<name>, messages]
	if u.Host != "channel" || len(parts) < 2 || parts[len(parts)-1] != "messages" {
		return "", 0, fmt.Errorf("unsupported URI %q", raw)
	}
	name = parts[len(parts)-2]
	limit = 20
	if l := u.Query().Get("limit"); l != "" {
		v, err := strconv.Atoi(l)
		if err != nil || v <= 0 {
			return "", 0, fmt.Errorf("invalid limit %q", l)
		}
		limit = v
	}
	return name, limit, nil
}

func jsonResource(uri string, payload any) (*mcpsdk.ReadResourceResult, error) {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("could not encode resource: %w", err)
	}
	return &mcpsdk.ReadResourceResult{
		Contents: []*mcpsdk.ResourceContents{{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(data),
		}},
	}, nil
}
