// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the review-2026-08-09 completeness fixes: sweep walks the posts
// feed's next_max_id pages, and thread reports explicit truncation instead of
// presenting a partial first page as a complete thread.

package cli

import (
	"encoding/json"
	"testing"
)

func TestExtractPostsCursor(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		cursor  string
		more    bool
	}{
		{"string cursor", `{"posts": [], "next_max_id": "QVFEbjF4"}`, "QVFEbjF4", true},
		{"numeric cursor", `{"posts": [], "next_max_id": 3141592653589793}`, "3141592653589793", true},
		{"explicit exhaustion wins over cursor", `{"more_available": false, "next_max_id": "QVFEbjF4"}`, "QVFEbjF4", false},
		{"has_more false", `{"has_more": false, "next_max_id": "abc"}`, "abc", false},
		{"no cursor means no more", `{"posts": [{"url": "https://x"}]}`, "", false},
		{"empty cursor means no more", `{"next_max_id": ""}`, "", false},
		{"zero numeric cursor means no more", `{"next_max_id": 0}`, "", false},
		{"non-object envelope", `[1,2,3]`, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cursor, more := extractPostsCursor(json.RawMessage(tc.raw))
			if cursor != tc.cursor || more != tc.more {
				t.Errorf("extractPostsCursor(%s) = (%q, %v), want (%q, %v)", tc.raw, cursor, more, tc.cursor, tc.more)
			}
		})
	}
}

func TestEnvelopeHasMore(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{"has_more true", `{"comments": [], "has_more": true}`, true},
		{"has_more false", `{"comments": [], "has_more": false}`, false},
		{"more_available true", `{"comments": [], "more_available": true}`, true},
		{"absent flags are not truncation evidence", `{"comments": [], "cursor": "QVFEbjF4"}`, false},
		{"non-boolean flag ignored", `{"has_more": "yes"}`, false},
		{"non-object envelope", `"nope"`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := envelopeHasMore(json.RawMessage(tc.raw)); got != tc.want {
				t.Errorf("envelopeHasMore(%s) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}
