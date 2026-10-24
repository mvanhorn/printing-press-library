// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

type searchOutput struct {
	Meta    searchMeta  `json:"meta"`
	Results []searchRow `json:"results"`
}

// searchHandler serves the Spotify search fixture for "spotify" and an
// empty result list for any other term.
func searchHandler(t *testing.T) func(r *http.Request) fakeResponse {
	spotify := fixtureBytes(t, "search_spotify_us.json")
	return func(r *http.Request) fakeResponse {
		if r.URL.Host == "itunes.apple.com" && r.URL.Path == "/search" {
			if strings.EqualFold(r.URL.Query().Get("term"), "spotify") {
				return okBody(spotify)
			}
			return okBody([]byte(`{"resultCount":0,"results":[]}`))
		}
		t.Errorf("unexpected request %s", r.URL)
		return notFoundResp()
	}
}

func TestSearchStoreRanksRowsWithSourceLabel(t *testing.T) {
	env := newS2bEnv(t, false, searchHandler(t))
	out, stderr, err := env.run("search-store", "spotify", "--limit", "3", "--json")
	if err != nil {
		t.Fatalf("search-store: %v\n%s", err, stderr)
	}
	var got searchOutput
	decodeJSON(t, out, &got)
	if len(got.Results) != 3 || got.Results[0].AppID != "324684580" || !strings.HasPrefix(got.Results[0].Title, "Spotify") {
		t.Fatalf("results = %+v", got.Results)
	}
	for i, r := range got.Results {
		if r.Rank != i+1 || r.Source != "itunes" || r.RatingCount == nil {
			t.Fatalf("row %d = %+v", i, r)
		}
	}
	u := env.fake.urls()[0]
	if !strings.Contains(u, "limit=3") || !strings.Contains(u, "entity=software") || !strings.Contains(u, "country=us") {
		t.Fatalf("search URL = %s", u)
	}
	rows, err := store.KeywordSeries(context.Background(), env.store(), store.KeywordFilter{Term: "spotify"})
	if err != nil || len(rows) != 1 || rows[0].AppID != "" || rows[0].Source != "itunes" || len(rows[0].TopIDs) != 5 || rows[0].TopIDs[0] != "324684580" {
		t.Fatalf("keyword capture = %+v %v", rows, err)
	}
}

func TestSearchStorePriceFilterScansAndNeverFabricates(t *testing.T) {
	env := newS2bEnv(t, false, searchHandler(t))
	out, _, err := env.run("search-store", "spotify", "--price", "paid", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got searchOutput
	decodeJSON(t, out, &got)
	if len(got.Results) != 0 {
		t.Fatalf("every fixture app is free, got %+v", got.Results)
	}
	if got.Meta.Scanned != 5 || !strings.Contains(strings.Join(got.Meta.Notes, " "), "none is paid") {
		t.Fatalf("meta = %+v", got.Meta)
	}
	if !strings.Contains(env.fake.urls()[0], "limit=200") {
		t.Fatalf("a price filter must scan Apple's first 200: %s", env.fake.urls()[0])
	}
	out, _, err = env.run("search-store", "spotify", "--price", "free", "--limit", "2", "--json")
	if err != nil {
		t.Fatal(err)
	}
	decodeJSON(t, out, &got)
	if len(got.Results) != 2 || got.Results[1].Rank != 2 {
		t.Fatalf("free rows = %+v", got.Results)
	}
}

func TestSearchStoreMismatchingTermReturnsNothing(t *testing.T) {
	env := newS2bEnv(t, false, searchHandler(t))
	out, _, err := env.run("search-store", "zzqx nonsense term", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got searchOutput
	decodeJSON(t, out, &got)
	if len(got.Results) != 0 || !strings.Contains(out, `"results": []`) {
		t.Fatalf("results = %+v", got.Results)
	}
	if !strings.Contains(strings.Join(got.Meta.Notes, " "), "verified empty") {
		t.Fatalf("notes = %v", got.Meta.Notes)
	}
	for _, r := range got.Results {
		if r.AppID == "324684580" {
			t.Fatal("an unrelated app leaked into a non-matching search")
		}
	}
}

func TestSearchStoreRejectsBadInputs(t *testing.T) {
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse {
		t.Errorf("unexpected request %s", r.URL)
		return notFoundResp()
	})
	cases := [][]string{
		{"search-store", ""},
		{"search-store", "x", "--price", "cheap"},
		{"search-store", "x", "--source", "google"},
		{"search-store", "x", "--platform", "watch"},
		{"search-store", "x", "--limit", "0"},
		{"search-store", "x", "--limit", "201"},
	}
	for _, args := range cases {
		if _, _, err := env.run(append(args, "--json")...); ExitCode(err) != 2 {
			t.Errorf("%v: exit %d (%v), want 2", args, ExitCode(err), err)
		}
	}
}

func TestSearchStoreWebSourceIsOptIn(t *testing.T) {
	searchPage := pageModelHTML(
		map[string]any{"$kind": "SearchResultsPageIntent", "term": "spotify"},
		map[string]any{
			"shelves": []any{map[string]any{"items": []any{
				map[string]any{"$kind": "Lockup", "adamId": "324684580", "title": "Spotify"},
				map[string]any{"$kind": "Lockup", "adamId": "835599320", "title": "TikTok"},
			}}},
			"nextPage": map[string]any{"remainingContent": []any{map[string]any{"id": "389801252"}}},
		},
	)
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse {
		switch {
		case r.URL.Host == "apps.apple.com" && strings.HasSuffix(r.URL.Path, "/search"):
			return okBody(searchPage)
		case r.URL.Path == "/lookup":
			return synthLookup(r, nil)
		case r.URL.Path == "/search":
			return okBody([]byte(`{"resultCount":0,"results":[]}`))
		}
		t.Errorf("unexpected request %s", r.URL)
		return notFoundResp()
	})
	// Default: the robots-disallowed web page is never touched.
	if _, _, err := env.run("search-store", "spotify", "--json"); err != nil {
		t.Fatal(err)
	}
	if n := env.fake.count("apps.apple.com"); n != 0 {
		t.Fatalf("default search touched apps.apple.com %d times", n)
	}
	out, stderr, err := env.run("search-store", "spotify", "--source", "web", "--json")
	if err != nil {
		t.Fatalf("web search: %v\n%s", err, stderr)
	}
	var got searchOutput
	decodeJSON(t, out, &got)
	if len(got.Results) != 3 || got.Results[0].AppID != "324684580" || got.Results[2].AppID != "389801252" {
		t.Fatalf("web rows = %+v", got.Results)
	}
	for _, r := range got.Results {
		if r.Source != "web" {
			t.Fatalf("row source = %q", r.Source)
		}
	}
	if n := env.fake.count("apps.apple.com/us/iphone/search?term=spotify"); n != 1 {
		t.Fatalf("web search page requests = %d, want 1", n)
	}
}

func TestSearchStoreThrottleExits7(t *testing.T) {
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse { return throttled() })
	if _, _, err := env.run("search-store", "spotify", "--json"); ExitCode(err) != 7 {
		t.Fatalf("exit %d (%v), want 7", ExitCode(err), err)
	}
	rows, _ := store.KeywordSeries(context.Background(), env.store(), store.KeywordFilter{Term: "spotify"})
	if len(rows) != 0 {
		t.Fatal("a throttled search was recorded")
	}
}

// A 404 from the Search API is an API failure (exit 5), not "not found"
// (exit 3): the research error catalogue records a transient /search 404.
func TestSearch404IsAnAPIFailureNotNotFound(t *testing.T) {
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse { return notFoundResp() })
	for _, args := range [][]string{{"search-store", "spotify"}, {"keyword-rank", "spotify"}} {
		_, _, err := env.run(append(args, "--json")...)
		if ExitCode(err) != 5 {
			t.Fatalf("%v with a /search 404: exit %d (%v), want 5", args, ExitCode(err), err)
		}
	}
}
