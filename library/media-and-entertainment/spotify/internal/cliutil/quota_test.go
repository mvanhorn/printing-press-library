// Copyright 2026 Rob Zehner and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(amend-2026-08-08: quota circuit breaker) — coverage for quota.go.

package cliutil

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// resetQuotaStateForTest clears the in-process quota cache so each test sees
// only its own temp-dir state.
func resetQuotaStateForTest(t *testing.T) {
	t.Helper()
	t.Setenv("SPOTIFY_CACHE_DIR", t.TempDir())
	quotaMu.Lock()
	quotaLoaded = nil
	quotaMu.Unlock()
	t.Cleanup(func() {
		quotaMu.Lock()
		quotaLoaded = nil
		quotaMu.Unlock()
	})
}

func respWithRetryAfter(value string) *http.Response {
	h := http.Header{}
	if value != "" {
		h.Set("Retry-After", value)
	}
	return &http.Response{Header: h}
}

func TestRetryAfterUncapped_NoClamp(t *testing.T) {
	// The whole point: 67209s (~18.7h) must survive parsing un-clamped —
	// the clamped RetryAfter would return MaxRetryWait (60s) here.
	got := RetryAfterUncapped(respWithRetryAfter("67209"))
	if want := 67209 * time.Second; got != want {
		t.Fatalf("RetryAfterUncapped(67209) = %v, want %v", got, want)
	}
}

func TestRetryAfterUncapped_MissingOrGarbageReturnsZero(t *testing.T) {
	if got := RetryAfterUncapped(respWithRetryAfter("")); got != 0 {
		t.Fatalf("RetryAfterUncapped(missing) = %v, want 0", got)
	}
	if got := RetryAfterUncapped(respWithRetryAfter("garbage")); got != 0 {
		t.Fatalf("RetryAfterUncapped(garbage) = %v, want 0", got)
	}
	if got := RetryAfterUncapped(nil); got != 0 {
		t.Fatalf("RetryAfterUncapped(nil) = %v, want 0", got)
	}
}

func TestQuotaClassForPath(t *testing.T) {
	cases := map[string]string{
		"/search":               "search",
		"/search?q=x&type=y":    "search",
		"/artists/abc/top":      "artists",
		"/me/player/devices":    "me",
		"me":                    "me",
		"/":                     "root",
		"/playlists/xyz/tracks": "playlists",
	}
	for path, want := range cases {
		if got := QuotaClassForPath(path); got != want {
			t.Errorf("QuotaClassForPath(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestQuotaBlockRoundTripAndExpiry(t *testing.T) {
	resetQuotaStateForTest(t)

	until := time.Now().Add(2 * time.Hour)
	RecordQuotaBlock("search", until)

	got, blocked := QuotaBlockedUntil("search")
	if !blocked {
		t.Fatal("expected search to be blocked")
	}
	if !got.Equal(until) && got.Sub(until).Abs() > time.Second {
		t.Fatalf("BlockedUntil = %v, want ~%v", got, until)
	}
	// Endpoint-class scoping: a search block must not block me.
	if _, blocked := QuotaBlockedUntil("me"); blocked {
		t.Fatal("me must not be blocked by a search block")
	}

	// Survives a fresh in-process load (simulated process restart).
	quotaMu.Lock()
	quotaLoaded = nil
	quotaMu.Unlock()
	if _, blocked := QuotaBlockedUntil("search"); !blocked {
		t.Fatal("block must survive reload from disk")
	}

	// Expired entries prune lazily.
	RecordQuotaBlock("artists", time.Now().Add(-time.Minute))
	if _, blocked := QuotaBlockedUntil("artists"); blocked {
		t.Fatal("expired block must report unblocked")
	}
	for _, b := range ActiveQuotaBlocks() {
		if b.Class == "artists" {
			t.Fatal("expired block must be pruned from ActiveQuotaBlocks")
		}
	}
}

func TestLimiterCeilingPersistenceAndSeed(t *testing.T) {
	resetQuotaStateForTest(t)

	RecordLimiterCeiling(2.0)
	if got := PersistedLimiterCeiling(); got != 2.0 {
		t.Fatalf("PersistedLimiterCeiling = %v, want 2.0", got)
	}
	// Zero/negative ignored.
	RecordLimiterCeiling(0)
	if got := PersistedLimiterCeiling(); got != 2.0 {
		t.Fatalf("PersistedLimiterCeiling after RecordLimiterCeiling(0) = %v, want 2.0", got)
	}

	l := NewAdaptiveLimiter(3.0)
	l.SeedCeiling(2.0)
	if got := l.Ceiling(); got != 2.0 {
		t.Fatalf("Ceiling after seed = %v, want 2.0", got)
	}
	if got := l.Rate(); got != 1.8 {
		t.Fatalf("Rate after seeding ceiling 2.0 = %v, want 1.8 (ceiling*0.9)", got)
	}

	// Nil-safety mirrors the rest of the AdaptiveLimiter API.
	var nilLimiter *AdaptiveLimiter
	nilLimiter.SeedCeiling(2.0)
	if got := nilLimiter.Ceiling(); got != 0 {
		t.Fatalf("nil limiter Ceiling = %v, want 0", got)
	}
}

func TestQuotaBlockErrorMessageCarriesWallClock(t *testing.T) {
	until := time.Now().Add(3 * time.Hour)
	err := &QuotaBlockError{Class: "search", BlockedUntil: until}
	msg := err.Error()
	for _, want := range []string{"quota exhausted", "/search", until.Local().Format("2006-01-02 15:04")} {
		if !strings.Contains(msg, want) {
			t.Errorf("QuotaBlockError message missing %q: %s", want, msg)
		}
	}
}
