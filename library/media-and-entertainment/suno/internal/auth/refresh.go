// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// JWT freshness wiring. Commands call EnsureFreshJWT before hitting the studio
// API so an expired Clerk-minted JWT is transparently re-minted from the stored
// __client cookie. No-op when auth comes from the SUNO_JWT env var (we can't
// re-mint a token the operator supplied) or when no __client cookie is stored.

package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/config"
)

// EnsureFreshJWT re-mints the stored JWT when it is expired or near-expiry and
// a __client cookie is available to re-mint with. It persists the new JWT (and
// session id, if it changed) into the config file. Returns nil when no refresh
// was needed or possible; mint/network failures are returned so the caller can
// decide whether to proceed with the (stale) token.
func EnsureFreshJWT(ctx context.Context, cfg *config.Config) error {
	if cfg == nil {
		return nil
	}
	// Env-supplied tokens are not ours to refresh (SUNO_TOKEN or SUNO_JWT).
	if strings.HasPrefix(cfg.AuthSource, "env:") {
		return nil
	}
	clientCookie := cfg.ClerkClientCookie()
	if clientCookie == "" {
		// No cookie to re-mint with — leave whatever JWT is stored as-is.
		return nil
	}
	if cfg.SunoJwt != "" && !JWTNeedsRefresh(cfg.SunoJwt) {
		return nil
	}

	httpClient := &http.Client{Timeout: 20 * time.Second}

	sessionID := cfg.ClerkSessionID()
	if sessionID == "" {
		resolved, err := ResolveSessionID(ctx, httpClient, clientCookie)
		if err != nil {
			return err
		}
		sessionID = resolved
	}

	jwt, err := MintJWT(ctx, httpClient, clientCookie, sessionID)
	if err != nil {
		return err
	}
	return cfg.SaveSunoSession(jwt, "", sessionID, "")
}
