// Roundtrip tests for the on-disk Config TOML, focused on the [fleet] block
// added by 2026-05-22-001 U2. The critical property under test is that
// SaveTokens (iOS-app owner-api block) and SaveFleetTokens (Fleet API block)
// don't clobber each other: an install can hold both credential sets at once
// because the two audiences route to different vehicles.
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newConfigAt(t *testing.T) *Config {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load(empty): %v", err)
	}
	return cfg
}

func TestSaveFleetTokens_Roundtrip(t *testing.T) {
	cfg := newConfigAt(t)

	expiry := time.Now().Add(8 * time.Hour).UTC().Truncate(time.Second)
	if err := cfg.SaveFleetTokens(
		"client-id-abc",
		"client-secret-xyz",
		"fleet-access-token-1",
		"fleet-refresh-token-1",
		expiry,
		"keys.example.com",
		"/home/user/.tesla/keys-private.pem",
	); err != nil {
		t.Fatalf("SaveFleetTokens: %v", err)
	}

	// Re-load from disk.
	got, err := Load(cfg.Path)
	if err != nil {
		t.Fatalf("Load(after save): %v", err)
	}
	ft := got.FleetTokens()
	if ft.ClientID != "client-id-abc" {
		t.Errorf("Fleet.ClientID: got %q want client-id-abc", ft.ClientID)
	}
	if ft.ClientSecret != "client-secret-xyz" {
		t.Errorf("Fleet.ClientSecret: got %q want client-secret-xyz", ft.ClientSecret)
	}
	if ft.AccessToken != "fleet-access-token-1" {
		t.Errorf("Fleet.AccessToken: got %q want fleet-access-token-1", ft.AccessToken)
	}
	if ft.RefreshToken != "fleet-refresh-token-1" {
		t.Errorf("Fleet.RefreshToken: got %q want fleet-refresh-token-1", ft.RefreshToken)
	}
	if !ft.TokenExpiry.Equal(expiry) {
		t.Errorf("Fleet.TokenExpiry: got %v want %v", ft.TokenExpiry, expiry)
	}
	if ft.PublicKeyDomain != "keys.example.com" {
		t.Errorf("Fleet.PublicKeyDomain: got %q want keys.example.com", ft.PublicKeyDomain)
	}
	if ft.PrivateKeyPath != "/home/user/.tesla/keys-private.pem" {
		t.Errorf("Fleet.PrivateKeyPath: got %q", ft.PrivateKeyPath)
	}
}

// SaveTokens (iOS-app block) followed by SaveFleetTokens MUST leave both
// credential sets intact on disk. This is the canonical KD3 regression test.
func TestSaveFleetTokens_PreservesIOSAppTokens(t *testing.T) {
	cfg := newConfigAt(t)

	iosExpiry := time.Now().Add(8 * time.Hour).UTC().Truncate(time.Second)
	if err := cfg.SaveTokens("ownerapi", "", "ios-access-1", "ios-refresh-1", iosExpiry); err != nil {
		t.Fatalf("SaveTokens(ios): %v", err)
	}

	fleetExpiry := time.Now().Add(7 * time.Hour).UTC().Truncate(time.Second)
	if err := cfg.SaveFleetTokens(
		"fleet-client",
		"fleet-secret",
		"fleet-access-1",
		"fleet-refresh-1",
		fleetExpiry,
		"keys.example.com",
		"",
	); err != nil {
		t.Fatalf("SaveFleetTokens: %v", err)
	}

	// Re-read from disk; both blocks should be present.
	got, err := Load(cfg.Path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.AccessToken != "ios-access-1" {
		t.Errorf("top-level AccessToken clobbered: got %q want ios-access-1", got.AccessToken)
	}
	if got.RefreshToken != "ios-refresh-1" {
		t.Errorf("top-level RefreshToken clobbered: got %q want ios-refresh-1", got.RefreshToken)
	}
	if got.ClientID != "ownerapi" {
		t.Errorf("top-level ClientID clobbered: got %q want ownerapi", got.ClientID)
	}
	if !got.TokenExpiry.Equal(iosExpiry) {
		t.Errorf("top-level TokenExpiry clobbered: got %v want %v", got.TokenExpiry, iosExpiry)
	}

	ft := got.FleetTokens()
	if ft.ClientID != "fleet-client" {
		t.Errorf("Fleet.ClientID: got %q want fleet-client", ft.ClientID)
	}
	if ft.AccessToken != "fleet-access-1" {
		t.Errorf("Fleet.AccessToken: got %q want fleet-access-1", ft.AccessToken)
	}
	if ft.RefreshToken != "fleet-refresh-1" {
		t.Errorf("Fleet.RefreshToken: got %q want fleet-refresh-1", ft.RefreshToken)
	}
	if ft.PublicKeyDomain != "keys.example.com" {
		t.Errorf("Fleet.PublicKeyDomain: got %q want keys.example.com", ft.PublicKeyDomain)
	}
}

// SaveFleetTokens then SaveTokens (reverse order) also leaves both blocks
// intact — SaveTokens shouldn't blank the [fleet] block either.
func TestSaveTokens_PreservesFleetBlock(t *testing.T) {
	cfg := newConfigAt(t)

	fleetExpiry := time.Now().Add(7 * time.Hour).UTC().Truncate(time.Second)
	if err := cfg.SaveFleetTokens(
		"fleet-client",
		"fleet-secret",
		"fleet-access-2",
		"fleet-refresh-2",
		fleetExpiry,
		"keys.example.com",
		"",
	); err != nil {
		t.Fatalf("SaveFleetTokens: %v", err)
	}

	iosExpiry := time.Now().Add(8 * time.Hour).UTC().Truncate(time.Second)
	if err := cfg.SaveTokens("ownerapi", "", "ios-access-2", "ios-refresh-2", iosExpiry); err != nil {
		t.Fatalf("SaveTokens(ios): %v", err)
	}

	got, err := Load(cfg.Path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	ft := got.FleetTokens()
	if ft.AccessToken != "fleet-access-2" {
		t.Errorf("Fleet.AccessToken cleared by SaveTokens: got %q want fleet-access-2", ft.AccessToken)
	}
	if ft.ClientID != "fleet-client" {
		t.Errorf("Fleet.ClientID cleared by SaveTokens: got %q", ft.ClientID)
	}
}

// SaveFleetTokens with empty fields only updates the non-empty ones; existing
// values for empty fields are preserved.
func TestSaveFleetTokens_PartialUpdate(t *testing.T) {
	cfg := newConfigAt(t)

	expiry := time.Now().Add(8 * time.Hour).UTC().Truncate(time.Second)
	if err := cfg.SaveFleetTokens(
		"client-1",
		"secret-1",
		"access-1",
		"refresh-1",
		expiry,
		"keys.example.com",
		"/p/key.pem",
	); err != nil {
		t.Fatalf("SaveFleetTokens(initial): %v", err)
	}

	// Now update only the access token (e.g. fleet-refresh keeps the rest).
	newExpiry := time.Now().Add(8 * time.Hour).UTC().Truncate(time.Second)
	if err := cfg.SaveFleetTokens("", "", "access-2", "", newExpiry, "", ""); err != nil {
		t.Fatalf("SaveFleetTokens(partial): %v", err)
	}

	got, err := Load(cfg.Path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	ft := got.FleetTokens()
	if ft.AccessToken != "access-2" {
		t.Errorf("AccessToken: got %q want access-2", ft.AccessToken)
	}
	if ft.RefreshToken != "refresh-1" {
		t.Errorf("RefreshToken changed unexpectedly: got %q want refresh-1", ft.RefreshToken)
	}
	if ft.ClientID != "client-1" {
		t.Errorf("ClientID changed unexpectedly: got %q want client-1", ft.ClientID)
	}
	if ft.PublicKeyDomain != "keys.example.com" {
		t.Errorf("PublicKeyDomain changed: got %q want keys.example.com", ft.PublicKeyDomain)
	}
}

// Loading a config.toml with a [fleet] block authored by hand should parse
// cleanly, matching the on-the-wire shape SaveFleetTokens writes.
func TestLoad_FleetBlockOnDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	contents := `base_url = "https://owner-api.teslamotors.com"
access_token = "ios-top-level"

[fleet]
client_id = "fleet-id-on-disk"
client_secret = "fleet-secret-on-disk"
access_token = "fleet-access-on-disk"
refresh_token = "fleet-refresh-on-disk"
public_key_domain = "keys.example.com"
private_key_path = "/p/private.pem"
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write seed config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AccessToken != "ios-top-level" {
		t.Errorf("top-level AccessToken: got %q want ios-top-level", cfg.AccessToken)
	}
	ft := cfg.FleetTokens()
	if ft.ClientID != "fleet-id-on-disk" {
		t.Errorf("Fleet.ClientID: got %q want fleet-id-on-disk", ft.ClientID)
	}
	if ft.AccessToken != "fleet-access-on-disk" {
		t.Errorf("Fleet.AccessToken: got %q want fleet-access-on-disk", ft.AccessToken)
	}
	if ft.PublicKeyDomain != "keys.example.com" {
		t.Errorf("Fleet.PublicKeyDomain: got %q want keys.example.com", ft.PublicKeyDomain)
	}
}

// Ensure the persisted TOML contains a [fleet] block header after
// SaveFleetTokens, not flat keys.
func TestSaveFleetTokens_WritesTOMLBlock(t *testing.T) {
	cfg := newConfigAt(t)
	if err := cfg.SaveFleetTokens("cid", "csec", "atok", "rtok", time.Now().Add(time.Hour), "d.example", ""); err != nil {
		t.Fatalf("SaveFleetTokens: %v", err)
	}
	data, err := os.ReadFile(cfg.Path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "[fleet]") {
		t.Errorf("expected [fleet] block header in config TOML, got:\n%s", s)
	}
	if !strings.Contains(s, "client_id = 'cid'") && !strings.Contains(s, `client_id = "cid"`) {
		t.Errorf("expected fleet client_id key in TOML, got:\n%s", s)
	}
}
