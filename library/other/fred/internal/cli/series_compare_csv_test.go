// Copyright 2026 Luke J and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

// Regression test for the --csv flag being silently ignored by `series compare`:
// the nested compare view must flatten to a one-row-per-date table that the shared
// CSV renderer can emit, with missing series values rendered as empty cells.
func TestFlattenCompareRowsToCSV(t *testing.T) {
	series := []string{"UNRATE", "CPIAUCSL"}
	rows := []compareRow{
		{Date: "2024-01-01", Values: map[string]string{"UNRATE": "3.7", "CPIAUCSL": "309.7"}},
		{Date: "2024-02-01", Values: map[string]string{"UNRATE": "3.9"}}, // CPIAUCSL absent
	}

	flat := flattenCompareRows(series, rows)
	if len(flat) != 2 {
		t.Fatalf("expected 2 flat rows, got %d", len(flat))
	}
	if flat[0]["date"] != "2024-01-01" || flat[0]["CPIAUCSL"] != "309.7" || flat[0]["UNRATE"] != "3.7" {
		t.Errorf("row 0 wrong: %#v", flat[0])
	}
	if _, ok := flat[1]["CPIAUCSL"]; !ok || flat[1]["CPIAUCSL"] != "" {
		t.Errorf("missing value should flatten to an empty string, got %#v", flat[1]["CPIAUCSL"])
	}

	data, err := json.Marshal(flat)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var buf bytes.Buffer
	if err := printCSV(&buf, data); err != nil {
		t.Fatalf("printCSV: %v", err)
	}
	// printCSV sorts headers alphabetically (consistent with every other command):
	// CPIAUCSL, UNRATE, date.
	want := "CPIAUCSL,UNRATE,date\n309.7,3.7,2024-01-01\n,3.9,2024-02-01\n"
	if got := buf.String(); got != want {
		t.Errorf("CSV mismatch:\n got: %q\nwant: %q", got, want)
	}
}
