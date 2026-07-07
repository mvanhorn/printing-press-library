// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/config"
)

func testConfigWithBaseURL(t *testing.T, baseURL string) *config.Config {
	t.Helper()
	return &config.Config{BaseURL: baseURL, SunoJwt: "t"}
}

// studioHostRedirect routes every request to the given test server's address
// while preserving the original request (so the studio-host gate in
// sunoRoundTripper fires and the Cookie header is injected on the wire).
type studioHostRedirect struct {
	base http.RoundTripper
	addr string
}

func (r studioHostRedirect) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = r.addr
	return r.base.RoundTrip(req)
}

func TestDo_RepullsCookiesOn422TokenValidation(t *testing.T) {
	var calls int
	var cookies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		cookies = append(cookies, r.Header.Get("Cookie"))
		if calls == 1 {
			w.WriteHeader(422)
			_, _ = w.Write([]byte(`{"detail":"token_validation_failed"}`))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	// BaseURL is the real studio host so the transport's host-gated cookie
	// injection fires; a redirecting base transport dials the test server.
	c := New(testConfigWithBaseURL(t, "https://studio-api-prod.suno.com"), 0, 0)
	srvAddr := strings.TrimPrefix(srv.URL, "http://")
	c.HTTPClient.Transport = studioHostRedirect{base: http.DefaultTransport, addr: srvAddr}
	InstallSunoTransport(c, "dev", "stale=1")
	refreshed := 0
	c.SetStudioCookieRefresher(func() string { refreshed++; return "fresh=2" })

	_, status, err := c.do(context.Background(), http.MethodPost, "/api/feed/v3", nil, map[string]any{"limit": 1}, nil)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	if status != 200 {
		t.Fatalf("status = %d, want 200 after cookie re-pull retry", status)
	}
	if refreshed != 1 {
		t.Errorf("refresher called %d times, want 1", refreshed)
	}
	if got := c.sunoRT.getCookieHeader(); got != "fresh=2" {
		t.Errorf("transport cookie = %q, want fresh=2", got)
	}
	if len(cookies) < 2 || cookies[1] != "fresh=2" {
		t.Errorf("second request Cookie = %v, want fresh=2 on retry", cookies)
	}
}

func TestDo_DoesNotRepullCookiesOnGenerate422(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(422)
		_, _ = w.Write([]byte(`{"detail":"token_validation_failed"}`))
	}))
	defer srv.Close()
	c := New(testConfigWithBaseURL(t, srv.URL), 0, 0)
	InstallSunoTransport(c, "dev", "stale=1")
	refreshed := 0
	c.SetStudioCookieRefresher(func() string { refreshed++; return "fresh=2" })
	_, status, _ := c.do(context.Background(), http.MethodPost, "/api/generate/v2-web/", nil, map[string]any{"x": 1}, nil)
	if status != 422 {
		t.Fatalf("status = %d, want 422 (generate must NOT cookie-retry)", status)
	}
	if refreshed != 0 {
		t.Errorf("refresher called %d times on generate, want 0", refreshed)
	}
}
