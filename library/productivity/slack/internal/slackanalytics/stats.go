// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package slackanalytics

import (
	"math"
	"sort"
	"time"
)

// MedianFloat returns the median of values. The input is copied before
// sorting so the caller's slice ordering survives. An empty input returns 0.
func MedianFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

// MedianDuration returns the median of durations, rounded to the nearest
// second so reply-latency output stays readable. Empty input returns 0.
func MedianDuration(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	seconds := make([]float64, 0, len(values))
	for _, v := range values {
		seconds = append(seconds, v.Seconds())
	}
	return time.Duration(math.Round(MedianFloat(seconds))) * time.Second
}

// PerDay converts a count over a window into a per-day rate rounded to two
// decimals. A non-positive window returns 0 rather than an infinity.
func PerDay(count int, window time.Duration) float64 {
	if count <= 0 || window <= 0 {
		return 0
	}
	days := window.Hours() / 24
	if days <= 0 {
		return 0
	}
	return math.Round(float64(count)/days*100) / 100
}

// LargestGap returns the biggest interval between consecutive timestamps in
// a synced range — the practical measure of "where are the holes in this
// mirror". Fewer than two timestamps means no measurable gap.
func LargestGap(times []time.Time) time.Duration {
	if len(times) < 2 {
		return 0
	}
	sorted := append([]time.Time(nil), times...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Before(sorted[j]) })
	var largest time.Duration
	for i := 1; i < len(sorted); i++ {
		if gap := sorted[i].Sub(sorted[i-1]); gap > largest {
			largest = gap
		}
	}
	return largest
}
