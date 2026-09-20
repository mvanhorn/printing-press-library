package nepraparse

import (
	"testing"
)

// TestParseWorkbookRowReconciliation reconciles every physical table row to
// something, so no row can be silently dropped: rows = plants + 3 header band
// + 2 blank separators + 1 title.
func TestParseWorkbookRowReconciliation(t *testing.T) {
	for _, y := range fullYears {
		t.Run(y.fy, func(t *testing.T) {
			w := parse(t, y.name, y.fy)

			if len(w.Plants) != y.plants {
				t.Errorf("plants = %d, want %d", len(w.Plants), y.plants)
			}
			if w.SeparatorRows != y.separatorRows {
				t.Errorf("SeparatorRows = %d, want %d", w.SeparatorRows, y.separatorRows)
			}
			const headerBandRows, titleRows = 3, 1
			if got := len(w.Plants) + headerBandRows + w.SeparatorRows + titleRows; got != y.tableRows {
				t.Errorf("row reconciliation: %d plants + %d band + %d blank + %d title = %d, want %d table rows",
					len(w.Plants), headerBandRows, w.SeparatorRows, titleRows, got, y.tableRows)
			}
			if w.TableRows != y.tableRows {
				t.Errorf("TableRows = %d, want %d", w.TableRows, y.tableRows)
			}
			if w.RawCells != y.rawCells {
				t.Errorf("RawCells = %d, want %d", w.RawCells, y.rawCells)
			}
			if w.TableCount != 1 {
				t.Errorf("TableCount = %d, want 1", w.TableCount)
			}
			if w.HeaderCellCount != 0 {
				t.Errorf("HeaderCellCount = %d, want 0", w.HeaderCellCount)
			}
			if w.PhysicalWidth != y.physicalWidth {
				t.Errorf("PhysicalWidth = %d, want %d", w.PhysicalWidth, y.physicalWidth)
			}
			if w.LogicalWidth != LogicalColumns {
				t.Errorf("LogicalWidth = %d, want %d", w.LogicalWidth, LogicalColumns)
			}
			if w.FiscalYear.Label() != y.fy {
				t.Errorf("FiscalYear = %s, want %s", w.FiscalYear.Label(), y.fy)
			}

			// Every numbered row is a plant: there are no subtotal or
			// grand-total rows to filter out.
			if !w.SNoContiguous {
				t.Error("SNoContiguous = false, want true — S.No must run 1..N with no gaps or duplicates")
			}
			if !w.NamesUnique {
				t.Error("NamesUnique = false, want true")
			}
			for _, p := range w.Plants {
				if p.Name == "" {
					t.Errorf("row %d has an empty name", p.RowIndex)
				}
				if looksLikeTotalRow(p.Name) {
					t.Errorf("row %d (%q) looks like a subtotal row; the measured files have none", p.RowIndex, p.Name)
				}
			}
		})
	}
}

func looksLikeTotalRow(name string) bool {
	for _, bad := range []string{"Total", "TOTAL", "Grand", "Sub-total", "Subtotal"} {
		if name == bad {
			return true
		}
	}
	return false
}

// TestCensusGroundTruth asserts every measured cell-state count. The census
// is the callers' proof of null discipline, so if it drifts the parse has
// changed meaning even when nothing errors.
func TestCensusGroundTruth(t *testing.T) {
	for _, y := range fullYears {
		t.Run(y.fy, func(t *testing.T) {
			c := parse(t, y.name, y.fy).Census

			checks := []struct {
				label     string
				got, want int
			}{
				{"Rows", c.Rows, y.plants},
				{"MonthlyCellsTotal", c.MonthlyCellsTotal, y.plants * MonthlyCells},
				{"Present", c.Present, y.present},
				{"NotReported", c.NotReported, y.notReported},
				{"Delicensed", c.Delicensed, y.delicensed},
				{"Decommissioned", c.Decommissioned, y.decommissioned},
				{"ExportToKElectric (monthly block)", c.ExportToKElectric, 0},
				{"UnknownText (monthly block)", c.UnknownText, 0},
				{"MeasuredZeros", c.MeasuredZeros, y.measuredZeros},
				{"ThousandsSeparated", c.ThousandsSeparated, y.thousandsSeparated},
				{"NotReportedRows", c.NotReportedRows, y.notReportedRows},
				{"FullyNotReportedRows", c.FullyNotReportedRows, y.notReportedRows},
				{"StatusRows", c.StatusRows, y.statusRows},
				{"CapacityCells", c.CapacityCells, y.plants * 2},
				{"CapacityStatus", c.CapacityStatus, y.capacityStatus},
				{"ResidueCells", c.ResidueCells, y.residueCells},
			}
			for _, ch := range checks {
				if ch.got != ch.want {
					t.Errorf("Census.%s = %d, want %d", ch.label, ch.got, ch.want)
				}
			}
			if !c.Balanced() {
				t.Errorf("census does not balance: %d+%d+%d+%d+%d+%d != %d monthly cells",
					c.Present, c.NotReported, c.Delicensed, c.Decommissioned,
					c.ExportToKElectric, c.UnknownText, c.MonthlyCellsTotal)
			}
		})
	}
}

// TestBlankIsNeverZero is the central correctness proof, in three parts.
//
//  1. The blank pattern is strictly all-or-nothing per row: any row with a
//     blank in the monthly block has exactly 26 blanks and exactly zero "0.00"
//     cells, and no row in any year mixes the two.
//  2. "0.00" only ever occurs in otherwise fully-populated rows.
//  3. Longitudinally, the same plant is blank in one year and reports explicit
//     zeros alongside real output in another — so the two cannot be the same
//     thing.
func TestBlankIsNeverZero(t *testing.T) {
	for _, y := range fullYears {
		t.Run(y.fy+" all-or-nothing", func(t *testing.T) {
			w := parse(t, y.name, y.fy)
			if w.Census.MixedBlankZeroRows != 0 {
				t.Fatalf("MixedBlankZeroRows = %d, want 0 — a row mixing a blank with a 0.00 would break the blank/zero distinction",
					w.Census.MixedBlankZeroRows)
			}
			blankRows := 0
			for _, p := range w.Plants {
				blanks, zeros := 0, 0
				for _, obs := range append(p.Months[:], p.Total) {
					for _, v := range [2]Value{obs.Utilisation, obs.Generation} {
						if v.State() == StateNotReported {
							blanks++
						}
						if n, ok := v.Float64(); ok && n == 0 {
							zeros++
						}
					}
				}
				if blanks == 0 {
					continue
				}
				blankRows++
				if blanks != MonthlyCells {
					t.Errorf("%s has %d blanks, want all %d (blank is all-or-nothing per row)", p.Name, blanks, MonthlyCells)
				}
				if zeros != 0 {
					t.Errorf("%s has %d blanks AND %d zeros in one row", p.Name, blanks, zeros)
				}
			}
			if blankRows != y.notReportedRows {
				t.Errorf("rows with blanks = %d, want %d", blankRows, y.notReportedRows)
			}
			if w.Census.MeasuredZeros != y.measuredZeros {
				t.Errorf("MeasuredZeros = %d, want %d", w.Census.MeasuredZeros, y.measuredZeros)
			}
		})
	}

	t.Run("K-2 longitudinally", func(t *testing.T) {
		const k2 = "Karachi Nuclear Power Plant-II (K-2)"

		// FY2017-18: licensed, not yet built. Every monthly cell blank AND
		// blank capacity. A parser that reads these as zeros invents a plant
		// that generated nothing when in fact nothing was measured.
		early := parse(t, fullFY1718, "2017-18")
		p, ok := early.PlantByName(k2)
		if !ok {
			t.Fatalf("%s missing from FY2017-18", k2)
		}
		for _, obs := range append(p.Months[:], p.Total) {
			for _, v := range [2]Value{obs.Utilisation, obs.Generation} {
				if v.State() != StateNotReported {
					t.Fatalf("FY2017-18 %s %s: state = %v, want %v", k2, obs.Label(), v.State(), StateNotReported)
				}
			}
		}
		if p.InstalledCapacity.State() != StateNotReported || p.DependableCapacity.State() != StateNotReported {
			t.Errorf("FY2017-18 %s capacity = %v/%v, want both not reported",
				k2, p.InstalledCapacity.State(), p.DependableCapacity.State())
		}

		// FY2020-21: the SAME plant reports sixteen explicit 0.00 month cells
		// alongside a non-zero annual Sum of 1,705.91 GWh. A measured zero
		// coexists with real output; a blank does not.
		later := parse(t, fullFY2021, "2020-21")
		p, ok = later.PlantByName(k2)
		if !ok {
			t.Fatalf("%s missing from FY2020-21", k2)
		}
		zeros, blanks := 0, 0
		for _, obs := range p.Months {
			for _, v := range [2]Value{obs.Utilisation, obs.Generation} {
				switch {
				case v.State() == StateNotReported:
					blanks++
				default:
					if n, ok := v.Float64(); ok && n == 0 {
						zeros++
					}
				}
			}
		}
		if zeros != 16 {
			t.Errorf("FY2020-21 %s: explicit 0.00 month cells = %d, want 16", k2, zeros)
		}
		if blanks != 0 {
			t.Errorf("FY2020-21 %s: blanks = %d, want 0", k2, blanks)
		}
		sum, ok := p.Total.Generation.Float64()
		if !ok {
			t.Fatalf("FY2020-21 %s: annual Sum is not a number (%v)", k2, p.Total.Generation)
		}
		if sum != 1705.91 {
			t.Errorf("FY2020-21 %s: Sum GWh = %v, want 1705.91", k2, sum)
		}
		if sum == 0 {
			t.Error("a plant with sixteen zero months still generated 1,705.91 GWh over the year")
		}
	})

	// The same blank-then-zero pattern for two more plants named in the
	// measurement, so the K-2 case is not a one-off.
	t.Run("hydro projects under construction", func(t *testing.T) {
		early := parse(t, fullFY1718, "2017-18")
		later := parse(t, fullFY2021, "2020-21")
		for _, name := range []string{
			"Tarbela Ext. 04 Hydropower Project (WAPDA)",
			"Golen Gol Hydropower Project (WAPDA)",
		} {
			before, ok := early.PlantByName(name)
			if !ok {
				t.Fatalf("%s missing from FY2017-18", name)
			}
			if before.Months[0].Generation.State() != StateNotReported {
				t.Errorf("FY2017-18 %s Jul GWh = %v, want not reported", name, before.Months[0].Generation)
			}
			after, ok := later.PlantByName(name)
			if !ok {
				t.Fatalf("%s missing from FY2020-21", name)
			}
			if !after.Total.Generation.Present() {
				t.Errorf("FY2020-21 %s Sum GWh = %v, want a real measurement", name, after.Total.Generation)
			}
		}
	})
}

// TestStatusSentinelsKeepCapacity: DELICENSED, DECOMMISSIONED and
// "Export to K.Electric" must not null a plant's capacity.
func TestStatusSentinelsKeepCapacity(t *testing.T) {
	fy2324 := parse(t, fullFY2324, "2023-24")
	fy2021 := parse(t, fullFY2021, "2020-21")

	tests := []struct {
		name           string
		workbook       *Workbook
		plant          string
		wantStatus     CellState
		wantInstalled  float64
		wantDependable float64
	}{
		{
			name: "DELICENSED keeps 174/120 MW", workbook: fy2324, plant: "Kotri Power Station",
			wantStatus: StateDelicensed, wantInstalled: 174, wantDependable: 120,
		},
		{
			name: "DELICENSED keeps 150/30 MW", workbook: fy2324, plant: "Lakhra Power Generation Company Limited.",
			wantStatus: StateDelicensed, wantInstalled: 150, wantDependable: 30,
		},
		{
			name: "DECOMMISSIONED keeps 140/129 MW", workbook: fy2324,
			plant:      "Habibullah Coastal Power Company (PVT) Limited. (HCPC)",
			wantStatus: StateDecommissioned, wantInstalled: 140, wantDependable: 129,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, ok := tc.workbook.PlantByName(tc.plant)
			if !ok {
				t.Fatalf("%s missing", tc.plant)
			}
			if p.Status != tc.wantStatus {
				t.Errorf("Status = %v, want %v", p.Status, tc.wantStatus)
			}
			// All 26 monthly cells carry the sentinel, because the source
			// writes one <td colspan=26>.
			for _, obs := range append(p.Months[:], p.Total) {
				if obs.Utilisation.State() != tc.wantStatus || obs.Generation.State() != tc.wantStatus {
					t.Fatalf("%s: %v/%v, want both %v", obs.Label(), obs.Utilisation.State(), obs.Generation.State(), tc.wantStatus)
				}
			}
			// ... and the capacity survives.
			inst, ok := p.InstalledCapacity.Float64()
			if !ok || inst != tc.wantInstalled {
				t.Errorf("InstalledCapacity = %v (ok=%v), want %v", inst, ok, tc.wantInstalled)
			}
			dep, ok := p.DependableCapacity.Float64()
			if !ok || dep != tc.wantDependable {
				t.Errorf("DependableCapacity = %v (ok=%v), want %v", dep, ok, tc.wantDependable)
			}
		})
	}

	// FY2023-24 has exactly 12 DELICENSED rows and 1 DECOMMISSIONED row, and
	// every one of them keeps a real installed capacity.
	t.Run("row counts in FY2023-24", func(t *testing.T) {
		deli, deco := 0, 0
		for _, p := range fy2324.Plants {
			switch p.Status {
			case StateDelicensed:
				deli++
			case StateDecommissioned:
				deco++
			default:
				continue
			}
			if !p.InstalledCapacity.Present() {
				t.Errorf("%s is %v but lost its installed capacity (%v)", p.Name, p.Status, p.InstalledCapacity)
			}
		}
		if deli != 12 {
			t.Errorf("DELICENSED rows = %d, want 12", deli)
		}
		if deco != 1 {
			t.Errorf("DECOMMISSIONED rows = %d, want 1", deco)
		}
		if got := deli * MonthlyCells; got != 312 {
			t.Errorf("DELICENSED expanded cells = %d, want 312", got)
		}
	})

	// "Export to K.Electric" sits in the Installed Capacity column, not the
	// monthly block — the one status sentinel that is not a row-wide colspan.
	t.Run("Export to K.Electric in FY2020-21", func(t *testing.T) {
		var found []string
		for _, p := range fy2021.Plants {
			if p.InstalledCapacity.State() != StateExportToKElectric {
				continue
			}
			found = append(found, p.Name)
			if p.InstalledCapacity.Raw() != "Export to K.Electric" {
				t.Errorf("%s: Raw() = %q, want %q", p.Name, p.InstalledCapacity.Raw(), "Export to K.Electric")
			}
			if p.InstalledCapacity.Present() {
				t.Errorf("%s: a status sentinel must not be Present", p.Name)
			}
			// Their monthly block is blank, not zero and not a sentinel.
			for _, obs := range append(p.Months[:], p.Total) {
				if obs.Generation.State() != StateNotReported {
					t.Errorf("%s %s GWh = %v, want not reported", p.Name, obs.Label(), obs.Generation)
				}
			}
		}
		want := []string{
			"Tenaga Generasi Ltd.",
			"Hydrochina Dawood Power (Pvt.) Ltd. (HDPPL)",
			"Zephyr Power (Pvt.) Ltd.",
		}
		if len(found) != len(want) {
			t.Fatalf("Export to K.Electric plants = %q, want %q", found, want)
		}
		for i := range want {
			if found[i] != want[i] {
				t.Errorf("plant %d = %q, want %q", i, found[i], want[i])
			}
		}
		if fy2021.Census.ExportToKElectric != 0 {
			t.Errorf("Census.ExportToKElectric (monthly block) = %d, want 0 — the sentinel is in the capacity column",
				fy2021.Census.ExportToKElectric)
		}
		if fy2021.Census.CapacityStatus != 3 {
			t.Errorf("Census.CapacityStatus = %d, want 3", fy2021.Census.CapacityStatus)
		}
		// It is FY2020-21 only.
		for _, other := range []*Workbook{fy2324, parse(t, fullFY1718, "2017-18")} {
			for _, p := range other.Plants {
				if p.InstalledCapacity.State() == StateExportToKElectric {
					t.Errorf("FY%s unexpectedly has an Export to K.Electric row: %s", other.FiscalYear.Label(), p.Name)
				}
			}
		}
	})
}

// TestEmptyColspanRowsAreNotReportedNotZero covers the two FY2023-24 data
// rows whose monthly block is a single EMPTY <td colspan=26>. They must come
// out as 26 blanks with a live capacity — not as zeros, and not as a status.
func TestEmptyColspanRowsAreNotReportedNotZero(t *testing.T) {
	w := parse(t, fullFY2324, "2023-24")
	tests := []struct {
		plant             string
		installed, depend float64
	}{
		{"Reshma Power Generation (Private) Limited. (RPGPL)", 97, 97},
		{"Gulf Powergen (Private) Limited. (GPPL)", 84, 84},
	}
	for _, tc := range tests {
		t.Run(tc.plant, func(t *testing.T) {
			p, ok := w.PlantByName(tc.plant)
			if !ok {
				t.Fatalf("%s missing", tc.plant)
			}
			if p.Status != StateNumeric {
				t.Errorf("Status = %v, want %v (an empty colspan is absence, not a status)", p.Status, StateNumeric)
			}
			for _, obs := range append(p.Months[:], p.Total) {
				for _, v := range [2]Value{obs.Utilisation, obs.Generation} {
					if v.State() != StateNotReported {
						t.Fatalf("%s: state = %v, want %v", obs.Label(), v.State(), StateNotReported)
					}
					if n, ok := v.Float64(); ok {
						t.Fatalf("%s: yielded %v; an empty colspan must not become a number", obs.Label(), n)
					}
				}
			}
			if got, ok := p.InstalledCapacity.Float64(); !ok || got != tc.installed {
				t.Errorf("InstalledCapacity = %v (ok=%v), want %v", got, ok, tc.installed)
			}
			if got, ok := p.DependableCapacity.Float64(); !ok || got != tc.depend {
				t.Errorf("DependableCapacity = %v (ok=%v), want %v", got, ok, tc.depend)
			}
		})
	}
	// These two are exactly the FY2023-24 blank rows.
	if w.Census.NotReportedRows != 2 || w.Census.NotReported != 52 {
		t.Errorf("FY2023-24 blank rows/cells = %d/%d, want 2/52", w.Census.NotReportedRows, w.Census.NotReported)
	}
}

// TestTrailingPaddingIsRecordedNotTrusted: the padding columns past the 32
// logical ones are normally empty, but one row per year carries a stray ";".
// "Strip trailing all-empty columns" would therefore leave a 39th/40th column
// in place; the header band fixes the width instead and the junk is recorded.
func TestTrailingPaddingIsRecordedNotTrusted(t *testing.T) {
	wantResidue := map[string]string{
		"2023-24": "Warsak Hydropower Project (WAPDA)",
		"2020-21": "Atlas Power Ltd (APL)",
		"2017-18": "Atlas Power Ltd (APL)",
	}
	for _, y := range fullYears {
		t.Run(y.fy, func(t *testing.T) {
			w := parse(t, y.name, y.fy)
			var withResidue []string
			for _, p := range w.Plants {
				if len(p.Residue) > 0 {
					withResidue = append(withResidue, p.Name)
					if len(p.Residue) != 1 || p.Residue[0] != ";" {
						t.Errorf("%s residue = %q, want one \";\"", p.Name, p.Residue)
					}
				}
				// Padding never shifts a real column: the last logical
				// column is always the Sum GWh cell and it is never junk.
				if p.Total.Generation.State() == StateUnknownText {
					t.Errorf("%s Sum GWh = %v, which means a padding cell shifted into a real column",
						p.Name, p.Total.Generation)
				}
			}
			if len(withResidue) != 1 || withResidue[0] != wantResidue[y.fy] {
				t.Errorf("plants with residue = %q, want [%q]", withResidue, wantResidue[y.fy])
			}
			if !hasWarning(w, "residue: "+wantResidue[y.fy]) {
				t.Errorf("no residue warning for %s; warnings = %q", wantResidue[y.fy], w.Warnings)
			}
			// Physical width really is wider than logical, by pure padding.
			if w.PhysicalWidth <= w.LogicalWidth {
				t.Errorf("PhysicalWidth %d <= LogicalWidth %d; the padding assumption is untested",
					w.PhysicalWidth, w.LogicalWidth)
			}
		})
	}
}

// TestHiddenSpanNamesAreRejoined: Excel splits overflowing cell text into a
// display:none span. Removing markup with a space inserted would yield
// "Company Limit ed." and break the name join across years.
func TestHiddenSpanNamesAreRejoined(t *testing.T) {
	const want = "Jamshoro Power Generation Company Limited."
	for _, y := range fullYears {
		w := parse(t, y.name, y.fy)
		if _, ok := w.PlantByName(want); !ok {
			var near []string
			for _, p := range w.Plants {
				if len(p.Name) > 8 && p.Name[:8] == "Jamshoro" {
					near = append(near, p.Name)
				}
			}
			t.Errorf("%s: %q not found; got %q", y.fy, want, near)
		}
	}
}

// TestSNoIsNotAStableCrossYearKey is why Plant.SNo is documented as an
// in-year ordinal. The names line up across years; the serials do not.
func TestSNoIsNotAStableCrossYearKey(t *testing.T) {
	byYear := map[string]map[string]int{}
	for _, y := range fullYears {
		w := parse(t, y.name, y.fy)
		m := make(map[string]int, len(w.Plants))
		for _, p := range w.Plants {
			m[p.Name] = p.SNo
		}
		byYear[y.fy] = m
	}

	tests := []struct {
		a, b          string
		wantShared    int
		wantSameSNo   int
		wantDifferent bool
	}{
		{a: "2017-18", b: "2020-21", wantShared: 107, wantSameSNo: 107},
		{a: "2020-21", b: "2023-24", wantShared: 106, wantSameSNo: 0, wantDifferent: true},
	}
	for _, tc := range tests {
		t.Run(tc.a+" -> "+tc.b, func(t *testing.T) {
			shared, same := 0, 0
			for name, sa := range byYear[tc.a] {
				sb, ok := byYear[tc.b][name]
				if !ok {
					continue
				}
				shared++
				if sa == sb {
					same++
				}
			}
			if shared != tc.wantShared {
				t.Errorf("plants present in both years = %d, want %d", shared, tc.wantShared)
			}
			if same != tc.wantSameSNo {
				t.Errorf("plants with an unchanged S.No = %d/%d, want %d", same, shared, tc.wantSameSNo)
			}
			if tc.wantDifferent && same != 0 {
				t.Error("FY2023-24 was re-sorted by technology; no S.No should carry over")
			}
		})
	}

	// The two moves named in the measurement.
	moves := []struct {
		plant              string
		from2021, from2324 int
	}{
		{"AES Lalpir power limited.", 1, 36},
		{"Allai Khwar Hydropower Project (WAPDA)", 56, 8},
	}
	for _, m := range moves {
		if got := byYear["2020-21"][m.plant]; got != m.from2021 {
			t.Errorf("%s FY2020-21 S.No = %d, want %d", m.plant, got, m.from2021)
		}
		if got := byYear["2023-24"][m.plant]; got != m.from2324 {
			t.Errorf("%s FY2023-24 S.No = %d, want %d", m.plant, got, m.from2324)
		}
	}
}

// TestTrimmedFixturesExerciseEveryState is the readable counterpart to the
// whole-file tests: the small committed fixtures must between them contain
// all five cell states, so a reviewer can see the markup each state comes
// from.
func TestTrimmedFixturesExerciseEveryState(t *testing.T) {
	tests := []struct {
		fixture string
		fy      string
		plants  int
		want    map[CellState]int // monthly-block counts
	}{
		{
			fixture: trimFY2324, fy: "2023-24", plants: 11,
			want: map[CellState]int{
				StateNumeric:        156, // 6 fully-numeric rows
				StateNotReported:    52,  // the 2 empty <td colspan=26> rows
				StateDelicensed:     52,  // Kotri + Lakhra
				StateDecommissioned: 26,
			},
		},
		{
			fixture: trimFY2021, fy: "2020-21", plants: 8,
			want: map[CellState]int{
				StateNumeric:     130, // 5 fully-numeric rows
				StateNotReported: 78,  // the 3 Export to K.Electric rows
			},
		},
		{
			fixture: trimFY1718, fy: "2017-18", plants: 7,
			want: map[CellState]int{
				StateNumeric:     104, // 4 fully-numeric rows
				StateNotReported: 78,  // K-2, Tarbela Ext. 04, Golen Gol
			},
		},
	}
	seen := map[CellState]bool{}
	for _, tc := range tests {
		t.Run(tc.fy, func(t *testing.T) {
			w := parse(t, tc.fixture, tc.fy)
			if len(w.Plants) != tc.plants {
				t.Fatalf("plants = %d, want %d", len(w.Plants), tc.plants)
			}
			got := map[CellState]int{
				StateNumeric:        w.Census.Present,
				StateNotReported:    w.Census.NotReported,
				StateDelicensed:     w.Census.Delicensed,
				StateDecommissioned: w.Census.Decommissioned,
			}
			for state, want := range tc.want {
				if got[state] != want {
					t.Errorf("%v cells = %d, want %d", state, got[state], want)
				}
				if want > 0 {
					seen[state] = true
				}
			}
			if !w.Census.Balanced() {
				t.Error("census does not balance")
			}
			// A trimmed fixture necessarily has a gappy S.No; the parser
			// must say so rather than pretend otherwise.
			if w.SNoContiguous {
				t.Error("SNoContiguous = true on a trimmed fixture, want false")
			}
			if !hasWarning(w, "S.No is not contiguous") {
				t.Errorf("no S.No warning; warnings = %q", w.Warnings)
			}
		})
	}
	// The fifth state lives in the capacity column of the FY2020-21 trim.
	w := parse(t, trimFY2021, "2020-21")
	if w.Census.CapacityStatus != 3 {
		t.Errorf("trimmed FY2020-21 CapacityStatus = %d, want 3 Export to K.Electric cells", w.Census.CapacityStatus)
	}
	seen[StateExportToKElectric] = w.Census.CapacityStatus == 3
	for _, state := range []CellState{StateNumeric, StateNotReported, StateDelicensed, StateDecommissioned, StateExportToKElectric} {
		if !seen[state] {
			t.Errorf("no trimmed fixture exercises %v", state)
		}
	}
}
