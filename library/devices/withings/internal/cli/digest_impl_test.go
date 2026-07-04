// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored synthetic-data test for `digest`.

package cli

import (
	"testing"
	"time"
)

func TestComputeDigest_LatestWeightAndCounts(t *testing.T) {
	s, _ := newTestStore(t)

	// Two recent weight groups within 24h window boundary (use a 3d window in
	// the test to be robust against midnight rollover).
	upsertJSON(t, s, "measure", "w-old", measureGrp(1, daysAgoEpoch(2),
		measureValue{Value: 80000, Type: 1, Unit: -3}))
	upsertJSON(t, s, "measure", "w-new", measureGrp(2, daysAgoEpoch(0),
		measureValue{Value: 79500, Type: 1, Unit: -3},
		measureValue{Value: 60, Type: 11, Unit: 0})) // resting pulse 60
	// A BP reading.
	upsertJSON(t, s, "measure", "bp", measureGrp(3, daysAgoEpoch(1),
		measureValue{Value: 125, Type: 10, Unit: 0},
		measureValue{Value: 80, Type: 9, Unit: 0}))
	// Activity steps today.
	upsertJSON(t, s, "activity", "a-today", map[string]any{
		"id": "a-today", "date": daysAgoYMD(0), "steps": 8200, "calories": 540.0,
	})
	// A sleep score last night.
	upsertJSON(t, s, "sleep", "s1", map[string]any{
		"id": "s1", "date": daysAgoYMD(0), "data": map[string]any{"sleep_score": 77},
	})
	// An AFib event.
	upsertJSON(t, s, "heart", "h1", map[string]any{
		"timestamp": daysAgoEpoch(0),
		"data":      map[string]any{"ecg": map[string]any{"afib": 1}},
	})

	res, err := computeDigest(s, time.Now().Add(-3*24*time.Hour), "72h")
	if err != nil {
		t.Fatalf("computeDigest: %v", err)
	}
	if res.LatestWeight == nil {
		t.Fatal("latest_weight is nil, want present")
	}
	if *res.LatestWeight != 79.5 {
		t.Errorf("latest_weight = %v, want 79.5", *res.LatestWeight)
	}
	if res.WeightChange == nil || *res.WeightChange != -0.5 {
		t.Errorf("weight_change = %v, want -0.5", res.WeightChange)
	}
	if res.StepsToday != 8200 {
		t.Errorf("steps_today = %d, want 8200", res.StepsToday)
	}
	if res.LastSleepScore == nil || *res.LastSleepScore != 77 {
		t.Errorf("last_sleep_score = %v, want 77", res.LastSleepScore)
	}
	if res.RestingHR == nil || *res.RestingHR != 60 {
		t.Errorf("resting_hr = %v, want 60", res.RestingHR)
	}
	if res.NewAfibEvents != 1 {
		t.Errorf("new_afib_events = %d, want 1", res.NewAfibEvents)
	}
	if res.NewBPReadings != 1 {
		t.Errorf("new_bp_readings = %d, want 1", res.NewBPReadings)
	}
}
