// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/config"
)

// urlParse is a test-only convenience wrapper around net/url.Parse so the
// host-helper table-test stays compact.
func urlParse(s string) (*url.URL, error) { return url.Parse(s) }

// authTestFixture wires together a temp-dir-backed token store, a mock
// Superhuman backend (httptest.Server), and a Client that uses both. Tests
// drive behaviour through:
//   - tokenIssued: counter incremented every time the mock refresh hook fires
//   - requests: counter incremented every time the mock backend receives a
//     request, so we can assert "one HTTP call, no refresh" vs "two HTTP
//     calls, one refresh" etc.
type authTestFixture struct {
	t            *testing.T
	store        *auth.Store
	server       *httptest.Server
	client       *Client
	cfg          *config.Config
	requests     int32
	refreshCalls int32
	// nextToken is what the mock refresh hook installs into the store on
	// each call. Default "refreshed.id.token.v1"; tests can mutate this
	// between scenarios.
	nextToken string
	// refreshErr, when non-nil, is returned by the injected RefreshFunc
	// instead of rotating the token. Used to simulate
	// ErrRefreshTokenExpired during pre-flight.
	refreshErr error
}

// statusSeq controls the response status the mock backend returns on each
// successive request. The fixture's handler pops the head of the slice and
// returns 200 by default once exhausted.
type statusSeq struct {
	codes []int
	idx   int32
}

func (s *statusSeq) next() int {
	i := int(atomic.AddInt32(&s.idx, 1)) - 1
	if i >= len(s.codes) {
		return http.StatusOK
	}
	return s.codes[i]
}

// newAuthFixture builds the test scaffolding. The seedToken is what the
// store starts with; seedExpiresAt is the SuperhumanToken expiry (epoch ms);
// statusCodes is the sequence of HTTP statuses the mock backend returns.
func newAuthFixture(t *testing.T, email, seedToken string, seedExpiresAt int64, statusCodes []int) *authTestFixture {
	t.Helper()

	storePath := filepath.Join(t.TempDir(), "tokens.json")
	store := auth.NewStoreAt(storePath)

	// Seed the store with one account whose token may be fresh or stale
	// depending on seedExpiresAt. RefreshToken is non-empty so the
	// production-path doRefresh (when not overridden) would have something
	// to send to Firebase, but every test in this file overrides
	// RefreshFunc so the value is just a marker.
	acct := auth.AccountTokens{
		Type:         "google",
		RefreshToken: "rt-original",
		Expires:      seedExpiresAt,
		SuperhumanToken: auth.SuperhumanToken{
			Token:   seedToken,
			Expires: seedExpiresAt,
		},
		LastUsedAt: time.Now().UnixMilli(),
	}
	if _, err := store.Upsert(email, acct); err != nil {
		t.Fatalf("seed Upsert: %v", err)
	}

	fx := &authTestFixture{
		t:         t,
		store:     store,
		nextToken: "refreshed.id.token.v1",
	}

	seq := &statusSeq{codes: statusCodes}

	fx.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&fx.requests, 1)
		status := seq.next()
		w.WriteHeader(status)
		// The Superhuman backend returns JSON; tests don't assert on the
		// body, but writing a minimal envelope keeps cache reads sane.
		w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(fx.server.Close)

	// Config is hand-constructed (not config.Load) so the test owns every
	// field. Path is set to a child of t.TempDir() because TokenStorePath
	// derives the store path from filepath.Dir(cfg.Path); pointing it at
	// the same dir as our seeded store wires them together without
	// touching $HOME or $XDG_CONFIG_HOME.
	fx.cfg = &config.Config{
		BaseURL:     fx.server.URL,
		Path:        filepath.Join(filepath.Dir(storePath), "config.toml"),
		ActiveEmail: email,
		RefreshFunc: func(ctx context.Context, em string, s *auth.Store) (string, error) {
			atomic.AddInt32(&fx.refreshCalls, 1)
			if fx.refreshErr != nil {
				return "", fx.refreshErr
			}
			// Rotate the SuperhumanToken in the store so the next
			// AuthHeader() picks up the fresh value. Mirrors what
			// auth.Refresh does on a real Firebase 200.
			a, ok, getErr := s.Get(em)
			if getErr != nil {
				return "", getErr
			}
			if !ok {
				return "", errors.New("mock refresh: account missing")
			}
			a.SuperhumanToken.Token = fx.nextToken
			a.SuperhumanToken.Expires = time.Now().Add(1 * time.Hour).UnixMilli()
			if _, err := s.Upsert(em, a); err != nil {
				return "", err
			}
			return fx.nextToken, nil
		},
	}

	fx.client = New(fx.cfg, 5*time.Second, 0)
	// Cache writes during these tests would leave files around and
	// occasionally short-circuit a second request. Tests own retry
	// behaviour; bypass the cache.
	fx.client.NoCache = true
	return fx
}

// TestAuthHappyPath_FreshToken: token is fresh (expires far in the future),
// server returns 200. Expect exactly one HTTP request and zero refresh calls.
func TestAuthHappyPath_FreshToken(t *testing.T) {
	t.Parallel()
	fx := newAuthFixture(t,
		"user2@example.com",
		"fresh.id.token",
		time.Now().Add(1*time.Hour).UnixMilli(),
		[]int{http.StatusOK},
	)

	body, err := fx.client.Get("/v3/test", nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !strings.Contains(string(body), `"ok":true`) {
		t.Fatalf("unexpected body: %s", body)
	}
	if got := atomic.LoadInt32(&fx.requests); got != 1 {
		t.Fatalf("requests: got %d want 1", got)
	}
	if got := atomic.LoadInt32(&fx.refreshCalls); got != 0 {
		t.Fatalf("refreshCalls: got %d want 0", got)
	}
}

// TestAuthPreflightRefresh: token expires within the 5-minute buffer (4
// minutes). The client should refresh BEFORE sending the request, then send
// exactly one HTTP request with the rotated token.
func TestAuthPreflightRefresh(t *testing.T) {
	t.Parallel()
	fx := newAuthFixture(t,
		"user2@example.com",
		"about.to.expire",
		time.Now().Add(4*time.Minute).UnixMilli(),
		[]int{http.StatusOK},
	)

	if _, err := fx.client.Get("/v3/test", nil); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got := atomic.LoadInt32(&fx.requests); got != 1 {
		t.Fatalf("requests: got %d want 1", got)
	}
	if got := atomic.LoadInt32(&fx.refreshCalls); got != 1 {
		t.Fatalf("refreshCalls: got %d want 1", got)
	}
	// The store should now have the rotated token; assert it survived.
	acct, ok, err := fx.store.Get("user2@example.com")
	if err != nil || !ok {
		t.Fatalf("post-refresh Get: ok=%v err=%v", ok, err)
	}
	if acct.SuperhumanToken.Token != "refreshed.id.token.v1" {
		t.Fatalf("token not rotated: %q", acct.SuperhumanToken.Token)
	}
}

// TestAuthMidFlight401: token looks fresh (expires in 1 hour) but the server
// returns 401 anyway (think clock skew or server-side rotation). The client
// should refresh and retry once, and the second attempt sees 200 → caller
// receives the 200 response, not the 401.
func TestAuthMidFlight401(t *testing.T) {
	t.Parallel()
	fx := newAuthFixture(t,
		"user2@example.com",
		"looks.fresh.but.rejected",
		time.Now().Add(1*time.Hour).UnixMilli(),
		[]int{http.StatusUnauthorized, http.StatusOK},
	)

	body, err := fx.client.Get("/v3/test", nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !strings.Contains(string(body), `"ok":true`) {
		t.Fatalf("unexpected body: %s", body)
	}
	if got := atomic.LoadInt32(&fx.requests); got != 2 {
		t.Fatalf("requests: got %d want 2", got)
	}
	if got := atomic.LoadInt32(&fx.refreshCalls); got != 1 {
		t.Fatalf("refreshCalls: got %d want 1", got)
	}
}

// TestAuthDouble401: the backend rejects auth even after a successful
// refresh. This is the account-level rejection case (revoked, banned).
// Expect ErrUnauthorized wrapping with the email.
func TestAuthDouble401(t *testing.T) {
	t.Parallel()
	fx := newAuthFixture(t,
		"user@example.com",
		"looks.fresh",
		time.Now().Add(1*time.Hour).UnixMilli(),
		[]int{http.StatusUnauthorized, http.StatusUnauthorized},
	)

	_, err := fx.client.Get("/v3/test", nil)
	if err == nil {
		t.Fatalf("Get: want error, got nil")
	}
	if !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("Get error: want ErrUnauthorized, got %v", err)
	}
	if !strings.Contains(err.Error(), "user@example.com") {
		t.Fatalf("error missing email: %v", err)
	}
	if got := atomic.LoadInt32(&fx.requests); got != 2 {
		t.Fatalf("requests: got %d want 2", got)
	}
	if got := atomic.LoadInt32(&fx.refreshCalls); got != 1 {
		t.Fatalf("refreshCalls: got %d want 1", got)
	}
}

// TestAuthPreflightRefreshTokenExpired: pre-flight refresh fails with
// ErrRefreshTokenExpired. No HTTP request should be made — the user must
// re-attach Chrome before any meaningful work can happen.
func TestAuthPreflightRefreshTokenExpired(t *testing.T) {
	t.Parallel()
	fx := newAuthFixture(t,
		"user2@example.com",
		"stale.token",
		time.Now().Add(2*time.Minute).UnixMilli(), // inside buffer → preflight fires
		[]int{http.StatusOK},                      // would succeed if sent, but shouldn't be
	)
	fx.refreshErr = auth.ErrRefreshTokenExpired

	_, err := fx.client.Get("/v3/test", nil)
	if err == nil {
		t.Fatalf("Get: want error, got nil")
	}
	if !errors.Is(err, auth.ErrRefreshTokenExpired) {
		t.Fatalf("Get error: want ErrRefreshTokenExpired, got %v", err)
	}
	if !strings.Contains(err.Error(), "user2@example.com") {
		t.Fatalf("error missing email: %v", err)
	}
	if got := atomic.LoadInt32(&fx.requests); got != 0 {
		t.Fatalf("requests: got %d want 0 (no HTTP attempt expected)", got)
	}
}

// TestAuthAccountNotFound: Config.ActiveEmail names an account that isn't in
// the store. The client should surface ErrAccountNotFound BEFORE sending any
// request, with the configured email in the error message.
func TestAuthAccountNotFound(t *testing.T) {
	t.Parallel()
	// Seed the store with one account, then point ActiveEmail at a
	// different one. The pre-flight resolver should catch the mismatch.
	fx := newAuthFixture(t,
		"present@example.com",
		"present.token",
		time.Now().Add(1*time.Hour).UnixMilli(),
		[]int{http.StatusOK},
	)
	fx.cfg.ActiveEmail = "missing@example.com"

	_, err := fx.client.Get("/v3/test", nil)
	if err == nil {
		t.Fatalf("Get: want error, got nil")
	}
	if !errors.Is(err, config.ErrAccountNotFound) {
		t.Fatalf("Get error: want ErrAccountNotFound, got %v", err)
	}
	if !strings.Contains(err.Error(), "missing@example.com") {
		t.Fatalf("error missing email: %v", err)
	}
	if got := atomic.LoadInt32(&fx.requests); got != 0 {
		t.Fatalf("requests: got %d want 0", got)
	}
}

// TestSuperhumanHeadersNotAttachedToTestServer: the fixture's httptest server
// is a 127.0.0.1 host, NOT *.superhuman.com. The header-attach helper must
// gate on host so the per-account identity metadata can't leak to mock
// servers, BaseURL overrides for printing-press verify, or unrelated services.
func TestSuperhumanHeadersNotAttachedToTestServer(t *testing.T) {
	t.Parallel()
	storePath := filepath.Join(t.TempDir(), "tokens.json")
	store := auth.NewStoreAt(storePath)
	if _, err := store.Upsert("user2@example.com", auth.AccountTokens{
		Type:           "google",
		UserID:         "106141970002595541286",
		UserExternalID: "user_11SzDnD012bqywr432",
		DeviceID:       "device-1234",
		SuperhumanToken: auth.SuperhumanToken{
			Token:   "fresh.id.token",
			Expires: time.Now().Add(1 * time.Hour).UnixMilli(),
		},
		LastUsedAt: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var capturedHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{
		BaseURL:     srv.URL,
		Path:        filepath.Join(filepath.Dir(storePath), "config.toml"),
		ActiveEmail: "user2@example.com",
	}
	c := New(cfg, 5*time.Second, 0)
	c.NoCache = true
	if _, err := c.Get("/v3/test", nil); err != nil {
		t.Fatalf("Get: %v", err)
	}
	for _, h := range []string{
		"x-superhuman-version",
		"x-superhuman-user-email",
		"x-superhuman-user-external-id",
		"x-superhuman-device-id",
		"x-superhuman-session-id",
		"x-superhuman-request-id",
	} {
		if v := capturedHeaders.Get(h); v != "" {
			t.Fatalf("non-superhuman host should not see %s; got %q", h, v)
		}
	}
}

// TestSuperhumanHostHelper validates the host-allowlist for the
// x-superhuman-* header attach. We don't drive a real superhuman.com request
// (would require network), but the helper is the single seam that decides
// whether to attach.
func TestSuperhumanHostHelper(t *testing.T) {
	t.Parallel()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://mail.superhuman.com/~backend/v3/x", true},
		{"https://accounts.superhuman.com/~backend/v3/x", true},
		{"https://superhuman.com/", true},
		{"http://127.0.0.1:8080/x", false},
		{"https://example.com/", false},
		{"https://superhuman.com.evil.com/", false},
		{"https://mail.superhuman.com:443/x", true},
	}
	for _, tc := range cases {
		u, err := urlParse(tc.url)
		if err != nil {
			t.Fatalf("parse %s: %v", tc.url, err)
		}
		got := isSuperhumanHost(u)
		if got != tc.want {
			t.Errorf("isSuperhumanHost(%s): got %v want %v", tc.url, got, tc.want)
		}
	}
}
