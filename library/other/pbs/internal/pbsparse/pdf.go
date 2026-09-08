// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package pbsparse

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ledongthuc/pdf"
)

// ExtractPDFText pulls the text layer out of a PBS annexure or report PDF.
//
// Pure-Go extraction is viable on every PBS vintage: measured across seven real
// files spanning 2023-07-13 to 2026-09-03, every page extracted with zero
// errors and zero panics in 4-23ms, yielding 3,240-3,299 two-decimal numbers per
// weekly annexure. The 4MB files carry 11-15 image objects, but those are
// decorative and the text layer is complete, so no OCR and no external
// extractor is required. This is why the CLI needs no macOS PDFKit bridge.
func ExtractPDFText(data []byte) (text string, pages int, err error) {
	defer func() {
		// Some malformed PDFs panic inside the reader rather than returning an
		// error. Convert that into an error so one bad release cannot abort a
		// whole backfill.
		if r := recover(); r != nil {
			text, pages, err = "", 0, fmt.Errorf("pdf reader panicked: %v", r)
		}
	}()
	rd, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", 0, fmt.Errorf("open pdf: %w", err)
	}
	pages = rd.NumPage()
	var sb strings.Builder
	var okPages int
	for i := 1; i <= pages; i++ {
		p := rd.Page(i)
		if p.V.IsNull() {
			continue
		}
		t, perr := p.GetPlainText(nil)
		if perr != nil {
			continue
		}
		sb.WriteString(t)
		sb.WriteString("\n")
		okPages++
	}
	if okPages == 0 {
		return "", pages, fmt.Errorf("pdf has %d pages but no extractable text layer", pages)
	}
	return sb.String(), pages, nil
}

// ExtractPDFRows reconstructs table rows from a PDF's positioned text runs.
//
// PBS annexure PDFs emit ONE TEXT RUN PER GLYPH, so a naive read yields
// ["1","W","h","e","a","t",...] rather than cells. Rows are therefore rebuilt
// geometrically in two stages:
//
//  1. group glyphs into lines by Y (PDF Y grows upward, so a larger Y is an
//     earlier line);
//  2. split each line into cells wherever the horizontal gap between one
//     glyph's right edge and the next glyph's left edge exceeds a
//     font-relative threshold.
//
// Stage 2 is the part that cannot be skipped. Glyphs inside a number are
// contiguous while adjacent table columns are separated by real space, so the
// gap is the only signal that distinguishes "160.00" followed by "178.64" from
// the single token "160.00178.64". This is the same class of trap as reading a
// right-aligned numeric column by its left edge.
func ExtractPDFRows(data []byte) (rows [][]string, err error) {
	defer func() {
		if r := recover(); r != nil {
			rows, err = nil, fmt.Errorf("pdf reader panicked: %v", r)
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
		texts := p.Content().Text
		if len(texts) == 0 {
			continue
		}
		type glyph struct {
			x, y, w, size float64
			s             string
		}
		var gs []glyph
		for _, t := range texts {
			if t.S == "" {
				continue
			}
			gs = append(gs, glyph{x: t.X, y: t.Y, w: t.W, size: t.FontSize, s: t.S})
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

		var line []glyph
		flush := func() {
			if len(line) == 0 {
				return
			}
			var cells []string
			var cur strings.Builder
			for j, g := range line {
				if j > 0 {
					prev := line[j-1]
					gap := g.x - (prev.x + prev.w)
					size := g.size
					if size <= 0 {
						size = prev.size
					}
					if size <= 0 {
						size = 8
					}
					if gap > pdfCellGapRatio*size {
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
			if len(cells) > 0 {
				rows = append(rows, cells)
			}
			line = nil
		}
		for j, g := range gs {
			if j > 0 {
				d := gs[j-1].y - g.y
				if d > pdfLineTol || d < -pdfLineTol {
					flush()
				}
			}
			line = append(line, g)
		}
		flush()
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("pdf yielded no positioned text runs")
	}
	return rows, nil
}

const (
	// pdfLineTol is the vertical tolerance, in points, for treating two glyphs
	// as being on the same line. PBS annexure rows are well separated so a
	// small value is safe and avoids merging adjacent table rows.
	pdfLineTol = 2.0
	// pdfCellGapRatio is the horizontal gap, as a fraction of font size, that
	// separates one table cell from the next. Tuned against the 2026-09-03
	// release, where both an .xlsx and a .pdf exist so the reconstruction can
	// be validated cell-for-cell against a known-good grid.
	pdfCellGapRatio = 0.28
)

var (
	// reCityTok matches a city header token in PDF text, where the code may be
	// separated from the name by the extraction.
	reCityTok = regexp.MustCompile(`^([A-Za-z][A-Za-z .'-]*?)\s*\((\d{2})\)$`)
	// rePDFNumber matches a PBS price as rendered in the text layer. PBS uses
	// two decimals throughout; integers appear for round prices.
	rePDFNumber = regexp.MustCompile(`^-?[\d,]+(?:\.\d+)?$`)
)

// ParseAnnexurePDF parses the price appendix out of an annexure PDF.
//
// The strategy mirrors the xlsx path deliberately — find the repeated city
// header bands, then attribute each data row to the band above it — but the
// mechanics differ because a PDF has no merge ranges. Each band's horizontal
// geometry is learned from its own MIN/AVG/MAX header line, and every glyph on
// a data line is assigned to a column by its own centre. Gap-based
// tokenization alone is not sufficient: a long description overflows its
// column and interleaves with the UNIT column, which corrupts the label while
// leaving the numbers intact.
func ParseAnnexurePDF(data []byte, asOf string) (*Annexure, error) {
	lines, err := extractPDFLines(data)
	if err != nil {
		return nil, err
	}
	out := &Annexure{AsOf: asOf, Surface: SurfaceAppendixA,
		NationalAvg: map[string]Value{}, Derived: map[string]map[string]Value{}}

	type bandState struct {
		cities    []string
		codes     []string
		headers   []string
		model     *pdfColumnModel
		triplets  [][3]int
		nationalT int
		// cardinalityErr is set when the band's triplet count does not match
		// its city count. A band in that state contributes no rows.
		cardinalityErr error
	}
	var band *bandState
	bandIdx := 0
	bandErrSeen := map[int]bool{}
	var bandErrs []error
	citySet := map[string]bool{}
	itemSeen := map[string]bool{}
	var vals []Value

	lastItemNo := 0
	for li, l := range lines {
		cells := cellsOf(l)

		// GUARD 1: an explicit section marker ends the current band. After
		// Appendix-A's data, the annexure continues with Appendix-B, whose
		// tables restart their own numbering at 1 and cover a different, smaller
		// city set. Leaving the band open attributes Appendix-B rows to
		// Appendix-A cities: measured, an electrician's per-point wage was being
		// emitted as the price of beef in Bannu.
		for _, c := range cells {
			if reAppendixB.MatchString(CleanText(c)) {
				band = nil
				lastItemNo = 0
				break
			}
		}

		// A band starts at a line carrying two or more coded city headers.
		var cities, codes []string
		for _, c := range cells {
			t := CleanText(c)
			if key, code := NormalizeCity(t); key != "" && code != "" {
				cities = append(cities, key)
				codes = append(codes, code)
			}
		}
		if len(cities) >= 2 {
			nb := &bandState{cities: cities, codes: codes, headers: cells, nationalT: -1}
			// The statistic header line sits within the next few lines.
			for probe := li + 1; probe < len(lines) && probe <= li+4; probe++ {
				if m, ok := buildColumnModel(lines[probe]); ok {
					nb.model = m
					break
				}
			}
			if nb.model != nil {
				// Group value columns into MIN/AVG/MAX triplets. A new triplet
				// starts at every MIN, so a band whose trailing groups are
				// two- or four-wide with different sub-labels contributes no
				// spurious triplet.
				var trip [3]int
				have := 0
				for i, st := range nb.model.stat {
					switch st {
					case StatMin:
						trip = [3]int{i, -1, -1}
						have = 1
					case StatAvg:
						if have >= 1 {
							trip[1] = i
							have = 2
						}
					case StatMax:
						if have == 2 {
							trip[2] = i
							nb.triplets = append(nb.triplets, trip)
							have = 0
						}
					}
				}
				// A band may carry one triplet more than it has cities: the
				// published National Average price group. Claim it only when a
				// header on this band actually says so.
				if len(nb.triplets) == len(cities)+1 {
					for _, h := range cells {
						if reNationalH.MatchString(CleanText(h)) {
							nb.nationalT = len(cities)
							break
						}
					}
				}
				// CARDINALITY GUARD. Triplets are zipped to cities BY INDEX, so
				// a single MIN/AVG/MAX header that fails to tokenize (the gap
				// heuristic merging it with a neighbour) yields one triplet too
				// few and silently shifts every later city's prices onto its
				// neighbour — plausible values, wrong city, and the band-
				// disjointness check still passes. Refuse the band instead.
				want := len(nb.cities)
				if nb.nationalT >= 0 {
					want++
				}
				if len(nb.triplets) != want {
					nb.cardinalityErr = fmt.Errorf(
						"band at line %d resolved %d MIN/AVG/MAX triplets for %d cities (expected %d): refusing to attribute prices by position",
						li, len(nb.triplets), len(nb.cities), want)
				}
				band = nb
				bandIdx++
				lastItemNo = 0
			}
			continue
		}
		if band == nil || band.model == nil {
			continue
		}
		if band.cardinalityErr != nil {
			// Record the refusal once, then skip the band's rows entirely.
			if !bandErrSeen[bandIdx] {
				bandErrSeen[bandIdx] = true
				bandErrs = append(bandErrs, band.cardinalityErr)
			}
			continue
		}

		// Hybrid extraction: the label region is tokenized by horizontal gap
		// (which recovers clean descriptions), while the numeric region is
		// assigned by column (which is what keeps right-aligned values in the
		// right city).
		labelRegion, _ := band.model.splitRegions(l)
		labelToks := cellsOf(labelRegion)
		if len(labelToks) < 2 {
			continue
		}
		itemNoRaw := strings.TrimSpace(labelToks[0])
		if !reItemNo.MatchString(itemNoRaw) {
			continue
		}
		itemNo, _ := strconv.Atoi(itemNoRaw)
		// GUARD 2: Appendix-A item numbers ascend monotonically within a band.
		// A number that does not exceed the previous row's means a new section
		// has begun even if its heading was not recognised, so the band is
		// closed rather than allowed to absorb foreign rows. This is defence in
		// depth behind GUARD 1: either alone stops the leak, and relying on a
		// heading match alone would break silently if PBS retitled the section.
		if itemNo <= lastItemNo {
			band = nil
			lastItemNo = 0
			continue
		}
		label := CleanText(strings.Join(labelToks[1:], " "))
		desc, unit := splitLabelUnit(label)
		// A real item description contains letters. This rejects the
		// column-numbering row that PBS prints beneath each band's header
		// ("1 2 3 ... 24"), which otherwise parses as item 1 with a
		// description of "2 3" — and which, once ingested, makes the
		// monotonic-item-number guard close the band on the FIRST real data
		// row and drop the entire panel.
		if desc == "" || !hasLetter(desc) {
			continue
		}
		// A description long enough to overflow its column physically overlaps
		// the UNIT column in the source, so the two cannot be separated
		// geometrically. Such rows are FLAGGED rather than silently accepted:
		// the numbers are still exact, but the label is best-effort.
		descSuspect := unit == ""
		cols := band.model.assign(l)
		if len(cols) == 0 {
			continue
		}
		itemSeen[desc] = true
		lastItemNo = itemNo

		for ti, trip := range band.triplets {
			if ti == band.nationalT {
				if trip[1] >= 0 && trip[1] < len(cols) {
					if v := ParseValue(cols[trip[1]], cols[trip[1]] == ""); v.Present() {
						out.NationalAvg[desc] = v
					}
				}
				continue
			}
			if ti >= len(band.cities) {
				break
			}
			city, code := band.cities[ti], band.codes[ti]
			for si, st := range []Stat{StatMin, StatAvg, StatMax} {
				ci := trip[si]
				if ci < 0 || ci >= len(cols) {
					continue
				}
				raw := cols[ci]
				v := ParseValue(raw, raw == "")
				vals = append(vals, v)
				citySet[city] = true
				out.Rows = append(out.Rows, PriceRow{
					AsOf: asOf, Surface: SurfaceAppendixA, City: city, CityCode: code,
					ItemNo: itemNo, ItemDesc: desc, Unit: unit, Stat: st, Value: v,
					Block: bandIdx, DescSuspect: descSuspect,
				})
			}
		}
	}

	if len(out.Rows) == 0 {
		return nil, fmt.Errorf("annexure PDF for %s parsed to zero price rows: no repeated city header bands with a MIN/AVG/MAX column model were found in the text layer", asOf)
	}
	if len(bandErrs) > 0 {
		// A refused band means the release is INCOMPLETE, not merely smaller.
		// Returning rows without saying so would present a partial panel as whole.
		return nil, fmt.Errorf("annexure PDF for %s: %d of %d header bands could not be attributed safely: %v",
			asOf, len(bandErrs), bandIdx, bandErrs[0])
	}
	out.Blocks = bandIdx
	for c := range citySet {
		out.Cities = append(out.Cities, c)
	}
	sort.Strings(out.Cities)
	for i := range itemSeen {
		out.Items = append(out.Items, i)
	}
	sort.Strings(out.Items)
	out.Census = Census(vals)
	return out, nil
}

// hasLetter reports whether s contains at least one ASCII letter.
func hasLetter(s string) bool {
	for i := 0; i < len(s); i++ {
		if (s[i] >= 'a' && s[i] <= 'z') || (s[i] >= 'A' && s[i] <= 'Z') {
			return true
		}
	}
	return false
}

func isSentinelTok(s string) bool {
	v := ParseValue(s, false)
	return v.State == StateNA || v.State == StateZero
}

// knownUnits are the unit strings PBS renders in the UNIT column. Matching
// against a closed set is safer than a heuristic because several item
// descriptions legitimately end in a quantity ("... 190 gm Packet").
var knownUnits = []string{
	"20 Kg", "40 Kg", "10 kg", "1 Kg", "1 kg", "1 Ltr", "1 Dozen", "1 mtr",
	"Per Plate", "Per Cup", "Per Unit", "Per Litre", "Per Minute",
	"Pair", "Each", "MMBTU", "Unit", "Daily", "P/Point",
}

// splitLabelUnit separates a trailing unit token from an item description.
func splitLabelUnit(label string) (desc, unit string) {
	for _, u := range knownUnits {
		if strings.HasSuffix(label, " "+u) {
			return strings.TrimSpace(strings.TrimSuffix(label, u)), u
		}
		if strings.EqualFold(label, u) {
			return "", u
		}
	}
	return label, ""
}
