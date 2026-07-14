// Package config loads ~/.config/mt5-pp-cli/config.toml.
//
// The file holds named broker connection profiles and the per-command
// guardrail thresholds enforced by the safety layer. It is created with sane
// defaults on first run if missing.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pelletier/go-toml/v2"
)

// Profile is one broker connection. Password is read from password_env at use
// time, never stored on disk.
type Profile struct {
	Account     int64  `toml:"account"`
	Server      string `toml:"server"`
	PasswordEnv string `toml:"password_env"`
	Description string `toml:"description,omitempty"`
}

// Guardrails are checked before any write command reaches the bridge.
type Guardrails struct {
	MaxVolumePerOrder float64 `toml:"max_volume_per_order"`
	MaxOpenPositions  int     `toml:"max_open_positions"`
	MaxDailyLoss      float64 `toml:"max_daily_loss"`
	KillSwitchFile    string  `toml:"kill_switch_file"`
}

// Config is the on-disk file shape.
type Config struct {
	DefaultProfile string             `toml:"default_profile,omitempty"`
	Profiles       map[string]Profile `toml:"profiles,omitempty"`
	Guardrails     Guardrails         `toml:"guardrails"`
}

// DefaultPath resolves the canonical config file location for the host.
// Override via $MT5_PP_CONFIG.
func DefaultPath() string {
	if env := os.Getenv("MT5_PP_CONFIG"); env != "" {
		return env
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "mt5-pp-cli", "config.toml")
	}
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		if app := os.Getenv("APPDATA"); app != "" {
			return filepath.Join(app, "mt5-pp-cli", "config.toml")
		}
		return filepath.Join(home, "AppData", "Roaming", "mt5-pp-cli", "config.toml")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "mt5-pp-cli", "config.toml")
	default:
		return filepath.Join(home, ".config", "mt5-pp-cli", "config.toml")
	}
}

// Load reads the config from disk, returning a zero-value Config if the file
// doesn't exist (so callers can rely on Guardrails defaults).
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultPath()
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var c Config
	if err := toml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return &c, nil
}

// WriteDefault writes a commented example config to disk. Safe to re-run —
// refuses to overwrite an existing file.
func WriteDefault(path string) error {
	if path == "" {
		path = DefaultPath()
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config already exists at %s — refusing to overwrite", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	const example = `# mt5-pp-cli config — generated default. Edit and re-run.
# See: mt5-pp-cli doctor for the resolved values.

# Default profile used when --profile is not passed.
# default_profile = "demo"

# [profiles.demo]
# account      = 12345678
# server       = "Broker-Demo"
# password_env = "MT5_DEMO_PASSWORD"
# description  = "Personal demo for testing"

# [profiles.live]
# account      = 87654321
# server       = "Broker-Live"
# password_env = "MT5_LIVE_PASSWORD"

# Guardrails enforced before any write command reaches the bridge.
# Any limit set to 0 is treated as "no limit" for that field.
[guardrails]
max_volume_per_order = 1.0      # reject order send with volume > 1.0 lots
max_open_positions   = 20       # reject if currently open >= 20
max_daily_loss       = 0        # 0 = disabled; set to e.g. 500.0 to enforce
kill_switch_file     = ""       # if this file exists, reject ALL writes
`
	return os.WriteFile(path, []byte(example), 0644)
}
