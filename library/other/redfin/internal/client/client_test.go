// Copyright 2026 rderwin and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import "testing"

// gisParams mirrors a real /stingray/api/gis request: enough keys that Go's
// randomized map iteration almost never repeats the same order twice in a row.
var gisParams = map[string]string{
	"al":          "1",
	"v":           "8",
	"region_id":   "30772",
	"region_type": "6",
	"status":      "7",
	"sf":          "1,2,3,5,6,7",
	"num_homes":   "100",
	"page_number": "1",
}

// TestCacheKeyDeterministic locks the contract that cacheKey returns the same
// value for the same (path, params) across calls.
//
// Background: cacheKey built its hash input by ranging directly over the
// params map. Go randomizes map iteration order per range, so two identical
// requests produced different key strings -> different sha256 -> different
// cache files. readCache therefore almost never found the entry writeCache
// had just written, silently defeating the 5-minute GET cache and forcing a
// fresh live request against Stingray's anti-bot gis endpoint every time.
func TestCacheKeyDeterministic(t *testing.T) {
	c := &Client{}
	const path = "/stingray/api/gis"
	want := c.cacheKey(path, gisParams)
	for i := 0; i < 1000; i++ {
		if got := c.cacheKey(path, gisParams); got != want {
			t.Fatalf("cacheKey not deterministic: call %d returned %q, first call returned %q", i, got, want)
		}
	}
}

// TestCacheKeyOrderIndependent verifies that two maps holding identical
// content hash to the same key regardless of how they were populated.
func TestCacheKeyOrderIndependent(t *testing.T) {
	c := &Client{}
	const path = "/stingray/api/gis"

	a := map[string]string{}
	for _, k := range []string{"al", "v", "region_id", "region_type", "status"} {
		a[k] = gisParams[k]
	}
	b := map[string]string{}
	for _, k := range []string{"status", "region_type", "region_id", "v", "al"} {
		b[k] = gisParams[k]
	}
	if c.cacheKey(path, a) != c.cacheKey(path, b) {
		t.Fatalf("cacheKey differs for maps with identical content")
	}
}

// TestCacheKeyDistinctParams guards against the opposite failure: different
// requests must not collide on the same cache file. Querying a different
// region (or path) has to produce a different key.
func TestCacheKeyDistinctParams(t *testing.T) {
	c := &Client{}
	dallas := c.cacheKey("/stingray/api/gis", map[string]string{"al": "1", "region_id": "30785"})
	whidbey := c.cacheKey("/stingray/api/gis", map[string]string{"al": "1", "region_id": "1387"})
	if dallas == whidbey {
		t.Fatalf("cacheKey collided for distinct region_id values: %q", dallas)
	}
	otherPath := c.cacheKey("/stingray/api/home/details/initialInfo", map[string]string{"al": "1", "region_id": "30785"})
	if dallas == otherPath {
		t.Fatalf("cacheKey collided across distinct paths: %q", dallas)
	}
}
