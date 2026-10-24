// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package appstore

// Robots-disallowed apps.apple.com surfaces are opt-in for every request.
// Review defect: only WebSearch() and /api were locked, so Page() fetched the
// web search page without EnableOptInSurfaces and a redirect from an allowed
// page to a search URL was followed.

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestOptInURL(t *testing.T) {
	cases := []struct {
		url   string
		optIn bool
	}{
		{"https://apps.apple.com/us/iphone/search?term=spotify", true},
		{"https://apps.apple.com/us/search?term=x", true},
		{"https://apps.apple.com/search?term=x", true},
		{"https://apps.apple.com/us/iphone/search/?term=x", true},
		{"https://apps.apple.com/us/iphone/search", true},
		{"https://apps.apple.com/api/apps/v1/catalog/us/apps/1/reviews", true},
		{"https://apps.apple.com/api", true},
		{"https://apps.apple.com/v1/catalog/us/apps/1", true},
		{"https://apps.apple.com/WebObjects/MZStore.woa/wa/viewSoftware?id=1", true},
		{"https://apps.apple.com/includes/commerce/x", true},
		// Allowed pages.
		{"https://apps.apple.com/us/app/spotify-music-and-podcasts/id324684580", false},
		{"https://apps.apple.com/us/iphone/charts/36?chart=top-free", false},
		{"https://apps.apple.com/us/iphone/today", false},
		{"https://apps.apple.com/us/developer/spotify/id324684583", false},
		{"https://apps.apple.com/us/app/id324684580?see-all=reviews", false},
		{"https://apps.apple.com/us/app/research-app/id1", false},
		{"https://apps.apple.com/us/app/searchlight/id1", false},
		{"https://apps.apple.com/apiary/x", false},
		// Other hosts are classified by ClassifyURL, not here: the documented
		// Search API and the hints endpoint are not apps.apple.com pages.
		{"https://itunes.apple.com/search?term=spotify", false},
		{"https://search.itunes.apple.com/WebObjects/MZSearchHints.woa/wa/hints?term=a", false},
	}
	for _, tc := range cases {
		u, err := url.Parse(tc.url)
		if err != nil {
			t.Fatal(err)
		}
		if got := OptInURL(u); got != tc.optIn {
			t.Errorf("OptInURL(%s) = %v, want %v", tc.url, got, tc.optIn)
		}
	}
	if OptInURL(nil) {
		t.Fatal("nil URL must not be an opt-in surface")
	}
}

// Reviewer probe RV-P2: Page() on the web search page without opt-in.
func TestRawPageFetchesOfDisallowedPathsNeedOptIn(t *testing.T) {
	searchPage := pageHTML(t, map[string]any{"$kind": "SearchPageIntent", "term": "spotify"}, map[string]any{"shelves": []any{
		map[string]any{"items": []any{map[string]any{"$kind": "Lockup", "adamId": "324684580", "title": "Spotify"}}},
	}})
	c, fake, rec := newTestClient(t, servePageBody(searchPage), nil)
	ctx := context.Background()
	for _, u := range []string{
		"https://apps.apple.com/us/iphone/search?term=spotify",
		"https://apps.apple.com/us/search?term=spotify",
		"https://apps.apple.com/v1/catalog/us/apps/1",
		"https://apps.apple.com/WebObjects/MZStore.woa/wa/search?term=x",
		"https://apps.apple.com/includes/x",
	} {
		if _, err := c.Page(ctx, u); !errors.Is(err, ErrOptInRequired) {
			t.Fatalf("Page(%s) = %v, want ErrOptInRequired", u, err)
		}
		if _, err := c.PageOfKind(ctx, u, KindChartsPage); !errors.Is(err, ErrOptInRequired) {
			t.Fatalf("PageOfKind(%s) = %v, want ErrOptInRequired", u, err)
		}
	}
	if _, err := c.WebSearch(ctx, "us", "iphone", "spotify"); !errors.Is(err, ErrOptInRequired) {
		t.Fatalf("WebSearch without opt-in = %v", err)
	}
	if fake.count() != 0 || len(rec.all()) != 0 {
		t.Fatalf("disallowed surfaces sent %d requests before opt-in", fake.count())
	}
	// With the explicit opt-in the search page is fetched, parsed and paced
	// as a page.
	c.EnableOptInSurfaces()
	page, err := c.WebSearch(ctx, "us", "iphone", "spotify")
	if err != nil || strings.Join(page.RankedIDs(), ",") != "324684580" || page.FetchedAt.IsZero() {
		t.Fatalf("opted-in web search: %v %+v", err, page)
	}
	if req := fake.last(); req.Path != "/us/iphone/search" || req.Query.Get("term") != "spotify" {
		t.Fatalf("web search request = %+v", req)
	}
	if rows := rec.all(); len(rows) != 1 || rows[0].Outcome != OutcomeOK || rows[0].EntryCount != 1 {
		t.Fatalf("web search capture = %+v", rows)
	}
}

// Reviewer probe RV-P9: an allowed page that redirects to a disallowed one.
func TestRedirectToDisallowedSurfaceIsRefused(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request, n int) {
		switch r.URL.Path {
		case "/us/iphone/today":
			w.Header().Set("Location", "https://apps.apple.com/us/iphone/search?term=x")
			w.WriteHeader(http.StatusFound)
		case "/us/app/id324684580":
			w.Header().Set("Location", "/api/apps/v1/catalog/us/apps/324684580")
			w.WriteHeader(http.StatusMovedPermanently)
		default:
			_, _ = w.Write(pageHTML(t, map[string]any{"$kind": "SearchPageIntent"}, map[string]any{"shelves": []any{}}))
		}
	}
	c, fake, rec := newTestClient(t, handler, nil)
	ctx := context.Background()
	_, err := c.TodayPage(ctx, "us", "iphone")
	if !errors.Is(err, ErrOptInRequired) || !strings.Contains(err.Error(), "refusing redirect") {
		t.Fatalf("Today -> search redirect: err = %v, want a refused redirect", err)
	}
	if fake.count() != 1 || fake.last().Path != "/us/iphone/today" {
		t.Fatalf("requests = %d (last %s), want only the Today request", fake.count(), fake.last().Path)
	}
	rows := rec.all()
	if len(rows) != 1 || rows[0].Status != http.StatusFound || rows[0].Outcome != OutcomeFailed || !strings.Contains(rows[0].Error, "search?term=x") {
		t.Fatalf("capture rows = %+v, want one failed 302 row naming the search URL", rows)
	}
	if _, err := c.ProductPage(ctx, ProductPageQuery{AppID: "324684580"}); !errors.Is(err, ErrOptInRequired) {
		t.Fatalf("product -> /api redirect: err = %v", err)
	}
	if fake.count() != 2 {
		t.Fatalf("the /api redirect target was requested (requests %d)", fake.count())
	}
	// Opted in, the hop is followed (and the search model is then rejected
	// as the wrong kind for a Today page).
	c.EnableOptInSurfaces()
	_, err = c.TodayPage(ctx, "us", "iphone")
	var pk *PageKindError
	if !errors.As(err, &pk) || fake.count() != 4 || fake.last().Path != "/us/iphone/search" {
		t.Fatalf("opted-in redirect: err %v requests %d", err, fake.count())
	}
}
