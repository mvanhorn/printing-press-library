// Tesla token storage facade. Single source of truth: config.toml's
// access_token / refresh_token / token_expiry slots, written via cfg.SaveTokens.
// This file owns the one-time migration from the legacy
// ~/.config/tesla-pp-cli/auth.json file that earlier hand-coded auth flows
// used. Hand-coded; lives outside the generator's emit set so it survives
// `printing-press generate --force` regens.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

// teslaTokens is the in-memory token bundle every Tesla auth path reads or
// writes. Source labels how the bundle was located so doctor and the verbose
// surfaces can show provenance.
type teslaTokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	Source       string // "config" | "env" | "migrated" | "none"
}

// loadTeslaTokens returns the current token bundle from cfg. If cfg's token
// slots are empty AND a legacy ~/.config/tesla-pp-cli/auth.json exists, the
// function migrates it into cfg via SaveTokens and renames the legacy file
// with a date breadcrumb. Best-effort: malformed legacy files emit a stderr
// warning and are skipped rather than failing the call.
func loadTeslaTokens(cfg *config.Config) (teslaTokens, error) {
	if cfg == nil {
		return teslaTokens{Source: "none"}, fmt.Errorf("config is nil")
	}
	if cfg.AccessToken != "" || cfg.RefreshToken != "" {
		src := "config"
		if cfg.AuthSource == "env:TESLA_AUTH_TOKEN" {
			src = "env"
		}
		return teslaTokens{
			AccessToken:  cfg.AccessToken,
			RefreshToken: cfg.RefreshToken,
			ExpiresAt:    cfg.TokenExpiry,
			Source:       src,
		}, nil
	}
	migrated, err := migrateLegacyAuthJSON(cfg)
	if err != nil {
		return teslaTokens{Source: "none"}, err
	}
	if migrated {
		return teslaTokens{
			AccessToken:  cfg.AccessToken,
			RefreshToken: cfg.RefreshToken,
			ExpiresAt:    cfg.TokenExpiry,
			Source:       "migrated",
		}, nil
	}
	return teslaTokens{Source: "none"}, nil
}

// saveTeslaTokens writes the bundle to cfg. Tesla's ownerapi OAuth flow uses
// PKCE (no client secret), so clientSecret is intentionally empty. clientID
// is hardcoded to "ownerapi" because Tesla never rotates that constant for
// the iOS-app client.
func saveTeslaTokens(cfg *config.Config, refreshToken, accessToken string, expiresAt time.Time) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	return cfg.SaveTokens(teslaClientID, "", accessToken, refreshToken, expiresAt)
}

// migrateLegacyAuthJSON moves tokens from the v0.1 auth.json file into the
// config layer. Returns true if a migration was performed. Renames the legacy
// file with a date breadcrumb (auth.json.migrated-YYYYMMDD); does not delete
// it. Subsequent calls see no legacy file and return false. Surfaces all
// non-fatal failures as stderr warnings so the caller flow continues.
func migrateLegacyAuthJSON(cfg *config.Config) (bool, error) {
	legacyDir, err := legacyAuthJSONDir()
	if err != nil {
		return false, nil
	}
	legacy := filepath.Join(legacyDir, "auth.json")
	f, err := os.Open(legacy)
	if err != nil {
		return false, nil
	}
	defer f.Close()

	var rec authRecord
	if err := json.NewDecoder(f).Decode(&rec); err != nil {
		fmt.Fprintf(os.Stderr, "warning: legacy %s is malformed (%v); skipping migration\n", legacy, err)
		return false, nil
	}
	if rec.AccessToken == "" && rec.RefreshToken == "" {
		return false, nil
	}
	expiresAt, _ := time.Parse(time.RFC3339, rec.ExpiresAt)
	if err := saveTeslaTokens(cfg, rec.RefreshToken, rec.AccessToken, expiresAt); err != nil {
		return false, fmt.Errorf("migrate legacy auth.json: %w", err)
	}
	breadcrumb := legacy + ".migrated-" + time.Now().UTC().Format("20060102")
	if rerr := os.Rename(legacy, breadcrumb); rerr != nil {
		fmt.Fprintf(os.Stderr, "warning: migrated tokens to config.toml but couldn't rename %s: %v\n", legacy, rerr)
		return true, nil
	}
	fmt.Fprintf(os.Stderr, "Migrated legacy auth.json into config.toml; previous file renamed to %s\n", breadcrumb)
	return true, nil
}

// legacyAuthJSONDir resolves the directory that held the v0.1 auth.json. Kept
// here (not in tesla_auth_refresh.go) so the migration is self-contained and
// can run before the rest of that file is touched in U3.
func legacyAuthJSONDir() (string, error) {
	if override := os.Getenv("TESLA_PP_AUTH_HOME"); override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "tesla-pp-cli"), nil
}
