// Tesla bearer auto-refresh hook for the API client.
//
// When the client transport receives a 401, it calls Client.OnTokenExpired.
// This file produces the callback closure: re-mint the bearer using the
// stored refresh token, persist via the U1 facade, and return the new
// Authorization header value. The client then rebuilds the request with
// the new header and retries once. If the user sets
// TESLA_PP_NO_AUTOREFRESH=1, the callback is not wired and 401s surface as
// errors for explicit handling (tools like `tesla doctor`).
//
// Hand-coded; lives outside the generator's emit set so it survives regens.
package cli

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

// teslaRefreshGuard serializes concurrent refresh attempts across goroutines.
// Without this, two requests hitting 401 simultaneously would both POST to
// /oauth2/v3/token; Tesla doesn't enforce single-use refresh tokens today,
// but the duplicate write to config.toml is a clear race we'd rather not
// rely on filesystem semantics to win.
var teslaRefreshGuard sync.Mutex

// makeTeslaAutoRefreshCallback builds the OnTokenExpired closure for the
// Client. The closure is safe to call concurrently; only one refresh exchange
// fires across all goroutines for a given cfg, and subsequent callers within
// that window see the freshly-minted token.
func makeTeslaAutoRefreshCallback(cfg *config.Config) func() (string, error) {
	if cfg == nil {
		return nil
	}
	return func() (string, error) {
		teslaRefreshGuard.Lock()
		defer teslaRefreshGuard.Unlock()

		// Re-read cfg from disk after acquiring the lock. Another goroutine
		// may have already refreshed; in that case the in-memory cfg held
		// by the first request is stale. Loading fresh state lets the
		// second caller pick up the refreshed bearer without retriggering
		// the network exchange.
		fresh, lerr := config.Load(cfg.Path)
		if lerr == nil && fresh.AccessToken != "" && fresh.TokenExpiry.After(time.Now()) {
			// Copy the new state into the original cfg so future calls on
			// the same struct see the new tokens (the client retains a
			// pointer to cfg, not a snapshot).
			cfg.AccessToken = fresh.AccessToken
			cfg.RefreshToken = fresh.RefreshToken
			cfg.TokenExpiry = fresh.TokenExpiry
			cfg.AuthSource = fresh.AuthSource
			return "Bearer " + fresh.AccessToken, nil
		}

		refresh := cfg.RefreshToken
		if refresh == "" && fresh != nil {
			refresh = fresh.RefreshToken
		}
		if refresh == "" {
			return "", fmt.Errorf("no refresh token available; run 'tesla auth login' first")
		}
		access, expiresIn, newRefresh, err := exchangeRefreshToken(refresh)
		if err != nil {
			return "", fmt.Errorf("auto-refresh: %w (run 'tesla auth login' to re-authenticate)", err)
		}
		expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second).UTC()
		finalRefresh := pickRefresh(refresh, newRefresh)
		if serr := saveTeslaTokens(cfg, finalRefresh, access, expiresAt); serr != nil {
			return "", fmt.Errorf("auto-refresh save: %w", serr)
		}
		return "Bearer " + access, nil
	}
}

// teslaAutoRefreshEnabled reports whether the auto-refresh hook should be wired.
// Defaults to enabled; set TESLA_PP_NO_AUTOREFRESH=1 to disable for explicit
// 401-checking tools.
func teslaAutoRefreshEnabled() bool {
	return os.Getenv("TESLA_PP_NO_AUTOREFRESH") == ""
}
