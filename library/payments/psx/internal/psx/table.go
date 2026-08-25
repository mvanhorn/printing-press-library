// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// Package psx provides a header-name-driven HTML table parser and a rate-limited
// HTTP client for the Pakistan Stock Exchange data portal.
//
// Header-name-driven parsing is deliberate. PSX reorders table columns without
// notice, so any parser keyed on cell position (cells[7], cells[10]) breaks
// silently and returns wrong-but-plausible numbers. Every row this package emits
// is keyed by the normalized text of its own <th>, so a column reorder is a
// no-op and a column rename surfaces as a missing key rather than bad data.
package psx

import (
	"strings"

	"golang.org/x/net/html"
)

// Table is one parsed HTML table: its normalized header names in document
// order, plus one map per body row keyed by those names.
type Table struct {
	ID      string              `json:"id,omitempty"`
	Caption string              `json:"caption,omitempty"`
	Headers []string            `json:"headers"`
	Rows    []map[string]string `json:"rows"`
}

// NormalizeHeader folds a raw <th> label into a stable snake_case key.
// "CHANGE (%)" -> "change_pct", "MARKET CAP. (B)" -> "market_cap_b".
func NormalizeHeader(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, "%", " pct ")
	var b strings.Builder
	prevUnderscore := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevUnderscore = false
		default:
			if !prevUnderscore && b.Len() > 0 {
				b.WriteByte('_')
				prevUnderscore = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

// ParseTables extracts every <table> in the document. PSX AJAX fragments often
// contain several (e.g. /performers returns active, gainers and losers).
func ParseTables(doc string) ([]Table, error) {
	root, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		return nil, err
	}
	out := make([]Table, 0)
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "table" {
			if t, ok := parseTable(n); ok {
				out = append(out, t)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out, nil
}

// FindTable returns the first table whose id matches, or whose header set
// contains every name in mustHave. Empty id and nil mustHave return the first
// table with any rows.
func FindTable(tables []Table, id string, mustHave ...string) (Table, bool) {
	for _, t := range tables {
		if id != "" && t.ID != id {
			continue
		}
		if !t.hasAll(mustHave) {
			continue
		}
		if len(t.Rows) == 0 {
			continue
		}
		return t, true
	}
	return Table{}, false
}

func (t Table) hasAll(names []string) bool {
	for _, want := range names {
		found := false
		for _, h := range t.Headers {
			if h == want {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// text collects the visible text of a node, collapsing whitespace and
// unescaping HTML entities exactly once via cliutil.CleanText.
//
// Known limitation (deferred, tracked in polish output review): this keeps only
// text nodes, so a cell holding a link contributes its anchor text ("View PDF")
// and the href is dropped — announcement filings are therefore not addressable
// from the output. Capturing hrefs into a companion column changes the row
// schema for every table-backed command and the synced store, so it belongs in
// a generator change rather than a hand-edit here. The same applies to lifting
// an upstream-embedded "as of" timestamp out of a cell (one /indices row
// renders as "HBLTTI (18-08-2026 18:30:00)"): stripping it generically would
// mangle legitimate parenthesised values in other tables.
func text(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
			b.WriteString(" ")
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	// golang.org/x/net/html already decodes entities into text nodes. Running
	// CleanText here would unescape a second time, corrupting any filing title
	// that legitimately contains a literal "&amp;" or "&lt;".
	return strings.Join(strings.Fields(b.String()), " ")
}

func childElems(n *html.Node, names ...string) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode {
				for _, want := range names {
					if c.Data == want {
						out = append(out, c)
					}
				}
				// Do not descend into a nested table; its rows belong to it.
				if c.Data == "table" {
					continue
				}
				walk(c)
			}
		}
	}
	walk(n)
	return out
}

func parseTable(tbl *html.Node) (Table, bool) {
	t := Table{ID: attr(tbl, "id"), Rows: make([]map[string]string, 0)}

	rows := childElems(tbl, "tr")
	if len(rows) == 0 {
		return t, false
	}

	// Header row: the first <tr> that contains any <th>. PSX sometimes emits
	// grouping <th colspan> rows above the real header, so prefer the LAST
	// all-<th> row before the first data row.
	headerIdx := -1
	for i, tr := range rows {
		ths := childElems(tr, "th")
		tds := childElems(tr, "td")
		if len(ths) > 0 && len(tds) == 0 {
			headerIdx = i
			continue
		}
		if len(tds) > 0 {
			break
		}
	}
	if headerIdx < 0 {
		return t, false
	}

	// Header rows may span columns and rows. PSX renders bid/ask depth as a
	// two-level header (<th rowspan=2>SYMBOL</th><th colspan=2>BID</th>...),
	// so a naive one-th-per-column read shifts every subsequent cell and
	// produces exactly the wrong-but-plausible numbers header-keying exists to
	// prevent. Expand colspan into repeated names and merge the parent level
	// into the sub level so the count matches the data row.
	headerRows := rows[:headerIdx+1]
	names := expandHeaderRows(headerRows)
	seen := map[string]int{}
	for _, raw := range names {
		name := NormalizeHeader(raw)
		if name == "" {
			name = "col"
		}
		if n, dup := seen[name]; dup {
			seen[name] = n + 1
			name = name + "_" + itoa(n+1)
		} else {
			seen[name] = 1
		}
		t.Headers = append(t.Headers, name)
	}
	if len(t.Headers) == 0 {
		return t, false
	}

	for _, tr := range rows[headerIdx+1:] {
		cells := childElems(tr, "td")
		if len(cells) == 0 {
			continue
		}
		row := make(map[string]string, len(t.Headers))
		for i, td := range cells {
			key := ""
			if i < len(t.Headers) {
				key = t.Headers[i]
			} else {
				// More cells than headers: keep the value under a positional
				// key rather than dropping it, so a layout change is visible
				// in the output instead of silently losing a column.
				key = "col_" + itoa(i+1)
			}
			row[key] = text(td)
		}
		if len(row) > 0 {
			t.Rows = append(t.Rows, row)
		}
	}
	return t, true
}

// expandHeaderRows flattens a possibly multi-row header into one name per
// data column, honouring colspan and rowspan. A cell with rowspan>1 occupies
// the same column in later rows; a cell with colspan>1 repeats its label,
// qualified by the sub-header beneath it when one exists.
func expandHeaderRows(headerRows []*html.Node) []string {
	if len(headerRows) == 0 {
		return nil
	}
	// grid[r][c] holds the label occupying that cell.
	grid := map[int]map[int]string{}
	occupied := map[int]map[int]bool{}
	for r := range headerRows {
		grid[r] = map[int]string{}
		occupied[r] = map[int]bool{}
	}
	for r, tr := range headerRows {
		col := 0
		for _, th := range childElems(tr, "th") {
			for occupied[r][col] {
				col++
			}
			label := text(th)
			cs := spanOf(th, "colspan")
			rs := spanOf(th, "rowspan")
			for dc := 0; dc < cs; dc++ {
				for dr := 0; dr < rs; dr++ {
					rr := r + dr
					if _, ok := grid[rr]; !ok {
						continue
					}
					if existing := grid[rr][col+dc]; existing != "" && existing != label {
						grid[rr][col+dc] = existing + " " + label
					} else {
						grid[rr][col+dc] = label
					}
					occupied[rr][col+dc] = true
				}
			}
			col += cs
		}
	}
	// The bottom header row defines the column count; join each column's
	// stack of labels top-to-bottom so "BID"+"PRICE" becomes "BID PRICE".
	last := len(headerRows) - 1
	width := 0
	for c := range grid[last] {
		if c+1 > width {
			width = c + 1
		}
	}
	out := make([]string, 0, width)
	for c := 0; c < width; c++ {
		parts := make([]string, 0, len(headerRows))
		for r := 0; r <= last; r++ {
			if v := strings.TrimSpace(grid[r][c]); v != "" {
				if len(parts) == 0 || parts[len(parts)-1] != v {
					parts = append(parts, v)
				}
			}
		}
		out = append(out, strings.Join(parts, " "))
	}
	return out
}

// spanOf reads a colspan/rowspan attribute, defaulting to 1.
func spanOf(n *html.Node, key string) int {
	v := strings.TrimSpace(attr(n, key))
	if v == "" {
		return 1
	}
	i := 0
	for _, r := range v {
		if r < '0' || r > '9' {
			return 1
		}
		i = i*10 + int(r-'0')
		if i > 64 {
			return 64
		}
	}
	if i < 1 {
		return 1
	}
	return i
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
