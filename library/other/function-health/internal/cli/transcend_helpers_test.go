// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestOutOfRangeDirection(t *testing.T) {
	// Prefer the draw's own value against the Quest reference range, ignoring a
	// stale record-level outOfRangeType.
	if got := outOfRangeDirection(200, 50, 150, 60, 120, "below_range"); got != "above" {
		t.Errorf("value above quest range = %q, want above (not the stale record type)", got)
	}
	if got := outOfRangeDirection(10, 50, 150, 60, 120, "above_range"); got != "below" {
		t.Errorf("value below quest range = %q, want below", got)
	}
	// No Quest range → fall back to the Function-optimal range.
	if got := outOfRangeDirection(130, 0, 0, 60, 120, ""); got != "above" {
		t.Errorf("value above optimal (no quest range) = %q, want above", got)
	}
	// No numeric bounds at all → last-resort record type.
	if got := outOfRangeDirection(99, 0, 0, 0, 0, "below_range"); got != "below" {
		t.Errorf("no bounds = %q, want fallback to record type below", got)
	}
	// No bounds and no record type → honest sentinel.
	if got := outOfRangeDirection(99, 0, 0, 0, 0, ""); got != "out" {
		t.Errorf("no info = %q, want out", got)
	}
}

// TestExtractResultRowsDirectionIsPerDraw locks in the fix for the stale
// direction bug: a biomarker whose CURRENT state is above_range but whose
// history includes a below-range draw must report each draw's own direction,
// not the latest round's outOfRangeType stamped across all of them.
func TestExtractResultRowsDirectionIsPerDraw(t *testing.T) {
	raw := []byte(`{
		"data": {
			"biomarkerResultsRecord": [
				{
					"units": "mg/dL",
					"rangeMin": "50",
					"rangeMax": "150",
					"optimalRangeMin": "60",
					"optimalRangeMax": "120",
					"outOfRangeType": "above_range",
					"biomarker": { "id": "bm-1", "name": "TestMarker" },
					"biomarkerResults": [
						{ "dateOfService": "2023-01-01", "testResult": "10",  "testResultOutOfRange": true,  "requisitionId": "R1" },
						{ "dateOfService": "2024-01-01", "testResult": "100", "testResultOutOfRange": false, "requisitionId": "R2" },
						{ "dateOfService": "2025-01-01", "testResult": "200", "testResultOutOfRange": true,  "requisitionId": "R3" }
					]
				}
			]
		}
	}`)
	rows := extractResultRows(raw, map[string]string{})
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3", len(rows))
	}
	wantDir := []string{"below", "in", "above"}
	wantStatus := []string{"out-of-range", "in-range", "out-of-range"}
	for i, r := range rows {
		if r.Direction != wantDir[i] {
			t.Errorf("row %d (value %.0f) direction = %q, want %q", i, r.Value, r.Direction, wantDir[i])
		}
		if r.Status != wantStatus[i] {
			t.Errorf("row %d status = %q, want %q", i, r.Status, wantStatus[i])
		}
	}
}
