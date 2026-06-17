// Copyright 2026 slinsmaier and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"math"
	"testing"
)

func TestMeanStdDev(t *testing.T) {
	cases := []struct {
		name       string
		vals       []float64
		wantMean   float64
		wantStdDev float64
	}{
		{"empty", nil, 0, 0},
		{"single", []float64{42}, 42, 0},
		{"simple", []float64{2, 4, 4, 4, 5, 5, 7, 9}, 5, 2.13808993},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mean, stdDev := meanStdDev(tc.vals)
			if math.Abs(mean-tc.wantMean) > 1e-6 {
				t.Errorf("mean = %v, want %v", mean, tc.wantMean)
			}
			if math.Abs(stdDev-tc.wantStdDev) > 1e-4 {
				t.Errorf("stdDev = %v, want %v", stdDev, tc.wantStdDev)
			}
		})
	}
}

func TestResolveSinceDay(t *testing.T) {
	got, err := resolveSinceDay("2026-01-01", 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "2026-01-01" {
		t.Errorf("got %q, want 2026-01-01", got)
	}

	got, err = resolveSinceDay("not-a-duration", 30)
	if err == nil {
		t.Errorf("expected error for invalid --since, got day=%q", got)
	}

	got, err = resolveSinceDay("", 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := addDays(today(), -7); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAddDaysAndDaysBetween(t *testing.T) {
	if got := addDays("2026-06-01", 5); got != "2026-06-06" {
		t.Errorf("addDays = %q, want 2026-06-06", got)
	}
	if got := addDays("2026-06-01", -1); got != "2026-05-31" {
		t.Errorf("addDays = %q, want 2026-05-31", got)
	}
	if got := daysBetween("2026-06-01", "2026-06-08"); got != 7 {
		t.Errorf("daysBetween = %d, want 7", got)
	}
}

func TestResolveMetric(t *testing.T) {
	if _, err := resolveMetric("readiness"); err != nil {
		t.Errorf("unexpected error for known metric: %v", err)
	}
	if _, err := resolveMetric("not-a-metric"); err == nil {
		t.Error("expected error for unknown metric")
	}
}

func TestSortedDays(t *testing.T) {
	m := map[string]float64{"2026-06-03": 1, "2026-06-01": 2, "2026-06-02": 3}
	got := sortedDays(m)
	want := []string{"2026-06-01", "2026-06-02", "2026-06-03"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
			break
		}
	}
}
