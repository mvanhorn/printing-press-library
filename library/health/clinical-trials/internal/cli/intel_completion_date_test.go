// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// rawCompletionStudy builds one CT.gov study object carrying the two
// completion-date structs, in the fixture style timeline_test.go uses: only the
// fields under test are populated, the rest of protocolSection is left absent.
func rawCompletionStudy(nct, primary, completion string) json.RawMessage {
	status := map[string]any{"overallStatus": "COMPLETED"}
	if primary != "" {
		status["primaryCompletionDateStruct"] = map[string]any{"date": primary}
	}
	if completion != "" {
		status["completionDateStruct"] = map[string]any{"date": completion}
	}
	b, _ := json.Marshal(map[string]any{
		"protocolSection": map[string]any{
			"identificationModule": map[string]any{
				"nctId":      nct,
				"briefTitle": "A completed trial",
			},
			"statusModule": status,
		},
	})
	return json.RawMessage(b)
}

// TestNormalizedFieldsRequestsCompletionDates guards the actual defect: the two
// completion-date fields must be asked for, or CT.gov never returns them.
func TestNormalizedFieldsRequestsCompletionDates(t *testing.T) {
	for _, f := range []string{"PrimaryCompletionDate", "CompletionDate"} {
		if !strings.Contains(normalizedFields, f) {
			t.Errorf("normalizedFields is missing %q — every intelligence command would return it empty", f)
		}
	}
}

func TestNormalizeStudyCarriesCompletionDates(t *testing.T) {
	t.Run("populated when CT.gov supplies them", func(t *testing.T) {
		tr, ok := normalizeStudy(rawCompletionStudy("NCT00000001", "2021-06-30", "2022-01-15"))
		if !ok {
			t.Fatal("normalizeStudy returned ok=false")
		}
		if tr.PrimaryCompletionDate != "2021-06-30" {
			t.Errorf("PrimaryCompletionDate = %q, want %q", tr.PrimaryCompletionDate, "2021-06-30")
		}
		if tr.CompletionDate != "2022-01-15" {
			t.Errorf("CompletionDate = %q, want %q", tr.CompletionDate, "2022-01-15")
		}

		b, err := json.Marshal(tr)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var got map[string]any
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		// Exact JSON names — the Trialvera web layer reads these two keys.
		if got["completion_date"] != "2022-01-15" {
			t.Errorf("json completion_date = %v, want 2022-01-15", got["completion_date"])
		}
		if got["primary_completion_date"] != "2021-06-30" {
			t.Errorf("json primary_completion_date = %v, want 2021-06-30", got["primary_completion_date"])
		}
	})

	t.Run("absent, not empty, when no date is recorded", func(t *testing.T) {
		tr, ok := normalizeStudy(rawCompletionStudy("NCT00000002", "", ""))
		if !ok {
			t.Fatal("normalizeStudy returned ok=false")
		}
		b, err := json.Marshal(tr)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var got map[string]any
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		// omitempty: downstream must be able to tell "no date recorded" from
		// "field present but blank".
		if _, present := got["completion_date"]; present {
			t.Error("completion_date present in JSON for a trial with no completion date; want omitted")
		}
		if _, present := got["primary_completion_date"]; present {
			t.Error("primary_completion_date present in JSON for a trial with no date; want omitted")
		}
	})
}
