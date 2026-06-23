// Copyright 2026 Nimrod Astarhan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"math"
	"testing"
	"time"
)

func TestComputeJobStats_Empty(t *testing.T) {
	s := computeJobStats(1, nil)
	if s.TotalRuns != 0 {
		t.Errorf("expected 0 runs, got %d", s.TotalRuns)
	}
	if s.SuccessRatePct != 0 {
		t.Errorf("expected 0%% success, got %.2f", s.SuccessRatePct)
	}
}

func TestComputeJobStats_AllSuccess(t *testing.T) {
	rows := []runsStatRow{
		{JobDefinitionID: 1, IsSuccess: true, DurationSec: 100, CreatedAt: time.Now()},
		{JobDefinitionID: 1, IsSuccess: true, DurationSec: 200, CreatedAt: time.Now()},
		{JobDefinitionID: 1, IsSuccess: true, DurationSec: 300, CreatedAt: time.Now()},
	}
	s := computeJobStats(1, rows)
	if s.TotalRuns != 3 {
		t.Errorf("expected 3 runs, got %d", s.TotalRuns)
	}
	if s.SuccessCount != 3 {
		t.Errorf("expected 3 successes, got %d", s.SuccessCount)
	}
	if s.SuccessRatePct != 100.0 {
		t.Errorf("expected 100%% success rate, got %.2f", s.SuccessRatePct)
	}
	if s.FailureCount != 0 {
		t.Errorf("expected 0 failures, got %d", s.FailureCount)
	}
	wantAvg := 200.0
	if math.Abs(s.AvgDurationSec-wantAvg) > 0.01 {
		t.Errorf("expected avg %.2f, got %.2f", wantAvg, s.AvgDurationSec)
	}
	// p95 of [100,200,300] = ceil(0.95*3)-1 = 2 → 300
	if math.Abs(s.P95DurationSec-300.0) > 0.01 {
		t.Errorf("expected p95=300, got %.2f", s.P95DurationSec)
	}
}

func TestComputeJobStats_MixedSuccess(t *testing.T) {
	rows := []runsStatRow{
		{JobDefinitionID: 2, IsSuccess: true, DurationSec: 60, CreatedAt: time.Now()},
		{JobDefinitionID: 2, IsSuccess: false, DurationSec: 30, CreatedAt: time.Now()},
		{JobDefinitionID: 2, IsSuccess: true, DurationSec: 90, CreatedAt: time.Now()},
		{JobDefinitionID: 2, IsSuccess: false, DurationSec: 0, CreatedAt: time.Now()}, // zero dur excluded from timing
	}
	s := computeJobStats(2, rows)
	if s.TotalRuns != 4 {
		t.Errorf("expected 4 runs, got %d", s.TotalRuns)
	}
	if s.SuccessCount != 2 {
		t.Errorf("expected 2 successes, got %d", s.SuccessCount)
	}
	if s.FailureCount != 2 {
		t.Errorf("expected 2 failures, got %d", s.FailureCount)
	}
	if math.Abs(s.SuccessRatePct-50.0) > 0.01 {
		t.Errorf("expected 50%% success rate, got %.2f", s.SuccessRatePct)
	}
}

// TestComputeJobStats_NullableRows verifies that in-progress runs (NULL is_success,
// NULL duration) still count in TotalRuns but not in SuccessCount or timing.
func TestComputeJobStats_NullableRows(t *testing.T) {
	rows := []runsStatRow{
		// confirmed success with duration
		{JobDefinitionID: 3, IsSuccess: true, DurationSec: 120, CreatedAt: time.Now()},
		// in-progress: IsSuccess=false (NULL mapped to false), DurationSec=0 (NULL mapped to 0)
		{JobDefinitionID: 3, IsSuccess: false, DurationSec: 0, CreatedAt: time.Now()},
		// confirmed failure with duration
		{JobDefinitionID: 3, IsSuccess: false, DurationSec: 60, CreatedAt: time.Now()},
	}
	s := computeJobStats(3, rows)
	// All 3 rows must count in TotalRuns
	if s.TotalRuns != 3 {
		t.Errorf("expected TotalRuns=3 (including in-progress), got %d", s.TotalRuns)
	}
	// Only the confirmed success counts
	if s.SuccessCount != 1 {
		t.Errorf("expected SuccessCount=1, got %d", s.SuccessCount)
	}
	// SuccessRatePct is over total (3 rows), not just rows with non-null status
	wantRate := math.Round(1.0/3.0*10000) / 100
	if math.Abs(s.SuccessRatePct-wantRate) > 0.01 {
		t.Errorf("expected SuccessRatePct=%.2f, got %.2f", wantRate, s.SuccessRatePct)
	}
	// Timing avg/p95 computed only over the 2 rows with DurationSec > 0
	wantAvg := (120.0 + 60.0) / 2.0
	if math.Abs(s.AvgDurationSec-wantAvg) > 0.01 {
		t.Errorf("expected AvgDurationSec=%.2f (excluding zero-duration row), got %.2f", wantAvg, s.AvgDurationSec)
	}
}

func TestParseDurationToSeconds(t *testing.T) {
	cases := []struct {
		input string
		want  float64
	}{
		{"", 0},
		{"60", 60},
		{"3600.5", 3600.5},
		{"01:00:00", 3600},
		{"00:01:30", 90},
		{"01:30:00", 5400},
		{"not-a-number", 0},
	}
	for _, tc := range cases {
		got := parseDurationToSeconds(tc.input)
		if math.Abs(got-tc.want) > 0.001 {
			t.Errorf("parseDurationToSeconds(%q) = %.3f, want %.3f", tc.input, got, tc.want)
		}
	}
}
