// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestExtractPostID(t *testing.T) {
	cases := map[string]string{
		"1234567890": "1234567890",
		"https://x.com/sama/status/1737145600000000000":           "1737145600000000000",
		"https://twitter.com/u/status/1737145600000000000?s=20":   "1737145600000000000",
		"https://mobile.twitter.com/u/status/1737145600000000000": "1737145600000000000",
		"https://x.com/i/web/status/1737145600000000000":          "1737145600000000000",
	}
	for input, want := range cases {
		got, err := extractPostID(input)
		if err != nil {
			t.Fatalf("extractPostID(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("extractPostID(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestExtractPostIDRejectsUnsupportedInput(t *testing.T) {
	for _, input := range []string{"not-a-url", "https://example.com/u/status/12345", "https://x.com/user"} {
		if got, err := extractPostID(input); err == nil {
			t.Fatalf("extractPostID(%q) = %q, want error", input, got)
		}
	}
}

func TestNormalizeTweetRecord(t *testing.T) {
	users := map[string]*postAuthorSummary{
		"42": {ID: "42", Username: "alice", Name: "Alice"},
	}
	rec, err := normalizeTweetRecord("123", []byte(`{
		"id":"123",
		"author_id":"42",
		"text":"hello",
		"created_at":"2026-01-01T00:00:00Z",
		"conversation_id":"100",
		"referenced_tweets":[{"type":"replied_to","id":"100"}],
		"public_metrics":{"like_count":7}
	}`), users, "live", "not_synced", parseIncludeSet("refs,metrics"))
	if err != nil {
		t.Fatalf("normalizeTweetRecord returned error: %v", err)
	}
	if rec.URL != "https://x.com/alice/status/123" {
		t.Fatalf("URL = %q", rec.URL)
	}
	if rec.PostType != "reply" {
		t.Fatalf("PostType = %q, want reply", rec.PostType)
	}
	if len(rec.ReferencedTweets) != 1 || rec.ReferencedTweets[0].ID != "100" {
		t.Fatalf("ReferencedTweets = %+v", rec.ReferencedTweets)
	}
	if rec.PublicMetrics["like_count"].(float64) != 7 {
		t.Fatalf("PublicMetrics = %+v", rec.PublicMetrics)
	}
}
