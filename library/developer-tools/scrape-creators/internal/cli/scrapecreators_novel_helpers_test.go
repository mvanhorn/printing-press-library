// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

func TestExtractFollowerCount(t *testing.T) {
	cases := []struct {
		name string
		json string
		want int64
		ok   bool
	}{
		{"tiktok_string_nested", `{"user":{"id":"1"},"stats":{"followerCount":"129001955","followingCount":"351"}}`, 129001955, true},
		{"youtube_number", `{"name":"MrBeast","subscriberCount":505000000,"videoCount":100}`, 505000000, true},
		{"twitter_nested_legacy", `{"creator_subscriptions_count":0,"legacy":{"fast_followers_count":0,"followers_count":18000000,"friends_count":12}}`, 18000000, true},
		{"instagram_edge", `{"data":{"user":{"edge_followed_by":{"count":42000000},"edge_follow":{"count":50}}}}`, 42000000, true},
		{"github_followers", `{"login":"torvalds","followers":309290,"following":0}`, 309290, true},
		{"bluesky_followerscount", `{"followersCount":1200,"followsCount":5,"postsCount":10}`, 1200, true},
		{"threads_follower_count", `{"username":"x","follower_count":999}`, 999, true},
		{"none", `{"nothing":"here","following":12}`, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := extractFollowerCount(json.RawMessage(c.json))
			if ok != c.ok || got != c.want {
				t.Fatalf("extractFollowerCount(%s) = (%d,%v), want (%d,%v)", c.name, got, ok, c.want, c.ok)
			}
		})
	}
}

func TestExtractFollowerCountIgnoresFollowing(t *testing.T) {
	// followingCount / followsCount must never be picked as a follower total.
	got, _ := extractFollowerCount(json.RawMessage(`{"followingCount":"351","followsCount":"22"}`))
	if got != 0 {
		t.Fatalf("following keys leaked into follower count: got %d", got)
	}
}

func TestToInt64(t *testing.T) {
	cases := []struct {
		in   any
		want int64
		ok   bool
	}{
		{float64(42), 42, true},
		{"  17 ", 17, true},
		{"3.9", 3, true},
		{"", 0, false},
		{"abc", 0, false},
		{json.Number("88"), 88, true},
	}
	for _, c := range cases {
		got, ok := toInt64(c.in)
		if got != c.want || ok != c.ok {
			t.Fatalf("toInt64(%v) = (%d,%v), want (%d,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestExtractContentMetrics(t *testing.T) {
	item := json.RawMessage(`{"aweme_id":"7","desc":"hi","statistics":{"play_count":9366236,"digg_count":1017621,"comment_count":17887,"share_count":16478}}`)
	m := extractContentMetrics(item)
	if m.views != 9366236 {
		t.Fatalf("views = %d", m.views)
	}
	if m.engagement() != 1017621+17887+16478 {
		t.Fatalf("engagement = %d", m.engagement())
	}
}

func TestResultArray(t *testing.T) {
	// keyed
	if got := resultArray(json.RawMessage(`{"posts":[1,2,3],"after":"x"}`), "posts"); len(got) != 3 {
		t.Fatalf("keyed len = %d", len(got))
	}
	// fallback to largest array when key missing
	if got := resultArray(json.RawMessage(`{"a":[1],"b":[1,2,3,4]}`), "missing"); len(got) != 4 {
		t.Fatalf("fallback len = %d", len(got))
	}
	// top-level array body
	if got := resultArray(json.RawMessage(`[{"x":1},{"x":2}]`), "anything"); len(got) != 2 {
		t.Fatalf("toplevel len = %d", len(got))
	}
	// empty object -> zero, not panic
	if got := resultArray(json.RawMessage(`{}`), "k"); len(got) != 0 {
		t.Fatalf("empty len = %d", len(got))
	}
}

func TestExtractItemID(t *testing.T) {
	cases := []struct {
		json    string
		idField string
		want    string
	}{
		{`{"ad_archive_id":"123","page_id":"9"}`, "ad_archive_id", "123"},
		{`{"id":456}`, "id", "456"},
		{`{"advertiser_id":"AR99"}`, "advertiser_id", "AR99"},
		{`{"aweme_id":"7654"}`, "missing", "7654"},
	}
	for _, c := range cases {
		if got := extractItemID(json.RawMessage(c.json), c.idField); got != c.want {
			t.Fatalf("extractItemID(%s, %s) = %q, want %q", c.json, c.idField, got, c.want)
		}
	}
}

func TestPlatformFromResourceType(t *testing.T) {
	cases := map[string]string{
		"tiktok-video-transcript":           "tiktok",
		"facebook-ad-library-ad-transcript": "facebook",
		"reddit-post-transcript":            "reddit",
		"nodash":                            "nodash",
	}
	for in, want := range cases {
		if got := platformFromResourceType(in); got != want {
			t.Fatalf("platformFromResourceType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExtractString(t *testing.T) {
	if got := extractString(json.RawMessage(`{"meta":{"username":"mrbeast"}}`), creatorNameKeys); got != "mrbeast" {
		t.Fatalf("creator = %q", got)
	}
	if got := extractString(json.RawMessage(`{"transcript":"hello world"}`), transcriptTextKeys); got != "hello world" {
		t.Fatalf("snippet = %q", got)
	}
}
