// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

package analysis

import (
	"sort"
	"time"
)

// ActionItem is one deterministic listing review recommendation.
type ActionItem struct {
	ListingID string    `json:"listing_id"`
	Action    string    `json:"action"`
	Reasons   []string  `json:"reasons"`
	Observed  time.Time `json:"observed_at,omitempty"`
}

// BuildActionQueue joins the newest listing observations across all four sources.
func BuildActionQueue(insights, ads, offsite, promotions []Snapshot) []ActionItem {
	adsByListing := LatestByListing(ads)
	offsiteByListing := LatestByListing(offsite)
	insightsByListing := insightListingSnapshots(insights)
	promotedListings := promotionListingSet(promotions)

	identifiers := make(map[string]struct{})
	for _, collection := range []map[string]Snapshot{adsByListing, offsiteByListing, insightsByListing} {
		for identifier := range collection {
			identifiers[identifier] = struct{}{}
		}
	}
	for identifier := range promotedListings {
		identifiers[identifier] = struct{}{}
	}

	results := make([]ActionItem, 0, len(identifiers))
	for identifier := range identifiers {
		action := "hold"
		reasons := []string{"no-review-trigger"}
		observed := newestTime(
			ObservedAt(adsByListing[identifier]),
			ObservedAt(offsiteByListing[identifier]),
			ObservedAt(insightsByListing[identifier]),
		)

		if _, found := insightsByListing[identifier]; !found {
			action = "research"
			reasons = []string{"missing-demand-evidence"}
		} else if adsSnapshot, found := adsByListing[identifier]; found &&
			nestedNumber(adsSnapshot.Value, "totalStats", "spentTotal") > 0 &&
			nestedNumber(adsSnapshot.Value, "totalStats", "conversions") == 0 {
			action = "review-ads"
			reasons = []string{"ad-spend-without-conversion"}
		} else if _, found := promotedListings[identifier]; found &&
			ordersForListing(identifier, adsByListing, offsiteByListing) == 0 {
			action = "review-promotion"
			reasons = []string{"promoted-without-observed-order"}
		}

		results = append(results, ActionItem{
			ListingID: identifier,
			Action:    action,
			Reasons:   reasons,
			Observed:  observed,
		})
	}
	sort.Slice(results, func(left, right int) bool {
		if results[left].Action == results[right].Action {
			return results[left].ListingID < results[right].ListingID
		}
		return results[left].Action < results[right].Action
	})
	return results
}

// EconomicsResult preserves Etsy's distinct attribution boundaries.
type EconomicsResult struct {
	Status               string   `json:"status"`
	AvailableSources     []string `json:"available_sources"`
	MissingSources       []string `json:"missing_sources"`
	AdsSpendCents        float64  `json:"ads_spend_cents"`
	AdsAttributedRevenue float64  `json:"ads_attributed_revenue_cents"`
	OffsiteFees          float64  `json:"offsite_fees"`
	OffsiteRevenue       float64  `json:"offsite_attributed_revenue"`
	PromotionRevenue     *float64 `json:"promotion_revenue,omitempty"`
	NetProfit            *float64 `json:"net_profit"`
	ExcludedCosts        []string `json:"excluded_costs"`
	AttributionWarning   string   `json:"attribution_warning"`
}

// ReconcileEconomics computes source subtotals without combining them into profit.
func ReconcileEconomics(ads, offsite, promotions []Snapshot) EconomicsResult {
	result := EconomicsResult{
		Status:             "insufficient-data",
		AvailableSources:   []string{},
		MissingSources:     []string{},
		ExcludedCosts:      []string{"cost_of_goods", "shipping", "base_etsy_fees", "taxes", "unobserved_discounts"},
		AttributionWarning: "Revenue fields retain their Etsy source attribution and must not be added together as net profit.",
	}
	if len(ads) == 0 {
		result.MissingSources = append(result.MissingSources, "ads")
	} else {
		result.AvailableSources = append(result.AvailableSources, "ads")
	}
	for _, snapshot := range LatestByListing(ads) {
		result.AdsSpendCents += nestedNumber(snapshot.Value, "totalStats", "spentTotal")
		result.AdsAttributedRevenue += nestedNumber(snapshot.Value, "totalStats", "revenue")
	}
	if latest, found := latestSummary(offsite, "totalRevenue", "fees"); found {
		result.AvailableSources = append(result.AvailableSources, "offsite-ads")
		result.OffsiteFees = Number(latest.Value, "fees")
		result.OffsiteRevenue = Number(latest.Value, "totalRevenue")
	} else {
		result.MissingSources = append(result.MissingSources, "offsite-ads")
	}
	if latest, found := latestSummary(promotions, "revenue", "revenue_stats"); found {
		result.AvailableSources = append(result.AvailableSources, "promotions")
		revenueValue := latest.Value
		if nested, ok := latest.Value["revenue_stats"].(map[string]any); ok {
			revenueValue = nested
		}
		revenue := Number(revenueValue, "revenue")
		result.PromotionRevenue = &revenue
	} else {
		result.MissingSources = append(result.MissingSources, "promotions")
	}
	switch len(result.AvailableSources) {
	case 3:
		result.Status = "ok"
	case 1, 2:
		result.Status = "partial-data"
	}
	return result
}

// ChannelGap is a listing-level onsite/offsite comparison.
type ChannelGap struct {
	ListingID         string   `json:"listing_id"`
	Classification    string   `json:"classification"`
	OnsiteOrders      float64  `json:"onsite_orders"`
	OnsiteClicks      float64  `json:"onsite_clicks"`
	OffsiteOrders     float64  `json:"offsite_orders"`
	OffsiteClicks     float64  `json:"offsite_clicks"`
	OnsiteEfficiency  *float64 `json:"onsite_orders_per_click"`
	OffsiteEfficiency *float64 `json:"offsite_orders_per_click"`
}

// AcquisitionChannelGaps classifies comparable listing-level channel observations.
func AcquisitionChannelGaps(ads, offsite []Snapshot) []ChannelGap {
	adsByListing := LatestByListing(ads)
	offsiteByListing := LatestByListing(offsite)
	identifiers := make(map[string]struct{})
	for identifier := range adsByListing {
		identifiers[identifier] = struct{}{}
	}
	for identifier := range offsiteByListing {
		identifiers[identifier] = struct{}{}
	}

	results := make([]ChannelGap, 0, len(identifiers))
	for identifier := range identifiers {
		adsSnapshot, hasAds := adsByListing[identifier]
		offsiteSnapshot, hasOffsite := offsiteByListing[identifier]
		onsiteClicks := nestedNumber(adsSnapshot.Value, "totalStats", "clickCount")
		onsiteOrders := nestedNumber(adsSnapshot.Value, "totalStats", "conversions")
		offsiteClicks := Number(offsiteSnapshot.Value, "clicksCount", "clicks_count")
		offsiteOrders := Number(offsiteSnapshot.Value, "ordersCount", "orders_count")
		onsiteEfficiency := ratio(onsiteOrders, onsiteClicks)
		offsiteEfficiency := ratio(offsiteOrders, offsiteClicks)
		classification := "insufficient-data"
		if hasAds && hasOffsite && onsiteEfficiency != nil && offsiteEfficiency != nil {
			switch {
			case *onsiteEfficiency > *offsiteEfficiency*1.25:
				classification = "onsite-strong"
			case *offsiteEfficiency > *onsiteEfficiency*1.25:
				classification = "offsite-strong"
			default:
				classification = "balanced"
			}
		}
		results = append(results, ChannelGap{
			ListingID: identifier, Classification: classification,
			OnsiteOrders: onsiteOrders, OnsiteClicks: onsiteClicks,
			OffsiteOrders: offsiteOrders, OffsiteClicks: offsiteClicks,
			OnsiteEfficiency: onsiteEfficiency, OffsiteEfficiency: offsiteEfficiency,
		})
	}
	sort.Slice(results, func(left, right int) bool { return results[left].ListingID < results[right].ListingID })
	return results
}

// QuotaRecommendation identifies consequential listings missing fresh demand evidence.
type QuotaRecommendation struct {
	ListingID       string  `json:"listing_id"`
	PriorityScore   float64 `json:"priority_score"`
	Reason          string  `json:"reason"`
	RecommendedRank int     `json:"recommended_rank"`
}

// AllocateResearchQuota ranks missing demand evidence without spending quota.
func AllocateResearchQuota(insights, ads, offsite, promotions []Snapshot, now time.Time) []QuotaRecommendation {
	remaining := 0
	var quotaObservedAt time.Time
	for _, snapshot := range insights {
		if _, camelFound := snapshot.Value["quotaRemaining"]; !camelFound {
			if _, snakeFound := snapshot.Value["quota_remaining"]; !snakeFound {
				continue
			}
		}
		if observedAt := ObservedAt(snapshot); quotaObservedAt.IsZero() || observedAt.After(quotaObservedAt) {
			remaining = int(Number(snapshot.Value, "quotaRemaining", "quota_remaining"))
			quotaObservedAt = observedAt
		}
	}
	if remaining <= 0 {
		return nil
	}
	known := insightListingSnapshots(insights)
	adsByListing := LatestByListing(ads)
	offsiteByListing := LatestByListing(offsite)
	promotionSet := promotionListingSet(promotions)
	identifiers := make(map[string]struct{})
	for identifier := range adsByListing {
		identifiers[identifier] = struct{}{}
	}
	for identifier := range offsiteByListing {
		identifiers[identifier] = struct{}{}
	}
	for identifier := range promotionSet {
		identifiers[identifier] = struct{}{}
	}

	recommendations := make([]QuotaRecommendation, 0)
	for identifier := range identifiers {
		if snapshot, found := known[identifier]; found && now.Sub(ObservedAt(snapshot)) <= 7*24*time.Hour {
			continue
		}
		score := nestedNumber(adsByListing[identifier].Value, "totalStats", "impressionCount") +
			10*nestedNumber(adsByListing[identifier].Value, "totalStats", "spentTotal") +
			25*Number(offsiteByListing[identifier].Value, "clicksCount", "clicks_count")
		if _, found := promotionSet[identifier]; found {
			score += 100
		}
		recommendations = append(recommendations, QuotaRecommendation{
			ListingID: identifier, PriorityScore: score, Reason: "missing-or-stale-demand-evidence",
		})
	}
	sort.Slice(recommendations, func(left, right int) bool {
		if recommendations[left].PriorityScore == recommendations[right].PriorityScore {
			return recommendations[left].ListingID < recommendations[right].ListingID
		}
		return recommendations[left].PriorityScore > recommendations[right].PriorityScore
	})
	if len(recommendations) > remaining {
		recommendations = recommendations[:remaining]
	}
	for index := range recommendations {
		recommendations[index].RecommendedRank = index + 1
	}
	return recommendations
}

// VisibilityGap compares explicit listing-keyword observations with paid performance.
type VisibilityGap struct {
	ListingID        string  `json:"listing_id"`
	Keyword          string  `json:"keyword"`
	Searches         float64 `json:"searches"`
	ObservedRank     int     `json:"observed_rank"`
	PaidOrders       float64 `json:"paid_orders"`
	PaidDataObserved bool    `json:"paid_data_observed"`
	Classification   string  `json:"classification"`
}

// VisibilityPerformanceGaps never infers mappings from listing titles.
func VisibilityPerformanceGaps(insights, ads, offsite []Snapshot) []VisibilityGap {
	const highDemandSearchThreshold = 100

	adsByListing := LatestByListing(ads)
	offsiteByListing := LatestByListing(offsite)
	results := make([]VisibilityGap, 0)
	for _, snapshot := range insights {
		keyword := String(snapshot.Value, "keyword", "searchTerm", "search_term")
		searches := Number(snapshot.Value, "searches", "searchVolume", "search_volume")
		rawListings, _ := snapshot.Value["listings"].([]any)
		for index, rawListing := range rawListings {
			listing, ok := rawListing.(map[string]any)
			if !ok {
				continue
			}
			identifier := ListingID(listing)
			if identifier == "" {
				continue
			}
			_, hasAds := adsByListing[identifier]
			_, hasOffsite := offsiteByListing[identifier]
			paidDataObserved := hasAds || hasOffsite
			paidOrders := nestedNumber(adsByListing[identifier].Value, "totalStats", "conversions") +
				Number(offsiteByListing[identifier].Value, "ordersCount", "orders_count")
			classification := "observed"
			switch {
			case searches >= highDemandSearchThreshold && index >= 20:
				classification = "high-demand-low-visibility"
			case index < 10 && !paidDataObserved:
				classification = "insufficient-paid-data"
			case index < 10 && paidOrders == 0:
				classification = "high-visibility-weak-paid-performance"
			}
			results = append(results, VisibilityGap{
				ListingID: identifier, Keyword: keyword, Searches: searches,
				ObservedRank: index + 1, PaidOrders: paidOrders,
				PaidDataObserved: paidDataObserved, Classification: classification,
			})
		}
	}
	sort.Slice(results, func(left, right int) bool {
		if results[left].ListingID == results[right].ListingID {
			return results[left].Keyword < results[right].Keyword
		}
		return results[left].ListingID < results[right].ListingID
	})
	return results
}

func insightListingSnapshots(insights []Snapshot) map[string]Snapshot {
	result := make(map[string]Snapshot)
	for _, snapshot := range insights {
		if identifier := ListingID(snapshot.Value); identifier != "" {
			result[identifier] = snapshot
		}
		rawListings, _ := snapshot.Value["listings"].([]any)
		for _, rawListing := range rawListings {
			listing, ok := rawListing.(map[string]any)
			if !ok {
				continue
			}
			identifier := ListingID(listing)
			if identifier == "" {
				continue
			}
			copyValue := make(map[string]any, len(listing)+2)
			for key, value := range listing {
				copyValue[key] = value
			}
			copyValue["keyword"] = String(snapshot.Value, "keyword", "searchTerm")
			copyValue["searches"] = Number(snapshot.Value, "searches", "searchVolume")
			copyValue["_observed_at"] = snapshot.ObservedAt.Format(time.RFC3339Nano)
			result[identifier] = Snapshot{Resource: snapshot.Resource, ObservedAt: snapshot.ObservedAt, Value: copyValue}
		}
	}
	return result
}

func promotionListingSet(promotions []Snapshot) map[string]struct{} {
	result := make(map[string]struct{})
	for _, snapshot := range promotions {
		for _, identifier := range listingIDs(snapshot.Value["reward_set_listing_ids"]) {
			result[identifier] = struct{}{}
		}
	}
	return result
}

func latestSummary(snapshots []Snapshot, keys ...string) (Snapshot, bool) {
	filtered := make([]Snapshot, 0)
	for _, snapshot := range snapshots {
		for _, key := range keys {
			if _, found := snapshot.Value[key]; found {
				filtered = append(filtered, snapshot)
				break
			}
		}
	}
	return Latest(filtered)
}

func ordersForListing(identifier string, ads, offsite map[string]Snapshot) float64 {
	return nestedNumber(ads[identifier].Value, "totalStats", "conversions") +
		Number(offsite[identifier].Value, "ordersCount", "orders_count")
}

func newestTime(values ...time.Time) time.Time {
	var newest time.Time
	for _, value := range values {
		if value.After(newest) {
			newest = value
		}
	}
	return newest
}

func ratio(numerator, denominator float64) *float64 {
	if denominator == 0 {
		return nil
	}
	value := numerator / denominator
	return &value
}
