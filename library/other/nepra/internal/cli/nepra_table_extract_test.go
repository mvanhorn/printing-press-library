// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.

package cli

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fixtures are the four Excel-exported data sheets verbatim, gzipped only
// so the repository does not carry ~215 KB of blobs. Decompressed they are
// byte-identical to what NEPRA serves, which is what lets these tests assert
// measured whole-file numbers rather than approximations.
func sheetFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "sheet-"+name+".htm.gz"))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("gzip %s: %v", name, err)
	}
	defer func() { _ = zr.Close() }()
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("decompress %s: %v", name, err)
	}
	return out
}

func extractRows(t *testing.T, name string) []map[string]string {
	t.Helper()
	out, err := extractNepraTable(sheetFixture(t, name))
	if err != nil {
		t.Fatalf("extractNepraTable(%s): %v", name, err)
	}
	var rows []map[string]string
	if err := json.Unmarshal(out, &rows); err != nil {
		t.Fatalf("unmarshalling %s rows: %v", name, err)
	}
	return rows
}

// TestExtractNepraTableShapes pins each sheet's measured shape. The generated
// html_extract has no table mode at all — it walks the DOM for <title>,
// <meta>, canonical <link> and <a> only — so before this extractor every one
// of these surfaces returned its hyperlinks and none of its figures.
func TestExtractNepraTableShapes(t *testing.T) {
	for _, tc := range []struct {
		name    string
		rows    int
		columns []string
	}{
		{
			name: "hydel", rows: 30,
			columns: []string{"Sr #", "Company Name", "Dependable Capacity (MW)", "Period"},
		},
		{
			// A two-level header stacked with rowspan: "CPPA - G" over
			// "FCA Requested" composes into one column name.
			name: "fca", rows: 48,
			columns: []string{"Year", "Month", "CPPA - G FCA Requested", "CPPA - G FCA Allowed (kWh)"},
		},
		{
			name: "sro", rows: 19,
			columns: []string{"Year", "SRO Date", "FESCO SRO Number", "LESCO SRO Number"},
		},
		{
			// The sheet that refutes a "leading rows with no numbers" rule:
			// its data rows hold only dates and "Not Yet Issued".
			name: "quarterly", rows: 163,
			columns: []string{"Quarter", "DISCOs Name", "Quarterly Adjustments (XWAPDA DISCOs) Requested By Company"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := extractRows(t, tc.name)
			if len(rows) != tc.rows {
				t.Errorf("rows = %d, want %d", len(rows), tc.rows)
			}
			if len(rows) == 0 {
				t.Fatal("no rows extracted")
			}
			for _, col := range tc.columns {
				if _, ok := rows[0][col]; !ok {
					var have []string
					for k := range rows[0] {
						have = append(have, k)
					}
					t.Errorf("column %q missing; have %q", col, have)
				}
			}
			// No column may be named positionally: that means the header band
			// was mis-detected and a label was lost.
			for k := range rows[0] {
				if strings.HasPrefix(k, "column_") {
					t.Errorf("column %q was named positionally, so its label was not found", k)
				}
			}
		})
	}
}

// TestFCACumulativeDisallowance is the strongest available check on the
// extractor: it recomputes, from the extracted rows alone, the figure the
// absorb manifest says no one had ever computed — and it must reproduce every
// digit. Getting the header band, the colspan expansion, the windows-1252
// decode or the accounting negatives wrong all move these numbers.
func TestFCACumulativeDisallowance(t *testing.T) {
	rows := extractRows(t, "fca")
	if len(rows) != 48 {
		t.Fatalf("rows = %d, want 48 gap-free months", len(rows))
	}

	var reqSum, alwSum float64
	var pairs, below, parenNegatives int
	seen := map[string]bool{}
	for _, r := range rows {
		seen[r["Year"]+"-"+r["Month"]] = true
		for _, v := range r {
			if isAccountingNegative(v) {
				parenNegatives++
			}
		}
		// THE SHIPPED PARSER, not a second copy of it. This test computes
		// the headline cumulative figures, so it must exercise the same code
		// the command does: a private duplicate here returned (0,false) for
		// an unparseable cell where fcaParseFigure returns an ERROR, which
		// meant this test could keep reproducing 81.9748 from a sheet the
		// shipped command had already refused.
		req, rok, rerr := fcaParseFigure(r["CPPA - G FCA Requested"])
		alw, aok, aerr := fcaParseFigure(r["CPPA - G FCA Allowed (kWh)"])
		if rerr != nil || aerr != nil {
			t.Fatalf("a published FCA cell did not parse: requested=%q (%v) allowed=%q (%v)",
				r["CPPA - G FCA Requested"], rerr, r["CPPA - G FCA Allowed (kWh)"], aerr)
		}
		if !rok || !aok {
			continue
		}
		pairs++
		reqSum += req
		alwSum += alw
		if alw < req {
			below++
		}
	}

	if len(seen) != 48 {
		t.Errorf("distinct year-month pairs = %d, want 48", len(seen))
	}
	if pairs != 48 {
		t.Errorf("months with both CPPA-G figures = %d, want 48", pairs)
	}
	// Measured against the published sheet, to four decimals as published.
	for _, c := range []struct {
		label string
		got   float64
		want  float64
	}{
		{"cumulative requested", reqSum, 81.9748},
		{"cumulative allowed", alwSum, 73.1323},
		{"cumulative disallowance", reqSum - alwSum, 8.8425},
	} {
		if math.Abs(c.got-c.want) > 5e-4 {
			t.Errorf("%s = %.4f, want %.4f", c.label, c.got, c.want)
		}
	}
	if below != 45 {
		t.Errorf("months where allowed < requested = %d, want 45", below)
	}
	// The accounting parentheses are the trap that makes float() throw or
	// silently sign-flip, so their count is pinned too.
	if parenNegatives != 49 {
		t.Errorf("accounting-parenthesised negatives = %d, want 49", parenNegatives)
	}
}

// TestExtractNepraTableRejectsNonTables checks the extractor fails loudly on a
// document with no table rather than returning an empty success — the failure
// shape that hid the frameset decoys and the gzip bug.
func TestExtractNepraTableRejectsNonTables(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"a frameset wrapper", `<html><head><meta name="Excel Workbook Frameset"></head><frameset><frame src="x_files/sheet001.htm"></frameset></html>`},
		{"a 404 stub", `Not Found`},
		{"prose with no table", `<html><body><p>no data here</p></body></html>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := extractNepraTable([]byte(tc.body))
			if err != nil {
				return // a named error is the correct outcome
			}
			var rows []map[string]string
			if uerr := json.Unmarshal(out, &rows); uerr != nil {
				t.Fatalf("unmarshalling: %v", uerr)
			}
			if len(rows) != 0 {
				t.Errorf("a document with no data table produced %d rows", len(rows))
			}
		})
	}
}

// TestComposeColumnsNeverSilentlyCollides guards the naming: two columns that
// compose to one name must be disambiguated, never allowed to overwrite each
// other in the row object. The real SRO sheet has two TESCO columns.
func TestComposeColumnsNeverSilentlyCollides(t *testing.T) {
	got := composeColumns([][]string{{"A", "A", "B"}, {"x", "x", "y"}}, 3)
	if len(got) != 3 {
		t.Fatalf("got %d names, want 3", len(got))
	}
	if got[0] == got[1] {
		t.Errorf("two columns share the name %q; one would overwrite the other", got[0])
	}
	// A blank header column is named positionally rather than left empty.
	blank := composeColumns([][]string{{"", "A"}}, 2)
	if blank[0] == "" {
		t.Error("a blank header column produced an empty name")
	}

	// And the real sheet keeps both TESCO columns.
	rows := extractRows(t, "sro")
	var tesco int
	for k := range rows[0] {
		if strings.HasPrefix(k, "TESCO") {
			tesco++
		}
	}
	if tesco != 2 {
		t.Errorf("TESCO columns in the SRO sheet = %d, want 2 (the sheet has two)", tesco)
	}
}

func isAccountingNegative(v string) bool {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "(") || !strings.HasSuffix(v, ")") {
		return false
	}
	_, ok, err := fcaParseFigure(v)
	return ok && err == nil
}
