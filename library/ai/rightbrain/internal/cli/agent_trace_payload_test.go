// Copyright 2026 Farouk Umar and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

// toolResultPayloadError decides whether a completed tool call actually failed.
// It has two ways to be wrong and both matter: missing a real failure restores
// the bug the command was written to fix, and flagging a healthy payload makes
// the failure count untrustworthy in the other direction. Both are covered here.
func TestToolResultPayloadErrorDetectsRealFailures(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		payload any
		wantSub string
	}{
		{"tool error marker", "[Tool Error] request failed", "[Tool Error]"},
		{"json error object", `{"object":"error","status":404}`, "JSON error object"},
		{"json error object spaced", `{"object": "error"}`, "JSON error object"},
		{"api response error", "notionhq.errors.APIResponseError: not found", "APIResponseError"},
		{"not-found code", `{"code":"object_not_found","message":"no such page"}`, "error code"},
		{"not-found code spaced", `{"code": "page_not_found"}`, "error code"},
		{"error_code key", `{"error_code":"validation_not_found"}`, "error code"},
		{"unauthorized code", `{"code":"unauthorized"}`, "error code"},
		{"numeric status", `{"status": 503}`, "HTTP 503"},
		{"status_code", `{"status_code":429}`, "HTTP 429"},
		{"stringified status", `{"status": "404 Not Found"}`, "HTTP 404"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := toolResultPayloadError(map[string]any{"tool_result": tc.payload})
			if !strings.Contains(got, tc.wantSub) {
				t.Errorf("toolResultPayloadError(%v) = %q, want it to mention %q",
					tc.payload, got, tc.wantSub)
			}
		})
	}
}

// TestToolResultPayloadErrorIgnoresHealthyPayloads is the false-positive guard.
//
// The first three cases are why the code-field match is anchored: a tool that
// searched documentation, listed an API's error codes, or returned source code
// can legitimately carry the text "_not_found" while having succeeded. Matching
// that substring anywhere reports a silent failure on a run where nothing
// failed — the same wrong-but-confident answer this command exists to correct,
// pointing the other way.
func TestToolResultPayloadErrorIgnoresHealthyPayloads(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		payload any
	}{
		{"docs search result mentioning the code", `{"results":[{"title":"Handling object_not_found errors"}]}`},
		{"reference listing error codes", `{"codes":["object_not_found","unauthorized"],"ok":true}`},
		{"source code containing the identifier", "func handle() { return page_not_found }"},
		{"successful status", `{"status": 200,"ok":true}`},
		{"redirect status", `{"status": 302}`},
		{"stringified success", `{"status": "success"}`},
		{"prose about errors", "The error handling guide explains what an error is."},
		{"empty payload", ""},
		{"no result key", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := map[string]any{}
			if tc.payload != nil {
				res["tool_result"] = tc.payload
			}
			if got := toolResultPayloadError(res); got != "" {
				t.Errorf("toolResultPayloadError(%v) = %q, want \"\" (payload is healthy)",
					tc.payload, got)
			}
		})
	}
}

// TestBuildAgentTraceCountsSilentFailureOnce checks the accounting: a payload
// error on a call the API called "success" is both an error and specifically a
// silent one, while an explicitly failed call is an error but not silent.
func TestBuildAgentTraceCountsSilentFailure(t *testing.T) {
	t.Parallel()

	events := []agentEvent{
		{
			Timestamp: tsWholeSecond,
			ToolCalls: []map[string]any{
				{"tool_call_id": "c1", "tool_name": "notion_fetch"},
				{"tool_call_id": "c2", "tool_name": "notion_search"},
			},
		},
		{
			Timestamp: tsSubSecond,
			ToolResults: []map[string]any{
				// Reported as success by the platform; the payload says 404.
				{"tool_call_id": "c1", "tool_outcome": "success",
					"tool_result": `{"object":"error","code":"object_not_found"}`},
				// Honestly reported failure: an error, but not a silent one.
				{"tool_call_id": "c2", "tool_outcome": "error", "tool_result": "boom"},
			},
		},
	}

	report := buildAgentTrace("run-1", "agent-1", "sess-1", events)
	if report.ToolErrors != 2 {
		t.Errorf("ToolErrors = %d, want 2 (one silent, one declared)", report.ToolErrors)
	}
	if report.SilentToolFailures != 1 {
		t.Errorf("SilentToolFailures = %d, want 1 (only the success-labelled 404)",
			report.SilentToolFailures)
	}
}
