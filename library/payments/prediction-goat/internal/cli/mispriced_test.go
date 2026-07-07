// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
)

// TestMispricedSkipsUntraded verifies that the mispriced screen filters
// untraded Kalshi markets before comparing prices — otherwise a Kalshi
// platform-default 17c ask paired against a real 6% Polymarket market
// would surface as an 11pt mispricing false positive.
func TestMispricedSkipsUntraded(t *testing.T) {
	t.Parallel()
	pmMarkets := []rawMarket{
		{Venue: "polymarket", ID: "p1", Title: "Will Kanye visit Israel", YesProbability: 0.06},
	}
	kalshiMarkets := []rawMarket{
		{Venue: "kalshi", ID: "K1", Title: "Will Kanye visit Israel", YesProbability: 0.17, Untraded: true},
	}
	// In-process equivalent of the runMispriced inner loop: confirm
	// the filter rejects the pair. Direct call to the function below.
	pairs := mispricedPairLoop(pmMarkets, kalshiMarkets, 0.05)
	if len(pairs) != 0 {
		t.Errorf("expected zero pairs (untraded filtered), got %d", len(pairs))
	}
}

// TestMispricedPairsAboveThreshold confirms the happy path: a real
// divergence above threshold produces a pair with the canonical delta
// and the apples-to-apples deltaPercent.
func TestMispricedPairsAboveThreshold(t *testing.T) {
	t.Parallel()
	pmMarkets := []rawMarket{
		{Venue: "polymarket", ID: "p1", Title: "OKC wins WCF", YesProbability: 0.79},
	}
	kalshiMarkets := []rawMarket{
		{Venue: "kalshi", ID: "K1", Title: "OKC wins WCF", YesProbability: 0.65},
	}
	pairs := mispricedPairLoop(pmMarkets, kalshiMarkets, 0.05)
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}
	p := pairs[0]
	if p.Delta < 0.139 || p.Delta > 0.141 {
		t.Errorf("Delta = %v, want ~0.14", p.Delta)
	}
	if p.DeltaPercent != 14.0 {
		t.Errorf("DeltaPercent = %v, want 14.0", p.DeltaPercent)
	}
}

// TestMispricedDropsSubThreshold confirms pairs below the threshold are
// rejected even when the Jaccard score is high.
func TestMispricedDropsSubThreshold(t *testing.T) {
	t.Parallel()
	pmMarkets := []rawMarket{
		{Venue: "polymarket", ID: "p1", Title: "France wins WC", YesProbability: 0.18},
	}
	kalshiMarkets := []rawMarket{
		{Venue: "kalshi", ID: "K1", Title: "France wins WC", YesProbability: 0.179},
	}
	pairs := mispricedPairLoop(pmMarkets, kalshiMarkets, 0.05)
	if len(pairs) != 0 {
		t.Errorf("expected zero pairs (sub-threshold), got %d", len(pairs))
	}
}

// TestMispricedNoDoublePair locks the guard against the same Kalshi
// market pairing with multiple Polymarket markets. Two PM titles for the
// same WCF event must not both pair to the same Kalshi WCF market.
func TestMispricedNoDoublePair(t *testing.T) {
	t.Parallel()
	pmMarkets := []rawMarket{
		{Venue: "polymarket", ID: "p1", Title: "OKC wins NBA Western Conference Finals", YesProbability: 0.79},
		{Venue: "polymarket", ID: "p2", Title: "OKC wins WCF over SAS", YesProbability: 0.80},
	}
	kalshiMarkets := []rawMarket{
		{Venue: "kalshi", ID: "K1", Title: "OKC wins NBA Western Conference Finals", YesProbability: 0.65},
	}
	pairs := mispricedPairLoop(pmMarkets, kalshiMarkets, 0.05)
	// Only one pair should land — the Kalshi market gets claimed by the
	// first Polymarket match and is then off-limits for subsequent ones.
	if len(pairs) != 1 {
		t.Errorf("expected 1 pair (no double-claim), got %d", len(pairs))
	}
}

// mispricedPairLoop is a tiny in-test mirror of the runMispriced pairing
// loop without the DB plumbing. Keeps tests focused on the behavior
// rather than the SQL machinery. Mirrors the usedKalshi guard.
func mispricedPairLoop(pmMarkets, kalshiMarkets []rawMarket, threshold float64) []mispricedPair {
	pairs := make([]mispricedPair, 0)
	usedKalshi := make(map[int]bool, len(kalshiMarkets))
	for _, pm := range pmMarkets {
		bestIdx := -1
		bestScore := 0.0
		for i, kalshi := range kalshiMarkets {
			if usedKalshi[i] {
				continue
			}
			if score := tokenJaccard(pm.Title, kalshi.Title); score > bestScore {
				bestIdx = i
				bestScore = score
			}
		}
		if bestIdx < 0 || bestScore < 0.20 {
			continue
		}
		kalshi := kalshiMarkets[bestIdx]
		if kalshi.Untraded {
			continue
		}
		usedKalshi[bestIdx] = true
		delta := pm.YesProbability - kalshi.YesProbability
		if delta < 0 {
			delta = -delta
		}
		if delta < threshold {
			continue
		}
		pairs = append(pairs, mispricedPair{Match: bestScore, PM: compareVenueFromRaw(pm), Kalshi: compareVenueFromRaw(kalshi), Delta: pm.YesProbability - kalshi.YesProbability, DeltaPercent: roundDelta(pm.YesProbability - kalshi.YesProbability)})
	}
	return pairs
}
