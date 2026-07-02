// Hand-authored sibling for Zoom-specific auth state. Owns the cached-token
// loader the generated config.Load() consults when no ZOOM_S2S_ACCESS_TOKEN is
// set in the environment. The token writer lives in internal/cli/zoom_auth.go.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/cliutil"
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

// RefreshHook, when non-nil, performs a live S2S OAuth token exchange and
// persists the result to the cache. internal/cli/zoom_auth.go wires this in
// an init() (reusing the exact exchange the `auth refresh` command runs) so
// that tryLoadCachedZoomToken can transparently refresh an expired cache
// instead of silently falling through to an unauthenticated request.
// internal/config cannot import internal/cli directly -- internal/cli
// already imports internal/config for Config/TokenCache, so that direction
// would be an import cycle. A package-level hook set from the higher layer
// is the standard way to break that cycle.
//
// PATCH(amend-2026-07-02: surface refresh-token expiry instead of a raw
// 401) -- previously an expired cache made this file return "", "" with no
// attempt to refresh, so config.Load() left AuthHeaderVal empty and every
// cloud command sent an unauthenticated request that Zoom answered with an
// opaque "HTTP 401: Invalid access token". auth status already reported
// cache_expired/cache_expires_at correctly; only the read path used by
// actual API calls was missing the refresh. See
// .printing-press-patches/s2s-token-cache-refreshes-transparently-on-expiry.json.
var RefreshHook func() (*TokenCache, error)

// tryLoadCachedZoomToken returns the cached bearer token (without the "Bearer "
// prefix) and the source label used by doctor when the cache is fresh; returns
// empty strings when the cache is missing, malformed, or expired and no
// refresh was possible.
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
		// Attempt a transparent refresh before giving up. Skipped under the
		// printing-press verifier (mock mode must never dial out); dogfood
		// runs are a real-network matrix and are meant to exercise this
		// path. RefreshHook itself no-ops (returns an error, no request
		// sent) when S2S credentials aren't present in the environment, so
		// this is a no-op fast path for the common case of no credentials.
		if RefreshHook != nil && !cliutil.IsVerifyEnv() {
			if refreshed, err := RefreshHook(); err == nil && refreshed != nil && refreshed.AccessToken != "" {
				return refreshed.AccessToken, "cache:refreshed:" + path
			}
		}
		return "", ""
	}
	return tc.AccessToken, "cache:" + path
}
