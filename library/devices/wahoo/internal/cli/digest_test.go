// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for the digest feature.

package cli

import (
	"testing"
	"time"
)

func TestComputeDigest(t *testing.T) {
	now, _ := time.Parse("2006-01-02", "2026-01-10")
	ws := []parsedWorkout{
		mkWorkout("2026-01-09", 3600, 200, 500, 30000, 300), // in window
		mkWorkout("2026-01-08", 3600, 0, 0, 20000, 0),       // in window, no power
		mkWorkout("2025-12-01", 3600, 250, 800, 50000, 600), // out of window
		mkWorkout("", 3600, 150, 0, 25000, 0),               // dateless — must NOT count in a window
	}
	ws[1].HasSummary = false // simulate an upload that never got a summary

	v := computeDigest(ws, 250, 7, now)
	if v.Workouts != 2 {
		t.Fatalf("workouts=%d want 2 (windowed)", v.Workouts)
	}
	if v.RidesMissingSummary != 1 {
		t.Errorf("missing-summary=%d want 1", v.RidesMissingSummary)
	}
	if v.TotalDistanceKm != 50 {
		t.Errorf("distance=%v want 50 (30+20)", v.TotalDistanceKm)
	}
	// Average power must exclude the no-power ride rather than averaging in a zero.
	if v.AvgPowerW != 200 {
		t.Errorf("avg power=%v want 200 (excludes no-power ride)", v.AvgPowerW)
	}
	if v.TotalAscentM != 300 {
		t.Errorf("ascent=%v want 300", v.TotalAscentM)
	}
	// An empty window (all rides older than the window) yields zeroes, not a crash.
	future, _ := time.Parse("2006-01-02", "2026-02-01")
	empty := computeDigest(ws, 250, 1, future)
	if empty.Workouts != 0 {
		t.Errorf("empty window workouts=%d want 0", empty.Workouts)
	}
}
