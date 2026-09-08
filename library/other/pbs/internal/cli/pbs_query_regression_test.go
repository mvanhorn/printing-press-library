// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"math"
	"testing"
)

// TestSpearmanRanksOverIntersection is the regression test for a defect found in
// pre-PR review: ranks were computed per release over only that release's present
// cities, then differenced. When the two city sets differ, that compares ranks
// drawn from 1..17 against ranks drawn from 1..3 and puts rho far outside
// [-1, 1] — a measured -146, which is not a correlation at all.
func TestSpearmanRanksOverIntersection(t *testing.T) {
	// First release has 17 cities; second has only the three that also appear
	// first, and in the same order. Correlation over the intersection is +1.
	a := map[string]float64{}
	for i, c := range []string{
		"c01", "c02", "c03", "c04", "c05", "c06", "c07", "c08", "c09",
		"c10", "c11", "c12", "c13", "c14", "c15", "c16", "c17",
	} {
		a[c] = float64(100 + i)
	}
	b := map[string]float64{"c15": 1, "c16": 2, "c17": 3}

	rho, n := spearmanFromValues(a, b)
	if n != 3 {
		t.Fatalf("intersection size = %d, want 3", n)
	}
	if rho < -1.000001 || rho > 1.000001 {
		t.Fatalf("rho = %v, outside [-1,1] — ranks are not being recomputed over the intersection", rho)
	}
	if math.Abs(rho-1) > 1e-9 {
		t.Errorf("rho = %v, want +1 for identical ordering", rho)
	}
}

func TestSpearmanPerfectInversion(t *testing.T) {
	a := map[string]float64{"x": 1, "y": 2, "z": 3}
	b := map[string]float64{"x": 3, "y": 2, "z": 1}
	rho, n := spearmanFromValues(a, b)
	if n != 3 {
		t.Fatalf("n = %d, want 3", n)
	}
	if math.Abs(rho+1) > 1e-9 {
		t.Errorf("rho = %v, want -1 for a reversed ordering", rho)
	}
}

// TestSpearmanTiesShareRank pins that equal prices do not manufacture an
// ordering. Three tied values must all take the midpoint rank.
func TestSpearmanTiesShareRank(t *testing.T) {
	vals := map[string]float64{"a": 5, "b": 5, "c": 5, "d": 9}
	r := tiedRanks(vals, []string{"a", "b", "c", "d"})
	for _, k := range []string{"a", "b", "c"} {
		if math.Abs(r[k]-2) > 1e-9 {
			t.Errorf("rank(%s) = %v, want 2 (midpoint of ranks 1..3)", k, r[k])
		}
	}
	if math.Abs(r["d"]-4) > 1e-9 {
		t.Errorf("rank(d) = %v, want 4", r["d"])
	}
}

func TestSpearmanRefusesTinyIntersection(t *testing.T) {
	a := map[string]float64{"x": 1, "y": 2}
	b := map[string]float64{"x": 1, "y": 2}
	if rho, n := spearmanFromValues(a, b); n != 2 || rho != 0 {
		t.Errorf("got rho=%v n=%d, want rho=0 n=2 for an intersection below three", rho, n)
	}
}
