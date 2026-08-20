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

// teslaShouldUseFleetForReads reports whether the read client should target the
// regional Fleet API with the Fleet bearer instead of the owner-api host. True
// when no owner-api credential is configured but a Fleet user token is present
// — the case for 2021+ vehicles and non-NA accounts, where the owner-api read
// path is gone. Set TESLA_PP_FORCE_FLEET_READS=0 to force the legacy path.
func teslaShouldUseFleetForReads(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	// Never affects users who haven't set up Fleet (e.g. pre-2021 cars on the
	// owner-api read path).
	if !teslaFleetConfigured(cfg) {
		return false
	}
	switch os.Getenv("TESLA_PP_FORCE_FLEET_READS") {
	case "0":
		return false
	case "1":
		return true
	}
	// Fleet is configured. Prefer it for reads unless a *usable* owner-api
	// credential exists: an explicit header/env override, or a file-stored
	// owner-api token that hasn't expired. A stale owner-api token — the common
	// leftover after moving a 2021+ car to Fleet, often persisted with a zero
	// expiry — must not block Fleet reads.
	//
	// This is a per-install decision, not per-vehicle: a mixed account (a
	// pre-2021 car on owner-api plus a 2021+/non-NA car that only works via
	// Fleet) with a *valid* owner token routes every read through owner-api
	// here. That case is recovered reactively — newClient arms a Fleet fallback
	// (Client.FleetFallback) that retries through the Fleet API when an
	// owner-api vehicle read 404s — so the Fleet-only car is still readable
	// without TESLA_PP_FORCE_FLEET_READS=1. This heuristic still picks the
	// fast path (no wasted owner-api round-trip) for the common single-profile
	// install.
	ownerUsable := cfg.AuthHeaderVal != "" || cfg.TeslaAuthToken != "" ||
		(cfg.AccessToken != "" && cfg.TokenExpiry.After(time.Now()))
	return !ownerUsable
}

// teslaFleetConfigured reports whether a Fleet user token is available, whether
// from the [fleet] block or the TESLA_FLEET_TOKEN env override (matching
// AuthHeader() and the other Fleet surfaces).
func teslaFleetConfigured(cfg *config.Config) bool {
	return cfg != nil && (cfg.Fleet.AccessToken != "" || os.Getenv("TESLA_FLEET_TOKEN") != "")
}

// makeTeslaFleetRefreshCallback mirrors makeTeslaAutoRefreshCallback but for the
// [fleet] block: on a 401 it re-mints the Fleet user token via the Fleet
// refresh-token grant and returns the fresh bearer. Used when reads route
// through the Fleet API (see newClient / teslaShouldUseFleetForReads). Shares
// teslaRefreshGuard so a Fleet and an owner-api refresh never race the config
// file.
func makeTeslaFleetRefreshCallback(cfg *config.Config) func() (string, error) {
	if cfg == nil {
		return nil
	}
	return func() (string, error) {
		teslaRefreshGuard.Lock()
		defer teslaRefreshGuard.Unlock()

		// Another invocation may have refreshed already; adopt disk state
		// rather than re-firing the network exchange.
		if fresh, lerr := config.Load(cfg.Path); lerr == nil &&
			fresh.Fleet.AccessToken != "" &&
			fresh.Fleet.TokenExpiry.After(time.Now()) &&
			fresh.Fleet.AccessToken != cfg.Fleet.AccessToken {
			cfg.Fleet = fresh.Fleet
			// AuthHeader() (UseFleetBearer) returns the new token from Fleet.AccessToken.
			return "Bearer " + fresh.Fleet.AccessToken, nil
		}

		ft := cfg.FleetTokens()
		if ft.RefreshToken == "" {
			return "", fmt.Errorf("no Fleet refresh token available; run 'tesla auth fleet-login' first")
		}
		effClientID := firstNonEmpty(os.Getenv("TESLA_FLEET_CLIENT_ID"), ft.ClientID)
		if effClientID == "" {
			return "", fmt.Errorf("no Fleet client_id available for refresh; run 'tesla auth fleet-register' first")
		}
		tokenURL := fleetTokenURL
		if base := os.Getenv("TESLA_FLEET_AUTH_URL"); base != "" {
			tokenURL = base + "/oauth2/v3/token"
		}
		_, curScope, _ := decodeJWTClaims(ft.AccessToken)
		tok, err := fleetRefreshGrant(tokenURL, effClientID, ft.RefreshToken, curScope)
		if err != nil {
			return "", fmt.Errorf("fleet auto-refresh: %w (run 'tesla auth fleet-login' to re-authenticate)", err)
		}
		expiresAt := time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second).UTC()
		finalRefresh := tok.RefreshToken
		if finalRefresh == "" {
			finalRefresh = ft.RefreshToken
		}
		if serr := cfg.SaveFleetTokens("", "", tok.AccessToken, finalRefresh, expiresAt, "", ""); serr != nil {
			return "", fmt.Errorf("fleet auto-refresh save: %w", serr)
		}
		// SaveFleetTokens updated cfg.Fleet.AccessToken in memory; AuthHeader()
		// (UseFleetBearer) now returns this token for subsequent requests.
		return "Bearer " + tok.AccessToken, nil
	}
}
