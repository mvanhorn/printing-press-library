package twb_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/tableau/internal/twb"
)

func fixture(t *testing.T, rel string) string {
	t.Helper()
	// tests run with package dir = internal/twb; fixtures at repo root testdata/
	p := filepath.Join("..", "..", "testdata", rel)
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("fixture %s: %v", p, err)
	}
	return p
}

func TestInspectAssignment1(t *testing.T) {
	wb, err := twb.Open(fixture(t, "superstore/Assignment_1.twb"))
	if err != nil {
		t.Fatal(err)
	}
	sum := wb.Inspect()
	if sum.SheetCount < 6 {
		t.Fatalf("sheets>=6, got %d %v", sum.SheetCount, sum.Sheets)
	}
	if sum.DashboardCount < 1 {
		t.Fatalf("dashboards>=1, got %d", sum.DashboardCount)
	}
	if sum.ZoneCount <= 0 {
		t.Fatalf("zones>0, got %d", sum.ZoneCount)
	}
}

func TestLintCleanFixtures(t *testing.T) {
	for _, rel := range []string{
		"official/sample-superstore.twb",
		"superstore/Assignment_1.twb",
	} {
		t.Run(rel, func(t *testing.T) {
			wb, err := twb.Open(fixture(t, rel))
			if err != nil {
				t.Fatal(err)
			}
			issues := wb.Lint()
			if twb.HasErrors(issues) {
				for _, iss := range issues {
					t.Logf("%+v", iss)
				}
				t.Fatalf("expected clean lint for %s, got %d issues", rel, len(issues))
			}
		})
	}
}

func TestAddCalcWriteReopen(t *testing.T) {
	src := fixture(t, "superstore/Assignment_1.twb")
	wb, err := twb.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	const caption = "Grok Test Margin"
	if err := wb.AddCalc(caption, "SUM([Profit])/SUM([Sales])", "real", "measure"); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "with-calc.twb")
	if err := wb.Write(out); err != nil {
		t.Fatal(err)
	}
	wb2, err := twb.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range wb2.ListCalcs() {
		if c.Caption == caption {
			found = true
			if !strings.Contains(c.Formula, "Profit") {
				t.Errorf("unexpected formula %q", c.Formula)
			}
		}
	}
	if !found {
		t.Fatalf("caption %q not found after re-open; calcs=%v", caption, wb2.ListCalcs())
	}
}

func TestAddCalcsBatch36(t *testing.T) {
	// Ann Jackson path: bulk CY/PY-style calculated fields in one structured op.
	wb, err := twb.Open(fixture(t, "official/sample-superstore.twb"))
	if err != nil {
		t.Fatal(err)
	}
	before := len(wb.ListCalcs())
	var specs []twb.CalcSpec
	for i := 1; i <= 36; i++ {
		specs = append(specs, twb.CalcSpec{
			Caption:  fmt.Sprintf("Agent Bulk Calc %02d", i),
			Formula:  fmt.Sprintf("SUM([Sales]) + %d", i),
			Datatype: "real",
			Role:     "measure",
		})
	}
	if err := wb.AddCalcs(specs); err != nil {
		t.Fatal(err)
	}
	after := len(wb.ListCalcs())
	if after < before+36 {
		t.Fatalf("expected +36 calcs, before=%d after=%d", before, after)
	}
	out := filepath.Join(t.TempDir(), "bulk36.twb")
	if err := wb.Write(out); err != nil {
		t.Fatal(err)
	}
	if issues := wb.Lint(); twb.HasErrors(issues) {
		t.Fatalf("lint after bulk: %+v", issues)
	}
	wb2, err := twb.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(wb2.ListCalcs()) < before+36 {
		t.Fatalf("re-open lost calcs: %d", len(wb2.ListCalcs()))
	}
}

func TestCloneSheet(t *testing.T) {
	wb, err := twb.Open(fixture(t, "superstore/Assignment_1.twb"))
	if err != nil {
		t.Fatal(err)
	}
	from := wb.ListSheets()[0]
	to := from + " (Clone)"
	if err := wb.CloneSheet(from, to); err != nil {
		t.Fatal(err)
	}
	sheets := wb.ListSheets()
	found := false
	for _, s := range sheets {
		if s == to {
			found = true
		}
	}
	if !found {
		t.Fatalf("cloned sheet %q missing; sheets=%v", to, sheets)
	}
	// Write + re-open
	out := filepath.Join(t.TempDir(), "cloned.twb")
	if err := wb.Write(out); err != nil {
		t.Fatal(err)
	}
	wb2, err := twb.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	found = false
	for _, s := range wb2.ListSheets() {
		if s == to {
			found = true
		}
	}
	if !found {
		t.Fatalf("cloned sheet missing after re-open")
	}
}

func TestLintRejectBold(t *testing.T) {
	src := fixture(t, "official/sample-superstore.twb")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	// Inject illegal enum value exactly "bold" (Ann Jackson case).
	injected := strings.Replace(
		string(data),
		"<workbook ",
		`<workbook agent-hack="bold" `,
		1,
	)
	dir := t.TempDir()
	path := filepath.Join(dir, "bold.twb")
	if err := os.WriteFile(path, []byte(injected), 0o644); err != nil {
		t.Fatal(err)
	}
	wb, err := twb.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	issues := wb.Lint()
	if !twb.HasErrors(issues) {
		t.Fatal("expected lint errors for bold attr value")
	}
	found := false
	for _, iss := range issues {
		if iss.Code == "illegal-bold-enum" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected illegal-bold-enum, got %+v", issues)
	}
}

func TestOpenTWBX(t *testing.T) {
	wb, err := twb.Open(fixture(t, "official/TABLEAU_10_TWBX.twbx"))
	if err != nil {
		t.Fatal(err)
	}
	if wb.Root() == nil || wb.Root().Tag != "workbook" {
		t.Fatal("expected workbook root from twbx")
	}
}

func TestApplyStyle(t *testing.T) {
	wb, err := twb.Open(fixture(t, "superstore/Assignment_1.twb"))
	if err != nil {
		t.Fatal(err)
	}
	sheets := wb.ListSheets()
	if len(sheets) < 2 {
		t.Fatal("need at least 2 sheets")
	}
	if err := wb.ApplyStyle(sheets[2], sheets[0]); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "styled.twb")
	if err := wb.Write(out); err != nil {
		t.Fatal(err)
	}
	// Re-open and ensure destination still present.
	wb2, err := twb.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(wb2.ListSheets()) < 2 {
		t.Fatal("sheets lost after style apply")
	}
}

func TestScaffoldTwoPaneDashboard(t *testing.T) {
	wb, err := twb.Open(fixture(t, "official/filtering.twb"))
	if err != nil {
		t.Fatal(err)
	}
	sheets := wb.ListSheets()
	if len(sheets) < 2 {
		t.Fatal("need 2 sheets")
	}
	if err := wb.ScaffoldDashboard("Agent Two Pane", "two-pane", sheets[:2]); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range wb.ListDashboards() {
		if d.Name == "Agent Two Pane" && d.ZoneCount >= 3 {
			found = true
		}
	}
	if !found {
		t.Fatalf("scaffold missing; dashboards=%v", wb.ListDashboards())
	}
	if issues := wb.Lint(); twb.HasErrors(issues) {
		t.Fatalf("lint after scaffold: %+v", issues)
	}
	out := filepath.Join(t.TempDir(), "scaffold.twb")
	if err := wb.Write(out); err != nil {
		t.Fatal(err)
	}
	wb2, err := twb.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	if issues := wb2.Lint(); twb.HasErrors(issues) {
		t.Fatalf("re-open lint: %+v", issues)
	}
}

func TestScaffoldRejectsUnknownTemplate(t *testing.T) {
	wb, err := twb.Open(fixture(t, "official/sample-superstore.twb"))
	if err != nil {
		t.Fatal(err)
	}
	err = wb.ScaffoldDashboard("X", "freeform-llm-dump", []string{"Sheet 1"})
	if err == nil {
		t.Fatal("expected unknown template error")
	}
}

func TestLintRejectsDashboardMissingSimpleID(t *testing.T) {
	src := fixture(t, "official/filtering.twb")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	// Strip simple-id to simulate Ann content-model failure.
	stripped := strings.ReplaceAll(string(data), "<simple-id uuid='{C0E32347-4DB4-4838-9BD4-6B21909C8980}' />", "")
	path := filepath.Join(t.TempDir(), "nosid.twb")
	if err := os.WriteFile(path, []byte(stripped), 0o644); err != nil {
		t.Fatal(err)
	}
	wb, err := twb.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	issues := wb.Lint()
	found := false
	for _, iss := range issues {
		if iss.Code == "dashboard-missing-simple-id" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected dashboard-missing-simple-id, got %+v", issues)
	}
}

func TestCloneDashboard(t *testing.T) {
	wb, err := twb.Open(fixture(t, "official/filtering.twb"))
	if err != nil {
		t.Fatal(err)
	}
	if err := wb.CloneDashboard("setTest", "setTest Copy"); err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, d := range wb.ListDashboards() {
		names[d.Name] = true
	}
	if !names["setTest Copy"] {
		t.Fatalf("clone missing: %v", names)
	}
	if issues := wb.Lint(); twb.HasErrors(issues) {
		t.Fatalf("lint: %+v", issues)
	}
}

func TestBuildYoYPackAndApply(t *testing.T) {
	specs := twb.BuildYoYPack([]string{"Sales", "Profit", "Quantity"}, "Order Date", 2017, 2016)
	if len(specs) != 12 {
		t.Fatalf("want 12 specs, got %d", len(specs))
	}
	wb, err := twb.Open(fixture(t, "official/sample-superstore.twb"))
	if err != nil {
		t.Fatal(err)
	}
	if err := wb.AddCalcs(specs); err != nil {
		t.Fatal(err)
	}
	if issues := wb.Lint(); twb.HasErrors(issues) {
		t.Fatalf("lint: %+v", issues)
	}
	found := 0
	for _, c := range wb.ListCalcs() {
		if strings.Contains(c.Caption, "YoY") || strings.HasSuffix(c.Caption, " CY") {
			found++
		}
	}
	if found < 6 {
		t.Fatalf("expected YoY/CY calcs present, found marker count %d", found)
	}
}

func TestThreeRowScaffold(t *testing.T) {
	wb, err := twb.Open(fixture(t, "superstore/Assignment_1.twb"))
	if err != nil {
		t.Fatal(err)
	}
	sheets := wb.ListSheets()
	if len(sheets) < 3 {
		t.Fatal("need 3 sheets")
	}
	if err := wb.ScaffoldDashboard("Three Row Board", "three-row", sheets[:3]); err != nil {
		t.Fatal(err)
	}
	if issues := wb.Lint(); twb.HasErrors(issues) {
		t.Fatalf("%+v", issues)
	}
}

func TestRefuseWriteFromTWBX(t *testing.T) {
	wb, err := twb.Open(fixture(t, "official/TABLEAU_10_TWBX.twbx"))
	if err != nil {
		t.Fatal(err)
	}
	if !wb.FromTWBX() {
		t.Fatal("expected FromTWBX")
	}
	out := filepath.Join(t.TempDir(), "stripped.twb")
	if err := wb.Write(out); err == nil {
		t.Fatal("expected refuse write from twbx without AllowDropPackage")
	}
	wb.AllowDropPackage = true
	if err := wb.Write(out); err != nil {
		t.Fatal(err)
	}
}
