// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/auth"
)

// fakeGmailServer is a small httptest.NewServer wrapper that records each
// inbound /users/me/messages/send request and answers per a configurable
// sequence of HTTP statuses. The sequence lets one test drive the
// 401->200, 401->401, 401-then-network-error paths without writing four
// near-identical fixtures.
type fakeGmailServer struct {
	srv         *httptest.Server
	statuses    []int          // pop-per-request status sequence
	cursor      atomic.Int32   // index into statuses
	authHeaders []string       // captured Authorization header per request
	requestN    atomic.Int32   // number of requests served
}

func newFakeGmailServer(t *testing.T, statuses ...int) *fakeGmailServer {
	t.Helper()
	f := &fakeGmailServer{statuses: statuses}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/users/me/messages/send") {
			http.Error(w, "wrong path: "+r.URL.Path, http.StatusNotFound)
			return
		}
		idx := int(f.cursor.Add(1)) - 1
		f.authHeaders = append(f.authHeaders, r.Header.Get("Authorization"))
		f.requestN.Add(1)
		if idx >= len(f.statuses) {
			http.Error(w, "out of scripted statuses", http.StatusInternalServerError)
			return
		}
		status := f.statuses[idx]
		w.WriteHeader(status)
		if status == http.StatusOK {
			_ = json.NewEncoder(w).Encode(map[string]string{
				"id":       fmt.Sprintf("gmail-id-%d", idx+1),
				"threadId": "thread-id",
			})
			return
		}
		_, _ = w.Write([]byte(`{"error":"scripted-status"}`))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// inputs returns a minimal sendInputs for a synthetic Gmail send. The exact
// body content doesn't matter for the refresh tests — only the HTTP status
// dance does.
func newRefreshTestInputs() sendInputs {
	return sendInputs{
		FromEmail: "user@example.com",
		To:        []string{"alice@example.com"},
		Subject:   "refresh test",
		Body:      "hello",
		DraftID:   "draft01234567890",
		Rfc822ID:  "<refresh-test@superhuman>",
		Now:       time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC),
	}
}

// withFakeGmail wires gmailAPIBaseURL to the fake server's URL for the
// duration of the test and restores it afterward.
func withFakeGmail(t *testing.T, baseURL string) {
	t.Helper()
	orig := gmailAPIBaseURL
	gmailAPIBaseURL = baseURL
	t.Cleanup(func() { gmailAPIBaseURL = orig })
}

// withGmailRefreshFn installs a stub refresh function for the duration of the
// test. fn may return an error to simulate refresh failure or a non-nil
// CookieAuthResult to simulate success.
func withGmailRefreshFn(t *testing.T, fn func(ctx context.Context, email, googleID string) (*auth.CookieAuthResult, error)) {
	t.Helper()
	orig := gmailRefreshFn
	gmailRefreshFn = fn
	t.Cleanup(func() { gmailRefreshFn = orig })
}

// TestSendGmailWithRefresh_HappyPath_NoRefresh covers the most-common path:
// first call returns 200, wrapper returns the gmail id, refresh never fires.
func TestSendGmailWithRefresh_HappyPath_NoRefresh(t *testing.T) {
	fake := newFakeGmailServer(t, http.StatusOK)
	withFakeGmail(t, fake.srv.URL+"/gmail/v1")

	refreshCalls := 0
	withGmailRefreshFn(t, func(ctx context.Context, email, googleID string) (*auth.CookieAuthResult, error) {
		refreshCalls++
		return nil, errors.New("refresh should not be called")
	})

	configPath, tokenStorePath := withConfigPath(t)
	_ = configPath
	seedSendStore(t, tokenStorePath, "user@example.com", "google-id-001")
	store := auth.NewStoreAt(tokenStorePath)

	var stderr bytes.Buffer
	id, err := sendGmailWithRefresh(context.Background(), &stderr, store,
		"user@example.com", "google-id-001",
		"ya29.initial-token", "Matt <user@example.com>", newRefreshTestInputs())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "gmail-id-1" {
		t.Fatalf("gmailID = %q want gmail-id-1", id)
	}
	if got := fake.requestN.Load(); got != 1 {
		t.Fatalf("fake server saw %d requests, want 1", got)
	}
	if refreshCalls != 0 {
		t.Fatalf("refresh fired %d times, want 0", refreshCalls)
	}
}

// TestSendGmailWithRefresh_401Then200_RefreshAndPersist covers the headline
// recovery path: first call 401, refresh succeeds, second call 200, store
// upserted with the fresh access token.
func TestSendGmailWithRefresh_401Then200_RefreshAndPersist(t *testing.T) {
	fake := newFakeGmailServer(t, http.StatusUnauthorized, http.StatusOK)
	withFakeGmail(t, fake.srv.URL+"/gmail/v1")

	refreshCalls := 0
	withGmailRefreshFn(t, func(ctx context.Context, email, googleID string) (*auth.CookieAuthResult, error) {
		refreshCalls++
		return &auth.CookieAuthResult{
			Email:          email,
			GoogleID:       googleID,
			IDToken:        "fresh-id-token",
			IDTokenExpires: time.Now().Add(time.Hour).UnixMilli(),
			AccessToken:    "ya29.fresh-token",
			ExternalID:     "user_refreshed",
			DeviceID:       "dev_refreshed",
		}, nil
	})

	configPath, tokenStorePath := withConfigPath(t)
	_ = configPath
	seedSendStore(t, tokenStorePath, "user@example.com", "google-id-001")
	store := auth.NewStoreAt(tokenStorePath)

	var stderr bytes.Buffer
	id, err := sendGmailWithRefresh(context.Background(), &stderr, store,
		"user@example.com", "google-id-001",
		"ya29.expired-token", "Matt <user@example.com>", newRefreshTestInputs())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "gmail-id-2" {
		t.Fatalf("gmailID = %q want gmail-id-2 (second request)", id)
	}
	if refreshCalls != 1 {
		t.Fatalf("refresh fired %d times, want exactly 1", refreshCalls)
	}
	// Authorization header on first request was the expired token; second
	// request used the refreshed token.
	if len(fake.authHeaders) != 2 {
		t.Fatalf("captured %d auth headers, want 2", len(fake.authHeaders))
	}
	if fake.authHeaders[0] != "Bearer ya29.expired-token" {
		t.Fatalf("first auth header = %q want Bearer ya29.expired-token", fake.authHeaders[0])
	}
	if fake.authHeaders[1] != "Bearer ya29.fresh-token" {
		t.Fatalf("second auth header = %q want Bearer ya29.fresh-token", fake.authHeaders[1])
	}
	// Store now contains the refreshed access token.
	got, exists, err := store.Get("user@example.com")
	if err != nil {
		t.Fatalf("store.Get: %v", err)
	}
	if !exists {
		t.Fatalf("account missing from store after refresh")
	}
	if got.AccessToken != "ya29.fresh-token" {
		t.Fatalf("store AccessToken = %q want ya29.fresh-token", got.AccessToken)
	}
	if got.SuperhumanToken.Token != "fresh-id-token" {
		t.Fatalf("store SuperhumanToken.Token = %q want fresh-id-token", got.SuperhumanToken.Token)
	}
}

// TestSendGmailWithRefresh_401ThenRefreshFails surfaces the refresh error to
// the caller without retrying the Gmail call a second time. The original 401
// detail is preserved inside the wrapper's error so debug output is useful.
func TestSendGmailWithRefresh_401ThenRefreshFails(t *testing.T) {
	fake := newFakeGmailServer(t, http.StatusUnauthorized)
	withFakeGmail(t, fake.srv.URL+"/gmail/v1")

	withGmailRefreshFn(t, func(ctx context.Context, email, googleID string) (*auth.CookieAuthResult, error) {
		return nil, errors.New("keychain prompt declined")
	})

	configPath, tokenStorePath := withConfigPath(t)
	_ = configPath
	seedSendStore(t, tokenStorePath, "user@example.com", "google-id-001")
	store := auth.NewStoreAt(tokenStorePath)

	var stderr bytes.Buffer
	_, err := sendGmailWithRefresh(context.Background(), &stderr, store,
		"user@example.com", "google-id-001",
		"ya29.expired-token", "Matt <user@example.com>", newRefreshTestInputs())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "refresh failed") {
		t.Fatalf("error %q does not mention refresh failure", err.Error())
	}
	if !strings.Contains(err.Error(), "auth login --disk") {
		t.Fatalf("error %q does not point user at auth login --disk", err.Error())
	}
	if got := fake.requestN.Load(); got != 1 {
		t.Fatalf("fake server saw %d requests, want exactly 1 (refresh-failed path must not retry)", got)
	}
}

// TestSendGmailWithRefresh_401Then401_CleanReAuthMessage covers the
// pathological case where the refresh succeeds but the new token also gets
// 401'd. The wrapper surfaces a clean "run auth login --disk" message rather
// than a confusing "still 401" error.
func TestSendGmailWithRefresh_401Then401_CleanReAuthMessage(t *testing.T) {
	fake := newFakeGmailServer(t, http.StatusUnauthorized, http.StatusUnauthorized)
	withFakeGmail(t, fake.srv.URL+"/gmail/v1")

	withGmailRefreshFn(t, func(ctx context.Context, email, googleID string) (*auth.CookieAuthResult, error) {
		return &auth.CookieAuthResult{
			Email:       email,
			GoogleID:    googleID,
			AccessToken: "ya29.also-expired",
			IDToken:     "fresh-id",
		}, nil
	})

	configPath, tokenStorePath := withConfigPath(t)
	_ = configPath
	seedSendStore(t, tokenStorePath, "user@example.com", "google-id-001")
	store := auth.NewStoreAt(tokenStorePath)

	var stderr bytes.Buffer
	_, err := sendGmailWithRefresh(context.Background(), &stderr, store,
		"user@example.com", "google-id-001",
		"ya29.expired-token", "Matt <user@example.com>", newRefreshTestInputs())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unauthorized after refresh") {
		t.Fatalf("error %q does not say unauthorized after refresh", err.Error())
	}
	if !strings.Contains(err.Error(), "auth login --disk") {
		t.Fatalf("error %q does not point user at auth login --disk", err.Error())
	}
	if got := fake.requestN.Load(); got != 2 {
		t.Fatalf("fake server saw %d requests, want exactly 2 (one original, one retry; no third attempt)", got)
	}
}

// TestSendGmailWithRefresh_429_NoRefresh proves the refresh path is gated on
// 401 specifically. A 429 (rate limit) or any non-401 4xx must surface as-is
// without firing a refresh — otherwise a runaway loop could spam the
// accounts.superhuman.com refresh endpoint.
func TestSendGmailWithRefresh_429_NoRefresh(t *testing.T) {
	fake := newFakeGmailServer(t, http.StatusTooManyRequests)
	withFakeGmail(t, fake.srv.URL+"/gmail/v1")

	refreshCalls := 0
	withGmailRefreshFn(t, func(ctx context.Context, email, googleID string) (*auth.CookieAuthResult, error) {
		refreshCalls++
		return nil, errors.New("refresh should not be called on 429")
	})

	configPath, tokenStorePath := withConfigPath(t)
	_ = configPath
	seedSendStore(t, tokenStorePath, "user@example.com", "google-id-001")
	store := auth.NewStoreAt(tokenStorePath)

	var stderr bytes.Buffer
	_, err := sendGmailWithRefresh(context.Background(), &stderr, store,
		"user@example.com", "google-id-001",
		"ya29.token", "Matt <user@example.com>", newRefreshTestInputs())
	if err == nil {
		t.Fatalf("expected error on 429, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 429") {
		t.Fatalf("error %q does not surface HTTP 429", err.Error())
	}
	if refreshCalls != 0 {
		t.Fatalf("refresh fired on 429 (got %d calls); refresh must be 401-only", refreshCalls)
	}
}

// TestSendGmailWithRefresh_RefreshReturnsEmptyAccessToken guards against the
// edge case where the refresh seam returns a non-nil result whose
// AccessToken is empty — surfaces as a clean re-auth message rather than
// silently retrying with "Bearer ".
func TestSendGmailWithRefresh_RefreshReturnsEmptyAccessToken(t *testing.T) {
	fake := newFakeGmailServer(t, http.StatusUnauthorized)
	withFakeGmail(t, fake.srv.URL+"/gmail/v1")

	withGmailRefreshFn(t, func(ctx context.Context, email, googleID string) (*auth.CookieAuthResult, error) {
		return &auth.CookieAuthResult{
			Email:    email,
			GoogleID: googleID,
			// AccessToken left empty on purpose.
		}, nil
	})

	configPath, tokenStorePath := withConfigPath(t)
	_ = configPath
	seedSendStore(t, tokenStorePath, "user@example.com", "google-id-001")
	store := auth.NewStoreAt(tokenStorePath)

	var stderr bytes.Buffer
	_, err := sendGmailWithRefresh(context.Background(), &stderr, store,
		"user@example.com", "google-id-001",
		"ya29.expired", "Matt <user@example.com>", newRefreshTestInputs())
	if err == nil {
		t.Fatalf("expected error when refresh returns empty AccessToken, got nil")
	}
	if !strings.Contains(err.Error(), "no access token") {
		t.Fatalf("error %q does not mention empty access token", err.Error())
	}
	if got := fake.requestN.Load(); got != 1 {
		t.Fatalf("fake server saw %d requests, want exactly 1 (no retry on empty access token)", got)
	}
}

// TestGmailAuthError_Format asserts the typed error's Error() string contains
// the status code so error wrapping at higher layers preserves the signal.
func TestGmailAuthError_Format(t *testing.T) {
	e := &gmailAuthError{Status: 401, Body: `{"error":{"code":401}}`}
	s := e.Error()
	if !strings.Contains(s, "401") {
		t.Fatalf("gmailAuthError.Error() = %q does not contain 401", s)
	}
}

// TestSendGmailWithRefresh_PersistFailure_StillReturnsSuccess proves the
// best-effort persist semantics: if store.Upsert fails for some reason
// (filesystem, etc.), the wrapper still returns the gmailID because the
// in-memory token worked. A warning is emitted to stderr so the user knows
// the on-disk state may be stale.
func TestSendGmailWithRefresh_PersistFailure_StillReturnsSuccess(t *testing.T) {
	fake := newFakeGmailServer(t, http.StatusUnauthorized, http.StatusOK)
	withFakeGmail(t, fake.srv.URL+"/gmail/v1")

	withGmailRefreshFn(t, func(ctx context.Context, email, googleID string) (*auth.CookieAuthResult, error) {
		return &auth.CookieAuthResult{
			Email:          email,
			GoogleID:       googleID,
			AccessToken:    "ya29.fresh-token",
			IDToken:        "fresh-id-token",
			IDTokenExpires: time.Now().Add(time.Hour).UnixMilli(),
		}, nil
	})

	// store == nil disables persistence — the wrapper must not crash and
	// must not fail the send. This mirrors how a future caller might pass
	// nil to skip persistence (e.g., a one-shot dry-run path).
	var stderr bytes.Buffer
	id, err := sendGmailWithRefresh(context.Background(), &stderr, nil,
		"user@example.com", "google-id-001",
		"ya29.expired", "Matt <user@example.com>", newRefreshTestInputs())
	if err != nil {
		t.Fatalf("nil store should not fail the send: %v", err)
	}
	if id == "" {
		t.Fatalf("expected non-empty gmailID")
	}
}

// nopReadCloser exists to silence the unused-import warning if io.NopCloser
// drifts; kept as a tiny safety net for future test helpers.
var _ io.ReadCloser = nopReadCloser{}

type nopReadCloser struct{}

func (nopReadCloser) Read(p []byte) (int, error) { return 0, io.EOF }
func (nopReadCloser) Close() error               { return nil }
