// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// See .printing-press-patches/nepra-html-table-extraction.json.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// extractNepraTable turns one of NEPRA's Excel "Save as Web Page" data sheets
// into rows.
//
// It exists because the generated html_extract supports only `page` and
// `links`, which walk the DOM for <title>, <meta>, canonical <link> and <a>
// hrefs. Neither returns a table. Four of this CLI's surfaces ARE tables —
// FCA (53 rows / 572 cells / 48 gap-free months), quarterly (168 / 582), SRO
// (23 / 245) and hydel (33 / 126) — so their numbers were unreachable while
// their hyperlinks came back fine.
//
// The parsing is nepraparse's, not a second implementation: Decode handles the
// windows-1252 payload whose HTTP header declares no charset, and BuildGrid
// expands colspan/rowspan into a dense rectangle. That is the same machinery
// the generation workbook goes through, because these files are siblings from
// the same directory and the same Excel 15 exporter.
func extractNepraTable(raw []byte) (json.RawMessage, error) {
	doc, charset, _, err := nepraparse.Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("decoding table response: %w", err)
	}
	g, err := nepraparse.BuildGrid(doc)
	if err != nil {
		return nil, fmt.Errorf("building table grid: %w", err)
	}
	tbl := shapeGrid(g)
	tbl.Charset = charset

	rows := make([]map[string]any, 0, len(tbl.Data))
	for _, r := range tbl.Data {
		obj := make(map[string]any, len(tbl.Columns))
		for i, name := range tbl.Columns {
			v := ""
			if i < len(r) {
				v = r[i]
			}
			obj[name] = v
		}
		rows = append(rows, obj)
	}
	out, err := json.Marshal(rows)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// shapedTable is the interpreted view of a grid.
type shapedTable struct {
	Charset    string
	Title      string
	Columns    []string
	HeaderRows int
	Data       [][]string
}

// shapeGrid finds the title, the header band and the data rows.
//
// The header band is found from the ROWSPAN SIGNATURE that Excel's export
// leaves behind, not from a content guess.
//
// When a sheet stacks a two-level header, the left-hand label cells carry
// rowspan=2 and BuildGrid expands them by repeating the value down the band.
// So consecutive header rows share identical values at identical column
// indexes, and that repetition stops exactly where the data begins. The band
// is therefore the run of leading rows that share a cell with their successor,
// plus one final label row.
//
// MEASURED against all four sheets, which is why the rule is this and not the
// obvious "rows with no numbers":
//
//	fca        rows 2-3 share "Year"/"Month"          -> 2 header rows
//	sro        rows 1-2 share "Year"/"SRO Date"       -> 2 header rows
//	quarterly  rows 1-2 share "Quarter"/"DISCOs Name" -> 2 header rows
//	hydel      row 1 shares nothing with row 2        -> 1 header row
//
// The numeric heuristic gets quarterly WRONG: its data rows hold only dates
// and the words "Not Yet Issued", so a "leading rows with no numbers" rule
// swallowed 161 data rows into the header and produced a column literally
// named "DISCOs Name FESCO GEPCO". A numeric cell still HARD-STOPS the band,
// because no sheet here labels a column with a figure.
func shapeGrid(g *nepraparse.Grid) shapedTable {
	rows := trimGrid(g.Rows)
	var out shapedTable

	// A row whose non-empty cells are all the same string is a colspanned
	// title, not data.
	for len(rows) > 0 && spannedTitle(rows[0]) != "" {
		if out.Title == "" {
			out.Title = spannedTitle(rows[0])
		}
		rows = rows[1:]
	}

	// maxHeaderRows bounds the band so a malformed sheet cannot consume its
	// own data as labels.
	const maxHeaderRows = 4
	header := 0
	for header < len(rows)-1 && header < maxHeaderRows-1 {
		if hasNumericCell(rows[header]) || !sharesCell(rows[header], rows[header+1]) {
			break
		}
		header++
	}
	// One more row closes the band: it is the innermost label row, the one
	// whose repetition with its successor has just stopped.
	if header < len(rows) && !hasNumericCell(rows[header]) {
		header++
	}
	out.HeaderRows = header
	out.Columns = composeColumns(rows[:header], gridWidth(rows))
	out.Data = rows[header:]
	return out
}

// sharesCell reports whether two rows carry the same non-empty value at the
// same column index — the fingerprint of a rowspan-stacked header.
func sharesCell(a, b []string) bool {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if v := strings.TrimSpace(a[i]); v != "" && v == strings.TrimSpace(b[i]) {
			return true
		}
	}
	return false
}

// composeColumns joins the header band top-down into one name per column.
// Repeated values produced by a colspan are collapsed, so "CPPA - G" over
// "FCA Requested" becomes "CPPA - G FCA Requested" rather than repeating the
// group label. A column the header band leaves blank is named positionally,
// and a duplicate name is suffixed rather than allowed to overwrite a sibling.
func composeColumns(header [][]string, width int) []string {
	names := make([]string, width)
	for c := 0; c < width; c++ {
		var parts []string
		for _, hr := range header {
			if c >= len(hr) {
				continue
			}
			v := strings.TrimSpace(hr[c])
			if v == "" {
				continue
			}
			if len(parts) > 0 && parts[len(parts)-1] == v {
				continue
			}
			parts = append(parts, v)
		}
		names[c] = strings.Join(parts, " ")
	}
	seen := map[string]int{}
	for c, n := range names {
		if n == "" {
			n = fmt.Sprintf("column_%d", c+1)
		}
		seen[n]++
		if k := seen[n]; k > 1 {
			n = fmt.Sprintf("%s (%d)", n, k)
		}
		names[c] = n
	}
	return names
}

// trimGrid drops rows that are entirely empty and columns that are entirely
// empty, which Excel's export adds around the real table.
func trimGrid(in [][]string) [][]string {
	var kept [][]string
	for _, r := range in {
		if !allEmpty(r) {
			kept = append(kept, r)
		}
	}
	if len(kept) == 0 {
		return nil
	}
	width := gridWidth(kept)
	first, last := width, -1
	for c := 0; c < width; c++ {
		for _, r := range kept {
			if c < len(r) && strings.TrimSpace(r[c]) != "" {
				if c < first {
					first = c
				}
				if c > last {
					last = c
				}
				break
			}
		}
	}
	if last < first {
		return nil
	}
	out := make([][]string, 0, len(kept))
	for _, r := range kept {
		row := make([]string, 0, last-first+1)
		for c := first; c <= last; c++ {
			v := ""
			if c < len(r) {
				v = strings.TrimSpace(r[c])
			}
			row = append(row, v)
		}
		out = append(out, row)
	}
	return out
}

func gridWidth(rows [][]string) int {
	w := 0
	for _, r := range rows {
		if len(r) > w {
			w = len(r)
		}
	}
	return w
}

func allEmpty(r []string) bool {
	for _, v := range r {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

// spannedTitle returns the single value a row carries across all its non-empty
// cells, or "" when the row holds more than one distinct value. A colspanned
// title expands to the same string in every covered cell.
func spannedTitle(r []string) string {
	var val string
	n := 0
	for _, v := range r {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if val == "" {
			val = v
		} else if v != val {
			return ""
		}
		n++
	}
	if n < 2 {
		// One lone cell is not evidence of a spanned title.
		return ""
	}
	return val
}

// hasNumericCell reports whether any cell in the row is a published number.
// It uses nepraparse's classifier so that a status sentinel, an NBSP blank and
// a real 0.00 are judged exactly as they are in the generation workbook.
func hasNumericCell(r []string) bool {
	for _, v := range r {
		if nepraparse.ParseValue(v).Present() {
			return true
		}
	}
	return false
}
