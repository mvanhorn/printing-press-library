// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package opentable

import (
	"testing"
	"time"
)

func TestEnrichWithCacheMetadata_FreshAge_StaleFalse(t *testing.T) {
	cachedAt := time.Now().Add(-2 * time.Minute)
	resp := []RestaurantAvailability{
		{RestaurantID: 25606, AvailabilityDays: []AvailabilityDay{{DayOffset: 0}}},
	}
	out := enrichWithCacheMetadata(resp, cachedAt, false)
	if len(out) != 1 {
		t.Fatalf("expected 1 row, got %d", len(out))
	}
	if out[0].Source != "cache_fallback" {
		t.Errorf("expected Source=cache_fallback, got %q", out[0].Source)
	}
	if out[0].Stale {
		t.Errorf("expected Stale=false for within-TTL entry, got true")
	}
	if !out[0].CachedAt.Equal(cachedAt) {
		t.Errorf("expected CachedAt=%v, got %v", cachedAt, out[0].CachedAt)
	}
}

func TestEnrichWithCacheMetadata_PastTTL_StaleTrue(t *testing.T) {
	cachedAt := time.Now().Add(-30 * time.Minute)
	resp := []RestaurantAvailability{
		{RestaurantID: 25606, AvailabilityDays: []AvailabilityDay{{DayOffset: 0}}},
	}
	out := enrichWithCacheMetadata(resp, cachedAt, true)
	if out[0].Source != "cache_fallback" {
		t.Errorf("expected Source=cache_fallback, got %q", out[0].Source)
	}
	if !out[0].Stale {
		t.Errorf("expected Stale=true for past-TTL entry, got false")
	}
}

func TestEnrichWithCacheMetadata_DoesNotMutateInput(t *testing.T) {
	cachedAt := time.Now().Add(-2 * time.Minute)
	resp := []RestaurantAvailability{
		{RestaurantID: 25606, AvailabilityDays: []AvailabilityDay{{DayOffset: 0}}},
	}
	_ = enrichWithCacheMetadata(resp, cachedAt, false)
	// Input slice's Source must remain empty — enrichment returns a copy.
	if resp[0].Source != "" {
		t.Errorf("input was mutated: Source=%q", resp[0].Source)
	}
}
