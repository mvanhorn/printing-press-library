// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"sort"
	"testing"
	"time"
)

// TestSnapshotTimeFormatSeparatesSameSecondCaptures pins the fix for a
// data-loss defect: psx_snapshots is keyed by (taken_at, kind, symbol) and
// written with INSERT OR REPLACE, so two captures of the same kind that share a
// taken_at value silently overwrite each other and destroy retained history —
// the exact history diff, drift, unusual and rotation read.
func TestSnapshotTimeFormatSeparatesSameSecondCaptures(t *testing.T) {
	base := time.Date(2026, 8, 19, 19, 37, 42, 0, time.UTC)
	first := base.Add(12 * time.Millisecond).Format(snapshotTimeFormat)
	second := base.Add(840 * time.Millisecond).Format(snapshotTimeFormat)

	if first == second {
		t.Fatalf("two captures in the same second produced identical taken_at %q; "+
			"INSERT OR REPLACE would silently drop the earlier capture", first)
	}
	// Second-resolution formatting is what made them collide.
	if a, b := base.Add(12*time.Millisecond).Format(time.RFC3339),
		base.Add(840*time.Millisecond).Format(time.RFC3339); a != b {
		t.Fatalf("precondition changed: RFC3339 no longer collides (%q vs %q)", a, b)
	}
}

// TestSnapshotTimeFormatIsLexicographicallyOrdered pins the second property the
// format must hold: taken_at is compared as TEXT in SQL and in Go string
// comparisons against cutoffs, so lexicographic order must equal chronological
// order. time.RFC3339Nano fails this because it strips trailing zeros.
func TestSnapshotTimeFormatIsLexicographicallyOrdered(t *testing.T) {
	base := time.Date(2026, 8, 19, 19, 37, 42, 0, time.UTC)
	times := []time.Time{
		base.Add(2 * time.Nanosecond),
		base.Add(20 * time.Millisecond),
		base.Add(300 * time.Millisecond),
		base.Add(1 * time.Second),
		base.Add(90 * time.Second),
	}
	formatted := make([]string, 0, len(times))
	for _, tm := range times {
		formatted = append(formatted, tm.Format(snapshotTimeFormat))
	}
	sorted := append([]string(nil), formatted...)
	sort.Strings(sorted)
	for i := range formatted {
		if formatted[i] != sorted[i] {
			t.Fatalf("string sort disagrees with chronological order at %d:\n  chronological: %v\n  string-sorted: %v",
				i, formatted, sorted)
		}
	}
	// All values must be the same width, or ordering is not guaranteed.
	for _, f := range formatted {
		if len(f) != len(formatted[0]) {
			t.Errorf("format is not fixed width: %q (%d) vs %q (%d)", f, len(f), formatted[0], len(formatted[0]))
		}
	}
	// Guard against a regression to RFC3339Nano, which strips trailing zeros.
	if got := base.Add(20 * time.Millisecond).Format(time.RFC3339Nano); len(got) == len(formatted[0]) {
		t.Errorf("precondition changed: RFC3339Nano is now fixed width (%q)", got)
	}
}

// TestSnapshotCutoffsUseTheStoredFormat pins that range cutoffs are stamped
// with the same layout as stored rows. A cutoff in a different layout compares
// incorrectly against stored values and silently shifts window boundaries.
func TestSnapshotCutoffsUseTheStoredFormat(t *testing.T) {
	at := time.Date(2026, 8, 19, 19, 37, 42, 500000000, time.UTC)
	stored := at.Format(snapshotTimeFormat)
	cutoff := at.Add(-24 * time.Hour).Format(snapshotTimeFormat)
	if len(stored) != len(cutoff) {
		t.Fatalf("cutoff width %d != stored width %d; string comparison is unsafe", len(cutoff), len(stored))
	}
	if !(cutoff < stored) {
		t.Fatalf("cutoff %q should sort before stored %q", cutoff, stored)
	}
}
