// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Cookie-jar seeding for Forkable's cookie/session auth. The generated
// New() builds the HTTP client with a nil jar and never seeds cookies from
// the stored credential, so authenticated GraphQL POSTs 401. This helper
// builds a jar from Config.CookieCredential() (the raw Cookie-header string
// captured by `auth login --chrome` / --cookies-file) and installs it on
// the client. Hand-authored; preserved across generate --force. Called from
// a one-line pp:hand-edit at the end of New() in client.go.

package client

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
)

// seedCookieJar builds an http.CookieJar from the client's stored cookie
// credential and installs it on the HTTP client. No-op when there is no
// cookie credential (env-var/bearer auth) or the base URL is unparseable.
func (c *Client) seedCookieJar() {
	if c == nil || c.Config == nil || c.HTTPClient == nil {
		return
	}
	raw := strings.TrimSpace(c.Config.CookieCredential())
	if raw == "" {
		return
	}
	cookies := parseCookieHeaderString(raw)
	if len(cookies) == 0 {
		return
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Host == "" {
		return
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return
	}
	jar.SetCookies(u, cookies)
	c.HTTPClient.Jar = jar
}

// parseCookieHeaderString parses a raw "k=v; k2=v2" Cookie-header string
// into http.Cookie values. Malformed segments are skipped.
func parseCookieHeaderString(header string) []*http.Cookie {
	header = strings.TrimSpace(header)
	header = strings.TrimPrefix(header, "Cookie:")
	header = strings.TrimPrefix(header, "cookie:")
	var cookies []*http.Cookie
	for _, seg := range strings.Split(header, ";") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		name, value, ok := strings.Cut(seg, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			continue
		}
		// #nosec G124 -- These are outbound request cookies parsed from the
		// user's own captured "Cookie:" request header, not server-set
		// response cookies. Secure/HttpOnly/SameSite are response-side
		// directives that are meaningless (and incorrect) to set on cookies
		// the client is sending. The jar transmits them over the base URL's
		// scheme, which is HTTPS for forkable.com.
		cookies = append(cookies, &http.Cookie{Name: name, Value: strings.TrimSpace(value)})
	}
	return cookies
}
