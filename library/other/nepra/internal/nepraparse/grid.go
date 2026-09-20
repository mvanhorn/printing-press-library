package nepraparse

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// ErrNoTable is returned when the document contains no <table> rows at all.
var ErrNoTable = errors.New("nepraparse: no table rows found")

// maxSpan bounds colspan/rowspan so a malformed export cannot make the grid
// allocate without limit. The largest real span in these files is 32.
const maxSpan = 256

// maxRowWidth bounds the TOTAL expanded width of one row.
//
// maxSpan caps each colspan/rowspan individually, and its own comment claimed
// that stopped the grid allocating without limit — but nothing capped their
// PRODUCT. A row of many legal 256-wide colspans advances the column cursor
// without bound, and the pending-rowspan map grows one entry per column, so
// the allocation is driven by a hostile document rather than by its size.
// The widest real table in this corpus is 40 physical columns against the
// frozen 32 logical ones.
const maxRowWidth = 4096

// Grid is a workbook table flattened into a dense rectangle of cell text,
// with every colspan and rowspan expanded so that Rows[r][c] is the value
// occupying logical position (r, c).
//
// Expanding before assigning columns is not cosmetic. Thirteen FY2023-24 data
// rows carry a single <td colspan=26>DELICENSED|DECOMMISSIONED and two carry
// an empty <td colspan=26>; raw td-per-tr in FY2023-24 is
// {8:1, 14:16, 20:1, 33:1, 39:120}. A parser that walks raw <td> elements
// reads "DELICENSED" as Jul %age and then shifts the trailing padding cells
// into Jul GWh..Oct %age. Because these files contain zero <th>, nothing
// catches the shift and the numbers stay plausible.
type Grid struct {
	// Rows holds the expanded cell text, entity-decoded and
	// whitespace-collapsed. Every row has exactly Width entries; positions
	// no cell reached are "" — absent, never a zero.
	Rows [][]string
	// Width is the widest expanded row.
	Width int
	// RawCellsPerRow[i] is the number of literal <td>/<th> elements in
	// source row i, before span expansion. Kept because the distribution is
	// the cheapest way to notice that a year's export changed shape.
	RawCellsPerRow []int
	// RawCells is the total number of literal <td>/<th> elements.
	RawCells int
	// Tables counts <table> elements. Every sampled year has exactly one.
	Tables int
	// HeaderCells counts <th> elements. Every sampled year has ZERO, which is
	// why header detection in this package is text-matched and positional.
	HeaderCells int
	// SpanCollisions counts slots a colspanning cell tried to write while a
	// rowspan from an earlier row already owned them. Such a table is
	// malformed — two cells claim one position — and the carried-down value
	// is KEPT rather than overwritten, because overwriting silently discards
	// data the document published.
	//
	// Every sampled year has ZERO. A non-zero count means the export's shape
	// changed and the column mapping should be re-derived rather than
	// trusted.
	SpanCollisions int
}

// gridRow accumulates one physical row during expansion. filled marks slots
// already claimed by a cell — including a spanned cell whose text is empty,
// which must still consume its slot rather than being overwritten.
type gridRow struct {
	text   []string
	filled []bool
}

func (r *gridRow) grow(n int) {
	for len(r.text) < n {
		r.text = append(r.text, "")
		r.filled = append(r.filled, false)
	}
}

func (r *gridRow) set(c int, text string) {
	r.grow(c + 1)
	r.text[c] = text
	r.filled[c] = true
}

func (r *gridRow) isFilled(c int) bool { return c < len(r.filled) && r.filled[c] }

type pendingSpan struct {
	text string
	rows int
}

// BuildGrid tokenises the decoded workbook and expands its table into a
// [Grid]. It uses a tokeniser rather than a DOM parse so row and cell order
// is exactly source order and no cell is ever re-parented.
func BuildGrid(doc string) (*Grid, error) {
	z := html.NewTokenizer(strings.NewReader(doc))
	g := &Grid{}

	var (
		inRow    bool
		inCell   bool
		cellText strings.Builder
		row      gridRow
		pending  = map[int]pendingSpan{}
		colspan  = 1
		rowspan  = 1
		col      int
		rawCells int
	)

	// startRow prepares the next physical row, first laying down the cells
	// carried into it by a rowspan from an earlier row. The header band's six
	// stub columns (S.No .. Dependable Capacity) reach rows 3 and 4 only
	// through this path.
	startRow := func() {
		row = gridRow{}
		for c, p := range pending {
			row.set(c, p.text)
			if p.rows-1 == 0 {
				delete(pending, c)
			} else {
				pending[c] = pendingSpan{text: p.text, rows: p.rows - 1}
			}
		}
		col = 0
		rawCells = 0
	}

	place := func(text string, cs, rs int) {
		for row.isFilled(col) {
			col++
		}
		// maxSpan bounds a SINGLE span; this bounds the ROW. Without it a
		// document with many maximal colspans walks `col` up without limit —
		// each cell is individually legal, and it is their product that
		// allocates. The widest real table here is 40 physical columns, so
		// maxRowWidth is three orders of magnitude clear of anything genuine
		// and truncates only a table that could not be data.
		if col >= maxRowWidth {
			g.SpanCollisions++
			return
		}
		if col+cs > maxRowWidth {
			cs = maxRowWidth - col
		}
		for k := 0; k < cs; k++ {
			c := col + k
			// A colspan can run INTO a slot a rowspan from an earlier row
			// already owns. That is a malformed table, and the previous
			// behaviour overwrote the carried-down value — losing a
			// published cell to a cell that merely spanned over it. Keep the
			// existing owner and count the collision so it is observable
			// instead of silent.
			if row.isFilled(c) {
				g.SpanCollisions++
				continue
			}
			row.set(c, text)
			if rs > 1 {
				pending[c] = pendingSpan{text: text, rows: rs - 1}
			}
		}
		col += cs
	}

	flushCell := func() {
		if !inCell {
			return
		}
		place(collapseText(html.UnescapeString(cellText.String())), colspan, rowspan)
		inCell = false
	}

	endRow := func() {
		out := make([]string, len(row.text))
		copy(out, row.text)
		g.Rows = append(g.Rows, out)
		g.RawCellsPerRow = append(g.RawCellsPerRow, rawCells)
		g.RawCells += rawCells
		if len(out) > g.Width {
			g.Width = len(out)
		}
		inRow = false
	}

	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		switch tt {
		case html.TextToken:
			if inCell {
				cellText.Write(z.Text())
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			switch atom.Lookup(name) {
			case atom.Table:
				g.Tables++
			case atom.Tr:
				flushCell()
				if inRow {
					endRow()
				}
				inRow = true
				startRow()
			case atom.Td, atom.Th:
				if atom.Lookup(name) == atom.Th {
					g.HeaderCells++
				}
				flushCell()
				if !inRow {
					// A cell outside any <tr>: open an implicit row so its
					// content is never dropped.
					inRow = true
					startRow()
				}
				inCell = true
				cellText.Reset()
				colspan, rowspan = 1, 1
				for hasAttr {
					var k, v []byte
					k, v, hasAttr = z.TagAttr()
					switch strings.ToLower(string(k)) {
					case "colspan":
						colspan = clampSpan(string(v))
					case "rowspan":
						rowspan = clampSpan(string(v))
					}
				}
				rawCells++
				if tt == html.SelfClosingTagToken {
					flushCell()
				}
			case atom.Br:
				// An in-cell line break is real whitespace. Every other tag
				// is removed WITHOUT inserting a space, because Excel splits
				// words across markup: "Company Limit<span
				// style='display:none'>ed.</span>" must rejoin as "Limited.".
				if inCell {
					cellText.WriteByte(' ')
				}
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			switch atom.Lookup(name) {
			case atom.Td, atom.Th:
				flushCell()
			case atom.Tr:
				flushCell()
				if inRow {
					endRow()
				}
			case atom.Table:
				flushCell()
				if inRow {
					endRow()
				}
			}
		}
	}
	flushCell()
	if inRow {
		endRow()
	}
	if err := z.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("nepraparse: tokenizing workbook: %w", err)
	}
	if len(g.Rows) == 0 {
		return nil, ErrNoTable
	}
	// Pad every row to the full width so callers can index without bounds
	// checks. Padding is "" — an absent cell, never a zero.
	for i, r := range g.Rows {
		if len(r) < g.Width {
			padded := make([]string, g.Width)
			copy(padded, r)
			g.Rows[i] = padded
		}
	}
	return g, nil
}

func clampSpan(v string) int {
	n, err := strconv.Atoi(strings.TrimSpace(strings.Trim(v, `"'`)))
	if err != nil || n < 1 {
		return 1
	}
	if n > maxSpan {
		return maxSpan
	}
	return n
}
