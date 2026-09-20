package nepraparse

import (
	"strings"
	"testing"
)

// TestExportedAPIHappyPath exercises the remaining exported surface on a real
// fixture, so no exported function is shipped without at least one assertion
// on its actual output.
func TestExportedAPIHappyPath(t *testing.T) {
	w := parse(t, trimFY2324, "2023-24")

	t.Run("Workbook lookups", func(t *testing.T) {
		if _, ok := w.PlantByName("No Such Plant"); ok {
			t.Error("PlantByName found a plant that is not there")
		}
		tech := w.Technologies()
		if len(tech) == 0 || tech[0] != "Coal" {
			t.Errorf("Technologies() = %q, want a sorted list starting at %q", tech, "Coal")
		}
		fuels := w.Fuels()
		if len(fuels) == 0 || fuels[0] != "COAL" {
			t.Errorf("Fuels() = %q, want a sorted list starting at %q", fuels, "COAL")
		}
	})

	t.Run("HeaderFingerprint", func(t *testing.T) {
		fp := w.Header.Fingerprint()
		if !strings.HasPrefix(fp, "S.No|Name of Companies|Technology|Fuel|") {
			t.Errorf("Fingerprint() = %q, want it to start with the stub columns", fp)
		}
		if !strings.HasSuffix(fp, "|Sum % age|Sum GWh") {
			t.Errorf("Fingerprint() = %q, want it to end with the Sum pair", fp)
		}
		if got := strings.Count(fp, "|"); got != LogicalColumns-1 {
			t.Errorf("Fingerprint() has %d separators, want %d", got, LogicalColumns-1)
		}
		if diff := w.Header.Diff(); len(diff) != 0 {
			t.Errorf("Diff() = %q, want empty", diff)
		}
		// Diff names the offending column when there is one.
		drifted := w.Header
		drifted.Columns[1] = "Company Name"
		d := drifted.Diff()
		if len(d) != 1 || !strings.Contains(d[0], "column 1") {
			t.Errorf("Diff() on a drifted header = %q, want one line naming column 1", d)
		}
	})

	t.Run("Plant and observation accessors", func(t *testing.T) {
		p, ok := w.PlantByName("Tarbela Hydropower Project (WAPDA)")
		if !ok {
			t.Fatal("Tarbela missing")
		}
		if p.SNoRaw != "1" || p.SNo != 1 {
			t.Errorf("SNo/SNoRaw = %d/%q, want 1/\"1\"", p.SNo, p.SNoRaw)
		}
		if p.Technology != "HYDEL-(WAPDA)" || !p.TechnologyKnown {
			t.Errorf("Technology = %q (known=%v)", p.Technology, p.TechnologyKnown)
		}
		if p.Fuel != "HYDEL" || !p.FuelKnown {
			t.Errorf("Fuel = %q (known=%v)", p.Fuel, p.FuelKnown)
		}
		if p.RowIndex != 5 {
			t.Errorf("RowIndex = %d, want 5 (the first data row)", p.RowIndex)
		}
		if p.Status != StateNumeric {
			t.Errorf("Status = %v, want %v", p.Status, StateNumeric)
		}
		jun, ok := p.Month(Jun)
		if !ok {
			t.Fatal("Month(Jun) not ok")
		}
		if jun.Label() != "Jun" {
			t.Errorf("Label() = %q, want Jun", jun.Label())
		}
		if jun.IsTotal {
			t.Error("Jun must not be flagged as the total")
		}
		if got, _ := jun.Generation.Float64(); got != 1641.15 {
			t.Errorf("Jun GWh = %v, want 1641.15", got)
		}
		if _, ok := p.Month(Month(0)); ok {
			t.Error("Month(0) must report ok = false")
		}
	})

	t.Run("SumCheck rendering", func(t *testing.T) {
		rep := w.CheckSum(FieldGWh, DefaultSumTolerance)
		if len(rep.Checks) == 0 {
			t.Fatal("no eligible rows in the trimmed fixture")
		}
		line := rep.Checks[0].String()
		if !strings.HasPrefix(line, "ok: ") {
			t.Errorf("SumCheck.String() = %q, want it to lead with the verdict", line)
		}
		if !strings.Contains(line, "delta=") {
			t.Errorf("SumCheck.String() = %q, want it to show the delta", line)
		}
		ratio, ok := rep.Ratio()
		if !ok || ratio != 1 {
			t.Errorf("Ratio() = %v/%v, want 1", ratio, ok)
		}
		if rep.Checks[0].RowIndex == 0 {
			t.Error("SumCheck.RowIndex was not populated")
		}
	})

	t.Run("Grid is reachable on its own", func(t *testing.T) {
		g := gridOf(t, trimFY2324)
		if g.Tables != 1 || g.HeaderCells != 0 {
			t.Errorf("Tables/HeaderCells = %d/%d, want 1/0", g.Tables, g.HeaderCells)
		}
		if len(g.RawCellsPerRow) != len(g.Rows) {
			t.Errorf("RawCellsPerRow has %d entries but there are %d rows", len(g.RawCellsPerRow), len(g.Rows))
		}
		if g.Rows[4][ColFirstMonthPair] != MetricPct {
			t.Errorf("leaf row column %d = %q, want %q", ColFirstMonthPair, g.Rows[4][ColFirstMonthPair], MetricPct)
		}
	})

	t.Run("DeclaredCharset on a document with none", func(t *testing.T) {
		if got := DeclaredCharset([]byte("<html><body>hi</body></html>")); got != "" {
			t.Errorf("DeclaredCharset = %q, want \"\"", got)
		}
	})
}
