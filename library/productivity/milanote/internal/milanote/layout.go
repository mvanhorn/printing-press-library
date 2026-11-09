// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.

package milanote

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Canvas geometry, measured in the web app at 100% zoom. Positions, widths
// and heights live on a grid of UnitPx CSS pixels: a card w units wide
// renders w*9-2 px, and every card's height is likewise a whole number of
// units (minus the 2px border), so integer gaps come out exactly even.
const (
	UnitPx          = 9.0
	CardWidth       = 34.0 // default note / to-do / link / column width
	BoardIconWidth  = 10.0 // a sub-board tile
	BoardIconHeight = 14.0
	ColumnWidth     = 34.0
	DefaultGutter   = 3.0

	cardInsetPx    = 50.0 // border plus left/right padding inside a note
	bodyGlyphPx    = 5.9  // average advance of body text
	h1GlyphPx      = 10.5 // heading 1
	h2GlyphPx      = 8.5  // heading 2
	calloutInsetPx = 56.0
	taskInsetPx    = 28.0
	listInsetPx    = 24.0
)

// Height rules in grid units, fitted to rendered cards:
//
//	note        = 4 + sum(blocks)        paragraph = 1 + 2*lines
//	heading 1   = 1 + 3*lines            heading 2 = 1 + 2*lines
//	list item   = 1 + 2*lines            callout   = 3 + 2*lines
//	heading card = 4 + 3*lines           to-do list = 4 + sum(1 + 2*lines)
const (
	noteBaseU      = 4.0
	headingCardU   = 4.0
	emptyColumnU   = 16.0
	columnHeaderU  = 7.0
	columnItemGapU = 1.0
	columnFooterU  = 2.0
)

// wrapLines greedily word-wraps text into lines that fit innerPx.
func wrapLines(text string, innerPx, glyphPx float64) int {
	perLine := int(innerPx / glyphPx)
	if perLine < 6 {
		perLine = 6
	}
	total := 0
	for _, para := range strings.Split(text, "\n") {
		lines, cur := 1, 0
		for _, word := range strings.Fields(para) {
			n := utf8.RuneCountInString(word)
			for n > perLine { // a word longer than a line breaks mid-word
				if cur > 0 {
					lines++
				}
				n -= perLine
				cur = perLine
			}
			switch {
			case cur == 0:
				cur = n
			case cur+1+n <= perLine:
				cur += 1 + n
			default:
				lines++
				cur = n
			}
		}
		total += lines
	}
	return total
}

func innerPx(widthUnits float64) float64 {
	if widthUnits <= 0 {
		widthUnits = CardWidth
	}
	return widthUnits*UnitPx - cardInsetPx
}

// NoteHeight estimates a note's height in grid units.
func NoteHeight(doc map[string]any, widthUnits float64) float64 {
	blocks, _ := doc["content"].([]any)
	if len(blocks) == 0 {
		return noteBaseU + 3
	}
	h := noteBaseU
	for _, raw := range blocks {
		h += blockUnits(raw, innerPx(widthUnits))
	}
	return h
}

func blockUnits(raw any, inner float64) float64 {
	b, _ := raw.(map[string]any)
	if b == nil {
		return 0
	}
	switch b["type"] {
	case "heading":
		attrs, _ := b["attrs"].(map[string]any)
		if num(attrs["level"]) >= 2 {
			return 1 + 2*float64(wrapLines(PlainText(b), inner, h2GlyphPx))
		}
		return 1 + 3*float64(wrapLines(PlainText(b), inner, h1GlyphPx))
	case "callout":
		return 3 + 2*float64(wrapLines(PlainText(b), inner-calloutInsetPx, bodyGlyphPx))
	case "bulletList", "orderedList", "taskList":
		h := 0.0
		for _, item := range asList(b["content"]) {
			im, _ := item.(map[string]any)
			for _, part := range asList(im["content"]) {
				h += blockUnits(part, inner-listInsetPx)
			}
		}
		return h
	default:
		return 1 + 2*float64(wrapLines(PlainText(b), inner, bodyGlyphPx))
	}
}

// HeadingHeight estimates a heading card's height in grid units.
func HeadingHeight(level int, text string, widthUnits float64) float64 {
	glyph := h1GlyphPx
	if level >= 2 {
		glyph = h2GlyphPx
	}
	return headingCardU + 3*float64(wrapLines(text, innerPx(widthUnits), glyph))
}

// TaskListHeight estimates a to-do list's height in grid units. A shown
// title sits above the card and takes no height inside it.
func TaskListHeight(items []Task, widthUnits float64) float64 {
	h := noteBaseU
	if len(items) == 0 {
		return h + 3
	}
	for _, t := range items {
		h += 1 + 2*float64(wrapLines(t.Text, innerPx(widthUnits)-taskInsetPx, bodyGlyphPx))
	}
	return h
}

// ImageHeight is the rendered height of an image card of widthUnits.
func ImageHeight(w, h int, widthUnits float64) float64 {
	if widthUnits <= 0 {
		widthUnits = CardWidth
	}
	if w <= 0 || h <= 0 {
		return math.Ceil(widthUnits * 0.75)
	}
	return math.Round(widthUnits * float64(h) / float64(w))
}

// Box is an element's footprint on the canvas, in grid units.
type Box struct {
	ID   string  `json:"id"`
	Type string  `json:"type"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	W    float64 `json:"w"`
	H    float64 `json:"h"`
	Text string  `json:"text,omitempty"`
}

// Right edge.
func (b Box) Right() float64 { return b.X + b.W }

// Bottom edge.
func (b Box) Bottom() float64 { return b.Y + b.H }

// Overlaps reports whether two boxes intersect.
func (b Box) Overlaps(o Box) bool {
	return b.X < o.Right() && o.X < b.Right() && b.Y < o.Bottom() && o.Y < b.Bottom()
}

// BoxOf estimates an element's canvas footprint.
func BoxOf(el *Element) Box {
	w := el.Width()
	b := Box{ID: el.ID, Type: el.ElementType, X: el.Location.X(), Y: el.Location.Y(), Text: firstLine(el.Text(), 60)}
	switch el.ElementType {
	case TypeBoard:
		if w <= 0 {
			w = BoardIconWidth
		}
		b.W, b.H = w, BoardIconHeight
	case TypeColumn:
		if w <= 0 {
			w = ColumnWidth
		}
		b.W, b.H = w, emptyColumnU
	case TypeTaskList:
		if w <= 0 {
			w = CardWidth
		}
		b.W, b.H = w, TaskListHeight(el.Tasks(), w)
	case TypeImage:
		if w <= 0 {
			w = CardWidth
		}
		img, _ := el.Content["image"].(map[string]any)
		b.W, b.H = w, ImageHeight(int(num(img["width"])), int(num(img["height"])), w)
	case TypeCommentThread:
		b.W, b.H = CardWidth, 8
	default:
		if w <= 0 {
			w = CardWidth
		}
		doc, _ := el.Content["textContent"].(map[string]any)
		if doc == nil {
			doc = Doc(Paragraph(el.Text()))
		}
		if level, text, ok := headingCard(el); ok {
			b.W, b.H = w, HeadingHeight(level, text, w)
		} else {
			b.W, b.H = w, NoteHeight(doc, w)
		}
	}
	return b
}

// headingCard reports whether a card is a transparent card holding a single
// heading, the shape the heading tool creates.
func headingCard(el *Element) (int, string, bool) {
	tr, _ := el.Content["transparent"].(map[string]any)
	if enabled, _ := tr["enabled"].(bool); !enabled {
		return 0, "", false
	}
	doc, _ := el.Content["textContent"].(map[string]any)
	blocks := asList(doc["content"])
	if len(blocks) != 1 {
		return 0, "", false
	}
	h, _ := blocks[0].(map[string]any)
	if h["type"] != "heading" {
		return 0, "", false
	}
	attrs, _ := h["attrs"].(map[string]any)
	return int(num(attrs["level"])), PlainText(h), true
}

func firstLine(s string, max int) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if utf8.RuneCountInString(s) > max {
		r := []rune(s)
		s = string(r[:max-1]) + "…"
	}
	return s
}

// Grid packs items into equal-width columns separated by Gutter units.
type Grid struct {
	X, Y   float64
	Cols   int
	ColW   float64
	Gutter float64
}

// Slot is where a packed item goes.
type Slot struct {
	X, Y float64
	Col  int
	Row  int
}

// Rows places items row by row: every row starts below the tallest item of
// the row above, so rows line up and gaps stay equal.
func (g Grid) Rows(heights []float64) []Slot {
	g = g.normalized()
	out := make([]Slot, len(heights))
	y := g.Y
	for start := 0; start < len(heights); start += g.Cols {
		end := start + g.Cols
		if end > len(heights) {
			end = len(heights)
		}
		tallest := 0.0
		for i := start; i < end; i++ {
			col := i - start
			out[i] = Slot{X: g.X + float64(col)*(g.ColW+g.Gutter), Y: y, Col: col, Row: start / g.Cols}
			tallest = math.Max(tallest, heights[i])
		}
		y += tallest + g.Gutter
	}
	return out
}

// Masonry drops each item into the currently shortest column (ties go
// left), giving equal vertical gaps inside every column.
func (g Grid) Masonry(heights []float64) []Slot {
	g = g.normalized()
	bottoms := make([]float64, g.Cols)
	counts := make([]int, g.Cols)
	for i := range bottoms {
		bottoms[i] = g.Y
	}
	out := make([]Slot, len(heights))
	for i, h := range heights {
		col := 0
		for c := 1; c < g.Cols; c++ {
			if bottoms[c] < bottoms[col]-0.01 {
				col = c
			}
		}
		out[i] = Slot{X: g.X + float64(col)*(g.ColW+g.Gutter), Y: bottoms[col], Col: col, Row: counts[col]}
		bottoms[col] += h + g.Gutter
		counts[col]++
	}
	return out
}

// PackItem is a card's footprint for Pack.
type PackItem struct {
	W, H float64
}

// CommonWidth returns the width most cards share (the wider one on a tie).
// Using it as the column width keeps one wide heading from stretching every
// column and opening a hole beside the normal cards.
func CommonWidth(widths []float64) float64 {
	counts := map[float64]int{}
	best, bestN := 0.0, 0
	for _, w := range widths {
		counts[w]++
		if counts[w] > bestN || (counts[w] == bestN && w > best) {
			best, bestN = w, counts[w]
		}
	}
	return best
}

func (g Grid) spanOf(w float64) int {
	span := int(math.Ceil((w+g.Gutter)/(g.ColW+g.Gutter) - 0.01))
	if span < 1 {
		return 1
	}
	if span > g.Cols {
		return g.Cols
	}
	return span
}

// Pack places cards of mixed widths: a card wider than one column spans as
// many as it needs. By default each card takes the highest free spot, leaving
// the gutter under whatever is above it; with rows set, the tops of each row
// line up instead.
func (g Grid) Pack(items []PackItem, rows bool) []Slot {
	g = g.normalized()
	out := make([]Slot, len(items))
	x := func(col int) float64 { return g.X + float64(col)*(g.ColW+g.Gutter) }
	if rows {
		col, row, y, tallest := 0, 0, g.Y, 0.0
		for i, it := range items {
			span := g.spanOf(it.W)
			if col > 0 && col+span > g.Cols {
				y += tallest + g.Gutter
				col, tallest = 0, 0
				row++
			}
			out[i] = Slot{X: x(col), Y: y, Col: col, Row: row}
			tallest = math.Max(tallest, it.H)
			col += span
		}
		return out
	}
	bottoms := make([]float64, g.Cols)
	counts := make([]int, g.Cols)
	for i := range bottoms {
		bottoms[i] = g.Y
	}
	for i, it := range items {
		span := g.spanOf(it.W)
		bestCol, bestTop := 0, math.Inf(1)
		for c := 0; c+span <= g.Cols; c++ {
			top := 0.0
			for k := c; k < c+span; k++ {
				top = math.Max(top, bottoms[k])
			}
			if top < bestTop-0.01 {
				bestCol, bestTop = c, top
			}
		}
		out[i] = Slot{X: x(bestCol), Y: bestTop, Col: bestCol, Row: counts[bestCol]}
		for k := bestCol; k < bestCol+span; k++ {
			bottoms[k] = bestTop + it.H + g.Gutter
			counts[k]++
		}
	}
	return out
}

// Stack places items top to bottom in one column.
func (g Grid) Stack(heights []float64) []Slot {
	g.Cols = 1
	return g.Masonry(heights)
}

func (g Grid) normalized() Grid {
	if g.Cols < 1 {
		g.Cols = 1
	}
	if g.ColW <= 0 {
		g.ColW = CardWidth
	}
	if g.Gutter < 0 {
		g.Gutter = 0
	}
	return g
}

// Snap rounds a coordinate to whole grid units.
func Snap(v float64) float64 { return math.Round(v) }

// FreeOrigin returns a spot below everything already on the canvas, aligned
// to the leftmost element, so new content never covers existing cards.
func FreeOrigin(boxes []Box, gutter float64) (float64, float64) {
	if len(boxes) == 0 {
		return 2, 2
	}
	minX, maxBottom := math.Inf(1), math.Inf(-1)
	for _, b := range boxes {
		minX = math.Min(minX, b.X)
		maxBottom = math.Max(maxBottom, b.Bottom())
	}
	return Snap(minX), Snap(maxBottom + gutter)
}

// BoxIn is BoxOf with access to the whole board, so a column's height counts
// the cards inside it.
func BoxIn(p *BoardPayload, el *Element) Box {
	b := BoxOf(el)
	if el.ElementType != TypeColumn || p == nil {
		return b
	}
	kids := p.Children(el.ID, SectionInbox)
	if len(kids) == 0 {
		return b
	}
	h := columnHeaderU
	for _, k := range kids {
		h += BoxOf(k).H + columnItemGapU
	}
	b.H = math.Max(b.H, h+columnFooterU)
	return b
}

// CanvasBoxes returns footprints of every canvas element directly on a board.
func CanvasBoxes(p *BoardPayload, boardID string) []Box {
	var out []Box
	for _, el := range p.Children(boardID, SectionCanvas) {
		if !OnCanvasGrid(el) {
			continue
		}
		out = append(out, BoxIn(p, el))
	}
	return out
}

// OnCanvasGrid reports whether an element takes up space in a layout. Lines,
// annotations and comment pins float over the cards.
func OnCanvasGrid(el *Element) bool {
	switch el.ElementType {
	case TypeLine, TypeAnnotation, TypeCommentThread:
		return false
	}
	return true
}

// Issue is one layout problem found by Check.
type Issue struct {
	Kind    string   `json:"kind"`
	IDs     []string `json:"ids"`
	Detail  string   `json:"detail"`
	Penalty float64  `json:"-"`
}

// CheckReport summarises a board's tidiness.
type CheckReport struct {
	Elements   int     `json:"elements"`
	Score      int     `json:"score"`
	Overlaps   int     `json:"overlaps"`
	Misaligned int     `json:"near_misses"`
	GapSpread  float64 `json:"gap_spread"`
	Issues     []Issue `json:"issues"`
}

// Check scores canvas tidiness: overlapping cards, edges that almost (but
// don't) line up, and uneven gaps between neighbouring cards, whether stacked
// or side by side. Both directions share one gutter, so a wide hole between
// columns shows up even when it repeats on every row.
func Check(boxes []Box, tolerance float64) CheckReport {
	if tolerance <= 0 {
		tolerance = 2
	}
	r := CheckReport{Elements: len(boxes)}
	sorted := append([]Box(nil), boxes...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Y != sorted[j].Y {
			return sorted[i].Y < sorted[j].Y
		}
		return sorted[i].X < sorted[j].X
	})
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			a, b := sorted[i], sorted[j]
			if a.Overlaps(b) {
				r.Overlaps++
				r.Issues = append(r.Issues, Issue{Kind: "overlap", IDs: []string{a.ID, b.ID},
					Detail: quote(a.Text) + " overlaps " + quote(b.Text), Penalty: 10})
				continue
			}
			if d := math.Abs(a.X - b.X); d > 0 && d <= tolerance && verticalNeighbours(a, b) {
				r.Misaligned++
				r.Issues = append(r.Issues, Issue{Kind: "left-edge", IDs: []string{a.ID, b.ID},
					Detail: quote(a.Text) + " and " + quote(b.Text) + " are " + fmtUnits(d) + " off a shared left edge", Penalty: 3})
			}
			if d := math.Abs(a.Y - b.Y); d > 0 && d <= tolerance && horizontalNeighbours(a, b) {
				r.Misaligned++
				r.Issues = append(r.Issues, Issue{Kind: "top-edge", IDs: []string{a.ID, b.ID},
					Detail: quote(a.Text) + " and " + quote(b.Text) + " are " + fmtUnits(d) + " off a shared top edge", Penalty: 2})
			}
		}
	}
	gaps := append(verticalGaps(sorted, tolerance), horizontalGaps(sorted)...)
	if len(gaps) >= 2 {
		lo, hi := gaps[0].gap, gaps[0].gap
		for _, g := range gaps {
			lo, hi = math.Min(lo, g.gap), math.Max(hi, g.gap)
		}
		r.GapSpread = hi - lo
		if r.GapSpread > tolerance {
			mode := modeGap(gaps)
			for _, g := range gaps {
				if math.Abs(g.gap-mode) > tolerance {
					r.Issues = append(r.Issues, Issue{Kind: "gap", IDs: []string{g.a, g.b},
						Detail: "gap of " + fmtUnits(g.gap) + " where most are " + fmtUnits(mode), Penalty: 1})
				}
			}
		}
	}
	penalty := 0.0
	for _, is := range r.Issues {
		penalty += is.Penalty
	}
	score := 100 - penalty
	if score < 0 {
		score = 0
	}
	r.Score = int(score)
	return r
}

type gapPair struct {
	a, b string
	gap  float64
}

// verticalGaps measures the gap between each card and the next card below
// it in the same column (same left edge within tolerance).
func verticalGaps(sorted []Box, tolerance float64) []gapPair {
	var out []gapPair
	for i, a := range sorted {
		best := -1
		for j := i + 1; j < len(sorted); j++ {
			b := sorted[j]
			if math.Abs(a.X-b.X) > tolerance || b.Y < a.Bottom()-0.5 {
				continue
			}
			if best < 0 || b.Y < sorted[best].Y {
				best = j
			}
		}
		if best >= 0 {
			out = append(out, gapPair{a.ID, sorted[best].ID, math.Max(0, sorted[best].Y-a.Bottom())})
		}
	}
	return out
}

// sectionGapU is the distance past which two side-by-side cards are treated
// as separate areas of the board rather than neighbours.
const sectionGapU = 60

// horizontalGaps measures the gap between each card and its nearest neighbour
// to the right, among cards that share part of the same rows.
func horizontalGaps(sorted []Box) []gapPair {
	var out []gapPair
	for i, a := range sorted {
		best, bestGap := -1, 0.0
		for j, b := range sorted {
			if i == j || b.X < a.Right()-0.5 || b.Y >= a.Bottom() || a.Y >= b.Bottom() {
				continue
			}
			if gap := math.Max(0, b.X-a.Right()); best < 0 || gap < bestGap {
				best, bestGap = j, gap
			}
		}
		if best >= 0 && bestGap <= sectionGapU {
			out = append(out, gapPair{a.ID, sorted[best].ID, bestGap})
		}
	}
	return out
}

func modeGap(gaps []gapPair) float64 {
	counts := map[float64]int{}
	best, bestN := gaps[0].gap, 0
	for _, g := range gaps {
		k := math.Round(g.gap)
		counts[k]++
		if counts[k] > bestN {
			best, bestN = k, counts[k]
		}
	}
	return best
}

func verticalNeighbours(a, b Box) bool {
	return math.Abs(b.Y-a.Bottom()) < 30 || math.Abs(a.Y-b.Bottom()) < 30
}

func horizontalNeighbours(a, b Box) bool {
	return math.Abs(b.X-a.Right()) < 30 || math.Abs(a.X-b.Right()) < 30
}

func quote(s string) string {
	if s == "" {
		return "(untitled)"
	}
	return "\"" + s + "\""
}

func fmtUnits(v float64) string {
	return strconv.FormatFloat(math.Round(v*10)/10, 'f', -1, 64) + "u"
}
