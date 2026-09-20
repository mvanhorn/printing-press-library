package nepraparse

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// TestCanonicalColumns pins the 32 verbatim column names, in order, including
// the two spellings that are easy to "fix" by accident: "% age" with a space,
// and a total column literally called "Sum" rather than "Total".
func TestCanonicalColumns(t *testing.T) {
	got := CanonicalColumns()
	want := [LogicalColumns]string{
		"S.No",
		"Name of Companies",
		"Technology",
		"Fuel",
		"Installed Capacity (MW)",
		"Dependable Capacity (MW)",
		"Jul % age", "Jul GWh",
		"Aug % age", "Aug GWh",
		"Sep % age", "Sep GWh",
		"Oct % age", "Oct GWh",
		"Nov % age", "Nov GWh",
		"Dec % age", "Dec GWh",
		"Jan % age", "Jan GWh",
		"Feb % age", "Feb GWh",
		"Mar % age", "Mar GWh",
		"Apr % age", "Apr GWh",
		"May % age", "May GWh",
		"Jun % age", "Jun GWh",
		"Sum % age", "Sum GWh",
	}
	if got != want {
		t.Errorf("CanonicalColumns() =\n  %q\nwant\n  %q", got, want)
	}
	if MetricPct != "% age" {
		t.Errorf("MetricPct = %q, want %q (the space is in the source)", MetricPct, "% age")
	}
	if pairLabels[SumPairIndex] != "Sum" {
		t.Errorf("total column label = %q, want %q", pairLabels[SumPairIndex], "Sum")
	}
	// Months must be in fiscal order, not calendar order.
	wantMonths := []string{"Jul", "Aug", "Sep", "Oct", "Nov", "Dec", "Jan", "Feb", "Mar", "Apr", "May", "Jun"}
	if !reflect.DeepEqual(pairLabels[:12], wantMonths) {
		t.Errorf("month order = %q, want fiscal order %q", pairLabels[:12], wantMonths)
	}
}

// TestHeaderBandIsIdenticalAcrossYears is the schema-stability assertion: the
// same 32 names, in the same order, with byte-identical text, in all three
// published years. Only the band label differs.
func TestHeaderBandIsIdenticalAcrossYears(t *testing.T) {
	canonical := CanonicalColumns()
	var firstFingerprint string
	for _, y := range fullYears {
		t.Run(y.fy, func(t *testing.T) {
			w := parse(t, y.name, y.fy)
			h := w.Header

			if h.Columns != canonical {
				t.Errorf("header drift: %v", h.Diff())
			}
			if !h.Matches() {
				t.Error("Matches() = false, want true")
			}
			if h.BandRows != [3]int{2, 3, 4} {
				t.Errorf("BandRows = %v, want [2 3 4] (tr#0 spacer, tr#1 title, tr#2..4 band)", h.BandRows)
			}
			if want := "FY " + y.fy; h.BandLabel != want {
				t.Errorf("BandLabel = %q, want %q", h.BandLabel, want)
			}
			if h.PctLeafCount != 13 {
				t.Errorf("PctLeafCount = %d, want 13", h.PctLeafCount)
			}
			// The literal '>% age<' occurrence count is 13 in the raw bytes.
			if got := bytes.Count(fixture(t, y.name), []byte(">% age<")); got != 13 {
				t.Errorf("raw '>%% age<' occurrences = %d, want 13", got)
			}
			// The leaf row alternates % age, GWh from column 6 onward.
			for i := ColFirstMonthPair; i < LogicalColumns; i += 2 {
				if h.LeafRow[i] != MetricPct || h.LeafRow[i+1] != MetricGWh {
					t.Fatalf("leaf columns %d/%d = %q/%q, want %q/%q — the pair order is the whole ballgame",
						i, i+1, h.LeafRow[i], h.LeafRow[i+1], MetricPct, MetricGWh)
				}
			}
			// The title row sits immediately above the band.
			if !strings.HasPrefix(w.Title, "Details of Installed & Dependable Capacity (MW)") {
				t.Errorf("Title = %q, want the colspan=32 title line", w.Title)
			}
			if firstFingerprint == "" {
				firstFingerprint = h.Fingerprint()
			} else if h.Fingerprint() != firstFingerprint {
				t.Errorf("fingerprint differs from the first year:\n  %q\n  %q", h.Fingerprint(), firstFingerprint)
			}
		})
	}
}

// TestSchemaDriftFailsTheParse covers the requirement that a changed header
// stops the parse instead of quietly producing a mis-aligned panel.
func TestSchemaDriftFailsTheParse(t *testing.T) {
	base := string(fixture(t, trimFY2324))

	tests := []struct {
		name    string
		mutate  func(string) string
		wantErr error
	}{
		{
			name:    "a renamed stub column is drift",
			mutate:  func(s string) string { return strings.Replace(s, "Name of\n  Companies", "Company Name", 1) },
			wantErr: ErrSchemaDrift,
		},
		{
			name:    "Sum renamed to Total is drift",
			mutate:  func(s string) string { return strings.Replace(s, ">Sum<", ">Total<", 1) },
			wantErr: ErrSchemaDrift,
		},
		{
			name: "reordering the month labels is drift",
			mutate: func(s string) string {
				s = strings.Replace(s, ">Jul<", ">XXX<", 1)
				s = strings.Replace(s, ">Aug<", ">Jul<", 1)
				return strings.Replace(s, ">XXX<", ">Aug<", 1)
			},
			wantErr: ErrSchemaDrift,
		},
		{
			name:    "losing the space in '% age' loses the band anchor",
			mutate:  func(s string) string { return strings.ReplaceAll(s, ">% age<", ">%age<") },
			wantErr: ErrNoHeaderBand,
		},
		{
			name:    "a document with no table at all",
			mutate:  func(string) string { return "<html><body>Not Found</body></html>" },
			wantErr: ErrNoTable,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mutated := tc.mutate(base)
			if mutated == base {
				t.Fatal("mutation did not change the fixture; the test would prove nothing")
			}
			_, err := ParseWorkbook([]byte(mutated), "2023-24")
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// TestFiscalYearMismatchFailsTheParse: handing the parser the wrong year's
// file is a mistake worth erroring on, because every observation would be
// dated by the caller's assumption rather than the file's own band label.
func TestFiscalYearMismatchFailsTheParse(t *testing.T) {
	_, err := ParseWorkbook(fixture(t, trimFY2324), "2020-21")
	if !errors.Is(err, ErrFiscalYearMismatch) {
		t.Fatalf("err = %v, want %v", err, ErrFiscalYearMismatch)
	}
	// An empty fy takes the file's own word for it.
	w, err := ParseWorkbook(fixture(t, trimFY2324), "")
	if err != nil {
		t.Fatalf("ParseWorkbook with empty fy: %v", err)
	}
	if got := w.FiscalYear.Label(); got != "2023-24" {
		t.Errorf("FiscalYear = %s, want 2023-24", got)
	}
	// A malformed fy is rejected before any parsing work happens.
	if _, err := ParseWorkbook(fixture(t, trimFY2324), "twenty-twentythree"); !errors.Is(err, ErrFiscalYear) {
		t.Fatalf("err = %v, want %v", err, ErrFiscalYear)
	}
}

// TestVocabulariesAreStableAcrossYears asserts the 9 technologies and 16
// fuels really are one identical set in all three years — and that an
// unknown value is flagged, not rejected.
func TestVocabulariesAreStableAcrossYears(t *testing.T) {
	if len(KnownTechnologies) != 9 {
		t.Fatalf("KnownTechnologies has %d entries, want 9", len(KnownTechnologies))
	}
	if len(KnownFuels) != 16 {
		t.Fatalf("KnownFuels has %d entries, want 16", len(KnownFuels))
	}
	if !KnownTechnology("THERMAL- COAL") {
		t.Error(`"THERMAL- COAL" (space after the hyphen, as published) must be a known technology`)
	}
	if !KnownFuel("Natural Gas/ Furnace Oil") {
		t.Error(`"Natural Gas/ Furnace Oil" must be a known fuel`)
	}

	var firstTech, firstFuel []string
	for _, y := range fullYears {
		w := parse(t, y.name, y.fy)
		tech, fuel := w.Technologies(), w.Fuels()
		if len(tech) != 9 {
			t.Errorf("%s: %d distinct technologies, want 9: %q", y.fy, len(tech), tech)
		}
		if len(fuel) != 16 {
			t.Errorf("%s: %d distinct fuels, want 16: %q", y.fy, len(fuel), fuel)
		}
		for _, v := range tech {
			if !KnownTechnology(v) {
				t.Errorf("%s: technology %q is not in KnownTechnologies", y.fy, v)
			}
		}
		for _, v := range fuel {
			if !KnownFuel(v) {
				t.Errorf("%s: fuel %q is not in KnownFuels", y.fy, v)
			}
		}
		if firstTech == nil {
			firstTech, firstFuel = tech, fuel
			continue
		}
		if !reflect.DeepEqual(tech, firstTech) {
			t.Errorf("%s technologies differ from the first year:\n  %q\n  %q", y.fy, tech, firstTech)
		}
		if !reflect.DeepEqual(fuel, firstFuel) {
			t.Errorf("%s fuels differ from the first year:\n  %q\n  %q", y.fy, fuel, firstFuel)
		}
	}
}

// TestUnknownVocabularyIsFlaggedNotRejected: an unseen technology or fuel
// must survive the parse verbatim and raise a warning. Dropping the row, or
// erroring, would lose a real plant.
func TestUnknownVocabularyIsFlaggedNotRejected(t *testing.T) {
	mutated := strings.Replace(string(fixture(t, trimFY2324)), ">HYDEL-(WAPDA)<", ">TIDAL-(WAPDA)<", 1)
	w, err := ParseWorkbook([]byte(mutated), "2023-24")
	if err != nil {
		t.Fatalf("an unknown technology must not fail the parse: %v", err)
	}
	p, ok := w.PlantByName("Tarbela Hydropower Project (WAPDA)")
	if !ok {
		t.Fatal("Tarbela missing from the parse")
	}
	if p.Technology != "TIDAL-(WAPDA)" {
		t.Errorf("Technology = %q, want the unknown value preserved verbatim", p.Technology)
	}
	if p.TechnologyKnown {
		t.Error("TechnologyKnown = true, want false")
	}
	if !hasWarning(w, `unknown Technology "TIDAL-(WAPDA)"`) {
		t.Errorf("no warning names the unknown technology; warnings = %q", w.Warnings)
	}
}

func hasWarning(w *Workbook, substr string) bool {
	for _, warn := range w.Warnings {
		if strings.Contains(warn, substr) {
			return true
		}
	}
	return false
}
