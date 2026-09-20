package nepraparse

import (
	"math"
	"strings"
	"testing"
)

// TestSumInvariant asserts the measured reconciliation rates. On the GWh
// columns the annual Sum equals the twelve monthly values for 97/97, 104/105
// and 118/118 eligible rows; on the "% age" columns for only 7/97, 12/105 and
// 3/118. Both halves matter: the GWh rate proves the numbers were read
// correctly, and the "% age" rate proves the columns were not read in the
// other order.
func TestSumInvariant(t *testing.T) {
	for _, y := range fullYears {
		t.Run(y.fy, func(t *testing.T) {
			w := parse(t, y.name, y.fy)

			gwh := w.CheckSum(FieldGWh, DefaultSumTolerance)
			if gwh.Eligible != y.gwhEligible {
				t.Errorf("GWh eligible rows = %d, want %d", gwh.Eligible, y.gwhEligible)
			}
			if gwh.Passed != y.gwhPassed {
				t.Errorf("GWh reconciling rows = %d/%d, want %d/%d\n%s",
					gwh.Passed, gwh.Eligible, y.gwhPassed, y.gwhEligible, gwh.Summary())
			}
			if gwh.Failed != y.gwhEligible-y.gwhPassed {
				t.Errorf("GWh failures = %d, want %d", gwh.Failed, y.gwhEligible-y.gwhPassed)
			}
			if len(gwh.Mismatches) != gwh.Failed {
				t.Errorf("Mismatches has %d entries but Failed = %d", len(gwh.Mismatches), gwh.Failed)
			}
			if len(gwh.Checks) != gwh.Eligible {
				t.Errorf("Checks has %d entries but Eligible = %d", len(gwh.Checks), gwh.Eligible)
			}

			pct := w.CheckSum(FieldPct, DefaultSumTolerance)
			if pct.Eligible != y.pctEligible {
				t.Errorf("%s eligible rows = %d, want %d", MetricPct, pct.Eligible, y.pctEligible)
			}
			if pct.Passed != y.pctPassed {
				t.Errorf("%s reconciling rows = %d/%d, want %d/%d",
					MetricPct, pct.Passed, pct.Eligible, y.pctPassed, y.pctEligible)
			}

			// Eligibility accounting: every plant is either eligible or
			// skipped with a stated reason. Nothing is quietly dropped.
			if got := gwh.Eligible + len(gwh.Skipped); got != len(w.Plants) {
				t.Errorf("%d eligible + %d skipped = %d, want %d plants",
					gwh.Eligible, len(gwh.Skipped), got, len(w.Plants))
			}
			for _, s := range gwh.Skipped {
				if s.Reason == 0 {
					t.Errorf("%s was skipped with no reason", s.Plant)
				}
			}
			// Skipped rows are exactly the blank rows plus the status rows.
			wantSkipped := y.notReportedRows + y.statusRows
			if len(gwh.Skipped) != wantSkipped {
				t.Errorf("skipped rows = %d, want %d (%d blank + %d status)",
					len(gwh.Skipped), wantSkipped, y.notReportedRows, y.statusRows)
			}
		})
	}
}

// TestSumInvariantReportsTheRealFY2021Mismatch: one FY2020-21 row genuinely
// does not reconcile. It must be reported with its numbers, not smoothed away
// by a looser tolerance and not hidden behind a pass count.
func TestSumInvariantReportsTheRealFY2021Mismatch(t *testing.T) {
	w := parse(t, fullFY2021, "2020-21")
	rep := w.CheckSum(FieldGWh, DefaultSumTolerance)

	if len(rep.Mismatches) != 1 {
		t.Fatalf("mismatches = %+v, want exactly one", rep.Mismatches)
	}
	m := rep.Mismatches[0]
	if m.Plant != "(NPPCL) - Balloki" {
		t.Errorf("mismatching plant = %q, want %q", m.Plant, "(NPPCL) - Balloki")
	}
	if math.Abs(m.MonthlySum-5945.21) > 0.005 {
		t.Errorf("monthly total = %.2f, want 5945.21", m.MonthlySum)
	}
	if math.Abs(m.Reported-5905.65) > 0.005 {
		t.Errorf("reported Sum = %.2f, want 5905.65", m.Reported)
	}
	if math.Abs(m.Delta-39.56) > 0.005 {
		t.Errorf("delta = %+.2f, want +39.56", m.Delta)
	}
	if m.OK {
		t.Error("the mismatching row must not be marked OK")
	}
	if !strings.Contains(rep.Summary(), "(NPPCL) - Balloki") {
		t.Errorf("Summary() hides the mismatch:\n%s", rep.Summary())
	}
	if !strings.Contains(rep.Summary(), "104/105") {
		t.Errorf("Summary() = %q, want it to state 104/105", rep.Summary())
	}
}

// TestSumToleranceIsNotDoingTheWork: the discrimination comes from the data,
// not from the threshold. Every reconciling row is within 0.03 and the single
// real failure is off by 39.56, so the result is stable across any sane
// tolerance.
func TestSumToleranceIsNotDoingTheWork(t *testing.T) {
	for _, y := range fullYears {
		t.Run(y.fy, func(t *testing.T) {
			w := parse(t, y.name, y.fy)
			for _, tol := range []float64{0.04, DefaultSumTolerance, 0.5, 5.0} {
				rep := w.CheckSum(FieldGWh, tol)
				if rep.Passed != y.gwhPassed {
					t.Errorf("tolerance %.2f: %d/%d rows reconcile, want %d",
						tol, rep.Passed, rep.Eligible, y.gwhPassed)
				}
			}
			// The largest residual among reconciling rows.
			worst := 0.0
			for _, c := range w.CheckSum(FieldGWh, DefaultSumTolerance).Checks {
				if c.OK && math.Abs(c.Delta) > worst {
					worst = math.Abs(c.Delta)
				}
			}
			// The measured worst case is 0.03; the extra 1e-4 is float
			// accumulation over twelve two-decimal addends, not slack.
			if worst > 0.0301 {
				t.Errorf("largest residual among reconciling rows = %.4f, want <= 0.03 (published rounding)", worst)
			}
			// A zero tolerance selects the default rather than passing
			// nothing.
			if got := w.CheckSum(FieldGWh, 0).Tolerance; got != DefaultSumTolerance {
				t.Errorf("tolerance 0 gave %v, want the default %v", got, DefaultSumTolerance)
			}
		})
	}
}

// TestColumnOrderIsProvenFromTheData is the column-order proof. Sum ==
// sum(12 months) is an identity for energy and not for a utilisation ratio,
// so whichever member of each pair reconciles is the GWh column. If the pair
// order ever flips, this reports it — while every individual number still
// looks plausible.
func TestColumnOrderIsProvenFromTheData(t *testing.T) {
	for _, y := range fullYears {
		t.Run(y.fy, func(t *testing.T) {
			w := parse(t, y.name, y.fy)
			v := w.ColumnOrder(DefaultSumTolerance)
			if !v.PctFirst {
				t.Fatalf("PctFirst = false: %s", v.Explanation)
			}
			gwhRatio, _ := v.GWh.Ratio()
			pctRatio, _ := v.Pct.Ratio()
			if gwhRatio-pctRatio < ColumnOrderMargin {
				t.Errorf("margin %.3f (GWh %.3f, %s %.3f) is below %.2f",
					gwhRatio-pctRatio, gwhRatio, MetricPct, pctRatio, ColumnOrderMargin)
			}
			if v.GWh.Passed != y.gwhPassed || v.Pct.Passed != y.pctPassed {
				t.Errorf("verdict reports %d/%d GWh and %d/%d %s, want %d/%d and %d/%d",
					v.GWh.Passed, v.GWh.Eligible, v.Pct.Passed, v.Pct.Eligible, MetricPct,
					y.gwhPassed, y.gwhEligible, y.pctPassed, y.pctEligible)
			}
			if !strings.Contains(v.Explanation, MetricGWh) {
				t.Errorf("Explanation does not name the metrics: %q", v.Explanation)
			}
		})
	}
}

// TestColumnOrderDetectsASwap swaps the two members of every month pair in a
// fixture and asserts the verdict flips. This is the test that would have
// caught a percent/GWh transposition.
func TestColumnOrderDetectsASwap(t *testing.T) {
	w := parse(t, fullFY2324, "2023-24")
	if !w.ColumnOrder(DefaultSumTolerance).PctFirst {
		t.Fatal("baseline verdict is already wrong")
	}

	// Swap Utilisation and Generation in every observation — exactly what a
	// parser that read "GWh" first would produce.
	swapped := *w
	swapped.Plants = make([]Plant, len(w.Plants))
	copy(swapped.Plants, w.Plants)
	for i := range swapped.Plants {
		p := &swapped.Plants[i]
		for j := range p.Months {
			p.Months[j].Utilisation, p.Months[j].Generation = p.Months[j].Generation, p.Months[j].Utilisation
		}
		p.Total.Utilisation, p.Total.Generation = p.Total.Generation, p.Total.Utilisation
	}

	v := swapped.ColumnOrder(DefaultSumTolerance)
	if v.PctFirst {
		t.Error("a swapped pair order was not detected")
	}
	if v.GWh.Passed != 3 || v.Pct.Passed != 118 {
		t.Errorf("after the swap: GWh %d/%d, %s %d/%d — want the rates to have exchanged (3 and 118)",
			v.GWh.Passed, v.GWh.Eligible, MetricPct, v.Pct.Passed, v.Pct.Eligible)
	}
	if !strings.Contains(v.Explanation, "must not be trusted") {
		t.Errorf("Explanation does not warn the caller: %q", v.Explanation)
	}
}

// TestTarbelaPairOrder is the direct spot check named in the measurement:
// FY2023-24 Tarbela column 6 is 94.94 (a utilisation percentage) and column 7
// is 2,456.58 (GWh). 2,456.58 GWh over July on 3,478 MW is 94.9% utilisation,
// which is the arithmetic that identifies which is which.
func TestTarbelaPairOrder(t *testing.T) {
	w := parse(t, fullFY2324, "2023-24")
	p, ok := w.PlantByName("Tarbela Hydropower Project (WAPDA)")
	if !ok {
		t.Fatal("Tarbela missing")
	}
	if p.SNo != 1 {
		t.Errorf("S.No = %d, want 1", p.SNo)
	}
	if got, _ := p.InstalledCapacity.Float64(); got != 3478 {
		t.Errorf("InstalledCapacity = %v, want 3478", got)
	}
	if !p.InstalledCapacity.HadThousandsSeparator() {
		t.Error(`InstalledCapacity was published as "3,478"; the comma flag should be set`)
	}

	jul, ok := p.Month(Jul)
	if !ok {
		t.Fatal("Jul observation missing")
	}
	if jul.Month != Jul {
		t.Errorf("first monthly observation is %v, want Jul (fiscal order)", jul.Month)
	}
	if got, _ := jul.Utilisation.Float64(); got != 94.94 {
		t.Errorf("Jul %s = %v, want 94.94", MetricPct, got)
	}
	if got, _ := jul.Generation.Float64(); got != 2456.58 {
		t.Errorf("Jul %s = %v, want 2456.58", MetricGWh, got)
	}
	if got, _ := p.Total.Generation.Float64(); got != 13365.93 {
		t.Errorf("Sum %s = %v, want 13365.93", MetricGWh, got)
	}
	if !p.Total.IsTotal || p.Total.Label() != "Sum" {
		t.Errorf("Total.Label() = %q (IsTotal=%v), want %q", p.Total.Label(), p.Total.IsTotal, "Sum")
	}

	// The arithmetic that settles which column is which: GWh / (MW * hours)
	// must land on the published percentage.
	const julyHours = 31 * 24
	mw, _ := p.InstalledCapacity.Float64()
	gwh, _ := jul.Generation.Float64()
	pctFromEnergy := gwh * 1000 / (mw * julyHours) * 100
	published, _ := jul.Utilisation.Float64()
	if math.Abs(pctFromEnergy-published) > 1.0 {
		t.Errorf("energy-derived utilisation %.2f%% does not match the published %.2f%%; the pair may be transposed",
			pctFromEnergy, published)
	}
}

func TestSumFieldString(t *testing.T) {
	if got := FieldGWh.String(); got != "GWh" {
		t.Errorf("FieldGWh = %q, want GWh", got)
	}
	if got := FieldPct.String(); got != "% age" {
		t.Errorf("FieldPct = %q, want %q", got, "% age")
	}
}

func TestSkipReasonsAreStated(t *testing.T) {
	w := parse(t, fullFY2324, "2023-24")
	rep := w.CheckSum(FieldGWh, DefaultSumTolerance)
	counts := map[SkipReason]int{}
	for _, s := range rep.Skipped {
		counts[s.Reason]++
	}
	// 13 status rows (12 DELICENSED + 1 DECOMMISSIONED) and 2 blank rows.
	if counts[SkipStatus] != 13 {
		t.Errorf("status skips = %d, want 13", counts[SkipStatus])
	}
	if counts[SkipNotReported] != 2 {
		t.Errorf("not-reported skips = %d, want 2", counts[SkipNotReported])
	}
	if got := SkipNotReported.String(); got != "not_reported" {
		t.Errorf("SkipNotReported = %q", got)
	}
	if got := SkipStatus.String(); got != "status" {
		t.Errorf("SkipStatus = %q", got)
	}
	if _, ok := (SumReport{}).Ratio(); ok {
		t.Error("Ratio() on an empty report must report ok = false")
	}
}
