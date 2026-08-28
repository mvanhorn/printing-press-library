// Copyright 2026 Farouk Umar and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// The Rightbrain API renders the same field two ways — with a microsecond
// component and, whenever that component is zero, without. A mirror synced from
// a real project held 235 of the first shape against 67 of the second, so the
// pairing below is ordinary data rather than a contrived edge case.
const (
	tsWholeSecond = "2026-04-27T13:53:00Z"
	tsSubSecond   = "2026-04-27T13:53:00.997704Z" // same second, 0.997s later
)

// TestAPITimeAfterOrdersMixedPrecision is the regression for the ordering bug.
// Comparing these two as bytes inverts them: '.' (0x2E) sorts below 'Z' (0x5A),
// so the whole-second value reads as the later one when it is in fact earlier.
// Every consumer of this helper picks a "most recent" record, so an inversion
// silently selects the wrong one.
func TestAPITimeAfterOrdersMixedPrecision(t *testing.T) {
	t.Parallel()

	if !(tsWholeSecond > tsSubSecond) {
		t.Fatalf("premise no longer holds: %q should sort above %q as raw bytes; "+
			"this test guards against exactly that byte comparison", tsWholeSecond, tsSubSecond)
	}
	if apiTimeAfter(tsWholeSecond, tsSubSecond) {
		t.Errorf("apiTimeAfter(%q, %q) = true, want false: the sub-second value is later",
			tsWholeSecond, tsSubSecond)
	}
	if !apiTimeAfter(tsSubSecond, tsWholeSecond) {
		t.Errorf("apiTimeAfter(%q, %q) = false, want true", tsSubSecond, tsWholeSecond)
	}
}

func TestAPITimeAfterCases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{"later day", "2026-04-28T00:00:00Z", "2026-04-27T23:59:59.999999Z", true},
		{"earlier day", "2026-04-27T23:59:59.999999Z", "2026-04-28T00:00:00Z", false},
		{"identical", tsSubSecond, tsSubSecond, false},
		{"zone-less parses", "2026-04-27T13:53:01", "2026-04-27T13:53:00Z", true},
		{"parseable beats unparseable", tsWholeSecond, "not a timestamp", true},
		{"unparseable loses to parseable", "not a timestamp", tsWholeSecond, false},
		{"empty loses", "", tsWholeSecond, false},
		{"both unparseable falls back to bytes", "zzz", "aaa", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := apiTimeAfter(tc.a, tc.b); got != tc.want {
				t.Errorf("apiTimeAfter(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// TestPickGateBaselineUsesRealTimeOrder drives the ordering through the function
// that actually decides a release. Two completed runs land in the same second —
// the burst a CI pipeline produces — and the newer one must win, which a byte
// comparison gets backwards.
func TestPickGateBaselineUsesRealTimeOrder(t *testing.T) {
	t.Parallel()

	stored := []gateStoredRun{
		{
			id: "older-run", revisionID: "rev-a", status: "completed",
			passRate: 0.60, hasPassRate: true, completedAt: tsWholeSecond,
		},
		{
			id: "newer-run", revisionID: "rev-b", status: "completed",
			passRate: 0.95, hasPassRate: true, completedAt: tsSubSecond,
		},
	}

	best, ok := pickGateBaseline(stored, "rev-candidate", "")
	if !ok {
		t.Fatal("pickGateBaseline found no baseline, want the newer completed run")
	}
	if best.id != "newer-run" {
		t.Errorf("baseline = %q (pass rate %.2f), want \"newer-run\" (0.95): "+
			"the sub-second timestamp is the later one", best.id, best.passRate)
	}
}
