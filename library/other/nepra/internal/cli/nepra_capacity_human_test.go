// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
//
// The HUMAN table is the only surface most readers see, and it is where the
// absent-vs-measured distinction is easiest to lose: every cell here is a
// string, so a rendering bug degrades silently instead of failing a decode.
//
// EVERY NUMBER ASSERTED HERE WAS MEASURED by rendering capacityWriteHuman
// over the committed FY2020-21 fixture, not copied from a document.

package cli

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// capacityHumanRender runs the real render path over a committed fixture.
func capacityHumanRender(t *testing.T, fy, by string) string {
	t.Helper()
	w, body := capacityParse(t, fy)
	rep, err := capacityGroupLive(w, by)
	if err != nil {
		t.Fatalf("capacityGroupLive(FY%s, %s): %v", fy, by, err)
	}
	surface := capacitySurface(t, fy)
	meta := capacityMeta(surface, by, "live", rep, capacityAssertLive(w, len(body), surface, rep))
	var out bytes.Buffer
	cmd := &cobra.Command{Use: "capacity"}
	cmd.SetOut(&out)
	if err := capacityWriteHuman(cmd, meta, rep); err != nil {
		t.Fatalf("capacityWriteHuman(FY%s, %s): %v", fy, by, err)
	}
	return out.String()
}

// capacityHumanRow returns the single rendered line whose first column is
// label, trimmed of the tabwriter's trailing padding.
func capacityHumanRow(t *testing.T, render, label string) string {
	t.Helper()
	var found []string
	for _, line := range strings.Split(render, "\n") {
		trimmed := strings.TrimRight(line, " \t")
		if strings.HasPrefix(trimmed, label) {
			found = append(found, trimmed)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want exactly one rendered row starting %q, got %d:\n%s", label, len(found), render)
	}
	return found[0]
}

// capacityTabCells splits a rendered row on the tabwriter's 2-or-more-space
// column gutter. It is only safe on rows where every column is non-empty.
func capacityTabCells(row string) []string {
	return regexp.MustCompile(`\s{2,}`).Split(row, -1)
}

// TestCapacityHumanLedgerQuotesPublishedStringForUnavailableRows pins the
// reconciliation ledger's two-way split, which is the whole point of
// published_as: a figure NEPRA published but this CLI cannot re-derive is
// rendered as the VERBATIM string, and a figure this build has never read
// stays `<unavailable>` with nothing quoted under it.
//
// MUTATION: flip `r.PublishedAs != ""` to `== ""` in capacityWriteHuman and
// the licence register's published string vanishes behind `<unavailable>`
// while sir_2020-21_system — a year whose SIR figure this build has NEVER
// read — starts claiming "published as" nothing at all, which is an invented
// attribution.
func TestCapacityHumanLedgerQuotesPublishedStringForUnavailableRows(t *testing.T) {
	render := capacityHumanRender(t, "2020-21", "system")

	// Read, published under another scope, NOT re-derivable here: the string
	// travels verbatim into the installed column.
	quoted := capacityTabCells(capacityHumanRow(t, render, "licence_register_gross"))
	if len(quoted) < 3 {
		t.Fatalf("licence_register_gross row did not render 3 columns: %q", quoted)
	}
	if quoted[1] != "unavailable" {
		t.Errorf("licence_register_gross state = %q, want unavailable", quoted[1])
	}
	if quoted[2] != "published as 47,559.97 MW over 331 of 335 entities" {
		t.Errorf("licence_register_gross installed cell = %q, want the verbatim published string "+
			"\"published as 47,559.97 MW over 331 of 335 entities\"", quoted[2])
	}

	// Never read for this year. NOTHING may be quoted here: the FY2023-24
	// SIR strings describe a different year's stock and an empty
	// published_as must not render as a published-as claim either.
	never := capacityTabCells(capacityHumanRow(t, render, "sir_2020-21_system"))
	if len(never) < 3 {
		t.Fatalf("sir_2020-21_system row did not render 3 columns: %q", never)
	}
	if never[2] != "<unavailable>" {
		t.Errorf("sir_2020-21_system installed cell = %q, want <unavailable>: this build read no SIR "+
			"figure for FY2020-21, so it may not print a `published as` prefix", never[2])
	}
	if strings.Contains(never[2], "published as") {
		t.Errorf("sir_2020-21_system must not claim a published figure: %q", never[2])
	}
	// And the quoting must be attached to the row that carries the string,
	// not sprayed over the ledger.
	if got := strings.Count(render, "published as "); got != 1 {
		t.Errorf("FY2020-21 must render exactly one `published as` cell, got %d:\n%s", got, render)
	}
}

// TestCapacityHumanNoCapacityNumberColumnPrintsMeasuredCounts pins the NO
// CAPACITY NUMBER column against the row that needs it most: FY2020-21's
// three plants that publish no installed sum at all, where the only thing
// explaining the gap is the per-plant cell counts beside it —
// 3 blank / 3 status / 0 unmodelled, measured by rendering this fixture.
//
// THE MEMBER CHANGED, AND THE OLD EXPECTATION WAS THE BUG. This test used to
// assert those counts on listed_no_data, and its own expected string is what
// gives the contradiction away: "3 blank / 3 STATUS" in a bucket whose
// definition is "NO status token". Those three plants (Tenaga Generasi,
// Hydrochina Dawood, Zephyr Power) publish the sentinel
// "Export to K.Electric" in their Installed Capacity cell, so they now sit in
// export_to_k_electric, where a status cell count of 3 is exactly right.
// listed_no_data is correctly empty for this year.
//
// MUTATION: flip `b.capacityCellCounts != nil` to `== nil` in
// capacityWriteBucketRow and every row that HAS counts prints `<absent>`
// instead, so the table stops explaining any unmeasured sum.
func TestCapacityHumanNoCapacityNumberColumnPrintsMeasuredCounts(t *testing.T) {
	render := capacityHumanRender(t, "2020-21", "status")

	const measured = "3 blank / 3 status / 0 unmodelled"
	for _, label := range []string{"export_to_k_electric", "TOTAL published"} {
		row := capacityHumanRow(t, render, label)
		if !strings.HasSuffix(row, measured) {
			t.Errorf("%s row must end with the measured counts %q, got %q", label, measured, row)
		}
		if strings.Contains(row, "<absent>") {
			t.Errorf("%s HAS cell counts and must not render <absent>: %q", label, row)
		}
	}
	// The counts are the reason the bucket has no megawatt sum, so the sum
	// itself must stay unmeasured rather than collapse to 0.00.
	cells := capacityTabCells(capacityHumanRow(t, render, "export_to_k_electric"))
	if len(cells) < 4 {
		t.Fatalf("export_to_k_electric row did not render its columns: %q", cells)
	}
	if cells[1] != "3" {
		t.Errorf("export_to_k_electric plants = %q, want 3", cells[1])
	}
	if cells[2] != "<not measured>" || cells[3] != "<not measured>" {
		t.Errorf("export_to_k_electric installed/dependable = %q/%q, want <not measured> for both", cells[2], cells[3])
	}

	// AND listed_no_data must be EMPTY for this year. Its basis text asserts
	// "capacity published, NO status token"; booking a plant there whose
	// capacity cell holds a status token makes the bucket lie about itself.
	lnd := capacityTabCells(capacityHumanRow(t, render, "listed_no_data"))
	if len(lnd) >= 2 && lnd[1] != "0" {
		t.Errorf("listed_no_data plants = %q, want 0: FY2020-21's three no-data plants publish the "+
			"\"Export to K.Electric\" sentinel in their capacity cell, so they belong to "+
			"export_to_k_electric. Booking them here makes the bucket's own basis text false", lnd[1])
	}
}

// TestCapacityHumanNoCapacityNumberColumnIsAbsentForAbsentBucket is the
// other half: k_electric_own_fleet carries NO counts struct at all, because
// K-Electric's fleet is outside these workbooks' scope, and
// absent-from-dataset is not a count of zero.
//
// MUTATION: the same `!= nil` -> `== nil` flip reads three counts off a nil
// embedded pointer for exactly this row.
func TestCapacityHumanNoCapacityNumberColumnIsAbsentForAbsentBucket(t *testing.T) {
	render := capacityHumanRender(t, "2020-21", "system")

	kel := capacityHumanRow(t, render, "k_electric_own_fleet")
	if !strings.HasSuffix(kel, "<absent>") {
		t.Errorf("k_electric_own_fleet must render <absent> in NO CAPACITY NUMBER, got %q", kel)
	}
	if strings.Contains(kel, "blank /") {
		t.Errorf("k_electric_own_fleet carries no cell counts and must print none, got %q", kel)
	}
	if strings.Contains(kel, "0 blank") {
		t.Errorf("k_electric_own_fleet must never print a zero count for an absent measurement, got %q", kel)
	}
	// Its neighbours in the same table DO carry counts, so `<absent>` here
	// is a statement about this bucket and not about the column.
	const measured = "3 blank / 3 status / 0 unmodelled"
	for _, label := range []string{"cppag", "export_to_k_electric", "TOTAL published"} {
		row := capacityHumanRow(t, render, label)
		if !strings.HasSuffix(row, measured) {
			t.Errorf("%s row must end with the measured counts %q, got %q", label, measured, row)
		}
	}
}
