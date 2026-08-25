// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for the load feature and shared analysis helpers.

package cli

import (
	"math"
	"testing"
	"time"
)

func TestParseLooseFloat(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want float64
		ok   bool
	}{
		{"float", float64(12.5), 12.5, true},
		{"numeric string", "1.91", 1.91, true},
		{"int string", "250", 250, true},
		{"empty string", "", 0, false},
		{"whitespace", "   ", 0, false},
		{"nil", nil, 0, false},
		{"garbage", "abc", 0, false},
		{"negative", "-3.5", -3.5, true},
	}
	for _, c := range cases {
		got, ok := parseLooseFloat(c.in)
		if ok != c.ok || (ok && math.Abs(got-c.want) > 1e-9) {
			t.Errorf("%s: parseLooseFloat(%v)=(%v,%v) want (%v,%v)", c.name, c.in, got, ok, c.want, c.ok)
		}
	}
}

// mkWorkout builds a parsedWorkout for analysis tests. Shared across
// load/bests/digest tests in this package.
func mkWorkout(date string, durActiveS, power, work, dist, ascent float64) parsedWorkout {
	w := parsedWorkout{HasSummary: true}
	if date != "" {
		if tt, err := time.Parse("2006-01-02", date); err == nil {
			w.Starts = tt
			w.HasStarts = true
		}
	}
	if durActiveS > 0 {
		w.DurationActiveS = durActiveS
	}
	if power > 0 {
		w.AvgPowerW = power
		w.HasPower = true
	}
	if work > 0 {
		w.WorkKJ = work
		w.HasWork = true
	}
	if dist > 0 {
		w.DistanceM = dist
		w.HasDistance = true
	}
	if ascent > 0 {
		w.AscentM = ascent
		w.HasAscent = true
	}
	return w
}

func TestEstimateRideLoad(t *testing.T) {
	// Real recorded TSS (power_bike_tss_last) wins over everything else.
	if got := estimateRideLoad(parsedWorkout{DurationActiveS: 3600, AvgPowerW: 250, HasPower: true, TSS: 88.5, HasTSS: true}, 250); got != 88.5 {
		t.Errorf("TSS load = %v want 88.5 (real TSS preferred)", got)
	}
	// No TSS: 1h at FTP (IF=1) -> 100
	if got := estimateRideLoad(mkWorkout("2026-01-01", 3600, 250, 0, 0, 0), 250); math.Abs(got-100) > 0.01 {
		t.Errorf("power load = %v want 100", got)
	}
	// No TSS, half power (IF=0.5), 2h -> 2 * 0.25 * 100 = 50
	if got := estimateRideLoad(mkWorkout("2026-01-01", 7200, 125, 0, 0, 0), 250); math.Abs(got-50) > 0.01 {
		t.Errorf("half-power load = %v want 50", got)
	}
	// No TSS, normalized power preferred over average when present.
	if got := estimateRideLoad(parsedWorkout{DurationActiveS: 3600, AvgPowerW: 200, HasPower: true, NP: 250, HasNP: true}, 250); math.Abs(got-100) > 0.01 {
		t.Errorf("NP-based load = %v want 100 (NP 250 / FTP 250)", got)
	}
	// No TSS, no power, no FTP: duration-only proxy 1h -> 0.65^2*100 = 42.25
	if got := estimateRideLoad(mkWorkout("2026-01-01", 3600, 0, 0, 0, 0), 0); math.Abs(got-42.25) > 0.01 {
		t.Errorf("duration load = %v want 42.25", got)
	}
	// zero duration -> 0
	if got := estimateRideLoad(parsedWorkout{}, 250); got != 0 {
		t.Errorf("zero-duration load = %v want 0", got)
	}
}

func TestComputeLoadSeries(t *testing.T) {
	ws := []parsedWorkout{
		mkWorkout("2026-01-01", 3600, 250, 0, 0, 0), // load 100
		mkWorkout("2026-01-02", 3600, 250, 0, 0, 0), // load 100
	}
	now, _ := time.Parse("2006-01-02", "2026-01-05")
	series := computeLoadSeries(ws, 250, 0, now)
	if len(series) != 5 { // Jan 1..5 inclusive, including rest days
		t.Fatalf("series len = %d want 5", len(series))
	}
	for _, p := range series {
		if math.Abs(p.TSB-(p.CTL-p.ATL)) > 0.2 {
			t.Errorf("%s: TSB %v != CTL-ATL %v", p.Date, p.TSB, p.CTL-p.ATL)
		}
	}
	// ATL (7-day) decays on the rest days after the two rides.
	if series[4].ATL >= series[1].ATL {
		t.Errorf("ATL did not decay on rest days: day2=%v day5=%v", series[1].ATL, series[4].ATL)
	}
	// trailing-N truncation
	if short := computeLoadSeries(ws, 250, 2, now); len(short) != 2 {
		t.Errorf("truncated series len = %d want 2", len(short))
	}
	// no dated workouts -> nil
	if computeLoadSeries(nil, 250, 0, now) != nil {
		t.Error("expected nil series for no workouts")
	}
}

func TestFormLabel(t *testing.T) {
	if formLabel(20) == formLabel(-40) {
		t.Error("fresh and fatigued should differ")
	}
	if formLabel(0) != "neutral (gray zone)" {
		t.Errorf("tsb 0 = %q", formLabel(0))
	}
}
