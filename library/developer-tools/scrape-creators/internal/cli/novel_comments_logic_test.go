// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"testing"
)

func TestParseCommentsPayload(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    int
		credits int64
	}{
		{"top-level comments", `{"comments":[{"id":"1"},{"id":"2"}],"credits_charged":1}`, 2, 1},
		{"replies key", `{"replies":[{"id":"9"}],"credits_charged":1}`, 1, 1},
		{"data wrapper", `{"data":{"comments":[{"id":"3"}]},"credits_charged":2}`, 1, 2},
		{"empty", `{"success":true,"credits_charged":1}`, 0, 1},
		{"not json", `[]`, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCommentsPayload(json.RawMessage(tc.raw))
			if len(got.comments) != tc.want {
				t.Fatalf("comments = %d, want %d", len(got.comments), tc.want)
			}
			if got.creditsCharged != tc.credits {
				t.Fatalf("credits = %d, want %d", got.creditsCharged, tc.credits)
			}
		})
	}
}

func TestCommentToRowsInlineReplies(t *testing.T) {
	raw := json.RawMessage(`{"id":"p1","text":"question?","like_count":3,"replies":[{"id":"r1","text":"answer"},{"id":"r2","text":"more"}]}`)
	rows, replies := commentToRows(raw, "https://example.invalid/post", "")
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	if replies != 2 {
		t.Fatalf("replies = %d, want 2", replies)
	}
	if rows[1].ParentID != "p1" || rows[2].ParentID != "p1" {
		t.Fatalf("reply rows must carry the parent id, got %q/%q", rows[1].ParentID, rows[2].ParentID)
	}
	if rows[0].LikeCount != 3 {
		t.Fatalf("like_count = %d, want 3", rows[0].LikeCount)
	}
}

func TestCommentToRowsPkFallbackAndMissingID(t *testing.T) {
	rows, _ := commentToRows(json.RawMessage(`{"pk":"77","text":"via pk"}`), "u", "")
	if len(rows) != 1 || rows[0].CommentID != "77" {
		t.Fatalf("pk fallback failed: %+v", rows)
	}
	rows, _ = commentToRows(json.RawMessage(`{"text":"no id"}`), "u", "")
	if len(rows) != 0 {
		t.Fatalf("rows without any id must be dropped, got %d", len(rows))
	}
}

func TestExtractPosts(t *testing.T) {
	raw := json.RawMessage(`{"posts":[{"url":"https://x/p/1","taken_at":1700000000},{"code":"abc"},{"caption":"no url"}]}`)
	got := extractPosts(raw)
	if len(got) != 2 {
		t.Fatalf("posts = %d, want 2 (the third has no url or code)", len(got))
	}
	if got[1].url != "https://www.instagram.com/p/abc" {
		t.Fatalf("code fallback url = %q", got[1].url)
	}
	if got[0].takenAt.IsZero() {
		t.Fatalf("taken_at should be parsed")
	}
}

func TestExtractUserIDAndTaggedIDs(t *testing.T) {
	if id := extractUserID(json.RawMessage(`{"data":{"user":{}},"user":{"pk":123456}}`)); id != "123456" {
		t.Fatalf("extractUserID = %q, want 123456", id)
	}
	if id := extractUserID(json.RawMessage(`{"id":"789"}`)); id != "789" {
		t.Fatalf("extractUserID top-level = %q, want 789", id)
	}
	ids := extractTaggedIDs(json.RawMessage(`{"items":[{"id":"a"},{"pk":42},{"nope":true}]}`))
	if len(ids) != 2 || ids[0] != "a" || ids[1] != "42" {
		t.Fatalf("extractTaggedIDs = %v", ids)
	}
}

func TestFtsQuoteEscapesQuotes(t *testing.T) {
	if q := ftsQuote(`say "hi"`); q != `"say ""hi"""` {
		t.Fatalf("ftsQuote = %q", q)
	}
}
