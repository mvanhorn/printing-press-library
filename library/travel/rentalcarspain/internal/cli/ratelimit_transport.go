// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.
// Per-host rate limiting for the HTML-scraping source clients: each supplier
// host is paced by its own adaptive limiter, so fanning out across suppliers
// (direct, selftest, querymatrix, dates) never hammers any single site.

package cli

import (
	"net/http"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/cliutil"
)

// defaultPerHostRate is the conservative starting request rate (requests/second,
// per host) for the source clients when the user sets no explicit --rate-limit.
// The supplier sites send no X-Ratelimit headers, so each host's limiter
// blind-ramps up from here after sustained success and halves on any 429.
const defaultPerHostRate = 5.0

// hostLimiterRegistry hands out one AdaptiveLimiter per host so requests to one
// supplier are paced independently of the others. Safe for concurrent use.
type hostLimiterRegistry struct {
	mu        sync.Mutex
	limiters  map[string]*cliutil.AdaptiveLimiter
	startRate float64
	maxRate   float64 // user's --rate-limit; 0 = auto (no hard ceiling, blind-ramp)
}

func newHostLimiterRegistry(maxRate float64) *hostLimiterRegistry {
	return &hostLimiterRegistry{
		limiters:  map[string]*cliutil.AdaptiveLimiter{},
		startRate: defaultPerHostRate,
		maxRate:   maxRate,
	}
}

// forHost returns the limiter for a host, creating it on first use. A user
// --rate-limit (maxRate>0) is a hard per-host ceiling; otherwise the limiter
// starts polite and adapts.
func (r *hostLimiterRegistry) forHost(host string) *cliutil.AdaptiveLimiter {
	r.mu.Lock()
	defer r.mu.Unlock()
	if l, ok := r.limiters[host]; ok {
		return l
	}
	var l *cliutil.AdaptiveLimiter
	if r.maxRate > 0 {
		l = cliutil.NewAdaptiveLimiter(r.maxRate)
	} else {
		l = cliutil.NewAdaptiveLimiterAuto(r.startRate)
	}
	r.limiters[host] = l
	return l
}

// perHostRateLimitTransport paces each outbound request by its host's limiter
// and feeds the response back so the limiter adapts: 429 halves the rate,
// X-Ratelimit headers (if any) drive budget pacing, and a plain success ramps.
type perHostRateLimitTransport struct {
	base http.RoundTripper
	reg  *hostLimiterRegistry
}

func (t *perHostRateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	lim := t.reg.forHost(req.URL.Host)
	lim.Wait()
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp == nil {
		return resp, err
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		lim.OnRateLimit()
		return resp, err
	}
	if rem, reset, ok := cliutil.ParseRateLimitHeaders(resp.Header); ok {
		lim.ObserveHeaders(rem, reset)
	}
	lim.OnSuccess()
	return resp, err
}

// hostRegistries caches one registry per distinct --rate-limit value so every
// source client in a process shares the same per-host limiters (coordinating
// concurrent suppliers and repeated fetches), without a single sync.Once global
// pinning the first request's rate for a long-lived MCP server's whole lifetime.
var (
	hostRegistriesMu sync.Mutex
	hostRegistries   = map[float64]*hostLimiterRegistry{}
)

func sharedHostRegistry(maxRate float64) *hostLimiterRegistry {
	hostRegistriesMu.Lock()
	defer hostRegistriesMu.Unlock()
	if r, ok := hostRegistries[maxRate]; ok {
		return r
	}
	r := newHostLimiterRegistry(maxRate)
	hostRegistries[maxRate] = r
	return r
}
