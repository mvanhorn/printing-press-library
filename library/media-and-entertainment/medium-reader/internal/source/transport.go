// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

// Package source defines the v2 fetch layer for the independent Medium reader.
//
// This is a NEW parallel layer to v1's RapidAPI-shaped internal/client. It
// sources data directly from Medium's public surfaces — RSS, article-page
// HTML, and Medium's own internal GraphQL endpoint — with NO API key, NO
// RapidAPI, and NO proxy at runtime. Cookies (the user's own Medium session)
// are an optional Tier-1 convenience, never required.
package source

import (
	"net/http"
	"strings"
	"time"

	enetxhttp "github.com/enetx/http"
	"github.com/enetx/surf"
)

// Cookies carries the optional, user-supplied Medium session material.
//
// Every field is optional. With all fields empty the transport runs fully
// anonymous (Tier 0), which Gate 0 validated clears Cloudflare on its own via
// Chrome impersonation. Populating Sid/Uid unlocks member-only full bodies on
// the read path (Tier 1); CfClearance is a passthrough in case a future
// Cloudflare posture requires it.
type Cookies struct {
	Sid         string // Medium session id ("sid" cookie)
	Uid         string // Medium user id ("uid" cookie)
	CfClearance string // Cloudflare clearance ("cf_clearance" cookie)
}

// IsZero reports whether no cookie material is present. Used by callers to
// decide whether they are running anonymous (Tier 0) or authenticated (Tier 1).
func (c Cookies) IsZero() bool {
	return c.Sid == "" && c.Uid == "" && c.CfClearance == ""
}

// Header renders the cookies as a single Cookie request-header value
// ("name=value; name=value"). Empty fields are skipped. Returns "" when no
// cookies are set, so callers can guard with a simple emptiness check.
//
// We attach cookies via an explicit header rather than an http.CookieJar
// because (a) the values are opaque single-domain tokens the caller already
// holds — there is no Set-Cookie round-trip to manage — and (b) it keeps the
// transport a plain *http.Client with no jar state, matching the Gate 0 probe
// that validated this exact path live against Medium.
func (c Cookies) Header() string {
	parts := make([]string, 0, 3)
	if c.Sid != "" {
		parts = append(parts, "sid="+c.Sid)
	}
	if c.Uid != "" {
		parts = append(parts, "uid="+c.Uid)
	}
	if c.CfClearance != "" {
		parts = append(parts, "cf_clearance="+c.CfClearance)
	}
	return strings.Join(parts, "; ")
}

// NewHTTPClient builds the Surf Chrome-impersonation *http.Client used by every
// v2 source. It mirrors v1's internal/client newHTTPClient minus the cookie
// jar (v2 attaches cookies per-request via AttachCookies instead).
//
// Why Surf + Chrome impersonation: Medium sits behind Cloudflare. Gate 0
// proved that a Chrome-impersonating client (matching TLS/JA3 fingerprint,
// HTTP/2 settings, and header ordering of a real Chrome) clears Cloudflare's
// bot check with NO cookies for the Tier-0 surfaces. A vanilla net/http client
// would be challenged. We force HTTP/2 and Session() (Surf's per-client
// connection reuse) to match the v1-validated configuration.
//
// The ResponseHeaderTimeout override exists because Surf's underlying
// *enetxhttp.Transport defaults ResponseHeaderTimeout to ~10s, which caps how
// long we wait for the first response byte independent of the overall client
// Timeout. Medium's slower endpoints (server-side-rendered article pages) can
// exceed that, so we raise the transport ceiling to track the caller's intent.
func NewHTTPClient(timeout time.Duration) *http.Client {
	b := surf.NewClient().
		Builder().
		Impersonate().
		Chrome().
		Timeout(timeout)
	b = b.ForceHTTP2()
	b = b.Session()
	sc := b.Build().Unwrap()
	if t, ok := sc.GetTransport().(*enetxhttp.Transport); ok {
		t.ResponseHeaderTimeout = timeout
	}
	hc := sc.Std()
	hc.Timeout = timeout
	return hc
}

// AttachCookies sets the Cookie header on req from the supplied cookies,
// leaving the request untouched when no cookies are present. Returns the same
// request for call-chaining convenience.
//
// This is the single seam through which Tier-1 session material reaches the
// wire. Sources call it just before issuing a request (read for member full
// body, optionally graphql); the anonymous Tier-0 path passes a zero Cookies
// and the request goes out clean.
func AttachCookies(req *http.Request, c Cookies) *http.Request {
	if req == nil {
		return req
	}
	if h := c.Header(); h != "" {
		req.Header.Set("Cookie", h)
	}
	return req
}

// GraphQLHeaders sets the headers Medium's /_/graphql endpoint expects on a
// POST request, as validated live in Gate 0. Inline queries are accepted; no
// persisted-query hash is required.
//
// Centralised here (rather than inlined in the graphql source) so the contract
// stays in one place alongside the transport, and so a stub source in tests can
// assert the same shape without importing the graphql package.
func GraphQLHeaders(req *http.Request) {
	if req == nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://medium.com")
	req.Header.Set("Referer", "https://medium.com/")
}
