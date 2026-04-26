// Package config persists CLI session credentials so users don't need to
// export environment variables on every shell.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	URL   string `json:"url"`
	Token string `json:"token"`
	Team  string `json:"team"`
}

// Path returns the absolute path to the config file. It honors $XDG_CONFIG_HOME
// when set and falls back to $HOME/.config/mm/config.json.
func Path() (string, error) {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "mm", "config.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "mm", "config.json"), nil
}

// Load returns the saved config, or (nil, nil) if no config file exists yet.
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not read config %s: %w", p, err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("could not parse config %s: %w", p, err)
	}
	return &c, nil
}

// Save writes the config with mode 0600 (secrets), creating parent dirs.
func Save(c *Config) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return fmt.Errorf("could not create config dir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("could not encode config: %w", err)
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return fmt.Errorf("could not write config %s: %w", p, err)
	}
	return nil
}

// Delete removes the config file. Returns nil if it doesn't exist.
func Delete() error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("could not delete config %s: %w", p, err)
	}
	return nil
}
