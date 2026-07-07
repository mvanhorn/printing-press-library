// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored synthetic-data test for `recovery`.

package cli

import (
	"testing"
	"time"
)

// workoutRow builds a workouts JSON value with HR-zone seconds in data.
func workoutRow(id, date string, zoneSecs [4]float64) map[string]any {
	return map[string]any{
		"id":       id,
		"date":     date,
		"category": 2,
		"data": map[string]any{
			"hr_zone_0": zoneSecs[0],
			"hr_zone_1": zoneSecs[1],
			"hr_zone_2": zoneSecs[2],
			"hr_zone_3": zoneSecs[3],
		},
	}
}

// sleepSummaryRow builds a sleep summary JSON value with a sleep score.
func sleepSummaryRow(id, date string, score int) map[string]any {
	return map[string]any{
		"id":   id,
		"date": date,
		"data": map[string]any{"sleep_score": score},
	}
}

func TestComputeRecovery_FlagSetOnLoadUpRecoveryDown(t *testing.T) {
	s, _ := newTestStore(t)

	// Early half (older days): low load, high sleep score.
	// Late half (recent days): high load, low sleep score.
	// Expect Summary.Flag == true.
	for _, d := range []struct {
		ago   int
		zone  [4]float64 // seconds
		sleep int
	}{
		{ago: 12, zone: [4]float64{600, 0, 0, 0}, sleep: 85},        // 10 min, good sleep
		{ago: 10, zone: [4]float64{600, 0, 0, 0}, sleep: 84},        // 10 min, good sleep
		{ago: 3, zone: [4]float64{1800, 1800, 600, 600}, sleep: 40}, // 80 min, poor sleep
		{ago: 1, zone: [4]float64{1800, 1800, 900, 900}, sleep: 38}, // 90 min, poor sleep
	} {
		day := daysAgoYMD(d.ago)
		upsertJSON(t, s, "workouts", "w"+day, workoutRow("w"+day, day, d.zone))
		upsertJSON(t, s, "sleep", "s"+day, sleepSummaryRow("s"+day, day, d.sleep))
	}

	res, err := computeRecovery(s, time.Now().Add(-14*24*time.Hour), "14d")
	if err != nil {
		t.Fatalf("computeRecovery: %v", err)
	}
	if len(res.Days) != 4 {
		t.Fatalf("expected 4 days, got %d (%+v)", len(res.Days), res.Days)
	}
	if !res.Summary.Flag {
		t.Errorf("expected overtraining flag to be set; summary=%+v days=%+v", res.Summary, res.Days)
	}
	if res.Summary.Trend != "load up, recovery down" {
		t.Errorf("trend = %q, want %q", res.Summary.Trend, "load up, recovery down")
	}

	// Spot-check a row carries the load minutes and sleep score.
	last := res.Days[len(res.Days)-1]
	if last.LoadMin <= 0 {
		t.Errorf("last day load_min = %v, want > 0", last.LoadMin)
	}
	if last.SleepScore == 0 {
		t.Errorf("last day sleep_score = 0, want non-zero")
	}
}

func TestComputeRecovery_NoFlagWhenStable(t *testing.T) {
	s, _ := newTestStore(t)
	// Consistent moderate load and good sleep across the window: no flag.
	for _, ago := range []int{12, 9, 5, 2} {
		day := daysAgoYMD(ago)
		upsertJSON(t, s, "workouts", "w"+day, workoutRow("w"+day, day, [4]float64{1200, 0, 0, 0}))
		upsertJSON(t, s, "sleep", "s"+day, sleepSummaryRow("s"+day, day, 80))
	}
	res, err := computeRecovery(s, time.Now().Add(-14*24*time.Hour), "14d")
	if err != nil {
		t.Fatalf("computeRecovery: %v", err)
	}
	if res.Summary.Flag {
		t.Errorf("did not expect flag for stable load/recovery; summary=%+v", res.Summary)
	}
}
