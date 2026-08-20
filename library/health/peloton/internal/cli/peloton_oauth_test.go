// Copyright 2026 Felix Banuchi and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
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
	if _, err := c.Get(context.Background(), "/catalog", nil); err == nil || !strings.Contains(err.Error(), "must be 2xx") {
		t.Fatalf("unexpected catalog error: %v", err)
	}
	if hits != 1 {
		t.Fatalf("catalog calls=%d, want 1", hits)
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
