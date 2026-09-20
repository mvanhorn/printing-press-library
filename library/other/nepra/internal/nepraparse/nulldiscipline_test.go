package nepraparse

import (
	"errors"
	"strings"
	"testing"
)

// TestEmptyVocabularyIsWarnedAbout is finding D. Technology and Fuel are
// plain strings, so a blank cell, an absent column and a plant NEPRA never
// classified all arrive as "". The warning guard used to read
// `p.Technology != "" && !p.TechnologyKnown`, which meant the one state
// carrying NO information was the only one that passed in silence.
func TestEmptyVocabularyIsWarnedAbout(t *testing.T) {
	tests := []struct {
		name       string
		technology string
		fuel       string
		wantSubstr []string
	}{
		{
			name: "empty technology", technology: "", fuel: "RFO",
			wantSubstr: []string{"EMPTY Technology"},
		},
		{
			name: "empty fuel", technology: "THERMAL", fuel: "",
			wantSubstr: []string{"EMPTY Fuel"},
		},
		{
			name: "both empty", technology: "", fuel: "",
			wantSubstr: []string{"EMPTY Technology", "EMPTY Fuel"},
		},
		{
			name: "unknown but non-empty still warns about the value", technology: "FUSION", fuel: "DEUTERIUM",
			wantSubstr: []string{`unknown Technology "FUSION"`, `unknown Fuel "DEUTERIUM"`},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// The Known flags are computed by buildPlant, so a hand-built
			// Plant must compute them the same way or it warns spuriously.
			p := Plant{
				Name: "Test Plant", SNo: 1,
				Technology: tc.technology, TechnologyKnown: KnownTechnology(tc.technology),
				Fuel: tc.fuel, FuelKnown: KnownFuel(tc.fuel),
			}
			warnings := plantWarnings([]Plant{p})
			joined := strings.Join(warnings, "\n")
			for _, want := range tc.wantSubstr {
				if !strings.Contains(joined, want) {
					t.Errorf("no warning containing %q; warnings:\n%s", want, joined)
				}
			}
		})
	}

	// A fully valid plant must not warn, or the signal is worthless.
	ok := Plant{
		Name: "Test Plant", SNo: 1,
		Technology: "THERMAL", TechnologyKnown: KnownTechnology("THERMAL"),
		Fuel: "RFO", FuelKnown: KnownFuel("RFO"),
	}
	if !ok.TechnologyKnown || !ok.FuelKnown {
		t.Fatal("THERMAL/RFO are not in the known vocabulary; this test's premise is wrong")
	}
	if w := plantWarnings([]Plant{ok}); len(w) != 0 {
		t.Errorf("a valid plant produced warnings: %q", w)
	}

	// And the real workbooks must stay clean, which is what makes the
	// warning meaningful: no sampled year has an empty vocabulary cell.
	for _, y := range fullYears {
		w := parse(t, y.name, y.fy)
		for _, warn := range w.Warnings {
			if strings.Contains(warn, "EMPTY Technology") || strings.Contains(warn, "EMPTY Fuel") {
				t.Errorf("FY%s: %s", y.fy, warn)
			}
		}
	}
}

// TestWhitespaceOnlyFiscalYearIsRefused is finding E. An empty fy argument
// deliberately skips the header cross-check; a whitespace-only one used to
// collapse to empty and skip it too — silently disabling the only guard
// against dating an entire workbook to the wrong fiscal year. The likely
// source of such an argument is an NBSP-padded cell passed straight through,
// which makes it a caller bug rather than a choice.
func TestWhitespaceOnlyFiscalYearIsRefused(t *testing.T) {
	data := fixture(t, trimFY2324)

	// The deliberate skip still works.
	if _, err := ParseWorkbook(data, ""); err != nil {
		t.Errorf(`ParseWorkbook(data, "") = %v; an empty fy must skip the cross-check, not fail`, err)
	}
	// The correct year still works.
	if _, err := ParseWorkbook(data, "2023-24"); err != nil {
		t.Errorf("ParseWorkbook(data, 2023-24) = %v", err)
	}
	// A wrong year is still caught.
	if _, err := ParseWorkbook(data, "2017-18"); !errors.Is(err, ErrFiscalYearMismatch) {
		t.Errorf("ParseWorkbook(data, 2017-18) err = %v, want ErrFiscalYearMismatch", err)
	}
	// Whitespace-only must now be refused rather than silently skipped.
	for _, fy := range []string{" ", "   ", "\t", "\n  ", " ", "  \n "} {
		if _, err := ParseWorkbook(data, fy); err == nil {
			t.Errorf("ParseWorkbook(data, %q) succeeded; a whitespace-only fy silently disables the cross-check", fy)
		}
	}
}

// TestSpanCollisionKeepsThePublishedCell is finding E's grid half. A colspan
// running into a slot a rowspan already owns is a malformed table; the old
// code overwrote the carried-down value, discarding a cell the document
// published in favour of one merely spanning over it.
func TestSpanCollisionKeepsThePublishedCell(t *testing.T) {
	// Row 1 puts KEEP-ME into column 1 and carries it down one row.
	// Row 2's colspan=3 cell starts at column 0 and would run over it.
	doc := `<table>
	  <tr><td>a</td><td rowspan=2>KEEP-ME</td><td>c</td></tr>
	  <tr><td colspan=3>WIDE</td></tr>
	</table>`
	g, err := BuildGrid(doc)
	if err != nil {
		t.Fatalf("BuildGrid: %v", err)
	}
	if len(g.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(g.Rows))
	}
	if g.Rows[0][1] != "KEEP-ME" {
		t.Fatalf("row 0 col 1 = %q, want KEEP-ME", g.Rows[0][1])
	}
	// The rowspan's carried value must survive the colspan.
	if got := g.Rows[1][1]; got != "KEEP-ME" {
		t.Errorf("row 1 col 1 = %q, want KEEP-ME: a colspan overwrote a rowspan-owned cell, discarding published data", got)
	}
	// And the malformation must be observable rather than silent.
	if g.SpanCollisions == 0 {
		t.Error("SpanCollisions = 0; an overlapping table must be reported, not quietly accepted")
	}

	// A well-formed table must report no collisions, or the counter is noise.
	clean := `<table>
	  <tr><td>a</td><td rowspan=2>b</td><td>c</td></tr>
	  <tr><td>d</td><td>e</td></tr>
	</table>`
	cg, err := BuildGrid(clean)
	if err != nil {
		t.Fatalf("BuildGrid(clean): %v", err)
	}
	if cg.SpanCollisions != 0 {
		t.Errorf("well-formed table reported %d span collisions", cg.SpanCollisions)
	}
	if cg.Rows[1][1] != "b" {
		t.Errorf("clean row 1 col 1 = %q, want the carried-down b", cg.Rows[1][1])
	}

	// The real workbooks must have none, which is what makes this latent
	// rather than active: the 13 FY2023-24 colspan=26 status rows do not
	// collide with anything.
	for _, y := range fullYears {
		rg, err := BuildGrid(mustDecode(t, fixture(t, y.name)))
		if err != nil {
			t.Fatalf("BuildGrid(%s): %v", y.fy, err)
		}
		if rg.SpanCollisions != 0 {
			t.Errorf("FY%s has %d span collisions; the column mapping must be re-derived", y.fy, rg.SpanCollisions)
		}
	}
}

func mustDecode(t *testing.T, data []byte) string {
	t.Helper()
	doc, _, _, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	return doc
}
