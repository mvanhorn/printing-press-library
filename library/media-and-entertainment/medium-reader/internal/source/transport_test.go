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

// TestNewHTTPClientForwardsCookieAcrossRedirect is the regression guard for the
// multi-domain read fix. read canonicalises every article to
// https://medium.com/p/<id>, which Medium 302-redirects to the post's canonical
// host. For a custom-domain publication that host is a DIFFERENT registrable
// domain (uxdesign.cc, uxplanet.org, …), and Go's stdlib strips the sensitive
// Cookie header on that cross-domain hop — so the Tier-1 session never reached
// the custom host and member posts came back as the anonymous preview.
//
// NewHTTPClient enables Surf's ForwardHeadersOnRedirect, which installs (via
// .Std()) a CheckRedirect that re-copies the original request's headers onto
// each redirect hop. Because stdlib runs CheckRedirect AFTER its sensitive-header
// strip, this restores the Cookie. We assert that behaviour directly on the
// returned client's CheckRedirect: the redirect request starts with no Cookie
// (as stdlib would leave it) and must come out carrying the original cookie. If
// someone drops the ForwardHeadersOnRedirect() call, the cookie is not copied
// and this test fails. No network and no real cookie value is used.
func TestNewHTTPClientForwardsCookieAcrossRedirect(t *testing.T) {
	hc := NewHTTPClient(30 * time.Second)
	if hc.CheckRedirect == nil {
		t.Fatal("NewHTTPClient must install a CheckRedirect that forwards headers across redirects")
	}

	const wantCookie = "sid=synthetic-test-sid; uid=synthetic-test-uid"

	// Original request to the medium.com short link, carrying the Tier-1 cookie
	// exactly as AttachCookies would have set it.
	orig, err := http.NewRequest(http.MethodGet, "https://medium.com/p/abc123", nil)
	if err != nil {
		t.Fatal(err)
	}
	orig.Header.Set("Cookie", wantCookie)

	// The upcoming redirect hop to a different registrable domain. Stdlib would
	// have stripped the sensitive Cookie here, so it starts empty — the exact
	// state CheckRedirect receives.
	next, err := http.NewRequest(http.MethodGet, "https://uxdesign.cc/some-post-abc123", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := next.Header.Get("Cookie"); got != "" {
		t.Fatalf("precondition: cross-domain redirect request should start with no Cookie, got %q", got)
	}

	if err := hc.CheckRedirect(next, []*http.Request{orig}); err != nil {
		t.Fatalf("CheckRedirect returned error: %v", err)
	}

	if got := next.Header.Get("Cookie"); got != wantCookie {
		t.Fatalf("Cookie not forwarded across the cross-domain redirect: got %q, want %q "+
			"(the multi-domain read bug — custom-domain member posts return the anonymous preview)", got, wantCookie)
	}
}
