// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pelletier/go-toml/v2"
)

// Config is the on-disk + env-derived configuration that every command
// consumes. Gorgias uses HTTP Basic auth — the account email is the
// username, the API key is the password — so only those two values plus
// the tenant base URL are auth-relevant.
type Config struct {
	BaseURL         string            `toml:"base_url"`
	AuthHeaderVal   string            `toml:"auth_header"`
	Headers         map[string]string `toml:"headers,omitempty"`
	AuthSource      string            `toml:"-"`
	Path            string            `toml:"-"`
	GorgiasUsername string            `toml:"username"`
	GorgiasApiKey   string            `toml:"api_key"`
	// PathExplicit is true when the path came from --config or GORGIAS_CONFIG
	// rather than the XDG default. PathExists is true when the file at Path
	// is readable. Doctor uses (PathExplicit && !PathExists) to surface a
	// warning — "you pointed at a file that doesn't exist, did you typo?".
	// At the default XDG path, a missing file is normal (env-var-driven auth).
	PathExplicit bool `toml:"-"`
	PathExists   bool `toml:"-"`
}

func Load(configPath string) (*Config, error) {
	// No default BaseURL — Gorgias is multi-tenant, every account has its own
	// host (`<tenant>.gorgias.com/api`). Defaulting to `app.gorgias.com/api`
	// hid configuration errors behind 404s; better to fail clearly when the
	// env var is missing than to silently hit a landing page.
	cfg := &Config{}

	// Resolve config path. An explicit override (--config flag or
	// GORGIAS_CONFIG env) is recorded so doctor can warn when the user
	// pointed at a missing file. The XDG default path missing is normal —
	// env-var-driven auth works fine without a config file.
	path := configPath
	explicit := false
	if path != "" {
		explicit = true
	}
	if path == "" {
		if v := os.Getenv("GORGIAS_CONFIG"); v != "" {
			path = v
			explicit = true
		}
	}
	if path == "" {
		path = filepath.Join(xdgConfigHome(), "gorgias-pp-cli", "config.toml")
	}
	cfg.Path = path
	cfg.PathExplicit = explicit

	// Try to load config file
	data, err := os.ReadFile(path)
	if err == nil {
		cfg.PathExists = true
		if err := toml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing config %s: %w", path, err)
		}
	}

	// Env var overrides
	if v := os.Getenv("GORGIAS_USERNAME"); v != "" {
		cfg.GorgiasUsername = v
		cfg.AuthSource = "env:GORGIAS_USERNAME"
	}
	if v := os.Getenv("GORGIAS_API_KEY"); v != "" {
		cfg.GorgiasApiKey = v
		cfg.AuthSource = "env:GORGIAS_API_KEY"
	}

	// Label config-file-derived credentials so doctor can distinguish
	// "credentials persisted on disk" from "no credentials at all" — without
	// this, users who saved via set-token without an env var see a blank
	// auth_source and can't tell whether their config is being picked up.
	// The label is the literal "config" rather than "config:<path>"; the
	// config file path is exposed separately as report["config_path"], and
	// embedding it in auth_source leaks the user's home directory through
	// doctor's JSON envelope.
	if cfg.AuthSource == "" && cfg.AuthHeaderVal != "" {
		cfg.AuthSource = "config"
	}
	if cfg.AuthSource == "" && cfg.GorgiasUsername != "" {
		cfg.AuthSource = "config"
	}
	if cfg.AuthSource == "" && cfg.GorgiasApiKey != "" {
		cfg.AuthSource = "config"
	}

	// Base URL override (used by printing-press verify to point at mock/test servers)
	if v := os.Getenv("GORGIAS_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	return cfg, nil
}

func (c *Config) AuthHeader() string {
	if c.AuthHeaderVal != "" {
		return c.AuthHeaderVal
	}
	if c.GorgiasUsername == "" || c.GorgiasApiKey == "" {
		return ""
	}
	credentials := c.GorgiasUsername + ":" + c.GorgiasApiKey
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(credentials))
}

// SaveCredentials persists Gorgias Basic-auth credentials: the account
// email as the username and the API key as the password. Both are
// required by `AuthHeader()`, which constructs `Basic base64(email:key)`.
// Clears any stored AuthHeaderVal first so it doesn't shadow the new values.
func (c *Config) SaveCredentials(email, apiKey string) error {
	c.AuthHeaderVal = ""
	c.GorgiasUsername = email
	c.GorgiasApiKey = apiKey
	return c.save()
}

// ClearTokens removes persisted credentials. Env-var GORGIAS_USERNAME /
// GORGIAS_API_KEY can still authenticate after this — those are the
// caller's responsibility to unset.
func (c *Config) ClearTokens() error {
	c.AuthHeaderVal = ""
	c.GorgiasUsername = ""
	c.GorgiasApiKey = ""
	return c.save()
}

func (c *Config) save() error {
	dir := filepath.Dir(c.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return os.WriteFile(c.Path, data, 0o600)
}

// xdgConfigHome returns $XDG_CONFIG_HOME, falling back to ~/.config on
// Unix-likes and %APPDATA% on Windows (via os.UserConfigDir). The CLI
// package has its own copy of this helper for its state/data paths;
// this one is local to keep config a leaf package with no internal/cli
// dependency.
func xdgConfigHome() string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v
	}
	if runtime.GOOS == "windows" {
		if dir, err := os.UserConfigDir(); err == nil {
			return dir
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}
