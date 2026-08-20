package client

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

// newTestClient wires a Client at the given test-server BaseURL with cache and
// dry-run disabled; OnTokenExpired left nil for the caller to set.
func newTestClient(t *testing.T, baseURL, initialAuth string) *Client {
	t.Helper()
	cfg := &config.Config{
		BaseURL:       baseURL,
		AccessToken:   strings.TrimPrefix(initialAuth, "Bearer "),
		AuthHeaderVal: "",
	}
	c := New(cfg, 5*time.Second, 0)
	c.cacheDir = "" // disable cache writes from tests
	return c
}

// statusOf extracts the HTTP status code from a client error. Returns 0 if
// the error isn't an APIError (transport failure, etc.).
func statusOf(err error) int {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode
	}
	return 0
}

func TestClient_AutoRefresh_RetriesAfter401(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		auth := r.Header.Get("Authorization")
		if n == 1 {
			if auth != "Bearer old" {
				t.Errorf("hit 1: got auth %q want Bearer old", auth)
			}
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"token expired (401)"}`))
			return
		}
		if auth != "Bearer fresh" {
			t.Errorf("hit 2: got auth %q want Bearer fresh", auth)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"response":{"ok":true}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, "Bearer old")
	var refreshCalled int32
	c.OnTokenExpired = func() (string, error) {
		atomic.AddInt32(&refreshCalled, 1)
		return "Bearer fresh", nil
	}

	body, err := c.Get("/api/1/products", nil)
	if err != nil {
		t.Fatalf("Get: %v (body=%s)", err, string(body))
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Errorf("server hits: got %d want 2", got)
	}
	if got := atomic.LoadInt32(&refreshCalled); got != 1 {
		t.Errorf("OnTokenExpired calls: got %d want 1", got)
	}
}

func TestClient_AutoRefresh_NoCallbackPasses401Through(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"token expired (401)"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, "Bearer old")
	c.OnTokenExpired = nil

	_, err := c.Get("/api/1/products", nil)
	if err == nil {
		t.Fatal("expected 401 error to bubble; got nil")
	}
	if got := statusOf(err); got != 401 {
		t.Errorf("status: got %d want 401", got)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("server hits: got %d want 1 (no retry when callback nil)", got)
	}
}

func TestClient_AutoRefresh_RefreshFailureBubblesOriginal401(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"token expired (401)"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, "Bearer old")
	c.OnTokenExpired = func() (string, error) {
		return "", errors.New("refresh token revoked")
	}

	_, err := c.Get("/api/1/products", nil)
	if err == nil {
		t.Fatal("expected error after failed refresh; got nil")
	}
	if got := statusOf(err); got != 401 {
		t.Errorf("status: got %d want 401 (original status bubbles)", got)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("server hits: got %d want 1 (no retry after refresh fails)", got)
	}
}

func TestClient_AutoRefresh_CapsAtOneRefreshPerRequest(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"token expired (401)"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, "Bearer old")
	var refreshCalled int32
	c.OnTokenExpired = func() (string, error) {
		atomic.AddInt32(&refreshCalled, 1)
		return "Bearer fresh", nil
	}

	_, err := c.Get("/api/1/products", nil)
	if err == nil {
		t.Fatal("expected persistent 401 error")
	}
	if got := statusOf(err); got != 401 {
		t.Errorf("status: got %d want 401", got)
	}
	// Server saw two hits (original + one refresh-retry). Refresh fired only once.
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Errorf("server hits: got %d want 2 (one retry only)", got)
	}
	if got := atomic.LoadInt32(&refreshCalled); got != 1 {
		t.Errorf("OnTokenExpired calls: got %d want 1 (capped)", got)
	}
}

func TestClient_AutoRefresh_DoesNotFireOn200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"response":{"ok":true}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, "Bearer old")
	var refreshCalled int32
	c.OnTokenExpired = func() (string, error) {
		atomic.AddInt32(&refreshCalled, 1)
		return "Bearer never", nil
	}

	_, err := c.Get("/api/1/products", nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got := atomic.LoadInt32(&refreshCalled); got != 0 {
		t.Errorf("OnTokenExpired calls: got %d want 0 (200 should not trigger)", got)
	}
}

func TestClient_AutoRefresh_DoesNotFireOn403(t *testing.T) {
	// 403 = signed-command-required, not token-expired; refresh should not fire.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"vehicle_command_protocol_required"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, "Bearer old")
	var refreshCalled int32
	c.OnTokenExpired = func() (string, error) {
		atomic.AddInt32(&refreshCalled, 1)
		return "Bearer never", nil
	}

	_, err := c.Get("/api/1/vehicles/123/command/honk_horn", nil)
	if err == nil {
		t.Fatal("expected 403 to bubble")
	}
	if got := statusOf(err); got != 403 {
		t.Errorf("status: got %d want 403", got)
	}
	if got := atomic.LoadInt32(&refreshCalled); got != 0 {
		t.Errorf("OnTokenExpired calls: got %d want 0 (403 should not trigger refresh)", got)
	}
}
