// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package source

import (
	"net/http"
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
