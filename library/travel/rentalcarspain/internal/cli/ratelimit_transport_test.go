// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net/http"
	"testing"
)

// fakeRoundTripper returns a canned status without touching the network.
type fakeRoundTripper struct {
	status int
	header http.Header
}

func (f fakeRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	h := f.header
	if h == nil {
		h = http.Header{}
	}
	return &http.Response{StatusCode: f.status, Body: http.NoBody, Header: h}, nil
}

// One limiter per host, shared across calls for the same host, distinct across
// hosts — so pacing one supplier never throttles another.
func TestHostLimiterRegistry_PerHost(t *testing.T) {
	reg := newHostLimiterRegistry(0)
	a1 := reg.forHost("a.example")
	a2 := reg.forHost("a.example")
	b := reg.forHost("b.example")
	if a1 != a2 {
		t.Errorf("same host should return the same limiter")
	}
	if a1 == b {
		t.Errorf("different hosts should get distinct limiters")
	}
}

// A user --rate-limit is a hard per-host ceiling: the limiter starts at exactly
// that rate.
func TestHostLimiterRegistry_MaxRateCeiling(t *testing.T) {
	reg := newHostLimiterRegistry(8.0)
	if got := reg.forHost("h").Rate(); got != 8.0 {
		t.Errorf("limiter should start at the --rate-limit ceiling 8.0, got %v", got)
	}
}

// A 429 response halves the host's rate; a success leaves a capped rate intact.
func TestPerHostRateLimitTransport_Adapts(t *testing.T) {
	// 429 → halve.
	regThrottled := newHostLimiterRegistry(8.0)
	trThrottled := &perHostRateLimitTransport{base: fakeRoundTripper{status: 429}, reg: regThrottled}
	req, _ := http.NewRequest(http.MethodGet, "https://supplier.example/quote", nil)
	if _, err := trThrottled.RoundTrip(req); err != nil {
		t.Fatalf("round trip: %v", err)
	}
	if got := regThrottled.forHost("supplier.example").Rate(); got != 4.0 {
		t.Errorf("429 should halve 8.0 → 4.0, got %v", got)
	}

	// 200 → capped rate unchanged (one success does not ramp).
	regOK := newHostLimiterRegistry(8.0)
	trOK := &perHostRateLimitTransport{base: fakeRoundTripper{status: 200}, reg: regOK}
	if _, err := trOK.RoundTrip(req); err != nil {
		t.Fatalf("round trip: %v", err)
	}
	if got := regOK.forHost("supplier.example").Rate(); got != 8.0 {
		t.Errorf("a single success should leave the ceiling rate at 8.0, got %v", got)
	}
}

// sharedHostRegistry caches by --rate-limit value so same-config clients share
// per-host limiters while different configs stay isolated.
func TestSharedHostRegistry_CachedByRate(t *testing.T) {
	r1 := sharedHostRegistry(3.0)
	r2 := sharedHostRegistry(3.0)
	r3 := sharedHostRegistry(9.0)
	if r1 != r2 {
		t.Errorf("same rate should return the same registry")
	}
	if r1 == r3 {
		t.Errorf("different rates should return distinct registries")
	}
}
