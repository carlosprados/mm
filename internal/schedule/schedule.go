// Package schedule persists messages to be delivered later. Because this
// Mattermost server has no scheduled-posts license, delivery is done by mm
// itself (the TUI's delivery loop), not the server — so messages are only sent
// while `mm tui` is running. The store is the shared source of truth for the
// CLI, the TUI and the MCP server.
package schedule

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Item is one pending message.
type Item struct {
	ID        string    `json:"id"`
	ChannelID string    `json:"channel_id"`
	Label     string    `json:"label"` // human target, e.g. "dev-backend" or "@luis"
	Message   string    `json:"message"`
	At        time.Time `json:"at"`
}

// Store is the on-disk set of pending scheduled messages.
type Store struct {
	Items []Item `json:"items"`
}

// Path honors $XDG_CONFIG_HOME, falling back to $HOME/.config/mm/scheduled.json.
func Path() (string, error) {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "mm", "scheduled.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "mm", "scheduled.json"), nil
}

// Load returns the store, or an empty (non-nil) store if no file exists.
func Load() (*Store, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return &Store{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not read scheduled %s: %w", p, err)
	}
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("could not parse scheduled %s: %w", p, err)
	}
	return &s, nil
}

// Save writes the store with mode 0600 (message bodies may be private).
func (s *Store) Save() error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return fmt.Errorf("could not create config dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("could not encode scheduled: %w", err)
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return fmt.Errorf("could not write scheduled %s: %w", p, err)
	}
	return nil
}

// Add appends a pending message. channelID is the resolved delivery target;
// label is the human-friendly target shown in listings.
func (s *Store) Add(channelID, label, message string, at time.Time) (Item, error) {
	if channelID == "" {
		return Item{}, fmt.Errorf("channel is required")
	}
	if strings.TrimSpace(message) == "" {
		return Item{}, fmt.Errorf("message is required")
	}
	if !at.After(time.Now()) {
		return Item{}, fmt.Errorf("scheduled time must be in the future")
	}
	it := Item{ID: newID(), ChannelID: channelID, Label: label, Message: message, At: at}
	s.Items = append(s.Items, it)
	return it, nil
}

// Remove deletes an item by ID; errors if not found.
func (s *Store) Remove(id string) error {
	for i, it := range s.Items {
		if it.ID == id {
			s.Items = append(s.Items[:i], s.Items[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("scheduled message %q not found", id)
}

// Due returns items whose delivery time has arrived (At <= now).
func (s *Store) Due(now time.Time) []Item {
	var due []Item
	for _, it := range s.Items {
		if !it.At.After(now) {
			due = append(due, it)
		}
	}
	return due
}

// Sorted returns the items ordered by delivery time.
func (s *Store) Sorted() []Item {
	out := make([]Item, len(s.Items))
	copy(out, s.Items)
	sort.Slice(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out
}

func newID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
