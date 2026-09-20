package nepraparse

import (
	"errors"
	"fmt"
	"strings"
)

// ErrSchemaDrift is returned when the header band does not match the schema
// this package was measured against. It is fatal on purpose: a shifted or
// renamed column produces numbers that look right, so the parse must stop
// rather than hand back a corrupted panel.
var ErrSchemaDrift = errors.New("nepraparse: header band does not match the expected schema")

// ErrNoHeaderBand is returned when the three-row header band cannot be
// located at all.
var ErrNoHeaderBand = errors.New("nepraparse: could not locate the 3-row header band")

// LogicalColumns is the number of logical columns in every sampled year: six
// identity/capacity columns plus thirteen month/metric pairs.
//
// Physical row width is larger — 40 in FY2017-18 and FY2020-21, 39 in
// FY2023-24 — but the surplus is trailing empty padding from the Excel export
// and never shifts a real column. The padding is not reliably empty either:
// one row per year carries a stray ";" in its last cell, which is why the
// logical width comes from the header band rather than from "strip trailing
// all-empty columns".
const LogicalColumns = 32

// Column indices of the six stub columns. They are constants because every
// sampled year has byte-identical header text in the same order.
const (
	ColSNo = iota
	ColName
	ColTechnology
	ColFuel
	ColInstalledCapacity
	ColDependableCapacity
	// ColFirstMonthPair is where the 13 month/metric pairs begin.
	ColFirstMonthPair
)

// PctOffset and GWhOffset are the offsets within a month pair.
//
// "% age" comes FIRST, then "GWh" — confirmed on Tarbela FY2023-24, whose
// col6 is 94.94 (a utilisation percentage) and col7 is 2,456.58 (GWh).
// Swapping them exchanges all 26 monthly fields and still yields plausible
// numbers, so [Workbook.ColumnOrder] re-derives the order from the data.
const (
	PctOffset = 0
	GWhOffset = 1
)

// PairCount is the number of month/metric pairs: twelve months plus Sum.
const PairCount = 13

// SumPairIndex is the pair index of the annual total. The column is literally
// labelled "Sum", not "Total".
const SumPairIndex = 12

// stubColumnNames are the first six header labels, verbatim.
var stubColumnNames = [ColFirstMonthPair]string{
	"S.No",
	"Name of Companies",
	"Technology",
	"Fuel",
	"Installed Capacity (MW)",
	"Dependable Capacity (MW)",
}

// pairLabels are the thirteen month-row labels, in fiscal order, ending in
// "Sum".
var pairLabels = [PairCount]string{
	"Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
	"Jan", "Feb", "Mar", "Apr", "May", "Jun",
	"Sum",
}

// MetricPct is the literal leaf label of the utilisation column: "% age",
// WITH the space. `grep -c '>% age<'` returns exactly 13 per year.
const MetricPct = "% age"

// MetricGWh is the literal leaf label of the generation column.
const MetricGWh = "GWh"

// CanonicalColumns returns the 32 logical column names in order, each month
// column flattened to "<month> <metric>" — the fingerprint a caller compares
// against to prove the schema has not drifted.
func CanonicalColumns() [LogicalColumns]string {
	var out [LogicalColumns]string
	copy(out[:], stubColumnNames[:])
	for p := 0; p < PairCount; p++ {
		out[ColFirstMonthPair+2*p+PctOffset] = pairLabels[p] + " " + MetricPct
		out[ColFirstMonthPair+2*p+GWhOffset] = pairLabels[p] + " " + MetricGWh
	}
	return out
}

// HeaderFingerprint is what the parser actually read out of the header band,
// so a caller can assert the schema rather than trust it.
type HeaderFingerprint struct {
	// Columns is the flattened 32-name fingerprint, comparable to
	// [CanonicalColumns].
	Columns [LogicalColumns]string
	// MonthRow is the middle band row as read (tr#3): the thirteen
	// colspan=2 month labels, each duplicated across its pair.
	MonthRow [LogicalColumns]string
	// LeafRow is the bottom band row as read (tr#4): the thirty-two leaf
	// labels, alternating "% age" and "GWh" from column 6.
	LeafRow [LogicalColumns]string
	// BandLabel is the <td colspan=26> label above the months, e.g.
	// "FY 2023-24". It is the only in-document statement of which year the
	// file covers.
	BandLabel string
	// BandRows are the source row indices of the three band rows.
	BandRows [3]int
	// PctLeafCount is how many leaf cells read exactly "% age". Every
	// sampled year has 13.
	PctLeafCount int
}

// Fingerprint returns the 32 column names joined with "|", a single string a
// caller can store and compare across releases.
func (h HeaderFingerprint) Fingerprint() string {
	return strings.Join(h.Columns[:], "|")
}

// Matches reports whether the band matched the canonical schema exactly.
func (h HeaderFingerprint) Matches() bool {
	return h.Columns == CanonicalColumns()
}

// Diff returns one human-readable line per column that differs from the
// canonical schema. Empty when the schema matched.
func (h HeaderFingerprint) Diff() []string {
	want := CanonicalColumns()
	var out []string
	for i := range want {
		if h.Columns[i] != want[i] {
			out = append(out, fmt.Sprintf("column %d: want %q, got %q", i, want[i], h.Columns[i]))
		}
	}
	return out
}

// findHeaderBand locates the three band rows by text, because these files
// contain ZERO <th>: any library that looks for semantic header cells reports
// "no header" and reads row 0 as data.
//
// The anchor is the leaf row — the only row with thirteen cells reading
// exactly "% age". The month row is the row above it and the band-label row
// the one above that.
func findHeaderBand(g *Grid) (HeaderFingerprint, error) {
	leaf := -1
	for i, row := range g.Rows {
		n := 0
		for _, c := range row {
			if c == MetricPct {
				n++
			}
		}
		if n == PairCount {
			leaf = i
			break
		}
	}
	if leaf < 2 {
		return HeaderFingerprint{}, fmt.Errorf("%w: no row carries %d %q leaf cells", ErrNoHeaderBand, PairCount, MetricPct)
	}
	if g.Width < LogicalColumns {
		return HeaderFingerprint{}, fmt.Errorf("%w: physical width %d is narrower than the %d logical columns",
			ErrNoHeaderBand, g.Width, LogicalColumns)
	}

	fp := HeaderFingerprint{
		BandRows: [3]int{leaf - 2, leaf - 1, leaf},
	}
	copy(fp.LeafRow[:], g.Rows[leaf][:LogicalColumns])
	copy(fp.MonthRow[:], g.Rows[leaf-1][:LogicalColumns])
	fp.BandLabel = g.Rows[leaf-2][ColFirstMonthPair]
	for _, c := range fp.LeafRow {
		if c == MetricPct {
			fp.PctLeafCount++
		}
	}
	for i := 0; i < ColFirstMonthPair; i++ {
		fp.Columns[i] = fp.LeafRow[i]
	}
	for i := ColFirstMonthPair; i < LogicalColumns; i++ {
		fp.Columns[i] = collapseText(fp.MonthRow[i] + " " + fp.LeafRow[i])
	}
	return fp, nil
}

// KnownTechnologies is the complete, stable set of Technology values —
// exactly 9, the identical set in FY2017-18, FY2020-21 and FY2023-24.
// "THERMAL- COAL" carries the upstream space after the hyphen and "Coal" is
// the only mixed-case entry; both are reproduced verbatim.
var KnownTechnologies = []string{
	"THERMAL",
	"THERMAL- COAL",
	"Coal",
	"HYDEL-(WAPDA)",
	"HYDEL-(IPP)",
	"NUCLEAR",
	"WIND",
	"SOLAR",
	"BIOGAS",
}

// KnownFuels is the complete, stable set of Fuel values — exactly 16, the
// identical set in all three sampled years.
var KnownFuels = []string{
	"WIND",
	"HYDEL",
	"RFO",
	"COAL",
	"BAGASSE",
	"SOLAR",
	"GAS",
	"NUCLEAR",
	"RLNG",
	"RLNG/HSD",
	"GAS/HSD",
	"RFO/RLNG/HSD",
	"BAGASSE/COAL",
	"Natural Gas",
	"Natural Gas/ Furnace Oil",
	"Lignite Coal",
}

var (
	technologySet = newStringSet(KnownTechnologies)
	fuelSet       = newStringSet(KnownFuels)
)

func newStringSet(vs []string) map[string]struct{} {
	m := make(map[string]struct{}, len(vs))
	for _, v := range vs {
		m[v] = struct{}{}
	}
	return m
}

// KnownTechnology reports whether s is one of the 9 measured Technology
// values. An unknown value is recorded and flagged by the parser, never
// rejected: the vocabulary is validated, not enforced.
func KnownTechnology(s string) bool {
	_, ok := technologySet[s]
	return ok
}

// KnownFuel reports whether s is one of the 16 measured Fuel values. As with
// technology, an unknown value is recorded and flagged rather than rejected.
func KnownFuel(s string) bool {
	_, ok := fuelSet[s]
	return ok
}
