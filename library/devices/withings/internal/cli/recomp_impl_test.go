// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored synthetic-data test for `recomp`.

package cli

import (
	"testing"
	"time"
)

func TestComputeRecomp_Recomposing(t *testing.T) {
	s, _ := newTestStore(t)

	// Three groups inside the 90d window, date-ascending. Weight ~steady,
	// fat mass falling (20 -> 19 -> 18 kg), lean mass steady (60 kg).
	// Values are raw with unit -3 (i.e. grams expressed as value*10^-3 kg).
	upsertJSON(t, s, "measure", "g1", measureGrp(1, daysAgoEpoch(60),
		measureValue{Value: 80000, Type: 1, Unit: -3},  // weight 80.0 kg
		measureValue{Value: 20000, Type: 8, Unit: -3},  // fat mass 20.0 kg
		measureValue{Value: 60000, Type: 5, Unit: -3})) // lean 60.0 kg
	upsertJSON(t, s, "measure", "g2", measureGrp(2, daysAgoEpoch(30),
		measureValue{Value: 79000, Type: 1, Unit: -3},  // weight 79.0
		measureValue{Value: 19000, Type: 8, Unit: -3},  // fat mass 19.0
		measureValue{Value: 60000, Type: 5, Unit: -3})) // lean 60.0
	upsertJSON(t, s, "measure", "g3", measureGrp(3, daysAgoEpoch(2),
		measureValue{Value: 78000, Type: 1, Unit: -3},  // weight 78.0
		measureValue{Value: 18000, Type: 8, Unit: -3},  // fat mass 18.0
		measureValue{Value: 60100, Type: 5, Unit: -3})) // lean 60.1 (held/up)

	groups, err := loadMeasureGroups(s, time.Now().Add(-90*24*time.Hour))
	if err != nil {
		t.Fatalf("loadMeasureGroups: %v", err)
	}
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups in window, got %d", len(groups))
	}

	res := computeRecomp(groups, "90d")

	if res.Verdict != "recomposing" {
		t.Errorf("verdict = %q, want %q", res.Verdict, "recomposing")
	}
	if res.Samples != 3 {
		t.Errorf("samples = %d, want 3", res.Samples)
	}
	if res.WeightStart != 80.0 {
		t.Errorf("weight_start = %v, want 80.0", res.WeightStart)
	}
	if res.WeightEnd != 78.0 {
		t.Errorf("weight_end = %v, want 78.0", res.WeightEnd)
	}
	if res.WeightChange != -2.0 {
		t.Errorf("weight_change = %v, want -2.0", res.WeightChange)
	}
	// Fat mass fell by 2.0 kg.
	if res.FatMassChange != -2.0 {
		t.Errorf("fat_mass_change = %v, want -2.0", res.FatMassChange)
	}
	// Lean mass rose slightly (held).
	if res.LeanMassChange < 0 {
		t.Errorf("lean_mass_change = %v, want >= 0", res.LeanMassChange)
	}
	// Rolling average over <=7 samples here is the mean of all three weights.
	wantRoll := roundN((80.0+79.0+78.0)/3.0, 3)
	if res.WeightRollAvg != wantRoll {
		t.Errorf("weight_rolling_avg = %v, want %v", res.WeightRollAvg, wantRoll)
	}
}

func TestComputeRecomp_InsufficientData(t *testing.T) {
	s, _ := newTestStore(t)
	// A single group cannot establish a trend.
	upsertJSON(t, s, "measure", "only", measureGrp(1, daysAgoEpoch(5),
		measureValue{Value: 80000, Type: 1, Unit: -3}))
	groups, err := loadMeasureGroups(s, time.Now().Add(-90*24*time.Hour))
	if err != nil {
		t.Fatalf("loadMeasureGroups: %v", err)
	}
	res := computeRecomp(groups, "90d")
	if res.Verdict != "insufficient data" {
		t.Errorf("verdict = %q, want %q", res.Verdict, "insufficient data")
	}
}
