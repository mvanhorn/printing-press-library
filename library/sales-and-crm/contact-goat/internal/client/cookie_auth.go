// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/chromecookies"
)

// Option configures optional behavior on a Client without breaking the
// existing constructor signature.
type Option func(*Client)

// cookieAuthState holds the cookie jar and Clerk-refresh machinery for a
// Client that was configured with WithCookieAuth. It is created lazily and
// guarded by a mutex to keep proactive refreshes race-free across concurrent
// requests.
type cookieAuthState struct {
	mu  sync.Mutex
	jar *cookiejar.Jar
	// lastRefresh prevents a thundering-herd of refresh requests when many
	// goroutines see an expired JWT at the same moment.
	lastRefresh time.Time
}

// Clerk frontend-API constants. The Happenstance web app refreshes a
// session by POSTing to the `touch` endpoint, which returns 200 and
// sets an updated __session cookie via Set-Cookie. It is NOT the
// `/tokens` endpoint — that is a different (server-to-server) Clerk
// surface and will return 404 when called with a browser session id.
//
// `clerkJSVersion` must be sent because Clerk treats missing or stale
// JS-version callers as potentially-compromised and denies refresh.
// Keep this in sync with what the Happenstance web app ships; sniffs
// live under library/sales-and-crm/contact-goat/.manuscripts/.
const (
	clerkAPIVersion   = "2025-11-10"
	clerkJSVersion    = "5.125.9"
	jwtExpirySlackSec = 10 // refresh proactively when < 10s remains
)

// clerkBaseURL is a var (not const) so tests can point the refresh
// call at an httptest server. Production code never writes to it.
var clerkBaseURL = "https://clerk.happenstance.ai"

// WithCookieAuth installs a cookiejar seeded with the provided cookies and
// enables automatic Clerk-session refresh on 401/204 `x-clerk-auth-status:
// signed-out` responses. Cookie values are kept in memory only and are never
// logged.
func WithCookieAuth(cookies []chromecookies.Cookie) Option {
	return func(c *Client) {
		jar, err := cookiejar.New(nil)
		if err != nil {
			// cookiejar.New with nil options never errors in the
			// current stdlib, but guard anyway.
			return
		}
		seedJar(jar, cookies)
		if c.HTTPClient == nil {
			c.HTTPClient = &http.Client{}
		}
		c.HTTPClient.Jar = jar
		c.cookieAuth = &cookieAuthState{jar: jar}
	}
}

// ApplyOptions applies one or more Options to an existing Client. Safe to
// call before the client issues any requests. Existing callers keep using
// client.New(...) and then ApplyOptions(WithCookieAuth(...)) when they want
// Happenstance cookie-based auth.
func (c *Client) ApplyOptions(opts ...Option) {
	for _, opt := range opts {
		opt(c)
	}
}

// seedJar walks the supplied cookies and installs each one onto the jar
// under its own domain. We need an http.Cookie per entry rather than a
// single SetCookies call because the cookies may belong to several
// sibling domains (happenstance.ai, clerk.happenstance.ai).
func seedJar(jar *cookiejar.Jar, cookies []chromecookies.Cookie) {
	for _, c := range cookies {
		host := strings.TrimPrefix(c.Domain, ".")
		u := &url.URL{Scheme: "https", Host: host, Path: "/"}
		hc := &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Path:     c.Path,
			Domain:   c.Domain,
			HttpOnly: c.HttpOnly,
			Secure:   c.Secure,
		}
		if !c.Expires.IsZero() {
			hc.Expires = c.Expires
		}
		jar.SetCookies(u, []*http.Cookie{hc})
	}
}

// IsJWTExpired parses the middle segment of a JWT, extracts the `exp` claim,
// and returns true when it's missing or within jwtExpirySlackSec seconds of
// now. It does NOT verify the signature — that's the server's job; we just
// need to know whether to refresh proactively.
func IsJWTExpired(jwt string) bool {
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return true
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Some tokens use standard base64 with padding. Try that too.
		payload, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return true
		}
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return true
	}
	if claims.Exp == 0 {
		return true
	}
	return time.Now().Unix()+jwtExpirySlackSec >= claims.Exp
}

// sessionCookieValue returns the current __session JWT from the jar, or ""
// if none is present.
func (c *Client) sessionCookieValue() string {
	if c.cookieAuth == nil || c.cookieAuth.jar == nil {
		return ""
	}
	u := &url.URL{Scheme: "https", Host: "happenstance.ai", Path: "/"}
	for _, ck := range c.cookieAuth.jar.Cookies(u) {
		if ck.Name == "__session" {
			return ck.Value
		}
	}
	return ""
}

// clerkSessionID returns the session id stored in clerk_active_context.
// Happenstance emits this cookie as a bare "sess_..." string (not JSON),
// but older Clerk versions used JSON like {"session_id":"sess_xxx",...}.
// Handle both shapes defensively.
func (c *Client) clerkSessionID() string {
	if c.cookieAuth == nil || c.cookieAuth.jar == nil {
		return ""
	}
	u := &url.URL{Scheme: "https", Host: "happenstance.ai", Path: "/"}
	for _, ck := range c.cookieAuth.jar.Cookies(u) {
		if ck.Name != "clerk_active_context" {
			continue
		}
		val, err := url.QueryUnescape(ck.Value)
		if err != nil {
			val = ck.Value
		}
		// Shape 1: bare session id, e.g. "sess_3Cac4..." (current Happenstance).
		// Strip trailing ":" and whitespace; the cookie's URL-encoded form
		// sometimes carries a trailing colon (observed on 2026-04-19).
		// Leaving it in place produces "/v1/client/sessions/sess_XXX:/touch"
		// which Clerk returns 404 for.
		if strings.HasPrefix(val, "sess_") {
			return strings.TrimRight(val, ": \t\r\n")
		}
		// Shape 2: JSON envelope, e.g. {"session_id":"sess_..."}.
		var ctx struct {
			SessionID string `json:"session_id"`
		}
		if err := json.Unmarshal([]byte(val), &ctx); err == nil && ctx.SessionID != "" {
			return ctx.SessionID
		}
		// Shape 3: Chrome v10/v11 decrypt prepends a 32-byte sha256 integrity
		// hash before the actual value. Scan for the "sess_..." literal anywhere
		// in the bytes (the suffix is ASCII alphanumeric).
		if idx := strings.Index(val, "sess_"); idx >= 0 {
			rest := val[idx:]
			// Take the run of [A-Za-z0-9_:] starting at the match
			end := 0
			for end < len(rest) {
				ch := rest[end]
				if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == ':' {
					end++
					continue
				}
				break
			}
			// Strip trailing colon if present (observed in production cookies)
			if end > 0 && rest[end-1] == ':' {
				end--
			}
			if end > len("sess_") {
				return rest[:end]
			}
		}
	}
	return ""
}

// refreshClerkSession hits Clerk's session-tokens endpoint to mint a fresh
// __session JWT and updates the cookie jar in place. Safe to call
// concurrently — duplicate refreshes within 2s are collapsed.
func (c *Client) refreshClerkSession() error {
	if c.cookieAuth == nil {
		return errors.New("cookie auth not configured")
	}
	c.cookieAuth.mu.Lock()
	defer c.cookieAuth.mu.Unlock()

	// Collapse rapid duplicate refreshes — if we just refreshed, whoever
	// called us will get the fresh cookie from the jar on the next pass.
	if time.Since(c.cookieAuth.lastRefresh) < 2*time.Second {
		return nil
	}

	sessionID := c.clerkSessionID()
	if sessionID == "" {
		return errors.New("no clerk session id in cookie jar (missing clerk_active_context)")
	}

	refreshURL := fmt.Sprintf("%s/v1/client/sessions/%s/touch?__clerk_api_version=%s&_clerk_js_version=%s",
		clerkBaseURL, sessionID, clerkAPIVersion, clerkJSVersion)

	req, err := http.NewRequest("POST", refreshURL, strings.NewReader(""))
	if err != nil {
		return fmt.Errorf("build refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", "contact-goat-pp-cli/0.1.0")
	req.Header.Set("Origin", "https://happenstance.ai")
	req.Header.Set("Referer", "https://happenstance.ai/")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("clerk refresh: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		// Drain the body but surface Clerk's own reason headers — they
		// tell us exactly why the refresh failed (token expired, session
		// revoked, etc.) without exposing any JWT material.
		_, _ = io.Copy(io.Discard, resp.Body)
		reason := resp.Header.Get("X-Clerk-Auth-Reason")
		msg := resp.Header.Get("X-Clerk-Auth-Message")
		status := resp.Header.Get("X-Clerk-Auth-Status")
		if reason == "" && msg == "" {
			return fmt.Errorf("clerk refresh: HTTP %d", resp.StatusCode)
		}
		return fmt.Errorf("clerk refresh: HTTP %d (status=%s reason=%s): %s",
			resp.StatusCode, status, reason, msg)
	}

	// Drain the response body. /touch emits fresh __session via
	// Set-Cookie; we parse those headers directly rather than relying on
	// Go's cookiejar to scope them correctly. Without a PublicSuffixList
	// the jar sometimes stores Set-Cookie Domain=.happenstance.ai as
	// host-only for the request host (clerk.happenstance.ai), which
	// means subsequent POSTs to happenstance.ai/api/search don't carry
	// the fresh JWT and the server returns 204 "signed-out".
	_, _ = io.Copy(io.Discard, resp.Body)

	var freshSession string
	// resp.Cookies() parses Set-Cookie from the response headers.
	for _, sc := range resp.Cookies() {
		if sc.Name == "__session" && sc.Value != "" {
			freshSession = sc.Value
			break
		}
	}
	if freshSession == "" {
		// Fall back to whatever the jar stored; some Clerk paths rely on
		// the jar's own Set-Cookie handling even when the response-level
		// parse returns nothing.
		freshSession = c.sessionCookieValue()
	}
	if freshSession == "" {
		return errors.New("clerk refresh: 200 OK but no __session cookie in response or jar")
	}

	// Mirror the fresh __session across every host that subsequent
	// Happenstance requests will target. Path "/" so the jar matches
	// every /api/* endpoint.
	newCookie := &http.Cookie{
		Name:     "__session",
		Value:    freshSession,
		Path:     "/",
		Domain:   ".happenstance.ai",
		HttpOnly: true,
		Secure:   true,
	}
	for _, host := range []string{"happenstance.ai", "www.happenstance.ai", "clerk.happenstance.ai"} {
		u := &url.URL{Scheme: "https", Host: host, Path: "/"}
		c.cookieAuth.jar.SetCookies(u, []*http.Cookie{newCookie})
	}

	c.cookieAuth.lastRefresh = time.Now()
	return nil
}

// MaybeRefreshSession refreshes the Clerk session JWT if it's within the
// expiry slack window. Safe to call before every request.
func (c *Client) MaybeRefreshSession() error {
	if c.cookieAuth == nil {
		return nil
	}
	jwt := c.sessionCookieValue()
	if jwt == "" {
		return nil // no session yet — let the request fail and the 401 path refresh
	}
	if !IsJWTExpired(jwt) {
		return nil
	}
	return c.refreshClerkSession()
}

// HasCookieAuth reports whether WithCookieAuth has been applied.
func (c *Client) HasCookieAuth() bool {
	return c != nil && c.cookieAuth != nil
}
