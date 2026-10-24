// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

// Every live command stamps its snapshots with the observation time (when
// Apple served the response; the pinned client clock here, far from the wall
// clock), and a second run served from the local cache records nothing new.
func TestCachedSecondRunRecordsNoNewSnapshots(t *testing.T) {
	testenv.Isolate(t)
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	// Every command uses the default store under --home (the app-page
	// commands have no --db flag).
	restore, err := cliutil.SetHomeOverride(home)
	if err != nil {
		t.Fatal(err)
	}
	db := defaultDBPath(appStoreCLIName)
	restore()
	charts := fixtureBytes(t, "charts_us_iphone_36.html")
	today := fixtureBytes(t, "today_us_iphone.html")
	search := fixtureBytes(t, "search_spotify_us.json")
	api := fixtureBytes(t, "api_reviews_candycrush_us.json")
	product := fixtureBytes(t, "product_spotify_us.html")
	webSearch := pageModelHTML(
		map[string]any{"$kind": "SearchResultsPageIntent", "term": "spotify"},
		map[string]any{"shelves": []any{map[string]any{"items": []any{
			map[string]any{"$kind": "Lockup", "adamId": "324684580", "title": "Spotify"},
			map[string]any{"$kind": "Lockup", "adamId": "835599320", "title": "TikTok"},
		}}}},
	)
	fake := &fakeApple{handler: func(r *http.Request) fakeResponse {
		p := r.URL.Path
		switch {
		case r.URL.Host == "apps.apple.com" && strings.Contains(p, "/charts/"):
			return okBody(charts)
		case r.URL.Host == "apps.apple.com" && strings.HasSuffix(p, "/today"):
			return okBody(today)
		case r.URL.Host == "apps.apple.com" && strings.HasSuffix(p, "/search"):
			return okBody(webSearch)
		case strings.HasPrefix(p, "/api/apps/v1/catalog/us/apps/553834731/reviews"):
			return okBody(api)
		case r.URL.Host == "apps.apple.com" && strings.Contains(p, "/app/"):
			return okBody(product)
		case p == "/search":
			return okBody(search)
		case p == "/lookup":
			return synthLookup(r, nil)
		case strings.Contains(p, "/rss/customerreviews/"):
			return okBody(synthReviewFeed(idRange("9100", 1, 5), time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)))
		}
		t.Errorf("unexpected request %s", r.URL)
		return notFoundResp()
	}}
	t0 := time.Date(2026, 10, 22, 8, 30, 0, 0, time.UTC)
	var mu sync.Mutex
	now := t0
	useFakeAppStore(t, fakeAppStore{Transport: fake, CacheDir: filepath.Join(dir, "cache"), Cached: true, Now: func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}})
	commands := [][]string{
		{"top", "--limit", "10"},
		{"featured"},
		{"search-store", "spotify"},
		{"search-store", "spotify", "--source", "web"},
		{"keyword-rank", "spotify", "--app", "324684580"},
		{"keyword-rank", "spotify", "--source", "web", "--app", "324684580"},
		{"reviews", "553834731", "--source", "api", "--pages", "1"},
		{"reviews", "6448311069", "--pages", "1", "--sort", "recent"},
		{"app", "324684580"},
		{"ratings", "324684580"},
		{"prices", "324684580", "835599320"},
	}
	runAll := func() {
		t.Helper()
		for _, args := range commands {
			root := RootCmd()
			var out, errb bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&errb)
			root.SetArgs(append(append([]string{}, args...), "--json", "--home", home))
			if err := root.Execute(); err != nil {
				t.Fatalf("%v: %v\n%s", args, err, errb.String())
			}
		}
	}
	tables := []string{"app_snapshots", "ratings_snapshots", "chart_snapshots", "featured_snapshots", "keyword_ranks", "reviews", "privacy_snapshots", "iap_prices", "app_events", "version_entries"}
	counts := func() map[string]int {
		st, err := store.OpenWithContext(context.Background(), db)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = st.Close() }()
		out := map[string]int{}
		for _, table := range tables {
			var n int
			if err := st.DB().QueryRowContext(context.Background(), "SELECT COUNT(*) FROM "+table).Scan(&n); err != nil {
				t.Fatal(err)
			}
			out[table] = n
		}
		return out
	}

	runAll()
	first, requests := counts(), len(fake.urls())
	for _, table := range []string{"app_snapshots", "ratings_snapshots", "chart_snapshots", "featured_snapshots", "keyword_ranks", "reviews"} {
		if first[table] == 0 {
			t.Fatalf("first run wrote no %s rows", table)
		}
	}
	// Five minutes later every response comes from the local cache.
	mu.Lock()
	now = t0.Add(5 * time.Minute)
	mu.Unlock()
	runAll()
	if n := len(fake.urls()); n != requests {
		t.Fatalf("second run sent %d requests, want 0 (served from the cache)", n-requests)
	}
	if second := counts(); !equalCounts(first, second) {
		t.Fatalf("a cache-served second run added snapshots:\nfirst  %v\nsecond %v", first, second)
	}

	// Every stored time is the observation time t0, never the wall clock.
	st, err := store.OpenWithContext(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	want := t0.Format(time.RFC3339)
	distinct := func(q string) []string {
		t.Helper()
		rows, err := st.DB().QueryContext(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rows.Close() }()
		var got []string
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				t.Fatal(err)
			}
			got = append(got, s)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return got
	}
	for _, q := range []string{
		"SELECT DISTINCT captured_at FROM app_snapshots",
		"SELECT DISTINCT captured_at FROM ratings_snapshots",
		"SELECT DISTINCT captured_at FROM chart_snapshots",
		"SELECT DISTINCT captured_at FROM featured_snapshots",
		"SELECT DISTINCT captured_at FROM keyword_ranks",
		"SELECT DISTINCT first_seen_at FROM reviews",
		"SELECT DISTINCT last_seen_at FROM reviews",
	} {
		if got := distinct(q); len(got) != 1 || got[0] != want {
			t.Fatalf("%s = %v, want only the observation time %s", q, got, want)
		}
	}
}

func equalCounts(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
