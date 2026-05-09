// Copyright 2026 Todd Dailey. Licensed under Apache-2.0. See LICENSE.

// Package config persists the harvested Peloton bearer token to disk so
// subsequent commands don't need to re-spawn Chrome each call. Tokens last
// ~1 hour Peloton-side; we don't try to be smarter than that, just record
// when we wrote them and let callers decide whether to re-auth.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// Config is the on-disk shape — kept minimal on purpose. Token + acquisition
// time, plus the user_id we discover from /api/me at login (every workout
// endpoint is scoped to user_id, so harvesting it once saves every command
// from a redundant /api/me round trip).
type Config struct {
	Path        string    `toml:"-"`
	Token       string    `toml:"token"`
	UserID      string    `toml:"user_id"`
	Username    string    `toml:"username"`
	HarvestedAt time.Time `toml:"harvested_at"`
}

// DefaultPath returns ~/.config/peloton-pp-cli/config.toml. Mirrors the
// other browser-auth CLIs in the catalog (dominos uses the same shape).
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating home dir: %w", err)
	}
	return filepath.Join(home, ".config", "peloton-pp-cli", "config.toml"), nil
}

// Load reads config from `path`; if path is empty, uses DefaultPath. Missing
// files are not an error — callers get a zero-valued Config and can decide
// whether to require auth.
func Load(path string) (*Config, error) {
	if path == "" {
		p, err := DefaultPath()
		if err != nil {
			return nil, err
		}
		path = p
	}
	cfg := &Config{Path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	cfg.Path = path
	return cfg, nil
}

// Save writes the config back to disk with 0600 perms. Creates the parent dir
// if missing.
func (c *Config) Save() error {
	if err := os.MkdirAll(filepath.Dir(c.Path), 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.WriteFile(c.Path, data, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", c.Path, err)
	}
	return nil
}

// AuthHeader returns the Authorization header value for HTTP calls, or ""
// if no token is loaded. Callers should treat "" as "must run auth login".
func (c *Config) AuthHeader() string {
	if c.Token == "" {
		return ""
	}
	return "Bearer " + c.Token
}

// Stale reports whether the saved token is older than `ttl` — a hint, not a
// guarantee, since Peloton can revoke server-side at any time.
func (c *Config) Stale(ttl time.Duration) bool {
	if c.HarvestedAt.IsZero() {
		return true
	}
	return time.Since(c.HarvestedAt) > ttl
}
