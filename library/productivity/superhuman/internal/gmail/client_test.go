// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package gmail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/auth"
)

// fakeServer is a small httptest harness that scripts an HTTP-status
// sequence and records the bearer token per request.
type fakeServer struct {
	srv         *httptest.Server
	statuses    []int
	bodies      []string
	cursor      atomic.Int32
	authHeaders []string
	requestN    atomic.Int32
}

func newFakeServer(t *testing.T, scripts ...scripted) *fakeServer {
	t.Helper()
	f := &fakeServer{}
	for _, s := range scripts {
		f.statuses = append(f.statuses, s.status)
		f.bodies = append(f.bodies, s.body)
	}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := int(f.cursor.Add(1)) - 1
		f.authHeaders = append(f.authHeaders, r.Header.Get("Authorization"))
		f.requestN.Add(1)
		if idx >= len(f.statuses) {
			http.Error(w, "out of scripted responses", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(f.statuses[idx])
		_, _ = w.Write([]byte(f.bodies[idx]))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

type scripted struct {
	status int
	body   string
}

// withBaseURL points the package-level BaseURL at the fake server for the
// duration of the test, restoring on cleanup.
func withBaseURL(t *testing.T, url string) {
	t.Helper()
	orig := BaseURL
	BaseURL = url
	t.Cleanup(func() { BaseURL = orig })
}

// newStore creates a fresh, file-backed auth store in a temp dir and
// pre-seeds one account so refresh tests can observe Upsert.
func newStore(t *testing.T) *auth.Store {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.json")
	store := auth.NewStoreAt(path)
	now := time.Now().UnixMilli()
	if _, err := store.Upsert("user@example.com", auth.AccountTokens{
		Type:           "google",
		AccessToken:    "ya29.initial",
		UserID:         "gid-001",
		UserExternalID: "user_initial",
		DeviceID:       "dev_initial",
		SuperhumanToken: auth.SuperhumanToken{
			Token:   "id-initial",
			Expires: now + int64(time.Hour/time.Millisecond),
		},
		LastUsedAt: now,
	}); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	return store
}

// TestDoWithRefresh_HappyPath_NoRefresh verifies the most-common path —
// first call returns 2xx, refresh seam is never invoked, AccessToken stays
// unchanged on the Client.
func TestDoWithRefresh_HappyPath_NoRefresh(t *testing.T) {
	fake := newFakeServer(t, scripted{status: 200, body: `{"ok":true}`})
	withBaseURL(t, fake.srv.URL)

	store := newStore(t)
	refreshes := 0
	c := New(store, "user@example.com", "gid-001", "ya29.initial")
	c.Refresh = func(ctx context.Context, email, gid string) (*auth.CookieAuthResult, error) {
		refreshes++
		return nil, errors.New("refresh should not fire")
	}

	err := c.GetJSON(context.Background(), "/test", nil)
	if err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if refreshes != 0 {
		t.Fatalf("refresh fired %d times, want 0", refreshes)
	}
	if got := fake.authHeaders[0]; got != "Bearer ya29.initial" {
		t.Fatalf("auth header = %q, want Bearer ya29.initial", got)
	}
	if c.AccessToken != "ya29.initial" {
		t.Fatalf("AccessToken should not change on 2xx, got %q", c.AccessToken)
	}
}

// TestDoWithRefresh_401Then200 verifies the headline recovery path.
func TestDoWithRefresh_401Then200(t *testing.T) {
	fake := newFakeServer(t,
		scripted{status: 401, body: `{"error":"unauthorized"}`},
		scripted{status: 200, body: `{"ok":true,"refreshed":true}`},
	)
	withBaseURL(t, fake.srv.URL)

	store := newStore(t)
	c := New(store, "user@example.com", "gid-001", "ya29.expired")
	c.Refresh = func(ctx context.Context, email, gid string) (*auth.CookieAuthResult, error) {
		return &auth.CookieAuthResult{
			Email:          email,
			GoogleID:       gid,
			AccessToken:    "ya29.fresh",
			IDToken:        "id-fresh",
			IDTokenExpires: time.Now().Add(time.Hour).UnixMilli(),
		}, nil
	}

	var out map[string]any
	err := c.GetJSON(context.Background(), "/test", &out)
	if err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if out["refreshed"] != true {
		t.Fatalf("decoded response missing refreshed=true: %v", out)
	}
	if c.AccessToken != "ya29.fresh" {
		t.Fatalf("Client.AccessToken should be updated to fresh value, got %q", c.AccessToken)
	}
	// Both calls visible on the fake.
	if got := fake.requestN.Load(); got != 2 {
		t.Fatalf("server saw %d requests, want 2", got)
	}
	if fake.authHeaders[0] != "Bearer ya29.expired" {
		t.Fatalf("first header = %q want Bearer ya29.expired", fake.authHeaders[0])
	}
	if fake.authHeaders[1] != "Bearer ya29.fresh" {
		t.Fatalf("second header = %q want Bearer ya29.fresh", fake.authHeaders[1])
	}
	// Store was upserted with the fresh token.
	got, exists, err := store.Get("user@example.com")
	if err != nil || !exists {
		t.Fatalf("store missing account after refresh: %v exists=%v", err, exists)
	}
	if got.AccessToken != "ya29.fresh" {
		t.Fatalf("store AccessToken = %q want ya29.fresh", got.AccessToken)
	}
}

// TestDoWithRefresh_429NotRefreshed gates the refresh path to 401 only.
// A 429 (rate-limit) must NOT trigger refresh, otherwise a rate-limited
// burst would spam the refresh endpoint.
func TestDoWithRefresh_429NotRefreshed(t *testing.T) {
	fake := newFakeServer(t, scripted{status: 429, body: `{"error":"too many"}`})
	withBaseURL(t, fake.srv.URL)

	refreshes := 0
	c := New(newStore(t), "user@example.com", "gid-001", "ya29.initial")
	c.Refresh = func(ctx context.Context, email, gid string) (*auth.CookieAuthResult, error) {
		refreshes++
		return nil, errors.New("refresh should not fire on 429")
	}

	err := c.GetJSON(context.Background(), "/test", nil)
	if err == nil {
		t.Fatalf("expected error on 429, got nil")
	}
	ok, status := IsAPI(err)
	if !ok || status != 429 {
		t.Fatalf("expected *APIError status=429, got err=%v ok=%v status=%d", err, ok, status)
	}
	if refreshes != 0 {
		t.Fatalf("refresh fired %d times on 429, want 0", refreshes)
	}
}

// TestDoWithRefresh_RefreshFails_ReturnsAuthError ensures a refresh-seam
// failure surfaces as *AuthError with the inner error preserved.
func TestDoWithRefresh_RefreshFails_ReturnsAuthError(t *testing.T) {
	fake := newFakeServer(t, scripted{status: 401, body: ``})
	withBaseURL(t, fake.srv.URL)

	c := New(newStore(t), "user@example.com", "gid-001", "ya29.expired")
	c.Refresh = func(ctx context.Context, email, gid string) (*auth.CookieAuthResult, error) {
		return nil, errors.New("keychain denied")
	}

	err := c.GetJSON(context.Background(), "/test", nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !IsAuth(err) {
		t.Fatalf("expected *AuthError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "refresh failed") {
		t.Fatalf("error %q missing refresh-failed reason", err.Error())
	}
	if !strings.Contains(err.Error(), "auth login --disk") {
		t.Fatalf("error %q missing remediation hint", err.Error())
	}
}

// TestDoWithRefresh_401Then401_CleanReAuthMessage covers the
// retry-also-401 path.
func TestDoWithRefresh_401Then401_CleanReAuthMessage(t *testing.T) {
	fake := newFakeServer(t,
		scripted{status: 401, body: ``},
		scripted{status: 401, body: ``},
	)
	withBaseURL(t, fake.srv.URL)

	c := New(newStore(t), "user@example.com", "gid-001", "ya29.expired")
	c.Refresh = func(ctx context.Context, email, gid string) (*auth.CookieAuthResult, error) {
		return &auth.CookieAuthResult{
			Email:       email,
			GoogleID:    gid,
			AccessToken: "ya29.also-expired",
		}, nil
	}

	err := c.GetJSON(context.Background(), "/test", nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !IsAuth(err) {
		t.Fatalf("expected *AuthError on second 401, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "still unauthorized after refresh") {
		t.Fatalf("error %q missing still-unauthorized reason", err.Error())
	}
	if got := fake.requestN.Load(); got != 2 {
		t.Fatalf("server saw %d requests, want exactly 2", got)
	}
}

// TestDoWithRefresh_PostBodyReplayed proves the retry path resends the same
// body on the second attempt (req.Body is single-use, so naively reusing
// the request loses the body).
func TestDoWithRefresh_PostBodyReplayed(t *testing.T) {
	receivedBodies := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := readAll(r.Body)
		receivedBodies = append(receivedBodies, string(b))
		if len(receivedBodies) == 1 {
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "ok"})
	}))
	defer srv.Close()
	withBaseURL(t, srv.URL)

	c := New(newStore(t), "user@example.com", "gid-001", "ya29.expired")
	c.Refresh = func(ctx context.Context, email, gid string) (*auth.CookieAuthResult, error) {
		return &auth.CookieAuthResult{AccessToken: "ya29.fresh", GoogleID: gid}, nil
	}

	var out map[string]string
	if err := c.PostJSON(context.Background(), "/x/modify", map[string]any{"addLabelIds": []string{"INBOX"}}, &out); err != nil {
		t.Fatalf("PostJSON: %v", err)
	}
	if len(receivedBodies) != 2 {
		t.Fatalf("server saw %d POSTs, want 2", len(receivedBodies))
	}
	if receivedBodies[0] != receivedBodies[1] {
		t.Fatalf("retry sent different body:\nfirst:  %s\nsecond: %s", receivedBodies[0], receivedBodies[1])
	}
	if !strings.Contains(receivedBodies[0], `"addLabelIds":["INBOX"]`) {
		t.Fatalf("body missing addLabelIds: %s", receivedBodies[0])
	}
}

// TestAPIError_AuthError_Format asserts both typed errors include the
// signal callers need (status code + remediation hint).
func TestAPIError_AuthError_Format(t *testing.T) {
	api := &APIError{Status: 404, Body: `{"error":"not found"}`}
	if !strings.Contains(api.Error(), "404") {
		t.Fatalf("APIError missing status: %s", api.Error())
	}
	auth1 := &AuthError{Reason: "refresh failed"}
	if !strings.Contains(auth1.Error(), "auth login --disk") {
		t.Fatalf("AuthError missing remediation: %s", auth1.Error())
	}
	auth2 := &AuthError{Reason: "refresh failed", Inner: errors.New("keychain")}
	if !strings.Contains(auth2.Error(), "keychain") {
		t.Fatalf("AuthError with Inner missing wrapped err: %s", auth2.Error())
	}
}

// readAll is a tiny helper so tests don't need to import io explicitly.
func readAll(r interface {
	Read([]byte) (int, error)
}) ([]byte, error) {
	var buf bytes.Buffer
	tmp := make([]byte, 4096)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if err != nil {
			if errors.Is(err, errEOFCompat()) || strings.Contains(err.Error(), "EOF") {
				return buf.Bytes(), nil
			}
			return buf.Bytes(), err
		}
	}
}

// errEOFCompat is io.EOF (declared here to avoid an explicit io import in
// this file — keeping the test file's dependency surface narrow).
var errEOFCompatVar = errors.New("EOF")

func errEOFCompat() error { return errEOFCompatVar }
