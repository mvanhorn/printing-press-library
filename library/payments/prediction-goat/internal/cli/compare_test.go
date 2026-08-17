// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
)

// TestBuildUnpaired covers the no-pair diagnostic payload: per-venue top
// N rows materialize into compareVenue shapes for the JSON envelope.
func TestBuildUnpaired(t *testing.T) {
	t.Parallel()
	pm := []rawMarket{
		{Venue: "polymarket", ID: "p1", Title: "A", YesProbability: 0.50, URL: "https://polymarket.com/market/p1"},
		{Venue: "polymarket", ID: "p2", Title: "B", YesProbability: 0.30, URL: "https://polymarket.com/market/p2"},
		{Venue: "polymarket", ID: "p3", Title: "C", YesProbability: 0.10, URL: "https://polymarket.com/market/p3"},
	}
	kalshi := []rawMarket{
		{Venue: "kalshi", ID: "K1", Title: "X", YesProbability: 0.70},
	}
	got := buildUnpaired(pm, kalshi, 2)
	if len(got.Polymarket) != 2 {
		t.Errorf("polymarket count = %d, want 2", len(got.Polymarket))
	}
	if len(got.Kalshi) != 1 {
		t.Errorf("kalshi count = %d, want 1", len(got.Kalshi))
	}
	if got.Polymarket[0].ID != "p1" {
		t.Errorf("polymarket[0].ID = %q, want p1", got.Polymarket[0].ID)
	}
	// yesPercent is populated even when missing from input (0 -> 0)
	if got.Polymarket[0].YesPercent != 50.0 {
		t.Errorf("polymarket[0].YesPercent = %v, want 50.0", got.Polymarket[0].YesPercent)
	}
	if got.Kalshi[0].YesPercent != 70.0 {
		t.Errorf("kalshi[0].YesPercent = %v, want 70.0", got.Kalshi[0].YesPercent)
	}
}

// TestBuildUnpaired_Empty handles the both-sides-empty case used when the
// topic doesn't match anything: returns a non-nil envelope with both
// lists empty so JSON shape is consistent.
func TestBuildUnpaired_Empty(t *testing.T) {
	t.Parallel()
	got := buildUnpaired(nil, nil, 5)
	if got == nil {
		t.Fatalf("buildUnpaired returned nil envelope")
	}
	if len(got.Polymarket) != 0 || len(got.Kalshi) != 0 {
		t.Errorf("expected empty lists, got pm=%d kalshi=%d", len(got.Polymarket), len(got.Kalshi))
	}
}

// TestCompareVenueFromRaw_UntradedPropagates verifies that compareVenue
// carries the Untraded flag from the underlying rawMarket so JSON
// consumers see untraded markets clearly when they appear in unpaired
// candidate lists.
func TestCompareVenueFromRaw_UntradedPropagates(t *testing.T) {
	t.Parallel()
	r := rawMarket{Venue: "kalshi", ID: "X", Title: "Untraded", YesProbability: 0, Untraded: true}
	v := compareVenueFromRaw(r)
	if !v.Untraded {
		t.Errorf("expected Untraded=true in compareVenue")
	}
	if v.YesPercent != 0 {
		t.Errorf("expected YesPercent=0 for zero-prob untraded market, got %v", v.YesPercent)
	}
}

// TestCompareVenueFromRaw_YesPercentPopulated locks the canonical pairing:
// canonical yesProbability (0-1 float) plus a yesPercent (0-100 rounded)
// for apples-to-apples display.
func TestCompareVenueFromRaw_YesPercentPopulated(t *testing.T) {
	t.Parallel()
	r := rawMarket{Venue: "polymarket", ID: "p", Title: "X", YesProbability: 0.792}
	v := compareVenueFromRaw(r)
	if v.YesPercent != 79.2 {
		t.Errorf("YesPercent = %v, want 79.2", v.YesPercent)
	}
	if v.YesProbability != 0.792 {
		t.Errorf("YesProbability = %v, want 0.792", v.YesProbability)
	}
}

// TestPairCompareMarkets_LimitCountsOnlyMatches locks the over-fetch
// contract: unmatched PM-only leaders must not consume the pair limit.
// With limit=2 and the only confident matches sitting after several
// unmatchable PM markets, the matches must still surface.
func TestPairCompareMarkets_LimitCountsOnlyMatches(t *testing.T) {
	t.Parallel()
	pm := []rawMarket{
		{Venue: "polymarket", ID: "p1", Title: "zzz qqq vvv", YesProbability: 0.10},
		{Venue: "polymarket", ID: "p2", Title: "rrr sss ttt", YesProbability: 0.20},
		{Venue: "polymarket", ID: "p3", Title: "uuu www xxx", YesProbability: 0.30},
		{Venue: "polymarket", ID: "p4", Title: "lakers win nba finals", YesProbability: 0.60},
		{Venue: "polymarket", ID: "p5", Title: "thunder win west conference", YesProbability: 0.40},
	}
	kalshi := []rawMarket{
		{Venue: "kalshi", ID: "K1", Title: "lakers win nba finals", YesProbability: 0.55},
		{Venue: "kalshi", ID: "K2", Title: "thunder win west conference", YesProbability: 0.45},
	}
	pairs := pairCompareMarkets("nba", pm, kalshi, 2)
	if len(pairs) != 2 {
		t.Fatalf("pairs = %d, want 2 (unmatched PM leaders must not consume the limit)", len(pairs))
	}
	for _, p := range pairs {
		if p.PM == nil || p.Kalshi == nil {
			t.Errorf("pair %+v has a nil side; only confident two-sided pairs should be returned", p)
		}
	}
	if pairs[0].PM.ID != "p4" || pairs[1].PM.ID != "p5" {
		t.Errorf("pair order = %s,%s want p4,p5", pairs[0].PM.ID, pairs[1].PM.ID)
	}
}

// TestPairCompareMarkets_LimitStillCaps verifies the cap applies to
// matched pairs themselves.
func TestPairCompareMarkets_LimitStillCaps(t *testing.T) {
	t.Parallel()
	pm := []rawMarket{
		{Venue: "polymarket", ID: "p1", Title: "alpha beta gamma", YesProbability: 0.10},
		{Venue: "polymarket", ID: "p2", Title: "delta epsilon zeta", YesProbability: 0.20},
		{Venue: "polymarket", ID: "p3", Title: "eta theta iota", YesProbability: 0.30},
	}
	kalshi := []rawMarket{
		{Venue: "kalshi", ID: "K1", Title: "alpha beta gamma", YesProbability: 0.15},
		{Venue: "kalshi", ID: "K2", Title: "delta epsilon zeta", YesProbability: 0.25},
		{Venue: "kalshi", ID: "K3", Title: "eta theta iota", YesProbability: 0.35},
	}
	pairs := pairCompareMarkets("greek", pm, kalshi, 2)
	if len(pairs) != 2 {
		t.Fatalf("pairs = %d, want 2 (limit must cap matched pairs)", len(pairs))
	}
}
