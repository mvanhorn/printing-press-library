// Copyright 2026 sdhilip200. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	naics := filters["naics_codes"].(map[string][]string)
	if naics["require"][0] != "541511" {
		t.Fatalf("naics code not preserved: %#v", naics)
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

func TestAgencyRejectsPublishInvalidSentinel(t *testing.T) {
	app := newNoNetworkTestApp(t)
	cmd := newRootCmd(app)
	cmd.SetArgs([]string{"agency", "__printing_press_invalid__"})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected invalid agency term to fail")
	}
}

func TestVendorRejectsPublishInvalidSentinel(t *testing.T) {
	app := newNoNetworkTestApp(t)
	cmd := newRootCmd(app)
	cmd.SetArgs([]string{"vendor", "__printing_press_invalid__"})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected invalid vendor term to fail")
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

func TestBuildGrantsPayloadOmitsEmptyKeyword(t *testing.T) {
	payload := buildGrantsPayload(grantsQuery{Status: "posted", Limit: 5})
	if _, ok := payload["keyword"]; ok {
		t.Fatalf("empty keyword should not be included: %#v", payload)
	}
}

func TestAgentContextDescribesOptionalSAMKey(t *testing.T) {
	var out bytes.Buffer
	app := &app{
		out:        &out,
		err:        &bytes.Buffer{},
		httpClient: http.DefaultClient,
		now:        func() time.Time { return time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC) },
		env:        func(string) string { return "" },
	}
	cmd := newRootCmd(app)
	cmd.SetArgs([]string{"agent-context"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var decoded agentContext
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if decoded.SchemaVersion != agentContextSchemaVersion {
		t.Fatalf("schema version = %q", decoded.SchemaVersion)
	}
	if decoded.Auth.Mode != "optional_api_key" {
		t.Fatalf("auth mode = %q", decoded.Auth.Mode)
	}
	if len(decoded.Auth.EnvVars) != 1 || decoded.Auth.EnvVars[0].Name != "GOVSPEND_SAM_API_KEY" {
		t.Fatalf("unexpected auth env vars: %#v", decoded.Auth.EnvVars)
	}
}

func TestDoctorLiveReportsGrantsApplicationError(t *testing.T) {
	var out bytes.Buffer
	app := &app{
		out: &out,
		err: &bytes.Buffer{},
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.String() {
			case usaSpendingBaseURL + "/api/v2/references/toptier_agencies/":
				return jsonResponse(`{"results":[]}`), nil
			case grantsSearchURL:
				return jsonResponse(`{"errorcode":429,"msg":"OVER_RATE_LIMIT"}`), nil
			default:
				t.Fatalf("unexpected request to %s", req.URL.String())
				return nil, nil
			}
		})},
		now: func() time.Time { return time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC) },
		env: func(string) string { return "" },
	}
	cmd := newRootCmd(app)
	cmd.SetArgs([]string{"doctor", "--live", "--agent"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var decoded doctorResult
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	for _, source := range decoded.Sources {
		if source.Name == "Grants.gov" {
			if !strings.Contains(source.Live, "OVER_RATE_LIMIT") {
				t.Fatalf("expected Grants.gov application error in live status, got %q", source.Live)
			}
			return
		}
	}
	t.Fatalf("Grants.gov source not found: %#v", decoded.Sources)
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

func TestOpportunitiesErrorRedactsSAMKey(t *testing.T) {
	const apiKey = "secret-sam-key"
	app := &app{
		out: &bytes.Buffer{},
		err: &bytes.Buffer{},
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.URL.Query().Get("api_key"); got != apiKey {
				t.Fatalf("api_key not sent to SAM.gov request: %q", got)
			}
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Status:     "403 Forbidden",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("forbidden")),
			}, nil
		})},
		now: func() time.Time { return time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC) },
		env: func(key string) string {
			if key == "GOVSPEND_SAM_API_KEY" {
				return apiKey
			}
			return ""
		},
	}
	cmd := newRootCmd(app)
	cmd.SetArgs([]string{"opportunities", "--query", "cybersecurity", "--posted-from", "05/01/2026", "--posted-to", "05/31/2026"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected SAM.gov error")
	}
	if strings.Contains(err.Error(), apiKey) {
		t.Fatalf("error leaked SAM.gov api key: %v", err)
	}
	if !strings.Contains(err.Error(), "api_key=REDACTED") {
		t.Fatalf("error did not include redacted api key marker: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	if fn == nil {
		return nil, fmt.Errorf("roundTripFunc is nil")
	}
	return fn(req)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func newNoNetworkTestApp(t *testing.T) *app {
	t.Helper()
	return &app{
		out: &bytes.Buffer{},
		err: &bytes.Buffer{},
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatalf("unexpected network request to %s", req.URL.String())
			return nil, nil
		})},
		now: func() time.Time { return time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC) },
		env: func(string) string { return "" },
	}
}
