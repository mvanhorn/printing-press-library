// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The cache must cover exactly the static lookup endpoints and NEVER a price or
// availability call — including Drivalia's get-vehicle-by-location (which
// contains "location") and Goldcar's session-seeding office call.
func TestCacheableLookupPath(t *testing.T) {
	cacheable := []string{
		"/api/location/all-locations",                            // Clickrent
		"/digital-backend/api/v1/short-term/search/get-location", // Drivalia
		"/branch/getall/",                                        // Centauro
		"/branch/getall",
	}
	for _, p := range cacheable {
		if !cacheableLookupPath(p) {
			t.Errorf("expected %q to be cacheable", p)
		}
	}
	notCacheable := []string{
		"/search/get-vehicle-by-location",      // Drivalia PRICE call (contains "location")
		"/search/enriched-offer",               // Drivalia price enrichment
		"/bookingAvailability/getAvailability", // Centauro prices
		"/api/bookings/groups",                 // Clickrent prices
		"/api/v1/disponibilidad",               // Goldcar prices
		"/api/v1/oficina/q/AGP/es",             // Goldcar office — seeds session cookie, must NOT cache
		"/offers",                              // Delpaso prices
	}
	for _, p := range notCacheable {
		if cacheableLookupPath(p) {
			t.Errorf("%q must NOT be cacheable (price or stateful call)", p)
		}
	}
}

// countingRT records how many times it is actually hit and returns a fixed body.
type countingRT struct {
	hits int64
	body string
}

func (c *countingRT) RoundTrip(req *http.Request) (*http.Response, error) {
	atomic.AddInt64(&c.hits, 1)
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(c.body)),
		Request:    req,
	}, nil
}

// A lookup response is cached on first fetch and served from cache on the second
// (network hit once); a price path always hits the network; --no-cache bypasses.
func TestLookupCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	req := func(path string) *http.Request {
		r, _ := http.NewRequest(http.MethodGet, "https://api.example.com"+path, nil)
		return r
	}
	drain := func(resp *http.Response) string {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return string(b)
	}

	// Two calls to a cacheable lookup → base hit once, body intact both times.
	base := &countingRT{body: `[{"id":"1"}]`}
	tr := &lookupCacheTransport{base: base, dir: dir, ttl: time.Hour}
	if got := drain(mustRT(t, tr, req("/api/location/all-locations"))); got != `[{"id":"1"}]` {
		t.Fatalf("first body = %q", got)
	}
	if got := drain(mustRT(t, tr, req("/api/location/all-locations"))); got != `[{"id":"1"}]` {
		t.Fatalf("cached body = %q", got)
	}
	if h := atomic.LoadInt64(&base.hits); h != 1 {
		t.Errorf("lookup should hit network once, got %d", h)
	}

	// A price path is never cached: every call hits the network.
	priceBase := &countingRT{body: `{}`}
	pt := &lookupCacheTransport{base: priceBase, dir: dir, ttl: time.Hour}
	drain(mustRT(t, pt, req("/api/bookings/groups")))
	drain(mustRT(t, pt, req("/api/bookings/groups")))
	if h := atomic.LoadInt64(&priceBase.hits); h != 2 {
		t.Errorf("price path must hit network every time, got %d", h)
	}

	// --no-cache (disabled) bypasses even for lookups.
	db := &countingRT{body: `[]`}
	dt := &lookupCacheTransport{base: db, dir: dir, ttl: time.Hour, disabled: true}
	drain(mustRT(t, dt, req("/api/location/all-locations")))
	drain(mustRT(t, dt, req("/api/location/all-locations")))
	if h := atomic.LoadInt64(&db.hits); h != 2 {
		t.Errorf("--no-cache must bypass the cache, got %d hits", h)
	}
}

func mustRT(t *testing.T, rt http.RoundTripper, req *http.Request) *http.Response {
	t.Helper()
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	return resp
}
