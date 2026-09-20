// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
//
// These two tests close mutation-survivor gaps. EVERY string and number below
// was read back out of running code (buildSourcesCatalogue and
// sourcesFetchDriftMessage against the committed catalogue), not copied from a
// comment or a document.

package cli

import (
	"strings"
	"testing"
)

// TestSourcesGen404NotesDistinguishTheTerminalYear pins the one 404 year whose
// note carries the end-of-series finding.
//
// sourcesGenYears404 holds four probed-and-absent fiscal years. Exactly one of
// them, FY2024-25, is the year immediately after the last published workbook,
// so its note records the four naming variants that were tried and the fact
// that the Detail-of-Generation series ends at FY2023-24. The other three get
// the bare "probed and not published." — 2015-16 and 2016-17 sit BEFORE the
// series starts and 2025-26 is simply beyond it, and none of them is evidence
// about where the series terminates.
//
// MUTATION-VERIFIED: flipping `fy == "2024-25"` to `!=` moves the end-of-series
// note onto the three years that cannot support it and strips it from the one
// year that can, which is how a pre-series gap turns into a claim that the
// series ended in 2015.
func TestSourcesGen404NotesDistinguishTheTerminalYear(t *testing.T) {
	notes := map[string]string{}
	states := map[string]string{}
	for _, d := range buildSourcesCatalogue() {
		notes[d.ID] = d.Note
		states[d.ID] = d.State
	}

	const bare = "probed and not published."

	// The terminal year carries the finding.
	terminal := notes["gen-2024-25"]
	if terminal == "" {
		t.Fatal("gen-2024-25 is not in the catalogue")
	}
	if !strings.Contains(terminal, "The Detail-of-Generation series definitively ends at FY2023-24.") {
		t.Errorf("gen-2024-25 note lost the end-of-series finding: %q", terminal)
	}
	if !strings.Contains(terminal, "four naming variants tested") {
		t.Errorf("gen-2024-25 note lost the variants that were probed: %q", terminal)
	}
	if terminal == bare {
		t.Error("gen-2024-25 got the bare note; the year after the last workbook is the one year " +
			"whose 404 is evidence about where the series stops")
	}

	// No other absent year may claim it.
	for _, id := range []string{"gen-2015-16", "gen-2016-17", "gen-2025-26"} {
		got, ok := notes[id]
		if !ok {
			t.Fatalf("%s is not in the catalogue", id)
		}
		if got != bare {
			t.Errorf("%s note = %q, want the bare %q: only FY2024-25's 404 is evidence about the "+
				"end of the series", id, got, bare)
		}
		if strings.Contains(got, "definitively ends") {
			t.Errorf("%s claims to end the series; it is not the year after the last workbook", id)
		}
	}

	// All four are absent, not reachable-with-nothing-in-them.
	for _, id := range []string{"gen-2015-16", "gen-2016-17", "gen-2024-25", "gen-2025-26"} {
		if states[id] != "unavailable" {
			t.Errorf("%s state = %q, want unavailable", id, states[id])
		}
	}
}

// TestSourcesFetchDriftMessageNamesTheBaseline drives sourcesFetchDriftMessage
// directly over the two shapes the bytes leg can take.
//
// When a baseline exists the --strict message must quote it, so a human reading
// the line can see both numbers. When NO baseline was ever recorded the line
// must say "unmeasured": an absent baseline is not a baseline of zero, and
// printing "a baseline of 0" would read as a measured empty document.
//
// MUTATION-VERIFIED: flipping `res.Baseline != nil` to `== nil` reports a real
// 1,022,593-byte baseline as "unmeasured", and on the nil-baseline row it
// dereferences the nil pointer and panics.
func TestSourcesFetchDriftMessageNamesTheBaseline(t *testing.T) {
	// Measured against the real baseline the catalogue records for this row.
	withBaseline := sourcesFetchResult{
		ID:       "per-fy2023-24",
		Baseline: intp(1022593),
		Measured: sourcesMeasurement{
			Bytes:   1024,
			Matches: &matchSet{Bytes: boolp(false)},
		},
	}
	got := sourcesFetchDriftMessage(withBaseline)
	if !strings.Contains(got, "bytes 1,024 against a baseline of 1,022,593") {
		t.Errorf("drift message did not quote the baseline it disagreed with: %q", got)
	}
	if strings.Contains(got, "unmeasured") {
		t.Errorf("a recorded 1,022,593-byte baseline was reported as unmeasured: %q", got)
	}
	if !strings.HasPrefix(got, "per-fy2023-24 disagrees with its baseline on ") {
		t.Errorf("drift message = %q, want it to name the row first", got)
	}

	// Same disagreement, but nothing was ever recorded to disagree WITH.
	noBaseline := withBaseline
	noBaseline.Baseline = nil
	got = sourcesFetchDriftMessage(noBaseline)
	if !strings.Contains(got, "against a baseline of unmeasured") {
		t.Errorf("an absent baseline was not reported as unmeasured: %q", got)
	}
	if strings.Contains(got, "baseline of 0") {
		t.Errorf("an absent baseline printed as a measured zero: %q", got)
	}

	// A matching bytes leg produces no bytes clause at all.
	matching := withBaseline
	matching.Measured.Matches = &matchSet{Bytes: boolp(true)}
	if got := sourcesFetchDriftMessage(matching); got != "" {
		t.Errorf("drift message = %q, want empty when every computed leg agrees", got)
	}
}
