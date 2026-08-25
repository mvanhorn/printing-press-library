package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/suppco/internal/client"
	"github.com/mvanhorn/printing-press-library/library/health/suppco/internal/config"
)

func TestConfigureClientRequiresBearerTokenBeforeRequests(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`{"products":[]}`))
	}))
	t.Cleanup(server.Close)

	c := client.New(&config.Config{BaseURL: server.URL}, time.Second, 0)
	if err := ConfigureClient(c, true); err == nil || !strings.Contains(err.Error(), "bearer token is required") {
		t.Fatalf("ConfigureClient() error = %v", err)
	}
	if requests.Load() != 0 {
		t.Fatalf("missing credentials triggered %d request(s)", requests.Load())
	}
}

func TestConfigureClientDoesNotRetainOrReplayResponseCookies(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if cookie := r.Header.Get("Cookie"); cookie != "" {
			t.Errorf("request replayed response cookie %q", cookie)
		}
		w.Header().Set("Set-Cookie", "synthetic_session=discard; Path=/; HttpOnly")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(server.Close)

	c := client.New(&config.Config{BaseURL: server.URL, AccessToken: "synthetic-token"}, time.Second, 0)
	if err := ConfigureClient(c, true); err != nil {
		t.Fatal(err)
	}
	if c.HTTPClient.Jar != nil {
		t.Fatal("SuppCo client retained a cookie jar")
	}
	for range 2 {
		if _, err := c.Get(context.Background(), "/stack", nil); err != nil {
			t.Fatal(err)
		}
	}
	if requests.Load() != 2 {
		t.Fatalf("requests = %d, want 2", requests.Load())
	}
}

func TestConfigureClientRejectsSavedCookieHeader(t *testing.T) {
	c := client.New(&config.Config{
		BaseURL:     APIOrigin,
		AccessToken: "synthetic-token",
		Headers:     map[string]string{"cOoKiE": "synthetic_session=discard"},
	}, time.Second, 0)
	if err := ConfigureClient(c, false); err == nil || !strings.Contains(err.Error(), "Cookie headers") {
		t.Fatalf("ConfigureClient() error = %v", err)
	}
}

func TestConfigureClientRejectsGenericAuthorizationHeader(t *testing.T) {
	c := client.New(&config.Config{
		BaseURL:     APIOrigin,
		AccessToken: "synthetic-token",
		Headers:     map[string]string{"aUtHoRiZaTiOn": "Bearer stale-synthetic-token"},
	}, time.Second, 0)
	if err := ConfigureClient(c, false); err == nil || !strings.Contains(err.Error(), "generic Authorization") {
		t.Fatalf("ConfigureClient() error = %v", err)
	}
}
