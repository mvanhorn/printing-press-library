package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	ProKey          string `json:"pro_key,omitempty"`
	StaleOverview   string `json:"stale_overview,omitempty"`
	StaleHistorical string `json:"stale_historical,omitempty"`
}

const (
	defaultStaleOverview   = "1h"
	defaultStaleHistorical = "24h"
)

func Dir() string {
	if v := os.Getenv("DEFILLAMA_PP_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".defillama-pp"
	}
	return filepath.Join(home, ".defillama-pp")
}

func DBPath() string  { return filepath.Join(Dir(), "defillama.db") }
func configPath() string { return filepath.Join(Dir(), "config.json") }

func ensureDir() error {
	return os.MkdirAll(Dir(), 0o755)
}

func Load() (*Config, error) {
	if err := ensureDir(); err != nil {
		return nil, err
	}
	c := &Config{StaleOverview: defaultStaleOverview, StaleHistorical: defaultStaleHistorical}
	b, err := os.ReadFile(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(b, c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if c.StaleOverview == "" {
		c.StaleOverview = defaultStaleOverview
	}
	if c.StaleHistorical == "" {
		c.StaleHistorical = defaultStaleHistorical
	}
	return c, nil
}

func Save(c *Config) error {
	if err := ensureDir(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), b, 0o600)
}

// ResolvedProKey returns env var first, then config.
func (c *Config) ResolvedProKey() string {
	if v := os.Getenv("DEFILLAMA_PRO_KEY"); v != "" {
		return v
	}
	return c.ProKey
}

func (c *Config) StaleOverviewDur() time.Duration {
	d, err := time.ParseDuration(c.StaleOverview)
	if err != nil {
		d, _ = time.ParseDuration(defaultStaleOverview)
	}
	return d
}

func (c *Config) StaleHistoricalDur() time.Duration {
	d, err := time.ParseDuration(c.StaleHistorical)
	if err != nil {
		d, _ = time.ParseDuration(defaultStaleHistorical)
	}
	return d
}
