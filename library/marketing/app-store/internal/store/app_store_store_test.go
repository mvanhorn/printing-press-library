// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
)

func openTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app-store.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st, path
}

func countRows(t *testing.T, st *Store, table, where string, args ...any) int {
	t.Helper()
	q := "SELECT COUNT(*) FROM " + table
	if where != "" {
		q += " WHERE " + where
	}
	var n int
	if err := st.DB().QueryRowContext(context.Background(), q, args...).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func fixturePage(t *testing.T, name string) appstore.ProductPage {
	t.Helper()
	html, err := os.ReadFile(filepath.Join("..", "appstore", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	m, err := appstore.ExtractPageModel(html)
	if err != nil {
		t.Fatal(err)
	}
	p, err := appstore.ParseProductPage(m)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func fixtureReviews(t *testing.T, name string) []appstore.Review {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "appstore", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	reviews, err := appstore.ParseReviewsFeed(body)
	if err != nil {
		t.Fatal(err)
	}
	return reviews
}

var appStoreTables = []string{
	"captures", "app_snapshots", "chart_snapshots", "reviews", "reviews_fts", "ratings_snapshots",
	"version_entries", "privacy_snapshots", "iap_prices", "app_events", "featured_snapshots", "keyword_ranks",
}

func TestAppStoreMigrationsIdempotent(t *testing.T) {
	st, path := openTestStore(t)
	ctx := context.Background()
	for _, table := range appStoreTables {
		countRows(t, st, table, "")
	}
	if err := RecordKeywordRank(ctx, st, KeywordRank{Storefront: "us", Platform: "iphone", Term: "Spotify", Source: SourceSearchAPI, AppID: "324684580", Rank: 1, ResultCount: 5, TopIDs: []string{"324684580"}}); err != nil {
		t.Fatal(err)
	}
	v1, err := st.SchemaVersion()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		st2, err := Open(path)
		if err != nil {
			t.Fatalf("reopen %d: %v", i, err)
		}
		v2, _ := st2.SchemaVersion()
		if v2 != v1 {
			t.Fatalf("schema version changed on reopen: %d -> %d", v1, v2)
		}
		if n := countRows(t, st2, "keyword_ranks", ""); n != 1 {
			t.Fatalf("reopen %d lost data: %d keyword rows", i, n)
		}
		// Running the extra migrations again on a live connection is a no-op.
		conn, err := st2.DB().Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := st2.migrateExtras(ctx, conn); err != nil {
			t.Fatalf("second migrateExtras: %v", err)
		}
		_ = conn.Close()
		_ = st2.Close()
	}
}

func TestRecordProductPage(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()
	page := fixturePage(t, "product_candycrush_us.html")
	t1 := time.Date(2026, 10, 23, 10, 0, 0, 0, time.UTC)
	page.FetchedAt = t1
	lk := appstore.LookupResult{
		WrapperType: "software", TrackID: 553834731, TrackName: "Candy Crush Saga", BundleID: "com.midasplayer.apps.candycrushsaga",
		Version: page.Versions[0].Version, Price: 0, Currency: "USD", FormattedPrice: "Free", AverageUserRating: 4.7,
		UserRatingCount: 4030718, ArtistID: 526656015, ArtistName: "King", PrimaryGenreID: 6014, PrimaryGenreName: "Games",
		TrackViewURL: "https://apps.apple.com/us/app/candy-crush-saga/id553834731?uo=4",
	}
	if err := RecordProductPage(ctx, st, "us", page, lk); err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		table string
		want  int
	}{
		{"app_snapshots", 1}, {"version_entries", 25}, {"privacy_snapshots", 1}, {"iap_prices", 10},
		{"app_events", 1}, {"ratings_snapshots", 1},
	}
	for _, c := range checks {
		if n := countRows(t, st, c.table, "app_id = ?", "553834731"); n != c.want {
			t.Fatalf("%s rows = %d, want %d", c.table, n, c.want)
		}
	}
	snaps, err := AppSnapshots(ctx, st, "553834731", "us", time.Time{})
	if err != nil || len(snaps) != 1 {
		t.Fatalf("snapshots: %v %d", err, len(snaps))
	}
	s := snaps[0]
	if s.Source != SourcePageLookup || s.Subtitle == "" || !s.HasIAP || s.IAPHash == "" || s.PrivacyHash == "" || s.EventsHash == "" || s.ContentHash == "" {
		t.Fatalf("snapshot = %+v", s)
	}
	if !s.CapturedAt.Equal(t1) || s.ChartPosition != "top-free/7003/24" || len(s.Fields.InAppPurchases) != 10 {
		t.Fatalf("snapshot fields = %v %s %d", s.CapturedAt, s.ChartPosition, len(s.Fields.InAppPurchases))
	}
	ratings, err := RatingsSeries(ctx, st, "553834731", "us", SourcePage)
	if err != nil || len(ratings) != 1 || len(ratings[0].Histogram) != 5 || ratings[0].RatingCount != page.TotalRatings {
		t.Fatalf("ratings = %v %+v", err, ratings)
	}

	// Same page an hour later: snapshots and ratings append, the rest dedupes.
	page.FetchedAt = t1.Add(time.Hour)
	if err := RecordProductPage(ctx, st, "us", page, lk); err != nil {
		t.Fatal(err)
	}
	again := []struct {
		table string
		want  int
	}{
		{"app_snapshots", 2}, {"version_entries", 25}, {"privacy_snapshots", 1}, {"iap_prices", 10},
		{"app_events", 1}, {"ratings_snapshots", 2},
	}
	for _, c := range again {
		if n := countRows(t, st, c.table, "app_id = ?", "553834731"); n != c.want {
			t.Fatalf("after repeat: %s rows = %d, want %d", c.table, n, c.want)
		}
	}
	snaps, _ = AppSnapshots(ctx, st, "553834731", "", time.Time{})
	if snaps[0].ContentHash != snaps[1].ContentHash {
		t.Fatal("identical listings produced different content hashes")
	}
	events, err := EventsHistory(ctx, st, "553834731", "us")
	if err != nil || len(events) != 1 || !events[0].FirstSeenAt.Equal(t1) || !events[0].LastSeenAt.Equal(t1.Add(time.Hour)) {
		t.Fatalf("events = %v %+v", err, events)
	}

	// A price change writes a new IAP list and changes the content hash.
	page.FetchedAt = t1.Add(2 * time.Hour)
	page.InAppPurchases = append([]appstore.IAPItem(nil), page.InAppPurchases...)
	page.InAppPurchases[0].Price = "$2.49"
	if err := RecordProductPage(ctx, st, "us", page, lk); err != nil {
		t.Fatal(err)
	}
	hist, err := IAPHistory(ctx, st, "553834731", "us")
	if err != nil || len(hist) != 2 || hist[1].Items[0].Price != "$2.49" || hist[0].Items[0].Price != "$1.99" {
		t.Fatalf("IAP history = %v %+v", err, hist)
	}
	snaps, _ = AppSnapshots(ctx, st, "553834731", "us", time.Time{})
	if len(snaps) != 3 || snaps[2].IAPHash == snaps[1].IAPHash || snaps[2].ContentHash == snaps[1].ContentHash {
		t.Fatal("IAP price change not reflected in the snapshot hashes")
	}
	for i := 1; i < len(snaps); i++ {
		if !snaps[i-1].CapturedAt.Before(snaps[i].CapturedAt) {
			t.Fatal("AppSnapshots not oldest first")
		}
	}
	privacy, err := PrivacyHistory(ctx, st, "553834731", "")
	if err != nil || len(privacy) != 1 || len(privacy[0].Privacy.Types) == 0 {
		t.Fatalf("privacy history = %v %+v", err, privacy)
	}
	versions, err := VersionEntries(ctx, st, "553834731")
	if err != nil || len(versions) != 25 || versions[0].ReleasedAt < versions[24].ReleasedAt {
		t.Fatalf("versions = %v (%d)", err, len(versions))
	}

	// A page-only record (no Lookup) writes no app snapshot.
	spotify := fixturePage(t, "product_spotify_us.html")
	if err := RecordProductPage(ctx, st, "us", spotify, appstore.LookupResult{}); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, st, "app_snapshots", "app_id = ?", "324684580"); n != 0 {
		t.Fatalf("page-only record wrote %d app snapshots", n)
	}
	if n := countRows(t, st, "version_entries", "app_id = ?", "324684580"); n != 25 {
		t.Fatalf("page-only versions = %d", n)
	}
	tracked, err := TrackedApps(ctx, st, time.Time{})
	if err != nil || len(tracked) != 1 || tracked[0].Snapshots != 3 || tracked[0].Title != "Candy Crush Saga" {
		t.Fatalf("tracked = %v %+v", err, tracked)
	}
	found, err := SearchApps(ctx, st, "candy crush", "us", 10)
	if err != nil || len(found) != 1 || found[0].AppID != "553834731" || !found[0].CapturedAt.Equal(t1.Add(2*time.Hour)) {
		t.Fatalf("search apps = %v %+v", err, found)
	}
}

func TestRecordLookupSnapshot(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()
	lk := appstore.LookupResult{TrackID: 324684580, TrackName: "Spotify: Music and Podcasts", Version: "9.1.92", Price: 0, Currency: "USD", UserRatingCount: 42703709}
	if err := RecordLookupSnapshot(ctx, st, "us", lk, time.Time{}); err != nil {
		t.Fatal(err)
	}
	snaps, err := AppSnapshots(ctx, st, "324684580", "us", time.Time{})
	if err != nil || len(snaps) != 1 || snaps[0].Source != SourceLookup || snaps[0].RatingCount != 42703709 {
		t.Fatalf("lookup snapshot = %v %+v", err, snaps)
	}
	if err := RecordLookupSnapshot(ctx, st, "us", appstore.LookupResult{}, time.Time{}); err == nil {
		t.Fatal("a lookup result without trackId must be rejected")
	}
}

func TestUpsertReviews(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()
	reviews := fixtureReviews(t, "reviews_rss_spotify_us_p1_recent.json")
	reviews[6].Body = "the checkout flow crashes every time"
	t1 := time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)
	batch := ReviewUpsert{Storefront: "us", AppID: "324684580", Sort: appstore.SortMostRecent, Source: SourceRSS, SeenAt: t1, Reviews: reviews}
	stats, err := UpsertReviews(ctx, st, batch)
	if err != nil || stats.Inserted != 50 || stats.Updated != 0 {
		t.Fatalf("first upsert = %v %+v", err, stats)
	}
	// The same reviews seen through the other sort: no new rows, sorts merged.
	batch.Sort, batch.SeenAt = appstore.SortMostHelpful, t1.Add(time.Hour)
	stats, err = UpsertReviews(ctx, st, batch)
	if err != nil || stats.Inserted != 0 || stats.Updated != 50 {
		t.Fatalf("second upsert = %v %+v", err, stats)
	}
	// Seen again through the first sort: nothing changes.
	batch.Sort = appstore.SortMostRecent
	stats, err = UpsertReviews(ctx, st, batch)
	if err != nil || stats.Inserted != 0 || stats.Unchanged != 50 {
		t.Fatalf("third upsert = %v %+v", err, stats)
	}
	if n, _ := CountReviews(ctx, st, ReviewFilter{AppID: "324684580"}); n != 50 {
		t.Fatalf("stored reviews = %d, want 50 (no duplicates)", n)
	}
	stored, err := Reviews(ctx, st, ReviewFilter{AppID: "324684580", Storefronts: []string{"us"}})
	if err != nil || len(stored) != 50 {
		t.Fatalf("reviews = %v %d", err, len(stored))
	}
	for i, r := range stored {
		if strings.Join(r.SortsSeen, ",") != "mosthelpful,mostrecent" || !r.FirstSeenAt.Equal(t1) || !r.LastSeenAt.Equal(t1.Add(time.Hour)) {
			t.Fatalf("review %d = %+v", i, r)
		}
		if i > 0 && stored[i-1].ReviewedAt < r.ReviewedAt {
			t.Fatal("Reviews not newest first")
		}
	}
	hits, err := SearchReviews(ctx, st, "crashes", ReviewFilter{AppID: "324684580"})
	if err != nil || len(hits) != 1 || hits[0].ReviewID != reviews[6].ID {
		t.Fatalf("fts search = %v %+v", err, hits)
	}
	// Editing the body re-indexes it: the old text no longer matches.
	reviews[6].Body = "works fine after the update"
	batch.Reviews = reviews[6:7]
	if _, err := UpsertReviews(ctx, st, batch); err != nil {
		t.Fatal(err)
	}
	if hits, _ := SearchReviews(ctx, st, "crashes", ReviewFilter{}); len(hits) != 0 {
		t.Fatalf("stale FTS row still matches: %+v", hits)
	}
	if hits, _ := SearchReviews(ctx, st, "update", ReviewFilter{AppID: "324684580"}); len(hits) != 1 {
		t.Fatalf("updated text not indexed: %d hits", len(hits))
	}
	if n := countRows(t, st, "reviews_fts", ""); n != 50 {
		t.Fatalf("reviews_fts rows = %d, want 50", n)
	}
	// Short tokens fall back to LIKE.
	if hits, _ := SearchReviews(ctx, st, "Sample body 7", ReviewFilter{AppID: "324684580"}); len(hits) == 0 {
		t.Fatal("LIKE fallback found nothing")
	}
	low, _ := Reviews(ctx, st, ReviewFilter{AppID: "324684580", MaxRating: 2})
	for _, r := range low {
		if r.Rating > 2 {
			t.Fatalf("MaxRating filter leaked rating %d", r.Rating)
		}
	}
	// The same review id in another storefront is a separate row.
	other := ReviewUpsert{Storefront: "gb", AppID: "324684580", Sort: appstore.SortMostRecent, Source: SourceRSS, Reviews: reviews[:1]}
	if stats, err := UpsertReviews(ctx, st, other); err != nil || stats.Inserted != 1 {
		t.Fatalf("gb upsert = %v %+v", err, stats)
	}
}

// Review reads bind Limit as the last query argument: Reviews and the
// SearchReviews FTS path return the first Limit rows of the unlimited answer,
// and the filter arguments before it still bind to their own placeholders.
func TestReviewQueriesBindTheLimit(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()
	batch := ReviewUpsert{Storefront: "us", AppID: "324684580", Sort: appstore.SortMostRecent, Source: SourceRSS,
		SeenAt: time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC), Reviews: fixtureReviews(t, "reviews_rss_spotify_us_p1_recent.json")}
	if _, err := UpsertReviews(ctx, st, batch); err != nil {
		t.Fatal(err)
	}
	filter := ReviewFilter{AppID: "324684580", Storefronts: []string{"us"}}
	all, err := Reviews(ctx, st, filter)
	if err != nil || len(all) != 50 {
		t.Fatalf("reviews = %v %d, want 50", err, len(all))
	}
	filter.Limit = 3
	limited, err := Reviews(ctx, st, filter)
	if err != nil || len(limited) != 3 {
		t.Fatalf("limited reviews = %v %d, want 3", err, len(limited))
	}
	for i, r := range limited {
		if r.ReviewID != all[i].ReviewID {
			t.Fatalf("limited review %d = %s, want %s (newest first)", i, r.ReviewID, all[i].ReviewID)
		}
	}
	allHits, err := SearchReviews(ctx, st, "Sample", ReviewFilter{AppID: "324684580", Storefronts: []string{"us"}})
	if err != nil || len(allHits) != 50 {
		t.Fatalf("fts search = %v %d hits, want 50", err, len(allHits))
	}
	hits, err := SearchReviews(ctx, st, "Sample", ReviewFilter{AppID: "324684580", Storefronts: []string{"us"}, Limit: 4})
	if err != nil || len(hits) != 4 {
		t.Fatalf("limited fts search = %v %d hits, want 4", err, len(hits))
	}
	for i, r := range hits {
		if r.ReviewID != allHits[i].ReviewID {
			t.Fatalf("limited fts hit %d = %s, want %s", i, r.ReviewID, allHits[i].ReviewID)
		}
	}
	if none, err := SearchReviews(ctx, st, "Sample", ReviewFilter{AppID: "1", Limit: 4}); err != nil || len(none) != 0 {
		t.Fatalf("fts search for another app = %v %d hits, want 0", err, len(none))
	}
}

func TestChartAccessors(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()
	key := ChartKey{Storefront: "us", Platform: "iphone", GenreID: "6014", Chart: "top-free"}
	base := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	snaps := [][]string{
		{"a", "b", "c"},
		{"b", "a", "d"},
		{"d", "b", "e"},
	}
	for i, ids := range snaps {
		rows := make([]ChartRow, 0, len(ids))
		for r, id := range ids {
			rows = append(rows, ChartRow{Rank: r + 1, AppID: id, Title: "App " + id})
		}
		snap := ChartSnapshot{CapturedAt: base.Add(time.Duration(i) * 24 * time.Hour), Storefront: key.Storefront, Platform: key.Platform, GenreID: key.GenreID, Chart: key.Chart, Source: SourceWebChart, RowLimit: 200, Rows: rows}
		if err := RecordChart(ctx, st, snap); err != nil {
			t.Fatal(err)
		}
		if i == 2 {
			// A repeat write in the same second replaces, never doubles.
			if err := RecordChart(ctx, st, snap); err != nil {
				t.Fatal(err)
			}
		}
	}
	if n := countRows(t, st, "chart_snapshots", ""); n != 9 {
		t.Fatalf("chart rows = %d, want 9", n)
	}
	series, err := ChartSeries(ctx, st, key, "b")
	if err != nil || len(series) != 3 || series[0].Rank != 2 || series[1].Rank != 1 || series[2].Rank != 2 {
		t.Fatalf("series = %v %+v", err, series)
	}
	for i := 1; i < len(series); i++ {
		if !series[i-1].CapturedAt.Before(series[i].CapturedAt) {
			t.Fatal("ChartSeries not oldest first")
		}
	}
	prev, latest, err := LatestTwoCharts(ctx, st, key)
	if err != nil || prev == nil || latest == nil {
		t.Fatalf("latest two = %v %v %v", err, prev, latest)
	}
	if !latest.CapturedAt.Equal(base.Add(48*time.Hour)) || !prev.CapturedAt.Equal(base.Add(24*time.Hour)) || latest.Rows[0].AppID != "d" || len(latest.Rows) != 3 {
		t.Fatalf("latest %+v prev %+v", latest, prev)
	}
	// An RSS capture (100 rows) is not comparable with the web captures.
	rss := ChartSnapshot{CapturedAt: base.Add(72 * time.Hour), Storefront: "us", Platform: "iphone", GenreID: "6014", Chart: "top-free", Source: SourceRSS, RowLimit: 100, Rows: []ChartRow{{Rank: 1, AppID: "z"}}}
	if err := RecordChart(ctx, st, rss); err != nil {
		t.Fatal(err)
	}
	prev, latest, err = LatestTwoCharts(ctx, st, key)
	if err != nil || latest == nil || latest.Source != SourceRSS || prev != nil {
		t.Fatalf("mixed sources: latest %+v prev %+v err %v", latest, prev, err)
	}
	webKey := key
	webKey.Source = SourceWebChart
	prev, latest, _ = LatestTwoCharts(ctx, st, webKey)
	if latest == nil || prev == nil || latest.Source != SourceWebChart {
		t.Fatal("source-pinned key should still find the two web captures")
	}
	caps, err := ChartCaptures(ctx, st, key)
	if err != nil || len(caps) != 4 || caps[3].Source != SourceRSS || caps[0].Rows != 3 {
		t.Fatalf("captures = %v %+v", err, caps)
	}
	one := ChartKey{Storefront: "gb", Platform: "iphone", GenreID: "36", Chart: "top-paid"}
	if p, l, err := LatestTwoCharts(ctx, st, one); err != nil || p != nil || l != nil {
		t.Fatal("never-captured key must return nil, nil")
	}
	rows, err := ChartRows(ctx, st, ChartQuery{Storefronts: []string{"us"}, AppID: "d", Source: SourceWebChart})
	if err != nil || len(rows) != 2 || rows[0].Rank != 3 || rows[1].Rank != 1 {
		t.Fatalf("chart rows = %v %+v", err, rows)
	}
}

func chartSnap(at time.Time, genre, band string, ids ...string) ChartSnapshot {
	rows := make([]ChartRow, 0, len(ids))
	for i, id := range ids {
		rows = append(rows, ChartRow{Rank: i + 1, AppID: id})
	}
	return ChartSnapshot{CapturedAt: at, Storefront: "us", Platform: "iphone", GenreID: genre, AgeBand: band, Chart: "top-free", Source: SourceWebChart, RowLimit: 200, Rows: rows}
}

func snapIDs(s *ChartSnapshot) string {
	if s == nil {
		return "<nil>"
	}
	ids := make([]string, 0, len(s.Rows))
	for _, r := range s.Rows {
		ids = append(ids, r.AppID)
	}
	return strings.Join(ids, ",")
}

// Review defect (RV-S2): a Kids chart (genre 36 + ageBandId) and the overall
// genre-36 chart shared one series, so movers paired them and invented two
// exits and two entries.
func TestKidsChartIsItsOwnSeries(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()
	base := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	overall := ChartKey{Storefront: "us", Platform: "iphone", GenreID: "36", Chart: "top-free", Source: SourceWebChart}
	kids := overall
	kids.AgeBand = "10001"

	if err := RecordChart(ctx, st, chartSnap(base, "36", "", "407558537", "6448311069")); err != nil {
		t.Fatal(err)
	}
	if err := RecordChart(ctx, st, chartSnap(base.Add(time.Hour), "36", "10001", "111", "222")); err != nil {
		t.Fatal(err)
	}
	prev, latest, err := LatestTwoCharts(ctx, st, overall)
	if err != nil || prev != nil || snapIDs(latest) != "407558537,6448311069" || latest.AgeBand != "" {
		t.Fatalf("overall after one overall + one Kids capture: prev=%s latest=%s err=%v (the Kids chart must not pair with it)", snapIDs(prev), snapIDs(latest), err)
	}
	prev, latest, err = LatestTwoCharts(ctx, st, kids)
	if err != nil || prev != nil || snapIDs(latest) != "111,222" || latest.AgeBand != "10001" || latest.GenreID != "36" {
		t.Fatalf("kids series: prev=%s latest=%+v err=%v", snapIDs(prev), latest, err)
	}
	if err := RecordChart(ctx, st, chartSnap(base.Add(2*time.Hour), "36", "", "6448311069", "407558537")); err != nil {
		t.Fatal(err)
	}
	prev, latest, _ = LatestTwoCharts(ctx, st, overall)
	if snapIDs(prev) != "407558537,6448311069" || snapIDs(latest) != "6448311069,407558537" {
		t.Fatalf("overall pair = %s -> %s", snapIDs(prev), snapIDs(latest))
	}
	// A key without a source still never reaches the Kids series.
	noSource := overall
	noSource.Source = ""
	if caps, _ := ChartCaptures(ctx, st, noSource); len(caps) != 2 {
		t.Fatalf("overall captures = %d, want 2", len(caps))
	}
	if caps, _ := ChartCaptures(ctx, st, kids); len(caps) != 1 {
		t.Fatalf("kids captures = %d, want 1", len(caps))
	}
	if s, _ := ChartSeries(ctx, st, overall, "111"); len(s) != 0 {
		t.Fatalf("a Kids-only app has an overall rank series: %+v", s)
	}
	if s, _ := ChartSeries(ctx, st, kids, "111"); len(s) != 1 || s[0].Rank != 1 {
		t.Fatalf("kids series for 111 = %+v", s)
	}

	// Overall and Kids written in the same second are both kept.
	same := base.Add(3 * time.Hour)
	if err := RecordChart(ctx, st, chartSnap(same, "36", "", "1", "2", "3")); err != nil {
		t.Fatal(err)
	}
	if err := RecordChart(ctx, st, chartSnap(same, "36", "10001", "7", "8")); err != nil {
		t.Fatal(err)
	}
	at := appstore.FormatTime(same)
	if n := countRows(t, st, "chart_snapshots", "captured_at = ? AND age_band = ''", at); n != 3 {
		t.Fatalf("same-second overall rows = %d, want 3 (the Kids write must not delete them)", n)
	}
	if n := countRows(t, st, "chart_snapshots", "captured_at = ? AND age_band = '10001'", at); n != 2 {
		t.Fatalf("same-second Kids rows = %d, want 2", n)
	}

	// ChartRows: "" = overall only, a band = that Kids chart, AnyAgeBand = all.
	if rows, _ := ChartRows(ctx, st, ChartQuery{AppID: "111"}); len(rows) != 0 {
		t.Fatalf("default ChartRows leaked Kids rows: %+v", rows)
	}
	if rows, _ := ChartRows(ctx, st, ChartQuery{AppID: "111", AgeBand: "10001"}); len(rows) != 1 || rows[0].AgeBand != "10001" {
		t.Fatalf("Kids ChartRows = %+v", rows)
	}
	all, _ := ChartRows(ctx, st, ChartQuery{GenreID: "36", AgeBand: AnyAgeBand})
	if len(all) != 2+2+2+3+2 {
		t.Fatalf("AnyAgeBand rows = %d, want 11", len(all))
	}

	// The interim composite genre key from slice 2b is read as 36 + band.
	if err := RecordChart(ctx, st, chartSnap(base, "36-kids-10002", "", "55")); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, st, "chart_snapshots", "genre_id = '36' AND age_band = '10002'"); n != 1 {
		t.Fatalf("composite key stored as %d normalized rows, want 1", n)
	}
	for _, k := range []ChartKey{
		{Storefront: "us", Platform: "iphone", GenreID: "36-kids-10002", Chart: "top-free"},
		{Storefront: "us", Platform: "iphone", GenreID: "36", AgeBand: "10002", Chart: "top-free"},
	} {
		if caps, _ := ChartCaptures(ctx, st, k); len(caps) != 1 {
			t.Fatalf("captures for %+v = %d, want 1", k, len(caps))
		}
	}
	if rows, _ := ChartRows(ctx, st, ChartQuery{GenreID: "36-kids-10002", AgeBand: AnyAgeBand}); len(rows) != 1 || rows[0].AgeBand != "10002" {
		t.Fatalf("composite-key ChartRows = %+v", rows)
	}
	if k := chartSnap(base, "36-kids-10002", "").Key(); k.GenreID != "36" || k.AgeBand != "10002" {
		t.Fatalf("Key() = %+v", k)
	}
	if err := RecordChart(ctx, st, chartSnap(base, "36", "six-to-eight", "1")); err == nil {
		t.Fatal("a non-numeric age band must be rejected")
	}
}

// A store created before the age_band column gets it on open: the column is
// added, interim composite keys are split, and the series index is rebuilt.
func TestChartSnapshotsAgeBandMigration(t *testing.T) {
	st, path := openTestStore(t)
	db := st.DB()
	for _, stmt := range []string{
		`DROP TABLE chart_snapshots`,
		`CREATE TABLE chart_snapshots (id INTEGER PRIMARY KEY, captured_at TEXT NOT NULL, storefront TEXT NOT NULL,
			platform TEXT NOT NULL, genre_id TEXT NOT NULL, chart TEXT NOT NULL, source TEXT NOT NULL, row_limit INTEGER,
			rank INTEGER NOT NULL, app_id TEXT NOT NULL, title TEXT, developer_name TEXT)`,
		`CREATE INDEX idx_chart_snapshots_key ON chart_snapshots(storefront, platform, genre_id, chart, captured_at)`,
		`INSERT INTO chart_snapshots (captured_at, storefront, platform, genre_id, chart, source, row_limit, rank, app_id)
			VALUES ('2026-10-20T00:00:00Z','us','iphone','36','top-free','web',200,1,'407558537'),
			       ('2026-10-21T00:00:00Z','us','iphone','36-kids-10001','top-free','web',200,1,'111')`,
	} {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("seed old schema: %v", err)
		}
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		st2, err := Open(path)
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		if n := countRows(t, st2, "pragma_table_info('chart_snapshots')", "name = 'age_band'"); n != 1 {
			t.Fatalf("open %d: age_band column missing", i)
		}
		if n := countRows(t, st2, "chart_snapshots", "genre_id = '36' AND age_band = '10001' AND app_id = '111'"); n != 1 {
			t.Fatalf("open %d: composite key not split", i)
		}
		if n := countRows(t, st2, "chart_snapshots", "genre_id = '36' AND age_band = '' AND app_id = '407558537'"); n != 1 {
			t.Fatalf("open %d: overall row changed", i)
		}
		if n := countRows(t, st2, "sqlite_master", "type = 'index' AND name = 'idx_chart_snapshots_series'"); n != 1 {
			t.Fatalf("open %d: series index missing", i)
		}
		if n := countRows(t, st2, "sqlite_master", "type = 'index' AND name = 'idx_chart_snapshots_key'"); n != 0 {
			t.Fatalf("open %d: pre-age_band index kept", i)
		}
		prev, latest, err := LatestTwoCharts(context.Background(), st2, ChartKey{Storefront: "us", Platform: "iphone", GenreID: "36", Chart: "top-free"})
		if err != nil || prev != nil || snapIDs(latest) != "407558537" {
			t.Fatalf("open %d: migrated overall series = %s -> %s (%v)", i, snapIDs(prev), snapIDs(latest), err)
		}
		_ = st2.Close()
	}
}

// Review defect (cache vs time series): snapshot writers default to the
// observation time their input carries, not the wall clock.
func TestSnapshotWritersUseObservationTime(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
	lk := appstore.LookupResult{TrackID: 324684580, TrackName: "Spotify: Music and Podcasts", Version: "9.1.92", UserRatingCount: 1000, FetchedAt: t0, FromCache: true}
	if err := RecordLookupSnapshot(ctx, st, "us", lk, time.Time{}); err != nil {
		t.Fatal(err)
	}
	snaps, _ := AppSnapshots(ctx, st, "324684580", "us", time.Time{})
	if len(snaps) != 1 || !snaps[0].CapturedAt.Equal(t0) {
		t.Fatalf("lookup snapshot stamped %v, want the Lookup's fetch time %v", snaps[0].CapturedAt, t0)
	}
	explicit := t0.Add(time.Minute)
	if err := RecordLookupSnapshot(ctx, st, "us", lk, explicit); err != nil {
		t.Fatal(err)
	}
	snaps, _ = AppSnapshots(ctx, st, "324684580", "us", time.Time{})
	if len(snaps) != 2 || !snaps[1].CapturedAt.Equal(explicit) {
		t.Fatalf("an explicit time must win: %+v", snaps)
	}

	page := fixturePage(t, "product_spotify_us.html")
	page.FetchedAt = time.Time{}
	t1 := t0.Add(2 * time.Hour)
	lk.FetchedAt = t1
	if err := RecordProductPage(ctx, st, "gb", page, lk); err != nil {
		t.Fatal(err)
	}
	gb, _ := AppSnapshots(ctx, st, "324684580", "gb", time.Time{})
	if len(gb) != 1 || !gb[0].CapturedAt.Equal(t1) || gb[0].Fields.LookupFetchedAt != appstore.FormatTime(t1) {
		t.Fatalf("page without a fetch time: %+v", gb)
	}
	page.FetchedAt = t1.Add(10 * time.Minute)
	if err := RecordProductPage(ctx, st, "gb", page, lk); err != nil {
		t.Fatal(err)
	}
	gb, _ = AppSnapshots(ctx, st, "324684580", "gb", time.Time{})
	if len(gb) != 2 || !gb[1].CapturedAt.Equal(page.FetchedAt) || gb[1].Fields.LookupFetchedAt != appstore.FormatTime(t1) {
		t.Fatalf("page+lookup snapshot: captured %v lookupFetchedAt %q", gb[1].CapturedAt, gb[1].Fields.LookupFetchedAt)
	}
	ratings, _ := RatingsSeries(ctx, st, "324684580", "gb", SourcePage)
	if len(ratings) != 2 || !ratings[0].CapturedAt.Equal(t1) {
		t.Fatalf("page ratings stamped %+v", ratings)
	}
}

func TestRatingsKeywordFeaturedCaptures(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()
	base := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	var snaps []RatingsSnapshot
	for i := 2; i >= 0; i-- { // written newest first, read back oldest first
		snaps = append(snaps, RatingsSnapshot{CapturedAt: base.Add(time.Duration(i) * 24 * time.Hour), Storefront: "us", AppID: "324684580", AverageRating: 4.8, RatingCount: int64(1000 + i), Source: SourceLookup})
	}
	if err := RecordRatings(ctx, st, snaps...); err != nil {
		t.Fatal(err)
	}
	series, err := RatingsSeries(ctx, st, "324684580", "us", SourceLookup)
	if err != nil || len(series) != 3 || series[0].RatingCount != 1000 || series[2].RatingCount != 1002 {
		t.Fatalf("ratings series = %v %+v", err, series)
	}
	if other, _ := RatingsSeries(ctx, st, "324684580", "us", SourcePage); len(other) != 0 {
		t.Fatal("source filter leaked rows")
	}

	for i, rank := range []int{3, 0, 1} {
		k := KeywordRank{CapturedAt: base.Add(time.Duration(i) * time.Hour), Storefront: "us", Platform: "iphone", Term: "Music", Source: SourceSearchAPI, AppID: "324684580", Rank: rank, ResultCount: 50, TopIDs: []string{"1", "2", "324684580"}}
		if err := RecordKeywordRank(ctx, st, k); err != nil {
			t.Fatal(err)
		}
	}
	kw, err := KeywordSeries(ctx, st, KeywordFilter{Term: "music", Storefront: "us", AppID: "324684580"})
	if err != nil || len(kw) != 3 || kw[0].Rank != 3 || kw[1].Rank != 0 || kw[2].Rank != 1 || len(kw[0].TopIDs) != 3 {
		t.Fatalf("keyword series = %v %+v", err, kw)
	}
	var nullRanks int
	_ = st.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM keyword_ranks WHERE rank IS NULL`).Scan(&nullRanks)
	if nullRanks != 1 {
		t.Fatalf("not-found rank should be stored as NULL, got %d NULL rows", nullRanks)
	}

	feat := FeaturedSnapshot{CapturedAt: base, Storefront: "us", Platform: "iphone", Stories: []FeaturedStory{
		{Position: 2, StoryID: "200", Title: "Second", AppIDs: []string{"9"}},
		{Position: 1, StoryID: "100", Title: "First", AppIDs: []string{"7", "8"}},
		{Position: 3, StoryID: "300", Title: "No apps"},
	}}
	if err := RecordFeatured(ctx, st, feat); err != nil {
		t.Fatal(err)
	}
	rows, err := FeaturedSince(ctx, st, "us", base.Add(-time.Hour))
	if err != nil || len(rows) != 4 || rows[0].StoryID != "100" || rows[3].StoryID != "300" || rows[3].AppID != "" {
		t.Fatalf("featured = %v %+v", err, rows)
	}
	if later, _ := FeaturedSince(ctx, st, "us", base.Add(time.Hour)); len(later) != 0 {
		t.Fatal("since filter ignored")
	}

	for i, outcome := range []string{appstore.OutcomeOK, appstore.OutcomeVerifiedEmpty, appstore.OutcomeFailed} {
		c := appstore.Capture{CapturedAt: base.Add(time.Duration(i) * time.Minute), URL: "https://itunes.apple.com/lookup?id=" + string(rune('1'+i)), Host: "itunes.apple.com", Status: 200, Outcome: outcome, EntryCount: i, ParserVersion: appstore.ParserVersion}
		if err := NewCaptureSink(st).RecordCapture(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	caps, err := Captures(ctx, st, CaptureFilter{Host: "itunes.apple.com"})
	if err != nil || len(caps) != 3 || caps[0].Outcome != appstore.OutcomeOK || caps[2].Outcome != appstore.OutcomeFailed {
		t.Fatalf("captures = %v %+v", err, caps)
	}
	latest, _ := Captures(ctx, st, CaptureFilter{Limit: 2})
	if len(latest) != 2 || latest[0].ID >= latest[1].ID || latest[1].Outcome != appstore.OutcomeFailed {
		t.Fatalf("limited captures = %+v", latest)
	}
	if empty, _ := Captures(ctx, st, CaptureFilter{Outcome: appstore.OutcomeVerifiedEmpty}); len(empty) != 1 {
		t.Fatal("outcome filter broken")
	}
	if NewCaptureSink(nil) != nil {
		t.Fatal("NewCaptureSink(nil) must be a nil interface")
	}
}
