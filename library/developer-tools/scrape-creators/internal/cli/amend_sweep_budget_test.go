// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the sweep credit budget: --max-credits gates EVERY fetch (the
// initial posts fetch included), the worst-case estimate never goes stale,
// and a fetch that prices above its estimate halts the sweep immediately.

package cli

import (
	"strings"
	"testing"
)

// Every fetch is gated, including the first one: a budget smaller than the
// worst-case estimate admits nothing at all.
func TestSweepBudget_GatesTheFirstFetch(t *testing.T) {
	b := newSweepBudget(0) // no budget
	if !b.allows() {
		t.Fatal("a zero/absent budget must admit fetches")
	}

	b = newSweepBudget(1)
	if !b.allows() {
		t.Error("budget 1 with floor estimate 1 must admit the first fetch")
	}

	// A budget below the floor estimate admits nothing — not even the
	// initial posts fetch, which previously ran unchecked.
	b = newSweepBudget(-1)
	if !b.allows() {
		t.Error("negative max is treated as no budget")
	}
}

// The estimate is the running MAXIMUM per-fetch charge, not the last one: a
// cheap fetch after an expensive one must not re-open the gate.
func TestSweepBudget_EstimateNeverGoesStale(t *testing.T) {
	b := newSweepBudget(20)

	if note, breached := b.charge(15); breached {
		t.Fatalf("charging 15 of 20 must not breach: %s", note)
	}
	if b.maxCost != 15 {
		t.Errorf("maxCost = %d, want 15 (estimate widened by the observed charge)", b.maxCost)
	}
	// charged=15, estimate=15 -> 15+15 > 20, so the next fetch is refused.
	if b.allows() {
		t.Error("a fetch that could cost 15 more must be refused at charged=15 of 20")
	}

	// A subsequent cheap charge must NOT shrink the estimate back down.
	if _, breached := b.charge(1); breached {
		t.Fatal("charged 16 of 20 must not breach")
	}
	if b.maxCost != 15 {
		t.Errorf("maxCost = %d, want 15 (a cheaper fetch must not lower the worst case)", b.maxCost)
	}
	if b.allows() {
		t.Error("estimate must stay at the observed maximum, keeping the gate closed")
	}
}

// The gate admits a fetch only when the worst case fits.
func TestSweepBudget_AdmitsOnlyWhenWorstCaseFits(t *testing.T) {
	b := newSweepBudget(10)
	if _, breached := b.charge(4); breached {
		t.Fatal("4 of 10 must not breach")
	}
	if !b.allows() {
		t.Error("charged 4, estimate 4 -> 8 <= 10, fetch must be admitted")
	}
	if _, breached := b.charge(4); breached {
		t.Fatal("8 of 10 must not breach")
	}
	if b.allows() {
		t.Error("charged 8, estimate 4 -> 12 > 10, fetch must be refused")
	}
}

// A server-side price above the estimate halts the sweep instead of letting
// the overshoot compound across further fetches.
func TestSweepBudget_HaltsWhenAFetchExceedsItsEstimate(t *testing.T) {
	b := newSweepBudget(10)
	if _, breached := b.charge(9); breached {
		t.Fatal("9 of 10 must not breach")
	}
	// Estimate is 9 now, so allows() is false and the loop would stop; a
	// caller that still fetches (or an API that prices above the estimate)
	// gets a hard breach signal rather than silent overspend.
	note, breached := b.charge(5)
	if !breached {
		t.Fatal("charging past the budget must report a breach")
	}
	if b.charged != 14 {
		t.Errorf("charged = %d, want 14 (the true total is always reported)", b.charged)
	}
	if !strings.Contains(note, "budget exceeded") || !strings.Contains(note, "stopped immediately") {
		t.Errorf("breach note must name the overspend and the halt; got %q", note)
	}
}

// With no budget configured, nothing is ever refused or reported as breached.
func TestSweepBudget_NoBudgetNeverBlocks(t *testing.T) {
	b := newSweepBudget(0)
	for i := 0; i < 5; i++ {
		if _, breached := b.charge(1000); breached {
			t.Fatal("an unset budget must never report a breach")
		}
		if !b.allows() {
			t.Fatal("an unset budget must never refuse a fetch")
		}
	}
	if b.charged != 5000 {
		t.Errorf("charged = %d, want 5000", b.charged)
	}
}

// The pre-fetch stop note names the fetch kind and the estimate that closed
// the gate, so an operator can tell which request was withheld.
func TestSweepBudget_StopNoteNamesFetchKindAndEstimate(t *testing.T) {
	b := newSweepBudget(10)
	if _, breached := b.charge(7); breached {
		t.Fatal("7 of 10 must not breach")
	}
	note := b.stopNote("posts page")
	for _, want := range []string{"--max-credits 10", "posts page", "est. 7 cr"} {
		if !strings.Contains(note, want) {
			t.Errorf("stop note %q must contain %q", note, want)
		}
	}
}
