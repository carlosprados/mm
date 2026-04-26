package client

import (
	"context"
	"fmt"
	"os"

	"github.com/mattermost/mattermost/server/public/model"
)

type MM struct {
	Client *model.Client4
	UserID string
	TeamID string
}

func New(ctx context.Context) (*MM, error) {
	serverURL := os.Getenv("MM_URL")
	token := os.Getenv("MM_TOKEN")
	teamName := os.Getenv("MM_TEAM")

	if serverURL == "" || token == "" {
		return nil, fmt.Errorf("MM_URL and MM_TOKEN env vars are required")
	}

	c := model.NewAPIv4Client(serverURL)
	c.SetToken(token)

	user, _, err := c.GetMe(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("auth failed: %w", err)
	}

	mm := &MM{Client: c, UserID: user.Id}

	if teamName != "" {
		team, _, err := c.GetTeamByName(ctx, teamName, "")
		if err != nil {
			return nil, fmt.Errorf("team not found: %w", err)
		}
		mm.TeamID = team.Id
	}

	return mm, nil
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
