// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/client"
)

// The Next.js dashboard has no csrf-token meta tag; checkout and cancel send
// the URL-decoded XSRF-TOKEN cookie as X-CSRF-Token instead.
func TestXSRFCookieTokenDecodesCookie(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	base, _ := url.Parse("https://www.taskrabbit.com")
	jar.SetCookies(base, []*http.Cookie{{Name: "XSRF-TOKEN", Value: "abc%2Bdef%3D%3D"}})
	c := &client.Client{BaseURL: base.String(), HTTPClient: &http.Client{Jar: jar}}
	if got := xsrfCookieToken(c); got != "abc+def==" {
		t.Fatalf("xsrfCookieToken = %q, want abc+def==", got)
	}
	if got := xsrfCookieToken(&client.Client{BaseURL: base.String(), HTTPClient: &http.Client{}}); got != "" {
		t.Fatalf("no jar: got %q, want empty", got)
	}
}
