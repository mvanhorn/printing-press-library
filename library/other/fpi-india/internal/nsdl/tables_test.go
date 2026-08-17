package nsdl

import (
	"strings"
	"testing"
)

func TestExtractLargestTable(t *testing.T) {
	html := []byte(`<html><body>
<table><tr><td>nav</td></tr></table>
<table id="rpt"><tr><th>Name</th><th>Value</th></tr><tr><td>Alpha</td><td>10</td></tr><tr><td>Beta</td><td>20</td></tr></table>
</body></html>`)

	tbl, err := ExtractLargestTable(html)
	if err != nil {
		t.Fatalf("ExtractLargestTable() error = %v", err)
	}
	if len(tbl.Rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(tbl.Rows))
	}
	if tbl.Rows[0][0] != "Alpha" || tbl.Rows[0][1] != "10" {
		t.Fatalf("unexpected first row: %v", tbl.Rows[0])
	}
}

func TestExtractTableByID(t *testing.T) {
	html := []byte(`<html><body>
<table id="other"><tr><td>1</td><td>2</td><td>3</td></tr></table>
<table id="rpt"><tr><th>Year</th><th>Amount</th></tr><tr><td>2024-25</td><td>-127041</td></tr></table>
</body></html>`)

	tbl, err := ExtractTableByID(html, "rpt")
	if err != nil {
		t.Fatalf("ExtractTableByID() error = %v", err)
	}
	if len(tbl.Rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(tbl.Rows))
	}
	if tbl.Rows[0][0] != "2024-25" {
		t.Fatalf("unexpected period: %q", tbl.Rows[0][0])
	}
}

func TestExtractTableByID_FallsBackToLargestWhenIDMissing(t *testing.T) {
	html := []byte(`<html><body><table><tr><td>Name</td><td>Value</td></tr><tr><td>Alpha</td><td>10</td></tr></table></body></html>`)
	tbl, err := ExtractTableByID(html, "does-not-exist")
	if err != nil {
		t.Fatalf("ExtractTableByID() error = %v", err)
	}
	if len(tbl.Rows) != 1 {
		t.Fatalf("got %d rows, want 1: %v", len(tbl.Rows), tbl.Rows)
	}
}

func TestParseTableNode_NestedTableDoesNotCorruptOuterRowCount(t *testing.T) {
	// Legacy static report pages nest a real data table inside a layout
	// table's cell. The outer wrapper must not report the nested table's
	// rows as its own (see countRows's table-boundary comment).
	html := []byte(`<html><body>
<table><tr><td>
  <table id="data"><tr><th>Sector</th><th>Total</th></tr><tr><td>Financial Services</td><td>2137845</td></tr></table>
</td></tr></table>
</body></html>`)
	tbl, err := ExtractLargestTable(html)
	if err != nil {
		t.Fatalf("ExtractLargestTable() error = %v", err)
	}
	if len(tbl.Rows) != 1 {
		t.Fatalf("got %d rows, want 1 (nested table row should not merge into wrapper)", len(tbl.Rows))
	}
	if tbl.Rows[0][0] != "Financial Services" {
		t.Fatalf("unexpected row: %v", tbl.Rows[0])
	}
}

func TestParseTableNode_ColspanRowspanHeaderGrid(t *testing.T) {
	// Mirrors NSDL's Yearwise.aspx grouped-header shape: a "Mutual Funds"
	// group spans 2 sub-columns whose leaf names collide with top-level
	// "Equity"/"Debt" columns elsewhere in the row.
	html := []byte(`<html><body><table id="rpt">
<tr><th rowspan="2">Year</th><th>Equity</th><th colspan="2">Mutual Funds</th></tr>
<tr><th>Equity</th><th>Debt</th></tr>
<tr><td>2024-25</td><td>100</td><td>5</td><td>2</td></tr>
</table></body></html>`)
	tbl, err := ExtractTableByID(html, "rpt")
	if err != nil {
		t.Fatalf("ExtractTableByID() error = %v", err)
	}
	if len(tbl.Columns) != 4 {
		t.Fatalf("got %d columns, want 4: %v", len(tbl.Columns), tbl.Columns)
	}
	hasTopLevelEquity := false
	hasDisambiguatedMF := false
	for _, c := range tbl.Columns {
		if c == "Equity" {
			hasTopLevelEquity = true
		}
		if strings.Contains(c, "Mutual Funds") {
			hasDisambiguatedMF = true
		}
	}
	if !hasTopLevelEquity {
		t.Fatalf("expected an unprefixed top-level Equity column: %v", tbl.Columns)
	}
	if !hasDisambiguatedMF {
		t.Fatalf("expected a Mutual-Funds-prefixed column disambiguating the colliding leaf name: %v", tbl.Columns)
	}
}
