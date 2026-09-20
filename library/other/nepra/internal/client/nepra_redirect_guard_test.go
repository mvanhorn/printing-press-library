// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Tests for the redirect guard.
//
// TestClientRefusesRedirectToAnInternalHost is the one that matters: the unit
// tests below exercise nepraRedirectAllowed directly and would all still pass
// if the CALL SITE in the generated client.go were lost to a regen. Only the
// end-to-end test drives a real redirect through a real client. This is the
// same lesson the gzip patch recorded — unit tests on the helper did not
// catch a dropped call site, and a mutation proved it.

package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/config"
)

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parsing %q: %v", raw, err)
	}
	return u
}

func viaChain(t *testing.T, urls ...string) []*http.Request {
	t.Helper()
	out := make([]*http.Request, 0, len(urls))
	for _, raw := range urls {
		out = append(out, &http.Request{URL: mustParseURL(t, raw)})
	}
	return out
}

func TestNepraRedirectAllowed(t *testing.T) {
	origin := "https://nepra.org.pk/a"
	cases := []struct {
		name    string
		next    string
		via     []string
		refused string // substring of the expected error; "" means allowed
	}{
		{"same origin https", "https://nepra.org.pk/b", []string{origin}, ""},
		{"other public https host", "https://example.org/b", []string{origin}, ""},
		{"downgrade to http", "http://nepra.org.pk/b", []string{origin}, "protocol downgrade"},
		{"downgrade off origin", "http://example.org/b", []string{origin}, "protocol downgrade"},
		{"non-http scheme", "file:///etc/passwd", []string{origin}, "only http and https"},
		{"loopback v4", "https://127.0.0.1/b", []string{origin}, "loopback"},
		{"loopback v6", "https://[::1]/b", []string{origin}, "loopback"},
		{"private 10/8", "https://10.0.0.5/b", []string{origin}, "private"},
		{"private 192.168", "https://192.168.1.1:8080/b", []string{origin}, "private"},
		{"link-local metadata", "https://169.254.169.254/latest/meta-data", []string{origin}, "link-local"},
		{"unspecified", "https://0.0.0.0/b", []string{origin}, "unspecified"},
		// The operator chose this host themselves; a self-redirect is not SSRF.
		{"self-redirect on a chosen loopback origin", "http://127.0.0.1:8080/b",
			[]string{"http://127.0.0.1:8080/a"}, ""},
		// A plaintext run that was never https has no downgrade to suffer.
		{"http throughout", "http://example.org/b", []string{"http://example.org/a"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := nepraRedirectAllowed(mustParseURL(t, tc.next), viaChain(t, tc.via...))
			if tc.refused == "" {
				if err != nil {
					t.Fatalf("redirect to %s was refused: %v", tc.next, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("redirect to %s was ALLOWED; want refusal mentioning %q", tc.next, tc.refused)
			}
			if !strings.Contains(err.Error(), tc.refused) {
				t.Fatalf("refusal for %s = %q, want it to mention %q", tc.next, err, tc.refused)
			}
		})
	}
}

// TestClientRefusesRedirectToAnInternalHost drives a real redirect through a
// real client, so it FAILS if the call site in the generated client.go is
// ever lost. The unit tests above cannot catch that.
//
// The client is built through New() and its HTTPClient is NOT replaced:
// New() is where CheckRedirect is installed, so swapping in
// server.Client() — as several tests in this package legitimately do —
// would discard the very behaviour under test.
func TestClientRefusesRedirectToAnInternalHost(t *testing.T) {
	// Stands in for an internal target. Its address is a loopback literal,
	// which is what the guard refuses on a hop that leaves the origin.
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("SECRET INTERNAL BODY"))
	}))
	defer internal.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, internal.URL+"/stolen", http.StatusFound)
	}))
	defer origin.Close()
	// Reach the origin as "localhost" so its Host differs from the
	// 127.0.0.1 literal the redirect names; otherwise the hop is
	// same-origin and is legitimately allowed.
	originURL := strings.Replace(origin.URL, "127.0.0.1", "localhost", 1)

	// 30s, not 5s: the client classifies a CheckRedirect refusal as a
	// network error and retries it three times with 1s/2s/4s backoff, so a
	// short deadline fires first and the surfaced error becomes "context
	// deadline exceeded" instead of the refusal. The refusal is still what
	// stopped the request either way; the longer budget just lets the real
	// reason reach the caller so this test can assert on it. (That a policy
	// refusal is retried at all is generic generated-client behaviour,
	// shared with the pre-existing "stopped after 10 redirects" error.)
	c := New(&config.Config{BaseURL: originURL}, 30*time.Second, 0)
	c.NoCache = true

	body, err := c.Get(context.Background(), "/start", nil)
	if err == nil {
		t.Fatalf("client FOLLOWED a redirect to an internal address and returned %d bytes: %q",
			len(body), truncateForTest(string(body)))
	}
	if strings.Contains(string(body), "SECRET INTERNAL BODY") {
		t.Fatalf("the internal body reached the caller: %q", string(body))
	}
	if !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("refusal = %v, want it to name the loopback address", err)
	}
}

// TestClientStillFollowsASameOriginRedirect is the other half: the guard must
// not have broken ordinary redirects. Without this, refusing everything would
// pass the test above.
func TestClientStillFollowsASameOriginRedirect(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, srv.URL+"/final", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := New(&config.Config{BaseURL: srv.URL}, 5*time.Second, 0)
	c.NoCache = true

	body, err := c.Get(context.Background(), "/start", nil)
	if err != nil {
		t.Fatalf("a same-origin redirect was refused: %v", err)
	}
	if !strings.Contains(string(body), `"ok":true`) {
		t.Fatalf("body after redirect = %q", string(body))
	}
}

func truncateForTest(s string) string {
	if len(s) > 120 {
		return s[:120] + "..."
	}
	return s
}
