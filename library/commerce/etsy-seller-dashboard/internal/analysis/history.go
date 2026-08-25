// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

package analysis

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"
)

// ErrInsufficientHistory indicates that a deterministic historical comparison cannot be made.
var ErrInsufficientHistory = errors.New("insufficient history")

// PromotionLift reports observed, non-causal changes around a promotion window.
type PromotionLift struct {
	PromotionID       string    `json:"promotion_id"`
	Source            string    `json:"source"`
	Metric            string    `json:"metric"`
	Unit              string    `json:"unit"`
	BaselineStart     time.Time `json:"baseline_start"`
	BaselineEnd       time.Time `json:"baseline_end"`
	PromotionStart    time.Time `json:"promotion_start"`
	PromotionEnd      time.Time `json:"promotion_end"`
	BaselineActivity  float64   `json:"baseline_activity"`
	PromotionActivity float64   `json:"promotion_activity"`
	AbsoluteDelta     float64   `json:"absolute_delta"`
	PercentDelta      *float64  `json:"percent_delta"`
	Interpretation    string    `json:"interpretation"`
}

// ObservedPromotionLift compares matching Offsite Ads order windows. Ads is
// intentionally excluded because its captured listing response has no request
// date window that can be aligned exactly with a promotion.
func ObservedPromotionLift(promotionID string, promotions, ads, offsite []Snapshot) (PromotionLift, error) {
	_ = ads
	selected, found := latestPromotion(promotionID, promotions)
	if !found {
		return PromotionLift{}, errors.New("promotion not found")
	}

	start := epochTime(Number(selected.Value, "start_date", "start_date_ms"))
	end := epochTime(Number(selected.Value, "end_date", "end_date_ms"))
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return PromotionLift{}, ErrInsufficientHistory
	}
	start = calendarDay(start)
	end = calendarDay(end)
	durationDays := int(end.Sub(start)/(24*time.Hour)) + 1
	baselineStart := start.AddDate(0, 0, -durationDays)
	baselineEnd := start.AddDate(0, 0, -1)
	listingSet := make(map[string]struct{})
	for _, identifier := range listingIDs(selected.Value["reward_set_listing_ids"]) {
		listingSet[identifier] = struct{}{}
	}

	labels := []string{"summary", "listings"}
	if len(listingSet) > 0 {
		labels = []string{"listings"}
	}
	for _, label := range labels {
		baseline := windowOrders(offsite, listingSet, baselineStart, baselineEnd, label)
		during := windowOrders(offsite, listingSet, start, end, label)
		if baseline.samples == 0 || during.samples == 0 {
			continue
		}
		return PromotionLift{
			PromotionID: promotionID, Source: "offsite-ads", Metric: "orders", Unit: "count",
			BaselineStart: baselineStart, BaselineEnd: baselineEnd,
			PromotionStart: start, PromotionEnd: end,
			BaselineActivity: baseline.value, PromotionActivity: during.value,
			AbsoluteDelta:  during.value - baseline.value,
			PercentDelta:   PercentDelta(during.value, baseline.value),
			Interpretation: "Observed change only; Etsy attribution data does not establish that the promotion caused the difference.",
		}, nil
	}
	return PromotionLift{}, ErrInsufficientHistory
}

func latestPromotion(promotionID string, promotions []Snapshot) (Snapshot, bool) {
	var selected Snapshot
	found := false
	for _, snapshot := range promotions {
		if String(snapshot.Value, "promotion_id") != promotionID &&
			strconv.FormatInt(int64(Number(snapshot.Value, "promotion_id")), 10) != promotionID {
			continue
		}
		if !found || ObservedAt(snapshot).After(ObservedAt(selected)) {
			selected = snapshot
			found = true
		}
	}
	return selected, found
}

// Anomaly is one deterministic weekly exception.
type Anomaly struct {
	Resource       string   `json:"resource"`
	Metric         string   `json:"metric"`
	Unit           string   `json:"unit"`
	Week           string   `json:"week"`
	Value          float64  `json:"value"`
	BaselineMedian float64  `json:"baseline_median"`
	AbsoluteDelta  float64  `json:"absolute_delta"`
	PercentDelta   *float64 `json:"percent_delta"`
	Direction      string   `json:"direction"`
	Interpretation string   `json:"interpretation"`
}

// CrossSurfaceAnomalies compares one compatible metric per surface. For each
// entity and week, only the newest snapshot is counted.
func CrossSurfaceAnomalies(resources map[string][]Snapshot, threshold float64) ([]Anomaly, error) {
	if threshold <= 0 {
		threshold = 0.5
	}
	results := make([]Anomaly, 0)
	hasEnoughHistory := false
	for resource, snapshots := range resources {
		weekly := latestWeeklyMetrics(resource, snapshots)
		if len(weekly.values) < 3 {
			continue
		}
		hasEnoughHistory = true
		weeks := make([]string, 0, len(weekly.values))
		for week := range weekly.values {
			weeks = append(weeks, week)
		}
		sort.Strings(weeks)
		latestWeek := weeks[len(weeks)-1]
		baselineValues := make([]float64, 0, len(weeks)-1)
		for _, week := range weeks[:len(weeks)-1] {
			baselineValues = append(baselineValues, weekly.values[week])
		}
		baseline := median(baselineValues)
		current := weekly.values[latestWeek]
		percent := PercentDelta(current, baseline)
		if percent == nil || math.Abs(*percent) < threshold || current == baseline {
			continue
		}
		direction := "down"
		if current > baseline {
			direction = "up"
		}
		results = append(results, Anomaly{
			Resource: resource, Metric: weekly.metric, Unit: weekly.unit,
			Week: latestWeek, Value: current,
			BaselineMedian: baseline, AbsoluteDelta: current - baseline,
			PercentDelta: percent, Direction: direction,
			Interpretation: "Coincident source movement only; this result does not claim causation.",
		})
	}
	if !hasEnoughHistory {
		return nil, ErrInsufficientHistory
	}
	sort.Slice(results, func(left, right int) bool { return results[left].Resource < results[right].Resource })
	return results, nil
}

type windowActivity struct {
	value   float64
	samples int
}

func windowOrders(
	snapshots []Snapshot,
	listingSet map[string]struct{},
	start time.Time,
	end time.Time,
	observationType string,
) windowActivity {
	startDate := start.UTC().Format("2006-01-02")
	endDate := end.UTC().Format("2006-01-02")
	latest := make(map[string]Snapshot)
	for _, snapshot := range snapshots {
		if String(snapshot.Value, "_observation_type") != observationType ||
			String(snapshot.Value, "_request_start_date") != startDate ||
			String(snapshot.Value, "_request_end_date") != endDate {
			continue
		}
		identifier := ListingID(snapshot.Value)
		if observationType == "listings" {
			if identifier == "" {
				continue
			}
			if len(listingSet) > 0 {
				if _, included := listingSet[identifier]; !included {
					continue
				}
			}
		} else {
			identifier = observationType
		}
		if previous, found := latest[identifier]; !found || ObservedAt(snapshot).After(ObservedAt(previous)) {
			latest[identifier] = snapshot
		}
	}
	result := windowActivity{}
	for _, snapshot := range latest {
		result.value += Number(snapshot.Value, "orders", "ordersCount", "orders_count", "order_count")
		result.samples++
	}
	return result
}

type weeklyMetrics struct {
	metric string
	unit   string
	values map[string]float64
}

type observedMetric struct {
	observed time.Time
	value    float64
}

func latestWeeklyMetrics(resource string, snapshots []Snapshot) weeklyMetrics {
	result := weeklyMetrics{values: make(map[string]float64)}
	latest := make(map[string]observedMetric)
	for _, snapshot := range snapshots {
		metric, unit, value, entity, ok := canonicalMetric(resource, snapshot)
		if !ok {
			continue
		}
		observed := ObservedAt(snapshot)
		if observed.IsZero() {
			continue
		}
		year, week := observed.ISOWeek()
		weekKey := fmt.Sprintf("%04d-W%02d", year, week)
		key := weekKey + "\x00" + entity
		if previous, found := latest[key]; !found || observed.After(previous.observed) {
			latest[key] = observedMetric{observed: observed, value: value}
		}
		result.metric = metric
		result.unit = unit
	}
	for key, metric := range latest {
		week, _, _ := splitMetricKey(key)
		result.values[week] += metric.value
	}
	return result
}

func canonicalMetric(resource string, snapshot Snapshot) (string, string, float64, string, bool) {
	value := snapshot.Value
	switch resource {
	case "marketplace-insights":
		if !hasAnyKey(value, "searches", "searchVolume", "search_volume") {
			return "", "", 0, "", false
		}
		entity := String(value, "keyword", "searchTerm", "search_term")
		if entity == "" {
			entity = String(value, "_observation_type")
		}
		return "searches", "count", Number(value, "searches", "searchVolume", "search_volume"), entity, true
	case "ads":
		if String(value, "_observation_type") != "" && String(value, "_observation_type") != "listings" {
			return "", "", 0, "", false
		}
		return "conversions", "count", nestedNumber(value, "totalStats", "conversions"), ListingID(value), true
	case "offsite-ads":
		if String(value, "_observation_type") != "summary" || !hasAnyKey(value, "orders", "orders_count") {
			return "", "", 0, "", false
		}
		return "orders", "count", Number(value, "orders", "orders_count"), "summary", true
	case "promotions":
		if !hasAnyKey(value, "uses", "uses_count", "redemptions", "redemption_count") {
			return "", "", 0, "", false
		}
		entity := String(value, "promotion_id")
		if entity == "" {
			entity = String(value, "id")
		}
		return "redemptions", "count", Number(value, "uses", "uses_count", "redemptions", "redemption_count"), entity, true
	default:
		return "", "", 0, "", false
	}
}

func hasAnyKey(value map[string]any, keys ...string) bool {
	for _, key := range keys {
		if _, found := value[key]; found {
			return true
		}
	}
	return false
}

func splitMetricKey(key string) (string, string, bool) {
	for index := range key {
		if key[index] == 0 {
			return key[:index], key[index+1:], true
		}
	}
	return key, "", false
}

func epochTime(value float64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	if value > 1e12 {
		value /= 1000
	}
	return time.Unix(int64(value), 0).UTC()
}

func calendarDay(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	middle := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[middle-1] + sorted[middle]) / 2
	}
	return sorted[middle]
}
