// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.

// Akamai cookie warming.
//
// Woolworths sits behind Akamai Bot Manager. Verified behaviour, 2026-08-23:
//
//   - GET endpoints answer fine on a cold connection provided browser-shaped
//     headers are present.
//   - POST endpoints (Search/products, browse/category) on a connection whose
//     cookie jar has never seen woolworths.com.au do NOT return an error. The
//     HTTP/2 stream is reset with INTERNAL_ERROR and the HTTP/1.1 fallback then
//     hangs until the client timeout. A cold `products search` took 45s and
//     failed; the same command took 926ms once the jar had been warmed.
//   - One plain GET of /shop populates bm_sz, bm_so, _abck, ak_bmsc and friends,
//     after which every POST works.
//
// WarmSession performs that single GET before the first POST on a Client whose
// cookie jar is still cold. Warming is per-Client because the jar is per-Client:
// a process that builds several Clients (specials-diff builds one per specials
// group) must warm each of them, or all but the first hit the unwarmed-Akamai
// hang path. The jar-already-has-cookies short-circuit below is what keeps this
// cheap and idempotent, so no process-wide sync.Once is used.
//
// It is deliberately best-effort: a failure to warm must not fail the command,
// because the request may still succeed (for example when a persisted jar was
// already loaded from disk).
//
// This file is hand-authored and lives beside the generated client so that
// `generate --force` preserves it. Do not fold it into client.go.

package client

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const warmPath = "/shop"

// WarmSession primes the Akamai cookie jar with a single GET of the site's shop
// page. It runs at most once per process and never returns an error to the
// caller — warming is an optimisation, not a precondition.
//
// Call it after constructing a Client and before issuing any POST.
func (c *Client) WarmSession(ctx context.Context) {
	if c == nil || c.HTTPClient == nil {
		return
	}
	c.warmSessionNow(ctx)
}

func (c *Client) warmSessionNow(ctx context.Context) {
	base := c.RequestBaseURL()
	if base == "" {
		return
	}

	u, err := url.Parse(base)
	if err != nil {
		return
	}

	// Only ever warm the real Woolworths host. The warm exists solely to satisfy
	// Akamai bot management on woolworths.com.au; issuing it anywhere else is
	// meaningless, and actively harmful in tests where an httptest server on
	// 127.0.0.1 would record the warm GET instead of the request under test.
	// This guard lives here, in a hand-authored file the generator preserves,
	// rather than in the generated client_test.go, which regeneration discards.
	host := strings.ToLower(u.Hostname())
	if host != "woolworths.com.au" && !strings.HasSuffix(host, ".woolworths.com.au") {
		return
	}

	// Skip entirely when the jar already carries cookies for this host, which
	// is the common case once a profile has been used once. This keeps the warm
	// off the hot path: one GET on a cold profile, nothing thereafter.
	if jar := c.HTTPClient.Jar; jar != nil && len(jar.Cookies(u)) > 0 {
		return
	}

	// Bound the warm independently of the caller's context so a slow warm can
	// never consume the whole command budget.
	warmCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(warmCtx, http.MethodGet, base+warmPath, nil)
	if err != nil {
		return
	}
	// Browser-shaped headers: an Accept of application/json here would be
	// answered with a 403 by Akamai, since /shop is an HTML document.
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-AU,en;q=0.9")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()
	// The body must be drained for the connection (and its cookies) to be
	// reusable; the content itself is irrelevant.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
}
