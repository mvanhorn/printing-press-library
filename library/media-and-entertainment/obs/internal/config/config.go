package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const configDir = ".obs-pp"
const configFile = "config.json"

// Config holds OBS WebSocket connection settings.
// Credentials are never stored in source — only in ~/.obs-pp/config.json.
type Config struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Password string `json:"password"`
}

func DefaultConfig() Config {
	return Config{
		Host:     "localhost",
		Port:     4455,
		Password: "",
	}
}

func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDir, configFile), nil
}

func Load() (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return Config{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, fmt.Errorf("OBS CLI not configured. Run: obs-pp-cli configure")
		}
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("invalid config file: %w", err)
	}
	return cfg, nil
}

func Save(cfg Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, configDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	path := filepath.Join(dir, configFile)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	// 0600 = owner read/write only — no credentials exposed to other users
	return os.WriteFile(path, data, 0600)
}

func (c Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
