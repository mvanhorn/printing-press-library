// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
)

func TestPctChange(t *testing.T) {
	tests := []struct {
		name       string
		prev, curr float64
		want       float64
	}{
		{"simple increase", 100, 150, 50},
		{"decrease", 200, 150, -25},
		{"new row from zero", 0, 10, 100},
		{"zero to zero", 0, 0, 0},
		{"negative baseline uses abs denominator", -100, -50, 50},
		{"no change", 80, 80, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pctChange(tt.prev, tt.curr); got != tt.want {
				t.Fatalf("pctChange(%v,%v) = %v, want %v", tt.prev, tt.curr, got, tt.want)
			}
		})
	}
}

func TestDiffReportRows(t *testing.T) {
	prev := []reportRow{
		{Key: "a", Value: 100},
		{Key: "b", Value: 50},
		{Key: "dropped", Value: 30},
	}
	curr := []reportRow{
		{Key: "a", Value: 130}, // +30%
		{Key: "b", Value: 55},  // +10%
		{Key: "new", Value: 5}, // +100%
	}

	changes, flagged := diffReportRows(prev, curr, 25)

	// One change per key in either snapshot: a, b, new, dropped.
	if len(changes) != 4 {
		t.Fatalf("len(changes) = %d, want 4 (got %#v)", len(changes), changes)
	}

	byKey := map[string]reportChange{}
	for _, c := range changes {
		byKey[c.Key] = c
	}
	if c := byKey["a"]; c.Prev != 100 || c.Curr != 130 || c.Delta != 30 || c.Pct != 30 {
		t.Fatalf("change[a] = %#v, want prev100 curr130 delta30 pct30", c)
	}
	if c := byKey["new"]; c.Prev != 0 || c.Curr != 5 || c.Pct != 100 {
		t.Fatalf("change[new] = %#v, want prev0 curr5 pct100", c)
	}
	if c := byKey["dropped"]; c.Prev != 30 || c.Curr != 0 {
		t.Fatalf("change[dropped] = %#v, want prev30 curr0", c)
	}

	// Flagged at threshold 25: a (30), new (100), dropped (-100). NOT b (10).
	flaggedKeys := map[string]bool{}
	for _, c := range flagged {
		flaggedKeys[c.Key] = true
	}
	if !flaggedKeys["a"] || !flaggedKeys["new"] || !flaggedKeys["dropped"] {
		t.Fatalf("expected a,new,dropped flagged; got %v", flaggedKeys)
	}
	if flaggedKeys["b"] {
		t.Fatalf("b (10%%) should not be flagged at threshold 25; got %v", flaggedKeys)
	}
}

func TestDiffReportRowsZeroThresholdFlagsNothing(t *testing.T) {
	// threshold 0 disables flagging (every row would otherwise qualify).
	prev := []reportRow{{Key: "a", Value: 1}}
	curr := []reportRow{{Key: "a", Value: 2}}
	_, flagged := diffReportRows(prev, curr, 0)
	if len(flagged) != 0 {
		t.Fatalf("threshold 0 should flag nothing, got %#v", flagged)
	}
}
