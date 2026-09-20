// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package nepraper

import (
	"strings"
	"testing"
)

// TestFusedCellIsFlaggedNotNumbered is finding 7. assignCells glues a band's
// tokens with NO separator, which is required — NEPRA emits one text-showing
// operation per glyph, so "851"+"6"+"."+"45" must rejoin as 8516.45 — but it
// means two genuinely different cells landing in one band fuse into a
// plausible number. 3547 and 1182 become 35471182, KindNumeric, unflagged,
// because the two-decimal-point rule only catches decimals.
//
// The guard cannot be geometric. MEASURED across all three fixtures: the
// largest LEGITIMATE gap between consecutive tokens inside one band is
// 35.88pt, while the FY2018-19 chart-axis label that would have fused sat
// 38.8pt away — and 79.1% of spans carry a ZERO advance width, so a "gap" is
// really the distance between glyph start positions and grows with the
// preceding token's length. So the guard is statistical, against the column's
// own other rows.
func TestFusedCellIsFlaggedNotNumbered(t *testing.T) {
	// An INTEGER column, which is where the hazard actually lives: a fusion
	// inside a column of decimals produces two decimal points and is already
	// refused by ParseFigure. These are pending-connection counts.
	rows := [][]string{
		{"3547"}, {"1182"}, {"8516"}, {"4152"}, {"1185"},
		{"13469"}, {"2304"}, {"250"}, {"841"},
		{"35471182"}, // 3547 and 1182 glued together
	}
	fused := flagFusedCells(rows, 1)
	if _, bad := fused[9][0]; !bad {
		t.Errorf("the fused cell %q was not flagged; flags = %v", rows[9][0], fused)
	}
	for i := 0; i < 9; i++ {
		if reason, bad := fused[i][0]; bad {
			t.Errorf("legitimate cell %q was flagged: %s", rows[i][0], reason)
		}
	}
	if reason := fused[9][0]; !strings.Contains(reason, "glued together") || !strings.Contains(reason, "unverified") {
		t.Errorf("the reason does not explain itself: %q", reason)
	}

	// A flagged cell must reach the caller as UNVERIFIED, never as a number.
	cols := []Column{{Index: 0, Header: "SAIDI FY 2024-25", Role: RoleReported}}
	tbl := &Table{Label: "Table 6", Page: 15, Metric: "saidi", Variant: VariantHeadline, Columns: cols, ValueColumn: 0}
	got := buildObservations(dataRow{entity: "MEPCO"}, rows[9], cols, tbl, "FY2024-25", fused[9])
	if len(got) != 1 {
		t.Fatalf("observations = %d, want 1", len(got))
	}
	if n, ok := got[0].Value.Float(); ok {
		t.Errorf("the fused cell reached the caller as the number %v; want unverified", n)
	}
	if got[0].Value.Kind != KindUnverified {
		t.Errorf("fused cell kind = %s, want unverified", got[0].Value.Kind)
	}
	if got[0].Value.Raw != "35471182" {
		t.Errorf("the raw text was not preserved: %q", got[0].Value.Raw)
	}

	// And an unflagged cell in the same shape must still be a number, or the
	// guard has broken the parser.
	ok := buildObservations(dataRow{entity: "MEPCO"}, rows[0], cols, tbl, "FY2024-25", nil)
	if n, isNum := ok[0].Value.Float(); !isNum || n != 3547 {
		t.Errorf("an unflagged cell parsed as (%v,%v), want (3547,true)", n, isNum)
	}
}

// TestFY2018_19NearMissIsCaught is the reviewer's concrete live near-miss.
// FY2018-19 Table 6 has chart axis labels sharing data-row baselines; the
// parser logged "25000" at x=331.3 as falling outside every band, which ends
// at x=292.5 — a miss of 38.8pt. Had it landed inside, it would have glued
// onto PESCO's breach cell "0.00" to make "0.0025000": ONE decimal point, all
// digits, so ParseFigure would have returned Numeric(0.0025) with no flag at
// all.
func TestFY2018_19NearMissIsCaught(t *testing.T) {
	// A breach column as FY2018-19 publishes it, with the fusion in row 0.
	rows := [][]string{
		{"0.0025000"}, {"0.16"}, {"0.44"}, {"1.74"}, {"0.00"},
		{"2.82"}, {"0.00"}, {"1.05"}, {"0.31"}, {"0.00"},
	}
	// Confirm the premise: unflagged, this parses as a plausible number.
	if v := ParseFigure(rows[0][0]); v.Kind != KindNumeric {
		t.Fatalf("premise wrong: %q parses as %s, not a silent number", rows[0][0], v.Kind)
	} else if n, _ := v.Float(); n != 0.0025 {
		t.Fatalf("premise wrong: %q parses to %v", rows[0][0], n)
	}
	fused := flagFusedCells(rows, 1)
	if _, bad := fused[0][0]; !bad {
		t.Errorf("the fused breach cell %q was not flagged; flags = %v", rows[0][0], fused)
	}
	for i := 1; i < len(rows); i++ {
		if reason, bad := fused[i][0]; bad {
			t.Errorf("legitimate breach %q was flagged: %s", rows[i][0], reason)
		}
	}
}

// TestFusionCheckMakesNoClaimOnASmallColumn guards the other direction: a
// false Unverified HIDES a real published figure, so a column with too few
// numeric cells to have a spread must not be judged at all.
func TestFusionCheckMakesNoClaimOnASmallColumn(t *testing.T) {
	// n is the TOTAL number of numeric cells, the flagged one included: the
	// suspect is part of its own control population.
	for n := 2; n <= minFusionPopulation; n++ {
		rows := make([][]string, 0, n)
		for i := 0; i < n-1; i++ {
			rows = append(rows, []string{"3067"})
		}
		rows = append(rows, []string{"35471182"})
		if fused := flagFusedCells(rows, 1); len(fused) != 0 && n < minFusionPopulation {
			t.Errorf("a column with %d numeric cells produced a fusion claim: %v", n, fused)
		}
	}
	// Ordinary variation inside one column must not be flagged either: real
	// SAIDI across the ten DISCOs spans 30.67 to 13469.55, a legitimate 4-to-7
	// digit range.
	ordinary := [][]string{
		{"250.30"}, {"841.17"}, {"1182.56"}, {"3547.00"}, {"4152.59"},
		{"8516.45"}, {"13469.55"}, {"1185.68"}, {"2304.60"}, {"30.67"},
	}
	if fused := flagFusedCells(ordinary, 1); len(fused) != 0 {
		t.Errorf("ordinary SAIDI spread was flagged as fusion: %v", fused)
	}
}

// TestRealFixturesHaveNoFusedCells is the false-positive guard that matters
// most: the three committed fixtures reproduce 90/90 and 120/120 published
// figures exactly, so ANY fusion flag on them is a bug in the detector, and
// every one of those figures must still arrive as a number.
func TestRealFixturesHaveNoFusedCells(t *testing.T) {
	for _, name := range []string{"fy2014-15.spans.tsv", "fy2018-19.spans.tsv", "fy2024-25.spans.tsv"} {
		fx := loadFixture(t, name)
		r := fx.report(t)
		for _, n := range r.Notes {
			if strings.Contains(n.Text, "glued together") {
				t.Errorf("%s: false fusion flag: %s", fx.FY, n.Text)
			}
		}
		var unverified int
		for _, o := range r.Observations {
			if o.Value.Kind == KindUnverified && strings.Contains(o.Value.Reason, "glued together") {
				t.Errorf("%s: %s/%s was demoted to unverified by the fusion check: %s",
					fx.FY, o.Entity, o.Metric, o.Value.Reason)
				unverified++
			}
		}
		if unverified > 0 {
			t.Errorf("%s: the fusion check demoted %d real published figures", fx.FY, unverified)
		}
	}
}

// TestReasonsDoNotMisattributeProvenance is finding 9d. A reason is a claim
// about the SOURCE, so it must not assert a document property that the token
// alone cannot establish.
func TestReasonsDoNotMisattributeProvenance(t *testing.T) {
	// A trailing-dot token has two possible causes and must not pick one.
	for _, raw := range []string{"4152.", "68.", "28.", "49.", "19,535."} {
		v := ParseFigure(raw)
		if v.Kind != KindUnverified {
			t.Errorf("ParseFigure(%q).Kind = %s, want unverified", raw, v.Kind)
		}
		if _, ok := v.Float(); ok {
			t.Errorf("ParseFigure(%q) yielded a number", raw)
		}
		// In this corpus every one of these except 19,535. is a split cell
		// that the band reassembles, so the reason must not assert the
		// chart-label provenance as fact.
		if strings.Contains(v.Reason, "the source is a truncated chart label") {
			t.Errorf("ParseFigure(%q) asserts a chart-label provenance it cannot know: %q", raw, v.Reason)
		}
		if !strings.Contains(v.Reason, "not determinable") {
			t.Errorf("ParseFigure(%q) does not disclose the ambiguity: %q", raw, v.Reason)
		}
	}

	// Punctuation-only tokens are stray glyphs, not published data.
	for _, raw := range []string{"(", "()", "%", "-.", "/"} {
		v := ParseFigure(raw)
		if v.Kind == KindQualitative {
			t.Errorf("ParseFigure(%q) = qualitative, asserting NEPRA published a datum there", raw)
		}
		if _, ok := v.Float(); ok {
			t.Errorf("ParseFigure(%q) yielded a number", raw)
		}
	}

	// But a real published label must still be qualitative.
	for _, raw := range []string{"Far Away", "Away", "Not Reported"} {
		if v := ParseFigure(raw); v.Kind != KindQualitative {
			t.Errorf("ParseFigure(%q).Kind = %s, want qualitative: it IS a published datum", raw, v.Kind)
		}
	}
}

// TestTableValueColumnSentinelIsReadable is the other half of finding 9e.
func TestTableValueColumnSentinelIsReadable(t *testing.T) {
	// A real table identifies its column.
	fx := loadFixture(t, "fy2024-25.spans.tsv")
	r := fx.report(t)
	var checked int
	for _, tbl := range r.Tables {
		if idx, ok := tbl.ValueColumnIndex(); ok {
			if idx < 0 {
				t.Errorf("%s reports column %d as identified", tbl.Label, idx)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Error("no table in the FY2024-25 fixture identified a value column")
	}
	// The unknown sentinel must read as unknown, not as column 0.
	unknown := Table{ValueColumn: -1}
	if idx, ok := unknown.ValueColumnIndex(); ok || idx >= 0 {
		t.Errorf("a -1 ValueColumn read as (%d,%v), want (-1,false)", idx, ok)
	}
}
