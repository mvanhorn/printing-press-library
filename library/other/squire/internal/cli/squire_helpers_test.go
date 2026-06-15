// Copyright 2026 Dev Basu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for transcendence pure-logic helpers (Phase 3).

package cli

import "testing"

func TestServiceMatches(t *testing.T) {
	cats := []any{map[string]any{"name": "Haircut"}}
	cases := []struct {
		name string
		svc  string
		cats []any
		term string
		want bool
	}{
		{"name substring", "Premium Haircut", nil, "haircut", true},
		{"case insensitive", "HAIRCUT", nil, "Haircut", true},
		{"whitespace trimmed", "  Haircut  ", nil, "haircut", true},
		{"category match", "The Works", cats, "haircut", true},
		{"non-match", "Beard Trim", nil, "haircut", false},
		{"empty term matches all", "anything", nil, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := serviceMatches(tc.svc, tc.cats, tc.term); got != tc.want {
				t.Fatalf("serviceMatches(%q,%v,%q)=%v want %v", tc.svc, tc.cats, tc.term, got, tc.want)
			}
		})
	}
}

func TestRosterScore(t *testing.T) {
	if s := rosterScore(5, 0); s != 0 {
		t.Fatalf("count 0 should score 0, got %v", s)
	}
	// Monotonic in count for a fixed rating.
	if rosterScore(5, 100) <= rosterScore(5, 10) {
		t.Fatalf("score should increase with review count")
	}
	// Monotonic in rating for a fixed count.
	if rosterScore(5, 50) <= rosterScore(4, 50) {
		t.Fatalf("score should increase with rating")
	}
	// Negative count is clamped, not panicking.
	if rosterScore(5, -3) != 0 {
		t.Fatalf("negative count should clamp to 0 score")
	}
}

func TestDiffServicePrices(t *testing.T) {
	old := map[string]int{"Haircut": 4500, "Beard": 2000, "Gone": 1000}
	cur := map[string]int{"Haircut": 5000, "Beard": 2000, "New": 3000}
	changes, added, removed := diffServicePrices(old, cur)
	if len(changes) != 1 || changes[0].Service != "Haircut" || changes[0].OldCents != 4500 || changes[0].NewCents != 5000 {
		t.Fatalf("expected one Haircut price change, got %+v", changes)
	}
	if len(added) != 1 || added[0] != "New" {
		t.Fatalf("expected 'New' added, got %v", added)
	}
	if len(removed) != 1 || removed[0] != "Gone" {
		t.Fatalf("expected 'Gone' removed, got %v", removed)
	}

	// No-change case yields empty (non-nil) slices.
	c2, a2, r2 := diffServicePrices(map[string]int{"X": 1}, map[string]int{"X": 1})
	if len(c2) != 0 || len(a2) != 0 || len(r2) != 0 {
		t.Fatalf("identical maps should produce no diffs")
	}
}
