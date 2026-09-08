// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package pbsparse

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Stat is which of the three prices a cell holds.
type Stat string

const (
	StatMin Stat = "min"
	StatAvg Stat = "avg"
	StatMax Stat = "max"
	// StatSingle is used where PBS publishes one price rather than a triplet,
	// which is how Appendix-B and the monthly CPI annex are shaped.
	StatSingle Stat = "single"
)

// Surface names which table within a release a row came from. Kept explicit
// because the surfaces have different city sets and different null conventions.
type Surface string

const (
	SurfaceAppendixA Surface = "appendix-a"
	SurfaceAppendixB Surface = "appendix-b"
	SurfaceCPIAnnex  Surface = "cpi-annex"
)

// PriceRow is one observation: a period, a city, an item, a statistic, a value.
type PriceRow struct {
	AsOf     string  `json:"as_of"`
	Surface  Surface `json:"surface"`
	City     string  `json:"city"`
	CityCode string  `json:"city_code,omitempty"`
	ItemNo   int     `json:"item_no"`
	ItemDesc string  `json:"item_desc"`
	Unit     string  `json:"unit,omitempty"`
	Stat     Stat    `json:"stat"`
	Value
	// Block records which stacked row-block this row was read from. Preserved
	// so a mis-assignment is auditable: a parser that reads one header row and
	// applies it to every data row would silently attribute two thirds of the
	// panel to the wrong city, with entirely plausible values.
	Block int `json:"block"`
	// DescSuspect marks a row whose item description could not be separated
	// cleanly from the unit column. It occurs only in the PDF path, where a
	// long description physically overflows and overlaps the UNIT column in
	// the source itself. The numeric value is still exact; only the label is
	// best-effort. Flagged so a caller can exclude these from a description
	// join rather than silently trusting a mangled key.
	DescSuspect bool `json:"desc_suspect,omitempty"`
}

// Annexure is one parsed price appendix.
type Annexure struct {
	AsOf    string      `json:"as_of"`
	Rows    []PriceRow  `json:"rows"`
	Cities  []string    `json:"cities"`
	Items   []string    `json:"items"`
	Blocks  int         `json:"blocks"`
	Census  StateCensus `json:"census"`
	Surface Surface     `json:"surface"`
	// NationalAvg holds the published National Ave. column per item, when the
	// surface carries one. It is NOT recomputed: PBS weights cities and does
	// not publish the city weight vector, so a rebuilt figure cannot reproduce
	// this and must not overwrite it.
	NationalAvg map[string]Value `json:"national_avg,omitempty"`
	// Derived holds the extra series Appendix-A publishes beside the city
	// panel, keyed by item then by series name: the previous week's and the
	// corresponding week's national averages, the percent changes over each,
	// and the two fiscal-year averages with their difference. These are read
	// through, never recomputed.
	Derived map[string]map[string]Value `json:"derived,omitempty"`
}

var (
	reStatLabel = regexp.MustCompile(`(?i)^(min|avg|max|average)$`)
	reNationalH = regexp.MustCompile(`(?i)national\s*(ave\.?|average)`)
	reItemNo    = regexp.MustCompile(`^\d{1,3}$`)
	reAppendixA = regexp.MustCompile(`(?i)appendix\s*-?\s*a`)
	reAppendixB = regexp.MustCompile(`(?i)appendix\s*-?\s*b`)
)

// headerGroup is one merged header span plus the sub-labels beneath it.
//
// Appendix-A's third band is NOT just cities: after the last three urban centres
// it carries a "National Average" min/avg/max triplet, a "National Ave."
// {Prv. Wk, Cor. Wk} pair, a "% Change over" pair, and a four-wide
// "Yearly Average Prices" group. Treating every header containing the word
// "national" as one flat set of columns picks the PREVIOUS week's national
// average instead of the current one — the same magnitude, a different variable.
// So each span is resolved to its own group with its own sub-labels.
type headerGroup struct {
	col   int
	span  int
	title string
	subs  map[int]string // column -> sub-label from the statistic row
}

// cityBlock is one repeated header band: a row plus the column mappings that
// apply to the data rows beneath it, until the next band.
type cityBlock struct {
	headerRow int
	statRow   int
	colCity   map[int]string
	colCode   map[int]string
	colStat   map[int]Stat
	// nationalAvgCol is the AVG column of the "National Average" price triplet,
	// i.e. the current period's published national figure. Zero when absent.
	nationalAvgCol int
	// derived maps a canonical extra-series name to its column.
	derived map[string]int
	// unresolvedStats names cities whose multi-column span had unreadable
	// statistic sub-labels. Such a city contributes no rows.
	unresolvedStats []string
}

var reYearlyGroup = regexp.MustCompile(`(?i)yearly\s+average`)
var rePctGroup = regexp.MustCompile(`(?i)%\s*change`)

// canonicalDerived names an extra column from its group title and sub-label.
func canonicalDerived(title, sub string) string {
	t := strings.ToLower(CleanText(title))
	s := strings.ToLower(CleanText(sub))
	norm := func(x string) string {
		switch {
		case strings.Contains(x, "prv") || strings.Contains(x, "prev"):
			return "prev_week"
		case strings.Contains(x, "cor"):
			return "corresponding_week"
		case strings.Contains(x, "diff"):
			return "diff"
		case strings.Contains(x, "chng") || strings.Contains(x, "change"):
			return "pct_change"
		}
		return strings.ReplaceAll(x, "-", "_")
	}
	switch {
	case reNationalH.MatchString(t):
		return "national_" + norm(s)
	case rePctGroup.MatchString(t):
		return "pct_over_" + norm(s)
	case reYearlyGroup.MatchString(t):
		return "yearly_" + norm(s)
	}
	return ""
}

// findCityBlocks locates every repeated header band in a sheet.
//
// PBS lays Appendix-A out as several stacked bands rather than one wide table:
// the 2026-09-03 workbook has bands at rows 3, 61 and 119 carrying seven, seven
// and three urban centres. Column spans are NOT uniform — the third band mixes
// two-column and four-column groups against three-column groups elsewhere — so
// every mapping is read from the merge ranges rather than assumed as a stride.
func findCityBlocks(sh *Sheet) []cityBlock {
	mergeByRow := map[int][]Merge{}
	for _, m := range sh.Merges {
		mergeByRow[m.R1] = append(mergeByRow[m.R1], m)
	}

	var blocks []cityBlock
	for _, r := range sh.Rows() {
		// A band is a row carrying at least two city headers. Cities are the
		// only headers with a two-digit PBS code, which is what distinguishes
		// them from the national and derived groups beside them.
		cityHits := 0
		for c := 1; c <= sh.MaxCol; c++ {
			if raw, ok := sh.Cell(r, c); ok {
				if _, code := NormalizeCity(CleanText(raw)); code != "" {
					cityHits++
				}
			}
		}
		if cityHits < 2 {
			continue
		}

		blk := cityBlock{
			headerRow: r,
			colCity:   map[int]string{},
			colCode:   map[int]string{},
			colStat:   map[int]Stat{},
			derived:   map[string]int{},
		}

		// Locate the sub-label row beneath the band.
		for probe := r + 1; probe <= r+3 && probe <= sh.MaxRow; probe++ {
			found := 0
			for c := 1; c <= sh.MaxCol; c++ {
				if raw, ok := sh.Cell(probe, c); ok && reStatLabel.MatchString(strings.TrimSpace(raw)) {
					found++
				}
			}
			if found >= 2 {
				blk.statRow = probe
				break
			}
		}

		subLabel := func(c int) string {
			if blk.statRow == 0 {
				return ""
			}
			if raw, ok := sh.Cell(blk.statRow, c); ok {
				return CleanText(raw)
			}
			return ""
		}
		statOf := func(c int) (Stat, bool) {
			m := reStatLabel.FindStringSubmatch(strings.TrimSpace(subLabel(c)))
			if m == nil {
				return "", false
			}
			switch strings.ToLower(m[1]) {
			case "min":
				return StatMin, true
			case "max":
				return StatMax, true
			default:
				return StatAvg, true
			}
		}

		// Build the header groups from the merges anchored on this row, plus
		// any unmerged single-column header.
		var groups []headerGroup
		for c := 1; c <= sh.MaxCol; c++ {
			raw, ok := sh.Cell(r, c)
			if !ok {
				continue
			}
			title := CleanText(raw)
			if title == "" {
				continue
			}
			span := 1
			for _, m := range mergeByRow[r] {
				if m.C1 == c {
					span = m.Cols()
					break
				}
			}
			g := headerGroup{col: c, span: span, title: title, subs: map[int]string{}}
			for k := 0; k < span; k++ {
				g.subs[c+k] = subLabel(c + k)
			}
			groups = append(groups, g)
		}

		for _, g := range groups {
			key, code := NormalizeCity(g.title)
			if code != "" {
				// A real urban centre: map every column it spans.
				//
				// A multi-column city span whose statistic sub-labels could not
				// be read is REFUSED, not defaulted. Stamping StatSingle on all
				// three columns makes them collide on the primary key
				// (as_of, surface, city, item, stat), so the upsert keeps one of
				// min/avg/max and Go's map iteration order decides which —
				// a different answer on every run, with sync still reporting
				// three rows written.
				if g.span > 1 {
					allResolved := true
					for k := 0; k < g.span; k++ {
						if _, ok := statOf(g.col + k); !ok {
							allResolved = false
							break
						}
					}
					if !allResolved {
						blk.unresolvedStats = append(blk.unresolvedStats, key)
						continue
					}
				}
				for k := 0; k < g.span; k++ {
					col := g.col + k
					blk.colCity[col] = key
					blk.colCode[col] = code
					if st, ok := statOf(col); ok {
						blk.colStat[col] = st
					} else {
						// Single-column city surfaces publish one price.
						blk.colStat[col] = StatSingle
					}
				}
				continue
			}
			if reNationalH.MatchString(g.title) {
				// The national PRICE triplet is the group whose sub-labels are
				// min/avg/max. The similarly-named "National Ave." group whose
				// sub-labels are Prv. Wk / Cor. Wk is a different series and is
				// captured as a derived column instead.
				isPriceTriplet := false
				for k := 0; k < g.span; k++ {
					if st, ok := statOf(g.col + k); ok && st == StatAvg {
						isPriceTriplet = true
					}
				}
				if isPriceTriplet {
					for k := 0; k < g.span; k++ {
						if st, ok := statOf(g.col + k); ok && st == StatAvg {
							blk.nationalAvgCol = g.col + k
						}
					}
					continue
				}
			}
			// Everything else is a derived series: previous-week and
			// corresponding-week national averages, percent changes, and the
			// two fiscal-year averages with their difference.
			for k := 0; k < g.span; k++ {
				col := g.col + k
				if name := canonicalDerived(g.title, g.subs[col]); name != "" {
					if _, exists := blk.derived[name]; !exists {
						blk.derived[name] = col
					}
				}
			}
		}
		if len(blk.colCity) == 0 {
			continue
		}
		blocks = append(blocks, blk)
	}
	sort.Slice(blocks, func(i, j int) bool { return blocks[i].headerRow < blocks[j].headerRow })
	return blocks
}

// blockFor returns the index of the band governing a data row: the nearest
// band whose header sits above it.
func blockFor(blocks []cityBlock, row int) int {
	idx := -1
	for i, b := range blocks {
		if b.headerRow < row {
			idx = i
		}
	}
	return idx
}

// ParseAnnexureXLSX parses the price appendix out of an annexure workbook.
//
// asOf is the authoritative release date from the index, not a date scraped
// from inside the file: the workbook renders its own date four different ways
// and two index rows disagree with their filename.
func ParseAnnexureXLSX(data []byte, asOf string) (*Annexure, error) {
	sheets, err := OpenXLSX(data)
	if err != nil {
		return nil, err
	}
	out := &Annexure{AsOf: asOf, Surface: SurfaceAppendixA,
		NationalAvg: map[string]Value{}, Derived: map[string]map[string]Value{}}
	citySet := map[string]bool{}
	itemSeen := map[string]bool{}
	var vals []Value
	var unresolved []string

	for si, sh := range sheets {
		surface := SurfaceAppendixA
		if si > 0 {
			surface = SurfaceAppendixB
		}
		// Prefer an explicit in-sheet marker over sheet position.
		for _, r := range sh.Rows()[:min(6, len(sh.Rows()))] {
			for c := 1; c <= min(sh.MaxCol, 12); c++ {
				if raw, ok := sh.Cell(r, c); ok {
					if reAppendixB.MatchString(raw) {
						surface = SurfaceAppendixB
					} else if reAppendixA.MatchString(raw) {
						surface = SurfaceAppendixA
					}
				}
			}
		}
		if surface != SurfaceAppendixA {
			// Appendix-B has a different and smaller city set and needs its own
			// resolution path. It is not modelled as price rows in this
			// version; its cells are still counted so the state census over a
			// release is complete rather than silently partial.
			for _, r := range sh.Rows() {
				for c := 4; c <= sh.MaxCol; c++ {
					vals = append(vals, sh.Value(r, c))
				}
			}
			continue
		}

		blocks := findCityBlocks(sh)
		if len(blocks) == 0 {
			continue
		}
		for _, b := range blocks {
			unresolved = append(unresolved, b.unresolvedStats...)
		}
		out.Blocks += len(blocks)

		for _, r := range sh.Rows() {
			noRaw, ok := sh.Cell(r, 1)
			if !ok || !reItemNo.MatchString(strings.TrimSpace(noRaw)) {
				continue
			}
			descRaw, ok := sh.Cell(r, 2)
			if !ok {
				continue
			}
			desc := CleanText(descRaw)
			if desc == "" || reItemNo.MatchString(desc) {
				continue
			}
			bi := blockFor(blocks, r)
			if bi < 0 {
				continue
			}
			blk := blocks[bi]
			itemNo, _ := strconv.Atoi(strings.TrimSpace(noRaw))
			unit := ""
			if u, ok := sh.Cell(r, 3); ok {
				unit = CleanText(u)
			}
			itemSeen[desc] = true

			for c, city := range blk.colCity {
				st, ok := blk.colStat[c]
				if !ok {
					st = StatSingle
				}
				v := sh.Value(r, c)
				vals = append(vals, v)
				citySet[city] = true
				out.Rows = append(out.Rows, PriceRow{
					AsOf: asOf, Surface: surface, City: city, CityCode: blk.colCode[c],
					ItemNo: itemNo, ItemDesc: desc, Unit: unit, Stat: st,
					Value: v, Block: bi + 1,
				})
			}
			if blk.nationalAvgCol > 0 {
				if v := sh.Value(r, blk.nationalAvgCol); v.Present() {
					out.NationalAvg[desc] = v
				}
			}
			for name, c := range blk.derived {
				v := sh.Value(r, c)
				if !v.Present() {
					continue
				}
				if out.Derived[desc] == nil {
					out.Derived[desc] = map[string]Value{}
				}
				out.Derived[desc][name] = v
			}
		}
	}

	if len(unresolved) > 0 {
		sort.Strings(unresolved)
		return nil, fmt.Errorf("annexure for %s: statistic sub-labels (MIN/AVG/MAX) could not be read for %d cities (%s); refusing rather than collapsing three columns onto one key",
			asOf, len(unresolved), strings.Join(unresolved, ", "))
	}
	if len(out.Rows) == 0 {
		return nil, fmt.Errorf("annexure for %s parsed to zero price rows: the workbook layout changed (expected stacked city header bands in Appendix-A)", asOf)
	}
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
