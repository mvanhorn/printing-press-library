// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package source

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestCookiesIsZero(t *testing.T) {
	if !(Cookies{}).IsZero() {
		t.Fatal("empty Cookies should be zero")
	}
	if (Cookies{Sid: "x"}).IsZero() {
		t.Fatal("Cookies with sid should not be zero")
	}
	if (Cookies{CfClearance: "x"}).IsZero() {
		t.Fatal("Cookies with cf_clearance should not be zero")
	}
}

func TestCookiesHeader(t *testing.T) {
	tests := []struct {
		name string
		in   Cookies
		want string
	}{
		{"empty", Cookies{}, ""},
		{"sid only", Cookies{Sid: "abc"}, "sid=abc"},
		{"uid only", Cookies{Uid: "u1"}, "uid=u1"},
		{"sid+uid", Cookies{Sid: "abc", Uid: "u1"}, "sid=abc; uid=u1"},
		{"all three", Cookies{Sid: "abc", Uid: "u1", CfClearance: "cf"}, "sid=abc; uid=u1; cf_clearance=cf"},
		{"cf only", Cookies{CfClearance: "cf"}, "cf_clearance=cf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.Header(); got != tt.want {
				t.Fatalf("Header() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAttachCookiesSetsHeader(t *testing.T) {
	req, _ := http.NewRequest("GET", "https://medium.com/p/abc", nil)
	AttachCookies(req, Cookies{Sid: "abc", Uid: "u1"})
	if got := req.Header.Get("Cookie"); got != "sid=abc; uid=u1" {
		t.Fatalf("Cookie header = %q, want %q", got, "sid=abc; uid=u1")
	}
}

func TestAttachCookiesNoOpWhenZero(t *testing.T) {
	req, _ := http.NewRequest("GET", "https://medium.com/p/abc", nil)
	AttachCookies(req, Cookies{})
	if got := req.Header.Get("Cookie"); got != "" {
		t.Fatalf("Cookie header should be empty for zero cookies, got %q", got)
	}
}

func TestAttachCookiesNilRequest(t *testing.T) {
	// Must not panic on a nil request.
	if got := AttachCookies(nil, Cookies{Sid: "x"}); got != nil {
		t.Fatal("AttachCookies(nil, ...) should return nil")
	}
}

func TestGraphQLHeaders(t *testing.T) {
	req, _ := http.NewRequest("POST", "https://medium.com/_/graphql", strings.NewReader("{}"))
	GraphQLHeaders(req)
	checks := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json",
		"Origin":       "https://medium.com",
		"Referer":      "https://medium.com/",
	}
	for k, want := range checks {
		if got := req.Header.Get(k); got != want {
			t.Fatalf("header %s = %q, want %q", k, got, want)
		}
	}
}

// TestNewHTTPClientBuilds is a smoke test: the Surf Chrome-impersonation
// builder must produce a usable *http.Client without panicking and with the
// configured Timeout propagated. No network is touched.
func TestNewHTTPClientBuilds(t *testing.T) {
	hc := NewHTTPClient(45 * time.Second)
	if hc == nil {
		t.Fatal("NewHTTPClient returned nil")
	}
	if hc.Timeout != 45*time.Second {
		t.Fatalf("Timeout = %v, want 45s", hc.Timeout)
	}
}

func TestNewHTTPClientStripsCredentialsAcrossOrigins(t *testing.T) {
	hc := NewHTTPClient(30 * time.Second)
	if hc.CheckRedirect == nil {
		t.Fatal("NewHTTPClient must install a redirect credential policy")
	}

	orig, err := http.NewRequest(http.MethodGet, "https://medium.com/p/abc123", nil)
	if err != nil {
		t.Fatal(err)
	}
	orig.Header.Set("Cookie", "sid=synthetic-test-sid; uid=synthetic-test-uid")
	next, err := http.NewRequest(http.MethodGet, "https://uxdesign.cc/some-post-abc123", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, header := range []string{"Authorization", "Proxy-Authorization", "Www-Authenticate", "Cookie", "Cookie2"} {
		next.Header.Set(header, "synthetic-secret")
	}

	if err := hc.CheckRedirect(next, []*http.Request{orig}); err != nil {
		t.Fatalf("CheckRedirect returned error: %v", err)
	}

	for _, header := range []string{"Authorization", "Proxy-Authorization", "Www-Authenticate", "Cookie", "Cookie2"} {
		if got := next.Header.Get(header); got != "" {
			t.Fatalf("%s crossed an origin boundary: %q", header, got)
		}
	}
}

func TestNewHTTPClientDoesNotSendCookieToCrossOriginRedirect(t *testing.T) {
	received := make(chan string, 1)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.Header.Get("Cookie")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()

	targetURL, err := url.Parse(target.URL)
	if err != nil {
		t.Fatal(err)
	}
	redirectTarget := "http://localhost:" + targetURL.Port() + "/target"
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget, http.StatusFound)
	}))
	defer source.Close()

	req, err := http.NewRequest(http.MethodGet, source.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	AttachCookies(req, Cookies{Sid: "synthetic-test-sid", Uid: "synthetic-test-uid"})

	resp, err := NewHTTPClient(10 * time.Second).Do(req)
	if err != nil {
		t.Fatalf("redirected request failed: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	select {
	case got := <-received:
		if got != "" {
			t.Fatalf("cross-origin target received Cookie %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cross-origin target did not receive the redirected request")
	}
}

func TestNewHTTPClientKeepsCookieOnSameOriginRedirect(t *testing.T) {
	hc := NewHTTPClient(30 * time.Second)
	orig, err := http.NewRequest(http.MethodGet, "https://medium.com/p/abc123", nil)
	if err != nil {
		t.Fatal(err)
	}
	next, err := http.NewRequest(http.MethodGet, "https://medium.com/article/abc123", nil)
	if err != nil {
		t.Fatal(err)
	}
	next.Header.Set("Cookie", "sid=synthetic-test-sid")

	if err := hc.CheckRedirect(next, []*http.Request{orig}); err != nil {
		t.Fatalf("CheckRedirect returned error: %v", err)
	}
	if got := next.Header.Get("Cookie"); got != "sid=synthetic-test-sid" {
		t.Fatalf("same-origin Cookie = %q, want it preserved", got)
	}
}

func TestNewHTTPClientSendsCookieThroughSameOriginRedirect(t *testing.T) {
	received := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			http.Redirect(w, r, "/target", http.StatusFound)
		case "/target":
			received <- r.Header.Get("Cookie")
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/start", nil)
	if err != nil {
		t.Fatal(err)
	}
	AttachCookies(req, Cookies{Sid: "synthetic-test-sid"})

	resp, err := NewHTTPClient(10 * time.Second).Do(req)
	if err != nil {
		t.Fatalf("redirected request failed: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	select {
	case got := <-received:
		if got != "sid=synthetic-test-sid" {
			t.Fatalf("same-origin target received Cookie %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("same-origin target did not receive the redirected request")
	}
}
