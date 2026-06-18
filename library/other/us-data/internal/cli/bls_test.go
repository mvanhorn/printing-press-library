// Copyright 2026 Dhilip Subramanian. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestParseBLSSkipsAnnualAverageAndComputesChange(t *testing.T) {
	body := []byte(`{
	  "status": "REQUEST_SUCCEEDED",
	  "message": [],
	  "Results": {"series": [{
	    "seriesID": "CUUR0000SA0",
	    "data": [
	      {"year": "2026", "period": "M05", "periodName": "May", "latest": "true", "value": "105.0"},
	      {"year": "2026", "period": "M04", "periodName": "April", "value": "100.0"},
	      {"year": "2025", "period": "M13", "periodName": "Annual", "value": "99.0"}
	    ]
	  }]}
	}`)

	result, err := parseBLS("CUUR0000SA0", "CPI", body)
	if err != nil {
		t.Fatalf("parseBLS returned error: %v", err)
	}
	if result.Latest.Period != "M05" {
		t.Fatalf("latest period = %s, want M05", result.Latest.Period)
	}
	if len(result.Observations) != 2 {
		t.Fatalf("observations = %d, want 2 monthly observations", len(result.Observations))
	}
	if result.PercentChange == nil || *result.PercentChange != 5 {
		t.Fatalf("percent change = %v, want 5", result.PercentChange)
	}
}

func TestParseBLSRejectsFailedStatus(t *testing.T) {
	body := []byte(`{"status":"REQUEST_FAILED","message":["bad series"],"Results":{"series":[]}}`)

	_, err := parseBLS("BAD", "Bad", body)
	if err == nil {
		t.Fatal("parseBLS returned nil error for failed response")
	}
}
