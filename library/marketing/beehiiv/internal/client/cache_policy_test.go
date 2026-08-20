package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/beehiiv/internal/config"
)

func TestGetFreshWithHeadersBypassesResponseCache(t *testing.T) {
	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"fresh"}`))
	}))
	defer server.Close()

	c := &Client{
		BaseURL:    server.URL,
		Config:     &config.Config{},
		HTTPClient: server.Client(),
		cacheDir:   filepath.Join(t.TempDir(), "cache"),
	}
	c.writeCache("/jwt_token", nil, json.RawMessage(`{"token":"stale"}`))

	got, err := c.GetFreshWithHeaders("/jwt_token", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"token":"fresh"}` {
		t.Fatalf("GetFreshWithHeaders returned %s, want fresh API response", got)
	}
	if hits != 1 {
		t.Fatalf("server hits = %d, want 1", hits)
	}

	cached, ok := c.readCache("/jwt_token", nil)
	if !ok {
		t.Fatal("expected stale cache entry to remain readable")
	}
	if string(cached) != `{"token":"stale"}` {
		t.Fatalf("cache was overwritten with %s, want stale entry unchanged", cached)
	}
}

func TestAuthHeaderErrorsWhenExpiredRefreshTokenCannotRefresh(t *testing.T) {
	c := &Client{
		Config: &config.Config{
			AccessToken:  "stale",
			RefreshToken: "refresh",
			TokenExpiry:  time.Now().Add(-time.Minute),
		},
	}

	_, err := c.authHeader()
	if err == nil {
		t.Fatal("authHeader returned nil error for expired token with unsupported refresh token")
	}
	if !strings.Contains(err.Error(), "no Beehiiv OAuth refresh endpoint") {
		t.Fatalf("authHeader error = %q, want unsupported refresh endpoint message", err)
	}
}
