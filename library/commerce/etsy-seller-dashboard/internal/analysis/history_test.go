// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

package analysis

import (
	"errors"
	"testing"
	"time"
)

func TestObservedPromotionLiftUsesEqualLengthBaseline(t *testing.T) {
	start := time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC)
	end := start.Add(6 * 24 * time.Hour)
	promotions := []Snapshot{snapshot("promotions", end, map[string]any{
		"promotion_id":           float64(42),
		"start_date":             float64(start.Unix()),
		"end_date":               float64(end.Unix()),
		"reward_set_listing_ids": []any{float64(1)},
	})}
	offsite := []Snapshot{
		offsiteWindowSnapshot(1, 12, start.Add(-time.Hour), "2026-01-01", "2026-01-07"),
		offsiteWindowSnapshot(1, 100, start.Add(time.Hour), "2026-01-08", "2026-01-14"),
		offsiteWindowSnapshot(1, 36, start.Add(2*time.Hour), "2026-01-08", "2026-01-14"),
	}

	result, err := ObservedPromotionLift("42", promotions, nil, offsite)
	if err != nil {
		t.Fatal(err)
	}
	if result.BaselineActivity != 12 || result.PromotionActivity != 36 {
		t.Fatalf("unexpected activity totals: %#v", result)
	}
	baselineDays := int(result.BaselineEnd.Sub(result.BaselineStart)/(24*time.Hour)) + 1
	promotionDays := int(result.PromotionEnd.Sub(result.PromotionStart)/(24*time.Hour)) + 1
	if !result.BaselineEnd.AddDate(0, 0, 1).Equal(result.PromotionStart) ||
		baselineDays != promotionDays {
		t.Fatalf("windows are not equal: %#v", result)
	}
	if result.Source != "offsite-ads" || result.Metric != "orders" || result.Unit != "count" {
		t.Fatalf("metric provenance missing: %#v", result)
	}
}

func TestCrossSurfaceAnomaliesSortISOWeeksChronologically(t *testing.T) {
	weekNine := time.Date(2026, 2, 23, 0, 0, 0, 0, time.UTC)
	resources := map[string][]Snapshot{
		"ads": {
			snapshot("ads", weekNine, adsValue(1, 0, 10, 0, 0)),
			snapshot("ads", weekNine.Add(7*24*time.Hour), adsValue(1, 0, 10, 0, 0)),
			snapshot("ads", weekNine.Add(14*24*time.Hour), adsValue(1, 0, 40, 0, 0)),
		},
	}
	results, err := CrossSurfaceAnomalies(resources, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Week != "2026-W11" {
		t.Fatalf("latest ISO week selected incorrectly: %#v", results)
	}
}

func TestObservedPromotionLiftReturnsTypedInsufficientHistory(t *testing.T) {
	promotions := []Snapshot{snapshot("promotions", time.Now().UTC(), map[string]any{
		"promotion_id": float64(42),
	})}
	_, err := ObservedPromotionLift("42", promotions, nil, nil)
	if !errors.Is(err, ErrInsufficientHistory) {
		t.Fatalf("got %v, want ErrInsufficientHistory", err)
	}
}

func TestCrossSurfaceAnomaliesRequireThreeWeeks(t *testing.T) {
	start := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	resources := map[string][]Snapshot{
		"ads": {
			snapshot("ads", start, adsValue(1, 0, 10, 0, 0)),
			snapshot("ads", start.Add(7*24*time.Hour), adsValue(1, 0, 10, 0, 0)),
			snapshot("ads", start.Add(14*24*time.Hour), adsValue(1, 0, 400, 0, 0)),
			snapshot("ads", start.Add(14*24*time.Hour+time.Hour), adsValue(1, 0, 40, 0, 0)),
		},
	}
	results, err := CrossSurfaceAnomalies(resources, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Direction != "up" {
		t.Fatalf("unexpected anomalies: %#v", results)
	}
	if results[0].Value != 40 || results[0].Metric != "conversions" || results[0].Unit != "count" {
		t.Fatalf("duplicate snapshots or metric provenance are wrong: %#v", results[0])
	}

	_, err = CrossSurfaceAnomalies(map[string][]Snapshot{"ads": resources["ads"][:2]}, 0.5)
	if !errors.Is(err, ErrInsufficientHistory) {
		t.Fatalf("got %v, want ErrInsufficientHistory", err)
	}
}

func TestObservedPromotionLiftRejectsObservationTimeWithoutRequestWindow(t *testing.T) {
	start := time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC)
	end := start.Add(7 * 24 * time.Hour)
	promotions := []Snapshot{snapshot("promotions", end, map[string]any{
		"promotion_id": float64(42), "start_date": float64(start.Unix()),
		"end_date": float64(end.Unix()), "reward_set_listing_ids": []any{float64(1)},
	})}
	offsite := []Snapshot{
		snapshot("offsite-ads", start.Add(-time.Hour), map[string]any{
			"listingId": float64(1), "ordersCount": float64(12), "_observation_type": "listings",
		}),
		snapshot("offsite-ads", start.Add(time.Hour), map[string]any{
			"listingId": float64(1), "ordersCount": float64(36), "_observation_type": "listings",
		}),
	}

	_, err := ObservedPromotionLift("42", promotions, nil, offsite)
	if !errors.Is(err, ErrInsufficientHistory) {
		t.Fatalf("got %v, want ErrInsufficientHistory", err)
	}
}

func offsiteWindowSnapshot(
	listingID int,
	orders float64,
	observedAt time.Time,
	startDate string,
	endDate string,
) Snapshot {
	return snapshot("offsite-ads", observedAt, map[string]any{
		"listingId": float64(listingID), "ordersCount": orders,
		"_observation_type":   "listings",
		"_request_start_date": startDate, "_request_end_date": endDate,
	})
}
