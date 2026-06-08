// Package alias maps short, human-friendly handles to canonical Mattermost
// usernames so users can DM "luisete" instead of "luisdavid.francisco".
//
// Aliases are a cross-cutting concept: the same store is consumed by the CLI
// (`mm send -u luis`, `mm alias`), the TUI and the MCP server, keeping the
// three surfaces at parity.
package alias

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Store holds the alias→username mapping. Keys (aliases) are normalized to
// lowercase; values are canonical usernames as stored in Mattermost.
type Store struct {
	Aliases map[string]string `json:"aliases"`
}

// Path returns the absolute path to the aliases file. It honors
// $XDG_CONFIG_HOME and falls back to $HOME/.config/mm/aliases.json.
//
// Unlike config.json, this file holds no secrets, so it is written 0644.
func Path() (string, error) {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "mm", "aliases.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "mm", "aliases.json"), nil
}

// Load returns the saved store, or an empty (non-nil) store if no file exists.
func Load() (*Store, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return &Store{Aliases: map[string]string{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not read aliases %s: %w", p, err)
	}
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("could not parse aliases %s: %w", p, err)
	}
	if s.Aliases == nil {
		s.Aliases = map[string]string{}
	}
	return &s, nil
}

// Save writes the store (0644), creating parent dirs.
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
		return fmt.Errorf("could not encode aliases: %w", err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return fmt.Errorf("could not write aliases %s: %w", p, err)
	}
	return nil
}

// Resolve maps an input to a canonical username. If input is a known alias it
// returns the mapped username; otherwise it returns input unchanged, so callers
// can pass user input through unconditionally with zero regression.
func (s *Store) Resolve(input string) string {
	if s == nil {
		return input
	}
	if u, ok := s.Aliases[normalize(input)]; ok {
		return u
	}
	return input
}

// AliasesFor returns the aliases pointing at a given username, sorted.
func (s *Store) AliasesFor(username string) []string {
	var out []string
	for a, u := range s.Aliases {
		if u == username {
			out = append(out, a)
		}
	}
	sort.Strings(out)
	return out
}

// Add registers (or overwrites) an alias → username mapping.
func (s *Store) Add(aliasName, username string) error {
	a := normalize(aliasName)
	if a == "" {
		return fmt.Errorf("alias cannot be empty")
	}
	if strings.ContainsAny(a, " \t") {
		return fmt.Errorf("alias cannot contain whitespace")
	}
	u := strings.TrimSpace(strings.TrimPrefix(username, "@"))
	if u == "" {
		return fmt.Errorf("username cannot be empty")
	}
	if s.Aliases == nil {
		s.Aliases = map[string]string{}
	}
	s.Aliases[a] = u
	return nil
}

// Remove deletes an alias. It is an error if the alias does not exist.
func (s *Store) Remove(aliasName string) error {
	a := normalize(aliasName)
	if _, ok := s.Aliases[a]; !ok {
		return fmt.Errorf("alias %q not found", aliasName)
	}
	delete(s.Aliases, a)
	return nil
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
