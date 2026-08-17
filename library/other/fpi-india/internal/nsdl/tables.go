// Package nsdl parses NSDL's server-rendered ASP.NET GridView report pages
// into structured records. NSDL/CDSL have no API; every historical FPI
// figure is embedded directly in an HTML <table> with rowspan/colspan
// grouped headers (e.g. a "Mutual Funds" group spanning five sub-columns).
package nsdl

import (
	"strings"

	xhtml "golang.org/x/net/html"
)

// Table is a parsed HTML table: composite leaf column names (built by
// expanding rowspan/colspan and prefixing collision-prone leaf headers with
// their parent group name) plus the data rows below the header block.
type Table struct {
	Columns []string
	Rows    [][]string
}

// gridCell is one cell of the fully-expanded virtual grid (every rowspan/
// colspan occupies its true footprint, exactly like a browser's rendered
// table layout).
type gridCell struct {
	text     string
	isHeader bool
}

// ExtractLargestTable finds the <table> with the most <tr> elements in the
// document and parses it. NSDL's report pages carry many small layout/
// navigation tables; the real GridView data grid is reliably the largest by
// row count across every report family observed (Yearwise, AUC, sector,
// trades, registry).
func ExtractLargestTable(body []byte) (*Table, error) {
	doc, err := xhtml.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	var best *xhtml.Node
	bestRows := 0
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && n.Data == "table" {
			rows := countRows(n)
			if rows > bestRows {
				bestRows = rows
				best = n
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if best == nil {
		return &Table{}, nil
	}
	return parseTableNode(best), nil
}

// ExtractTableByID finds a <table id="..."> and parses it directly, for
// pages where the target table has a stable, known id (e.g. NSDL's
// Yearwise.aspx uses id="rpt" consistently).
func ExtractTableByID(body []byte, id string) (*Table, error) {
	doc, err := xhtml.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	var found *xhtml.Node
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if found != nil {
			return
		}
		if n.Type == xhtml.ElementNode && n.Data == "table" && attr(n, "id") == id {
			found = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if found == nil {
		return ExtractLargestTable(body)
	}
	return parseTableNode(found), nil
}

// countRows counts only <tr> elements directly owned by this table, not
// rows belonging to a nested <table> inside one of its cells. Legacy
// layout-table-heavy pages (NSDL's pre-2010 StaticReports .htm files) nest
// tables inside table cells for visual layout; without this boundary, an
// outer wrapper table's row count balloons with its nested data table's
// rows, which both corrupts row/column counts for the wrapper and makes the
// "largest table" heuristic pick the wrong node.
func countRows(table *xhtml.Node) int {
	n := 0
	var walk func(*xhtml.Node, bool)
	walk = func(node *xhtml.Node, isTableRoot bool) {
		if node.Type == xhtml.ElementNode && node.Data == "table" && !isTableRoot {
			return // stop at a nested table's boundary
		}
		if node.Type == xhtml.ElementNode && node.Data == "tr" {
			n++
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c, false)
		}
	}
	walk(table, true)
	return n
}

func attr(n *xhtml.Node, name string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, name) {
			return a.Val
		}
	}
	return ""
}

// maxSpanValue caps rowspan/colspan values from fetched HTML. NSDL/CDSL
// pages are legacy static reports with no cell spanning this large; a
// malformed or corrupted page advertising e.g. rowspan="99999999" would
// otherwise drive the grid-expansion loop in parseTableNode into
// unbounded O(rowspan*colspan) allocations.
const maxSpanValue = 1000

func attrInt(n *xhtml.Node, name string, def int) int {
	v := attr(n, name)
	if v == "" {
		return def
	}
	out := 0
	for _, r := range v {
		if r < '0' || r > '9' {
			return def
		}
		out = out*10 + int(r-'0')
		if out > maxSpanValue {
			return maxSpanValue
		}
	}
	if out == 0 {
		return def
	}
	return out
}

func nodeText(n *xhtml.Node) string {
	var sb strings.Builder
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node.Type == xhtml.TextNode {
			sb.WriteString(node.Data)
		}
		if node.Type == xhtml.ElementNode && (node.Data == "script" || node.Data == "style") {
			return
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(sb.String()), " ")
}

// parseTableNode expands rowspan/colspan into a dense virtual grid, then
// splits the grid into a header block (leading rows made entirely of <th>
// cells) and a data block (every row after that).
func parseTableNode(table *xhtml.Node) *Table {
	type rawCell struct {
		text     string
		isHeader bool
		rowspan  int
		colspan  int
	}
	var rawRows [][]rawCell
	var walkRows func(*xhtml.Node)
	walkRows = func(n *xhtml.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == xhtml.ElementNode && c.Data == "tr" {
				var row []rawCell
				for cc := c.FirstChild; cc != nil; cc = cc.NextSibling {
					if cc.Type == xhtml.ElementNode && (cc.Data == "td" || cc.Data == "th") {
						row = append(row, rawCell{
							text:     nodeText(cc),
							isHeader: cc.Data == "th",
							rowspan:  attrInt(cc, "rowspan", 1),
							colspan:  attrInt(cc, "colspan", 1),
						})
					}
				}
				if len(row) > 0 {
					rawRows = append(rawRows, row)
				}
			} else {
				walkRows(c)
			}
		}
	}
	walkRows(table)

	if len(rawRows) == 0 {
		return &Table{}
	}

	// Expand into a dense grid honoring rowspan/colspan.
	grid := make([][]gridCell, len(rawRows))
	occupied := map[[2]int]bool{}
	for r, row := range rawRows {
		if grid[r] == nil {
			grid[r] = []gridCell{}
		}
		col := 0
		ri := 0
		for _, cell := range row {
			for occupied[[2]int{r, col}] {
				col++
			}
			for dr := 0; dr < cell.rowspan; dr++ {
				for dc := 0; dc < cell.colspan; dc++ {
					rr := r + dr
					cc := col + dc
					if rr >= len(grid) {
						continue
					}
					occupied[[2]int{rr, cc}] = true
					for len(grid[rr]) <= cc {
						grid[rr] = append(grid[rr], gridCell{})
					}
					grid[rr][cc] = gridCell{text: cell.text, isHeader: cell.isHeader}
				}
			}
			col += cell.colspan
			ri++
		}
	}

	// Header block = leading rows that are entirely header cells (or blank
	// filler from rowspan). Data starts at the first row containing any
	// non-header, non-empty cell in a position not covered by a spanning
	// header.
	headerRows := 0
	for _, row := range grid {
		allHeader := true
		anyContent := false
		for _, cell := range row {
			if strings.TrimSpace(cell.text) != "" {
				anyContent = true
				if !cell.isHeader {
					allHeader = false
				}
			}
		}
		if anyContent && allHeader {
			headerRows++
		} else {
			break
		}
	}
	if headerRows == 0 {
		headerRows = 1 // degrade gracefully: treat first row as header
	}
	// Legacy static report pages (pre-2012 .htm layout tables) frequently
	// style header rows with plain <td> instead of semantic <th>, which the
	// pass above never classifies as "header". Extend the header block past
	// any additional leading rows that look like column labels rather than
	// data: every cell present, none of them numeric, still within the
	// first few rows. A genuine data row for these reports always leads
	// with a serial number, ISIN, or similar numeric/coded value.
	for headerRows < len(grid) && headerRows < 4 {
		row := grid[headerRows]
		looksLikeLabels := len(row) > 0
		anyContent := false
		for _, cell := range row {
			text := strings.TrimSpace(cell.text)
			if text == "" {
				continue
			}
			anyContent = true
			if _, ok := ParseNumber(text); ok {
				looksLikeLabels = false
				break
			}
		}
		if !anyContent || !looksLikeLabels {
			break
		}
		headerRows++
	}
	if headerRows > len(grid) {
		headerRows = len(grid)
	}

	width := 0
	for _, row := range grid {
		if len(row) > width {
			width = len(row)
		}
	}

	columns := buildColumnNames(grid[:headerRows], width)

	var dataRows [][]string
	for _, row := range grid[headerRows:] {
		out := make([]string, width)
		for c := 0; c < width && c < len(row); c++ {
			out[c] = strings.TrimSpace(row[c].text)
		}
		// Skip fully-empty rows (layout spacer rows some NSDL pages include).
		nonEmpty := false
		for _, v := range out {
			if v != "" {
				nonEmpty = true
				break
			}
		}
		if nonEmpty {
			dataRows = append(dataRows, out)
		}
	}

	return &Table{Columns: columns, Rows: dataRows}
}

// buildColumnNames derives one composite leaf name per column index from the
// header block. The last header row supplies the leaf label; when a column's
// leaf label collides with another column's leaf label elsewhere in the
// table (e.g. "Equity" under both the top-level group and the "Mutual Funds"
// group), the immediate group label from the row above is prefixed — but
// only when the leaf label doesn't already contain the group label (NSDL's
// own "Debt-General Limit" style sub-labels already disambiguate themselves).
func buildColumnNames(headerRows [][]gridCell, width int) []string {
	if len(headerRows) == 0 || width == 0 {
		return nil
	}
	leaf := make([]string, width)
	group := make([]string, width)
	lastIdx := len(headerRows) - 1
	for c := 0; c < width; c++ {
		if c < len(headerRows[lastIdx]) {
			leaf[c] = strings.TrimSpace(headerRows[lastIdx][c].text)
		}
		if lastIdx > 0 && c < len(headerRows[lastIdx-1]) {
			group[c] = strings.TrimSpace(headerRows[lastIdx-1][c].text)
		}
	}

	seen := map[string]int{}
	for _, l := range leaf {
		if l == "" {
			continue
		}
		seen[strings.ToLower(l)]++
	}

	names := make([]string, width)
	usedNames := map[string]int{}
	for c := 0; c < width; c++ {
		name := leaf[c]
		if name == "" {
			name = group[c]
		}
		if name == "" {
			name = "col"
		}
		if seen[strings.ToLower(leaf[c])] > 1 && group[c] != "" &&
			!strings.Contains(strings.ToLower(leaf[c]), strings.ToLower(group[c])) {
			name = group[c] + " - " + leaf[c]
		}
		usedNames[name]++
		if usedNames[name] > 1 {
			name = name + " " + itoa(usedNames[name])
		}
		names[c] = name
	}
	return names
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}
