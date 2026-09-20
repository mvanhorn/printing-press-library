package nepraparse

import (
	"fmt"
	"math"
	"strings"
)

// SumField selects which of the two published metrics the Sum invariant is
// evaluated on.
type SumField uint8

const (
	// FieldGWh is the "GWh" column of each pair — energy generated.
	FieldGWh SumField = iota
	// FieldPct is the "% age" column of each pair — plant utilisation. A
	// twelve-month utilisation does not sum to an annual utilisation, so
	// this field is expected to FAIL the invariant; that expected failure is
	// exactly what makes the check a column-order proof.
	FieldPct
)

// String names the field with the workbook's own label.
func (f SumField) String() string {
	if f == FieldPct {
		return MetricPct
	}
	return MetricGWh
}

// DefaultSumTolerance is the absolute tolerance for Sum == sum(12 months).
//
// The workbooks publish two decimals, so each of the twelve addends carries
// up to 0.005 of rounding and the accumulated worst case is 0.06. The
// measured spread is well inside that: the largest residual among reconciling
// rows in any sampled year is 0.03, while the single genuine mismatch is
// 39.56, so the threshold is not doing any of the discriminating work.
const DefaultSumTolerance = 0.06

// SumCheck is one plant's result for the Sum invariant.
type SumCheck struct {
	// SNo and Plant identify the row. SNo is an in-year ordinal only.
	SNo   int
	Plant string
	// RowIndex is the source table row.
	RowIndex int
	// MonthlySum is the total of the twelve monthly values.
	MonthlySum float64
	// Reported is the value published in the "Sum" column.
	Reported float64
	// Delta is MonthlySum - Reported.
	Delta float64
	// OK reports whether |Delta| <= the tolerance in force.
	OK bool
}

// String renders one check compactly for a report.
func (c SumCheck) String() string {
	verdict := "FAIL"
	if c.OK {
		verdict = "ok"
	}
	return fmt.Sprintf("%s: %s months=%.2f sum=%.2f delta=%+.2f", verdict, c.Plant, c.MonthlySum, c.Reported, c.Delta)
}

// SkipReason says why a plant was not eligible for the invariant.
type SkipReason uint8

const (
	// SkipNotReported means at least one of the thirteen cells is an NBSP
	// blank. A blank is not a zero, so the row cannot be summed at all —
	// substituting zero here is precisely the corruption this package
	// exists to prevent.
	SkipNotReported SkipReason = iota + 1
	// SkipStatus means the row's monthly block is a status sentinel
	// (DELICENSED / DECOMMISSIONED / Export to K.Electric).
	SkipStatus
	// SkipUnknownText means at least one cell held unmodelled text.
	SkipUnknownText
)

// String names the skip reason.
func (r SkipReason) String() string {
	switch r {
	case SkipNotReported:
		return "not_reported"
	case SkipStatus:
		return "status"
	case SkipUnknownText:
		return "unknown_text"
	}
	return "eligible"
}

// SumSkip records a row excluded from the invariant, and why. Skips are
// reported rather than silently folded into the pass count.
type SumSkip struct {
	SNo      int
	Plant    string
	RowIndex int
	Reason   SkipReason
}

// SumReport is the result of the Sum invariant over a whole workbook.
//
// On the GWh columns this identity holds for 97/97 (FY2017-18), 104/105
// (FY2020-21) and 118/118 (FY2023-24) eligible rows. On the "% age" columns
// it holds for only 7/97, 12/105 and 3/118. That asymmetry is what proves the
// parser read "% age" first and "GWh" second: read the pair backwards and the
// two ratios swap.
type SumReport struct {
	// Field is the metric checked.
	Field SumField
	// Tolerance is the absolute tolerance applied.
	Tolerance float64
	// Eligible is the number of rows whose thirteen cells are all numeric.
	Eligible int
	// Passed and Failed partition Eligible.
	Passed int
	Failed int
	// Checks holds every eligible row's result, in source order.
	Checks []SumCheck
	// Mismatches holds only the failures, for direct reporting.
	Mismatches []SumCheck
	// Skipped holds the ineligible rows and their reasons.
	Skipped []SumSkip
}

// Ratio returns Passed/Eligible, and false when nothing was eligible.
func (r SumReport) Ratio() (float64, bool) {
	if r.Eligible == 0 {
		return 0, false
	}
	return float64(r.Passed) / float64(r.Eligible), true
}

// SummaryMismatchLimit is how many mismatches [SumReport.Summary] spells out
// before it switches to a count. The GWh series has at most one in any
// sampled year; the "% age" series has dozens by design, and a summary that
// printed them all would bury the headline.
const SummaryMismatchLimit = 5

// Summary renders the headline "104/105" plus the first few mismatches. The
// FY2020-21 mismatch ((NPPCL) - Balloki) is real and is reported, not hidden;
// [SumReport.Mismatches] always holds the complete list.
func (r SumReport) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %d/%d rows reconcile (tolerance %.2f, %d skipped)",
		r.Field, r.Passed, r.Eligible, r.Tolerance, len(r.Skipped))
	for i, m := range r.Mismatches {
		if i == SummaryMismatchLimit {
			fmt.Fprintf(&b, "\n  ... and %d more mismatches", len(r.Mismatches)-i)
			break
		}
		fmt.Fprintf(&b, "\n  mismatch %s", m)
	}
	return b.String()
}

// CheckSum evaluates Sum == sum(12 monthly values) for one metric.
//
// A tolerance <= 0 selects [DefaultSumTolerance]. Rows containing a blank, a
// status sentinel or unmodelled text are skipped and listed in
// [SumReport.Skipped]; they are never treated as zeros.
func (w *Workbook) CheckSum(field SumField, tolerance float64) SumReport {
	if tolerance <= 0 {
		tolerance = DefaultSumTolerance
	}
	rep := SumReport{Field: field, Tolerance: tolerance}
	for _, p := range w.Plants {
		vals, reason := seriesFor(p, field)
		if reason != 0 {
			rep.Skipped = append(rep.Skipped, SumSkip{SNo: p.SNo, Plant: p.Name, RowIndex: p.RowIndex, Reason: reason})
			continue
		}
		var months float64
		for i := 0; i < 12; i++ {
			months += vals[i]
		}
		chk := SumCheck{
			SNo:        p.SNo,
			Plant:      p.Name,
			RowIndex:   p.RowIndex,
			MonthlySum: months,
			Reported:   vals[SumPairIndex],
			Delta:      months - vals[SumPairIndex],
		}
		chk.OK = math.Abs(chk.Delta) <= tolerance
		rep.Eligible++
		if chk.OK {
			rep.Passed++
		} else {
			rep.Failed++
			rep.Mismatches = append(rep.Mismatches, chk)
		}
		rep.Checks = append(rep.Checks, chk)
	}
	return rep
}

// seriesFor extracts a plant's thirteen values for one metric, or the reason
// the row is ineligible.
func seriesFor(p Plant, field SumField) ([PairCount]float64, SkipReason) {
	var out [PairCount]float64
	if p.Status.Status() {
		return out, SkipStatus
	}
	obs := append(p.Months[:], p.Total)
	for i, o := range obs {
		v := o.Generation
		if field == FieldPct {
			v = o.Utilisation
		}
		n, ok := v.Float64()
		if !ok {
			switch v.State() {
			case StateNotReported:
				return out, SkipNotReported
			case StateUnknownText:
				return out, SkipUnknownText
			default:
				return out, SkipStatus
			}
		}
		out[i] = n
	}
	return out, 0
}

// ColumnOrderVerdict is the outcome of proving column order from the data.
type ColumnOrderVerdict struct {
	// GWh and Pct are the two Sum reports the verdict rests on.
	GWh SumReport
	Pct SumReport
	// PctFirst reports whether the parser's assumption — "% age" in the
	// first column of each pair, "GWh" in the second — is confirmed. It is
	// true when the GWh series reconciles far better than the "% age"
	// series, which is only possible if the columns were read in that order.
	PctFirst bool
	// Explanation states the evidence in one line.
	Explanation string
}

// ColumnOrderMargin is how much better the GWh series must reconcile than
// the "% age" series before the ordering is treated as proven. The measured
// margins are large — 1.00 vs 0.07, 0.99 vs 0.11, 1.00 vs 0.03 — so a
// half-the-rows threshold is comfortably clear of noise.
const ColumnOrderMargin = 0.5

// ColumnOrder proves, from the published numbers alone, that each month pair
// is "% age" then "GWh" rather than the reverse.
//
// Sum == sum(12 months) is an identity for energy and not for a utilisation
// ratio, so whichever member of the pair reconciles is the GWh column. If a
// future file swapped the two, this check would report PctFirst false while
// every individual number still looked plausible — which is the whole reason
// it exists.
func (w *Workbook) ColumnOrder(tolerance float64) ColumnOrderVerdict {
	v := ColumnOrderVerdict{
		GWh: w.CheckSum(FieldGWh, tolerance),
		Pct: w.CheckSum(FieldPct, tolerance),
	}
	gwhRatio, gwhOK := v.GWh.Ratio()
	pctRatio, _ := v.Pct.Ratio()
	if !gwhOK {
		v.Explanation = "no row had all thirteen numeric cells in either metric; column order not provable from this input"
		return v
	}
	v.PctFirst = gwhRatio-pctRatio >= ColumnOrderMargin
	if v.PctFirst {
		v.Explanation = fmt.Sprintf(
			"Sum == sum(12 months) holds for %d/%d rows on the second column of each pair (%s) but only %d/%d on the first (%s); the second column is therefore energy and the first is utilisation",
			v.GWh.Passed, v.GWh.Eligible, MetricGWh, v.Pct.Passed, v.Pct.Eligible, MetricPct)
		return v
	}
	v.Explanation = fmt.Sprintf(
		"the annual-total identity does NOT single out the second column of each pair (%s: %d/%d, %s: %d/%d); the pair order may have changed and the monthly fields must not be trusted",
		MetricGWh, v.GWh.Passed, v.GWh.Eligible, MetricPct, v.Pct.Passed, v.Pct.Eligible)
	return v
}
