// Hand-authored sibling for Zoom-specific auth state. Owns the cached-token
// loader the generated config.Load() consults when no ZOOM_S2S_ACCESS_TOKEN is
// set in the environment. The token writer lives in internal/cli/zoom_auth.go.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// TokenCache is the on-disk shape written by `zoom-pp-cli auth set-token`.
// The CLI exchanges S2S OAuth account credentials for a bearer token at
// https://zoom.us/oauth/token (account_credentials grant) and persists the
// result here so subsequent invocations don't re-exchange. The cache lives at
// ~/.config/zoom-pp-cli/token.json with mode 0600.
type TokenCache struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
	Scope       string    `json:"scope,omitempty"`
	AccountID   string    `json:"account_id,omitempty"`
}

// TokenCachePath returns the canonical location for the S2S OAuth token cache.
// Exported so internal/cli/zoom_auth.go can write to the same path.
func TokenCachePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "zoom-pp-cli", "token.json")
}

// tryLoadCachedZoomToken returns the cached bearer token (without the "Bearer "
// prefix) and the source label used by doctor when the cache is fresh; returns
// empty strings when the cache is missing, malformed, or expired.
func tryLoadCachedZoomToken() (string, string) {
	path := TokenCachePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	var tc TokenCache
	if err := json.Unmarshal(data, &tc); err != nil {
		return "", ""
	}
	if tc.AccessToken == "" {
		return "", ""
	}
	// Treat any token expiring in the next 60s as expired so callers don't
	// race the refresh window.
	if !tc.ExpiresAt.IsZero() && time.Until(tc.ExpiresAt) < 60*time.Second {
		return "", ""
	}
	return tc.AccessToken, "cache:" + path
}
