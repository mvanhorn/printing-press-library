// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

package analysis

import (
	"testing"
	"time"
)

func TestBuildActionQueueProducesStableReasonCodes(t *testing.T) {
	observedAt := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	insights := []Snapshot{snapshot("marketplace-insights", observedAt, map[string]any{
		"keyword": "shirt", "listings": []any{
			map[string]any{"listingId": float64(1)},
			map[string]any{"listingId": float64(3)},
			map[string]any{"listingId": float64(4)},
		},
	})}
	ads := []Snapshot{
		snapshot("ads", observedAt, adsValue(1, 100, 0, 20, 0)),
		snapshot("ads", observedAt, adsValue(2, 0, 0, 40, 0)),
		snapshot("ads", observedAt, adsValue(3, 0, 0, 10, 0)),
		snapshot("ads", observedAt, adsValue(4, 20, 1, 15, 1)),
	}
	promotions := []Snapshot{snapshot("promotions", observedAt, map[string]any{
		"promotion_id": float64(9), "reward_set_listing_ids": []any{float64(3)},
	})}

	results := BuildActionQueue(insights, ads, nil, promotions)
	got := make(map[string]string)
	for _, result := range results {
		got[result.ListingID] = result.Action
	}
	want := map[string]string{"1": "review-ads", "2": "research", "3": "review-promotion", "4": "hold"}
	for listingID, action := range want {
		if got[listingID] != action {
			t.Fatalf("listing %s: got %q, want %q", listingID, got[listingID], action)
		}
	}
}

func TestReconcileEconomicsPreservesAttributionBoundaries(t *testing.T) {
	observedAt := time.Now().UTC()
	ads := []Snapshot{
		snapshot("ads", observedAt, adsValue(1, 125, 2, 10, 500)),
		snapshot("ads", observedAt, adsValue(2, 75, 1, 5, 300)),
	}
	offsite := []Snapshot{snapshot("offsite-ads", observedAt, map[string]any{
		"totalRevenue": "120.50", "fees": "18.00",
	})}

	result := ReconcileEconomics(ads, offsite, nil)
	if result.AdsSpendCents != 200 || result.AdsAttributedRevenue != 800 {
		t.Fatalf("unexpected ads totals: %#v", result)
	}
	if result.OffsiteRevenue != 120.5 || result.OffsiteFees != 18 {
		t.Fatalf("unexpected offsite totals: %#v", result)
	}
	if result.NetProfit != nil {
		t.Fatalf("net profit must remain unavailable: %#v", result.NetProfit)
	}
	if result.Status != "partial-data" || len(result.MissingSources) != 1 || result.MissingSources[0] != "promotions" {
		t.Fatalf("missing source must remain explicit: %#v", result)
	}
}

func TestReconcileEconomicsDistinguishesObservedZeroFromMissing(t *testing.T) {
	observedAt := time.Now().UTC()
	promotions := []Snapshot{snapshot("promotions", observedAt, map[string]any{"revenue": 0})}

	result := ReconcileEconomics(nil, nil, promotions)
	if result.Status != "partial-data" {
		t.Fatalf("status = %q, want partial-data", result.Status)
	}
	if result.PromotionRevenue == nil || *result.PromotionRevenue != 0 {
		t.Fatalf("observed zero promotion revenue must not look missing: %#v", result.PromotionRevenue)
	}
	if len(result.MissingSources) != 2 {
		t.Fatalf("missing_sources = %v, want ads and offsite-ads", result.MissingSources)
	}
	if result.AvailableSources == nil {
		t.Fatal("available_sources must encode as [] instead of null")
	}
}

func TestReconcileEconomicsReadsNestedPromotionRevenueStats(t *testing.T) {
	observedAt := time.Now().UTC()
	promotions := []Snapshot{snapshot("promotions", observedAt, map[string]any{
		"revenue_stats": map[string]any{"revenue": float64(275)},
	})}

	result := ReconcileEconomics(nil, nil, promotions)
	if result.PromotionRevenue == nil || *result.PromotionRevenue != 275 {
		t.Fatalf("nested promotion revenue not read: %#v", result.PromotionRevenue)
	}
}

func TestAcquisitionChannelGapsHandleZeroDenominators(t *testing.T) {
	observedAt := time.Now().UTC()
	ads := []Snapshot{
		snapshot("ads", observedAt, adsValue(1, 0, 5, 10, 0)),
		snapshot("ads", observedAt, adsValue(2, 0, 0, 0, 0)),
	}
	offsite := []Snapshot{
		snapshot("offsite-ads", observedAt, map[string]any{"listingId": float64(1), "clicksCount": float64(10), "ordersCount": float64(1)}),
		snapshot("offsite-ads", observedAt, map[string]any{"listingId": float64(2), "clicksCount": float64(0), "ordersCount": float64(0)}),
	}

	results := AcquisitionChannelGaps(ads, offsite)
	if results[0].Classification != "onsite-strong" {
		t.Fatalf("got %q, want onsite-strong", results[0].Classification)
	}
	if results[1].Classification != "insufficient-data" ||
		results[1].OnsiteEfficiency != nil || results[1].OffsiteEfficiency != nil {
		t.Fatalf("zero denominators must remain insufficient: %#v", results[1])
	}
}

func TestAllocateResearchQuotaDoesNotRecommendFreshEvidence(t *testing.T) {
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	insights := []Snapshot{
		snapshot("marketplace-insights", now, map[string]any{"quotaRemaining": float64(2)}),
		snapshot("marketplace-insights", now.Add(-24*time.Hour), map[string]any{
			"keyword": "fresh", "listings": []any{map[string]any{"listingId": float64(1)}},
		}),
	}
	ads := []Snapshot{
		snapshot("ads", now, adsValue(1, 100, 0, 100, 0)),
		snapshot("ads", now, adsValue(2, 50, 0, 500, 0)),
		snapshot("ads", now, adsValue(3, 20, 0, 250, 0)),
	}

	results := AllocateResearchQuota(insights, ads, nil, nil, now)
	if len(results) != 2 {
		t.Fatalf("got %d recommendations, want 2", len(results))
	}
	if results[0].ListingID != "2" || results[1].ListingID != "3" {
		t.Fatalf("unexpected ranking: %#v", results)
	}
	for _, result := range results {
		if result.ListingID == "1" {
			t.Fatal("fresh listing was incorrectly recommended")
		}
	}
}

func TestVisibilityPerformanceGapsUseObservedMappingsOnly(t *testing.T) {
	observedAt := time.Now().UTC()
	listings := make([]any, 21)
	for index := range listings {
		listings[index] = map[string]any{"listingId": float64(index + 1)}
	}
	insights := []Snapshot{snapshot("marketplace-insights", observedAt, map[string]any{
		"keyword": "seasonal gift", "searches": float64(500), "listings": listings,
	})}

	results := VisibilityPerformanceGaps(insights, nil, nil)
	var rankTwentyOne VisibilityGap
	for _, result := range results {
		if result.ListingID == "21" {
			rankTwentyOne = result
			break
		}
	}
	if rankTwentyOne.ObservedRank != 21 || rankTwentyOne.Classification != "high-demand-low-visibility" {
		t.Fatalf("unexpected rank classification: %#v", rankTwentyOne)
	}
	if results[0].Classification != "insufficient-paid-data" || results[0].PaidDataObserved {
		t.Fatalf("missing paid data must remain explicit: %#v", results[0])
	}
}

func TestVisibilityPerformanceGapsRequireMeaningfulDemandThreshold(t *testing.T) {
	observedAt := time.Now().UTC()
	listings := make([]any, 21)
	for index := range listings {
		listings[index] = map[string]any{"listingId": float64(index + 1)}
	}
	insights := []Snapshot{snapshot("marketplace-insights", observedAt, map[string]any{
		"keyword": "niche phrase", "searches": float64(1), "listings": listings,
	})}
	ads := []Snapshot{snapshot("ads", observedAt, adsValue(1, 10, 0, 2, 0))}

	results := VisibilityPerformanceGaps(insights, ads, nil)
	var rankOne, rankTwentyOne VisibilityGap
	for _, result := range results {
		switch result.ListingID {
		case "1":
			rankOne = result
		case "21":
			rankTwentyOne = result
		}
	}
	if rankOne.Classification != "high-visibility-weak-paid-performance" || !rankOne.PaidDataObserved {
		t.Fatalf("observed zero paid performance classified incorrectly: %#v", rankOne)
	}
	if rankTwentyOne.Classification != "observed" {
		t.Fatalf("one search must not be high demand: %#v", rankTwentyOne)
	}
}

func snapshot(resource string, observedAt time.Time, value map[string]any) Snapshot {
	value["_observed_at"] = observedAt.Format(time.RFC3339Nano)
	return Snapshot{Resource: resource, ObservedAt: observedAt, Value: value}
}

func adsValue(listingID int, spend, conversions, clicks, revenue float64) map[string]any {
	return map[string]any{
		"listing": map[string]any{"listingId": float64(listingID)},
		"totalStats": map[string]any{
			"spentTotal": spend, "conversions": conversions,
			"clickCount": clicks, "revenue": revenue,
		},
	}
}
