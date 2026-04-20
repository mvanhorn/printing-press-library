// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// testKey is a fake bearer token used in unit tests. The redaction tests grep
// for this literal string in captured output and assert it never leaks.
const testKey = "hpn_live_personal_TESTKEY_DO_NOT_LEAK_abc123"

// newTestClient wires a Client at the given httptest server URL using the
// fake test bearer key.
func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	return NewClient(testKey, WithBaseURL(srv.URL))
}

func TestClient_Me_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+testKey {
			t.Errorf("Authorization header = %q, want Bearer <key>", got)
		}
		if r.URL.Path != "/users/me" {
			t.Errorf("path = %q, want /users/me", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"email":"matt@example.com","name":"Matt VH","friends":[{"email":"a@b.co","name":"Alice"},{"email":"c@d.co","name":"Carol"}]}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	user, err := c.Me(context.Background())
	if err != nil {
		t.Fatalf("Me() error = %v", err)
	}
	if user.Email != "matt@example.com" {
		t.Errorf("Email = %q, want matt@example.com", user.Email)
	}
	if user.Name != "Matt VH" {
		t.Errorf("Name = %q, want Matt VH", user.Name)
	}
	if len(user.Friends) != 2 {
		t.Fatalf("len(Friends) = %d, want 2", len(user.Friends))
	}
	if user.Friends[0].Name != "Alice" {
		t.Errorf("Friends[0].Name = %q, want Alice", user.Friends[0].Name)
	}
}

func TestClient_Me_EmptyFriendsArray(t *testing.T) {
	// Edge case: empty friends array decodes as []User (non-nil) so that range
	// loops on user.Friends are safe without nil-checking.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"email":"solo@example.com","name":"Solo","friends":[]}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	user, err := c.Me(context.Background())
	if err != nil {
		t.Fatalf("Me() error = %v", err)
	}
	if user.Friends == nil {
		t.Fatal("Friends is nil; expected non-nil empty slice for safe range")
	}
	if len(user.Friends) != 0 {
		t.Errorf("len(Friends) = %d, want 0", len(user.Friends))
	}
	// Range loop should be a no-op without panicking.
	count := 0
	for range user.Friends {
		count++
	}
	if count != 0 {
		t.Errorf("range count = %d, want 0", count)
	}
}

func TestClient_Me_401IncludesEnvVarHintAndRotationURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":"invalid api key"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Me(context.Background())
	if err == nil {
		t.Fatal("expected error on 401, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "HAPPENSTANCE_API_KEY") {
		t.Errorf("error %q does not mention HAPPENSTANCE_API_KEY", msg)
	}
	if !strings.Contains(msg, "https://happenstance.ai") {
		t.Errorf("error %q does not include rotation URL https://happenstance.ai", msg)
	}
}

func TestClient_402_OutOfCreditsAndUsagePath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		io.WriteString(w, `{"error":"out of credits"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Me(context.Background())
	if err == nil {
		t.Fatal("expected error on 402, got nil")
	}
	msg := err.Error()
	if !strings.Contains(strings.ToLower(msg), "out of credits") {
		t.Errorf("error %q does not contain 'out of credits'", msg)
	}
	if !strings.Contains(msg, "/v1/usage") {
		t.Errorf("error %q does not reference /v1/usage", msg)
	}
}

func TestClient_429_TypedRateLimitError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"error":"rate limit reached"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Me(context.Background())
	if err == nil {
		t.Fatal("expected error on 429, got nil")
	}
	var rle *RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("error is not *RateLimitError; got %T: %v", err, err)
	}
	if rle.RetryAfterSeconds != 30 {
		t.Errorf("RetryAfterSeconds = %d, want 30", rle.RetryAfterSeconds)
	}
}

func TestClient_MalformedJSONIncludesBodyTail(t *testing.T) {
	// Simulate an HTML 502 page from a CDN sitting in front of the API. The
	// error must include the first chunk of the body so an operator can grok
	// what happened without needing to re-curl.
	htmlBody := `<html><head><title>502 Bad Gateway</title></head><body><h1>502 Bad Gateway</h1><p>nginx/1.18 says: upstream timed out (110: Connection timed out) while reading response header from upstream, client: 192.0.2.1, server: api.happenstance.ai, request: "GET /v1/users/me HTTP/1.1"</p></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadGateway)
		io.WriteString(w, htmlBody)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Me(context.Background())
	if err == nil {
		t.Fatal("expected error on 502, got nil")
	}
	msg := err.Error()
	// The first 200 bytes of the body should appear in the error so it is
	// actionable. We assert on a stable substring within that range.
	if !strings.Contains(msg, "502 Bad Gateway") {
		t.Errorf("error %q missing body excerpt '502 Bad Gateway'", msg)
	}
}

func TestClient_DryRun_RedactsBearerKey(t *testing.T) {
	// Stand up a server that fails the test if hit — dry-run must not send.
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	// Capture stderr via the standard Go testing pattern (pipe + restore).
	// Dry-run output goes to stderr to match internal/client/client.go's
	// convention and keep stdout clean for --json piping.
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w

	c := NewClient(testKey, WithBaseURL(srv.URL), WithDryRun(true))
	_, callErr := c.Me(context.Background())

	w.Close()
	os.Stderr = oldStderr

	if hit {
		t.Fatal("dry-run sent a real request to the server")
	}
	if callErr != nil {
		t.Fatalf("dry-run returned error: %v", callErr)
	}

	got, _ := io.ReadAll(r)
	out := string(got)

	// Literal redaction string must appear verbatim.
	if !strings.Contains(out, "Bearer <HAPPENSTANCE_API_KEY>") {
		t.Errorf("dry-run output missing literal redaction. Got:\n%s", out)
	}
	// The actual key value must NEVER appear.
	if strings.Contains(out, testKey) {
		t.Errorf("dry-run output LEAKED the bearer key value. Got:\n%s", out)
	}
}

func TestClient_WithBaseURL_RoutesToTestServer(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Path != "/users/me" {
			t.Errorf("path = %q, want /users/me", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"email":"x@y.z","name":"X","friends":[]}`)
	}))
	defer srv.Close()

	c := NewClient(testKey, WithBaseURL(srv.URL))
	if _, err := c.Me(context.Background()); err != nil {
		t.Fatalf("Me() error = %v", err)
	}
	if !called {
		t.Fatal("WithBaseURL did not route the request to the test server")
	}
	// Also confirm the default base URL is what the plan specifies.
	def := NewClient(testKey)
	if def.baseURL != "https://api.happenstance.ai/v1" {
		t.Errorf("default baseURL = %q, want https://api.happenstance.ai/v1", def.baseURL)
	}
}

// TestClient_NoKeyLeakageInAnyError loops every error path with the test key
// and asserts the literal value never appears in the returned error message.
// This is the contract-boundary lock-down: even if a future change adds a new
// error path, this test will catch a regression that includes the bearer key.
func TestClient_NoKeyLeakageInAnyError(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"401", http.StatusUnauthorized, `{"error":"invalid"}`},
		{"402", http.StatusPaymentRequired, `{"error":"no credits"}`},
		{"403", http.StatusForbidden, `{"error":"forbidden"}`},
		{"404", http.StatusNotFound, `{"error":"not found"}`},
		{"422", http.StatusUnprocessableEntity, `{"error":"bad input"}`},
		{"429", http.StatusTooManyRequests, `{"error":"rate limited"}`},
		{"500", http.StatusInternalServerError, `<html>internal error</html>`},
		{"503", http.StatusServiceUnavailable, `<html>unavailable</html>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			}))
			defer srv.Close()
			c := newTestClient(t, srv)
			_, err := c.Me(context.Background())
			if err == nil {
				t.Fatalf("expected error for status %d", tc.status)
			}
			if strings.Contains(err.Error(), testKey) {
				t.Errorf("error LEAKED the bearer key for status %d: %v", tc.status, err)
			}
		})
	}
}
