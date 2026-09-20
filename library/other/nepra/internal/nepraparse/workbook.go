package nepraparse

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
)

// ErrFiscalYearMismatch is returned when the caller's fy argument disagrees
// with the band label inside the file. Fetching the wrong year's sheet is an
// easy mistake and one that would date every observation wrongly, so it is an
// error rather than a warning.
var ErrFiscalYearMismatch = errors.New("nepraparse: fiscal year argument disagrees with the workbook's header band")

// MonthlyObservation is one month's pair of published fields. Both carry
// their own [CellState]: a plant can have a reported utilisation and an
// unreported generation, and the pair must not be collapsed into one
// nullability.
type MonthlyObservation struct {
	// Month is the fiscal month, or 0 for the annual Sum column.
	Month Month
	// Utilisation is the "% age" field — plant utilisation in percent.
	Utilisation Value
	// Generation is the "GWh" field — energy generated.
	Generation Value
	// IsTotal marks the annual Sum column rather than a month.
	IsTotal bool
}

// Label names the observation's column group, "Jul".."Jun" or "Sum".
func (o MonthlyObservation) Label() string {
	if o.IsTotal {
		return pairLabels[SumPairIndex]
	}
	return o.Month.String()
}

// Plant is one numbered row of the workbook: a single generating plant.
// There are no subtotal or grand-total rows in any sampled year, so every
// numbered row is a plant.
type Plant struct {
	// SNo is the workbook's serial number, an IN-YEAR ORDINAL ONLY.
	//
	// It is NOT a stable cross-year key. It is stable FY2017-18 ->
	// FY2020-21 (107/107 unchanged) but 0/106 unchanged in FY2023-24,
	// because that file was re-sorted by technology: AES Lalpir moved 1->36
	// and Allai Khwar 56->8. Joining years on S.No silently shuffles the
	// panel. Cross-year identity belongs to a different package.
	SNo int
	// SNoRaw is the S.No cell as published, kept for the rare row whose
	// serial is not a plain integer.
	SNoRaw string
	// Name is the "Name of Companies" value, whitespace-collapsed and
	// rejoined across hidden spans ("Company Limit<span
	// style='display:none'>ed.</span>" -> "Company Limited."). Unique within
	// every sampled year.
	Name string
	// Technology is the published technology string, verbatim.
	Technology string
	// TechnologyKnown reports membership of [KnownTechnologies].
	TechnologyKnown bool
	// Fuel is the published fuel string, verbatim.
	Fuel string
	// FuelKnown reports membership of [KnownFuels].
	FuelKnown bool
	// InstalledCapacity is "Installed Capacity (MW)". This is where
	// "Export to K.Electric" appears — three FY2020-21 rows — so the field
	// is a [Value] rather than a float.
	InstalledCapacity Value
	// DependableCapacity is "Dependable Capacity (MW)".
	DependableCapacity Value
	// Months holds the twelve monthly observations in FISCAL order, index 0
	// = Jul .. index 11 = Jun. Use [Plant.Month] to fetch one by name.
	Months [12]MonthlyObservation
	// Total is the annual "Sum" column, which is literally labelled "Sum".
	Total MonthlyObservation
	// RowIndex is the source table row this plant came from, for diagnostics.
	RowIndex int
	// Residue holds any non-empty cell found beyond the 32 logical columns.
	// Trailing padding is normally empty, but one row per sampled year
	// carries a stray ";" there. Recorded so the padding assumption is
	// observable rather than assumed.
	Residue []string
	// Status is the single status sentinel that covers this plant's whole
	// monthly block, or [StateNumeric] when the block is ordinary data.
	// DELICENSED and DECOMMISSIONED rows keep valid capacity values.
	Status CellState
}

// Month returns the observation for a fiscal month.
func (p Plant) Month(m Month) (MonthlyObservation, bool) {
	if !m.Valid() {
		return MonthlyObservation{}, false
	}
	return p.Months[int(m)-1], true
}

// MonthlyCells is the number of cells in a plant's monthly block: twelve
// months plus Sum, each a "% age"/"GWh" pair.
const MonthlyCells = PairCount * 2

// Census counts cell states for one parse so a caller can prove null
// discipline instead of trusting it. Every count here is a fact about the
// file, not a policy.
type Census struct {
	// Rows is the number of plant rows.
	Rows int
	// MonthlyCellsTotal is Rows * [MonthlyCells] and must equal the sum of
	// the six state counts below.
	MonthlyCellsTotal int

	// The five modelled states plus the unmodelled bucket, over the monthly
	// block only.
	Present           int
	NotReported       int
	Delicensed        int
	Decommissioned    int
	ExportToKElectric int
	UnknownText       int

	// MeasuredZeros counts monthly cells whose published value is exactly
	// zero. These are real measurements: 437/540/471 across
	// FY2017-18/FY2020-21/FY2023-24.
	MeasuredZeros int
	// ThousandsSeparated counts cells whose published text used commas:
	// 61/59/68 across FY2017-18/FY2020-21/FY2023-24.
	ThousandsSeparated int

	// NotReportedRows is the number of plants with at least one blank in the
	// monthly block. 11/3/2 across FY2017-18/FY2020-21/FY2023-24 — in
	// FY2023-24 both are the two rows carrying an EMPTY <td colspan=26>.
	NotReportedRows int
	// FullyNotReportedRows is the number of plants with all 26 monthly cells
	// blank. It equals NotReportedRows in every sampled year, because the
	// blank pattern is strictly all-or-nothing per row.
	FullyNotReportedRows int
	// MixedBlankZeroRows is the number of plants mixing a blank and a 0.00
	// in the monthly block. It is 0 in every sampled year, and that is the
	// evidence that blank and zero are different things rather than two
	// spellings of the same thing.
	MixedBlankZeroRows int
	// StatusRows counts plants whose whole monthly block is a status
	// sentinel: 12 DELICENSED + 1 DECOMMISSIONED in FY2023-24, 0 elsewhere.
	StatusRows int

	// Capacity columns, counted separately because a status sentinel must
	// not null them.
	CapacityCells       int
	CapacityPresent     int
	CapacityNotReported int
	CapacityStatus      int

	// ResidueCells counts non-empty cells beyond the 32 logical columns.
	ResidueCells int
}

// Balanced reports whether the six monthly state counts add up to
// MonthlyCellsTotal. A false result means a cell escaped classification.
func (c Census) Balanced() bool {
	return c.Present+c.NotReported+c.Delicensed+c.Decommissioned+
		c.ExportToKElectric+c.UnknownText == c.MonthlyCellsTotal
}

// Workbook is one parsed fiscal year.
type Workbook struct {
	// FiscalYear is the year the file covers, taken from the header band and
	// cross-checked against the caller's argument.
	FiscalYear FiscalYear
	// Title is the tr#1 <td colspan=32> title line.
	Title string
	// Header is what the parser read out of the three-row band.
	Header HeaderFingerprint
	// Plants are the plant rows in source order.
	Plants []Plant
	// Census counts the cell states of this parse.
	Census Census
	// Charset is the character set actually applied when decoding. It equals
	// [CharsetWindows1252] on both the declared and the assumed path; read
	// CharsetDeclared to tell them apart.
	Charset string
	// CharsetDeclared reports whether the document declared the charset in a
	// <meta> tag, as every sampled year does, rather than the parser assuming
	// it. The HTTP response declares no charset, so this is the only
	// declaration there is.
	CharsetDeclared bool
	// PhysicalWidth is the widest expanded row: 40 in
	// FY2017-18/FY2020-21, 39 in FY2023-24.
	PhysicalWidth int
	// LogicalWidth is always [LogicalColumns]; kept explicit so the
	// difference from PhysicalWidth is visible in output.
	LogicalWidth int
	// TableCount, HeaderCellCount, TableRows and RawCells are structural
	// facts: every sampled year has exactly 1 table and ZERO <th>;
	// FY2023-24 has 139 rows / 4,965 raw cells and FY2017-18 and FY2020-21
	// both have 114 / 4,479.
	TableCount      int
	HeaderCellCount int
	TableRows       int
	RawCells        int
	// SeparatorRows is the number of entirely empty table rows outside the
	// header band: 2 in every sampled year.
	SeparatorRows int
	// SNoContiguous reports whether S.No runs 1..N with no gaps and no
	// duplicates. True for every full sampled year.
	SNoContiguous bool
	// NamesUnique reports whether Name of Companies is unique within the
	// year. True for every sampled year.
	NamesUnique bool
	// Warnings are non-fatal observations: unknown vocabulary values,
	// unmodelled cell text, residue past the logical width, S.No gaps.
	// A warning never means a value was fabricated or dropped.
	Warnings []string
}

// PlantByName returns the plant with the given name.
func (w *Workbook) PlantByName(name string) (Plant, bool) {
	for _, p := range w.Plants {
		if p.Name == name {
			return p, true
		}
	}
	return Plant{}, false
}

// Technologies returns the distinct Technology values in the workbook, sorted.
func (w *Workbook) Technologies() []string {
	return w.distinct(func(p Plant) string { return p.Technology })
}

// Fuels returns the distinct Fuel values in the workbook, sorted.
func (w *Workbook) Fuels() []string { return w.distinct(func(p Plant) string { return p.Fuel }) }

func (w *Workbook) distinct(f func(Plant) string) []string {
	seen := map[string]struct{}{}
	for _, p := range w.Plants {
		if v := f(p); v != "" {
			seen[v] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// ParseWorkbook parses one fiscal year's plant-level generation workbook.
//
// data is the raw sheet001.htm bytes, undecoded — the function decodes
// windows-1252 itself, because the HTTP response declares no charset and a
// strict UTF-8 read of these files fails.
//
// fy is the expected fiscal year ("2023-24"; "FY 2023-24" and "2023-2024"
// also parse). Pass "" to take whatever the file's header band says. A
// non-empty fy that disagrees with the band is [ErrFiscalYearMismatch].
//
// The parse fails on schema drift ([ErrSchemaDrift]) and on a missing header
// band ([ErrNoHeaderBand]). It does not fail on unknown vocabulary values or
// unmodelled cell text: those are recorded in [Workbook.Warnings] and in the
// cell's own [CellState], because dropping an unexpected value is worse than
// surfacing it.
func ParseWorkbook(data []byte, fy string) (*Workbook, error) {
	wantFY, err := normaliseFYArg(fy)
	if err != nil {
		return nil, err
	}

	doc, charset, charsetDeclared, err := Decode(data)
	if err != nil {
		return nil, err
	}

	g, err := BuildGrid(doc)
	if err != nil {
		return nil, err
	}

	header, err := findHeaderBand(g)
	if err != nil {
		return nil, err
	}
	if !header.Matches() {
		return nil, fmt.Errorf("%w: %v", ErrSchemaDrift, header.Diff())
	}

	bandFY, err := ParseFiscalYear(header.BandLabel)
	if err != nil {
		return nil, fmt.Errorf("nepraparse: header band label %q: %w", header.BandLabel, err)
	}
	if !wantFY.Zero() && wantFY != bandFY {
		return nil, fmt.Errorf("%w: caller said %s, file says %s",
			ErrFiscalYearMismatch, wantFY.Label(), bandFY.Label())
	}

	w := &Workbook{
		FiscalYear:      bandFY,
		Header:          header,
		Charset:         charset,
		CharsetDeclared: charsetDeclared,
		PhysicalWidth:   g.Width,
		LogicalWidth:    LogicalColumns,
		TableCount:      g.Tables,
		HeaderCellCount: g.HeaderCells,
		TableRows:       len(g.Rows),
		RawCells:        g.RawCells,
	}
	if header.BandRows[0] > 0 {
		w.Title = g.Rows[header.BandRows[0]-1][0]
	}
	if g.Tables != 1 {
		w.Warnings = append(w.Warnings, fmt.Sprintf("structure: expected exactly 1 <table>, found %d", g.Tables))
	}
	if g.HeaderCells != 0 {
		w.Warnings = append(w.Warnings, fmt.Sprintf("structure: expected zero <th>, found %d", g.HeaderCells))
	}

	for i := header.BandRows[2] + 1; i < len(g.Rows); i++ {
		row := g.Rows[i]
		if rowEmpty(row) {
			w.SeparatorRows++
			continue
		}
		w.Plants = append(w.Plants, buildPlant(row, i))
	}
	// Rows above the band that are entirely empty are separators too — tr#0
	// is one in every sampled year.
	for i := 0; i < header.BandRows[0]; i++ {
		if rowEmpty(g.Rows[i]) {
			w.SeparatorRows++
		}
	}

	w.Census = censusOf(w.Plants)
	w.SNoContiguous = snoContiguous(w.Plants)
	w.NamesUnique = namesUnique(w.Plants)
	w.Warnings = append(w.Warnings, plantWarnings(w.Plants)...)
	if !w.SNoContiguous {
		w.Warnings = append(w.Warnings, "sno: S.No is not contiguous 1..N (expected for a trimmed fixture, suspicious for a full year)")
	}
	if !w.NamesUnique {
		w.Warnings = append(w.Warnings, "name: Name of Companies is not unique within the year")
	}
	return w, nil
}

func rowEmpty(row []string) bool {
	for _, c := range row {
		if c != "" {
			return false
		}
	}
	return true
}

func buildPlant(row []string, rowIndex int) Plant {
	cell := func(i int) string {
		if i < len(row) {
			return row[i]
		}
		return ""
	}
	p := Plant{
		SNo:                -1,
		SNoRaw:             cell(ColSNo),
		Name:               cell(ColName),
		Technology:         cell(ColTechnology),
		Fuel:               cell(ColFuel),
		InstalledCapacity:  ParseValue(cell(ColInstalledCapacity)),
		DependableCapacity: ParseValue(cell(ColDependableCapacity)),
		RowIndex:           rowIndex,
		Status:             StateNumeric,
	}
	p.TechnologyKnown = KnownTechnology(p.Technology)
	p.FuelKnown = KnownFuel(p.Fuel)
	if n, err := strconv.Atoi(p.SNoRaw); err == nil {
		p.SNo = n
	}

	for pair := 0; pair < PairCount; pair++ {
		base := ColFirstMonthPair + 2*pair
		obs := MonthlyObservation{
			Utilisation: ParseValue(cell(base + PctOffset)),
			Generation:  ParseValue(cell(base + GWhOffset)),
		}
		if pair == SumPairIndex {
			obs.IsTotal = true
			p.Total = obs
			continue
		}
		obs.Month = MonthsInFiscalOrder[pair]
		p.Months[pair] = obs
	}

	for i := LogicalColumns; i < len(row); i++ {
		if row[i] != "" {
			p.Residue = append(p.Residue, row[i])
		}
	}

	p.Status = blockStatus(p)
	return p
}

// blockStatus reports the single status sentinel covering a plant's whole
// monthly block, or StateNumeric when the block is not uniformly a status.
// A DELICENSED row is DELICENSED in all 26 cells because the source writes
// one <td colspan=26>; requiring uniformity here means a partial or shifted
// read cannot masquerade as a clean status row.
func blockStatus(p Plant) CellState {
	first := p.Months[0].Utilisation.State()
	if !first.Status() {
		return StateNumeric
	}
	for _, obs := range append(p.Months[:], p.Total) {
		if obs.Utilisation.State() != first || obs.Generation.State() != first {
			return StateNumeric
		}
	}
	return first
}

func censusOf(plants []Plant) Census {
	c := Census{Rows: len(plants), MonthlyCellsTotal: len(plants) * MonthlyCells}
	for _, p := range plants {
		blanks, zeros := 0, 0
		for _, obs := range append(p.Months[:], p.Total) {
			for _, v := range [2]Value{obs.Utilisation, obs.Generation} {
				switch v.State() {
				case StateNumeric:
					c.Present++
					if v.num == 0 {
						c.MeasuredZeros++
						zeros++
					}
				case StateNotReported:
					c.NotReported++
					blanks++
				case StateDelicensed:
					c.Delicensed++
				case StateDecommissioned:
					c.Decommissioned++
				case StateExportToKElectric:
					c.ExportToKElectric++
				default:
					c.UnknownText++
				}
				if v.HadThousandsSeparator() {
					c.ThousandsSeparated++
				}
			}
		}
		if blanks > 0 {
			c.NotReportedRows++
		}
		if blanks == MonthlyCells {
			c.FullyNotReportedRows++
		}
		if blanks > 0 && zeros > 0 {
			c.MixedBlankZeroRows++
		}
		if p.Status.Status() {
			c.StatusRows++
		}
		for _, v := range [2]Value{p.InstalledCapacity, p.DependableCapacity} {
			c.CapacityCells++
			switch {
			case v.Present():
				c.CapacityPresent++
				if v.HadThousandsSeparator() {
					c.ThousandsSeparated++
				}
			case v.State() == StateNotReported:
				c.CapacityNotReported++
			case v.State().Status():
				c.CapacityStatus++
			}
		}
		c.ResidueCells += len(p.Residue)
	}
	return c
}

func snoContiguous(plants []Plant) bool {
	for i, p := range plants {
		if p.SNo != i+1 {
			return false
		}
	}
	return true
}

func namesUnique(plants []Plant) bool {
	seen := make(map[string]struct{}, len(plants))
	for _, p := range plants {
		if _, dup := seen[p.Name]; dup {
			return false
		}
		seen[p.Name] = struct{}{}
	}
	return true
}

// plantWarnings surfaces every unmodelled or unexpected value the parse met.
// Nothing here is fatal and nothing here was dropped.
func plantWarnings(plants []Plant) []string {
	var out []string
	for _, p := range plants {
		who := p.Name
		if who == "" {
			who = "row " + strconv.Itoa(p.RowIndex)
		}
		if p.Name == "" {
			out = append(out, fmt.Sprintf("name: row %d has no Name of Companies", p.RowIndex))
		}
		if p.SNo < 0 {
			out = append(out, fmt.Sprintf("sno: %s has a non-integer S.No %q", who, p.SNoRaw))
		}
		// Technology and Fuel are plain strings, so a blank cell, an absent
		// column and a plant NEPRA simply did not classify all arrive as "".
		// The empty case therefore has to be warned about explicitly:
		// guarding on `!= ""` first — as this did — means the one state that
		// carries NO information is the only one that passes silently.
		// Measured: no row in any sampled year has an empty Technology or
		// Fuel, so a warning here means the source or the column mapping has
		// changed.
		switch {
		case p.Technology == "":
			out = append(out, fmt.Sprintf("vocabulary: %s has an EMPTY Technology; blank, absent and unclassified are indistinguishable in this field", who))
		case !p.TechnologyKnown:
			out = append(out, fmt.Sprintf("vocabulary: %s has unknown Technology %q", who, p.Technology))
		}
		switch {
		case p.Fuel == "":
			out = append(out, fmt.Sprintf("vocabulary: %s has an EMPTY Fuel; blank, absent and unclassified are indistinguishable in this field", who))
		case !p.FuelKnown:
			out = append(out, fmt.Sprintf("vocabulary: %s has unknown Fuel %q", who, p.Fuel))
		}
		for _, cv := range [2]struct {
			label string
			v     Value
		}{{"Installed Capacity (MW)", p.InstalledCapacity}, {"Dependable Capacity (MW)", p.DependableCapacity}} {
			if cv.v.State() == StateUnknownText {
				out = append(out, fmt.Sprintf("cell: %s has unmodelled %s text %q", who, cv.label, cv.v.Raw()))
			}
		}
		for _, obs := range append(p.Months[:], p.Total) {
			for _, mv := range [2]struct {
				metric string
				v      Value
			}{{MetricPct, obs.Utilisation}, {MetricGWh, obs.Generation}} {
				if mv.v.State() == StateUnknownText {
					out = append(out, fmt.Sprintf("cell: %s %s %s has unmodelled text %q",
						who, obs.Label(), mv.metric, mv.v.Raw()))
				}
			}
		}
		if len(p.Residue) > 0 {
			out = append(out, fmt.Sprintf("residue: %s has %d non-empty cell(s) past logical column %d: %q",
				who, len(p.Residue), LogicalColumns-1, p.Residue))
		}
	}
	return out
}
