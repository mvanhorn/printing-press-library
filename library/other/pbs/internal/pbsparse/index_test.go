// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package pbsparse

import (
	"os"
	"testing"
	"time"
)

const testBase = "https://www.pbs.gov.pk"

func loadPage(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("testdata/price-statistics.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(b)
}

// TestParseIndexCounts pins the parser against counts measured independently
// (in Python, straight off the same served bytes) before this code existed.
// If the two disagree, one of them is wrong and the build must stop.
func TestParseIndexCounts(t *testing.T) {
	idx, err := ParseIndex(loadPage(t), testBase)
	if err != nil {
		t.Fatalf("ParseIndex: %v", err)
	}
	if got, want := len(idx.Weekly), 148; got != want {
		t.Errorf("weekly releases = %d, want %d", got, want)
	}
	if got, want := len(idx.Monthly), 50; got != want {
		t.Errorf("monthly releases = %d, want %d", got, want)
	}
}

// TestParseIndexSortsDescending is the regression test for the trap that the
// upstream array is NOT sorted: its last element is 2024-08-08 while the true
// oldest release is 2023-07-13. Reading the tail as "oldest" understates depth
// by more than two years.
func TestParseIndexSortsDescending(t *testing.T) {
	idx, err := ParseIndex(loadPage(t), testBase)
	if err != nil {
		t.Fatalf("ParseIndex: %v", err)
	}
	w := idx.Weekly
	if len(w) < 2 {
		t.Fatal("need at least two weekly releases")
	}
	for i := 0; i+1 < len(w); i++ {
		if w[i].AsOf.Before(w[i+1].AsOf) {
			t.Fatalf("not sorted descending at %d: %s before %s",
				i, w[i].AsOfKey(), w[i+1].AsOfKey())
		}
	}
	if got, want := w[0].AsOfKey(), "2026-09-03"; got != want {
		t.Errorf("newest = %s, want %s", got, want)
	}
	if got, want := w[len(w)-1].AsOfKey(), "2023-07-13"; got != want {
		t.Errorf("oldest = %s, want %s (the unsorted-array trap)", got, want)
	}
}

// TestXLSXEraBoundary pins the measured split: only 44 of 148 weekly releases
// carry an .xlsx annexure, so the PDF path covers the other 104 and is not
// optional.
func TestXLSXEraBoundary(t *testing.T) {
	idx, err := ParseIndex(loadPage(t), testBase)
	if err != nil {
		t.Fatalf("ParseIndex: %v", err)
	}
	var withXLSX, withPDF int
	for _, r := range idx.Weekly {
		if _, ok := r.File(RoleAnnexure, "xlsx"); ok {
			if f, _ := r.File(RoleAnnexure, "xlsx"); f.Ext == "xlsx" {
				withXLSX++
			}
		}
		if f, ok := r.File(RoleAnnexure, "pdf"); ok && f.Ext == "pdf" {
			withPDF++
		}
	}
	// 48, not 44: 44 rows carry the `annexureExcel` key, and 4 more
	// (2025-10-16, 2025-07-31, 2025-07-24, 2025-07-17) point at an .xlsx from
	// the plain `annexure` key. Classifying by FILENAME finds all 48;
	// counting the key finds only 44 and understates the xlsx era.
	if got, want := withXLSX, 48; got != want {
		t.Errorf("weekly releases with xlsx annexure = %d, want %d", got, want)
	}
	if withPDF < 100 {
		t.Errorf("weekly releases with pdf annexure = %d, want >= 100", withPDF)
	}
}

// TestDuplicateURLDetection is the upstream-data-loss detector. One CPI annex
// URL is listed under both March 2024 and February 2024; the file itself
// contains only March, so February 2024 is already gone. The parser must
// surface that rather than silently overwrite one month with the other.
func TestDuplicateURLDetection(t *testing.T) {
	idx, err := ParseIndex(loadPage(t), testBase)
	if err != nil {
		t.Fatalf("ParseIndex: %v", err)
	}
	if len(idx.DuplicateURLs) == 0 {
		t.Fatal("expected at least one duplicated file URL (the Feb-2024 loss)")
	}
	var found bool
	for u, keys := range idx.DuplicateURLs {
		if len(keys) < 2 {
			t.Errorf("duplicate entry %s has %d keys, want >= 2", u, len(keys))
		}
		for _, k := range keys {
			if k == "cpi-monthly:2024-02-01" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected the February 2024 collision in %v", idx.DuplicateURLs)
	}
}

// TestFilenameDateMismatchFlagged pins the two known rows whose filename date
// disagrees with the authoritative index date. They are FLAGGED, never
// corrected — the index field wins.
func TestFilenameDateMismatchFlagged(t *testing.T) {
	idx, err := ParseIndex(loadPage(t), testBase)
	if err != nil {
		t.Fatalf("ParseIndex: %v", err)
	}
	var n int
	for _, r := range idx.Weekly {
		if r.FilenameDateMismatch {
			n++
		}
	}
	// TWO rows disagree, both flagged and neither corrected:
	//   2025-08-21 -> Annex_13.08.2025.pdf                        (8 days, annexure key)
	//   2026-02-19 -> Executive-Summary-SPI-Report_20.02.2026.pdf (1 day, report key)
	// 2025-06-04 -> Annex.pdf carries no date and agrees vacuously.
	// An earlier hand-check inspected only the annexure key and found one of the two.
	if n != 2 {
		t.Errorf("filename/index date mismatches = %d, want 2", n)
	}
}

// TestWeeklyGapsToleranceBased pins the cadence detector. A weekday rule would
// report eighteen false gaps because releases land on six different weekdays.
func TestWeeklyGapsToleranceBased(t *testing.T) {
	idx, err := ParseIndex(loadPage(t), testBase)
	if err != nil {
		t.Fatalf("ParseIndex: %v", err)
	}
	gaps := idx.WeeklyGaps()
	var missing int
	for _, g := range gaps {
		missing += g.MissingWeeks
	}
	if missing < 12 || missing > 22 {
		t.Errorf("total missing weeks = %d, want 12..22 (measured 17)", missing)
	}
	// The autumn-2024 hole is the largest and must be present.
	var foundBig bool
	for _, g := range gaps {
		if g.After == "2024-09-12" && g.Before == "2024-11-07" {
			foundBig = true
			if g.MissingWeeks < 6 {
				t.Errorf("autumn-2024 hole missing_weeks = %d, want >= 6", g.MissingWeeks)
			}
		}
	}
	if !foundBig {
		t.Errorf("expected the 2024-09-12..2024-11-07 hole in %+v", gaps)
	}
}

// TestConstructionFilesOnSomeMonths pins the trailing-space key trap: the CPI
// array literally contains `Urban :` beside `Rural:`, and without trimming the
// key thirteen months of construction data vanish silently.
func TestConstructionFilesOnSomeMonths(t *testing.T) {
	idx, err := ParseIndex(loadPage(t), testBase)
	if err != nil {
		t.Fatalf("ParseIndex: %v", err)
	}
	var months int
	for _, r := range idx.Monthly {
		if _, ok := r.File(RoleConstruction, ""); ok {
			months++
		}
	}
	if got, want := months, 13; got != want {
		t.Errorf("CPI months with construction files = %d, want %d (trailing-space key trap)", got, want)
	}
}

func TestClassifyFileRole(t *testing.T) {
	cases := []struct {
		name string
		want FileRole
	}{
		{"Annex_03.09.2026.xlsx", RoleAnnexure},
		{"Annex_03.09.2026.pdf", RoleAnnexure},
		{"Annex.pdf", RoleAnnexure},
		{"SPI_AnnexUSCP_13072023.pdf", RoleAnnexure},
		{"SPI20Annex26USCP_21032024.pdf", RoleAnnexure},
		{"Annexture220USCP_03.09.2026.pdf", RoleAnnexure},
		{"3.-SPI-Report-03.09.2026.xlsx", RoleReport},
		{"SPI_Executive-SumaryReport_28102023.pdf", RoleReport},
		// Contains BOTH "Sumary" and "Report" and also no "annex": report.
		{"SPI-Executive-SumarySPI-Report_11112023.pdf", RoleReport},
		{"Monthly-Review-August-2026-1.pdf", RoleUnknown},
		{"Urbn-Construction-9.xlsx", RoleConstruction},
		{"Rural-Construction-11.xlsx", RoleConstruction},
	}
	for _, c := range cases {
		if got := ClassifyFileRole(c.name); got != c.want {
			t.Errorf("ClassifyFileRole(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestResolveURLMixedConventions(t *testing.T) {
	// The weekly array mixes relative-without-slash and absolute forms.
	cases := []struct{ in, want string }{
		{"wp-content/uploads/2020/07/Annex_03.09.2026.xlsx", testBase + "/wp-content/uploads/2020/07/Annex_03.09.2026.xlsx"},
		{"/wp-content/uploads/2020/07/x.pdf", testBase + "/wp-content/uploads/2020/07/x.pdf"},
		{"https://www.pbs.gov.pk/wp-content/uploads/2020/07/y.pdf", "https://www.pbs.gov.pk/wp-content/uploads/2020/07/y.pdf"},
		{"", ""},
	}
	for _, c := range cases {
		if got := resolveURL(testBase, c.in); got != c.want {
			t.Errorf("resolveURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFilenameDateAgrees(t *testing.T) {
	asOf := time.Date(2025, 8, 21, 0, 0, 0, 0, time.UTC)
	if filenameDateAgrees("Annex_13.08.2025.pdf", asOf) {
		t.Error("13.08.2025 should NOT agree with 2025-08-21")
	}
	if !filenameDateAgrees("Annex_21.08.2025.pdf", asOf) {
		t.Error("21.08.2025 should agree with 2025-08-21")
	}
	// No date in the filename agrees vacuously; "Annex.pdf" is a real row.
	if !filenameDateAgrees("Annex.pdf", asOf) {
		t.Error("a filename with no date must agree vacuously")
	}
}

func TestParseIndexRejectsPageWithoutArrays(t *testing.T) {
	if _, err := ParseIndex("<html><body>nothing here</body></html>", testBase); err == nil {
		t.Fatal("expected an error when neither array is present")
	}
}
