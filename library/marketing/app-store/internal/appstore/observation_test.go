// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package appstore

// Results carry their observation time, and nothing is re-served from the
// cache for longer than 15 minutes. Review defect: Lookup and Search
// (max-age=86400) were cached for 24 h with no fetch time on the results, so
// a re-run recorded day-old counts as fresh observations stamped now.

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *testClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *testClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// Reviewer probe RV-P5: t0 1000 ratings, t0+23h Apple says 1900.
func TestLookupIsReobservedAfterTheCacheWindow(t *testing.T) {
	clock := &testClock{t: time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)}
	var mu sync.Mutex
	count := 1000
	c, fake, rec := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
		mu.Lock()
		ratings := count
		mu.Unlock()
		w.Header().Set("Cache-Control", "max-age=86400")
		_, _ = fmt.Fprintf(w, `{"resultCount":1,"results":[{"wrapperType":"software","trackId":324684580,"trackName":"Spotify","userRatingCount":%d}]}`, ratings)
	}, func(o *Options) { o.Now = clock.now })
	ctx := context.Background()
	lookup := func() *LookupResponse {
		t.Helper()
		resp, err := c.Lookup(ctx, LookupQuery{IDs: []string{"324684580"}})
		if err != nil || len(resp.Results) != 1 {
			t.Fatalf("lookup: %v", err)
		}
		return resp
	}
	t0 := clock.now()
	first := lookup()
	if r := first.Results[0]; r.UserRatingCount != 1000 || !r.FetchedAt.Equal(t0) || r.FromCache || !first.FetchedAt.Equal(t0) || first.FromCache {
		t.Fatalf("first observation = %+v (response fetchedAt %v fromCache %v)", r, first.FetchedAt, first.FromCache)
	}

	// Apple's count moves on; inside the 15-minute window the cached answer
	// is served, labelled with its original fetch time.
	mu.Lock()
	count = 1900
	mu.Unlock()
	clock.advance(10 * time.Minute)
	cached := lookup()
	if r := cached.Results[0]; r.UserRatingCount != 1000 || !r.FromCache || !r.FetchedAt.Equal(t0) || !cached.FromCache || !cached.FetchedAt.Equal(t0) {
		t.Fatalf("cached result = %+v fromCache %v fetchedAt %v, want the t0 observation labelled as cached", r, r.FromCache, r.FetchedAt)
	}
	if fake.count() != 1 {
		t.Fatalf("requests = %d, want 1", fake.count())
	}

	// Past the window (and certainly 23 h later) Apple is asked again.
	for _, step := range []time.Duration{6 * time.Minute, 23 * time.Hour} {
		clock.advance(step)
		fresh := lookup()
		if r := fresh.Results[0]; r.UserRatingCount != 1900 || r.FromCache || !r.FetchedAt.Equal(clock.now()) {
			t.Fatalf("after +%v: result = %+v, want a fresh 1900 observation stamped %v", step, r, clock.now())
		}
	}
	if fake.count() != 3 || len(rec.all()) != 3 {
		t.Fatalf("requests %d captures %d, want 3/3", fake.count(), len(rec.all()))
	}
}

// Every typed result says when Apple served it.
func TestResultsCarryObservationTime(t *testing.T) {
	clock := &testClock{t: time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)}
	c, _, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
		w.Header().Set("Cache-Control", "max-age=600")
		switch {
		case strings.HasPrefix(r.URL.Path, "/search"):
			_, _ = w.Write(readFixture(t, "search_spotify_us.json"))
		case strings.Contains(r.URL.Path, "customerreviews"):
			_, _ = w.Write(readFixture(t, "reviews_rss_spotify_us_p1_recent.json"))
		case strings.Contains(r.URL.Path, "/rss/"):
			_, _ = w.Write(readFixture(t, "rss_topgrossing_us_6014.json"))
		case strings.Contains(r.URL.Path, "/charts/"):
			_, _ = w.Write(readFixture(t, "charts_us_iphone_36.html"))
		default:
			_, _ = w.Write(readFixture(t, "hints_insta_us.plist"))
		}
	}, func(o *Options) { o.Now = clock.now })
	ctx := context.Background()
	t0 := clock.now()
	search, err := c.Search(ctx, SearchQuery{Term: "spotify", Limit: 5})
	if err != nil || !search.FetchedAt.Equal(t0) || search.FromCache || len(search.Results) != 5 || !search.Results[4].FetchedAt.Equal(t0) {
		t.Fatalf("search: %v fetchedAt %v", err, search.FetchedAt)
	}
	feed, err := c.ReviewsRSS(ctx, "us", "324684580", 1, "recent")
	if err != nil || !feed.FetchedAt.Equal(t0) || feed.FromCache {
		t.Fatalf("reviews: %v fetchedAt %v", err, feed.FetchedAt)
	}
	chart, err := c.ChartRSS(ctx, "us", "topgrossingapplications", "6014", 100)
	if err != nil || !chart.FetchedAt.Equal(t0) || chart.FromCache {
		t.Fatalf("chart rss: %v fetchedAt %v", err, chart.FetchedAt)
	}
	page, err := c.ChartsPage(ctx, ChartsPageQuery{GenreID: "36"})
	if err != nil || !page.FetchedAt.Equal(t0) || page.FromCache {
		t.Fatalf("charts page: %v fetchedAt %v", err, page.FetchedAt)
	}
	hints, err := c.Hints(ctx, "us", "insta")
	if err != nil || !hints.FetchedAt.Equal(t0) {
		t.Fatalf("hints: %v", err)
	}
	// Five minutes later every one of them comes from the cache with t0.
	clock.advance(5 * time.Minute)
	search, _ = c.Search(ctx, SearchQuery{Term: "spotify", Limit: 5})
	feed, _ = c.ReviewsRSS(ctx, "us", "324684580", 1, "recent")
	chart, _ = c.ChartRSS(ctx, "us", "topgrossingapplications", "6014", 100)
	page, _ = c.ChartsPage(ctx, ChartsPageQuery{GenreID: "36"})
	for name, got := range map[string]struct {
		at    time.Time
		cache bool
	}{
		"search": {search.FetchedAt, search.FromCache}, "reviews": {feed.FetchedAt, feed.FromCache},
		"chart rss": {chart.FetchedAt, chart.FromCache}, "charts page": {page.FetchedAt, page.FromCache},
		"search row": {search.Results[0].FetchedAt, search.Results[0].FromCache},
	} {
		if !got.cache || !got.at.Equal(t0) {
			t.Fatalf("%s from cache: fetchedAt %v fromCache %v, want %v/true", name, got.at, got.cache, t0)
		}
	}
}

// CurrentVersion: a Lookup CDN copy can be hours older than the product page
// fetched with it (live 2026-10-23: a single-id Lookup copy from 06:35:19Z
// said 9.1.92 while the page, fetched at 21:53:30Z, listed 9.1.94 released
// 17:59:53Z). The page's newest version entry wins only when its data is
// newer and it names another version.
func TestCurrentVersionTakesTheNewerSource(t *testing.T) {
	copyAt := time.Date(2026, 10, 23, 6, 35, 19, 0, time.UTC)
	fetch := time.Date(2026, 10, 23, 21, 53, 30, 0, time.UTC)
	lookup := LookupResult{Version: "9.1.92", CurrentVersionReleaseDate: "2026-10-19T13:57:56Z", ReleaseNotes: "Lookup notes", DataAsOf: copyAt}
	page := &ProductPage{DataAsOf: fetch, Versions: []VersionEntry{
		{Version: "9.1.94", ReleasedAt: "2026-10-23T17:59:53Z", Notes: "Page notes"},
		{Version: "9.1.92", ReleasedAt: "2026-10-19T13:57:56Z"},
	}}
	samePage := &ProductPage{DataAsOf: fetch, Versions: []VersionEntry{{Version: "9.1.92", ReleasedAt: "2026-10-19T13:57:56Z"}}}
	olderPage := &ProductPage{DataAsOf: copyAt.Add(-time.Hour), Versions: page.Versions}
	undated := LookupResult{Version: "9.1.92", CurrentVersionReleaseDate: "2026-10-19T13:57:56Z"}
	noVersion := LookupResult{DataAsOf: fetch}
	cases := []struct {
		name     string
		lk       LookupResult
		page     *ProductPage
		version  string
		released string
		fromPage bool
	}{
		{"stale Lookup copy, newer page names a newer version", lookup, page, "9.1.94", "2026-10-23T17:59:53Z", true},
		{"no page", lookup, nil, "9.1.92", "2026-10-19T13:57:56Z", false},
		{"page agrees", lookup, samePage, "9.1.92", "2026-10-19T13:57:56Z", false},
		{"page data older than the Lookup copy", lookup, olderPage, "9.1.92", "2026-10-19T13:57:56Z", false},
		{"Lookup data time unknown", undated, page, "9.1.92", "2026-10-19T13:57:56Z", false},
		{"page without versions", lookup, &ProductPage{DataAsOf: fetch}, "9.1.92", "2026-10-19T13:57:56Z", false},
		{"Lookup row without a version", noVersion, olderPage, "9.1.94", "2026-10-23T17:59:53Z", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CurrentVersion(tc.lk, tc.page)
			if got.Version != tc.version || got.ReleasedAt != tc.released || got.FromPage != tc.fromPage || got.LookupVersion != tc.lk.Version {
				t.Fatalf("CurrentVersion = %+v, want %s %s fromPage=%v", got, tc.version, tc.released, tc.fromPage)
			}
		})
	}
	if got := CurrentVersion(lookup, page); got.Notes != "Page notes" {
		t.Fatalf("notes = %q, want the page entry's notes with its version", got.Notes)
	}
}
