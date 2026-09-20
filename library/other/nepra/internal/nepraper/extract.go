// Package nepraper extracts reliability metrics from NEPRA's DISCO
// Performance Evaluation Report (PER) PDFs.
//
// # Why this package exists in this shape
//
// NEPRA publishes one PER per fiscal year. The reports are real text-layer
// PDFs (no OCR needed) but they are not a dataset: table numbers drift year to
// year, the entity roster differs between tables *inside a single report*,
// targets and breaches sit alongside actuals, and at least one report prints
// the same metric for the same DISCO twice with two different values, each
// internally consistent with its own chart.
//
// Consequently this package is deliberately *not* a "give me the SAIDI for
// MEPCO" API. It returns every Observation it saw with full provenance (metric,
// entity, source table caption, page, variant label) and exposes Conflicts to
// surface same-key disagreements. It never averages, prefers or drops an
// observation, and it never fabricates one: absence is representable as a typed
// Value kind, never as a zero.
package nepraper

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/ledongthuc/pdf"
)

// Span is one piece of text drawn on a page, with the position the PDF content
// stream placed it at. Positions are retained because they are the only way to
// independently verify a figure that appears more than once in a report.
type Span struct {
	Text string
	X    float64
	Y    float64
	// W is the advance width of the span in points. It is what makes reliable
	// cell reconstruction possible: NEPRA PERs emit one text-showing operation
	// per glyph, so the only way to tell "31419.30 9704.00" from a single
	// 16-digit number is the horizontal gap between glyph boxes.
	W    float64
	Page int // 1-indexed
}

// EndX is the right edge of the span.
func (s Span) EndX() float64 { return s.X + s.W }

// Line is a set of spans that share (approximately) a baseline, ordered left to
// right. Table rows in NEPRA PERs are laid out as independent text-showing
// operations per cell, so reconstructing lines is a prerequisite for parsing.
type Line struct {
	Page  int
	Y     float64
	Spans []Span
}

// Text joins the line's spans with single spaces, left to right.
func (l Line) Text() string {
	parts := make([]string, 0, len(l.Spans))
	for _, s := range l.Spans {
		t := strings.TrimSpace(s.Text)
		if t == "" {
			continue
		}
		parts = append(parts, t)
	}
	return strings.Join(parts, " ")
}

// PageText is the extracted content of a single page.
type PageText struct {
	Number int // 1-indexed
	Text   string
	Spans  []Span
	Lines  []Line
}

// CharCount is the number of characters in the page's flat text. Pages under
// ~20 characters are covers or image-only pages; the count is reported rather
// than judged so callers can apply their own threshold.
func (p PageText) CharCount() int { return len([]rune(p.Text)) }

// Doc is a fully extracted PER PDF.
type Doc struct {
	// NumPages is the page count read from the PDF page tree. It is NOT taken
	// from file(1), which reports "1 pages" for most NEPRA PERs.
	NumPages int
	Pages    []PageText
	// Creator is the /Creator string from the document info dictionary
	// ("Nitro Pro 8" on most NEPRA PERs), empty if absent.
	Creator string
	// Producer is the /Producer string from the document info dictionary.
	Producer string
}

// FullText concatenates all page text with form feeds between pages.
func (d *Doc) FullText() string {
	parts := make([]string, 0, len(d.Pages))
	for _, p := range d.Pages {
		parts = append(parts, p.Text)
	}
	return strings.Join(parts, "\f")
}

// CharCount is the total character count across all pages, excluding the
// page separators added by FullText.
func (d *Doc) CharCount() int {
	n := 0
	for _, p := range d.Pages {
		n += p.CharCount()
	}
	return n
}

// LowTextPages returns the 1-indexed page numbers whose character count is
// below min. A NEPRA PER with a real text layer has none of these except,
// in some years, the cover.
func (d *Doc) LowTextPages(min int) []int {
	var out []int
	for _, p := range d.Pages {
		if p.CharCount() < min {
			out = append(out, p.Number)
		}
	}
	return out
}

// AllLines returns every reconstructed line in the document in page order.
func (d *Doc) AllLines() []Line {
	var out []Line
	for _, p := range d.Pages {
		out = append(out, p.Lines...)
	}
	return out
}

// minPDFBytesPerPage is the floor used to sanity-check a PDF's self-declared
// page count against its actual size. Measured against this corpus: the seven
// PER documents run 1.0-3.0 MB over 27-43 pages, i.e. tens of kilobytes per
// page, so a floor of 32 bytes is three orders of magnitude below anything
// real and rejects only counts that could not possibly be represented.
const minPDFBytesPerPage = 32

// ErrNotPDF is returned when the input does not begin with a PDF header. It
// exists because NEPRA's own index links 404 to an HTML error body, which would
// otherwise be parsed as an empty document.
var ErrNotPDF = errors.New("nepraper: input is not a PDF (missing %PDF header)")

// lineTolerance is the vertical distance, in PDF points, within which two spans
// are treated as sharing a baseline. NEPRA tables are typeset with cell text
// jittering by well under a point; 2.0 groups a row without merging adjacent
// rows, whose leading is >= 9pt in every observed report.
const lineTolerance = 2.0

// ExtractText extracts the text layer of a PER PDF.
//
// It returns per-page text, per-page positioned spans, reconstructed lines, and
// the page count read from the PDF page tree. It never OCRs and never guesses:
// a page that yields no text yields an empty string, and the caller can see
// that via PageText.CharCount.
func ExtractText(pdfBytes []byte) (doc *Doc, err error) {
	// A PANIC FROM THE PDF LIBRARY BECOMES AN ERROR, NOT A CRASH.
	//
	// pageSpans and plainText each already recover, which shows the third
	// party panics on malformed input — but pdf.NewReader, r.NumPage(),
	// r.Trailer() and r.Page(i) all run on THIS frame with no guard, so a
	// document that panics in the page tree rather than in a content stream
	// takes the whole process down. Every input here is a multi-megabyte
	// file fetched over the network from a site this CLI does not control,
	// and the caller's contract is an error value.
	defer func() {
		if r := recover(); r != nil {
			doc = nil
			err = fmt.Errorf("nepraper: the pdf library panicked reading this document (%v); "+
				"treating it as unreadable rather than crashing. %d bytes starting %q",
				r, len(pdfBytes), safePrefix(pdfBytes, 16))
		}
	}()
	if len(pdfBytes) < 5 || !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
		return nil, fmt.Errorf("%w (got %d bytes starting %q)", ErrNotPDF,
			len(pdfBytes), safePrefix(pdfBytes, 16))
	}
	r, rerr := pdf.NewReader(bytes.NewReader(pdfBytes), int64(len(pdfBytes)))
	if rerr != nil {
		return nil, fmt.Errorf("nepraper: open pdf: %w", rerr)
	}
	n := r.NumPage()
	if n <= 0 {
		return nil, fmt.Errorf("nepraper: pdf page tree reports %d pages", n)
	}
	// THE PAGE COUNT IS THE DOCUMENT'S OWN CLAIM, NOT A MEASUREMENT, so it is
	// sanity-checked against the bytes actually present before it is used as
	// a make() capacity or a loop bound.
	//
	// /Count sits in the page tree and a malformed or hostile PDF can declare
	// any value it likes; a two-kilobyte file claiming 2,000,000,000 pages
	// would otherwise have this preallocate a PageText slice for all of them.
	// A PDF page cannot be represented in fewer than a few dozen bytes even
	// empty, so minPDFBytesPerPage is a deliberately generous floor: the
	// largest real document in this corpus is 331 MB across a few hundred
	// pages, orders of magnitude above the bound.
	if maxPages := len(pdfBytes) / minPDFBytesPerPage; n > maxPages {
		return nil, fmt.Errorf(
			"nepraper: pdf page tree claims %d pages but the document is only %d bytes, which cannot hold "+
				"more than %d; refusing to allocate against a self-declared count",
			n, len(pdfBytes), maxPages)
	}
	doc = &Doc{NumPages: n, Pages: make([]PageText, 0, n)}

	info := r.Trailer().Key("Info")
	if !info.IsNull() {
		doc.Creator = strings.TrimSpace(info.Key("Creator").Text())
		doc.Producer = strings.TrimSpace(info.Key("Producer").Text())
	}

	for i := 1; i <= n; i++ {
		pg := PageText{Number: i}
		p := r.Page(i)
		if !p.V.IsNull() {
			pg = PageFromSpans(i, pageSpans(p, i))
			pg.Text = plainText(p, pg.Lines)
		}
		doc.Pages = append(doc.Pages, pg)
	}
	return doc, nil
}

// PageFromSpans reconstructs a page from positioned spans: it groups them into
// lines, glues glyph runs back into cells, and derives the flat text.
//
// It is exported so that a caller holding a text layer captured elsewhere -- a
// trimmed test fixture, for instance -- goes through exactly the same
// reconstruction as ExtractText rather than a parallel implementation.
func PageFromSpans(num int, spans []Span) PageText {
	pg := PageText{Number: num, Spans: spans}
	pg.Lines = groupLines(spans)
	parts := make([]string, 0, len(pg.Lines))
	for _, l := range pg.Lines {
		parts = append(parts, l.Text())
	}
	pg.Text = strings.Join(parts, "\n")
	return pg
}

func safePrefix(b []byte, n int) string {
	if len(b) < n {
		n = len(b)
	}
	return string(b[:n])
}

// pageSpans collects positioned text spans, tolerating a panic from the PDF
// content interpreter on a malformed page: a bad page yields no spans rather
// than failing the whole document.
func pageSpans(p pdf.Page, num int) (spans []Span) {
	defer func() {
		if recover() != nil {
			spans = nil
		}
	}()
	c := p.Content()
	spans = make([]Span, 0, len(c.Text))
	for _, t := range c.Text {
		txt := strings.ReplaceAll(t.S, "\uFFFD", "")
		if strings.TrimSpace(txt) == "" {
			// NEPRA content streams are littered with bare "\n" and " "
			// showing operations used as cell separators, plus U+FFFD glyphs
			// the fonts do not map. They carry no text and their advance
			// widths are unreliable, so they are dropped here and cell
			// boundaries are recovered from geometry instead.
			continue
		}
		spans = append(spans, Span{Text: txt, X: t.X, Y: t.Y, W: t.W, Page: num})
	}
	return spans
}

// plainText prefers the library's own plain-text extraction (which handles
// inter-word spacing) and falls back to the reconstructed lines when the
// library errors out on a page.
func plainText(p pdf.Page, lines []Line) string {
	txt, err := func() (s string, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic: %v", r)
			}
		}()
		return p.GetPlainText(nil)
	}()
	if err == nil && strings.TrimSpace(txt) != "" {
		return txt
	}
	parts := make([]string, 0, len(lines))
	for _, l := range lines {
		parts = append(parts, l.Text())
	}
	return strings.Join(parts, "\n")
}

// groupLines buckets spans into lines by baseline, top to bottom, then orders
// each line left to right. Adjacent spans that abut without a gap are merged so
// that a cell split across content-stream operations reads as one token.
func groupLines(spans []Span) []Line {
	if len(spans) == 0 {
		return nil
	}
	idx := make([]int, len(spans))
	for i := range idx {
		idx[i] = i
	}
	// Group into lines by Y only. The within-line ordering — including the
	// X quantisation and the content-stream tiebreak that make a
	// zero-advance-width cell reconstruct correctly — is done by
	// sortLineSpans, which is where that logic and its rationale live.
	sort.SliceStable(idx, func(a, b int) bool {
		return spans[idx[a]].Y > spans[idx[b]].Y
	})

	var lines []Line
	var cur []Span
	curY := spans[idx[0]].Y
	flush := func() {
		if len(cur) == 0 {
			return
		}
		c := make([]Span, len(cur))
		copy(c, cur)
		sortLineSpans(c)
		lines = append(lines, Line{Page: c[0].Page, Y: curY, Spans: mergeAdjacent(c)})
		cur = cur[:0]
	}
	for _, i := range idx {
		s := spans[i]
		if len(cur) > 0 && absf(s.Y-curY) > lineTolerance {
			flush()
			curY = s.Y
		}
		if len(cur) == 0 {
			curY = s.Y
		}
		cur = append(cur, s)
	}
	flush()
	return lines
}

// glueGap is the largest horizontal gap, in points, between the right edge of
// one glyph and the left edge of the next that still counts as "same cell".
// Observed intra-cell gaps in NEPRA PERs are <= 0.2pt; the narrowest observed
// inter-cell gap is > 20pt, and the narrowest inter-word gap inside a prose
// paragraph is ~1.4pt, so 1.0 separates cells without splitting numbers.
const glueGap = 1.0

// xQuantum is the horizontal quantum, in points, used when ordering spans
// within a line. The narrowest genuine inter-glyph advance observed in a NEPRA
// PER is 2.4pt (a comma), so 1.0 collapses float noise between co-located
// glyphs without merging two real positions.
const xQuantum = 1.0

// sortLineSpans orders one line's spans left to right, breaking ties within
// xQuantum by the order the PDF content stream drew them.
//
// The content-stream tiebreak is LOAD-BEARING, not cosmetic. Some NEPRA tables
// (FY2024-25 Table 17, for one) draw every glyph of a cell at the same pen
// position with a zero advance width — 79.1% of spans in these documents have
// W == 0 — so the X coordinates of "1" "0" "." "2" "3" differ only by float
// noise, and in MEPCO's case they actually DESCEND, 476.74 down to 476.67.
// Sorting on raw X there scrambles the cell: 10.23 comes out as 32.01, a
// plausible-looking number that is simply wrong, and it would then be compared
// against the headline table's 30.67 as if it were data. Quantising X to
// xQuantum collapses that noise into one bucket and the stable stream order
// reproduces the cell as drawn.
func sortLineSpans(c []Span) {
	type keyed struct {
		bucket float64
		stream int
	}
	keys := make([]keyed, len(c))
	for i, s := range c {
		keys[i] = keyed{bucket: math.Round(s.X / xQuantum), stream: i}
	}
	ord := make([]int, len(c))
	for i := range ord {
		ord[i] = i
	}
	sort.Slice(ord, func(a, b int) bool {
		ka, kb := keys[ord[a]], keys[ord[b]]
		if ka.bucket != kb.bucket {
			return ka.bucket < kb.bucket
		}
		return ka.stream < kb.stream
	})
	tmp := make([]Span, len(c))
	for i, o := range ord {
		tmp[i] = c[o]
	}
	copy(c, tmp)
}

// mergeAdjacent concatenates glyph spans that touch horizontally, producing one
// span per word or numeric cell. NEPRA emits "3" "1" "4" "1" "9" "." "3" "0" as
// eight separate operations; without geometric gluing "31419.30" is never seen
// as a number, and with naive character-class gluing the whole row fuses into
// "31419.309704.0021,715.3".
func mergeAdjacent(in []Span) []Span {
	out := make([]Span, 0, len(in))
	for _, s := range in {
		if len(out) > 0 {
			prev := &out[len(out)-1]
			if s.X-prev.EndX() <= glueGap && s.X >= prev.X-glueGap {
				prev.Text += s.Text
				prev.W = s.EndX() - prev.X
				continue
			}
		}
		out = append(out, s)
	}
	for i := range out {
		out[i].Text = strings.TrimSpace(out[i].Text)
	}
	kept := out[:0]
	for _, s := range out {
		if s.Text != "" {
			kept = append(kept, s)
		}
	}
	return kept
}

func absf(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// ExtractTextFrom reads all of r and extracts it. Provided so callers holding a
// stream do not have to buffer it themselves; the PDF reader requires random
// access, so the bytes are read fully either way.
func ExtractTextFrom(r io.Reader) (*Doc, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("nepraper: read pdf: %w", err)
	}
	return ExtractText(b)
}
