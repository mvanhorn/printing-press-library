// Copyright 2026 github-actionsbot and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: see .printing-press-patches/2026-08-30-nextauth-cookie-surface.md.

package client

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"testing"
)

func TestRequiresSessionBearerToken(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		targetURL string
		want      bool
	}{
		{"aisandbox-pa host needs the Bearer token", "https://aisandbox-pa.googleapis.com/v1/credits?key=x", true},
		{"labs.google host is cookie-authenticated, no Bearer token needed", "https://labs.google/fx/api/trpc/project.searchUserProjects", false},
		{"labs.google subpath still matches by host, not full URL", "https://labs.google/fx/tools/flow", false},
		{"a host that merely contains labs.google as a substring is not labs.google", "https://not-labs.google.evil.example/fx/api/trpc/project.searchUserProjects", true},
		{"malformed URL fails safe (requires the token)", "://not a url", true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := requiresSessionBearerToken(tc.targetURL)
			if got != tc.want {
				t.Fatalf("requiresSessionBearerToken(%q) = %v, want %v", tc.targetURL, got, tc.want)
			}
		})
	}
}

func TestSessionManagerHasCookieFor(t *testing.T) {
	t.Parallel()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	m := &SessionManager{jar: jar}

	u, err := url.Parse("https://labs.google/")
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	jar.SetCookies(u, []*http.Cookie{
		{Name: "__Secure-next-auth.session-token", Value: "abc123", Path: "/"},
	})

	if !m.HasCookieFor("https://labs.google/", "__Secure-next-auth.session-token") {
		t.Fatalf("expected the imported cookie to be found for the matching domain/name")
	}
	if m.HasCookieFor("https://labs.google/", "__Secure-next-auth.callback-url") {
		t.Fatalf("expected no match for a cookie name that was never set")
	}
	if m.HasCookieFor("https://aisandbox-pa.googleapis.com/v1", "__Secure-next-auth.session-token") {
		t.Fatalf("expected no match for a domain the cookie was never scoped to")
	}
}

func TestSessionManagerHasCookieForNilJar(t *testing.T) {
	t.Parallel()

	m := &SessionManager{}
	if m.HasCookieFor("https://labs.google/", "__Secure-next-auth.session-token") {
		t.Fatalf("expected false when the manager has no cookie jar")
	}
}

func TestSessionManagerHasCookieForMalformedURL(t *testing.T) {
	t.Parallel()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	m := &SessionManager{jar: jar}
	if m.HasCookieFor("://not a url", "anything") {
		t.Fatalf("expected false for a URL that fails to parse")
	}
}
