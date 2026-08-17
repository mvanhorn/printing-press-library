// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// All fixtures below use placeholder ids and invented message text.

// slackSearchEnvelope mirrors the real shape of a Slack search.messages response:
// hits are nested under messages.matches, which none of the generic wrapper keys
// in extractSearchResults reach.
const slackSearchEnvelope = `{
  "ok": true,
  "query": "deploy",
  "messages": {
    "total": 2,
    "paging": {"count": 20, "total": 2, "page": 1, "pages": 1},
    "matches": [
      {
        "type": "message",
        "channel": {"id": "C00000000", "name": "general"},
        "user": "U00000000",
        "username": "sample.user",
        "ts": "1700000000.000100",
        "text": "deploy finished for the staging environment",
        "permalink": "https://example.slack.com/archives/C00000000/p1700000000000100"
      },
      {
        "type": "message",
        "channel": {"id": "C00000000", "name": "random"},
        "user": "U00000000",
        "username": "sample.user",
        "ts": "1700000001.000200",
        "text": "rolling the deploy back",
        "permalink": "https://example.slack.com/archives/C00000000/p1700000001000200"
      }
    ]
  }
}`

func TestExtractSearchResultsUnwrapsSlackMatches(t *testing.T) {
	got := extractSearchResults(json.RawMessage(slackSearchEnvelope), "messages.matches")
	if len(got) != 2 {
		t.Fatalf("expected 2 matches unwrapped from messages.matches, got %d", len(got))
	}
	var first map[string]any
	if err := json.Unmarshal(got[0], &first); err != nil {
		t.Fatalf("first match is not a JSON object: %v", err)
	}
	if first["text"] != "deploy finished for the staging environment" {
		t.Errorf("unexpected first match text: %v", first["text"])
	}
}

// A Slack match has no top-level title/name/identifier/id and no score, so the
// pre-patch isNilOrEmpty discarded every hit. Guards the regression that made
// live search print "No results" with exit 0.
func TestSlackMatchSurvivesNilOrEmptyFilter(t *testing.T) {
	matches := extractSearchResults(json.RawMessage(slackSearchEnvelope), "messages.matches")
	for i, m := range matches {
		if isNilOrEmpty(m) {
			t.Errorf("match %d was discarded as nil/empty: %s", i, string(m))
		}
	}
}

// End-to-end of the two stacked defects: unwrap, then filter, and assert results
// actually survive to the caller.
func TestLiveSearchPipelineKeepsMatches(t *testing.T) {
	var kept []json.RawMessage
	for _, r := range extractSearchResults(json.RawMessage(slackSearchEnvelope), "messages.matches") {
		if !isNilOrEmpty(r) {
			kept = append(kept, r)
		}
	}
	if len(kept) != 2 {
		t.Fatalf("expected 2 results to survive the filter, got %d", len(kept))
	}
}

// A genuinely empty result set must still read as empty — the fix must not
// manufacture a false positive out of a zero-match envelope.
func TestEmptySlackSearchStillEmpty(t *testing.T) {
	const empty = `{"ok":true,"query":"nothing","messages":{"total":0,"matches":[]}}`
	var kept []json.RawMessage
	for _, r := range extractSearchResults(json.RawMessage(empty), "messages.matches") {
		if !isNilOrEmpty(r) {
			kept = append(kept, r)
		}
	}
	if len(kept) != 0 {
		t.Fatalf("expected zero results for an empty search, got %d", len(kept))
	}
}

// Slack reports application errors as HTTP 200 with ok:false. These must surface
// as errors rather than being coerced into an empty result set.
func TestCheckSlackAPIErrorSurfacesOkFalse(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"plain error", `{"ok":false,"error":"invalid_auth"}`, true},
		{"missing scope", `{"ok":false,"error":"missing_scope","needed":"search:read"}`, true},
		{"success", `{"ok":true,"messages":{"total":0,"matches":[]}}`, false},
		{"non-slack payload", `{"data":[]}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkSlackAPIError(json.RawMessage(tc.body))
			if (err != nil) != tc.want {
				t.Fatalf("checkSlackAPIError(%s) error = %v, want error: %v", tc.body, err, tc.want)
			}
		})
	}
}

// The scope-error path should name the missing scope so the user can act on it.
func TestMissingScopeErrorNamesScope(t *testing.T) {
	err := checkSlackAPIError(json.RawMessage(`{"ok":false,"error":"missing_scope","needed":"search:read"}`))
	if err == nil {
		t.Fatal("expected an error for missing_scope")
	}
	if got := err.Error(); !strings.Contains(got, "search:read") {
		t.Errorf("error should name the needed scope, got: %s", got)
	}
}
