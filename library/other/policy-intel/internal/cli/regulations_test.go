// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseRegulationsList(t *testing.T) {
	body := []byte(`{
	  "data": [{
	    "id": "FDA-2023-C-5679-0002",
	    "type": "documents",
	    "attributes": {
	      "agencyId": "FDA",
	      "documentType": "Other",
	      "title": "Color Additive Petition",
	      "docketId": "FDA-2023-C-5679",
	      "frDocNum": null,
	      "postedDate": "2024-01-11T05:00:00Z",
	      "commentStartDate": "2024-01-11T05:00:00Z",
	      "commentEndDate": "2026-06-30T03:59:59Z",
	      "openForComment": true,
	      "withinCommentPeriod": true
	    }
	  }],
	  "meta": {"totalElements": 8, "pageSize": 5, "hasNextPage": true}
	}`)

	result, err := parseRegulationsList(body)
	if err != nil {
		t.Fatalf("parseRegulationsList returned error: %v", err)
	}
	if result.Total != 8 || result.PageSize != 5 || !result.HasNextPage {
		t.Fatalf("unexpected meta: %#v", result)
	}
	doc := result.Results[0]
	if doc.AgencyID != "FDA" || doc.DocketID != "FDA-2023-C-5679" {
		t.Fatalf("unexpected document: %#v", doc)
	}
	if !doc.OpenForComment || !doc.WithinCommentPeriod {
		t.Fatalf("expected open comment flags: %#v", doc)
	}
}

func TestRegulationsAPIKeyFallsBackToDemoKey(t *testing.T) {
	t.Setenv("POLICY_INTEL_REGULATIONS_API_KEY", "")
	if got := regulationsAPIKey(); got != "DEMO_KEY" {
		t.Fatalf("regulationsAPIKey = %q, want DEMO_KEY", got)
	}
	t.Setenv("POLICY_INTEL_REGULATIONS_API_KEY", "real-key")
	if got := regulationsAPIKey(); got != "real-key" {
		t.Fatalf("regulationsAPIKey = %q, want configured key", got)
	}
}

func TestRegulationsDocumentJSONIncludesFalseCommentFlags(t *testing.T) {
	doc := RegulationsDocument{
		ID:                  "EPA-HQ-OPPT-2018-0462-0001",
		OpenForComment:      false,
		WithinCommentPeriod: false,
	}
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	got := string(body)
	for _, want := range []string{`"open_for_comment":false`, `"within_comment_period":false`} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %s in JSON, got %s", want, got)
		}
	}
}
