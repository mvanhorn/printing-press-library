// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
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
	if posts[0].Score == nil || *posts[0].Score != 50 || posts[0].ScoreMetric != "likes" {
		t.Fatalf("score = %+v, want 50 likes", posts[0])
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

func TestIDNewerNumeric(t *testing.T) {
	// Cross-length: numerically 10000 > 9999, but lexicographically "9999" > "10000".
	if !idNewer("10000", "9999") {
		t.Fatal("expected 10000 to be newer than 9999 (numeric, not lexicographic)")
	}
	if idNewer("9999", "10000") {
		t.Fatal("expected 9999 to be older than 10000")
	}
	// Same-length Snowflake-shaped ids compare numerically.
	if !idNewer("1790000000000000002", "1790000000000000001") {
		t.Fatal("expected the higher snowflake id to be newer")
	}
	// Non-numeric fallback stays deterministic and does not panic.
	if idNewer("abc", "abc") {
		t.Fatal("equal non-numeric ids should not report newer")
	}
}

func TestRankTopPostsRecencyTieBreakIsNumeric(t *testing.T) {
	// All metrics equal → tie-break by newer id, compared numerically.
	items := []tweetItem{
		{ID: "9999", PublicMetrics: publicMetrics{LikeCount: 5}},
		{ID: "10000", PublicMetrics: publicMetrics{LikeCount: 5}},
	}
	posts := rankTopPosts(items, "", "likes", 2)
	if posts[0].ID != "10000" {
		t.Fatalf("numeric recency tie-break failed: leader = %s, want 10000", posts[0].ID)
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

func TestTopPostsRejectsNonNumericUserIDBeforeAnyRequest(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newTopPostsCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--user-id", "42/tweets?extra=true"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "only digits") {
		t.Fatalf("user ID must be rejected before a request, got %v", err)
	}
}

func TestMissingImpressionsCount(t *testing.T) {
	items := []tweetItem{
		{PublicMetrics: publicMetrics{LikeCount: 3}},
		{PublicMetrics: publicMetrics{ImpressionCount: intPtr(0)}},
		{PublicMetrics: publicMetrics{ImpressionCount: intPtr(9)}},
	}
	if got := missingImpressionsCount(items); got != 1 {
		t.Fatalf("missing impressions = %d, want 1", got)
	}
}

func TestRankTopPostsPartialImpressionsKeepUnknownSeparateFromZero(t *testing.T) {
	items := []tweetItem{
		{ID: "1", PublicMetrics: publicMetrics{ImpressionCount: intPtr(0)}},
		{ID: "2", PublicMetrics: publicMetrics{LikeCount: 99}},
	}
	posts := rankTopPosts(items, "", "impressions", 2)
	if posts[0].ID != "1" || posts[0].Score == nil || *posts[0].Score != 0 {
		t.Fatalf("measured zero must rank before unknown impressions, got %+v", posts[0])
	}
	if posts[1].ID != "2" || posts[1].Score != nil || posts[1].Impressions != nil || posts[1].ScoreMetric != "impressions" {
		t.Fatalf("unknown impressions must have null score, got %+v", posts[1])
	}
}

func TestTopPostsRejectsExcessiveFetchBeforeAnyRequest(t *testing.T) {
	cmd := newTopPostsCmd(&rootFlags{dryRun: true})
	cmd.SetArgs([]string{"--max-fetch", "1001"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "at most 1000") {
		t.Fatalf("excessive fetch must be rejected before a request, got %v", err)
	}
}

func TestTopPostsPagesThroughEmptyIntermediatePageWithoutExtraUserLookup(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.String())
		if r.URL.Path != "/2/users/42/tweets" || r.URL.Query().Get("tweet.fields") != "public_metrics,created_at" || r.URL.Query().Get("max_results") != "100" {
			t.Errorf("unexpected request: %s", r.URL.String())
		}
		switch r.URL.Query().Get("pagination_token") {
		case "":
			fmt.Fprint(w, `{"data":[{"id":"1","text":"first","public_metrics":{"like_count":2}}],"meta":{"next_token":"A"}}`)
		case "A":
			fmt.Fprint(w, `{"data":[],"meta":{"next_token":"B"}}`)
		case "B":
			fmt.Fprint(w, `{"data":[{"id":"2","text":"second","public_metrics":{"like_count":5}}]}`)
		default:
			t.Errorf("unexpected pagination token")
		}
	}))
	defer server.Close()
	t.Setenv("X_TWITTER_BASE_URL", server.URL)
	t.Setenv("X_BEARER_TOKEN", "synthetic-app-token")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")
	t.Setenv("X_TWITTER_CONFIG", t.TempDir()+"/missing-config.toml")
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"top-posts", "--user-id", "42", "--max-fetch", "200", "--metric", "likes", "--json", "--no-cache"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("synthetic timeline: %v", err)
	}
	if len(calls) != 3 {
		t.Fatalf("requests = %v, want three timeline pages and no user lookup", calls)
	}
	var posts []rankedPost
	if err := json.Unmarshal(out.Bytes(), &posts); err != nil {
		t.Fatalf("decode ranked posts: %v; output=%q", err, out.String())
	}
	if len(posts) != 2 || posts[0].ID != "2" || posts[0].ScoreMetric != "likes" || posts[0].URL != "https://x.com/i/web/status/2" {
		t.Fatalf("ranked posts = %+v", posts)
	}
}

func TestTopPostsRejectsRepeatedPaginationTokenBeforeAnotherPaidRead(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		fmt.Fprint(w, `{"data":[],"meta":{"next_token":"AGAIN"}}`)
	}))
	defer server.Close()
	t.Setenv("X_TWITTER_BASE_URL", server.URL)
	t.Setenv("X_BEARER_TOKEN", "synthetic-app-token")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")
	t.Setenv("X_TWITTER_CONFIG", t.TempDir()+"/missing-config.toml")
	cmd := RootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"top-posts", "--user-id", "42", "--json", "--no-cache"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "repeated") || requests != 2 {
		t.Fatalf("repeated token must stop after two reads: err=%v requests=%d", err, requests)
	}
}

func TestTopPostsFallbackMetricIsVisibleInJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"id":"1","text":"first","public_metrics":{"like_count":2}}]}`)
	}))
	defer server.Close()
	t.Setenv("X_TWITTER_BASE_URL", server.URL)
	t.Setenv("X_BEARER_TOKEN", "synthetic-app-token")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")
	t.Setenv("X_TWITTER_CONFIG", t.TempDir()+"/missing-config.toml")
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"top-posts", "--user-id", "42", "--metric", "impressions", "--json", "--no-cache"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("synthetic timeline: %v", err)
	}
	var posts []rankedPost
	if err := json.Unmarshal(out.Bytes(), &posts); err != nil || len(posts) != 1 || posts[0].ScoreMetric != "engagement" {
		t.Fatalf("fallback metric must be visible in JSON: posts=%+v err=%v", posts, err)
	}
}

func TestTopPostsResolvesAuthenticatedUserThenReadsTimeline(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		switch r.URL.Path {
		case "/2/users/me":
			fmt.Fprint(w, `{"data":{"id":"42","username":"fixture-user"}}`)
		case "/2/users/42/tweets":
			fmt.Fprint(w, `{"data":[{"id":"1","text":"first","public_metrics":{"like_count":2}}]}`)
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("X_TWITTER_BASE_URL", server.URL)
	t.Setenv("X_BEARER_TOKEN", "")
	t.Setenv("X_OAUTH2_USER_TOKEN", "synthetic-user-token")
	t.Setenv("X_TWITTER_CONFIG", t.TempDir()+"/missing-config.toml")
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"top-posts", "--json", "--no-cache"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("synthetic authenticated timeline: %v", err)
	}
	if len(calls) != 2 || calls[0] != "/2/users/me" || calls[1] != "/2/users/42/tweets" {
		t.Fatalf("calls = %v, want identity lookup then timeline", calls)
	}
	var posts []rankedPost
	if err := json.Unmarshal(out.Bytes(), &posts); err != nil || len(posts) != 1 || posts[0].URL != "https://x.com/fixture-user/status/1" {
		t.Fatalf("ranked posts = %+v, error=%v", posts, err)
	}
}

func TestTopPostsStopsLongEmptyPaginationAtPaidReadBudget(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		fmt.Fprintf(w, `{"data":[],"meta":{"next_token":%q}}`, strconv.Itoa(requests))
	}))
	defer server.Close()
	t.Setenv("X_TWITTER_BASE_URL", server.URL)
	t.Setenv("X_BEARER_TOKEN", "synthetic-app-token")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")
	t.Setenv("X_TWITTER_CONFIG", t.TempDir()+"/missing-config.toml")
	cmd := RootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"top-posts", "--user-id", "42", "--json", "--no-cache"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "limit paid API reads") || requests != 4 {
		t.Fatalf("empty pages must stop at four synthetic reads: err=%v requests=%d", err, requests)
	}
}

func TestTopPostsKeepsPartialLeaderboardAtPaidReadBudget(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		fmt.Fprintf(w, `{"data":[{"id":%q,"text":"sparse","public_metrics":{"like_count":1}}],"meta":{"next_token":%q}}`, strconv.Itoa(requests), strconv.Itoa(requests))
	}))
	defer server.Close()
	t.Setenv("X_TWITTER_BASE_URL", server.URL)
	t.Setenv("X_BEARER_TOKEN", "synthetic-app-token")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")
	t.Setenv("X_TWITTER_CONFIG", t.TempDir()+"/missing-config.toml")
	cmd := RootCmd()
	var out, errout bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errout)
	cmd.SetArgs([]string{"top-posts", "--user-id", "42", "--json", "--no-cache"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("partial leaderboard should succeed with a warning: %v", err)
	}
	var posts []rankedPost
	if err := json.Unmarshal(out.Bytes(), &posts); err != nil || len(posts) != 4 || requests != 4 {
		t.Fatalf("want four saved rows after four reads: posts=%+v requests=%d err=%v", posts, requests, err)
	}
	for _, post := range posts {
		if !post.Truncated {
			t.Fatalf("partial row missing truncated marker: %+v", post)
		}
	}
	if !strings.Contains(errout.String(), "truncated") || !strings.Contains(errout.String(), "more may exist") {
		t.Fatalf("missing accurate warnings for partial ranking: %q", errout.String())
	}
}
