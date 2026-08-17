// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored synthetic-data test for `correlate`.

package cli

import (
	"math"
	"testing"
	"time"
)

func TestComputeCorrelate_PerfectPositive(t *testing.T) {
	s, _ := newTestStore(t)

	// Five days, weight and fat_ratio moving in perfect lockstep:
	// weight = 78,79,80,81,82 ; fat_ratio = 18,19,20,21,22.
	for i := 0; i < 5; i++ {
		ago := 10 - i*2 // 10,8,6,4,2 days ago (ascending in time)
		epoch := daysAgoEpoch(ago)
		weight := 78 + i // kg
		fatRatio := 18 + i
		upsertJSON(t, s, "measure", "g"+itoa(i), measureGrp(int64(i+1), epoch,
			measureValue{Value: weight * 1000, Type: 1, Unit: -3},    // weight kg
			measureValue{Value: fatRatio * 1000, Type: 6, Unit: -3})) // fat ratio %
	}

	res, err := computeCorrelate(s, "weight", "fat_ratio", time.Now().Add(-90*24*time.Hour))
	if err != nil {
		t.Fatalf("computeCorrelate: %v", err)
	}
	if res.MatchedDays != 5 {
		t.Fatalf("matched_days = %d, want 5", res.MatchedDays)
	}
	if res.PearsonR == nil {
		t.Fatal("pearson_r is nil, want ~1.0")
	}
	if math.Abs(*res.PearsonR-1.0) > 1e-6 {
		t.Errorf("pearson_r = %v, want ~1.0", *res.PearsonR)
	}
	// At zero lag the series are identical-day matched and perfectly
	// correlated, so the best lag is 0 with r ~ 1.0.
	if res.BestLagR == nil || math.Abs(*res.BestLagR-1.0) > 1e-6 {
		t.Errorf("best_lag_r = %v, want ~1.0", res.BestLagR)
	}
	if res.BestLagDays != 0 {
		t.Errorf("best_lag_days = %d, want 0", res.BestLagDays)
	}
}

func TestComputeCorrelate_LaggedSeries(t *testing.T) {
	s, _ := newTestStore(t)

	// steps leads sleep_score by one day: a high-step day is followed by a
	// high sleep score the next day. Construct so the best lag is +1.
	base := []int{10, 9, 8, 7, 6, 5} // days ago, descending in time
	stepVals := []int{3000, 6000, 4000, 8000, 5000, 9000}
	for i, ago := range base {
		day := daysAgoYMD(ago)
		upsertJSON(t, s, "activity", "a"+day, map[string]any{
			"id": "a" + day, "date": day, "steps": stepVals[i],
		})
		// sleep score on day D mirrors steps on day D-1 (the prior calendar day).
		if i > 0 {
			score := 40 + stepVals[i-1]/200 // monotic in prior-day steps
			upsertJSON(t, s, "sleep", "s"+day, map[string]any{
				"id": "s" + day, "date": day, "data": map[string]any{"sleep_score": score},
			})
		}
	}

	res, err := computeCorrelate(s, "steps", "sleep_score", time.Now().Add(-90*24*time.Hour))
	if err != nil {
		t.Fatalf("computeCorrelate: %v", err)
	}
	// The best-lag correlation should be strong and positive at lag +1
	// (steps today vs sleep tomorrow).
	if res.BestLagR == nil {
		t.Fatal("best_lag_r is nil")
	}
	if res.BestLagDays != 1 {
		t.Errorf("best_lag_days = %d, want 1; best_lag_r=%v", res.BestLagDays, res.BestLagR)
	}
	if *res.BestLagR < 0.9 {
		t.Errorf("best_lag_r = %v, want strong positive (>=0.9)", *res.BestLagR)
	}
}

func TestComputeCorrelate_UnknownMetricRejected(t *testing.T) {
	// Unknown metric names are rejected at the command layer; verify the
	// helper that backs that decision.
	if isKnownMetric("bogus") {
		t.Error("isKnownMetric(bogus) = true, want false")
	}
	for _, m := range correlateMetrics {
		if !isKnownMetric(m) {
			t.Errorf("isKnownMetric(%q) = false, want true", m)
		}
	}
}
