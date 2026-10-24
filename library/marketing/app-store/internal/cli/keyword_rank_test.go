// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

func TestKeywordRankFindsTargetAndRecords(t *testing.T) {
	env := newS2bEnv(t, false, searchHandler(t))
	out, stderr, err := env.run("keyword-rank", "spotify", "--app", "324684580", "--json")
	if err != nil {
		t.Fatalf("keyword-rank: %v\n%s", err, stderr)
	}
	var got keywordRankResult
	decodeJSON(t, out, &got)
	if got.Rank == nil || *got.Rank != 1 {
		t.Fatalf("rank = %v, want 1", got.Rank)
	}
	if got.App == nil || got.App.AppID != "324684580" || !strings.HasPrefix(got.App.Title, "Spotify") {
		t.Fatalf("app = %+v", got.App)
	}
	if got.ResultCount != 5 || len(got.Top) != 5 || got.Signals.TopN != 5 || len(got.Signals.TopRatingCounts) != 5 {
		t.Fatalf("result = %+v", got)
	}
	// "Spotify: Music and Podcasts" and "stats.fm for Spotify Music App"
	// contain the term; YouTube, YouTube Music and Instagram do not.
	if got.Signals.TitleMatches != 2 || got.Signals.TitleMatchShare != 0.4 {
		t.Fatalf("signals = %+v", got.Signals)
	}
	if *got.Signals.TopRatingCounts[0] != 42719156 {
		t.Fatalf("top rating count = %d", *got.Signals.TopRatingCounts[0])
	}
	if !got.Recorded || got.Source != "itunes" {
		t.Fatalf("recorded %v source %s", got.Recorded, got.Source)
	}
	if !strings.Contains(env.fake.urls()[0], "limit=200") {
		t.Fatalf("keyword-rank must scan 200 results: %s", env.fake.urls()[0])
	}
	rows, _ := store.KeywordSeries(context.Background(), env.store(), store.KeywordFilter{Term: "spotify", AppID: "324684580"})
	if len(rows) != 1 || rows[0].Rank != 1 || rows[0].ResultCount != 5 || len(rows[0].TopIDs) != 5 {
		t.Fatalf("stored = %+v", rows)
	}
}

func TestKeywordRankMissingTargetIsNullNotGuessed(t *testing.T) {
	env := newS2bEnv(t, false, searchHandler(t))
	out, _, err := env.run("keyword-rank", "spotify", "--app", "999999999", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"rank": null`) {
		t.Fatalf("rank must be JSON null:\n%s", out)
	}
	var got keywordRankResult
	decodeJSON(t, out, &got)
	if got.Rank != nil || !strings.Contains(strings.Join(got.Notes, " "), "not in the first 5 results") {
		t.Fatalf("got %+v", got)
	}
	rows, _ := store.KeywordSeries(context.Background(), env.store(), store.KeywordFilter{Term: "spotify", AppID: "999999999"})
	if len(rows) != 1 || rows[0].Rank != 0 {
		t.Fatalf("stored = %+v (rank must be stored as not found)", rows)
	}
}

func TestKeywordRankWithoutTargetCapturesTheSERP(t *testing.T) {
	env := newS2bEnv(t, false, searchHandler(t))
	out, _, err := env.run("keyword-rank", "spotify", "--serp", "3", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"app": null`) || !strings.Contains(out, `"rank": null`) {
		t.Fatalf("app and rank must be null without --app:\n%s", out)
	}
	var got keywordRankResult
	decodeJSON(t, out, &got)
	if len(got.Top) != 3 || got.Signals.TopN != 3 || got.ResultCount != 5 {
		t.Fatalf("got %+v", got)
	}
	rows, _ := store.KeywordSeries(context.Background(), env.store(), store.KeywordFilter{Term: "spotify"})
	if len(rows) != 1 || rows[0].AppID != "" || len(rows[0].TopIDs) != 5 {
		t.Fatalf("stored = %+v", rows)
	}
}

func TestKeywordRankBundleIDMatchesWithoutLookup(t *testing.T) {
	env := newS2bEnv(t, false, searchHandler(t))
	out, _, err := env.run("keyword-rank", "spotify", "--app", "com.spotify.client", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got keywordRankResult
	decodeJSON(t, out, &got)
	if got.Rank == nil || *got.Rank != 1 || got.App.AppID != "324684580" {
		t.Fatalf("got rank %v app %+v", got.Rank, got.App)
	}
	if n := env.fake.count("/lookup"); n != 0 {
		t.Fatalf("lookups = %d, want 0", n)
	}
}

func TestKeywordRankRerunFromCacheIsNotRecordedTwice(t *testing.T) {
	env := newS2bEnv(t, true, searchHandler(t))
	for i := 0; i < 2; i++ {
		out, _, err := env.run("keyword-rank", "spotify", "--app", "324684580", "--json")
		if err != nil {
			t.Fatal(err)
		}
		var got keywordRankResult
		decodeJSON(t, out, &got)
		if got.Recorded != (i == 0) || got.FromCache != (i == 1) {
			t.Fatalf("run %d: recorded %v fromCache %v", i, got.Recorded, got.FromCache)
		}
	}
	if n := env.fake.count("/search"); n != 1 {
		t.Fatalf("search requests = %d, want 1", n)
	}
	st := env.store()
	rows, _ := store.KeywordSeries(context.Background(), st, store.KeywordFilter{Term: "spotify"})
	if len(rows) != 1 {
		t.Fatalf("stored captures = %d, want 1", len(rows))
	}
	// A different target on the same cached search is a new data point.
	out, _, err := env.run("keyword-rank", "spotify", "--app", "389801252", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got keywordRankResult
	decodeJSON(t, out, &got)
	if !got.Recorded || got.Rank == nil || *got.Rank != 4 {
		t.Fatalf("instagram capture = %+v", got)
	}
	rows, _ = store.KeywordSeries(context.Background(), st, store.KeywordFilter{Term: "spotify"})
	if len(rows) != 2 {
		t.Fatalf("stored captures = %d, want 2", len(rows))
	}
}

func TestKeywordRankRejectsBadInputs(t *testing.T) {
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse {
		t.Errorf("unexpected request %s", r.URL)
		return notFoundResp()
	})
	cases := [][]string{
		{"keyword-rank", ""},
		{"keyword-rank", "spotify", "--serp", "0"},
		{"keyword-rank", "spotify", "--serp", "201"},
		{"keyword-rank", "spotify", "--source", "bing"},
		{"keyword-rank", "spotify", "--app", "not an app"},
		{"keyword-rank", "spotify", "--platform", "tv"},
	}
	for _, args := range cases {
		if _, _, err := env.run(append(args, "--json")...); ExitCode(err) != 2 {
			t.Errorf("%v: exit %d (%v), want 2", args, ExitCode(err), err)
		}
	}
}

func TestKeywordRankThrottleExits7(t *testing.T) {
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse { return throttled() })
	if _, _, err := env.run("keyword-rank", "spotify", "--app", "324684580", "--json"); ExitCode(err) != 7 {
		t.Fatalf("exit %d (%v), want 7", ExitCode(err), err)
	}
	rows, _ := store.KeywordSeries(context.Background(), env.store(), store.KeywordFilter{Term: "spotify"})
	if len(rows) != 0 {
		t.Fatal("a throttled capture was recorded")
	}
}
