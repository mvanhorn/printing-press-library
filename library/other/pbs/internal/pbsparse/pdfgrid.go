// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package pbsparse

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/ledongthuc/pdf"
)

// pdfGlyph is one positioned text run. PBS annexure PDFs emit one run per
// glyph, so these are single characters.
type pdfGlyph struct {
	x, y, w, size float64
	s             string
}

// pdfLine is one horizontal band of glyphs.
type pdfLine struct {
	y      float64
	glyphs []pdfGlyph
}

// center returns the horizontal midpoint of a glyph, which is what column
// assignment keys on. Using the left edge instead is unsafe: PBS right-aligns
// its numeric columns, so a value's left edge drifts with its digit count and
// a wide number can appear to start inside the previous column.
func (g pdfGlyph) center() float64 { return g.x + g.w/2 }

// extractPDFLines groups every page's glyphs into vertical bands.
func extractPDFLines(data []byte) (lines []pdfLine, err error) {
	defer func() {
		if r := recover(); r != nil {
			lines, err = nil, fmt.Errorf("pdf reader panicked: %v", r)
		}
	}()
	rd, rerr := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if rerr != nil {
		return nil, fmt.Errorf("open pdf: %w", rerr)
	}
	for i := 1; i <= rd.NumPage(); i++ {
		p := rd.Page(i)
		if p.V.IsNull() {
			continue
		}
		var gs []pdfGlyph
		for _, t := range p.Content().Text {
			if t.S == "" {
				continue
			}
			gs = append(gs, pdfGlyph{x: t.X, y: t.Y, w: t.W, size: t.FontSize, s: t.S})
		}
		if len(gs) == 0 {
			continue
		}
		sort.Slice(gs, func(a, b int) bool {
			if d := gs[a].y - gs[b].y; d > pdfLineTol || d < -pdfLineTol {
				return gs[a].y > gs[b].y
			}
			return gs[a].x < gs[b].x
		})
		var cur pdfLine
		for j, g := range gs {
			if j > 0 {
				d := gs[j-1].y - g.y
				if d > pdfLineTol || d < -pdfLineTol {
					lines = append(lines, cur)
					cur = pdfLine{}
				}
			}
			if len(cur.glyphs) == 0 {
				cur.y = g.y
			}
			cur.glyphs = append(cur.glyphs, g)
		}
		if len(cur.glyphs) > 0 {
			lines = append(lines, cur)
		}
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("pdf yielded no positioned text runs")
	}
	return lines, nil
}

// cellsOf splits a line into cells at font-relative horizontal gaps.
func cellsOf(l pdfLine) []string {
	var cells []string
	var cur strings.Builder
	for j, g := range l.glyphs {
		if j > 0 {
			prev := l.glyphs[j-1]
			size := g.size
			if size <= 0 {
				size = prev.size
			}
			if size <= 0 {
				size = 8
			}
			if g.x-(prev.x+prev.w) > pdfCellGapRatio*size {
				if t := strings.TrimSpace(cur.String()); t != "" {
					cells = append(cells, t)
				}
				cur.Reset()
			}
		}
		cur.WriteString(g.s)
	}
	if t := strings.TrimSpace(cur.String()); t != "" {
		cells = append(cells, t)
	}
	return cells
}

// pdfColumnModel is the horizontal geometry of one stacked band, learned from
// that band's own header rows rather than assumed.
//
// This exists because gap-based tokenization alone is not enough: a long item
// description overflows its column and its glyphs interleave with the UNIT
// column's, producing labels like "Rice Basmati Broken (Average Qualit1y )K".
// The numeric cells survive that, but the label does not. Assigning glyphs to
// columns by their own X centre against learned boundaries separates them
// cleanly.
type pdfColumnModel struct {
	// bounds[i] is the right-hand edge of column i.
	bounds []float64
	// stat maps a value-column index to its statistic.
	stat []Stat
	// firstValueCol is the index of the first numeric column.
	firstValueCol int
	// descCol and unitCol are the label columns.
	descCol, unitCol int
	// valueLeft is the left edge of the first numeric column. Glyphs left of
	// it belong to the label region and are tokenized by gap instead of by
	// column, because the DESCRIPTION header is centred over a wide column
	// while its text is left-aligned — using the header midpoint as a boundary
	// pulls the first letters of the description into the item-number column.
	valueLeft float64
}

// splitRegions divides a data line into its label glyphs and value glyphs.
func (m *pdfColumnModel) splitRegions(l pdfLine) (label, values pdfLine) {
	label.y, values.y = l.y, l.y
	for _, g := range l.glyphs {
		if g.center() < m.valueLeft {
			label.glyphs = append(label.glyphs, g)
		} else {
			values.glyphs = append(values.glyphs, g)
		}
	}
	return label, values
}

// buildColumnModel derives column edges from a MIN/AVG/MAX header line.
func buildColumnModel(header pdfLine) (*pdfColumnModel, bool) {
	type hcell struct {
		text     string
		lo, hi   float64
		centreAt float64
	}
	var hs []hcell
	var cur strings.Builder
	var lo, hi float64
	started := false
	flush := func() {
		if !started {
			return
		}
		if t := strings.TrimSpace(cur.String()); t != "" {
			hs = append(hs, hcell{text: t, lo: lo, hi: hi, centreAt: (lo + hi) / 2})
		}
		cur.Reset()
		started = false
	}
	for j, g := range header.glyphs {
		if j > 0 {
			prev := header.glyphs[j-1]
			size := g.size
			if size <= 0 {
				size = prev.size
			}
			if size <= 0 {
				size = 8
			}
			if g.x-(prev.x+prev.w) > pdfCellGapRatio*size {
				flush()
			}
		}
		if !started {
			lo = g.x
			started = true
		}
		hi = g.x + g.w
		cur.WriteString(g.s)
	}
	flush()

	m := &pdfColumnModel{descCol: -1, unitCol: -1, firstValueCol: -1}
	for i, h := range hs {
		up := strings.ToUpper(h.text)
		switch {
		case strings.HasPrefix(up, "DESCRIPTION"):
			m.descCol = i
		case up == "UNIT":
			m.unitCol = i
		}
		var st Stat
		switch up {
		case "MIN":
			st = StatMin
		case "AVG", "AVERAGE":
			st = StatAvg
		case "MAX":
			st = StatMax
		}
		if st != "" && m.firstValueCol < 0 {
			m.firstValueCol = i
		}
		m.stat = append(m.stat, st)
	}
	if m.firstValueCol < 0 || m.descCol < 0 {
		return nil, false
	}
	// Column boundaries are the midpoints between adjacent header cells. These
	// are reliable for the numeric columns, whose right-aligned values sit
	// under centred MIN/AVG/MAX headers.
	for i := 0; i+1 < len(hs); i++ {
		m.bounds = append(m.bounds, (hs[i].hi+hs[i+1].lo)/2)
	}
	// The value region begins midway between the last label header and the
	// first numeric header.
	if m.firstValueCol > 0 && m.firstValueCol-1 < len(hs) {
		m.valueLeft = (hs[m.firstValueCol-1].hi + hs[m.firstValueCol].lo) / 2
	} else if m.firstValueCol < len(hs) {
		m.valueLeft = hs[m.firstValueCol].lo
	}
	return m, true
}

// assign splits a data line into cells using the learned column boundaries.
func (m *pdfColumnModel) assign(l pdfLine) []string {
	out := make([]string, len(m.bounds)+1)
	for _, g := range l.glyphs {
		c := sort.SearchFloat64s(m.bounds, g.center())
		if c >= len(out) {
			c = len(out) - 1
		}
		out[c] += g.s
	}
	for i := range out {
		out[i] = strings.TrimSpace(out[i])
	}
	return out
}
