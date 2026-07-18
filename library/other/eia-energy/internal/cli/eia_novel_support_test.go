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
