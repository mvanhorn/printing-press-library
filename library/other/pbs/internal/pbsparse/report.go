// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package pbsparse

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// QuintileIndex is one row of the published SPI index table.
//
// The income BANDS are captured alongside the index because PBS revises them,
// and a revision is invisible to anyone who did not store the previous
// release: the site publishes no change log and keeps only the current edition.
type QuintileIndex struct {
	AsOf      string  `json:"as_of"`
	Quintile  string  `json:"quintile"`
	BandRaw   string  `json:"band_raw,omitempty"`
	BandLow   float64 `json:"band_low,omitempty"`
	BandHigh  float64 `json:"band_high,omitempty"`
	Index     Value   `json:"index"`
	PrevWeek  Value   `json:"prev_week"`
	CorWeek   Value   `json:"corresponding_week"`
	PctPrev   Value   `json:"pct_change_prev_week"`
	PctCorWk  Value   `json:"pct_change_corresponding_week"`
	IsCombine bool    `json:"is_combined"`
}

// TrendPoint is one row of the rolling ten-week trend table.
type TrendPoint struct {
	AsOf        string `json:"as_of"`
	WeekEnded   string `json:"week_ended"`
	SPILowest   Value  `json:"spi_lowest"`
	LowPctPrev  Value  `json:"spi_lowest_pct_prev"`
	LowPctCor   Value  `json:"spi_lowest_pct_corresponding"`
	Combined    Value  `json:"spi_combined"`
	CombPctPrev Value  `json:"spi_combined_pct_prev"`
	CombPctCor  Value  `json:"spi_combined_pct_corresponding"`
}

// MovementSection is one of the three ranked groups the report publishes.
type MovementSection string

const (
	SectionIncreased MovementSection = "increased"
	SectionDecreased MovementSection = "decreased"
	SectionUnchanged MovementSection = "unchanged"
)

// ItemWeight is one item's national price, weight and impact for a release.
//
// Sr is deliberately NOT a key: it restarts at 1 inside each of the three
// sections and the sections are re-ranked every week, so the item description
// is the only stable join key.
type ItemWeight struct {
	AsOf           string          `json:"as_of"`
	Section        MovementSection `json:"section"`
	Sr             int             `json:"sr"`
	ItemDesc       string          `json:"item_desc"`
	Unit           string          `json:"unit,omitempty"`
	NationalPrice  Value           `json:"national_price"`
	PricePrevWeek  Value           `json:"price_prev_week"`
	PriceCorWeek   Value           `json:"price_corresponding_week"`
	PctPrevWeek    Value           `json:"pct_change_prev_week"`
	PctCorWeek     Value           `json:"pct_change_corresponding_week"`
	WeightLowest   Value           `json:"weight_lowest"`
	WeightCombined Value           `json:"weight_combined"`
	ImpactLowest   Value           `json:"impact_lowest"`
	ImpactCombined Value           `json:"impact_combined"`
}

// SectionTotal is a published TOTAL row.
//
// TOTAL rows are INVARIANTS, not data. Summing items together with their
// section totals double-counts the release exactly once per section.
type SectionTotal struct {
	Section        MovementSection `json:"section"`
	DeclaredCount  int             `json:"declared_count"`
	ObservedCount  int             `json:"observed_count"`
	WeightLowest   Value           `json:"weight_lowest"`
	WeightCombined Value           `json:"weight_combined"`
	ImpactLowest   Value           `json:"impact_lowest"`
	ImpactCombined Value           `json:"impact_combined"`
}

// Report is one parsed SPI executive summary.
type Report struct {
	AsOf      string          `json:"as_of"`
	Quintiles []QuintileIndex `json:"quintiles"`
	Trend     []TrendPoint    `json:"trend"`
	Items     []ItemWeight    `json:"items"`
	Totals    []SectionTotal  `json:"totals"`
	Census    StateCensus     `json:"census"`
	Notes     []string        `json:"notes,omitempty"`
}

var (
	reQuintile = regexp.MustCompile(`(?i)^\s*(Q[1-5])\s*\((.*)\)\s*$`)
	reCombined = regexp.MustCompile(`(?i)^\s*combined\s*$`)
	reBandUpto = regexp.MustCompile(`(?i)upto\s*Rs\.?\s*([\d,]+)`)
	reBandAbov = regexp.MustCompile(`(?i)above\s*Rs\.?\s*([\d,]+)`)
	reBandRnge = regexp.MustCompile(`(?i)Rs\.?\s*([\d,]+)\s*-\s*([\d,]+)`)
	reWeekDate = regexp.MustCompile(`^\d{2}-\d{2}-\d{4}$`)
	reTotalRow = regexp.MustCompile(`(?i)^\s*total\s*$`)
	// Section headings declare their own item counts, e.g.
	// "i.    Average prices of the following 17 items registered INCREASE."
	reSectionHdr = regexp.MustCompile(`(?i)following\s+(\d+)\s+items?\s+(?:registered\s+)?(INCREASE|DECREASE)|following\s+(\d+)\s+items?\s+remained\s+(UNCHANGED)`)
)

func parseMoney(s string) float64 {
	f, _ := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(s), ",", ""), 64)
	return f
}

// ParseReportXLSX parses the SPI executive-summary workbook.
//
// asOf is the authoritative release date from the index. The report carries the
// weight vector and the quintile index table; the annexure carries the
// city panel. Neither file alone is a release, which is why sync fetches both.
func ParseReportXLSX(data []byte, asOf string) (*Report, error) {
	sheets, err := OpenXLSX(data)
	if err != nil {
		return nil, err
	}
	rep := &Report{AsOf: asOf}
	var vals []Value

	for _, sh := range sheets {
		var curSection MovementSection
		var declared int
		var observed int
		for _, r := range sh.Rows() {
			bRaw, _ := sh.Cell(r, 2)
			b := CleanText(bRaw)

			// --- quintile index table -------------------------------------
			if m := reQuintile.FindStringSubmatch(b); m != nil {
				q := QuintileIndex{AsOf: asOf, Quintile: strings.ToUpper(m[1]), BandRaw: CleanText(m[2])}
				applyBand(&q)
				q.Index = sh.Value(r, 4)
				q.PrevWeek = sh.Value(r, 5)
				q.CorWeek = sh.Value(r, 6)
				q.PctPrev = sh.Value(r, 7)
				q.PctCorWk = sh.Value(r, 8)
				vals = append(vals, q.Index, q.PrevWeek, q.CorWeek)
				rep.Quintiles = append(rep.Quintiles, q)
				continue
			}
			if reCombined.MatchString(b) {
				q := QuintileIndex{AsOf: asOf, Quintile: "COMBINED", IsCombine: true}
				q.Index = sh.Value(r, 4)
				q.PrevWeek = sh.Value(r, 5)
				q.CorWeek = sh.Value(r, 6)
				q.PctPrev = sh.Value(r, 7)
				q.PctCorWk = sh.Value(r, 8)
				vals = append(vals, q.Index, q.PrevWeek, q.CorWeek)
				rep.Quintiles = append(rep.Quintiles, q)
				continue
			}

			// --- rolling trend table --------------------------------------
			if reWeekDate.MatchString(b) {
				tp := TrendPoint{AsOf: asOf, WeekEnded: b}
				tp.SPILowest = sh.Value(r, 3)
				tp.LowPctPrev = sh.Value(r, 4)
				tp.LowPctCor = sh.Value(r, 5)
				tp.Combined = sh.Value(r, 6)
				tp.CombPctPrev = sh.Value(r, 7)
				tp.CombPctCor = sh.Value(r, 8)
				if tp.SPILowest.Present() || tp.Combined.Present() {
					vals = append(vals, tp.SPILowest, tp.Combined)
					rep.Trend = append(rep.Trend, tp)
					continue
				}
			}

			// --- section headings -----------------------------------------
			if m := reSectionHdr.FindStringSubmatch(b); m != nil {
				if curSection != "" {
					rep.Notes = append(rep.Notes,
						fmt.Sprintf("section %s: declared %d items, observed %d", curSection, declared, observed))
				}
				switch {
				case m[2] != "":
					declared, _ = strconv.Atoi(m[1])
					if strings.EqualFold(m[2], "INCREASE") {
						curSection = SectionIncreased
					} else {
						curSection = SectionDecreased
					}
				default:
					declared, _ = strconv.Atoi(m[3])
					curSection = SectionUnchanged
				}
				observed = 0
				continue
			}

			// --- TOTAL rows (invariants, never data) ----------------------
			if reTotalRow.MatchString(b) && curSection != "" {
				tot := SectionTotal{
					Section:        curSection,
					DeclaredCount:  declared,
					ObservedCount:  observed,
					WeightLowest:   sh.Value(r, 10),
					WeightCombined: sh.Value(r, 11),
					ImpactLowest:   sh.Value(r, 12),
					ImpactCombined: sh.Value(r, 13),
				}
				rep.Totals = append(rep.Totals, tot)
				continue
			}

			// --- item rows -------------------------------------------------
			if curSection == "" || !reItemNo.MatchString(b) {
				continue
			}
			descRaw, ok := sh.Cell(r, 3)
			if !ok {
				continue
			}
			desc := CleanText(descRaw)
			if desc == "" || !hasLetter(desc) {
				continue
			}
			sr, _ := strconv.Atoi(b)
			unit := ""
			if u, ok := sh.Cell(r, 4); ok {
				unit = CleanText(u)
			}
			iw := ItemWeight{
				AsOf: asOf, Section: curSection, Sr: sr, ItemDesc: desc, Unit: unit,
				NationalPrice:  sh.Value(r, 5),
				PricePrevWeek:  sh.Value(r, 6),
				PriceCorWeek:   sh.Value(r, 7),
				PctPrevWeek:    sh.Value(r, 8),
				PctCorWeek:     sh.Value(r, 9),
				WeightLowest:   sh.Value(r, 10),
				WeightCombined: sh.Value(r, 11),
				ImpactLowest:   sh.Value(r, 12),
				ImpactCombined: sh.Value(r, 13),
			}
			vals = append(vals, iw.NationalPrice, iw.WeightLowest, iw.WeightCombined)
			rep.Items = append(rep.Items, iw)
			observed++
		}
		if curSection != "" {
			rep.Notes = append(rep.Notes,
				fmt.Sprintf("section %s: declared %d items, observed %d", curSection, declared, observed))
		}
	}

	if len(rep.Quintiles) == 0 && len(rep.Items) == 0 {
		return nil, fmt.Errorf("report for %s parsed to zero quintile rows and zero item rows: the workbook layout changed", asOf)
	}
	sort.SliceStable(rep.Quintiles, func(i, j int) bool { return rep.Quintiles[i].Quintile < rep.Quintiles[j].Quintile })
	rep.Census = Census(vals)
	return rep, nil
}

func applyBand(q *QuintileIndex) {
	switch {
	case reBandUpto.MatchString(q.BandRaw):
		m := reBandUpto.FindStringSubmatch(q.BandRaw)
		q.BandHigh = parseMoney(m[1])
	case reBandAbov.MatchString(q.BandRaw):
		m := reBandAbov.FindStringSubmatch(q.BandRaw)
		q.BandLow = parseMoney(m[1])
	case reBandRnge.MatchString(q.BandRaw):
		m := reBandRnge.FindStringSubmatch(q.BandRaw)
		q.BandLow = parseMoney(m[1])
		q.BandHigh = parseMoney(m[2])
	}
}

// WeightTotals sums the published section TOTAL rows.
//
// The published item weights are a percentage decomposition of household
// expenditure, so the section totals must sum to 100 on both the lowest-quintile
// and combined columns. A deviation beyond rounding is a parse failure, not a
// finding about the economy.
func (r *Report) WeightTotals() (lowest, combined float64, sections int) {
	for _, t := range r.Totals {
		if t.WeightLowest.Present() {
			lowest += t.WeightLowest.Num
		}
		if t.WeightCombined.Present() {
			combined += t.WeightCombined.Num
		}
		sections++
	}
	return lowest, combined, sections
}

// SectionCountsAgree reports whether every section's declared item count
// matches the number of item rows actually parsed under it.
func (r *Report) SectionCountsAgree() (ok bool, detail []string) {
	ok = true
	byS := map[MovementSection]int{}
	for _, it := range r.Items {
		byS[it.Section]++
	}
	for _, t := range r.Totals {
		got := byS[t.Section]
		if t.DeclaredCount != got {
			ok = false
			detail = append(detail, fmt.Sprintf("%s: declared %d, parsed %d", t.Section, t.DeclaredCount, got))
		}
	}
	return ok, detail
}
