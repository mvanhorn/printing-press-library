// Copyright 2026 salmonumbrella. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// PATCH(env-trim): collapsed five alias env vars + five Config fields to a
// single canonical credential (LUNCHMONEY_API_KEY -> AccessToken). Dropped
// LUNCH_MONEY_CONFIG (covered by --config flag) and LUNCH_MONEY_BASE_URL
// (covered by config file base_url field). See .printing-press-patches.json.
type Config struct {
	BaseURL       string            `toml:"base_url"`
	AuthHeaderVal string            `toml:"auth_header"`
	Headers       map[string]string `toml:"headers,omitempty"`
	AuthSource    string            `toml:"-"`
	AccessToken   string            `toml:"access_token"`
	RefreshToken  string            `toml:"refresh_token"`
	TokenExpiry   time.Time         `toml:"token_expiry"`
	ClientID      string            `toml:"client_id"`
	ClientSecret  string            `toml:"client_secret"`
	Path          string            `toml:"-"`
}

func Load(configPath string) (*Config, error) {
	cfg := &Config{
		BaseURL: "https://api.lunchmoney.dev/v2",
	}

	path := configPath
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".config", "lunch-money-pp-cli", "config.toml")
	}
	cfg.Path = path

	data, err := os.ReadFile(path)
	if err == nil {
		if err := toml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing config %s: %w", path, err)
		}
	}

	// PATCH(env-trim): single canonical env var; the value IS the bearer token,
	// AuthHeader() prepends "Bearer ".
	if v := os.Getenv("LUNCHMONEY_API_KEY"); v != "" {
		cfg.AccessToken = v
		cfg.AuthSource = "env:LUNCHMONEY_API_KEY"
	}

	if cfg.AuthSource == "" && (cfg.AuthHeaderVal != "" || cfg.AccessToken != "") {
		cfg.AuthSource = "config"
	}

	return cfg, nil
}

func (c *Config) AuthHeader() string {
	if c.AuthHeaderVal != "" {
		return c.AuthHeaderVal
	}
	if c.AccessToken != "" {
		return "Bearer " + c.AccessToken
	}
	return ""
}

func (c *Config) SaveTokens(clientID, clientSecret, accessToken, refreshToken string, expiry time.Time) error {
	c.ClientID = clientID
	c.ClientSecret = clientSecret
	c.AccessToken = accessToken
	c.RefreshToken = refreshToken
	c.TokenExpiry = expiry
	return c.save()
}

func (c *Config) ClearTokens() error {
	c.AccessToken = ""
	c.RefreshToken = ""
	c.TokenExpiry = time.Time{}
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
