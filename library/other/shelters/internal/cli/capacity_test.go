// Copyright 2026 Abe Diaz (@abe238) and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"math"
	"strings"
	"testing"
)

func findCapRow(rows []capacityRow, id int) *capacityRow {
	for i := range rows {
		if rows[i].ShelterID == id {
			return &rows[i]
		}
	}
	return nil
}

func TestBuildCapacityCounts(t *testing.T) {
	shelters := parseFixture(t, syntheticFixture)
	d := buildCapacity(shelters)

	if d.ComputableCount != 10 {
		t.Errorf("computable_count = %d, want 10", d.ComputableCount)
	}
	if d.UnknownCount != 2 {
		t.Errorf("unknown_count = %d, want 2", d.UnknownCount)
	}
	if d.AtCapacityCount != 2 {
		t.Errorf("at_capacity_count = %d, want 2", d.AtCapacityCount)
	}
	if d.ReportedFull != 2 {
		t.Errorf("reported_full_count = %d, want 2", d.ReportedFull)
	}
}

// TestCapacityNeverInventsDenominator: a shelter with null population must have
// nil utilization, never a fabricated number.
func TestCapacityNeverInventsDenominator(t *testing.T) {
	shelters := parseFixture(t, syntheticFixture)
	d := buildCapacity(shelters)
	// id 500110 (Orange) has null population.
	r := findCapRow(d.Shelters, 500110)
	if r == nil {
		t.Fatal("missing shelter 500110")
	}
	if r.UtilizationPct != nil {
		t.Errorf("shelter with null population has utilization %v, want nil", *r.UtilizationPct)
	}
}

// TestCapacityPostImpactFallback: a shelter with null evacuation_capacity but a
// post_impact_capacity must compute against post_impact and label it.
func TestCapacityPostImpactFallback(t *testing.T) {
	shelters := parseFixture(t, syntheticFixture)
	d := buildCapacity(shelters)
	r := findCapRow(d.Shelters, 500107) // Corpus Christi: evac null, post 180, pop 90
	if r == nil {
		t.Fatal("missing shelter 500107")
	}
	if r.UtilizationPct == nil {
		t.Fatal("expected computable utilization for 500107")
	}
	if math.Abs(*r.UtilizationPct-50.0) > 0.1 {
		t.Errorf("utilization = %.1f, want 50.0 (90/180 post-impact)", *r.UtilizationPct)
	}
	if !strings.Contains(r.CapacityBasis, "post_impact") {
		t.Errorf("capacity_basis = %q, want it to name post_impact", r.CapacityBasis)
	}
}

// TestCapacityFullAndOver: FULL status and >100% both flag at-capacity.
func TestCapacityFullAndOver(t *testing.T) {
	shelters := parseFixture(t, syntheticFixture)
	d := buildCapacity(shelters)

	lc := findCapRow(d.Shelters, 500105) // Lake Charles: 380/350 = 108.6%, status FULL
	if lc == nil || !lc.AtCapacity || !lc.ReportedFull {
		t.Fatalf("Lake Charles should be at-capacity and reported-full: %+v", lc)
	}
	if lc.UtilizationPct == nil || *lc.UtilizationPct < 100 {
		t.Errorf("Lake Charles utilization = %v, want >100", lc.UtilizationPct)
	}

	gv := findCapRow(d.Shelters, 500103) // Galveston: 200/200 = 100%, status FULL
	if gv == nil || !gv.AtCapacity {
		t.Fatalf("Galveston should be at-capacity: %+v", gv)
	}
}
