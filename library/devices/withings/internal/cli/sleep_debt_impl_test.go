// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored synthetic-data test for `sleep debt`.

package cli

import (
	"testing"
	"time"
)

// sleepNight builds a sleep summary JSON value with total_sleep_time seconds.
func sleepNight(id, date string, totalSleepSeconds int) map[string]any {
	return map[string]any{
		"id":   id,
		"date": date,
		"data": map[string]any{"total_sleep_time": totalSleepSeconds},
	}
}

func TestComputeSleepDebt_PositiveDebt(t *testing.T) {
	s, _ := newTestStore(t)

	// Three nights, each 6h (21600s) against an 8h target => 2h debt/night,
	// 6h cumulative.
	for i, ago := range []int{1, 3, 5} {
		day := daysAgoYMD(ago)
		upsertJSON(t, s, "sleep", "n"+day+itoa(i), sleepNight("n"+day, day, 6*3600))
	}

	res, err := computeSleepDebt(s, time.Now().Add(-14*24*time.Hour), 8*time.Hour, "14d")
	if err != nil {
		t.Fatalf("computeSleepDebt: %v", err)
	}
	if res.Nights != 3 {
		t.Fatalf("nights = %d, want 3", res.Nights)
	}
	if res.TargetHours != 8 {
		t.Errorf("target_hours = %v, want 8", res.TargetHours)
	}
	if res.AvgSleepHours != 6 {
		t.Errorf("avg_sleep_hours = %v, want 6", res.AvgSleepHours)
	}
	if res.CumulativeDebt <= 0 {
		t.Fatalf("cumulative_debt_hours = %v, want > 0", res.CumulativeDebt)
	}
	if res.CumulativeDebt != 6 {
		t.Errorf("cumulative_debt_hours = %v, want 6", res.CumulativeDebt)
	}
}

func TestComputeSleepDebt_SurplusIsNegative(t *testing.T) {
	s, _ := newTestStore(t)
	// Two nights of 9h against an 8h target => -1h/night, -2h cumulative.
	for i, ago := range []int{2, 4} {
		day := daysAgoYMD(ago)
		upsertJSON(t, s, "sleep", "n"+day+itoa(i), sleepNight("n"+day, day, 9*3600))
	}
	res, err := computeSleepDebt(s, time.Now().Add(-14*24*time.Hour), 8*time.Hour, "14d")
	if err != nil {
		t.Fatalf("computeSleepDebt: %v", err)
	}
	if res.CumulativeDebt != -2 {
		t.Errorf("cumulative_debt_hours = %v, want -2", res.CumulativeDebt)
	}
}
