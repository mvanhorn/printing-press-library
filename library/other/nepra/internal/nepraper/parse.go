package nepraper

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
)

// ColumnRole says what a reconstructed table column means.
type ColumnRole int

const (
	// RoleOther is a column whose header did not identify it.
	RoleOther ColumnRole = iota
	// RoleReported is the DISCO's own reported figure.
	RoleReported
	// RoleTarget is the target NEPRA set or allowed in tariff.
	RoleTarget
	// RoleBreach is the breach of target, numeric in early years and a
	// qualitative label ("Far Away") from FY2020-21 on.
	RoleBreach
	// RoleFiscalYear is a column of a multi-year comparison table; the
	// column's PeriodFY says which year.
	RoleFiscalYear
)

func (r ColumnRole) String() string {
	switch r {
	case RoleReported:
		return "reported"
	case RoleTarget:
		return "target"
	case RoleBreach:
		return "breach"
	case RoleFiscalYear:
		return "fiscal_year"
	}
	return "other"
}

// Column is one reconstructed table column.
type Column struct {
	Index int
	// Header is the reconstructed header text, words joined with spaces.
	Header string
	// HeaderTokens is the header words as extracted, kept because header text
	// in these PDFs is assembled out of order and the joined string is not
	// always readable.
	HeaderTokens []string
	Role         ColumnRole
	// PeriodFY is the fiscal year this column reports, when its header carries
	// one. It is set independently of Role.
	PeriodFY string
	MinX     float64
	MaxX     float64
}

// Table is one reconstructed PER table, keyed on its caption rather than its
// number.
type Table struct {
	Label    string // "Table 5" -- recorded as provenance, never used as a key
	Caption  string // the caption line as extracted
	Page     int    // 1-indexed PDF page (not the printed page number)
	Metric   Metric
	Variant  string
	Columns  []Column
	Roster   []Entity // the entities this table actually lists, in table order
	HasWAvg  bool     // whether a weighted-average summary row was found
	RowCount int
	// ValueColumn is the column taken as the metric's own figure, or -1 when
	// no column could be identified and the first was used positionally.
	//
	// NOTE the unsafe zero value: a Table{} has ValueColumn 0, which READS as
	// "column 0 is the value column" when the sentinel for "unknown" is -1.
	// Every Table this package builds sets it explicitly, so construct one
	// through the parser rather than as a literal, and prefer
	// [Table.ValueColumnIndex] over comparing the field.
	ValueColumn int
	// ValueColumnBasis records how ValueColumn was chosen, so a caller can see
	// whether the choice was labelled or positional.
	ValueColumnBasis string
}

// ValueColumnIndex returns the identified value column and whether one was
// identified at all. It exists so a caller never has to know that -1 is the
// sentinel, and so a zero-valued Table cannot silently answer "column 0".
func (t Table) ValueColumnIndex() (int, bool) {
	if t.ValueColumn < 0 {
		return -1, false
	}
	return t.ValueColumn, true
}

// Provenance records exactly where an Observation came from.
type Provenance struct {
	ReportFY     string
	TableLabel   string
	TableCaption string
	Page         int
	RowY         float64
	ColumnHeader string
	ColumnIndex  int
	Variant      string
}

func (p Provenance) String() string {
	return fmt.Sprintf("%s %s (%q) p%d col%d %q variant=%s",
		p.ReportFY, p.TableLabel, p.TableCaption, p.Page, p.ColumnIndex, p.ColumnHeader, p.Variant)
}

// Extra is a published column that is not the metric's value, target or breach
// -- for instance the FY2014-15 complaints table's consumer counts. Kept so
// that no printed figure is silently discarded.
type Extra struct {
	Header string
	Value  Value
}

// Key identifies a single measurement: one metric, one entity, one fiscal year.
// It deliberately does NOT include the table or the variant, because two tables
// disagreeing about the same Key is the thing this package exists to surface.
type Key struct {
	PeriodFY string
	Entity   Entity
	Metric   Metric
}

func (k Key) String() string {
	return fmt.Sprintf("%s/%s/%s", k.PeriodFY, k.Entity, k.Metric)
}

// Observation is one figure as published, with its provenance.
//
// There is intentionally no "the value" accessor at report level: PeriodFY,
// Entity and Metric together do not identify a unique number in this corpus.
type Observation struct {
	ReportFY string // the fiscal year of the report the figure was printed in
	PeriodFY string // the fiscal year the figure describes
	Entity   Entity
	Metric   Metric
	Value    Value
	Target   Value // KindAbsent when the table publishes no target
	Breach   Value // KindAbsent when the table publishes no breach
	Extras   []Extra
	Prov     Provenance
}

// Key returns the Observation's conflict key.
func (o Observation) Key() Key {
	return Key{PeriodFY: o.PeriodFY, Entity: o.Entity, Metric: o.Metric}
}

// Note records something the parser saw but did not turn into an Observation:
// a dropped token, a table with no rows, a roster anomaly. Notes exist so that
// "the parser ignored it" is visible rather than silent.
type Note struct {
	Page  int
	Table string
	Text  string
}

// Report is everything one PER PDF says about the metrics this package knows.
type Report struct {
	FY         string
	DetectedFY string
	NumPages   int
	Creator    string
	Tables     []Table
	// Observations holds only real entities. The weighted-average row is NOT
	// in here; see WeightedAverages.
	Observations []Observation
	// WeightedAverages holds the "W. Av:" summary rows, kept separate so they
	// can never be mistaken for an eleventh DISCO.
	WeightedAverages []Observation
	Notes            []Note
}

// ErrNoDoc is returned by ParseReliability for a nil or empty document.
var ErrNoDoc = errors.New("nepraper: no extracted document to parse")

// maxRowGap is how many consecutive non-row lines the row scan tolerates inside
// a table before deciding the table has ended. NEPRA typesets narrative prose
// in a left column beside the FY2018-19 loss table, interleaving up to three
// prose lines between data rows, so the gap must be at least 4.
const maxRowGap = 5

// maxHeaderLines caps how far the header scan reaches past the table body. The
// FY2018-19 SAIFI table has seven lines of units, column numbering and chart
// furniture between its top data row and its real header.
const maxHeaderLines = 10

// headerLineMaxChars and headerLineMaxTokens keep narrative paragraphs out of
// the header region.
const (
	headerLineMaxChars  = 90
	headerLineMaxTokens = 20
	// headerTokenMaxChars rejects the glued prose blobs the FY2024-25 text
	// layer produces ("enforcesstringenttargetsforDISCOs,withthe...") as
	// header material. Real header words in this corpus are under 20 chars.
	headerTokenMaxChars = 30
)

// minTableRows is the fewest entity rows a caption must have adjacent to it
// before its observations are trusted. Every real DISCO table in this corpus
// has ten or more rows; a caption with two or three "rows" is a figure caption
// whose neighbourhood happens to contain a sentence naming some DISCOs, and
// parsing that produces figures that do not exist.
const minTableRows = 4

// headerAttachSlack is how far outside a column band, in points, a header word
// may sit and still be attached to that column. Header words are centred over
// wide columns and routinely overhang; data cells are not attached this way.
const headerAttachSlack = 30

// ParseReliability turns an extracted PER into per-entity, per-metric
// observations.
//
// fy is the report's own fiscal year, e.g. "FY2024-25"; it is used as the
// PeriodFY for headline tables and recorded on every Observation as ReportFY.
// If the document states a different fiscal year, that is recorded in
// Report.DetectedFY and as a Note -- never silently overridden.
func ParseReliability(doc *Doc, fy string) (*Report, error) {
	if doc == nil || len(doc.Pages) == 0 {
		return nil, ErrNoDoc
	}
	reportFY := NormalizeFY(fy)
	if reportFY == "" {
		return nil, errors.New("nepraper: ParseReliability requires the report's fiscal year")
	}
	r := &Report{
		FY:       reportFY,
		NumPages: doc.NumPages,
		Creator:  doc.Creator,
	}
	if got, ok := DetectFY(doc); ok {
		r.DetectedFY = got
		if got != reportFY {
			r.Notes = append(r.Notes, Note{Text: fmt.Sprintf(
				"caller said %s but the document's running header says %s; both recorded, neither overridden",
				reportFY, got)})
		}
	}

	boiler := boilerplateLines(doc)
	for pi := range doc.Pages {
		page := &doc.Pages[pi]
		classes := classifyLines(page.Lines, boiler)
		for i, c := range classes {
			if !c.isCaption {
				continue
			}
			tbl, obs, wavg, notes := parseTableAt(page, classes, i, reportFY)
			r.Notes = append(r.Notes, notes...)
			if tbl == nil {
				continue
			}
			r.Tables = append(r.Tables, *tbl)
			r.Observations = append(r.Observations, obs...)
			r.WeightedAverages = append(r.WeightedAverages, wavg...)
		}
	}
	return r, nil
}

// lineClass is the parser's view of one reconstructed line.
type lineClass struct {
	isCaption bool
	label     string
	caption   string
	isHeading bool
	heading   Metric
	// entity is set when the line reads as a table data row.
	entity    Entity
	entityIdx int // index into Line.Spans of the entity cell's first span
	entityEnd int // one past the entity cell's last span
	hasEntity bool
	// boilerplate marks a running header or footer that repeats across pages.
	boilerplate bool
	text        string
	tokens      int
}

var headingPattern = regexp.MustCompile(`^[0-9]{1,2}\.[0-9]{1,2}[.:]?\s`)

// boilerplateLines finds the running headers and footers that repeat across
// pages. They must be excluded from table reconstruction: NEPRA's running
// header sits directly above the top table row on several pages, and attaching
// "|PerformanceEvaluationReportofDistributionCompaniesFY2024-25|" to a column
// as if it were that column's heading destroys role detection.
func boilerplateLines(doc *Doc) map[string]bool {
	pages := map[string]map[int]bool{}
	for _, p := range doc.Pages {
		n := len(p.Lines)
		for i, l := range p.Lines {
			// Only the outermost lines of a page can be a running header or
			// footer. Without this positional guard the repeated fiscal-year
			// header of every five-year comparison table ("2016-17 2017-18
			// 2018-19 2019-20 2020-21") is itself seen as boilerplate and
			// dropped, which silently turns every comparison table into an
			// unlabelled one.
			if i >= boilerplateEdge && i < n-boilerplateEdge {
				continue
			}
			if fiscalYearHeavy(p.Lines[i]) {
				continue
			}
			k := normalizeCaption(l.Text())
			if len(k) < 8 {
				continue
			}
			if pages[k] == nil {
				pages[k] = map[int]bool{}
			}
			pages[k][p.Number] = true
		}
	}
	out := map[string]bool{}
	threshold := len(doc.Pages) / 3
	if threshold < 3 {
		threshold = 3
	}
	for k, seen := range pages {
		if len(seen) >= threshold {
			out[k] = true
		}
	}
	return out
}

// boilerplateEdge is how many lines at each end of a page may be a running
// header or footer.
const boilerplateEdge = 2

// fiscalYearHeavy reports whether at least half a line's cells are fiscal-year
// labels, which makes it a comparison table's header rather than page furniture.
func fiscalYearHeavy(l Line) bool {
	years := 0
	for _, s := range l.Spans {
		if _, ok := FiscalYearFromHeader(s.Text); ok {
			years++
		}
	}
	return years > 0 && years*2 >= len(l.Spans)
}

func classifyLines(lines []Line, boiler map[string]bool) []lineClass {
	out := make([]lineClass, len(lines))
	for i, l := range lines {
		txt := l.Text()
		c := lineClass{text: txt, tokens: len(l.Spans)}
		if boiler[normalizeCaption(txt)] {
			c.boilerplate = true
			out[i] = c
			continue
		}
		texts := make([]string, len(l.Spans))
		for si, s := range l.Spans {
			texts[si] = s.Text
		}
		if label, body, ok := parseCaptionInLine(texts); ok {
			c.isCaption = true
			c.label = label
			c.caption = body
		}
		if headingPattern.MatchString(txt) {
			if m, ok := MetricFromCaption(txt); ok {
				c.isHeading = true
				c.heading = m
			}
		}
		if e, st, en, ok := LookupEntitySpans(texts); ok {
			c.entity = e
			c.entityIdx = st
			c.entityEnd = en
			c.hasEntity = true
		}
		out[i] = c
	}
	return out
}

// dataRow is a candidate table row.
type dataRow struct {
	entity Entity
	// idx is the row's index into the page's Lines, needed to place the header
	// scan relative to the rows that actually survived filtering.
	idx  int
	line Line
	// tokens are the cells to the right of the entity name.
	tokens []Span
	y      float64
	xEnt   float64
	endEnt float64
}

// parseTableAt reconstructs the table belonging to the caption at index ci.
//
// NEPRA is not consistent about where the caption sits: FY2014-15 puts it above
// the table, every other year in this corpus puts it below. Rather than guess
// per year, both directions are scanned and the side with more entity rows
// wins; a table with rows on both sides is recorded as a Note.
func parseTableAt(page *PageText, classes []lineClass, ci int, reportFY string) (*Table, []Observation, []Observation, []Note) {
	var notes []Note
	up := scanRows(page, classes, ci, -1)
	down := scanRows(page, classes, ci, +1)

	rows, dir := up, -1
	if len(down) > len(up) {
		rows, dir = down, +1
	}
	label := classes[ci].label
	caption := classes[ci].caption
	if len(up) >= 3 && len(down) >= 3 {
		notes = append(notes, Note{Page: page.Number, Table: label, Text: fmt.Sprintf(
			"caption has %d entity rows above and %d below; took the %s side",
			len(up), len(down), map[int]string{-1: "upper", 1: "lower"}[dir])})
	}
	if len(rows) < minTableRows {
		notes = append(notes, Note{Page: page.Number, Table: label, Text: fmt.Sprintf(
			"caption has only %d entity row(s) adjacent to it; no observations emitted", len(rows))})
		return nil, nil, nil, notes
	}

	rows, dropped := filterRowsByEntityColumn(rows)
	for _, d := range dropped {
		notes = append(notes, Note{Page: page.Number, Table: label, Text: fmt.Sprintf(
			"line at y=%.2f mentions %s but its name cell is at x=%.1f, far from the table's "+
				"entity column; treated as prose, not a row", d.y, d.entity, d.xEnt)})
	}
	if len(rows) < minTableRows {
		notes = append(notes, Note{Page: page.Number, Table: label, Text: fmt.Sprintf(
			"only %d entity row(s) survived filtering, below the %d-row minimum for a real "+
				"DISCO table; treated as a figure caption beside prose, no observations emitted",
			len(rows), minTableRows)})
		return nil, nil, nil, notes
	}

	// The header scan must start from the outermost row that SURVIVED
	// filtering. Starting from the raw scan's last row silently skips the real
	// header whenever a header line was mistaken for a row and then dropped.
	hdrFrom := headerStart(rows, dir, ci)

	bands, bandNotes := deriveBands(rows)
	for _, n := range bandNotes {
		notes = append(notes, Note{Page: page.Number, Table: label, Text: n})
	}
	if len(bands) == 0 {
		notes = append(notes, Note{Page: page.Number, Table: label,
			Text: "no numeric columns could be reconstructed; no observations emitted"})
		return nil, nil, nil, notes
	}

	headerTokens := collectHeaderTokens(page, classes, hdrFrom, dir)
	cols := buildColumns(bands, headerTokens)

	metric, ok := MetricFromCaption(caption)
	if !ok {
		// Fall back to the nearest section heading in scan direction.
		if m, found := nearestHeading(classes, ci, dir); found {
			metric = m
		} else {
			notes = append(notes, Note{Page: page.Number, Table: label, Text: fmt.Sprintf(
				"caption %q matches no known metric and no section heading was found; recorded as %s",
				caption, MetricUnknown)})
		}
	}

	variant, ok := VariantFromCaption(caption)
	if !ok {
		if countFiscalYearColumns(cols) >= 2 {
			variant = VariantComparison
		} else {
			variant = VariantHeadline
		}
	}

	tbl := &Table{
		Label:       label,
		Caption:     caption,
		Page:        page.Number,
		Metric:      metric,
		Variant:     variant,
		Columns:     cols,
		RowCount:    len(rows),
		ValueColumn: -1,
	}

	// Cells are assigned for EVERY row first, because the fused-cell check
	// below uses each column's other rows as its control population.
	rowCells := make([][]string, len(rows))
	for i, row := range rows {
		cells, drops := assignCells(row, bands)
		rowCells[i] = cells
		for _, d := range drops {
			notes = append(notes, Note{Page: page.Number, Table: label, Text: fmt.Sprintf(
				"row %s: token %q at x=%.1f falls outside every column band; dropped "+
					"(chart label or stray glyph)", row.entity, d.Text, d.X)})
		}
	}
	fused := flagFusedCells(rowCells, len(bands))
	for i, row := range rows {
		for j, reason := range fused[i] {
			notes = append(notes, Note{Page: page.Number, Table: label, Text: fmt.Sprintf(
				"row %s column %d: %s", row.entity, j, reason)})
		}
	}

	var obs, wavg []Observation
	for i, row := range rows {
		if row.entity == EntityWeightedAverage {
			tbl.HasWAvg = true
		} else {
			tbl.Roster = append(tbl.Roster, row.entity)
		}
		rowObs := buildObservations(row, rowCells[i], cols, tbl, reportFY, fused[i])
		if row.entity == EntityWeightedAverage {
			wavg = append(wavg, rowObs...)
		} else {
			obs = append(obs, rowObs...)
		}
	}
	if extra := rosterAnomaly(tbl.Roster); extra != "" {
		notes = append(notes, Note{Page: page.Number, Table: label, Text: extra})
	}
	if tbl.ValueColumn < 0 && tbl.Variant != VariantComparison {
		var hdrs []string
		for _, c := range cols {
			hdrs = append(hdrs, fmt.Sprintf("col%d=%q", c.Index, c.Header))
		}
		notes = append(notes, Note{Page: page.Number, Table: label, Text: fmt.Sprintf(
			"%s (%s)", tbl.ValueColumnBasis, strings.Join(hdrs, ", "))})
	}
	return tbl, obs, wavg, notes
}

// scanRows walks away from the caption in direction dir collecting entity rows.
// It returns the rows (in page order) and the index at which the header scan
// should continue.
// headerStart returns the line index at which the header scan begins.
//
// Where the header sits depends on where the caption sits. With the caption
// below its table (every year in this corpus but FY2014-15) the header is above
// the topmost row. With the caption above its table the header is between the
// caption and the first row, so the scan starts just past the caption -- not
// past the last row, which would land in the paragraph underneath and read
// "Table 2 contains analysis of the complaints data based on two parameters" as
// the column headings.
func headerStart(rows []dataRow, dir, captionIdx int) int {
	if dir > 0 {
		return captionIdx + 1
	}
	if len(rows) == 0 {
		return captionIdx - 1
	}
	outer := rows[0].idx
	for _, r := range rows {
		if r.idx < outer {
			outer = r.idx
		}
	}
	return outer - 1
}

func scanRows(page *PageText, classes []lineClass, ci, dir int) []dataRow {
	var rows []dataRow
	gap := 0
	i := ci + dir
	for i >= 0 && i < len(classes) {
		c := classes[i]
		if c.isCaption {
			break
		}
		if c.isHeading && len(rows) > 0 {
			break
		}
		if c.hasEntity {
			l := page.Lines[i]
			ent := l.Spans[c.entityIdx]
			rows = append(rows, dataRow{
				entity: c.entity,
				idx:    i,
				line:   l,
				tokens: l.Spans[c.entityEnd:],
				y:      l.Y,
				xEnt:   ent.X,
				endEnt: l.Spans[c.entityEnd-1].EndX(),
			})
			gap = 0
		} else {
			if strings.TrimSpace(c.text) != "" {
				gap++
			}
			if gap > maxRowGap && len(rows) > 0 {
				break
			}
		}
		i += dir
	}
	if dir < 0 {
		// rows were collected bottom-up; restore page order.
		for a, b := 0, len(rows)-1; a < b; a, b = a+1, b-1 {
			rows[a], rows[b] = rows[b], rows[a]
		}
	}
	return rows
}

// filterRowsByEntityColumn discards lines that merely mention a DISCO in prose.
// A real table row's name cell sits in the table's entity column; a prose
// mention sits wherever the sentence put it. Rows are also de-duplicated, since
// a caption's neighbourhood can contain both the table row and a sentence.
func filterRowsByEntityColumn(rows []dataRow) (kept []dataRow, dropped []dataRow) {
	if len(rows) < 2 {
		return rows, nil
	}
	xs := make([]float64, len(rows))
	for i, r := range rows {
		xs[i] = r.xEnt
	}
	med := median(xs)
	const slack = 25.0
	seen := map[Entity]int{}
	for _, r := range rows {
		if math.Abs(r.xEnt-med) > slack || len(r.tokens) == 0 {
			dropped = append(dropped, r)
			continue
		}
		if prev, ok := seen[r.entity]; ok {
			// Keep whichever candidate sits closer to the entity column.
			if math.Abs(r.xEnt-med) < math.Abs(kept[prev].xEnt-med) {
				dropped = append(dropped, kept[prev])
				kept[prev] = r
			} else {
				dropped = append(dropped, r)
			}
			continue
		}
		seen[r.entity] = len(kept)
		kept = append(kept, r)
	}
	return kept, dropped
}

func median(xs []float64) float64 {
	c := make([]float64, len(xs))
	copy(c, xs)
	sort.Float64s(c)
	n := len(c)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return c[n/2]
	}
	return (c[n/2-1] + c[n/2]) / 2
}

// band is a reconstructed column's horizontal extent.
type band struct {
	lo, hi float64
	minX   float64
	maxX   float64
}

// deriveBands recovers the column geometry from the data rows themselves.
//
// The modal token count across rows fixes the number of columns; rows with that
// many tokens fix each column's X extent; the boundaries are the midpoints
// between neighbouring columns. This is threshold-free in the interior, which
// matters because intra-cell gaps (a cell drawn as "15.3" then "9" 18pt later)
// and inter-column gaps (40-74pt) are not cleanly separable by a single
// constant.
func deriveBands(rows []dataRow) ([]band, []string) {
	var notes []string
	counts := map[int]int{}
	for _, r := range rows {
		counts[len(r.tokens)]++
	}
	m, best := 0, 0
	for k, v := range counts {
		if k == 0 {
			continue
		}
		if v > best || (v == best && k < m) {
			m, best = k, v
		}
	}
	if m == 0 {
		return nil, []string{"no row carried any cell to the right of its name"}
	}
	mins := make([]float64, m)
	maxs := make([]float64, m)
	for j := 0; j < m; j++ {
		mins[j] = math.Inf(1)
		maxs[j] = math.Inf(-1)
	}
	used := 0
	for _, r := range rows {
		if len(r.tokens) != m {
			continue
		}
		used++
		for j, t := range r.tokens {
			mins[j] = math.Min(mins[j], t.X)
			maxs[j] = math.Max(maxs[j], t.X)
		}
	}
	if used == 0 {
		return nil, []string{"internal: modal token count matched no row"}
	}
	if used < len(rows) {
		notes = append(notes, fmt.Sprintf(
			"column geometry derived from the %d of %d rows with the modal %d cells; "+
				"the remaining rows had cells split across glyph runs and were re-assembled by band",
			used, len(rows), m))
	}
	// Sanity: the modal ordinals must be strictly increasing in X.
	for j := 1; j < m; j++ {
		if mins[j] <= maxs[j-1] {
			notes = append(notes, fmt.Sprintf(
				"columns %d and %d overlap in x (%.1f..%.1f vs %.1f..%.1f); "+
					"cell assignment for this table is unreliable",
				j-1, j, mins[j-1], maxs[j-1], mins[j], maxs[j]))
		}
	}
	bands := make([]band, m)
	for j := 0; j < m; j++ {
		b := band{minX: mins[j], maxX: maxs[j]}
		if j == 0 {
			pad := 12.0
			if m > 1 {
				pad = math.Min(pad, math.Max(4, (mins[1]-maxs[0])/2))
			}
			b.lo = mins[0] - pad
		} else {
			b.lo = (maxs[j-1] + mins[j]) / 2
		}
		if j == m-1 {
			pad := 20.0
			if m > 1 {
				pad = clamp((mins[m-1]-maxs[m-2])/2, 8, 40)
			}
			b.hi = maxs[m-1] + pad
		} else {
			b.hi = (maxs[j] + mins[j+1]) / 2
		}
		bands[j] = b
	}
	return bands, notes
}

func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}

// assignCells buckets a row's tokens into the column bands, concatenating
// tokens that share a band. Tokens outside every band are returned so the
// caller can record them; they are never folded into a neighbouring cell.
// assignCells places a row's tokens into column bands and glues each band's
// tokens into one cell.
//
// The glue is deliberately empty: NEPRA emits one text-showing operation per
// GLYPH, so a single published cell routinely arrives as several tokens
// ("851"+"6"+"."+"45" -> 8516.45) and any separator would destroy it.
//
// The risk this creates is that two genuinely different cells landing in one
// band fuse into a plausible number — 3547 and 1182 becoming 35471182 as a
// KindNumeric with no flag, since the two-decimal-point rule only catches
// decimals. That cannot be guarded geometrically. MEASURED over all three
// fixtures: the largest LEGITIMATE gap between consecutive tokens inside one
// band is 35.88pt ("13469.5" -> "5"), while the FY2018-19 chart-axis label
// that would have fused sat 38.8pt away — about 3pt of headroom, and the
// nearest legitimate gaps are 30.36 and 24.84pt. A gap threshold would break
// real reconstructions before it caught a fusion.
//
// The reason the gaps are so large is that 79.1% of spans in these PDFs carry
// an advance width of ZERO, so EndX == X and a "gap" is really the distance
// between glyph START positions, which grows with the preceding token's
// length. Span.W is not the reliable discriminator its own doc comment claims.
//
// So the guard is statistical instead, and lives in [flagFusedCells]: a fused
// cell has far more digits than any other cell in its own column, which the
// table's other rows measure for us.
func assignCells(row dataRow, bands []band) ([]string, []Span) {
	parts := make([][]string, len(bands))
	var dropped []Span
	for _, t := range row.tokens {
		idx := -1
		for j, b := range bands {
			if t.X >= b.lo && t.X <= b.hi {
				idx = j
				break
			}
		}
		if idx < 0 {
			dropped = append(dropped, t)
			continue
		}
		parts[idx] = append(parts[idx], t.Text)
	}
	cells := make([]string, len(bands))
	for j := range parts {
		cells[j] = strings.Join(parts[j], "")
	}
	return cells, dropped
}

// collectHeaderTokens gathers the header words sitting past the table body.
// Pure numbers are excluded: chart axis labels and NEPRA's "(1) (2) (3)"
// column-numbering row live in the same region and would otherwise be attached
// to columns as if they were headers.
func collectHeaderTokens(page *PageText, classes []lineClass, from, dir int) []Span {
	var out []Span
	lines := 0
	for i := from; i >= 0 && i < len(classes) && lines < maxHeaderLines; i += dir {
		c := classes[i]
		if c.isCaption || c.hasEntity {
			break
		}
		if strings.TrimSpace(c.text) == "" || c.boilerplate {
			continue
		}
		if len(c.text) > headerLineMaxChars || c.tokens > headerLineMaxTokens ||
			hasLongToken(page.Lines[i].Spans) {
			break
		}
		lines++
		for _, s := range page.Lines[i].Spans {
			if _, isFY := FiscalYearFromHeader(s.Text); !isFY {
				if ParseFigure(s.Text).IsNumeric() {
					continue
				}
			}
			out = append(out, s)
		}
		if c.isHeading {
			break
		}
	}
	return out
}

// hasLongToken reports whether a line contains a glued prose blob, which marks
// it as narrative rather than header material.
func hasLongToken(spans []Span) bool {
	for _, s := range spans {
		if len(s.Text) > headerTokenMaxChars {
			return true
		}
	}
	return false
}

// buildColumns attaches header words to bands and reads each column's role.
func buildColumns(bands []band, headers []Span) []Column {
	texts := make([][]string, len(bands))
	fys := make([]string, len(bands))
	for _, s := range headers {
		center := s.X + s.W/2
		idx, bestDist := -1, math.Inf(1)
		for j, b := range bands {
			var d float64
			switch {
			case center < b.lo:
				d = b.lo - center
			case center > b.hi:
				d = center - b.hi
			}
			if d < bestDist {
				idx, bestDist = j, d
			}
		}
		if idx < 0 || bestDist > headerAttachSlack {
			continue
		}
		texts[idx] = append(texts[idx], s.Text)
		if fy, ok := FiscalYearFromHeader(s.Text); ok && fys[idx] == "" {
			fys[idx] = fy
		}
	}
	cols := make([]Column, len(bands))
	for j := range bands {
		h := strings.Join(texts[j], " ")
		cols[j] = Column{
			Index:    j,
			Header:   h,
			MinX:     bands[j].minX,
			MaxX:     bands[j].maxX,
			PeriodFY: fys[j],
		}
		cols[j].HeaderTokens = append([]string(nil), texts[j]...)
	}
	assignRoles(cols)
	return cols
}

// roleKeywords maps a header word to the role it argues for.
//
// Roles are assigned by counting these words across ALL columns and then
// giving each role to its strongest column, rather than by testing one
// column's header in isolation. That matters because NEPRA sets narrative
// prose in a left column that horizontally overlaps the table: the FY2021-22
// loss table's "Actual Reported (%)" column also picks up the sentence
// "...always give strict targets regarding T&D Losses...", and an
// isolated test that looks for "target" first labels the reported column as
// the target column and reads the whole table one column across.
var roleKeywords = map[string]ColumnRole{
	"breach":    RoleBreach,
	"target":    RoleTarget,
	"targets":   RoleTarget,
	"allowed":   RoleTarget,
	"limit":     RoleTarget,
	"reported":  RoleReported,
	"actual":    RoleReported,
	"reporting": RoleReported,
}

// assignRoles fills in Column.Role for a headline table.
func assignRoles(cols []Column) {
	score := make([]map[ColumnRole]int, len(cols))
	for j := range cols {
		score[j] = map[ColumnRole]int{}
		for _, w := range cols[j].HeaderTokens {
			n := normalizeCaption(w)
			if r, ok := roleKeywords[n]; ok {
				score[j][r]++
			}
		}
	}
	taken := make([]bool, len(cols))
	// Breach first (its keyword is unambiguous), then reported, then target.
	for _, role := range []ColumnRole{RoleBreach, RoleReported, RoleTarget} {
		best, bestScore := -1, 0
		for j := range cols {
			if taken[j] {
				continue
			}
			if score[j][role] > bestScore {
				best, bestScore = j, score[j][role]
			}
		}
		if best >= 0 {
			cols[best].Role = role
			taken[best] = true
		}
	}
	for j := range cols {
		if taken[j] {
			continue
		}
		if cols[j].PeriodFY != "" {
			cols[j].Role = RoleFiscalYear
		} else {
			cols[j].Role = RoleOther
		}
	}
}

// countFiscalYearColumns counts columns carrying a fiscal-year header,
// regardless of the role words their header may also have picked up.
func countFiscalYearColumns(cols []Column) int {
	n := 0
	for _, c := range cols {
		if c.PeriodFY != "" {
			n++
		}
	}
	return n
}

// buildObservations turns one row's cells into Observations.
//
// A multi-year comparison table yields one Observation per fiscal-year column,
// each carrying that column's own PeriodFY -- which is what makes a five-year
// table's 2024-25 column comparable with the headline table, and what surfaces
// the FY2024-25 MEPCO disagreement.
func buildObservations(row dataRow, cells []string, cols []Column, tbl *Table, reportFY string, fused map[int]string) []Observation {
	// figure reads a cell, demoting it to KindUnverified when the fused-cell
	// check flagged it. A cell that may be two cells glued together must
	// never reach a caller as a number.
	figure := func(j int) Value {
		if reason, bad := fused[j]; bad {
			return Unverified(cells[j], reason)
		}
		return ParseFigure(cells[j])
	}
	prov := func(c Column) Provenance {
		return Provenance{
			ReportFY:     reportFY,
			TableLabel:   tbl.Label,
			TableCaption: tbl.Caption,
			Page:         tbl.Page,
			RowY:         row.y,
			ColumnHeader: c.Header,
			ColumnIndex:  c.Index,
			Variant:      tbl.Variant,
		}
	}
	// Comparison mode treats EVERY fiscal-year column as a value for its own
	// year. Entering it on the column count alone was unsafe: a HEADLINE
	// table whose "Reported Figure FY 2024-25" and "Target by NEPRA
	// FY 2024-25" headers both carry the year has two fiscal-year columns and
	// would emit the TARGET as a VALUE under the same Key — manufacturing a
	// conflict between the actual figure and the allowed-in-tariff figure,
	// which are different quantities that must never be compared. It also
	// wiped Target and Breach to Absent in the process.
	//
	// Table 05 of FY2024-25 is one glyph-gluing accident away from this: its
	// col1 header already resolves to FY2024-25, and the table stays in
	// headline mode only because a free-floating "FY"/"2024-25" header pair
	// straddles the column band boundary. So the table's own declared variant
	// is required as well, and any column already identified as a target or a
	// breach is skipped rather than read as a value.
	if countFiscalYearColumns(cols) >= 2 && tbl.Variant == VariantComparison {
		tbl.ValueColumnBasis = "multi-year comparison table: every fiscal-year column is a value"
		var out []Observation
		for j, c := range cols {
			if c.PeriodFY == "" {
				continue
			}
			if c.Role == RoleTarget || c.Role == RoleBreach {
				tbl.ValueColumnBasis += fmt.Sprintf("; column %d %q is a %s, not a value, and was skipped",
					c.Index, c.Header, c.Role)
				continue
			}
			v := figure(j)
			if v.Kind == KindAbsent {
				continue
			}
			out = append(out, Observation{
				ReportFY: reportFY,
				PeriodFY: c.PeriodFY,
				Entity:   row.entity,
				Metric:   tbl.Metric,
				Value:    v,
				Target:   Absent(),
				Breach:   Absent(),
				Prov:     prov(c),
			})
		}
		return out
	}

	valIdx, tgtIdx, brIdx := selectColumns(cols, tbl)
	obs := Observation{
		ReportFY: reportFY,
		PeriodFY: reportFY,
		Entity:   row.entity,
		Metric:   tbl.Metric,
		Value:    Absent(),
		Target:   Absent(),
		Breach:   Absent(),
	}
	if valIdx >= 0 {
		obs.Value = figure(valIdx)
		obs.Prov = prov(cols[valIdx])
	} else {
		obs.Value = Value{Kind: KindAbsent, Reason: tbl.ValueColumnBasis}
		obs.Prov = prov(Column{Index: -1, Header: tbl.ValueColumnBasis})
	}
	if tgtIdx >= 0 {
		obs.Target = figure(tgtIdx)
	}
	if brIdx >= 0 {
		obs.Breach = figure(brIdx)
	}
	for j, c := range cols {
		if j == valIdx || j == tgtIdx || j == brIdx {
			continue
		}
		v := figure(j)
		if v.Kind == KindAbsent {
			continue
		}
		obs.Extras = append(obs.Extras, Extra{Header: c.Header, Value: v})
	}
	return []Observation{obs}
}

// selectColumns decides which reconstructed column holds the metric's own
// figure, which holds the target and which holds the breach.
//
// The order of preference is deliberate. A header that says "Reported" or
// "Actual" is trusted first. Failing that, the column whose header restates the
// table's caption wins -- FY2024-25's fault-rate table has columns for line
// length, fault count and fault rate, and only the caption match picks the
// rate. Failing that, a "Total" column wins, which is what the safety table
// needs (its first two columns are employee and public fatalities, and only the
// third is the figure the rest of the report quotes). Only if all of that fails
// is the first column used positionally, and that is recorded as such rather
// than presented as a labelled reading.
func selectColumns(cols []Column, tbl *Table) (valIdx, tgtIdx, brIdx int) {
	valIdx, tgtIdx, brIdx = -1, -1, -1
	for j, c := range cols {
		switch c.Role {
		case RoleReported:
			if valIdx < 0 {
				valIdx = j
				tbl.ValueColumnBasis = "header says reported/actual"
			}
		case RoleTarget:
			if tgtIdx < 0 {
				tgtIdx = j
			}
		case RoleBreach:
			if brIdx < 0 {
				brIdx = j
			}
		}
	}
	defer func() { tbl.ValueColumn = valIdx }()
	if valIdx >= 0 {
		return valIdx, tgtIdx, brIdx
	}

	// Remaining candidates: everything that is not the target or the breach.
	var cand []int
	for j := range cols {
		if j != tgtIdx && j != brIdx {
			cand = append(cand, j)
		}
	}
	if len(cand) == 0 {
		return -1, tgtIdx, brIdx
	}
	if len(cand) == 1 {
		valIdx = cand[0]
		if len(cols) == 1 {
			tbl.ValueColumnBasis = "single-column table"
		} else {
			tbl.ValueColumnBasis = "sole column that is neither target nor breach"
		}
		return valIdx, tgtIdx, brIdx
	}

	core := captionCore(tbl.Caption)
	if core == "" {
		return -1, tgtIdx, brIdx
	}
	// First try: a column header that restates the caption. FY2024-25's
	// fault-rate table has columns for line length, fault count and fault
	// rate, and only this test picks the rate.
	best, bestScore := -1, 0
	for _, j := range cand {
		h := normalizeCaption(cols[j].Header)
		if h == "" {
			continue
		}
		score := 0
		switch {
		case strings.Contains(h, core):
			score = len(core)
		case strings.Contains(core, h):
			score = len(h)
		}
		if score > bestScore {
			best, bestScore = j, score
		}
	}
	if best >= 0 {
		tbl.ValueColumnBasis = "column header restates the table caption"
		valIdx = best
		return valIdx, tgtIdx, brIdx
	}

	// Second try: how many of a column's header words appear in the caption.
	// This is what separates "No. of Consumers made complaints about the
	// voltage" from "Total No. of Consumers in DISCO" under a caption that
	// says "No. of Consumer Complaints made about Nominal Voltages".
	best, bestScore = -1, 0
	ties := 0
	for _, j := range cand {
		score := 0
		for _, w := range cols[j].HeaderTokens {
			n := normalizeCaption(w)
			if len(n) < 4 {
				continue
			}
			if strings.Contains(core, n) {
				score++
			}
		}
		switch {
		case score > bestScore:
			best, bestScore, ties = j, score, 1
		case score == bestScore && score > 0:
			ties++
		}
	}
	if ties > 1 {
		// Two columns argue equally well for being the metric's own figure --
		// FY2014-15's complaints table has three columns whose headers all say
		// "complaints". Picking one would present a coin flip as a reading.
		tbl.ValueColumnBasis = fmt.Sprintf(
			"ambiguous: %d columns match the caption equally well; value left absent "+
				"and all columns recorded as extras", ties)
		return -1, tgtIdx, brIdx
	}
	if best >= 0 && bestScore > 0 {
		tbl.ValueColumnBasis = fmt.Sprintf(
			"%d of the column's header words appear in the caption", bestScore)
		valIdx = best
		return valIdx, tgtIdx, brIdx
	}

	// Nothing identified the metric's own figure. Rather than pick a column
	// positionally and present a guess as a reading, the row is recorded with
	// an absent value and every column preserved in Extras.
	tbl.ValueColumnBasis = "unidentifiable: no column header names the metric; " +
		"value left absent and all columns recorded as extras"
	return -1, tgtIdx, brIdx
}

// captionCore strips the "Table 14 :" prefix from a caption, leaving the phrase
// that names the metric.
func captionCore(caption string) string {
	n := normalizeCaptionKey(caption)
	if m := captionPattern.FindString(n); m != "" {
		n = n[len(m):]
	}
	return normalizeCaption(n)
}

// nearestHeading finds the section heading that governs a table whose caption
// is only a bare number ("TABLE 10"), as FY2014-15 through FY2019-20 print
// them.
//
// It searches on the same side of the caption as the table's rows FIRST. That
// direction matters: FY2019-20 puts the complaints table and the safety table
// on one page, each with its caption underneath, and searching the wrong way
// labels the complaints figures as fatalities.
func nearestHeading(classes []lineClass, ci, dir int) (Metric, bool) {
	for i := ci + dir; i >= 0 && i < len(classes); i += dir {
		if classes[i].isHeading {
			return classes[i].heading, true
		}
	}
	for i := ci - dir; i >= 0 && i < len(classes); i -= dir {
		if classes[i].isHeading {
			return classes[i].heading, true
		}
	}
	return MetricUnknown, false
}

// rosterAnomaly reports a per-table roster that differs from the canonical ten.
// The roster genuinely varies inside a single report -- FY2014-15's complaints
// table lists twelve entities while its SAIFI chart lists ten -- so this is
// recorded, not corrected.
func rosterAnomaly(got []Entity) string {
	if len(got) == 0 {
		return ""
	}
	inGot := map[Entity]bool{}
	for _, e := range got {
		inGot[e] = true
	}
	var extra, missing []string
	for _, e := range got {
		if !InRoster(e) {
			extra = append(extra, string(e))
		}
	}
	for _, e := range roster {
		if !inGot[e] {
			missing = append(missing, string(e))
		}
	}
	if len(extra) == 0 && len(missing) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "per-table roster is %d entities", len(got))
	if len(extra) > 0 {
		fmt.Fprintf(&b, "; beyond the canonical ten: %s", strings.Join(extra, ", "))
	}
	if len(missing) > 0 {
		fmt.Fprintf(&b, "; absent from this table: %s", strings.Join(missing, ", "))
	}
	return b.String()
}

var runningHeaderFY = regexp.MustCompile(`(?i)(?:fy|year)?\s*(20[0-9]{2})\s*[-–]\s*(?:20)?([0-9]{2})`)

// DetectFY reads the report's own fiscal year out of its running header. It is
// advisory: ParseReliability records a mismatch rather than overriding the
// caller, because the running header is itself sometimes stale.
func DetectFY(doc *Doc) (string, bool) {
	counts := map[string]int{}
	for _, p := range doc.Pages {
		for _, l := range p.Lines {
			t := l.Text()
			if !strings.Contains(strings.ToLower(t), "performance") &&
				!strings.Contains(strings.ToLower(t), "evaluation") {
				continue
			}
			for _, m := range runningHeaderFY.FindAllStringSubmatch(t, -1) {
				counts["FY"+m[1]+"-"+m[2]]++
			}
		}
	}
	best, bestN := "", 0
	for k, v := range counts {
		if v > bestN || (v == bestN && k > best) {
			best, bestN = k, v
		}
	}
	return best, best != ""
}

// Lookup returns every Observation matching a Key -- plural by design.
//
// This is the whole point of the package: for FY2024-25 MEPCO SAIDI it returns
// two Observations, 3547.00 from Table 06 and 1182.56 from Table 18, and leaves
// the caller to see that the report disagrees with itself. A function that
// returned one number here would be a wrong API for this data.
//
// For an entity NEPRA excludes on the record, Lookup returns a single
// Observation whose Value is KindExcluded carrying NEPRA's stated reason.
func Lookup(r *Report, k Key) []Observation {
	if r == nil {
		return nil
	}
	var out []Observation
	for _, o := range r.Observations {
		if o.Key() == k {
			out = append(out, o)
		}
	}
	if len(out) == 0 {
		// The exclusion is asserted ONLY for the report that states it. It
		// used to be returned for any report and any period — the FY2014-15
		// report, which includes TESCO with real data, was made to assert its
		// exclusion in wording published a decade later, and the claim even
		// fired for a nonexistent fiscal year.
		//
		// Prov.ReportFY is the year of the QUOTE, not the year queried:
		// stamping the queried year attributed NEPRA's FY2024-25 sentence to
		// a report that never contained it.
		if v := ExclusionReasonInFY(k.Entity, r.FY); v.Kind == KindExcluded {
			return []Observation{{
				ReportFY: TESCOExclusionStatedFY,
				PeriodFY: k.PeriodFY,
				Entity:   k.Entity,
				Metric:   k.Metric,
				Value:    v,
				Target:   Absent(),
				Breach:   Absent(),
				Prov: Provenance{
					ReportFY:     TESCOExclusionStatedFY,
					TableCaption: TESCOExclusionSource,
				},
			}}
		}
	}
	return out
}

// MetricCounts reports how many Observations were extracted per entity per
// metric. It is the honest way to answer "what did the extractor actually get",
// and is measured from outside the extractor's own bookkeeping.
func MetricCounts(r *Report) map[Entity]map[Metric]int {
	out := map[Entity]map[Metric]int{}
	if r == nil {
		return out
	}
	for _, o := range r.Observations {
		if out[o.Entity] == nil {
			out[o.Entity] = map[Metric]int{}
		}
		out[o.Entity][o.Metric]++
	}
	return out
}

// minFusionPopulation is how many numeric cells a column needs before its
// digit-count spread means anything. Below this, no fusion claim is made:
// a suspicion computed from two or three cells would fire on ordinary
// variation, and a false Unverified hides a real published figure.
const minFusionPopulation = 5

// minFusionExcessDigits is how many digits above the column's third quartile a
// cell must reach before it is called a fusion rather than a large figure.
// MEASURED against the real spread: SAIDI across the ten DISCOs runs from
// 30.67 to 13469.55, a legitimate range of 4 to 7 digits, so a threshold of 3
// clears every genuine figure while a fusion of two 4-digit cells lands 4
// digits clear of Q3.
const minFusionExcessDigits = 3

// flagFusedCells finds cells that are probably two published cells glued
// together, using each column's OWN other rows as the control population.
//
// This is the statistical guard [assignCells] cannot provide geometrically:
// with 79.1% of spans carrying a zero advance width, the horizontal gap
// between tokens is not a usable signal (measured: legitimate intra-band gaps
// reach 35.88pt against an illegitimate 38.8pt). What a fusion DOES do is
// produce a figure with far more digits than the column can plausibly hold —
// 3547 and 1182 fusing to 35471182 turns a 4-digit column into an 8-digit
// one — and the sibling rows measure "plausible" for us.
//
// The test is Tukey's: flag a digit count above Q3 + 1.5*IQR. Two extra
// guards keep it from firing on ordinary variation: the count must also be at
// least double the column's median, and the column needs
// [minFusionPopulation] numeric cells. Flagged cells are demoted to
// KindUnverified with the reason recorded — never dropped, and never
// corrected, because which two cells were glued is not recoverable.
//
// Returns rowIndex -> columnIndex -> reason.
func flagFusedCells(rowCells [][]string, ncols int) map[int]map[int]string {
	out := map[int]map[int]string{}
	for j := 0; j < ncols; j++ {
		type cell struct {
			row    int
			digits int
		}
		var pop []cell
		for i := range rowCells {
			if j >= len(rowCells[i]) {
				continue
			}
			v := ParseFigure(rowCells[i][j])
			if v.Kind != KindNumeric {
				continue
			}
			pop = append(pop, cell{i, countDigits(rowCells[i][j])})
		}
		if len(pop) < minFusionPopulation {
			continue
		}
		counts := make([]int, len(pop))
		for i, c := range pop {
			counts[i] = c.digits
		}
		sort.Ints(counts)
		q1 := quantileInt(counts, 0.25)
		q3 := quantileInt(counts, 0.75)
		med := quantileInt(counts, 0.50)
		fence := q3 + 1.5*(q3-q1)
		for _, c := range pop {
			d := float64(c.digits)
			// Two conditions, and both are needed. The Tukey fence alone is
			// too sensitive when a column is tidy: an IQR of 0 collapses the
			// fence onto Q3, and then an ordinary 13469.55 sitting one digit
			// above a column of 4-digit figures would be called a fusion.
			// So the count must ALSO exceed Q3 by at least 3 digits. A fusion
			// roughly DOUBLES a cell's digits, so its excess over Q3 is about
			// a whole cell's width — comfortably more than 3 for any real
			// column — while normal cross-DISCO spread is one or two digits.
			if d <= fence || d < q3+minFusionExcessDigits {
				continue
			}
			if out[c.row] == nil {
				out[c.row] = map[int]string{}
			}
			out[c.row][j] = fmt.Sprintf(
				"cell %q has %d digits where this column's other rows have a median of %.0f "+
					"(Tukey upper fence %.1f over %d numeric cells); it is most likely two "+
					"published cells glued together by glyph-level cell reconstruction, so it is "+
					"recorded as unverified rather than as a number",
				rowCells[c.row][j], c.digits, med, fence, len(pop))
		}
	}
	return out
}

// countDigits counts the decimal digits in a published figure, ignoring
// separators, sign and the decimal point. It is the length a fusion inflates.
func countDigits(raw string) int {
	n := 0
	for i := 0; i < len(raw); i++ {
		if raw[i] >= '0' && raw[i] <= '9' {
			n++
		}
	}
	return n
}

// quantileInt is a linear-interpolation quantile over a SORTED slice.
func quantileInt(sorted []int, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return float64(sorted[0])
	}
	pos := q * float64(len(sorted)-1)
	lo := int(pos)
	hi := lo + 1
	if hi >= len(sorted) {
		return float64(sorted[len(sorted)-1])
	}
	frac := pos - float64(lo)
	return float64(sorted[lo]) + frac*float64(sorted[hi]-sorted[lo])
}
