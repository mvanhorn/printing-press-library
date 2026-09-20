package nepraparse

import (
	"fmt"
	"strings"
	"testing"
)

// TestCensusReportIsSelfConsistent walks each published year and asserts the
// census against the plants it describes, recomputed independently of the
// parser's own accumulator. It also logs the full census so a reviewer can
// read the numbers straight out of `go test -v` rather than taking the
// table's word for them.
func TestCensusReportIsSelfConsistent(t *testing.T) {
	for _, y := range fullYears {
		t.Run(y.fy, func(t *testing.T) {
			w := parse(t, y.name, y.fy)
			recomputed := recount(w.Plants)
			if recomputed != w.Census {
				t.Errorf("census disagrees with an independent recount:\n got %+v\nwant %+v", w.Census, recomputed)
			}
			t.Log("\n" + censusReport(w))
		})
	}
}

// recount rebuilds the census from the parsed plants using a deliberately
// different traversal from censusOf, so the two agreeing means something.
func recount(plants []Plant) Census {
	c := Census{Rows: len(plants), MonthlyCellsTotal: len(plants) * MonthlyCells}
	for _, p := range plants {
		var blanks, zeros int
		cells := make([]Value, 0, MonthlyCells)
		for _, m := range MonthsInFiscalOrder {
			obs, _ := p.Month(m)
			cells = append(cells, obs.Utilisation, obs.Generation)
		}
		cells = append(cells, p.Total.Utilisation, p.Total.Generation)
		for _, v := range cells {
			switch v.State() {
			case StateNumeric:
				c.Present++
				if n, _ := v.Float64(); n == 0 {
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
		for _, v := range []Value{p.InstalledCapacity, p.DependableCapacity} {
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

func censusReport(w *Workbook) string {
	c := w.Census
	var b strings.Builder
	f := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }
	f("FY %s  charset=%s  physical=%d logical=%d  rows=%d (%d plants, 3 band, %d blank, 1 title)\n",
		w.FiscalYear.Label(), w.Charset, w.PhysicalWidth, w.LogicalWidth, w.TableRows, len(w.Plants), w.SeparatorRows)
	f("  tables=%d  <th>=%d  raw <td>=%d  S.No contiguous=%v  names unique=%v\n",
		w.TableCount, w.HeaderCellCount, w.RawCells, w.SNoContiguous, w.NamesUnique)
	f("  monthly cells %d = present %d + not_reported %d + delicensed %d + decommissioned %d + export %d + unknown %d (balanced=%v)\n",
		c.MonthlyCellsTotal, c.Present, c.NotReported, c.Delicensed, c.Decommissioned,
		c.ExportToKElectric, c.UnknownText, c.Balanced())
	f("  measured 0.00 cells=%d  thousands-separated cells=%d  residue cells=%d\n",
		c.MeasuredZeros, c.ThousandsSeparated, c.ResidueCells)
	f("  rows with blanks=%d (all-26 blanks=%d)  rows mixing blank+zero=%d  status rows=%d\n",
		c.NotReportedRows, c.FullyNotReportedRows, c.MixedBlankZeroRows, c.StatusRows)
	f("  capacity cells=%d present=%d not_reported=%d status=%d\n",
		c.CapacityCells, c.CapacityPresent, c.CapacityNotReported, c.CapacityStatus)
	v := w.ColumnOrder(DefaultSumTolerance)
	f("  %s\n  %s\n", v.GWh.Summary(), v.Pct.Summary())
	f("  column order: %% age first = %v\n", v.PctFirst)
	for _, warn := range w.Warnings {
		f("  warning: %s\n", warn)
	}
	return b.String()
}
