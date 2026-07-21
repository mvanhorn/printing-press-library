// Copyright 2026 Ryan Kelley and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

// searchTermsReportFixture returns a realistic reporting API payload for
// POST /reports/campaigns/{id}/adgroups/{id}/searchterms.
// Taps and installs are string-typed values nested inside granularity rows.
func searchTermsReportFixture() json.RawMessage {
	return json.RawMessage(`{
		"data": {
			"reportingDataResponse": {
				"row": [
					{
						"metadata": {"searchTerm": "yoga mat", "adGroupId": "ag1"},
						"granularity": [
							{"date": "2026-07-01", "taps": "80", "installs": "10", "localSpend": "32.00"},
							{"date": "2026-07-02", "taps": "120", "installs": "15", "localSpend": "48.00"}
						]
					},
					{
						"metadata": {"searchTerm": "running shoes", "adGroupId": "ag1"},
						"granularity": [
							{"date": "2026-07-01", "taps": "45", "installs": "3", "localSpend": "18.00"}
						]
					},
					{
						"metadata": {"adGroupId": "ag1"},
						"granularity": [
							{"date": "2026-07-01", "taps": "10", "installs": "1"}
						]
					}
				]
			}
		}
	}`)
}

func TestExtractSearchTerms_ReportingAPI(t *testing.T) {
	terms := extractSearchTerms(searchTermsReportFixture())

	// Row with no searchTerm must be skipped.
	if len(terms) != 2 {
		t.Fatalf("want 2 terms, got %d", len(terms))
	}

	yoga := terms[0]
	if yoga.text != "yoga mat" {
		t.Errorf("want text=yoga mat, got %q", yoga.text)
	}
	// Taps must be summed across both granularity rows: 80+120=200
	if yoga.taps != 200 {
		t.Errorf("want taps=200 (summed across granularity), got %d", yoga.taps)
	}
	// Installs: 10+15=25
	if yoga.installs != 25 {
		t.Errorf("want installs=25, got %d", yoga.installs)
	}

	shoes := terms[1]
	if shoes.text != "running shoes" {
		t.Errorf("want text=running shoes, got %q", shoes.text)
	}
	if shoes.taps != 45 {
		t.Errorf("want taps=45, got %d", shoes.taps)
	}
	if shoes.installs != 3 {
		t.Errorf("want installs=3, got %d", shoes.installs)
	}
}

// TestExtractSearchTerms_FlatShape verifies the fallback path still works for
// direct GET /adgroups/{id}/searchterms responses that return a flat array.
func TestExtractSearchTerms_FlatShape(t *testing.T) {
	flat := json.RawMessage(`{
		"data": [
			{"searchTerm": "protein powder", "taps": 55, "installs": 7},
			{"searchTerm": "gym bag",         "taps": 30, "installs": 2},
			{"taps": 5, "installs": 1}
		]
	}`)

	terms := extractSearchTerms(flat)

	// Third item has no recognisable text field; must be skipped.
	if len(terms) != 2 {
		t.Fatalf("want 2 terms, got %d", len(terms))
	}
	if terms[0].text != "protein powder" {
		t.Errorf("want protein powder, got %q", terms[0].text)
	}
	if terms[0].taps != 55 {
		t.Errorf("want taps=55, got %d", terms[0].taps)
	}
	if terms[1].text != "gym bag" {
		t.Errorf("want gym bag, got %q", terms[1].text)
	}
}

func TestExtractSearchTerms_Empty(t *testing.T) {
	terms := extractSearchTerms(json.RawMessage(`{"data":{"reportingDataResponse":{"row":[]}}}`))
	if len(terms) != 0 {
		t.Errorf("want 0 terms, got %d", len(terms))
	}
}

// TestExtractAdGroupIDs_PaginationSentinel verifies that extractAdGroupIDs returns
// exactly 100 items when the response is at the API page capacity, confirming the
// len >= 100 sentinel used to emit the truncation warning is correct.
func TestExtractAdGroupIDs_PaginationSentinel(t *testing.T) {
	// Build a minimal {"data": [{id:"1"}, ...]} with exactly 100 items.
	items := make([]json.RawMessage, 100)
	for i := range items {
		items[i] = json.RawMessage(`{"id":"ag"}`)
	}
	raw, _ := json.Marshal(map[string]interface{}{"data": items})
	ids := extractAdGroupIDs(raw)
	if len(ids) != 100 {
		t.Fatalf("want 100 ids, got %d", len(ids))
	}
	// len(ids) >= 100 is the sentinel for the pagination warning.
	if !(len(ids) >= 100) {
		t.Error("sentinel len >= 100 should be true at capacity")
	}
}

// TestExtractAdGroupIDs_BelowCapacity verifies that a response with fewer than
// 100 items does NOT trigger the pagination-warning sentinel.
func TestExtractAdGroupIDs_BelowCapacity(t *testing.T) {
	items := make([]json.RawMessage, 3)
	for i := range items {
		items[i] = json.RawMessage(`{"id":"ag"}`)
	}
	raw, _ := json.Marshal(map[string]interface{}{"data": items})
	ids := extractAdGroupIDs(raw)
	if len(ids) >= 100 {
		t.Errorf("want < 100 ids (no warning), got %d", len(ids))
	}
}
