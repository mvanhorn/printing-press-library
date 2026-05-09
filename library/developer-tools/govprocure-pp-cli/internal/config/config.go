package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/BurntSushi/toml"
)

// Config holds all application configuration.
type Config struct {
	SAMAPIKey   string `toml:"sam_api_key"`
	DBPath      string `toml:"db_path"`
	GrantsURL   string `toml:"grants_url"`
	SAMURL      string `toml:"sam_url"`
	USASpendURL string `toml:"usaspending_url"`
	SyncDays    int    `toml:"sync_days"`
}

const (
	DefaultGrantsURL   = "https://apply07.grants.gov/grantsws/rest/opportunities/search/"
	DefaultSAMURL      = "https://api.sam.gov/opportunities/v2/search"
	DefaultUSASpendURL = "https://api.usaspending.gov/api/v2"
	DefaultSyncDays    = 90
)

// configDir returns the OS-appropriate config directory.
func configDir() (string, error) {
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("APPDATA")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, "AppData", "Roaming")
		}
	default:
		xdg := os.Getenv("XDG_CONFIG_HOME")
		if xdg != "" {
			base = xdg
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".config")
		}
	}
	return filepath.Join(base, "govprocure-pp-cli"), nil
}

// dataDir returns the OS-appropriate data directory.
func dataDir() (string, error) {
	var base string
	switch runtime.GOOS {
	case "windows":
		local := os.Getenv("LOCALAPPDATA")
		if local == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			local = filepath.Join(home, "AppData", "Local")
		}
		base = local
	default:
		xdg := os.Getenv("XDG_DATA_HOME")
		if xdg != "" {
			base = xdg
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".local", "share")
		}
	}
	return filepath.Join(base, "govprocure-pp-cli"), nil
}

// ConfigPath returns the path to the config file.
func ConfigPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

// DefaultDBPath returns the default SQLite DB path.
func DefaultDBPath() (string, error) {
	dir, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "data.db"), nil
}

// Load reads config from disk, returning defaults if not found.
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, fmt.Errorf("config path: %w", err)
	}

	dbPath, err := DefaultDBPath()
	if err != nil {
		return nil, fmt.Errorf("db path: %w", err)
	}

	cfg := &Config{
		GrantsURL:   DefaultGrantsURL,
		SAMURL:      DefaultSAMURL,
		USASpendURL: DefaultUSASpendURL,
		DBPath:      dbPath,
		SyncDays:    DefaultSyncDays,
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	// Apply defaults for empty fields
	if cfg.GrantsURL == "" {
		cfg.GrantsURL = DefaultGrantsURL
	}
	if cfg.SAMURL == "" {
		cfg.SAMURL = DefaultSAMURL
	}
	if cfg.USASpendURL == "" {
		cfg.USASpendURL = DefaultUSASpendURL
	}
	if cfg.DBPath == "" {
		cfg.DBPath = dbPath
	}
	if cfg.SyncDays == 0 {
		cfg.SyncDays = DefaultSyncDays
	}

	return cfg, nil
}

// Save writes config to disk.
func Save(cfg *Config) error {
	path, err := ConfigPath()
	if err != nil {
		return fmt.Errorf("config path: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create config file: %w", err)
	}
	defer f.Close()

	enc := toml.NewEncoder(f)
	if err := enc.Encode(cfg); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	return nil
}

// SetSAMKey updates the SAM API key in config and saves.
func SetSAMKey(key string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.SAMAPIKey = key
	return Save(cfg)
}
