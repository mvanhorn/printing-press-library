// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for the bests feature and workout parsing.

package cli

import (
	"encoding/json"
	"testing"
)

func TestParseWorkoutObj(t *testing.T) {
	// Uses the REAL live-API field names: power_avg, power_bike_np_last,
	// power_bike_tss_last, and work_accum in joules.
	raw := `{"id":1234567,"name":"Morning Ride","starts":"2026-01-02T07:00:00Z","minutes":90,
		"workout_summary":{"distance_accum":"40234.5","power_avg":"212","power_bike_np_last":"225",
		"power_bike_tss_last":"88.5","work_accum":"1450000","ascent_accum":"512",
		"duration_active_accum":"5400","heart_rate_avg":"148"}}`
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		t.Fatal(err)
	}
	w := parseWorkoutObj(obj)
	if w.ID != "1234567" {
		t.Errorf("ID=%q want 1234567 (no scientific notation)", w.ID)
	}
	if !w.HasStarts {
		t.Error("HasStarts false")
	}
	if !w.HasPower || w.AvgPowerW != 212 {
		t.Errorf("power=%v want 212 (from power_avg)", w.AvgPowerW)
	}
	if !w.HasNP || w.NP != 225 {
		t.Errorf("np=%v want 225 (from power_bike_np_last)", w.NP)
	}
	if !w.HasTSS || w.TSS != 88.5 {
		t.Errorf("tss=%v want 88.5 (from power_bike_tss_last)", w.TSS)
	}
	if !w.HasWork || w.WorkKJ != 1450 {
		t.Errorf("workKJ=%v want 1450 (work_accum joules / 1000)", w.WorkKJ)
	}
	if !w.HasDistance || w.DistanceM != 40234.5 {
		t.Errorf("distance=%v", w.DistanceM)
	}
	if w.DurationHours() != 1.5 {
		t.Errorf("durationHours=%v want 1.5", w.DurationHours())
	}
	// Spec-name fallback: power_bike_avg still works when power_avg is absent.
	wf := parseWorkoutObj(map[string]any{"id": "5", "workout_summary": map[string]any{"power_bike_avg": "180"}})
	if !wf.HasPower || wf.AvgPowerW != 180 {
		t.Errorf("fallback power=%v want 180 (from power_bike_avg)", wf.AvgPowerW)
	}
	// Missing summary: no phantom metrics.
	w2 := parseWorkoutObj(map[string]any{"id": "9", "name": "x"})
	if w2.HasSummary {
		t.Error("expected HasSummary false")
	}
	if _, ok := w2.metricValue("power"); ok {
		t.Error("expected no power metric on summary-less workout")
	}
}

func TestComputeBests(t *testing.T) {
	ws := []parsedWorkout{
		mkWorkout("2026-01-01", 3600, 200, 500, 30000, 300),
		mkWorkout("2026-01-02", 7200, 180, 900, 60000, 800), // longest, most work/dist/ascent
		mkWorkout("2026-01-03", 1800, 320, 200, 10000, 100), // most power
	}
	byMetric := map[string]bestRecord{}
	for _, r := range computeBests(ws, "") {
		byMetric[r.Metric] = r
	}
	if byMetric["power"].Value != 320 {
		t.Errorf("power best=%v want 320", byMetric["power"].Value)
	}
	if byMetric["distance"].Unit != "km" || byMetric["distance"].Value != 60 {
		t.Errorf("distance best=%v %s want 60 km", byMetric["distance"].Value, byMetric["distance"].Unit)
	}
	if byMetric["ascent"].Value != 800 {
		t.Errorf("ascent best=%v want 800", byMetric["ascent"].Value)
	}
	if byMetric["work"].Value != 900 {
		t.Errorf("work best=%v want 900", byMetric["work"].Value)
	}
	if byMetric["duration"].Value != 2 {
		t.Errorf("duration best=%v want 2", byMetric["duration"].Value)
	}
	if d := byMetric["power"].Date; d != "2026-01-03" {
		t.Errorf("power record date=%q want 2026-01-03", d)
	}
	// --metric filter
	only := computeBests(ws, "power")
	if len(only) != 1 || only[0].Metric != "power" {
		t.Errorf("metric filter failed: %+v", only)
	}
	// no usable metrics -> empty (not phantom zero records)
	if none := computeBests([]parsedWorkout{{HasStarts: false}}, ""); len(none) != 0 {
		t.Errorf("expected no records, got %d", len(none))
	}
}
