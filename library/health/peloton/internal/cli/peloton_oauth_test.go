// Copyright 2026 Felix Banuchi and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/peloton/internal/client"
	"github.com/mvanhorn/printing-press-library/library/health/peloton/internal/config"
	"github.com/spf13/cobra"
)

func withOAuthTestState(t *testing.T) string {
	t.Helper()
	oldPath, oldNow, oldClient, oldURL := oauthBundlePath, oauthNow, oauthHTTPClient, oauthTokenURL
	path := filepath.Join(t.TempDir(), "private", "oauth-token.json")
	oauthBundlePath = func() (string, error) { return path, nil }
	oauthNow = func() time.Time { return time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(func() {
		oauthBundlePath, oauthNow, oauthHTTPClient, oauthTokenURL = oldPath, oldNow, oldClient, oldURL
	})
	return path
}

func TestManagedPelotonAccessTokenReusesValidBundle(t *testing.T) {
	withOAuthTestState(t)
	if err := saveOAuthBundle(pelotonTokenBundle{AccessToken: "fabricated-access", RefreshToken: "fabricated-refresh", ExpiresAt: oauthNow().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	oauthHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("valid bundle made a token request")
		return nil, nil
	})}
	got, err := managedPelotonAccessToken()
	if err != nil || got != "fabricated-access" {
		t.Fatalf("token=%q err=%v", got, err)
	}
}

func TestManagedOAuthUsesProvenPublicProviderDefaults(t *testing.T) {
	t.Setenv("PELOTON_OAUTH_CLIENT_ID", "")
	t.Setenv("PELOTON_OAUTH_REALM", "")
	t.Setenv("PELOTON_OAUTH_AUDIENCE", "")
	t.Setenv("PELOTON_OAUTH_SCOPE", "")
	if got := oauthClientID(); got != pelotonOAuthClientID {
		t.Fatalf("client id default mismatch")
	}
	if got := oauthRealm(); got != pelotonOAuthRealm {
		t.Fatalf("realm default mismatch")
	}
	if got := oauthAudience(); got != pelotonOAuthAudience {
		t.Fatalf("audience default mismatch")
	}
	if got := oauthScope(); got != pelotonOAuthScope {
		t.Fatalf("scope default mismatch")
	}
}

func TestManagedOAuthProviderOverridesRemainControlled(t *testing.T) {
	t.Setenv("PELOTON_OAUTH_CLIENT_ID", "public-test-client")
	t.Setenv("PELOTON_OAUTH_REALM", "public-test-realm")
	t.Setenv("PELOTON_OAUTH_AUDIENCE", "https://example.test/")
	t.Setenv("PELOTON_OAUTH_SCOPE", "test-scope")
	if oauthClientID() != "public-test-client" || oauthRealm() != "public-test-realm" || oauthAudience() != "https://example.test/" || oauthScope() != "test-scope" {
		t.Fatal("provider override was not honored")
	}
}

func TestManagedPelotonAccessTokenRefreshesOnceAndRetainsRotation(t *testing.T) {
	path := withOAuthTestState(t)
	if err := saveOAuthBundle(pelotonTokenBundle{AccessToken: "expired", RefreshToken: "refresh-before", ExpiresAt: oauthNow().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if err := r.ParseForm(); err != nil || r.Form.Get("grant_type") != "refresh_token" {
			t.Fatalf("unexpected refresh request: %v", err)
		}
		_, _ = w.Write([]byte(`{"access_token":"refreshed","refresh_token":"refresh-after","expires_in":3600}`))
	}))
	defer server.Close()
	oauthHTTPClient, oauthTokenURL = server.Client(), server.URL
	got, err := managedPelotonAccessToken()
	if err != nil || got != "refreshed" || calls != 1 {
		t.Fatalf("token=%q calls=%d err=%v", got, calls, err)
	}
	bundle, err := loadOAuthBundle()
	if err != nil || bundle.RefreshToken != "refresh-after" {
		t.Fatalf("bundle=%+v err=%v", bundle, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%v err=%v", info.Mode(), err)
	}
}

func TestManagedPelotonAccessTokenKeepsRefreshTokenWhenProviderDoesNotRotate(t *testing.T) {
	withOAuthTestState(t)
	if err := saveOAuthBundle(pelotonTokenBundle{AccessToken: "expired", RefreshToken: "refresh-before", ExpiresAt: oauthNow().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"refreshed","expires_in":3600}`))
	}))
	defer server.Close()
	t.Setenv("PELOTON_OAUTH_CLIENT_ID", "public-test-client")
	oauthHTTPClient, oauthTokenURL = server.Client(), server.URL
	if _, err := managedPelotonAccessToken(); err != nil {
		t.Fatal(err)
	}
	bundle, err := loadOAuthBundle()
	if err != nil || bundle.RefreshToken != "refresh-before" {
		t.Fatalf("bundle=%+v err=%v", bundle, err)
	}
}

func TestManagedPelotonAccessTokenBootstrapsAndRedactsProviderFailure(t *testing.T) {
	withOAuthTestState(t)
	t.Setenv("PELOTON_OAUTH_CLIENT_ID", "public-test-client")
	t.Setenv("PELOTON_OAUTH_REALM", "public-test-realm")
	t.Setenv("PELOTON_OAUTH_USERNAME", "fixture-user")
	t.Setenv("PELOTON_OAUTH_PASSWORD", "fixture-password")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil || r.Form.Get("grant_type") != pelotonOAuthGrant || r.Form.Get("realm") != "public-test-realm" || r.Form.Get("audience") != pelotonOAuthAudience || r.Form.Get("scope") != pelotonOAuthScope {
			t.Fatalf("unexpected bootstrap request: %v", err)
		}
		_, _ = w.Write([]byte(`{"access_token":"bootstrapped","refresh_token":"bootstrap-refresh","expires_in":3600}`))
	}))
	defer server.Close()
	oauthHTTPClient, oauthTokenURL = server.Client(), server.URL
	got, err := managedPelotonAccessToken()
	if err != nil || got != "bootstrapped" {
		t.Fatalf("token=%q err=%v", got, err)
	}

	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"fixture-secret-never-print"}`))
	}))
	defer failing.Close()
	oauthTokenURL = failing.URL
	if _, err := requestPelotonToken(map[string][]string{"client_id": {"public-test-client"}, "grant_type": {"refresh_token"}, "refresh_token": {"fixture-secret-never-print"}}); err == nil || strings.Contains(err.Error(), "fixture-secret-never-print") {
		t.Fatalf("unsafe provider error: %v", err)
	}
}

func TestManagedOAuthDoesNotFollowRedirects(t *testing.T) {
	withOAuthTestState(t)
	hits := 0
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		hits++
	}))
	defer target.Close()
	issuer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer issuer.Close()
	oauthTokenURL = issuer.URL
	if _, err := requestPelotonToken(map[string][]string{"client_id": {"public-test-client"}, "grant_type": {"refresh_token"}, "refresh_token": {"fixture-refresh"}}); err == nil {
		t.Fatal("redirected OAuth request unexpectedly succeeded")
	}
	if hits != 0 {
		t.Fatalf("redirect target received %d OAuth request(s)", hits)
	}
}

func TestManagedBearerOutranksPersistedAuthorization(t *testing.T) {
	withOAuthTestState(t)
	if err := saveOAuthBundle(pelotonTokenBundle{AccessToken: "managed-access", RefreshToken: "managed-refresh", ExpiresAt: oauthNow().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		AuthHeaderVal: "Basic stale-header",
		Headers: map[string]string{
			"Authorization":    "Basic stale-config-header",
			"Cookie":           "stale-cookie",
			"Peloton-Platform": "web",
		},
	}
	c := client.New(cfg, time.Second, 0)
	if err := installManagedPelotonBearer(c); err != nil {
		t.Fatal(err)
	}
	if got := cfg.AuthHeader(); got != "Bearer managed-access" || cfg.AuthHeaderVal != "" {
		t.Fatalf("managed bearer precedence failed: %q", got)
	}
	if _, ok := cfg.Headers["Authorization"]; ok {
		t.Fatal("stale authorization header was retained")
	}
	if _, ok := cfg.Headers["Cookie"]; ok {
		t.Fatal("stale cookie header was retained")
	}
	if cfg.Headers["Peloton-Platform"] != "web" {
		t.Fatal("Peloton-Platform header was not preserved")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer managed-access" {
			t.Fatalf("authorization=%q", got)
		}
		if got := r.Header.Get("Cookie"); got != "" {
			t.Fatalf("cookie=%q", got)
		}
		if got := r.Header.Get("Peloton-Platform"); got != "web" {
			t.Fatalf("Peloton-Platform=%q", got)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	c.BaseURL = server.URL
	c.NoCache = true
	if _, err := c.Get(context.Background(), "/catalog", nil); err != nil {
		t.Fatal(err)
	}
}

func TestManagedCatalogRejectsNon2xxWithoutRetry(t *testing.T) {
	withOAuthTestState(t)
	if err := saveOAuthBundle(pelotonTokenBundle{AccessToken: "managed-access", RefreshToken: "managed-refresh", ExpiresAt: oauthNow().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	c := client.New(&config.Config{BaseURL: server.URL}, time.Second, 0)
	if err := installManagedPelotonBearer(c); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get(context.Background(), "/catalog", nil); err == nil || !strings.Contains(err.Error(), "must not be a redirect") {
		t.Fatalf("unexpected catalog error: %v", err)
	}
	if hits != 1 {
		t.Fatalf("catalog calls=%d, want 1", hits)
	}
}

// TestManagedCatalogPreservesRealStatusCodeFor4xx guards a fix for a live
// gap found investigating a stale-refresh-token report: an earlier version
// of pelotonTwoXXRoundTripper masked every non-2xx response (redirects and
// ordinary 4xx/5xx API errors alike) into a generic "must be 2xx" error,
// discarding the real status code before client.Client.do() could build
// its usual *client.APIError. tools.go's status-specific error messages
// (permission denied, auth failed, ...) match on strings.Contains(err,
// "HTTP 403") and siblings -- with the mask in place those branches could
// never fire for any managed Peloton request, regardless of what Peloton's
// real API actually returned. Only 3xx (the shape the roundtripper exists
// to catch, since do()'s own success check incorrectly treats a 3xx as
// success too) should still be intercepted.
func TestManagedCatalogPreservesRealStatusCodeFor4xx(t *testing.T) {
	withOAuthTestState(t)
	if err := saveOAuthBundle(pelotonTokenBundle{AccessToken: "managed-access", RefreshToken: "managed-refresh", ExpiresAt: oauthNow().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"expired token"}`))
	}))
	defer server.Close()
	c := client.New(&config.Config{BaseURL: server.URL}, time.Second, 0)
	if err := installManagedPelotonBearer(c); err != nil {
		t.Fatal(err)
	}
	_, err := c.Get(context.Background(), "/workouts", nil)
	if err == nil {
		t.Fatal("expected an error for a 403 response")
	}
	if !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("expected the real status code to survive (tools.go matches on \"HTTP 403\"), got: %v", err)
	}
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("expected a *client.APIError with StatusCode 403, got: %v", err)
	}
}

func TestManagedOAuthCommandsReplaceManualBearerCommands(t *testing.T) {
	root := newRootCmd(&rootFlags{})
	var auth *cobra.Command
	for _, cmd := range root.Commands() {
		if cmd.Name() == "auth" {
			auth = cmd
			break
		}
	}
	if auth == nil {
		t.Fatal("auth command missing")
	}
	for _, child := range auth.Commands() {
		if child.Name() == "set-token" {
			t.Fatal("manual bearer command was retained")
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestManagedSessionCookieAttachedFromCache(t *testing.T) {
	withOAuthTestState(t)
	if err := saveOAuthBundle(pelotonTokenBundle{AccessToken: "managed-access", RefreshToken: "managed-refresh", ExpiresAt: oauthNow().Add(time.Hour), SessionID: "cached-session"}); err != nil {
		t.Fatal(err)
	}
	oauthHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("cached session made a login request")
		return nil, nil
	})}
	cfg := &config.Config{}
	c := client.New(cfg, time.Second, 0)
	if err := installManagedPelotonBearer(c); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Headers["Cookie"]; got != "peloton_session_id=cached-session" {
		t.Fatalf("cookie=%q", got)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Cookie"); got != "peloton_session_id=cached-session" {
			t.Fatalf("cookie=%q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer managed-access" {
			t.Fatalf("authorization=%q", got)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	c.BaseURL = server.URL
	c.NoCache = true
	if _, err := c.Get(context.Background(), "/api/me", nil); err != nil {
		t.Fatal(err)
	}
}

func TestManagedSessionCookieBootstrapsViaRealLogin(t *testing.T) {
	withOAuthTestState(t)
	if err := saveOAuthBundle(pelotonTokenBundle{AccessToken: "managed-access", RefreshToken: "managed-refresh", ExpiresAt: oauthNow().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PELOTON_OAUTH_USERNAME", "fixture-user")
	t.Setenv("PELOTON_OAUTH_PASSWORD", "fixture-password")

	loginServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/login" {
			t.Fatalf("unexpected login path: %s", r.URL.Path)
		}
		var body pelotonLoginRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.UsernameOrEmail != "fixture-user" || body.Password != "fixture-password" {
			t.Fatalf("unexpected login body: %+v", body)
		}
		if got := r.Header.Get("Peloton-Platform"); got != "web" {
			t.Fatalf("Peloton-Platform=%q", got)
		}
		_, _ = w.Write([]byte(`{"session_id":"fresh-session"}`))
	}))
	defer loginServer.Close()
	oauthHTTPClient = loginServer.Client()

	cfg := &config.Config{BaseURL: loginServer.URL}
	c := client.New(cfg, time.Second, 0)
	if err := installManagedPelotonBearer(c); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Headers["Cookie"]; got != "peloton_session_id=fresh-session" {
		t.Fatalf("cookie=%q", got)
	}

	bundle, err := loadOAuthBundle()
	if err != nil || bundle.SessionID != "fresh-session" || bundle.AccessToken != "managed-access" {
		t.Fatalf("bundle=%+v err=%v", bundle, err)
	}
}

func TestManagedSessionCookieBootstrapFromSetCookieHeader(t *testing.T) {
	withOAuthTestState(t)
	if err := saveOAuthBundle(pelotonTokenBundle{AccessToken: "managed-access", RefreshToken: "managed-refresh", ExpiresAt: oauthNow().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PELOTON_OAUTH_USERNAME", "fixture-user")
	t.Setenv("PELOTON_OAUTH_PASSWORD", "fixture-password")

	loginServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "peloton_session_id", Value: "cookie-shaped-session"})
		_, _ = w.Write([]byte(`{}`))
	}))
	defer loginServer.Close()
	oauthHTTPClient = loginServer.Client()

	cfg := &config.Config{BaseURL: loginServer.URL}
	c := client.New(cfg, time.Second, 0)
	if err := installManagedPelotonBearer(c); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Headers["Cookie"]; got != "peloton_session_id=cookie-shaped-session" {
		t.Fatalf("cookie=%q", got)
	}
}

// TestManagedSessionCookieMissingCredsIsNonFatal ensures a session that
// can't be bootstrapped yet (no env creds, nothing cached) never fails the
// command — only the bearer-only transport that already works today is used.
func TestManagedSessionCookieMissingCredsIsNonFatal(t *testing.T) {
	withOAuthTestState(t)
	if err := saveOAuthBundle(pelotonTokenBundle{AccessToken: "managed-access", RefreshToken: "managed-refresh", ExpiresAt: oauthNow().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	oauthHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("no credentials should not attempt a login request")
		return nil, nil
	})}
	cfg := &config.Config{}
	c := client.New(cfg, time.Second, 0)
	if err := installManagedPelotonBearer(c); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Headers["Cookie"]; ok {
		t.Fatalf("cookie unexpectedly set: %q", cfg.Headers["Cookie"])
	}
}

// TestManagedBearerRefreshPreservesCachedSessionID guards the bundle-rebuild
// path in managedPelotonAccessToken: a routine bearer refresh/bootstrap must
// not silently drop an already-cached session cookie.
func TestManagedBearerRefreshPreservesCachedSessionID(t *testing.T) {
	path := withOAuthTestState(t)
	if err := saveOAuthBundle(pelotonTokenBundle{AccessToken: "expired", RefreshToken: "refresh-before", ExpiresAt: oauthNow().Add(-time.Minute), SessionID: "kept-session"}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"refreshed","refresh_token":"refresh-after","expires_in":3600}`))
	}))
	defer server.Close()
	oauthHTTPClient, oauthTokenURL = server.Client(), server.URL
	if _, err := managedPelotonAccessToken(); err != nil {
		t.Fatal(err)
	}
	bundle, err := loadOAuthBundle()
	if err != nil || bundle.SessionID != "kept-session" {
		t.Fatalf("bundle=%+v err=%v path=%s", bundle, err, path)
	}
}

// TestManagedPelotonAccessTokenFallsBackToBootstrapWhenRefreshTokenIsDead
// guards the fix for a live report: a container's refresh_token had gone
// permanently invalid (provider-side rotation from concurrent CLI usage
// under the same account, most likely), and every call failed with
// "managed Peloton OAuth request failed with HTTP 403" for over a week —
// with PELOTON_OAUTH_USERNAME/PASSWORD configured, a fresh login could
// have recovered automatically, but the old code only ever tried
// bootstrapPelotonToken() when the bundle had no refresh_token at all, not
// when a refresh attempt failed. A dead refresh_token now falls back to a
// fresh bootstrap login when one is possible.
func TestManagedPelotonAccessTokenFallsBackToBootstrapWhenRefreshTokenIsDead(t *testing.T) {
	withOAuthTestState(t)
	if err := saveOAuthBundle(pelotonTokenBundle{AccessToken: "expired", RefreshToken: "dead-refresh", ExpiresAt: oauthNow().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PELOTON_OAUTH_USERNAME", "fixture-user")
	t.Setenv("PELOTON_OAUTH_PASSWORD", "fixture-password")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parsing token request: %v", err)
		}
		if r.Form.Get("grant_type") == "refresh_token" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"bootstrapped-after-dead-refresh","refresh_token":"fresh-refresh","expires_in":3600}`))
	}))
	defer server.Close()
	oauthHTTPClient, oauthTokenURL = server.Client(), server.URL

	got, err := managedPelotonAccessToken()
	if err != nil || got != "bootstrapped-after-dead-refresh" {
		t.Fatalf("token=%q err=%v", got, err)
	}
	bundle, err := loadOAuthBundle()
	if err != nil || bundle.RefreshToken != "fresh-refresh" {
		t.Fatalf("bundle=%+v err=%v", bundle, err)
	}
}

// TestManagedPelotonAccessTokenReturnsOriginalRefreshErrorWithoutBootstrapCreds
// guards the exact scenario from the live report: no
// PELOTON_OAUTH_USERNAME/PASSWORD configured at all, so there is no
// automatic recovery possible — the caller must see the original,
// diagnostic refresh failure (not a generic "bootstrap credentials
// unavailable" that would obscure what actually failed).
func TestManagedPelotonAccessTokenReturnsOriginalRefreshErrorWithoutBootstrapCreds(t *testing.T) {
	withOAuthTestState(t)
	if err := saveOAuthBundle(pelotonTokenBundle{AccessToken: "expired", RefreshToken: "dead-refresh", ExpiresAt: oauthNow().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer server.Close()
	oauthHTTPClient, oauthTokenURL = server.Client(), server.URL

	_, err := managedPelotonAccessToken()
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("expected the original refresh failure (HTTP 403) to survive with no bootstrap creds configured, got: %v", err)
	}
}

// TestManagedPelotonAccessTokenKeepsOriginalRefreshErrorWhenBootstrapAlsoFails
// guards the case where bootstrap credentials are configured but are
// themselves wrong (or Peloton also rejects them) — the more diagnostic
// original refresh error should survive, not get replaced by the
// bootstrap attempt's own failure.
func TestManagedPelotonAccessTokenKeepsOriginalRefreshErrorWhenBootstrapAlsoFails(t *testing.T) {
	withOAuthTestState(t)
	if err := saveOAuthBundle(pelotonTokenBundle{AccessToken: "expired", RefreshToken: "dead-refresh", ExpiresAt: oauthNow().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PELOTON_OAUTH_USERNAME", "fixture-user")
	t.Setenv("PELOTON_OAUTH_PASSWORD", "fixture-password")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer server.Close()
	oauthHTTPClient, oauthTokenURL = server.Client(), server.URL

	_, err := managedPelotonAccessToken()
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("expected the original refresh failure to survive when bootstrap also fails, got: %v", err)
	}
}
