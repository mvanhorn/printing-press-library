package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

// withTempAuthHome points the migration helpers at a temp directory and points
// the config layer at a temp config.toml inside it. Returns the legacy auth.json
// path the test can write to (or stat) plus a freshly-loaded cfg.
func withTempAuthHome(t *testing.T) (legacyPath string, cfg *config.Config) {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	t.Setenv("TESLA_PP_AUTH_HOME", dir)
	t.Setenv("TESLA_CONFIG", cfgPath)
	t.Setenv("TESLA_AUTH_TOKEN", "")
	c, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return filepath.Join(dir, "auth.json"), c
}

func writeLegacyAuthJSON(t *testing.T, path string, rec authRecord) {
	t.Helper()
	raw, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		t.Fatalf("marshal authRecord: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write legacy auth.json: %v", err)
	}
}

func TestLoadTeslaTokens_EmptyConfig_NoLegacy(t *testing.T) {
	_, cfg := withTempAuthHome(t)
	tokens, err := loadTeslaTokens(cfg)
	if err != nil {
		t.Fatalf("loadTeslaTokens: %v", err)
	}
	if tokens.Source != "none" {
		t.Errorf("source: got %q want %q", tokens.Source, "none")
	}
	if tokens.AccessToken != "" || tokens.RefreshToken != "" {
		t.Errorf("expected empty tokens, got access=%q refresh=%q", tokens.AccessToken, tokens.RefreshToken)
	}
}

func TestLoadTeslaTokens_ConfigPopulated(t *testing.T) {
	_, cfg := withTempAuthHome(t)
	want := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	if err := saveTeslaTokens(cfg, "refresh_abc", "access_xyz", want); err != nil {
		t.Fatalf("saveTeslaTokens: %v", err)
	}
	tokens, err := loadTeslaTokens(cfg)
	if err != nil {
		t.Fatalf("loadTeslaTokens: %v", err)
	}
	if tokens.Source != "config" {
		t.Errorf("source: got %q want config", tokens.Source)
	}
	if tokens.AccessToken != "access_xyz" || tokens.RefreshToken != "refresh_abc" {
		t.Errorf("tokens: got access=%q refresh=%q", tokens.AccessToken, tokens.RefreshToken)
	}
	if !tokens.ExpiresAt.Equal(want) {
		t.Errorf("expires_at: got %v want %v", tokens.ExpiresAt, want)
	}
}

func TestLoadTeslaTokens_MigrationFromLegacy(t *testing.T) {
	legacy, cfg := withTempAuthHome(t)
	writeLegacyAuthJSON(t, legacy, authRecord{
		AccessToken:  "access_old",
		RefreshToken: "refresh_old",
		ExpiresAt:    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		IssuedAt:     time.Now().UTC().Format(time.RFC3339),
	})

	tokens, err := loadTeslaTokens(cfg)
	if err != nil {
		t.Fatalf("loadTeslaTokens: %v", err)
	}
	if tokens.Source != "migrated" {
		t.Errorf("source: got %q want migrated", tokens.Source)
	}
	if tokens.AccessToken != "access_old" || tokens.RefreshToken != "refresh_old" {
		t.Errorf("post-migration tokens missing: %+v", tokens)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy file should be renamed, but it still exists at %s", legacy)
	}
	matches, _ := filepath.Glob(legacy + ".migrated-*")
	if len(matches) == 0 {
		t.Errorf("breadcrumb file not created (looked for %s.migrated-*)", legacy)
	}

	// Second call: config now populated, no further migration; source is "config".
	tokens2, err := loadTeslaTokens(cfg)
	if err != nil {
		t.Fatalf("second loadTeslaTokens: %v", err)
	}
	if tokens2.Source != "config" {
		t.Errorf("second call source: got %q want config", tokens2.Source)
	}
}

func TestLoadTeslaTokens_ConfigPresent_LegacyIgnored(t *testing.T) {
	legacy, cfg := withTempAuthHome(t)
	// Populate config first
	if err := saveTeslaTokens(cfg, "refresh_new", "access_new", time.Now().Add(time.Hour).UTC()); err != nil {
		t.Fatalf("saveTeslaTokens: %v", err)
	}
	// Drop legacy file in place
	writeLegacyAuthJSON(t, legacy, authRecord{
		AccessToken:  "access_legacy",
		RefreshToken: "refresh_legacy",
		ExpiresAt:    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	})

	tokens, err := loadTeslaTokens(cfg)
	if err != nil {
		t.Fatalf("loadTeslaTokens: %v", err)
	}
	if tokens.AccessToken != "access_new" {
		t.Errorf("config should win over legacy; got access=%q", tokens.AccessToken)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Errorf("legacy file should be left untouched, got stat err: %v", err)
	}
}

func TestMigrateLegacyAuthJSON_MalformedFile(t *testing.T) {
	legacy, cfg := withTempAuthHome(t)
	if err := os.WriteFile(legacy, []byte("this is not json {{"), 0o600); err != nil {
		t.Fatalf("write malformed: %v", err)
	}
	tokens, err := loadTeslaTokens(cfg)
	if err != nil {
		t.Fatalf("loadTeslaTokens: %v", err)
	}
	if tokens.Source != "none" {
		t.Errorf("malformed legacy should not migrate; got source %q", tokens.Source)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Errorf("legacy file should still exist after migration failure: %v", err)
	}
}

func TestMigrateLegacyAuthJSON_EmptyTokens(t *testing.T) {
	legacy, cfg := withTempAuthHome(t)
	writeLegacyAuthJSON(t, legacy, authRecord{}) // all blank
	tokens, err := loadTeslaTokens(cfg)
	if err != nil {
		t.Fatalf("loadTeslaTokens: %v", err)
	}
	if tokens.Source != "none" {
		t.Errorf("empty legacy should not migrate; got source %q", tokens.Source)
	}
}

func TestSaveTeslaTokens_RoundTrip(t *testing.T) {
	_, cfg := withTempAuthHome(t)
	want := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	if err := saveTeslaTokens(cfg, "r1", "a1", want); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Re-load by parsing the on-disk config.toml directly via config.Load (matches
	// what the rest of the CLI does on every command).
	reloaded, err := config.Load(cfg.Path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.AccessToken != "a1" || reloaded.RefreshToken != "r1" {
		t.Errorf("round-trip tokens: got access=%q refresh=%q", reloaded.AccessToken, reloaded.RefreshToken)
	}
	if !reloaded.TokenExpiry.Equal(want) {
		t.Errorf("round-trip expiry: got %v want %v", reloaded.TokenExpiry, want)
	}
	if reloaded.ClientID != teslaClientID {
		t.Errorf("client_id should be %q, got %q", teslaClientID, reloaded.ClientID)
	}
}

func TestSaveTeslaTokens_NilConfig(t *testing.T) {
	if err := saveTeslaTokens(nil, "r", "a", time.Now()); err == nil {
		t.Error("expected error for nil config")
	}
}

// Reference unused imports for the linter; strings ensures we don't accidentally
// drift the import list.
var _ = strings.TrimSpace
