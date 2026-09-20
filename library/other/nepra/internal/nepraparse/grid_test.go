package nepraparse

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func gridOf(t *testing.T, name string) *Grid {
	t.Helper()
	doc, _, _, err := Decode(fixture(t, name))
	if err != nil {
		t.Fatalf("Decode(%s): %v", name, err)
	}
	g, err := BuildGrid(doc)
	if err != nil {
		t.Fatalf("BuildGrid(%s): %v", name, err)
	}
	return g
}

// TestBuildGridStructure asserts the measured shape of each published file,
// including the two facts that break most table readers: exactly one <table>
// and ZERO <th>.
func TestBuildGridStructure(t *testing.T) {
	for _, y := range fullYears {
		t.Run(y.fy, func(t *testing.T) {
			g := gridOf(t, y.name)
			if g.Tables != 1 {
				t.Errorf("Tables = %d, want 1", g.Tables)
			}
			if g.HeaderCells != 0 {
				t.Errorf("HeaderCells (<th>) = %d, want 0 — header detection must be text-matched", g.HeaderCells)
			}
			if len(g.Rows) != y.tableRows {
				t.Errorf("rows = %d, want %d", len(g.Rows), y.tableRows)
			}
			if g.RawCells != y.rawCells {
				t.Errorf("raw <td> count = %d, want %d", g.RawCells, y.rawCells)
			}
			if g.Width != y.physicalWidth {
				t.Errorf("physical width = %d, want %d", g.Width, y.physicalWidth)
			}
			hist := map[int]int{}
			for _, n := range g.RawCellsPerRow {
				hist[n]++
			}
			if !reflect.DeepEqual(hist, y.rawCellHistogram) {
				t.Errorf("raw <td>-per-<tr> histogram = %v, want %v", hist, y.rawCellHistogram)
			}
			// Every expanded row is the same width: colspan and rowspan have
			// been resolved into a real rectangle.
			for i, r := range g.Rows {
				if len(r) != g.Width {
					t.Fatalf("row %d has width %d, want %d", i, len(r), g.Width)
				}
			}
		})
	}
}

// TestBuildGridExpandsSpans is the colspan/rowspan unit test, written against
// markup shaped exactly like the workbook's header band and its DELICENSED
// data rows.
func TestBuildGridExpandsSpans(t *testing.T) {
	tests := []struct {
		name string
		html string
		want [][]string
	}{
		{
			name: "rowspan carries the stub columns down the header band",
			html: `<table>
			  <tr><td rowspan=3>S.No</td><td rowspan=3>Name</td><td colspan=4>FY 2023-24</td></tr>
			  <tr><td colspan=2>Jul</td><td colspan=2>Aug</td></tr>
			  <tr><td>% age</td><td>GWh</td><td>% age</td><td>GWh</td></tr>
			</table>`,
			want: [][]string{
				{"S.No", "Name", "FY 2023-24", "FY 2023-24", "FY 2023-24", "FY 2023-24"},
				{"S.No", "Name", "Jul", "Jul", "Aug", "Aug"},
				{"S.No", "Name", "% age", "GWh", "% age", "GWh"},
			},
		},
		{
			name: "a status colspan on a data row fills its whole block",
			html: `<table><tr><td>14</td><td>Kotri</td><td>174</td><td colspan=4>DELICENSED</td><td></td></tr></table>`,
			want: [][]string{
				{"14", "Kotri", "174", "DELICENSED", "DELICENSED", "DELICENSED", "DELICENSED", ""},
			},
		},
		{
			name: "an EMPTY colspan still consumes its columns",
			html: `<table><tr><td>64</td><td>Reshma</td><td colspan=3>&nbsp;</td><td>tail</td></tr></table>`,
			want: [][]string{
				{"64", "Reshma", "", "", "", "tail"},
			},
		},
		{
			name: "markup inside a cell is removed without inserting a space",
			html: `<table><tr><td>Company Limit<span style='display:none'>ed.</span></td>` +
				`<td class=xl77><a href="x.pdf"><span style='font-size:16.0pt'>43.50</span></a></td></tr></table>`,
			want: [][]string{{"Company Limited.", "43.50"}},
		},
		{
			name: "source line wraps and mso spacerun runs collapse to one space",
			html: "<table><tr><td>Name of\n  Companies</td><td>Natural Gas/\n  Furnace Oil</td>" +
				"<td>Export\n  to K.Electric</td><td>Jamshoro Power\n  Generation<span style='mso-spacerun:yes'>  </span>Company Limited.</td></tr></table>",
			want: [][]string{{"Name of Companies", "Natural Gas/ Furnace Oil", "Export to K.Electric", "Jamshoro Power Generation Company Limited."}},
		},
		{
			name: "a br inside a cell is real whitespace",
			html: `<table><tr><td><br>Three Gorges First Wind Farm Pakistan (Pvt.) Ltd. (TGF)</td></tr></table>`,
			want: [][]string{{"Three Gorges First Wind Farm Pakistan (Pvt.) Ltd. (TGF)"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, err := BuildGrid(tc.html)
			if err != nil {
				t.Fatalf("BuildGrid: %v", err)
			}
			if !reflect.DeepEqual(g.Rows, tc.want) {
				t.Errorf("rows =\n  %q\nwant\n  %q", g.Rows, tc.want)
			}
		})
	}
}

// TestBuildGridRejectsEmptyInput keeps the no-table case an error rather than
// an empty, silently plausible workbook.
func TestBuildGridRejectsEmptyInput(t *testing.T) {
	if _, err := BuildGrid("<html><body><p>404 Not Found</p></body></html>"); !errors.Is(err, ErrNoTable) {
		t.Errorf("err = %v, want %v", err, ErrNoTable)
	}
}

// TestIgnoringColspanShiftsColumns demonstrates the corruption this package
// exists to prevent. It reads the FY2023-24 Kotri row the naive way — raw
// <td> elements in document order, no span expansion — and shows that
// "DELICENSED" lands in the Jul "% age" slot while the trailing padding
// cells slide up into Jul GWh..Oct % age, with nothing to catch it.
func TestIgnoringColspanShiftsColumns(t *testing.T) {
	doc, _, _, err := Decode(fixture(t, trimFY2324))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	rowHTML := rowContaining(t, doc, "Kotri Power Station")

	naive := naiveCells(rowHTML)
	if len(naive) != 14 {
		t.Fatalf("naive raw <td> count = %d, want 14 (6 stub + 1 colspan=26 + 7 padding)", len(naive))
	}
	if naive[ColFirstMonthPair+PctOffset] != "DELICENSED" {
		t.Fatalf("naive Jul %s = %q, want the DELICENSED text to land there", MetricPct, naive[ColFirstMonthPair+PctOffset])
	}
	// Naive column 7 should be Jul GWh. It is padding — and the row is now
	// 14 wide instead of 32, so 18 real columns have silently vanished.
	if naive[ColFirstMonthPair+GWhOffset] != "" {
		t.Fatalf("naive Jul %s = %q, want the shifted padding cell", MetricGWh, naive[ColFirstMonthPair+GWhOffset])
	}

	// The expanded grid puts DELICENSED in all 26 monthly cells and keeps
	// the row 32 columns wide.
	w := parse(t, trimFY2324, "2023-24")
	kotri, ok := w.PlantByName("Kotri Power Station")
	if !ok {
		t.Fatal("Kotri Power Station missing from the parse")
	}
	for _, obs := range append(kotri.Months[:], kotri.Total) {
		if obs.Utilisation.State() != StateDelicensed || obs.Generation.State() != StateDelicensed {
			t.Fatalf("%s: states = %v/%v, want both %v", obs.Label(),
				obs.Utilisation.State(), obs.Generation.State(), StateDelicensed)
		}
	}
}

// rowContaining returns the raw <tr> markup of the row holding needle.
func rowContaining(t *testing.T, doc, needle string) string {
	t.Helper()
	i := strings.Index(doc, needle)
	if i < 0 {
		t.Fatalf("%q not found in the document", needle)
	}
	start := strings.LastIndex(doc[:i], "<tr")
	end := strings.Index(doc[i:], "</tr>")
	if start < 0 || end < 0 {
		t.Fatalf("could not delimit the row containing %q", needle)
	}
	return doc[start : i+end+len("</tr>")]
}

// naiveCells is the wrong way to read a row, kept as a test fixture so the
// wrongness is demonstrated rather than described: it walks <td> elements in
// document order and ignores colspan entirely.
func naiveCells(rowHTML string) []string {
	var out []string
	rest := rowHTML
	for {
		i := strings.Index(rest, "<td")
		if i < 0 {
			return out
		}
		rest = rest[i:]
		gt := strings.Index(rest, ">")
		if gt < 0 {
			return out
		}
		body := rest[gt+1:]
		end := strings.Index(body, "</td>")
		if end < 0 {
			end = len(body)
		}
		out = append(out, collapseText(stripTags(body[:end])))
		rest = body[end:]
	}
}

func stripTags(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch {
		case r == '<':
			depth++
		case r == '>' && depth > 0:
			depth--
		case depth == 0:
			b.WriteRune(r)
		}
	}
	return strings.ReplaceAll(b.String(), "&nbsp;", " ")
}
