// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package mufap

import (
	"math"
	"sort"
	"strings"
)

// Percentile returns the linear-interpolated percentile of vs (p in [0,100]).
// vs is sorted in place. Returns ok=false for an empty slice rather than 0,
// because a zero rate and "no funds reported" are different facts.
func Percentile(vs []float64, p float64) (float64, bool) {
	if len(vs) == 0 {
		return 0, false
	}
	sort.Float64s(vs)
	if len(vs) == 1 {
		return vs[0], true
	}
	pos := (p / 100) * float64(len(vs)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return vs[lo], true
	}
	return vs[lo] + (vs[hi]-vs[lo])*(pos-float64(lo)), true
}

// Median is Percentile(vs, 50).
func Median(vs []float64) (float64, bool) { return Percentile(vs, 50) }

// RatePoint is one date's market-implied short rate.
//
// FundCount travels with the rate deliberately: a median over three funds and
// a median over thirty are not the same observation, and a consumer that
// cannot see the width cannot tell them apart. FundCount is always the count
// the median and percentiles were actually computed over.
//
// OutlierRows and the fences travel with it for the same reason, one layer
// down. Measured on the mirror: two annualized money-market funds per date sit
// below the bulk of the cross-section — Pak Oman Daily Dividend Fund "(0.20)"
// and Faysal Islamic Punjab Pension Fund "(1.92)", both MUFAP accounting
// negatives (endpoint contract §I). An earlier version DELETED them against a
// fixed (0, 60] band, which moved the reported p10 from 8.593 to 9.165 — it
// clipped exactly the left tail the p10 exists to describe, and no emitted
// count showed it. They are now kept and flagged instead.
type RatePoint struct {
	Date        string  `json:"date"`
	MedianYield float64 `json:"median_yield"`
	P10         float64 `json:"p10"`
	P90         float64 `json:"p90"`
	FundCount   int     `json:"fund_count"`
	Column      string  `json:"column"`
	// OutlierRows counts yields beyond the Tukey fences. They are INCLUDED in
	// every statistic below. An earlier version DELETED them against a fixed
	// (0, 60] band, which quietly moved the reported p10 from 8.593 to 9.165 --
	// it clipped exactly the left tail the band was meant to describe. The
	// median is robust to a handful of extremes on its own; what was missing
	// was any signal that an extreme was there.
	OutlierRows int     `json:"outlier_rows"`
	FenceLow    float64 `json:"outlier_fence_low"`
	FenceHigh   float64 `json:"outlier_fence_high"`
	Min         float64 `json:"min"`
	Max         float64 `json:"max"`
}

// IsMoneyMarketCategory reports whether a MUFAP category label denotes a
// money-market fund, including its Shariah-compliant and pension variants.
//
// MUFAP suffixes categories with their return convention, e.g.
// "Money Market (Annualized Return )", and prefixes pension variants with
// "VPS-", so an equality test against "Money Market" silently drops most of
// the universe.
func IsMoneyMarketCategory(cat string) bool {
	c := strings.ToLower(cat)
	return strings.Contains(c, "money market")
}

// IsAnnualizedCategory reports whether the category's returns are annualized
// rather than absolute. Mixing the two in one median compares a yield with a
// price change, which is meaningless.
func IsAnnualizedCategory(cat string) bool {
	return strings.Contains(strings.ToLower(cat), "annualized")
}

// DeriveRate computes the cross-sectional short-rate proxy for one date from
// the money-market funds' annualized returns in the given column.
//
// Only annualized money-market categories contribute. Funds that did not
// report are excluded rather than zero-filled, and are NOT counted as
// outliers: not reporting and reporting an extreme number are different facts
// and must not share a counter.
//
// Nothing that parses is discarded. Values beyond the Tukey fences are COUNTED
// into OutlierRows and the fences are echoed on the point, so a reader can see
// that an extreme is present without the statistic having been silently
// reshaped. The median is robust to a handful of extremes on its own; a
// four-digit MUFAP artefact shows up in Max and in OutlierRows rather than
// being deleted on the CLI's own judgement.
func DeriveRate(date, column string, rows []map[string]string) RatePoint {
	pt := RatePoint{Date: date, Column: column}
	vals := make([]float64, 0, len(rows))
	for _, r := range rows {
		cat := r["Category"]
		if !IsMoneyMarketCategory(cat) || !IsAnnualizedCategory(cat) {
			continue
		}
		v, ok := ParseNumber(r[column])
		if !ok {
			// Did not report. Not a value, and not an outlier either.
			continue
		}
		vals = append(vals, v)
	}
	pt.FundCount = len(vals)
	if len(vals) == 0 {
		return pt
	}
	pt.MedianYield, _ = Median(append([]float64(nil), vals...))
	pt.P10, _ = Percentile(append([]float64(nil), vals...), 10)
	pt.P90, _ = Percentile(append([]float64(nil), vals...), 90)
	pt.Min, _ = Percentile(append([]float64(nil), vals...), 0)
	pt.Max, _ = Percentile(append([]float64(nil), vals...), 100)
	if lo, hi, ok := tukeyFences(vals); ok {
		pt.FenceLow, pt.FenceHigh = lo, hi
		for _, v := range vals {
			if v < lo || v > hi {
				pt.OutlierRows++
			}
		}
	}
	return pt
}

// OutlierIQRMultiple is the Tukey "far out" multiple: a value beyond
// Q1 - k*IQR or Q3 + k*IQR is flagged as an outlier.
//
// WHY A FENCE AND NOT A FIXED BAND. An earlier attempt bounded returns to a
// fixed +/-200pp. That is unusable here because one column set is not one
// scale: "1 Day" carries a daily move while "3 Years" carries a CUMULATIVE
// return, and Pakistani equity funds legitimately sit at 190-305% over three
// years. The fixed band rejected 8 of 9 VPS-Equity funds on "3 Years" and kept
// only the launch artefact at -100.00 -- it deleted the market and preserved
// the thing it was written to remove.
//
// A fence derived from the sample's own quartiles has no scale built into it,
// so it behaves the same on a 10% yield and a 300% cumulative return.
const OutlierIQRMultiple = 3.0

// Outliers are FLAGGED, NEVER REMOVED. Percentiles are already robust to a few
// extreme values; what the reader actually lacked was any signal that an
// extreme was present. Dropping rows instead would make the emitted median and
// percentiles wrong in a way no consumer could correct without re-reading the
// mirror.

// DispersionPoint is one date's cross-sectional return spread for a category.
//
// ImplausibleRows and the band bounds are carried for the reason given on
// RatePoint, and under the same json names: the two derivations are routinely
// run over the same column of the same mirror, so they must be readable
// against each other rather than disagreeing by orders of magnitude on the
// tails with nothing in either payload to say why.
type DispersionPoint struct {
	Date      string `json:"date"`
	Category  string `json:"category"`
	Column    string `json:"column"`
	FundCount int    `json:"fund_count"`
	// OutlierRows counts values beyond the Tukey fences. They are INCLUDED in
	// every statistic below; the count exists so a spread driven by one
	// upstream artefact is visible rather than silently believed.
	OutlierRows int     `json:"outlier_rows"`
	FenceLow    float64 `json:"outlier_fence_low"`
	FenceHigh   float64 `json:"outlier_fence_high"`
	Min         float64 `json:"min"`
	Max         float64 `json:"max"`
	Median      float64 `json:"median"`
	P10         float64 `json:"p10"`
	P90         float64 `json:"p90"`
	Spread      float64 `json:"spread_p90_p10"`
}

// tukeyFences returns the far-out fences for vs. ok=false when vs is too small
// for quartiles to mean anything.
func tukeyFences(vs []float64) (low, high float64, ok bool) {
	if len(vs) < 4 {
		return 0, 0, false
	}
	q1, _ := Percentile(append([]float64(nil), vs...), 25)
	q3, _ := Percentile(append([]float64(nil), vs...), 75)
	iqr := q3 - q1
	if iqr <= 0 {
		return 0, 0, false
	}
	return q1 - OutlierIQRMultiple*iqr, q3 + OutlierIQRMultiple*iqr, true
}

// DeriveDispersion computes the p90-p10 return spread within a category.
//
// Returns outside [-DispersionBandAbs, +DispersionBandAbs] are excluded as
// upstream annualization artefacts, COUNTED into ImplausibleRows, and the band
// applied is echoed on the point. FundCount stays the count the percentiles
// were computed over. A fund that did not report is excluded without being
// counted here, exactly as in DeriveRate.
func DeriveDispersion(date, category, column string, rows []map[string]string) DispersionPoint {
	d := DispersionPoint{Date: date, Category: category, Column: column}
	vals := make([]float64, 0, len(rows))
	for _, r := range rows {
		if category != "" && !strings.Contains(strings.ToLower(r["Category"]), strings.ToLower(category)) {
			continue
		}
		v, ok := ParseNumber(r[column])
		if !ok {
			// Did not report. Not a value, and not an outlier either.
			continue
		}
		vals = append(vals, v)
	}
	d.FundCount = len(vals)
	if len(vals) == 0 {
		return d
	}
	d.Median, _ = Median(append([]float64(nil), vals...))
	d.P10, _ = Percentile(append([]float64(nil), vals...), 10)
	d.P90, _ = Percentile(append([]float64(nil), vals...), 90)
	d.Spread = d.P90 - d.P10
	d.Min, _ = Percentile(append([]float64(nil), vals...), 0)
	d.Max, _ = Percentile(append([]float64(nil), vals...), 100)
	if lo, hi, ok := tukeyFences(vals); ok {
		d.FenceLow, d.FenceHigh = lo, hi
		for _, v := range vals {
			if v < lo || v > hi {
				d.OutlierRows++
			}
		}
	}
	return d
}

// AllocationAssetFields are the asset-side percent columns of the allocation
// payload. Liabilities are deliberately excluded: they are subtracted, not
// added, when the components are netted to 100.
var AllocationAssetFields = []string{
	"Cashpercent",
	"PlacementsWithBanksandDFIsPercent",
	"PlacementsWithNBFCsPercent",
	"ReverseReposAgainstGovernmentSecuritiesPercent",
	"ReverseReposAgainstAllOtherSecuritiesPercent",
	"TFCsPercent",
	"GovernmentBackedORGuaranteedSecuritiesPercent",
	"StocksOREquitiesPercent",
	"PIBsPercent",
	"TBillsPercent",
	"IjarahSukuksPercent",
	"CommercialpapersPercent",
	"CFSPercent",
	"SpreadTransactionPercent",
	"OtherIncludingReceivablePercent",
	"OtherInvestAmountFundOfFundPercent",
}

// AllocationCheck is the outcome of the netting invariant for one fund-month.
//
// PercentsPopulated is reported separately from the netting result because
// MUFAP leaves every percent column at 0.0 for months before roughly 2024
// while the amount columns remain correct. Without this distinction an
// all-zero historical month looks like a catastrophic invariant failure
// instead of what it is: a column that was never filled in.
type AllocationCheck struct {
	Fund              string  `json:"fund"`
	Month             string  `json:"month"`
	AssetsPercent     float64 `json:"assets_percent"`
	LiabilitiesPct    float64 `json:"liabilities_percent"`
	NetPercent        float64 `json:"net_percent"`
	Deviation         float64 `json:"deviation_from_100"`
	PercentsPopulated bool    `json:"percents_populated"`
	Pass              bool    `json:"pass"`
}

// CheckAllocation applies the netting invariant: the asset-class percentages
// less the liabilities percentage must equal 100.
//
// tolerance is in percentage points. A month that fails is corrupt input, not
// a weak signal, and should be excluded before modelling. MUFAP's own
// TotalPercentage field always reads "100%" and cannot be used for this.
func CheckAllocation(fund, month string, row map[string]float64, tolerance float64) AllocationCheck {
	c := AllocationCheck{Fund: fund, Month: month}
	nonZero := 0
	for _, f := range AllocationAssetFields {
		v := row[f]
		if v != 0 {
			nonZero++
		}
		c.AssetsPercent += v
	}
	c.LiabilitiesPct = row["LaibilitiesPercent"]
	if c.LiabilitiesPct != 0 {
		nonZero++
	}
	c.PercentsPopulated = nonZero > 0
	c.NetPercent = c.AssetsPercent - c.LiabilitiesPct
	c.Deviation = c.NetPercent - 100
	// An unpopulated month cannot pass or fail the invariant; it is reported
	// as not-populated so the caller can fall back to the amount columns.
	c.Pass = c.PercentsPopulated && math.Abs(c.Deviation) <= tolerance
	return c
}
