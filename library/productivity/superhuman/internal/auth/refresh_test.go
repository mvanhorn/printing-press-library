// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/cliutil"
)

// newRefreshStore returns a freshly-initialized Store at t.TempDir() with one
// account that has a refresh token. Tests mutate the store via Refresh and
// then re-Get to assert the persisted state.
func newRefreshStore(t *testing.T, email, refreshToken string) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tokens.json")
	s := NewStoreAt(path)
	acct := AccountTokens{
		Type:         "google",
		RefreshToken: refreshToken,
		Expires:      time.Now().UnixMilli() + 30_000, // any non-zero value
		SuperhumanToken: SuperhumanToken{
			Token:   "old.id.token",
			Expires: time.Now().UnixMilli() + 30_000,
		},
		LastUsedAt: 1_000,
	}
	if _, err := s.Upsert(email, acct); err != nil {
		t.Fatalf("seed Upsert: %v", err)
	}
	return s
}

// firebaseOKBody is the canonical happy-path response shape. Note that
// expires_in is a STRING — Firebase returns it stringified, not as a number.
const firebaseOKBody = `{
	"id_token": "new.id.token.value",
	"refresh_token": "rt-same",
	"expires_in": "3600",
	"user_id": "uid-123",
	"project_id": "superhuman-prod",
	"token_type": "Bearer"
}`

// TestRefreshHappyPath: 200 + valid body → new token returned, store updated,
// expiry reflects now + (3600 - 60) seconds within a ±10s tolerance.
func TestRefreshHappyPath(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s want POST", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type: got %q want application/x-www-form-urlencoded", got)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "grant_type=refresh_token") {
			t.Errorf("body missing grant_type: %s", body)
		}
		if !strings.Contains(string(body), "refresh_token=rt-original") {
			t.Errorf("body missing refresh_token: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, firebaseOKBody)
	}))
	defer srv.Close()

	email := "user2@example.com"
	store := newRefreshStore(t, email, "rt-original")

	beforeMs := time.Now().UnixMilli()
	got, err := RefreshWithClient(context.Background(), email, store, srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("RefreshWithClient: %v", err)
	}
	afterMs := time.Now().UnixMilli()

	if got != "new.id.token.value" {
		t.Errorf("returned token: got %q want %q", got, "new.id.token.value")
	}

	// Confirm persistence: re-load and check fields.
	acct, ok, err := store.Get(email)
	if err != nil {
		t.Fatalf("Get after refresh: %v", err)
	}
	if !ok {
		t.Fatalf("account missing after refresh")
	}
	if acct.SuperhumanToken.Token != "new.id.token.value" {
		t.Errorf("persisted token: got %q", acct.SuperhumanToken.Token)
	}

	// Expiry = now + (3600 - 60) * 1000 ms within a ±10s window centered
	// on the observed wall-clock range.
	wantLo := beforeMs + (3600-60)*1000 - 10_000
	wantHi := afterMs + (3600-60)*1000 + 10_000
	if acct.SuperhumanToken.Expires < wantLo || acct.SuperhumanToken.Expires > wantHi {
		t.Errorf("expiry: got %d, want in [%d, %d] (now+3540s ±10s)",
			acct.SuperhumanToken.Expires, wantLo, wantHi)
	}

	// LastUsedAt should be bumped to roughly now.
	if acct.LastUsedAt < beforeMs || acct.LastUsedAt > afterMs+1_000 {
		t.Errorf("LastUsedAt: got %d, want in [%d, %d]", acct.LastUsedAt, beforeMs, afterMs+1_000)
	}

	// Refresh token unchanged because the server returned the same value.
	if acct.RefreshToken != "rt-original" && acct.RefreshToken != "rt-same" {
		t.Errorf("refresh token: got %q, want rt-original or rt-same", acct.RefreshToken)
	}
}

// TestRefreshRotation: response includes a NEW refresh_token differing from
// the input → persisted RefreshToken updated to the new value.
func TestRefreshRotation(t *testing.T) {
	t.Parallel()

	rotatedBody := `{
		"id_token": "new.id.rotated",
		"refresh_token": "rt-rotated-NEW",
		"expires_in": "3600",
		"user_id": "uid-123",
		"project_id": "superhuman-prod",
		"token_type": "Bearer"
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, rotatedBody)
	}))
	defer srv.Close()

	email := "user@example.com"
	store := newRefreshStore(t, email, "rt-original")

	if _, err := RefreshWithClient(context.Background(), email, store, srv.Client(), srv.URL); err != nil {
		t.Fatalf("RefreshWithClient: %v", err)
	}

	acct, ok, err := store.Get(email)
	if err != nil || !ok {
		t.Fatalf("Get after rotation: ok=%v err=%v", ok, err)
	}
	if acct.RefreshToken != "rt-rotated-NEW" {
		t.Errorf("rotated refresh token not persisted: got %q want %q",
			acct.RefreshToken, "rt-rotated-NEW")
	}
	if acct.SuperhumanToken.Token != "new.id.rotated" {
		t.Errorf("rotated id token not persisted: got %q", acct.SuperhumanToken.Token)
	}
}

// TestRefreshTokenExpired: Firebase returns 400 with TOKEN_EXPIRED →
// ErrRefreshTokenExpired returned, store NOT modified.
func TestRefreshTokenExpired(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		body string
	}{
		{name: "TOKEN_EXPIRED", body: `{"error":{"code":400,"message":"TOKEN_EXPIRED","status":"INVALID_ARGUMENT"}}`},
		{name: "INVALID_REFRESH_TOKEN", body: `{"error":{"code":400,"message":"INVALID_REFRESH_TOKEN","status":"INVALID_ARGUMENT"}}`},
		{name: "USER_DISABLED", body: `{"error":{"code":400,"message":"USER_DISABLED","status":"INVALID_ARGUMENT"}}`},
		{name: "USER_NOT_FOUND", body: `{"error":{"code":400,"message":"USER_NOT_FOUND","status":"INVALID_ARGUMENT"}}`},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()

			email := "test@example.com"
			store := newRefreshStore(t, email, "rt-expired")

			_, err := RefreshWithClient(context.Background(), email, store, srv.Client(), srv.URL)
			if !errors.Is(err, ErrRefreshTokenExpired) {
				t.Fatalf("err: got %v want ErrRefreshTokenExpired", err)
			}

			// Store should NOT have been updated.
			acct, ok, gerr := store.Get(email)
			if gerr != nil || !ok {
				t.Fatalf("Get after expired refresh: ok=%v err=%v", ok, gerr)
			}
			if acct.SuperhumanToken.Token != "old.id.token" {
				t.Errorf("token mutated after error path: got %q want old.id.token", acct.SuperhumanToken.Token)
			}
		})
	}
}

// TestRefreshRateLimitedThenSuccess: first call returns 429, second call
// returns 200 → limiter backs off, retry succeeds, returned token is from
// the second response.
func TestRefreshRateLimitedThenSuccess(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "0") // skip the wait
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":{"code":429,"message":"RATE_LIMITED"}}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"id_token": "after.retry.token",
			"refresh_token": "rt-after",
			"expires_in": "3600",
			"user_id": "uid",
			"project_id": "p"
		}`)
	}))
	defer srv.Close()

	email := "rl@example.com"
	store := newRefreshStore(t, email, "rt-initial")

	got, err := RefreshWithClient(context.Background(), email, store, srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("RefreshWithClient (with retry): %v", err)
	}
	if got != "after.retry.token" {
		t.Errorf("post-retry token: got %q want %q", got, "after.retry.token")
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("server calls: got %d want 2", n)
	}
}

// TestRefresh5xxPersistent: every call returns 500 → wrapped error mentions
// "firebase refresh", store not modified.
func TestRefresh5xxPersistent(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":{"code":500,"message":"upstream borked"}}`)
	}))
	defer srv.Close()

	email := "fivex@example.com"
	store := newRefreshStore(t, email, "rt-5xx")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := RefreshWithClient(ctx, email, store, srv.Client(), srv.URL)
	if err == nil {
		t.Fatalf("err: got nil, want a wrapped 5xx error")
	}
	if !strings.Contains(err.Error(), "firebase refresh") {
		t.Errorf("err message: got %q, want substring 'firebase refresh'", err.Error())
	}
	// Should not be ErrRefreshTokenExpired — 5xx is transient.
	if errors.Is(err, ErrRefreshTokenExpired) {
		t.Errorf("5xx mapped to ErrRefreshTokenExpired; want generic wrap")
	}

	acct, ok, _ := store.Get(email)
	if !ok || acct.SuperhumanToken.Token != "old.id.token" {
		t.Errorf("store mutated after 5xx: token=%q ok=%v", acct.SuperhumanToken.Token, ok)
	}
}

// TestRefreshNetworkError: target server closed before request → wrapped
// error mentions "firebase refresh", carries underlying transport message.
func TestRefreshNetworkError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	endpoint := srv.URL
	srv.Close() // shut down before any request lands

	email := "netfail@example.com"
	store := newRefreshStore(t, email, "rt-net")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := RefreshWithClient(ctx, email, store, &http.Client{Timeout: 2 * time.Second}, endpoint)
	if err == nil {
		t.Fatalf("err: got nil, want a wrapped network error")
	}
	if !strings.Contains(err.Error(), "firebase refresh") {
		t.Errorf("err message: got %q, want substring 'firebase refresh'", err.Error())
	}
}

// TestRefreshNoRefreshTokenInStore: account has no refresh token (legacy
// bare-JWT case from `auth set-token`) → ErrRefreshTokenExpired, store
// untouched. This is the U3 contract: bare-JWT accounts can't auto-refresh
// and the user must re-attach Chrome.
func TestRefreshNoRefreshTokenInStore(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "tokens.json")
	store := NewStoreAt(path)
	email := "legacy@example.com"
	if _, err := store.Upsert(email, AccountTokens{
		Type: "google",
		// RefreshToken intentionally empty
		SuperhumanToken: SuperhumanToken{Token: "bare.jwt", Expires: 1},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, err := RefreshWithClient(context.Background(), email, store, http.DefaultClient, "http://unused.invalid")
	if !errors.Is(err, ErrRefreshTokenExpired) {
		t.Fatalf("err: got %v want ErrRefreshTokenExpired", err)
	}
}

// TestRefreshAccountNotFound: email isn't in the store → typed-shaped error
// mentioning the email, not ErrRefreshTokenExpired (which would be the wrong
// remediation — the user needs to know the account isn't recorded at all).
func TestRefreshAccountNotFound(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "tokens.json")
	store := NewStoreAt(path)

	_, err := RefreshWithClient(context.Background(), "missing@example.com", store, http.DefaultClient, "http://unused.invalid")
	if err == nil {
		t.Fatalf("err: got nil, want account-not-found error")
	}
	if errors.Is(err, ErrRefreshTokenExpired) {
		t.Errorf("missing account mapped to ErrRefreshTokenExpired; want distinct error")
	}
	if !strings.Contains(err.Error(), "missing@example.com") {
		t.Errorf("err missing email context: %v", err)
	}
}

// TestRefreshRateLimitErrorTypeOnPersistent: simulate persistent 429s by
// asserting that the package surface uses cliutil.RateLimitError after
// retries are exhausted. This pins the AGENTS.md per-source-rate-limiting
// contract: empty-on-throttle is forbidden, the typed error must be
// surfaceable.
func TestRefreshRateLimitErrorTypeOnPersistent(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":{"code":429,"message":"RATE_LIMITED"}}`)
	}))
	defer srv.Close()

	email := "rl-persistent@example.com"
	store := newRefreshStore(t, email, "rt-rl")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := RefreshWithClient(ctx, email, store, srv.Client(), srv.URL)
	if err == nil {
		t.Fatalf("err: got nil, want persistent 429 to surface")
	}
	var rl *cliutil.RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("err type: got %T (%v), want *cliutil.RateLimitError", err, err)
	}
}
