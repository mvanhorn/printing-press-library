// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"
)

func TestNormalizeEchoDate(t *testing.T) {
	for input, want := range map[string]string{"07/01/2023": "2023-07-01", "20260717": "2026-07-17"} {
		if got := normalizeEchoDate(input); got != want {
			t.Fatalf("normalizeEchoDate(%q)=%q want %q", input, got, want)
		}
	}
}

func TestChangedSectionsPreservesBeforeAndAfter(t *testing.T) {
	changes := changedSections(map[string]any{"A": 1, "B": 2}, map[string]any{"A": 1, "B": 3})
	if len(changes) != 1 || changes[0]["section"] != "B" {
		t.Fatalf("unexpected changes: %#v", changes)
	}
}

func TestRankedFacilityUsesDeterministicEvidence(t *testing.T) {
	row := rankedFacility(map[string]any{"RegistryID": "110009441979", "FacName": "CAPITAL EXXON", "FacState": "RI"}, map[string]string{"name": "Capital Exxon", "state": "RI"})
	if row["match_score"].(int) != 65 || len(row["match_evidence"].([]string)) != 2 {
		t.Fatalf("ranked row = %#v", row)
	}
}

func TestNormalizedSectionRecordsCountsOnlyExplicitExceedance(t *testing.T) {
	records := normalizedSectionRecords(map[string]any{"CWA": []any{map[string]any{"Pollutant": "Lead", "Period": "2026-Q1", "Exceedance": "Y"}, map[string]any{"Pollutant": "Copper"}}})
	if len(records) != 2 || explicitExceedanceCount(records) != 1 {
		t.Fatalf("records=%#v count=%d", records, explicitExceedanceCount(records))
	}
}

func TestParseLookback(t *testing.T) {
	got, err := parseLookback("30d")
	if err != nil || got.Hours() != 720 {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestFilterTimelineSinceRetainsUnparsedEvidence(t *testing.T) {
	entries := []map[string]any{{"date": "EPA-date-format", "id": "A"}, {"date": "2020-01-01", "id": "B"}}
	got := filterTimelineSince(entries, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if len(got) != 1 || got[0]["id"] != "A" || got[0]["since_filter_status"] != "unparsed_date_included" {
		t.Fatalf("got=%#v", got)
	}
}
