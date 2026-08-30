// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Forkable-specific CSRF handshake. Forkable's private GraphQL API
// (POST /api/v2/graphql) requires an x-csrf-token header alongside the
// session cookie. The token is fetched from GET /api/v2/csrf_token, which
// is itself unauthenticated. We fetch it lazily once per Client and cache
// it for the process lifetime.
//
// The my-account SPA's "Contact Support" modal (methods.submitContactForm
// in mc/js/app.*.js) POSTs the same x-csrf-token header, minted from the
// same handshake, to a second, non-GraphQL route: /submit_contact_form.
// needsCSRF covers both routes for that reason.
//
// This lives in a separate file (not client.go) so `generate --force` does
// not clobber it. The single call site inside client.go's request loop is a
// documented, regen-mergeable one-line hand-edit.

package client

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
)

// csrfPath is the endpoint that mints a CSRF token for the session.
const csrfPath = "/api/v2/csrf_token"

// graphqlPath is the GraphQL route that requires the CSRF header.
const graphqlPath = "/api/v2/graphql"

// contactFormPath is the my-account SPA's "Contact Support" REST route.
// Discovered by static analysis of mc/js/app.*.js (methods.submitContactForm);
// it is not part of the GraphQL surface but requires the same CSRF header.
const contactFormPath = "/submit_contact_form"

// csrfCache holds a per-Client CSRF token. It caches only a successful
// fetch: a transient failure leaves the token empty so the next GraphQL
// request retries the handshake instead of permanently omitting the header.
type csrfCache struct {
	mu    sync.Mutex
	token string
}

// forkableCSRF is a per-Client cache. It is keyed by *Client so distinct
// Clients (different sessions / base URLs) do not share a token. A
// package-level map avoids editing the generated Client struct in client.go
// (which regen re-emits); the only client.go hand-edit is the one-line call
// site in the request loop.
var (
	forkableCSRFMu    sync.Mutex
	forkableCSRFByCli = map[*Client]*csrfCache{}
)

// csrfTokenResponse is the shape of GET /api/v2/csrf_token.
type csrfTokenResponse struct {
	Token string `json:"token"`
}

// needsCSRF reports whether a request to the given path should carry the
// x-csrf-token header. The GraphQL route and the contact-form route both
// need it; every other route rides cookie-only auth.
func needsCSRF(path string) bool {
	return strings.Contains(path, graphqlPath) || strings.Contains(path, contactFormPath)
}

func (c *Client) csrfCache() *csrfCache {
	forkableCSRFMu.Lock()
	defer forkableCSRFMu.Unlock()
	cc := forkableCSRFByCli[c]
	if cc == nil {
		cc = &csrfCache{}
		forkableCSRFByCli[c] = cc
	}
	return cc
}

// ensureCSRFToken returns this Client's cached CSRF token, fetching it on
// first use (and retrying on a prior failure). It uses the Client's own HTTP
// client so the request rides the same cookie jar and timeout. Failures are
// non-fatal: the header is omitted and the caller may still succeed if the
// server does not enforce CSRF for reads.
func (c *Client) ensureCSRFToken(ctx context.Context) string {
	if c.HTTPClient == nil {
		return ""
	}
	cc := c.csrfCache()
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if cc.token != "" {
		return cc.token
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+csrfPath, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://forkable.com")
	req.Header.Set("Referer", "https://forkable.com/mc/")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}
	var parsed csrfTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return ""
	}
	cc.token = parsed.Token // cache only on success
	return cc.token
}
