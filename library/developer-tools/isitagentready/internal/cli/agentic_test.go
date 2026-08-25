// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/client"
)

// problem404 is the literal upstream application/problem+json body for a
// never-scanned domain (verified, do not alter).
const problem404 = `{"type":"https://is-agentic.com/docs#report-not-found","title":"Completed report not found",
 "status":404,"detail":"No completed Is Agentic report is stored for this URL.",
 "instance":"...","code":"report_not_found",
 "resolution":"Start a scan at https://is-agentic.com/scan/mvanhorn.com, wait for it to complete, and retry this request.",
 "documentation_url":"https://is-agentic.com/docs#errors"}`

// problem400 is the literal upstream application/problem+json body for an
// invalid URL.
const problem400 = `{"type":"https://is-agentic.com/docs#invalid-url","title":"Invalid public URL","status":400,
 "detail":"Enter a URL to scan.","instance":"...","code":"invalid_url",
 "resolution":"Pass one public HTTP or HTTPS URL in the required url query parameter.",
 "documentation_url":"https://is-agentic.com/docs#errors"}`

func TestClassifyAgenticError(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		body     string
		wantCode int
		wantSub  string // a substring that must appear verbatim in the message
	}{
		{"404 report_not_found", 404, problem404, 3, "Start a scan at https://is-agentic.com/scan/mvanhorn.com, wait for it to complete, and retry this request."},
		{"400 invalid_url", 400, problem400, 2, "Pass one public HTTP or HTTPS URL in the required url query parameter."},
		{"429 rate limit", 429, `{"type":"x","title":"Too Many Requests","status":429,"detail":"Rate limit exceeded","code":"rate_limited"}`, 7, "Rate limit exceeded"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			apiErr := &client.APIError{Method: "GET", Path: "/api/v1/report", StatusCode: tc.status, Body: tc.body}
			err := classifyAgenticError(apiErr, nil)
			if got := ExitCode(err); got != tc.wantCode {
				t.Fatalf("ExitCode = %d, want %d (err=%v)", got, tc.wantCode, err)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("message %q does not contain upstream substring %q verbatim", err.Error(), tc.wantSub)
			}
		})
	}
}

// TestClassifyAgenticErrorFallback covers the non-problem+json fallback:
// when the body does not parse, classifyAgenticError hands off to
// classifyAPIError so the generic HTTP classification (and its exit code)
// still applies instead of silently returning a no-data result.
func TestClassifyAgenticErrorFallback(t *testing.T) {
	apiErr := &client.APIError{Method: "GET", Path: "/api/v1/report", StatusCode: 500, Body: "not json at all"}
	err := classifyAgenticError(apiErr, nil)
	// 500 with unparseable body falls through to classifyAPIError -> apiErr.
	if got := ExitCode(err); got != 5 {
		t.Fatalf("ExitCode = %d, want 5 (apiErr fallback), err=%v", got, err)
	}
}
