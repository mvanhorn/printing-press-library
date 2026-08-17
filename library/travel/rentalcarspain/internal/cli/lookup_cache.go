// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.
// A disk cache scoped strictly to the suppliers' static office/location lookups.
// Prices and availability are NEVER cached — a live-price tool must never serve a
// stale price as current — so only standalone, non-stateful lookup reads qualify.

package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/cliutil"
)

// lookupCacheTTL is how long a cached office/location list is trusted. These
// lists change rarely and a slightly stale office list is harmless (unlike a
// stale price), so the TTL is long.
const lookupCacheTTL = 24 * time.Hour

// cacheableLookupPath reports whether a request path is one of the static
// office/location lookups safe to cache: standalone data reads that are NOT part
// of a stateful cookie/CSRF/session-seeding flow and never carry price or
// availability data.
//
// Deliberately EXCLUDED: every price/availability endpoint, and Goldcar's
// /api/v1/oficina/... call — which, despite being a "lookup", also seeds the
// session cookie, so caching it (skipping the network) would break the flow.
// Drivalia's /search/get-location is included but /search/get-vehicle-by-location
// (a price call) is not — the leading-slash suffix match distinguishes them.
func cacheableLookupPath(path string) bool {
	switch {
	case strings.HasSuffix(path, "/all-locations"): // Clickrent office list
		return true
	case strings.HasSuffix(path, "/branch/getall"), strings.HasSuffix(path, "/branch/getall/"): // Centauro branches
		return true
	case strings.HasSuffix(path, "/get-location"): // Drivalia locations (NOT get-vehicle-by-location)
		return true
	}
	return false
}

type cachedLookup struct {
	StoredAt   time.Time           `json:"stored_at"`
	StatusCode int                 `json:"status_code"`
	Header     map[string][]string `json:"header"`
	Body       []byte              `json:"body"`
}

// lookupCacheTransport caches responses for the static lookup endpoints only.
// Every other request — including all price/availability calls — passes straight
// through to base. Honors --no-cache via disabled.
type lookupCacheTransport struct {
	base     http.RoundTripper
	dir      string
	ttl      time.Duration
	disabled bool
}

// wrapLookupCache wraps base with the static-lookup disk cache, or returns base
// unchanged when the cache directory can't be resolved.
func wrapLookupCache(base http.RoundTripper, noCache bool) http.RoundTripper {
	dir, err := cliutil.CacheDir()
	if err != nil || dir == "" {
		return base
	}
	return &lookupCacheTransport{
		base:     base,
		dir:      filepath.Join(dir, "carsource-lookups"),
		ttl:      lookupCacheTTL,
		disabled: noCache,
	}
}

func (t *lookupCacheTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.disabled || t.dir == "" || !cacheableLookupPath(req.URL.Path) {
		return t.base.RoundTrip(req)
	}
	// Read the request body (POST lookups) so it keys the cache and can be
	// replayed to the real transport on a miss.
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
		req.Body.Close()
	}
	path := filepath.Join(t.dir, lookupCacheKey(req.Method, req.URL.String(), body)+".json")
	if resp := t.read(path, req); resp != nil {
		return resp, nil
	}
	if body != nil {
		req.Body = io.NopCloser(bytes.NewReader(body))
	}
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp == nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, err
	}
	t.write(path, resp) // consumes and replaces resp.Body
	return resp, err
}

func lookupCacheKey(method, url string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(method))
	h.Write([]byte{0})
	h.Write([]byte(url))
	h.Write([]byte{0})
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

func (t *lookupCacheTransport) read(path string, req *http.Request) *http.Response {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var c cachedLookup
	if json.Unmarshal(data, &c) != nil {
		return nil
	}
	if time.Since(c.StoredAt) > t.ttl {
		return nil
	}
	return &http.Response{
		StatusCode: c.StatusCode,
		Header:     http.Header(c.Header),
		Body:       io.NopCloser(bytes.NewReader(c.Body)),
		Request:    req,
	}
}

func (t *lookupCacheTransport) write(path string, resp *http.Response) {
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	// Always hand a fresh reader back to the caller, even if caching fails.
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return
	}
	c := cachedLookup{StoredAt: time.Now(), StatusCode: resp.StatusCode, Header: map[string][]string(resp.Header), Body: body}
	data, err := json.Marshal(c)
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}
