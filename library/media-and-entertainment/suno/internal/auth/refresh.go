// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// JWT freshness wiring. Commands call EnsureFreshJWT before hitting the studio
// API so an expired Clerk-minted JWT is transparently re-minted from the stored
// __client cookie. No-op when auth comes from the SUNO_JWT env var (we can't
// re-mint a token the operator supplied) or when no __client cookie is stored.

package auth

import (
	"context"
	"fmt"
	"net/http"
	"os"
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

// readStudioCookieHeader is the browser-cookie read behind EnsureFreshSession,
// indirected through a package var so tests can stub it without a real Chrome
// cookie store.
var readStudioCookieHeader = SunoStudioCookieHeader

// RefreshStudioCookies force-pulls the studio cookie header from the browser
// (bypassing the cache), persists it, and returns it. Used by the client to
// recover from a stale-cookie rejection. Returns the stored header unchanged
// when the browser pull is empty (don't clobber a good cache with nothing).
func RefreshStudioCookies(ctx context.Context, cfg *config.Config) string {
	if cfg == nil || cfg.IsEnvAuth() {
		return ""
	}
	header := readStudioCookieHeader(ctx)
	if header == "" {
		return cfg.StudioCookieHeader()
	}
	var exp int64
	if e, err := JWTExpiry(cfg.SunoJwt); err == nil {
		exp = e
	}
	if err := cfg.SaveStudioSession(header, exp); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not persist refreshed studio cookies: %v\n", err)
	}
	return header
}

// EnsureFreshSession keeps the studio session usable while reading the browser
// at most once. It re-mints the JWT when stale (EnsureFreshJWT) and then, only
// when no cookie header is stored, pulls cookies from the browser once and
// persists them with the JWT's expiry. Studio cookies are intentionally
// decoupled from the JWT: suno.com cookies are long-lived, so they are
// preserved across JWT re-mints and re-pulled only when absent here or when the
// API rejects them as stale (422 token_validation_failed, handled at request
// time). No-op for env-supplied tokens (not ours to manage).
func EnsureFreshSession(ctx context.Context, cfg *config.Config) error {
	if cfg == nil || cfg.IsEnvAuth() {
		return nil
	}
	if err := EnsureFreshJWT(ctx, cfg); err != nil {
		return err
	}
	if cfg.StudioCookieHeader() != "" {
		return nil
	}
	header := readStudioCookieHeader(ctx)
	if header == "" {
		// Browser unavailable / no cookies: persist nothing and retry next run.
		// (We only reach here when no header is stored, so there's nothing to keep.)
		return nil
	}
	var exp int64
	if e, err := JWTExpiry(cfg.SunoJwt); err == nil {
		exp = e
	}
	return cfg.SaveStudioSession(header, exp)
}
