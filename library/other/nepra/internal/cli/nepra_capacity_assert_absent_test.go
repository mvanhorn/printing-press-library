// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
//
// The completeness FAILURE SENTENCE is the only place the undetermined
// bucket's megawatts are ever quoted, and it is quoted from a sum that may
// not exist. EVERY NUMBER ASSERTED HERE WAS MEASURED by running this code
// over the committed FY2022-23 fixture.

package cli

import (
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// TestCapacityUndeterminedFailureQuotesMeasuredMWOnly pins BOTH halves of
// the megawatt clause in the no_unmodelled_status_sentinels failure.
//
// FY2022-23 is the year with the blind spot: two rows spell the sentinel
// "DELICENSE", nepraparse classifies neither, and the failure sentence names
// their installed capacity so a reader can see how much capacity is
// unclassified. That number is read from the undetermined bucket's sum,
// which is a POINTER and is absent for any grouping that carries no
// installed column — so the clause has to degrade to "no capacity number"
// rather than dereference it.
//
// MUTATION: change `found && b.Installed != nil` to `found || ...` in
// capacityAssertLive and the absent-sum case reads Float64 off a nil
// *capacityMW.
func TestCapacityUndeterminedFailureQuotesMeasuredMWOnly(t *testing.T) {
	w, body := capacityParse(t, "2022-23")
	surface := capacitySurface(t, "2022-23")

	// Control: the sum IS measured on this path, so the sentence quotes it.
	// 181.00 MW is measured by this build over this fixture.
	rep, err := capacityGroupLive(w, "status")
	if err != nil {
		t.Fatal(err)
	}
	live := capacityAssertLive(w, len(body), surface, rep)
	if len(live.Failed) != 1 {
		t.Fatalf("FY2022-23 must produce exactly one failure, got %v", live.Failed)
	}
	if !strings.Contains(live.Failed[0], "181.00 MW installed") {
		t.Errorf("the failure must quote the measured undetermined capacity 181.00 MW installed: %q", live.Failed[0])
	}
	if strings.Contains(live.Failed[0], "no capacity number") {
		t.Errorf("the sum WAS measured here, so the sentence must not claim there is no number: %q", live.Failed[0])
	}

	// The clause is derived from the PLANTS, not from the report's buckets,
	// so stripping a bucket's sum must NOT change it.
	//
	// This half of the test originally asserted the opposite: it set the
	// undetermined bucket's Installed to nil and required the sentence to
	// read "(no capacity number)". That pinned an implementation detail
	// rather than the semantic, and the detail was a BUG. The sentence is a
	// claim about the workbook's unclassifiable ROWS — "2 plant row(s)
	// publish a monthly block this build cannot classify (X)" — so X is a
	// property of those rows, not of how --by happened to bucket them.
	// Reading it from the bucket made the sentence grouping-dependent: under
	// --by technology the member is nested inside a parent's ByStatus and the
	// lookup missed, and under --by system no status member exists at all, so
	// both printed "no capacity number" over 181.00 MW of PUBLISHED capacity.
	// Asserting an absence over real data is the one thing this command
	// exists to refuse. The stripped state below is also unreachable in
	// production: capacityGroupLive always populates that sum when the rows
	// carry measured capacity.
	absent, err := capacityGroupLive(w, "status")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range absent.Buckets {
		if absent.Buckets[i].Key == string(capStatusUndetermined) {
			absent.Buckets[i].Installed = nil
			found = true
		}
	}
	if !found {
		t.Fatalf("no %s bucket to strip in %d buckets", capStatusUndetermined, len(absent.Buckets))
	}
	a := capacityAssertLive(w, len(body), surface, absent)
	if a.NoUnmodelledStatusSentinels == nil || *a.NoUnmodelledStatusSentinels {
		t.Fatalf("no_unmodelled_status_sentinels = %v, want false: the two DELICENSE rows are still there",
			a.NoUnmodelledStatusSentinels)
	}
	if len(a.Failed) != 1 {
		t.Fatalf("want exactly one failure, got %v", a.Failed)
	}
	if !strings.Contains(a.Failed[0], "181.00 MW installed") {
		t.Errorf("the rows' capacity is measured regardless of the bucket, so the sentence must still quote 181.00 MW: %q", a.Failed[0])
	}
	// THE PROTECTION THAT STILL MATTERS: an absent sum must never surface as
	// a zero. It is preserved because the clause falls back to "no capacity
	// number" only when NO undetermined row has a measured capacity.
	if strings.Contains(a.Failed[0], "0.00 MW") {
		t.Errorf("an absent sum must never be reported as 0.00: %q", a.Failed[0])
	}
	// And the fallback itself must still exist: with no measured row there is
	// no number to quote. Exercised on a workbook whose only unclassifiable
	// row publishes no capacity at all.
	synth := &nepraparse.Workbook{Plants: []nepraparse.Plant{{
		Name:              "Synthetic Unclassifiable",
		InstalledCapacity: nepraparse.ParseValue(""),
		Status:            nepraparse.StateUnknownText,
	}}}
	for i := range synth.Plants[0].Months {
		synth.Plants[0].Months[i].Utilisation = nepraparse.ParseValue("DELICENSE")
		synth.Plants[0].Months[i].Generation = nepraparse.ParseValue("DELICENSE")
	}
	if capacityStatusOf(synth.Plants[0]) == capStatusUndetermined {
		sa := capacityAssertLive(synth, 0, surface, capacityReport{})
		var sentence string
		for _, f := range sa.Failed {
			if strings.Contains(f, "cannot classify") {
				sentence = f
			}
		}
		if sentence == "" {
			t.Error("a synthetic unclassifiable row with no capacity produced no shortfall")
		} else {
			if !strings.Contains(sentence, "no capacity number") {
				t.Errorf("with no measured row the clause must read `no capacity number`: %q", sentence)
			}
			if strings.Contains(sentence, "MW installed") {
				t.Errorf("with nothing measured the sentence must not quote megawatts: %q", sentence)
			}
		}
	}

	// The evidence beside it is not affected by the missing sum.
	if !strings.Contains(a.Failed[0], "DELICENSE") {
		t.Errorf("the verbatim sentinel text must still travel: %q", a.Failed[0])
	}
}

// TestCapacityBucketFindsNestedStatusMember pins a REAL BUG found by mutation
// testing, not merely a coverage gap.
//
// capacityReport.bucket scanned r.Buckets only. That is correct for
// --by status, where the four status members ARE the top-level buckets, and
// wrong for --by technology and --by system, where every parent carries its own
// nested ByStatus split and no status key appears at the top level at all.
//
// The caller that matters is the NoUnmodelledStatusSentinels assertion, which
// looks up capStatusUndetermined to quote how much capacity the unclassifiable
// rows cover. Under the two nesting groupings the lookup missed, so the failure
// sentence read "no capacity number" for rows whose capacity IS published and
// measured — asserting an absence over real data, which is precisely what this
// command exists to refuse.
//
// FY2022-23 is the year that exposes it: two plants spell the sentinel
// "DELICENSE" (truncated, no final D), which nepraparse classifies as neither
// a status nor a blank, and they carry 181.00 MW between them. They are the
// SAME two plants — Reshma Power 97.00 MW and Gulf Powergen 84.00 MW — that
// appear in FY2023-24 with a fully blank monthly block, so the same 181.00 MW
// reaches this command through two different representations in consecutive
// years.
func TestCapacityBucketFindsNestedStatusMember(t *testing.T) {
	w, _ := capacityParse(t, "2022-23")

	// --by status exposes the member at top level; --by technology nests it
	// inside each parent's ByStatus. --by system is deliberately EXCLUDED: it
	// partitions by CPPA-G / export / K-Electric-own-fleet and carries no
	// status split at all, so there is no member there to find. That is why
	// the assertion sentence below must not depend on the grouping.
	for _, by := range []string{"status", "technology"} {
		t.Run(by, func(t *testing.T) {
			rep, err := capacityGroupLive(w, by)
			if err != nil {
				t.Fatalf("capacityGroupLive(%s): %v", by, err)
			}
			b, found := rep.bucket(capStatusUndetermined)
			if !found {
				t.Fatalf("--by %s: bucket(%q) not found; under a nesting grouping it lives inside a parent's ByStatus",
					by, capStatusUndetermined)
			}
			// It must carry the MEASURED capacity, not merely exist.
			if b.Installed == nil {
				t.Fatalf("--by %s: the undetermined member carries no installed sum", by)
			}
			mw, measured := b.Installed.Float64()
			if !measured {
				t.Fatalf("--by %s: the undetermined member's sum is UNMEASURED", by)
			}
			if mw != 181 {
				t.Errorf("--by %s: undetermined installed = %.2f MW, want 181.00 (Reshma 97.00 + Gulf Powergen 84.00)", by, mw)
			}
			if b.Plants == nil || *b.Plants != 2 {
				got := -1
				if b.Plants != nil {
					got = *b.Plants
				}
				t.Errorf("--by %s: undetermined plants = %d, want 2", by, got)
			}
			// The verbatim source text must be preserved, never interpreted.
			if len(b.UnmodelledCellText) == 0 {
				t.Errorf("--by %s: no verbatim unmodelled text recorded", by)
			}
			var sawTruncated bool
			for _, s := range b.UnmodelledCellText {
				if s == "DELICENSE" {
					sawTruncated = true
				}
			}
			if !sawTruncated {
				t.Errorf("--by %s: verbatim text %q does not include the truncated sentinel \"DELICENSE\"",
					by, b.UnmodelledCellText)
			}
		})
	}

	// Pin the exclusion rather than assume it: --by system must carry no
	// status nesting, so a future grouping change that adds one is noticed.
	t.Run("system has no status split", func(t *testing.T) {
		rep, err := capacityGroupLive(w, "system")
		if err != nil {
			t.Fatalf("capacityGroupLive(system): %v", err)
		}
		if _, found := rep.bucket(capStatusUndetermined); found {
			t.Error("--by system now carries a status member; fold it into the lookup test above")
		}
		for _, b := range rep.Buckets {
			if len(b.ByStatus) != 0 {
				t.Errorf("--by system bucket %q gained a ByStatus split", b.Key)
			}
		}
	})

	// And the assertion sentence must quote the megawatts rather than claim
	// there is no number — the user-visible half of the same bug. This runs
	// for ALL THREE groupings, including system, because the assertion is a
	// statement about the workbook and must not vary with --by.
	for _, by := range []string{"status", "technology", "system"} {
		rep, err := capacityGroupLive(w, by)
		if err != nil {
			t.Fatalf("capacityGroupLive(%s): %v", by, err)
		}
		a := capacityAssertLive(w, len(capacityFixture(t, "2022-23")), capacitySurface(t, "2022-23"), rep)
		var sentence string
		for _, s := range a.Failed {
			if strings.Contains(s, "cannot classify") {
				sentence = s
			}
		}
		// The named assertion must also read false, not merely the prose.
		if a.NoUnmodelledStatusSentinels == nil || *a.NoUnmodelledStatusSentinels {
			t.Errorf("--by %s: no_unmodelled_status_sentinels is not false despite 2 unclassifiable rows", by)
		}
		if sentence == "" {
			t.Fatalf("--by %s: no unclassifiable-rows shortfall was reported at all", by)
		}
		if strings.Contains(sentence, "no capacity number") {
			t.Errorf("--by %s: sentence claims no capacity number for 181.00 MW of published capacity: %s", by, sentence)
		}
		if !strings.Contains(sentence, "181.00 MW installed") {
			t.Errorf("--by %s: sentence does not quote the measured 181.00 MW: %s", by, sentence)
		}
	}
}

// TestCapacityByteFloorNeedsABodyToMeasure closes the last surviving mutation
// in this command: `surface.DecodedByteFloor > 0 && bodyLen > 0` at
// nepra_capacity_assert.go:198.
//
// Both conditions are required for a REASON. The floor guard alone stops a
// year with no measured floor from asserting against 0. The bodyLen guard
// stops the reverse: a caller with a measured floor but NO BODY TO MEASURE —
// any path that did not fetch — would otherwise compare 0 bytes against the
// floor, find 0 < 493187, and report "the response decoded to 0 bytes, below
// the 493187 measured for FY2023-24". That is a fabricated failure about a
// response that was never received, which is worse than asserting nothing.
//
// Flipping the && to || makes exactly that happen, and nothing noticed.
func TestCapacityByteFloorNeedsABodyToMeasure(t *testing.T) {
	w, body := capacityParse(t, "2023-24")
	surface := capacitySurface(t, "2023-24")
	if !surface.FloorsMeasured || surface.DecodedByteFloor <= 0 {
		t.Skip("FY2023-24 carries no measured byte floor; this test has no premise")
	}
	rep, err := capacityGroupLive(w, "status")
	if err != nil {
		t.Fatal(err)
	}

	// With a real body the floor IS asserted and met.
	withBody := capacityAssertLive(w, len(body), surface, rep)
	if withBody.ByteFloor == nil || withBody.ByteFloorMet == nil {
		t.Fatalf("with a %d-byte body the floor must be asserted", len(body))
	}
	if *withBody.ByteFloor != surface.DecodedByteFloor {
		t.Errorf("byte floor = %d, want %d", *withBody.ByteFloor, surface.DecodedByteFloor)
	}
	if !*withBody.ByteFloorMet {
		t.Errorf("the committed fixture is %d bytes against a %d floor and must MEET it",
			len(body), surface.DecodedByteFloor)
	}
	for _, f := range withBody.Failed {
		if strings.Contains(f, "decoded to") {
			t.Errorf("unexpected byte-floor failure on a good body: %q", f)
		}
	}

	// With NO body the floor must be ABSENT, not failed. There is nothing to
	// measure, and an unasserted check is honest where a failed one is not.
	noBody := capacityAssertLive(w, 0, surface, rep)
	if noBody.ByteFloor != nil || noBody.ByteFloorMet != nil {
		t.Errorf("with no body the byte floor must not be asserted at all; got floor=%v met=%v",
			noBody.ByteFloor, noBody.ByteFloorMet)
	}
	for _, f := range noBody.Failed {
		if strings.Contains(f, "decoded to") {
			t.Errorf("a fabricated byte-floor failure about a response that was never received: %q", f)
		}
		if strings.Contains(f, "0 bytes") {
			t.Errorf("asserted 0 bytes against a floor with no body to measure: %q", f)
		}
	}
	// The OTHER floor is unaffected by the missing body: plant rows come from
	// the parsed workbook, not from the response length.
	if noBody.PlantRowFloorMet == nil || !*noBody.PlantRowFloorMet {
		t.Error("the plant-row floor must still be asserted and met without a body")
	}
}
