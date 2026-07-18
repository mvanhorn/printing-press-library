// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import "testing"

func TestParseFacet(t *testing.T) {
	name, value, err := parseFacet("respondent=CISO")
	if err != nil || name != "respondent" || value != "CISO" {
		t.Fatalf("unexpected facet: %q %q %v", name, value, err)
	}
	if _, _, err := parseFacet("missing-separator"); err == nil {
		t.Fatal("expected invalid facet error")
	}
}

func TestCompareThreshold(t *testing.T) {
	tests := []struct {
		op   string
		want bool
	}{{">", true}, {">=", true}, {"<", false}, {"<=", false}}
	for _, tt := range tests {
		got, err := compareThreshold(10, tt.op, 5)
		if err != nil || got != tt.want {
			t.Fatalf("%s: got %v, %v", tt.op, got, err)
		}
	}
	if _, err := compareThreshold(1, "=", 1); err == nil {
		t.Fatal("expected invalid operator error")
	}
}

func TestSeriesStatsIgnoresNonNumeric(t *testing.T) {
	stats := seriesStats([]map[string]any{{"value": "2"}, {"value": "bad"}, {"value": "4"}}, "value")
	if stats["count"] != 2 || stats["mean"] != 3.0 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
}

func TestRequireCompletePageRejectsTruncation(t *testing.T) {
	if err := requireCompletePage(eiaPage{Total: "5001", Rows: make([]map[string]any, 5000)}, "test analysis"); err == nil {
		t.Fatal("expected a truncated page error")
	}
	if err := requireCompletePage(eiaPage{Total: "2", Rows: make([]map[string]any, 2)}, "test analysis"); err != nil {
		t.Fatalf("complete page rejected: %v", err)
	}
}

func TestBuildSpreadRowsReportsInvalidAlignedValues(t *testing.T) {
	left := map[string]map[string]any{"2026-01": {"period": "2026-01", "value": "not-available", "value-units": "MW"}}
	right := map[string]map[string]any{"2026-01": {"period": "2026-01", "value": "12", "value-units": "MW"}}
	rows, excluded, err := buildSpreadRows(left, right, "value")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 || len(excluded) != 1 || excluded[0]["period"] != "2026-01" {
		t.Fatalf("rows=%#v excluded=%#v", rows, excluded)
	}
}
