// Copyright 2026 sdhilip200. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestBuildAwardPayloadIncludesVendorAndDateWindow(t *testing.T) {
	payload := buildAwardPayload(awardQuery{
		Vendor: "Palantir",
		NAICS:  "541511",
		From:   "2025-01-01",
		To:     "2025-12-31",
		Limit:  3,
	})
	filters := payload["filters"].(map[string]any)
	recipients := filters["recipient_search_text"].([]string)
	if recipients[0] != "Palantir" {
		t.Fatalf("recipient search text = %q", recipients[0])
	}
	if filters["naics_codes"].([]string)[0] != "541511" {
		t.Fatalf("naics code not preserved")
	}
	if payload["limit"].(int) != 3 {
		t.Fatalf("limit = %v", payload["limit"])
	}
}

func TestBuildAwardPayloadUsesSupportedSearchFilters(t *testing.T) {
	payload := buildAwardPayload(awardQuery{
		Query:  "cloud migration",
		Agency: "National Aeronautics and Space Administration",
		From:   "2025-01-01",
		To:     "2025-12-31",
		Limit:  2,
	})
	filters := payload["filters"].(map[string]any)
	if filters["keywords"].([]string)[0] != "cloud migration" {
		t.Fatalf("keywords filter not set: %#v", filters)
	}
	agencies := filters["agencies"].([]map[string]string)
	if agencies[0]["type"] != "awarding" || agencies[0]["tier"] != "toptier" {
		t.Fatalf("agency filter shape = %#v", agencies[0])
	}
	if _, ok := filters["keyword_search"]; ok {
		t.Fatalf("unsupported keyword_search filter should not be present")
	}
}

func TestSummarizeAwardsRollsUpReturnedPage(t *testing.T) {
	result := summarizeAwards(awardQuery{Vendor: "Example", From: "2025-01-01", To: "2025-12-31", Limit: 2}, spendingByAwardResponse{
		Results: []map[string]any{
			{"Award ID": "A1", "Recipient Name": "Vendor A", "Awarding Agency": "NASA", "Award Amount": 100.0, "NAICS": map[string]any{"code": "541511", "description": "Custom software"}},
			{"Award ID": "A2", "Recipient Name": "Vendor A", "Awarding Agency": "NASA", "Award Amount": 50.0, "PSC": map[string]any{"code": "D302", "description": "IT systems"}},
		},
		PageMetadata: map[string]any{"hasNext": true},
	})
	if result.TotalAmount != 150 {
		t.Fatalf("total amount = %v", result.TotalAmount)
	}
	if !result.HasNext {
		t.Fatalf("expected has next")
	}
	if len(result.TopAgencies) != 1 || result.TopAgencies[0].Name != "NASA" {
		t.Fatalf("unexpected agency rollup: %#v", result.TopAgencies)
	}
}

func TestMatchAgencyPrefersExactAbbreviation(t *testing.T) {
	match, alternates := matchAgency("NASA", []agencyReference{
		{Abbreviation: "DOE", AgencyName: "Department of Energy"},
		{Abbreviation: "NASA", AgencyName: "National Aeronautics and Space Administration"},
	})
	if match.Abbreviation != "NASA" {
		t.Fatalf("match = %#v", match)
	}
	if len(alternates) != 0 {
		t.Fatalf("alternates = %#v", alternates)
	}
}

func TestAgencyDryRunDoesNotCallNetwork(t *testing.T) {
	var out bytes.Buffer
	app := &app{
		out: &out,
		err: &bytes.Buffer{},
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatalf("dry-run made network request to %s", req.URL.String())
			return nil, nil
		})},
		now: func() time.Time { return time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC) },
		env: func(string) string { return "" },
	}
	cmd := newRootCmd(app)
	cmd.SetArgs([]string{"agency", "NASA", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var decoded dryRunResult
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if decoded.Method != "POST" || decoded.Source != "USAspending.gov" {
		t.Fatalf("unexpected dry-run result: %#v", decoded)
	}
}

func TestBuildGrantsPayloadUsesPublicSearchFields(t *testing.T) {
	payload := buildGrantsPayload(grantsQuery{Keyword: "climate", Agency: "DOC", Category: "ST", Status: "posted", Limit: 5})
	if payload["keyword"] != "climate" || payload["rows"] != 5 {
		t.Fatalf("bad grants payload: %#v", payload)
	}
	if payload["agencies"] != "DOC" || payload["fundingCategories"] != "ST" {
		t.Fatalf("missing grants filters: %#v", payload)
	}
}

func TestOpportunitiesMissingSAMKeyReturnsSetupJSON(t *testing.T) {
	var out bytes.Buffer
	app := &app{
		out:        &out,
		err:        &bytes.Buffer{},
		httpClient: http.DefaultClient,
		now:        func() time.Time { return time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC) },
		env:        func(string) string { return "" },
	}
	cmd := newRootCmd(app)
	cmd.SetArgs([]string{"opportunities", "--agent", "--query", "cloud migration"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var decoded opportunitiesResult
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if decoded.Configured {
		t.Fatalf("expected configured=false")
	}
	if !strings.Contains(decoded.Setup, "GOVSPEND_SAM_API_KEY") {
		t.Fatalf("setup did not mention env var: %q", decoded.Setup)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	if fn == nil {
		return nil, fmt.Errorf("roundTripFunc is nil")
	}
	return fn(req)
}
