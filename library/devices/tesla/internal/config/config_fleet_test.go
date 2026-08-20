package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAuthHeaderPrefersFleetBearer(t *testing.T) {
	t.Setenv("TESLA_FLEET_TOKEN", "")
	t.Run("UseFleetBearer routes to the fleet token", func(t *testing.T) {
		c := &Config{UseFleetBearer: true, Fleet: FleetConfig{AccessToken: "ft"}}
		if got := c.AuthHeader(); got != "Bearer ft" {
			t.Errorf("AuthHeader = %q, want %q", got, "Bearer ft")
		}
	})
	t.Run("without the flag, the owner token wins", func(t *testing.T) {
		c := &Config{AccessToken: "owner", Fleet: FleetConfig{AccessToken: "ft"}}
		if got := c.AuthHeader(); got != "Bearer owner" {
			t.Errorf("AuthHeader = %q, want %q", got, "Bearer owner")
		}
	})
	t.Run("flag set but no fleet token falls through", func(t *testing.T) {
		c := &Config{UseFleetBearer: true, AccessToken: "owner"}
		if got := c.AuthHeader(); got != "Bearer owner" {
			t.Errorf("AuthHeader = %q, want %q", got, "Bearer owner")
		}
	})
	t.Run("UseFleetBearer prefers TESLA_FLEET_TOKEN env over persisted", func(t *testing.T) {
		t.Setenv("TESLA_FLEET_TOKEN", "envft")
		c := &Config{UseFleetBearer: true, Fleet: FleetConfig{AccessToken: "ft"}}
		if got := c.AuthHeader(); got != "Bearer envft" {
			t.Errorf("AuthHeader = %q, want %q", got, "Bearer envft")
		}
	})
}

// TestFleetRefreshDoesNotPersistTransientRouting guards the regression where
// Fleet read-routing leaked transient state to disk: a Fleet token refresh must
// persist only the [fleet] block, never an auth_header or base_url that would
// disable Fleet routing (or misdirect the owner-api path) on the next run.
func TestFleetRefreshDoesNotPersistTransientRouting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	t.Setenv("TESLA_CONFIG", path)
	t.Setenv("TESLA_AUTH_TOKEN", "")
	t.Setenv("TESLA_FLEET_API_URL", "")

	seed, err := Load(path) // BaseURL defaults to owner-api
	if err != nil {
		t.Fatalf("seed load: %v", err)
	}
	if err := seed.SaveFleetTokens("cid", "secret", "oldtok", "r", time.Now().Add(time.Hour), "host.example", ""); err != nil {
		t.Fatalf("seed save: %v", err)
	}

	// Simulate the cli layer's in-memory routing (the fix: flag only).
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	cfg.UseFleetBearer = true
	if got := cfg.AuthHeader(); got != "Bearer oldtok" {
		t.Fatalf("AuthHeader = %q, want Bearer oldtok", got)
	}

	// Simulate the refresh callback persisting a new fleet token.
	if err := cfg.SaveFleetTokens("", "", "newtok", "r2", time.Now().Add(time.Hour), "", ""); err != nil {
		t.Fatalf("refresh save: %v", err)
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.AuthHeaderVal != "" {
		t.Errorf("auth_header leaked to disk: %q", reloaded.AuthHeaderVal)
	}
	if reloaded.UseFleetBearer {
		t.Errorf("UseFleetBearer must never persist")
	}
	if reloaded.BaseURL != "https://owner-api.teslamotors.com" {
		t.Errorf("base_url polluted: %q", reloaded.BaseURL)
	}
	if reloaded.Fleet.AccessToken != "newtok" {
		t.Errorf("fleet token not updated: %q", reloaded.Fleet.AccessToken)
	}
}
