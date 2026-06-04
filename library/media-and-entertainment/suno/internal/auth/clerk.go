// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// Hand-built Clerk browser-auth flow for Suno. Suno's web app authenticates
// through Clerk (auth.suno.com): a long-lived __client cookie identifies the
// browser session, from which a short-lived JWT is minted per session and sent
// as Bearer to the studio API. This package implements the three wire steps
// (resolve session id, mint JWT, decode/expiry-check JWT) so the CLI can
// reproduce the browser's auth without a headless browser.

package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/cliutil"
)

// clerkLimiter throttles the auth-handshake calls to auth.suno.com. Clerk's
// frontend API rate-limits aggressively, so even though these calls fire only
// on login/refresh we ramp politely and back off on 429 using the same
// adaptive limiter the data-plane client uses.
var clerkLimiter = cliutil.NewAdaptiveLimiter(2.0)

const (
	// ClerkHost is the Clerk frontend API host for Suno.
	ClerkHost = "https://auth.suno.com"
	// ClerkQueryParams are appended to every Clerk request; the versions are
	// the ones the Suno web app currently pins.
	ClerkQueryParams = "__clerk_api_version=2025-11-10&_clerk_js_version=5.117.0"
)

// clerkHeaders returns the headers every Clerk call must carry. The __client
// value is sent both as the (non-Bearer) Authorization header and as a Cookie.
func clerkHeaders(clientCookie string) http.Header {
	h := http.Header{}
	h.Set("Authorization", clientCookie)
	h.Set("Cookie", "__client="+clientCookie)
	h.Set("Origin", "https://suno.com")
	h.Set("Referer", "https://suno.com/")
	h.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36")
	return h
}

// ResolveSessionID performs GET /v1/client and returns the active session id.
// Prefers last_active_session_id, falling back to sessions[0].id.
func ResolveSessionID(ctx context.Context, httpClient *http.Client, clientCookie string) (string, error) {
	url := ClerkHost + "/v1/client?" + ClerkQueryParams
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header = clerkHeaders(clientCookie)

	clerkLimiter.Wait()
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("clerk /v1/client request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusTooManyRequests {
		clerkLimiter.OnRateLimit()
		return "", fmt.Errorf("clerk /v1/client rate limited (HTTP 429)%s; wait a moment and retry", retryAfterHint(resp))
	}
	clerkLimiter.OnSuccess()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("clerk /v1/client returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	// Clerk wraps payloads under "response".
	var parsed struct {
		Response struct {
			LastActiveSessionID string `json:"last_active_session_id"`
			Sessions            []struct {
				ID string `json:"id"`
			} `json:"sessions"`
		} `json:"response"`
		// Some Clerk responses are unwrapped; accept the flat shape too.
		LastActiveSessionID string `json:"last_active_session_id"`
		Sessions            []struct {
			ID string `json:"id"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parsing clerk /v1/client response: %w", err)
	}

	if parsed.Response.LastActiveSessionID != "" {
		return parsed.Response.LastActiveSessionID, nil
	}
	if len(parsed.Response.Sessions) > 0 && parsed.Response.Sessions[0].ID != "" {
		return parsed.Response.Sessions[0].ID, nil
	}
	if parsed.LastActiveSessionID != "" {
		return parsed.LastActiveSessionID, nil
	}
	if len(parsed.Sessions) > 0 && parsed.Sessions[0].ID != "" {
		return parsed.Sessions[0].ID, nil
	}
	return "", fmt.Errorf("no active Clerk session found; log in to suno.com in Chrome and try again")
}

// MintJWT performs POST /v1/client/sessions/{id}/tokens and returns the JWT.
func MintJWT(ctx context.Context, httpClient *http.Client, clientCookie, sessionID string) (string, error) {
	url := fmt.Sprintf("%s/v1/client/sessions/%s/tokens?%s", ClerkHost, sessionID, ClerkQueryParams)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(""))
	if err != nil {
		return "", err
	}
	req.Header = clerkHeaders(clientCookie)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	clerkLimiter.Wait()
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("clerk token mint request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusTooManyRequests {
		clerkLimiter.OnRateLimit()
		return "", fmt.Errorf("clerk token mint rate limited (HTTP 429)%s; wait a moment and retry", retryAfterHint(resp))
	}
	clerkLimiter.OnSuccess()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("clerk token mint returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	var parsed struct {
		JWT string `json:"jwt"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parsing clerk token response: %w", err)
	}
	if parsed.JWT == "" {
		return "", fmt.Errorf("clerk token response did not contain a jwt")
	}
	return parsed.JWT, nil
}

// JWTExpiry decodes the JWT payload (segment 1, base64url no padding) and
// returns its exp as a unix timestamp. Returns (0, error) when the token is
// malformed or carries no exp claim.
func JWTExpiry(jwt string) (int64, error) {
	jwt = strings.TrimSpace(strings.TrimPrefix(jwt, "Bearer "))
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return 0, fmt.Errorf("token is not a JWT (expected 3 segments, got %d)", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, fmt.Errorf("decoding JWT payload: %w", err)
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return 0, fmt.Errorf("parsing JWT claims: %w", err)
	}
	if claims.Exp == 0 {
		return 0, fmt.Errorf("JWT has no exp claim")
	}
	return claims.Exp, nil
}

// JWTNeedsRefresh reports whether the JWT is expired or expires within the next
// 30 minutes (the same 1800s skew the browser uses). A token that can't be
// decoded is treated as needing refresh.
func JWTNeedsRefresh(jwt string) bool {
	exp, err := JWTExpiry(jwt)
	if err != nil {
		return true
	}
	return time.Now().Unix()+1800 >= exp
}

// retryAfterHint formats a " (retry after Ns)" suffix when the response carries
// a Retry-After header, and an empty string otherwise.
func retryAfterHint(resp *http.Response) string {
	if ra := strings.TrimSpace(resp.Header.Get("Retry-After")); ra != "" {
		return " (retry after " + ra + "s)"
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
