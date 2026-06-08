package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/carlosprados/mm/internal/alias"
)

// Input/output structs are exported so the MCP SDK can derive a JSON schema
// from the struct tags.

type listChannelsIn struct{}

type channelInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type listChannelsOut struct {
	Channels []channelInfo `json:"channels"`
}

type listUsersIn struct{}

type userInfo struct {
	Username string `json:"username"`
	Name     string `json:"name,omitempty"`
}

type listUsersOut struct {
	Users []userInfo `json:"users"`
}

type readChannelIn struct {
	Channel string `json:"channel" jsonschema:"name of the channel (slug, e.g. town-square)"`
	Limit   int    `json:"limit,omitempty" jsonschema:"max messages to fetch (default 20)"`
}

type messageInfo struct {
	Time string `json:"time"`
	From string `json:"from"`
	Text string `json:"text"`
}

type readChannelOut struct {
	Channel  string        `json:"channel"`
	Messages []messageInfo `json:"messages"`
}

type sendMessageIn struct {
	Channel string `json:"channel,omitempty" jsonschema:"target channel name (mutually exclusive with user)"`
	User    string `json:"user,omitempty" jsonschema:"target username for a DM (mutually exclusive with channel)"`
	Message string `json:"message" jsonschema:"message body"`
}

type sendMessageOut struct {
	OK        bool   `json:"ok"`
	ChannelID string `json:"channel_id"`
	PostID    string `json:"post_id"`
}

type manageAliasIn struct {
	Action   string `json:"action" jsonschema:"one of: list, add, remove"`
	Alias    string `json:"alias,omitempty" jsonschema:"the short handle (required for add/remove)"`
	Username string `json:"username,omitempty" jsonschema:"canonical username the alias maps to (required for add)"`
}

type aliasEntry struct {
	Alias    string `json:"alias"`
	Username string `json:"username"`
}

type manageAliasOut struct {
	OK      bool         `json:"ok"`
	Aliases []aliasEntry `json:"aliases"`
}

type whoamiIn struct{}

type whoamiOut struct {
	Username string `json:"username"`
	URL      string `json:"url"`
	Team     string `json:"team,omitempty"`
	Source   string `json:"source"`
}

func (s *Server) registerTools() {
	mcpsdk.AddTool(s.srv,
		&mcpsdk.Tool{
			Name:        "list_channels",
			Description: "List the channels the authenticated user has joined in the configured team.",
		},
		func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ listChannelsIn) (*mcpsdk.CallToolResult, listChannelsOut, error) {
			channels, _, err := s.mm.Client.GetChannelsForTeamForUser(ctx, s.mm.TeamID, s.mm.UserID, false, "")
			if err != nil {
				return nil, listChannelsOut{}, fmt.Errorf("could not fetch channels: %w", err)
			}
			out := listChannelsOut{Channels: make([]channelInfo, 0, len(channels))}
			for _, ch := range channels {
				out.Channels = append(out.Channels, channelInfo{Name: ch.Name, Type: channelTypeLabel(ch.Type)})
			}
			return nil, out, nil
		},
	)

	mcpsdk.AddTool(s.srv,
		&mcpsdk.Tool{
			Name:        "list_users",
			Description: "List members of the configured team with their @username handle.",
		},
		func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ listUsersIn) (*mcpsdk.CallToolResult, listUsersOut, error) {
			users, _, err := s.mm.Client.GetUsersInTeam(ctx, s.mm.TeamID, 0, 200, "")
			if err != nil {
				return nil, listUsersOut{}, fmt.Errorf("could not fetch users: %w", err)
			}
			out := listUsersOut{Users: make([]userInfo, 0, len(users))}
			for _, u := range users {
				out.Users = append(out.Users, userInfo{
					Username: u.Username,
					Name:     strings.TrimSpace(u.FirstName + " " + u.LastName),
				})
			}
			return nil, out, nil
		},
	)

	mcpsdk.AddTool(s.srv,
		&mcpsdk.Tool{
			Name:        "read_channel",
			Description: "Read the most recent messages from a channel, oldest to newest, with usernames resolved.",
		},
		func(ctx context.Context, _ *mcpsdk.CallToolRequest, in readChannelIn) (*mcpsdk.CallToolResult, readChannelOut, error) {
			if in.Channel == "" {
				return nil, readChannelOut{}, fmt.Errorf("channel is required")
			}
			limit := in.Limit
			if limit <= 0 {
				limit = 20
			}
			msgs, err := s.fetchMessages(ctx, in.Channel, limit)
			if err != nil {
				return nil, readChannelOut{}, err
			}
			return nil, readChannelOut{Channel: in.Channel, Messages: msgs}, nil
		},
	)

	mcpsdk.AddTool(s.srv,
		&mcpsdk.Tool{
			Name:        "send_message",
			Description: "Send a message to a channel or as a direct message to a user. The user field accepts either a canonical username or a configured alias. Side effect: creates a post. Provide either channel or user, never both.",
		},
		func(ctx context.Context, _ *mcpsdk.CallToolRequest, in sendMessageIn) (*mcpsdk.CallToolResult, sendMessageOut, error) {
			if in.Message == "" {
				return nil, sendMessageOut{}, fmt.Errorf("message is required")
			}
			if (in.Channel == "") == (in.User == "") {
				return nil, sendMessageOut{}, fmt.Errorf("provide exactly one of channel or user")
			}

			var channelID string
			if in.User != "" {
				store, err := alias.Load()
				if err != nil {
					return nil, sendMessageOut{}, err
				}
				user, _, err := s.mm.Client.GetUserByUsername(ctx, store.Resolve(in.User), "")
				if err != nil {
					return nil, sendMessageOut{}, fmt.Errorf("user not found: %w", err)
				}
				channelID, err = s.mm.GetDirectChannelWith(ctx, user.Id)
				if err != nil {
					return nil, sendMessageOut{}, err
				}
			} else {
				ch, _, err := s.mm.Client.GetChannelByName(ctx, in.Channel, s.mm.TeamID, "")
				if err != nil {
					return nil, sendMessageOut{}, fmt.Errorf("channel not found: %w", err)
				}
				channelID = ch.Id
			}

			post, _, err := s.mm.Client.CreatePost(ctx, &model.Post{ChannelId: channelID, Message: in.Message})
			if err != nil {
				return nil, sendMessageOut{}, fmt.Errorf("could not send message: %w", err)
			}
			return nil, sendMessageOut{OK: true, ChannelID: channelID, PostID: post.Id}, nil
		},
	)

	mcpsdk.AddTool(s.srv,
		&mcpsdk.Tool{
			Name:        "manage_alias",
			Description: "List, add or remove username aliases. Side effect (add/remove): persists the aliases file. Aliases let send_message target a user by a short handle.",
		},
		func(ctx context.Context, _ *mcpsdk.CallToolRequest, in manageAliasIn) (*mcpsdk.CallToolResult, manageAliasOut, error) {
			store, err := alias.Load()
			if err != nil {
				return nil, manageAliasOut{}, err
			}
			switch in.Action {
			case "add":
				if err := store.Add(in.Alias, in.Username); err != nil {
					return nil, manageAliasOut{}, err
				}
				if err := store.Save(); err != nil {
					return nil, manageAliasOut{}, err
				}
			case "remove":
				if err := store.Remove(in.Alias); err != nil {
					return nil, manageAliasOut{}, err
				}
				if err := store.Save(); err != nil {
					return nil, manageAliasOut{}, err
				}
			case "list", "":
				// no-op, fall through to return current state
			default:
				return nil, manageAliasOut{}, fmt.Errorf("unknown action %q (use list, add or remove)", in.Action)
			}

			out := manageAliasOut{OK: true, Aliases: make([]aliasEntry, 0, len(store.Aliases))}
			names := make([]string, 0, len(store.Aliases))
			for a := range store.Aliases {
				names = append(names, a)
			}
			sort.Strings(names)
			for _, a := range names {
				out.Aliases = append(out.Aliases, aliasEntry{Alias: a, Username: store.Aliases[a]})
			}
			return nil, out, nil
		},
	)

	mcpsdk.AddTool(s.srv,
		&mcpsdk.Tool{
			Name:        "whoami",
			Description: "Return the active session: username, server URL, team and credential source.",
		},
		func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ whoamiIn) (*mcpsdk.CallToolResult, whoamiOut, error) {
			return nil, whoamiOut{
				Username: s.mm.Username,
				URL:      s.mm.URL,
				Team:     s.mm.TeamName,
				Source:   s.mm.Source,
			}, nil
		},
	)
}

// fetchMessages is shared by tools and prompts.
func (s *Server) fetchMessages(ctx context.Context, channelName string, limit int) ([]messageInfo, error) {
	channel, _, err := s.mm.Client.GetChannelByName(ctx, channelName, s.mm.TeamID, "")
	if err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}
	posts, _, err := s.mm.Client.GetPostsForChannel(ctx, channel.Id, 0, limit, "", false, false)
	if err != nil {
		return nil, fmt.Errorf("could not fetch posts: %w", err)
	}

	ids := make([]string, 0, len(posts.Order))
	for _, id := range posts.Order {
		ids = append(ids, posts.Posts[id].UserId)
	}
	usernames, err := s.mm.ResolveUsernames(ctx, ids)
	if err != nil {
		return nil, err
	}

	out := make([]messageInfo, 0, len(posts.Order))
	for i := len(posts.Order) - 1; i >= 0; i-- {
		p := posts.Posts[posts.Order[i]]
		out = append(out, messageInfo{
			Time: time.UnixMilli(p.CreateAt).Format(time.RFC3339),
			From: usernames[p.UserId],
			Text: p.Message,
		})
	}
	return out, nil
}

func channelTypeLabel(t model.ChannelType) string {
	switch t {
	case model.ChannelTypePrivate:
		return "private"
	case model.ChannelTypeDirect:
		return "dm"
	case model.ChannelTypeGroup:
		return "group"
	default:
		return "public"
	}
}
