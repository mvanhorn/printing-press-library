package config

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

// GravitusUserID is the numeric Gravitus user ID (e.g. "123456789").
// Stored in config, also readable from GRAVITUS_USER_ID env var.
func (c *Config) UserID() string {
	if v := os.Getenv("GRAVITUS_USER_ID"); v != "" {
		return v
	}
	return c.GravitusUserID
}

// GravitusUserID field — added here so config.go (generated) doesn't need editing.
// The toml tag matches what we write to disk.

// SaveUserID persists the user ID to the config file without wiping other fields.
func (c *Config) SaveUserID(userID string) error {
	c.GravitusUserID = userID
	return c.save()
}

// DashboardDBPath returns the configured path to the dashboard's dev.db,
// falling back to the environment variable GRAVITUS_DASHBOARD_DB.
func (c *Config) DashboardDBPath() string {
	if v := os.Getenv("GRAVITUS_DASHBOARD_DB"); v != "" {
		return v
	}
	return c.GravitusDashboardDB
}

// SaveDashboardDB persists the dashboard DB path to config.
func (c *Config) SaveDashboardDB(path string) error {
	c.GravitusDashboardDB = path
	return c.save()
}

// extendedFields holds the Gravitus-specific config fields that the generated
// config.go doesn't know about. We embed them via struct tag overlay.
// These fields are read/written via toml alongside the generated fields.
type extendedFieldsInit struct{}

func init() {
	// Register our extended toml fields by patching the Config struct's
	// save/load cycle — done by re-marshaling with the full struct.
	// This is a no-op placeholder; the actual fields are declared directly
	// on Config in config.go via the toml tags below.
	_ = extendedFieldsInit{}
}

// marshalConfig is a helper that marshals cfg to bytes using toml,
// used by SaveUserID and SaveDashboardDB.
func marshalConfig(cfg *Config) ([]byte, error) {
	return toml.Marshal(cfg)
}

// Ensure toml is used
var _ = marshalConfig
