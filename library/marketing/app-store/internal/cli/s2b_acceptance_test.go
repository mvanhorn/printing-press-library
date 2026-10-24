// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// The slice-2b contract acceptance checks, run offline against the
// sanitized fixtures (real Apple captures of 2026-10-23, trimmed) with the
// real parsers, client, store and output pipeline. Lookup bodies are
// synthetic where no fixture covers the requested ids. Each subtest logs the
// raw values it asserts on, so `go test -v -run TestS2bContractAcceptance`
// is the raw acceptance record.

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

func TestS2bContractAcceptance(t *testing.T) {
	t.Run("top free genre 36 limit 200", func(t *testing.T) {
		env := newS2bEnv(t, false, usChartsHandler(t))
		out, _, err := env.run("top", "--chart", "top-free", "--genre", "36", "--limit", "200", "--json")
		if err != nil {
			t.Fatal(err)
		}
		var got topOutput
		decodeJSON(t, out, &got)
		nonZero, ranksOK := 0, true
		for i, r := range got.Results {
			ranksOK = ranksOK && r.Rank == i+1
			if r.RatingCount != nil && *r.RatingCount > 0 {
				nonZero++
			}
		}
		t.Logf("rows=%d ranks_1_to_200=%v nonzero_ratingCount=%d chart_page_requests=%d lookup_requests=%d snapshotRows=%d ratingsStored=%d",
			len(got.Results), ranksOK, nonZero, env.fake.count("/charts/"), env.fake.count("/lookup"), got.Meta.SnapshotRows, got.Meta.RatingsStored)
		if len(got.Results) != 200 || !ranksOK || nonZero <= 190 {
			t.Fatal("contract: exactly 200 rows, ranks 1..200, > 190 non-zero ratingCount")
		}
	})
	t.Run("top grossing limit 150", func(t *testing.T) {
		env := newS2bEnv(t, false, usChartsHandler(t))
		out, _, err := env.run("top", "--chart", "top-grossing", "--limit", "150", "--json")
		if err != nil {
			t.Fatal(err)
		}
		var got topOutput
		decodeJSON(t, out, &got)
		t.Logf("rows=%d surface=%s notes=%q", len(got.Results), got.Meta.Surface, got.Meta.Notes)
		if len(got.Results) != 100 || !strings.Contains(strings.Join(got.Meta.Notes, " "), "capped at 100") {
			t.Fatal("contract: 100 rows and a meta note saying so")
		}
	})
	t.Run("top pk 6014 top-paid prices contain Rs", func(t *testing.T) {
		charts := fixtureBytes(t, "charts_pk_iphone_6014.html")
		env := newS2bEnv(t, false, func(r *http.Request) fakeResponse {
			if strings.Contains(r.URL.Path, "/charts/") {
				return okBody(charts)
			}
			// Apps the Lookup leaves out keep the chart page's own price.
			return okBody([]byte(`{"resultCount":0,"results":[]}`))
		})
		out, _, err := env.run("top", "--country", "pk", "--genre", "6014", "--chart", "top-paid", "--json")
		if err != nil {
			t.Fatal(err)
		}
		var got topOutput
		decodeJSON(t, out, &got)
		prices := make([]string, 0, 3)
		for _, r := range got.Results[:3] {
			prices = append(prices, r.Price)
		}
		withRs := 0
		for _, r := range got.Results {
			if strings.Contains(r.Price, "Rs") {
				withRs++
			}
		}
		t.Logf("rows=%d with_Rs=%d (the 25 chart-page lockups; the fake Lookup returned no apps, so ranks 26-100 carry no price) first_prices=%q",
			len(got.Results), withRs, prices)
		if withRs != 25 {
			t.Fatal("contract: PK paid prices contain Rs")
		}
	})
	t.Run("reviews both sorts pages 2 and a second run", func(t *testing.T) {
		pages := map[string]map[int][]string{
			"mostrecent":  {1: idRange("r", 1, 50), 2: idRange("r", 51, 100)},
			"mosthelpful": {1: idRange("r", 41, 90), 2: idRange("h", 1, 50)},
		}
		env := newS2bEnv(t, true, reviewFeeds(t, pages))
		args := []string{"reviews", "6448311069", "--countries", "us", "--sort", "both", "--pages", "2", "--json"}
		out, _, err := env.run(args...)
		if err != nil {
			t.Fatal(err)
		}
		var first, second reviewsOutput
		decodeJSON(t, out, &first)
		badRating := 0
		for _, r := range first.Results {
			if r.Rating < 1 || r.Rating > 5 {
				badRating++
			}
		}
		out, _, err = env.run(args...)
		if err != nil {
			t.Fatal(err)
		}
		decodeJSON(t, out, &second)
		stored, _ := store.CountReviews(context.Background(), env.store(), store.ReviewFilter{AppID: "6448311069"})
		t.Logf("run1 unique=%d fetched=%d bad_ratings=%d stored=%+v | run2 stored=%+v | reviews table=%d", first.Meta.Unique, first.Meta.Fetched, badRating, first.Meta.Stored, second.Meta.Stored, stored)
		if first.Meta.Unique > 200 || badRating != 0 || second.Meta.Stored.Inserted != 0 || stored != first.Meta.Unique {
			t.Fatal("contract: <= 200 unique ids rated 1-5, second run stores 0 new rows")
		}
	})
	t.Run("suggest insta", func(t *testing.T) {
		env := newS2bEnv(t, false, hintsHandler(t))
		out, _, err := env.run("suggest", "insta", "--json")
		if err != nil {
			t.Fatal(err)
		}
		var got suggestResult
		decodeJSON(t, out, &got)
		t.Logf("suggestions=%q", got.Suggestions)
		for _, s := range got.Suggestions {
			if !strings.HasPrefix(s, "insta") {
				t.Fatalf("%q does not start with insta", s)
			}
		}
		if len(got.Suggestions) < 5 {
			t.Fatal("contract: >= 5 suggestions starting with insta")
		}
	})
	t.Run("suggest empty term exits 2", func(t *testing.T) {
		env := newS2bEnv(t, false, hintsHandler(t))
		_, _, err := env.run("suggest", "", "--json")
		t.Logf("exit=%d err=%v requests=%d", ExitCode(err), err, len(env.fake.urls()))
		if ExitCode(err) != 2 || len(env.fake.urls()) != 0 {
			t.Fatal("contract: suggest \"\" exits 2")
		}
	})
	t.Run("keyword-rank spotify app 324684580", func(t *testing.T) {
		env := newS2bEnv(t, false, searchHandler(t))
		out, _, err := env.run("keyword-rank", "spotify", "--app", "324684580", "--json")
		if err != nil {
			t.Fatal(err)
		}
		var got keywordRankResult
		decodeJSON(t, out, &got)
		rank := -1
		if got.Rank != nil {
			rank = *got.Rank
		}
		counts := make([]int64, 0, len(got.Signals.TopRatingCounts))
		for _, c := range got.Signals.TopRatingCounts {
			if c != nil {
				counts = append(counts, *c)
			}
		}
		t.Logf("rank=%d app=%+v resultCount=%d topRatingCounts=%v titleMatchShare=%.3f recorded=%v",
			rank, *got.App, got.ResultCount, counts, got.Signals.TitleMatchShare, got.Recorded)
		if rank != 1 {
			t.Fatal("contract: keyword-rank spotify --app 324684580 returns rank 1")
		}
	})
	t.Run("featured stories", func(t *testing.T) {
		env := newS2bEnv(t, false, todayHandler(t))
		out, _, err := env.run("featured", "--json")
		if err != nil {
			t.Fatal(err)
		}
		var got featuredOutput
		decodeJSON(t, out, &got)
		titles := make([]string, 0, 3)
		for _, s := range got.Results[:3] {
			titles = append(titles, s.Title)
		}
		rows, _ := store.FeaturedSince(context.Background(), env.store(), "us", time.Time{})
		t.Logf("stories=%d distinctApps=%d first=%q stored_rows=%d", len(got.Results), got.Meta.DistinctApps, titles, len(rows))
		if len(got.Results) < 5 {
			t.Fatal("contract: featured returns >= 5 stories")
		}
	})
	t.Run("reviews source api pages 1 is one opt-in request", func(t *testing.T) {
		api := fixtureBytes(t, "api_reviews_candycrush_us.json")
		env := newS2bEnv(t, false, func(r *http.Request) fakeResponse {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				return okBody(api)
			}
			return notFoundResp()
		})
		out, _, err := env.run("reviews", "6448311069", "--source", "api", "--pages", "1", "--json")
		if err != nil {
			t.Fatal(err)
		}
		var got reviewsOutput
		decodeJSON(t, out, &got)
		withResponse := 0
		for _, r := range got.Results {
			if r.DeveloperResponse != "" {
				withResponse++
			}
		}
		t.Logf("api_requests=%d urls=%v rows=%d with_developer_response=%d", env.fake.count("/api/"), env.fake.urls(), len(got.Results), withResponse)
		if env.fake.count("/api/") != 1 || len(got.Results) == 0 {
			t.Fatal("contract: one owner-approved /api request")
		}
	})
}
