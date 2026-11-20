package client

import (
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/config"
	"testing"
)

func TestEtsyEffectiveRateLimit(t *testing.T) {
	isolateEtsyQuota(t)
	cfg := &config.Config{EtsyApiKey: "testkey"}
	check := func(requested, want float64) {
		t.Helper()
		if got := New(cfg, 0, requested).RateLimit(); got != want {
			t.Fatalf("requested %v: rate = %v, want %v", requested, got, want)
		}
	}
	check(RateLimitAuto, 4.5)
	h := quotaHeaders()
	h.Set("x-limit-per-second", "10")
	ObserveEtsyQuota(h, "testkey")
	check(RateLimitAuto, 9)
	check(2, 2)
	check(0, 0)
}
