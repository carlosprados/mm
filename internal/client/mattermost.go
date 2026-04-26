package client

import (
	"context"
	"fmt"
	"os"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/carlosprados/mm/internal/config"
)

type MM struct {
	Client   *model.Client4
	UserID   string
	Username string
	TeamID   string
	TeamName string
	URL      string
	// Source describes where credentials came from: "env" or "config".
	Source string
}

// New builds an authenticated MM client. Precedence:
//  1. Environment variables MM_URL/MM_TOKEN/MM_TEAM (any subset; missing fields
//     fall back to the config file).
//  2. Saved config from `mm login`.
//
// The function calls GetMe() to validate the token before returning.
func New(ctx context.Context) (*MM, error) {
	url, token, team, source, err := resolveCredentials()
	if err != nil {
		return nil, err
	}
	return NewWithCredentials(ctx, url, token, team, source)
}

// NewWithCredentials builds an authenticated client from explicit credentials.
// Used by `mm login` to validate before persisting.
func NewWithCredentials(ctx context.Context, url, token, team, source string) (*MM, error) {
	if url == "" || token == "" {
		return nil, fmt.Errorf("missing credentials: run 'mm login' or set MM_URL and MM_TOKEN")
	}

	c := model.NewAPIv4Client(url)
	c.SetToken(token)

	user, _, err := c.GetMe(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("auth failed: %w", err)
	}

	mm := &MM{
		Client:   c,
		UserID:   user.Id,
		Username: user.Username,
		URL:      url,
		Source:   source,
	}

	if team != "" {
		t, _, err := c.GetTeamByName(ctx, team, "")
		if err != nil {
			return nil, fmt.Errorf("team not found: %w", err)
		}
		mm.TeamID = t.Id
		mm.TeamName = t.Name
	}

	return mm, nil
}

func resolveCredentials() (url, token, team, source string, err error) {
	url = os.Getenv("MM_URL")
	token = os.Getenv("MM_TOKEN")
	team = os.Getenv("MM_TEAM")

	envHasAny := url != "" || token != "" || team != ""
	source = "env"

	if url == "" || token == "" {
		cfg, cerr := config.Load()
		if cerr != nil {
			return "", "", "", "", cerr
		}
		if cfg != nil {
			if url == "" {
				url = cfg.URL
			}
			if token == "" {
				token = cfg.Token
			}
			if team == "" {
				team = cfg.Team
			}
			if !envHasAny {
				source = "config"
			} else {
				source = "env+config"
			}
		}
	}

	if url == "" || token == "" {
		return "", "", "", "", fmt.Errorf("not authenticated: run 'mm login' or set MM_URL and MM_TOKEN")
	}
	return url, token, team, source, nil
}

// GetDirectChannelWith returns the DM channel ID between the current user and another user.
func (mm *MM) GetDirectChannelWith(ctx context.Context, otherUserID string) (string, error) {
	channel, _, err := mm.Client.CreateDirectChannel(ctx, mm.UserID, otherUserID)
	if err != nil {
		return "", fmt.Errorf("could not open DM: %w", err)
	}
	return channel.Id, nil
}

// ResolveUsernames fetches usernames for a list of user IDs.
// Returns a map of userID -> "@username".
func (mm *MM) ResolveUsernames(ctx context.Context, userIDs []string) (map[string]string, error) {
	seen := make(map[string]struct{})
	unique := make([]string, 0, len(userIDs))
	for _, id := range userIDs {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}

	if len(unique) == 0 {
		return map[string]string{}, nil
	}

	users, _, err := mm.Client.GetUsersByIds(ctx, unique)
	if err != nil {
		return nil, fmt.Errorf("could not resolve usernames: %w", err)
	}

	result := make(map[string]string, len(unique))
	for _, u := range users {
		result[u.Id] = "@" + u.Username
	}
	for _, id := range unique {
		if _, ok := result[id]; !ok {
			result[id] = id
		}
	}

	return result, nil
}
