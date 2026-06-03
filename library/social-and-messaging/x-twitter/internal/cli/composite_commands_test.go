// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func intPtr(i int) *int { return &i }

func TestMetricScore(t *testing.T) {
	pm := publicMetrics{
		LikeCount:       10,
		RetweetCount:    4,
		ReplyCount:      2,
		QuoteCount:      1,
		BookmarkCount:   7,
		ImpressionCount: intPtr(1000),
	}
	cases := map[string]int{
		"engagement":  17, // 10+4+2+1
		"likes":       10,
		"retweets":    4,
		"replies":     2,
		"quotes":      1,
		"bookmarks":   7,
		"impressions": 1000,
	}
	for metric, want := range cases {
		got, ok := metricScore(pm, metric)
		if !ok {
			t.Fatalf("metric %q reported unavailable", metric)
		}
		if got != want {
			t.Fatalf("metric %q = %d, want %d", metric, got, want)
		}
	}
}

func TestMetricScoreImpressionsUnavailable(t *testing.T) {
	pm := publicMetrics{LikeCount: 5} // no impression_count
	got, ok := metricScore(pm, "impressions")
	if ok {
		t.Fatal("impressions should be unavailable when impression_count is nil")
	}
	if got != 0 {
		t.Fatalf("unavailable impressions score = %d, want 0", got)
	}
}

func TestImpressionsAvailable(t *testing.T) {
	none := []tweetItem{{PublicMetrics: publicMetrics{LikeCount: 3}}}
	if impressionsAvailable(none) {
		t.Fatal("expected impressions unavailable when no item carries impression_count")
	}
	some := []tweetItem{
		{PublicMetrics: publicMetrics{LikeCount: 3}},
		{PublicMetrics: publicMetrics{ImpressionCount: intPtr(0)}}, // genuine zero, still present
	}
	if !impressionsAvailable(some) {
		t.Fatal("expected impressions available when an item carries impression_count (even zero)")
	}
}

func TestRankTopPostsOrdersByMetricAndLimits(t *testing.T) {
	items := []tweetItem{
		{ID: "1", Text: "a", PublicMetrics: publicMetrics{LikeCount: 5}},
		{ID: "2", Text: "b", PublicMetrics: publicMetrics{LikeCount: 50}},
		{ID: "3", Text: "c", PublicMetrics: publicMetrics{LikeCount: 20}},
	}
	posts := rankTopPosts(items, "acme", "likes", 2)
	if len(posts) != 2 {
		t.Fatalf("limit not applied: got %d rows, want 2", len(posts))
	}
	if posts[0].ID != "2" || posts[1].ID != "3" {
		t.Fatalf("wrong order: got %s,%s want 2,3", posts[0].ID, posts[1].ID)
	}
	if posts[0].Rank != 1 || posts[1].Rank != 2 {
		t.Fatalf("ranks not assigned: %d,%d", posts[0].Rank, posts[1].Rank)
	}
	if posts[0].Score != 50 {
		t.Fatalf("score = %d, want 50", posts[0].Score)
	}
	if posts[0].URL != "https://x.com/acme/status/2" {
		t.Fatalf("url = %q", posts[0].URL)
	}
}

func TestRankTopPostsTieBreakByEngagementThenRecency(t *testing.T) {
	// Equal like_count → break tie by total engagement, then by newer id.
	items := []tweetItem{
		{ID: "10", PublicMetrics: publicMetrics{LikeCount: 5, ReplyCount: 0}},
		{ID: "11", PublicMetrics: publicMetrics{LikeCount: 5, ReplyCount: 9}}, // higher engagement
		{ID: "12", PublicMetrics: publicMetrics{LikeCount: 5, ReplyCount: 0}}, // ties 10, newer id
	}
	posts := rankTopPosts(items, "", "likes", 3)
	if posts[0].ID != "11" {
		t.Fatalf("engagement tie-break failed: leader = %s, want 11", posts[0].ID)
	}
	if posts[1].ID != "12" || posts[2].ID != "10" {
		t.Fatalf("recency tie-break failed: got %s,%s want 12,10", posts[1].ID, posts[2].ID)
	}
}

func TestPostURLFallback(t *testing.T) {
	if got := postURL("", "999"); got != "https://x.com/i/web/status/999" {
		t.Fatalf("fallback url = %q", got)
	}
	if got := postURL("jane", "999"); got != "https://x.com/jane/status/999" {
		t.Fatalf("named url = %q", got)
	}
}

func TestFlattenText(t *testing.T) {
	if got := flattenText("line one\n\nline two\ttabbed", 100); got != "line one line two tabbed" {
		t.Fatalf("whitespace not collapsed: %q", got)
	}
	// Truncation lands on a rune boundary and appends a single ellipsis rune.
	got := flattenText("héllo wörld", 6)
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected ellipsis suffix, got %q", got)
	}
	if []rune(got)[0] != 'h' {
		t.Fatalf("unexpected leading rune in %q", got)
	}
	for _, r := range got {
		if r == '�' {
			t.Fatalf("truncation split a multibyte rune: %q", got)
		}
	}
}

func TestDecodeUserEnvelope(t *testing.T) {
	id, uname, err := decodeUserEnvelope(json.RawMessage(`{"data":{"id":"42","username":"acme"}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "42" || uname != "acme" {
		t.Fatalf("decoded id=%q username=%q", id, uname)
	}
}

func TestDecodeTweetsPageShapes(t *testing.T) {
	// Full page with pagination token.
	items, token, err := decodeTweetsPage(json.RawMessage(
		`{"data":[{"id":"1","text":"hi","public_metrics":{"like_count":3}}],"meta":{"next_token":"NEXT"}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].PublicMetrics.LikeCount != 3 {
		t.Fatalf("decoded items wrong: %+v", items)
	}
	if token != "NEXT" {
		t.Fatalf("token = %q, want NEXT", token)
	}
	// Empty timeline: no data, no meta — empty slice, empty token, no error.
	empty, token2, err := decodeTweetsPage(json.RawMessage(`{"meta":{"result_count":0}}`))
	if err != nil {
		t.Fatalf("empty page errored: %v", err)
	}
	if len(empty) != 0 || token2 != "" {
		t.Fatalf("empty page decoded to items=%d token=%q", len(empty), token2)
	}
}

func TestIsValidTopPostsMetric(t *testing.T) {
	for _, m := range topPostsMetrics {
		if !isValidTopPostsMetric(m) {
			t.Fatalf("expected %q to be valid", m)
		}
	}
	if isValidTopPostsMetric("views") {
		t.Fatal("expected 'views' to be rejected")
	}
}

// Dry-run contract: returns before any network call and emits nothing.
func TestTopPostsDryRunEmitsNothing(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newTopPostsCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run returned error: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("dry-run emitted output: %q", out.String())
	}
}
