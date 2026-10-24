// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Synthetic snapshot-store scenarios for the slice-3b commands. Every row is
// written through the typed store accessors with times relative to a base
// "now", so unit tests (base pinned) and binary acceptance runs (base = the
// current hour, seeded by TestS3BSeedAcceptanceStore) share the same data.
// Reviewer names, titles and bodies are synthetic; app ids are public
// catalogue ids or obviously synthetic ones (1000000001...).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

// s3bTestNow is the pinned clock for slice-3b unit tests.
var s3bTestNow = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

// s3bAgo returns the UTC day `days` before base at the given hour.
func s3bAgo(base time.Time, days, hour int) time.Time {
	d := base.UTC().AddDate(0, 0, -days)
	return time.Date(d.Year(), d.Month(), d.Day(), hour, 0, 0, 0, time.UTC)
}

func s3bMust(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// s3bNewStore opens a fresh store file in a temp dir.
func s3bNewStore(t testing.TB) (*store.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "s3b.db")
	st, err := store.Open(path)
	s3bMust(t, err)
	t.Cleanup(func() { _ = st.Close() })
	return st, path
}

// s3bPinClock pins the commands' clock to s3bTestNow for one test.
func s3bPinClock(t *testing.T) {
	t.Helper()
	prev := s3bNow
	s3bNow = func() time.Time { return s3bTestNow }
	t.Cleanup(func() { s3bNow = prev })
}

// runS3B executes the CLI with args against dbPath and returns stdout,
// stderr and the error.
func runS3B(t *testing.T, dbPath string, args ...string) (string, string, error) {
	t.Helper()
	cmd := RootCmd()
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	full := append(append([]string{}, args...), "--db", dbPath)
	cmd.SetArgs(full)
	err := cmd.Execute()
	return out.String(), errb.String(), err
}

// decodeS3B unmarshals stdout JSON into v or fails with the raw output.
func decodeS3B(t *testing.T, stdout string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(stdout), v); err != nil {
		t.Fatalf("stdout is not the expected JSON: %v\n%s", err, stdout)
	}
}

func s3bRatingsRow(sf, app, source string, at time.Time, count int64) store.RatingsSnapshot {
	return store.RatingsSnapshot{CapturedAt: at, Storefront: sf, AppID: app, RatingCount: count, AverageRating: 4.5, Source: source}
}

func chartSnap(sf, platform, genre, chart, source string, rowLimit int, at time.Time, ranks map[string]int) store.ChartSnapshot {
	rows := make([]store.ChartRow, 0, len(ranks))
	for id, r := range ranks {
		rows = append(rows, store.ChartRow{Rank: r, AppID: id, Title: "App " + id, DeveloperName: "Developer " + id})
	}
	return store.ChartSnapshot{CapturedAt: at, Storefront: sf, Platform: platform, GenreID: genre, Chart: chart, Source: source, RowLimit: rowLimit, Rows: rows}
}

func synthReview(id string, rating int, at time.Time) appstore.Review {
	return appstore.Review{ID: id, Rating: rating, Version: "1.0", Title: "Sample title " + id, Body: "Sample body " + id,
		Author: "reviewer-" + id, ReviewedAt: appstore.FormatTime(at)}
}

// --- traction: Spotify (324684580) -------------------------------------------

const s3bSpotify = "324684580"

func seedTractionScenario(t testing.TB, st *store.Store, base time.Time) {
	t.Helper()
	ctx := context.Background()
	lastYear := time.Date(base.Year()-1, 12, 28, 9, 0, 0, 0, time.UTC)
	s3bMust(t, store.RecordRatings(ctx, st,
		s3bRatingsRow("us", s3bSpotify, store.SourceLookup, lastYear, 90000),
		s3bRatingsRow("us", s3bSpotify, store.SourceLookup, s3bAgo(base, 10, 9), 100000),
		s3bRatingsRow("us", s3bSpotify, store.SourceLookup, s3bAgo(base, 9, 9), 100500),
		s3bRatingsRow("us", s3bSpotify, store.SourceLookup, s3bAgo(base, 8, 9), 100400), // -0.1%: removed ratings, counts 0
		s3bRatingsRow("us", s3bSpotify, store.SourceLookup, s3bAgo(base, 7, 9), 2000),   // -98%: a reset
		s3bRatingsRow("us", s3bSpotify, store.SourceLookup, s3bAgo(base, 6, 6), 2500),
		s3bRatingsRow("us", s3bSpotify, store.SourceLookup, s3bAgo(base, 6, 18), 2600), // last capture of the day wins
		s3bRatingsRow("us", s3bSpotify, store.SourcePage, s3bAgo(base, 9, 10), 999999), // ignored: lookup preferred
		s3bRatingsRow("gb", s3bSpotify, store.SourcePage, s3bAgo(base, 9, 9), 5000),
		s3bRatingsRow("gb", s3bSpotify, store.SourcePage, s3bAgo(base, 8, 9), 5100),
		s3bRatingsRow("jp", s3bSpotify, store.SourceLookup, s3bAgo(base, 5, 9), 3000),
	))
	music := func(sf string, at time.Time, ranks map[string]int) {
		s3bMust(t, store.RecordChart(ctx, st, chartSnap(sf, "iphone", "6011", "top-free", store.SourceWebChart, 200, at, ranks)))
	}
	music("us", s3bAgo(base, 9, 9), map[string]int{"2100000001": 1, s3bSpotify: 5})
	music("us", s3bAgo(base, 8, 9), map[string]int{"2100000001": 1, s3bSpotify: 3})
	music("us", s3bAgo(base, 7, 9), map[string]int{"2100000001": 1}) // captured, Spotify absent
	music("us", s3bAgo(base, 6, 9), map[string]int{"2100000001": 1, s3bSpotify: 4})
	music("gb", s3bAgo(base, 8, 9), map[string]int{"2100000001": 1, s3bSpotify: 12})

	_, err := store.UpsertReviews(ctx, st, store.ReviewUpsert{Storefront: "us", AppID: s3bSpotify, Sort: appstore.SortMostRecent, Source: store.SourceRSS,
		SeenAt: s3bAgo(base, 8, 12), Reviews: []appstore.Review{
			synthReview("t1", 5, s3bAgo(base, 9, 10)),
			synthReview("t2", 4, s3bAgo(base, 9, 11)),
			synthReview("t3", 1, s3bAgo(base, 8, 10)),
		}})
	s3bMust(t, err)
	_, err = store.UpsertReviews(ctx, st, store.ReviewUpsert{Storefront: "us", AppID: s3bSpotify, Sort: appstore.SortMostHelpful, Source: store.SourceRSS,
		SeenAt: s3bAgo(base, 8, 12), Reviews: []appstore.Review{synthReview("t4", 3, time.Date(base.Year()-1, 12, 20, 10, 0, 0, 0, time.UTC))}})
	s3bMust(t, err)
	capped := make([]appstore.Review, 0, 500)
	for i := 0; i < 500; i++ {
		capped = append(capped, synthReview("g"+strconv.Itoa(i), 1+i%5, s3bAgo(base, 8, 10)))
	}
	_, err = store.UpsertReviews(ctx, st, store.ReviewUpsert{Storefront: "gb", AppID: s3bSpotify, Sort: appstore.SortMostRecent, Source: store.SourceRSS,
		SeenAt: s3bAgo(base, 8, 12), Reviews: capped})
	s3bMust(t, err)
	// That capture reached the feed's last page full, as the capture log
	// records it (traction reads the 500-review cap from there).
	s3bMust(t, store.RecordCapture(ctx, st, appstore.Capture{CapturedAt: s3bAgo(base, 8, 12), Host: "itunes.apple.com", Status: 200,
		URL: "https://itunes.apple.com/gb/rss/customerreviews/page=10/id=" + s3bSpotify + "/sortby=mostrecent/json", Outcome: appstore.OutcomeOK, EntryCount: 50}))
}

// --- impact and changes: Candy Crush Saga (553834731) ------------------------

const s3bCandy = "553834731"

func candyLookup(version, title string, price float64, formatted, released string) appstore.LookupResult {
	return appstore.LookupResult{TrackID: 553834731, TrackName: title, BundleID: "com.king.candycrushsaga", Version: version,
		Price: price, FormattedPrice: formatted, Currency: "USD", UserRatingCount: 1000, AverageUserRating: 4.6,
		CurrentVersionReleaseDate: released, ArtistID: 526656015, ArtistName: "King", PrimaryGenreID: 6014, PrimaryGenreName: "Games",
		TrackViewURL: "https://apps.apple.com/us/app/candy-crush-saga/id553834731?uo=4"}
}

func seedCandyScenario(t testing.TB, st *store.Store, base time.Time) {
	t.Helper()
	ctx := context.Background()
	f := appstore.FormatTime
	v1299, v1300, v1301 := s3bAgo(base, 21, 10), s3bAgo(base, 14, 10), s3bAgo(base, 7, 10)

	// Listing snapshots: lookup d-20, page d-10, page d-6, lookup d-2.
	s3bMust(t, store.RecordLookupSnapshot(ctx, st, "us", candyLookup("1.299.0", "Candy Crush Saga", 0, "Free", f(v1299)), s3bAgo(base, 20, 8)))
	priv1 := appstore.Privacy{Types: []appstore.PrivacyType{{Identifier: "DATA_LINKED_TO_YOU", Title: "Data Linked to You",
		Categories: []appstore.PrivacyCategory{{Identifier: "IDENTIFIERS", Title: "Identifiers", DataTypes: []string{"User ID"}}}}}}
	priv2 := appstore.Privacy{Types: []appstore.PrivacyType{
		{Identifier: "DATA_LINKED_TO_YOU", Title: "Data Linked to You", Categories: []appstore.PrivacyCategory{
			{Identifier: "IDENTIFIERS", Title: "Identifiers", DataTypes: []string{"User ID"}},
			{Identifier: "LOCATION", Title: "Location", DataTypes: []string{"Precise Location"}}}},
		{Identifier: "DATA_USED_TO_TRACK_YOU", Title: "Data Used to Track You", Categories: []appstore.PrivacyCategory{
			{Identifier: "IDENTIFIERS", Title: "Identifiers", DataTypes: []string{"Device ID"}}}},
	}}
	summer := appstore.AppEvent{EventID: "e-summer", Title: "Summer Fest", Kind: "Special Event", StartDate: f(s3bAgo(base, 12, 0)), EndDate: f(s3bAgo(base, 2, 0))}
	halloween := appstore.AppEvent{EventID: "e-halloween", Title: "Halloween Party", Kind: "Challenge", StartDate: f(s3bAgo(base, 5, 0)), EndDate: f(s3bAgo(base, -7, 0)), Badge: "NOW AVAILABLE"}
	page1 := appstore.ProductPage{AdamID: s3bCandy, Subtitle: "Sweet puzzle", FetchedAt: s3bAgo(base, 10, 8),
		InAppPurchases: []appstore.IAPItem{{Position: 1, Name: "Gold Bars", Price: "$0.99"}, {Position: 2, Name: "Lives", Price: "$1.99"}},
		Privacy:        priv1, Events: []appstore.AppEvent{summer},
		Versions: []appstore.VersionEntry{{Version: "1.300.0", ReleasedAt: f(v1300), Notes: "Sweeter levels and bug fixes."}, {Version: "1.299.0", ReleasedAt: f(v1299), Notes: "Bug fixes."}}}
	s3bMust(t, store.RecordProductPage(ctx, st, "us", page1, candyLookup("1.300.0", "Candy Crush Saga", 0, "Free", f(v1300))))
	page2 := appstore.ProductPage{AdamID: s3bCandy, Subtitle: "Sweetest puzzle", EditorsChoice: true, FetchedAt: s3bAgo(base, 6, 8),
		InAppPurchases: []appstore.IAPItem{{Position: 1, Name: "Gold Bars", Price: "$1.29"}, {Position: 2, Name: "Booster Pack", Price: "$4.99"}},
		Privacy:        priv2, Events: []appstore.AppEvent{summer, halloween},
		Versions: []appstore.VersionEntry{{Version: "1.301.0", ReleasedAt: f(v1301), Notes: "Halloween levels are here."}, {Version: "1.300.0", ReleasedAt: f(v1300)}}}
	s3bMust(t, store.RecordProductPage(ctx, st, "us", page2, candyLookup("1.301.0", "Candy Crush Saga!", 0, "Free", f(v1301))))
	s3bMust(t, store.RecordLookupSnapshot(ctx, st, "us", candyLookup("1.301.0", "Candy Crush Saga!", 0.99, "$0.99", f(v1301)), s3bAgo(base, 2, 8)))

	// Rating counts: three capture days before the 1.301.0 release, three after.
	s3bMust(t, store.RecordRatings(ctx, st,
		s3bRatingsRow("us", s3bCandy, store.SourceLookup, s3bAgo(base, 13, 9), 1000),
		s3bRatingsRow("us", s3bCandy, store.SourceLookup, s3bAgo(base, 11, 9), 1200),
		s3bRatingsRow("us", s3bCandy, store.SourceLookup, s3bAgo(base, 9, 9), 1300),
		s3bRatingsRow("us", s3bCandy, store.SourceLookup, s3bAgo(base, 6, 9), 1500),
		s3bRatingsRow("us", s3bCandy, store.SourceLookup, s3bAgo(base, 4, 9), 1900),
		s3bRatingsRow("us", s3bCandy, store.SourceLookup, s3bAgo(base, 2, 9), 2100),
		s3bRatingsRow("gb", s3bCandy, store.SourceLookup, s3bAgo(base, 3, 9), 800), // one capture: insufficient data
	))
	puzzle := func(at time.Time, rank int) {
		s3bMust(t, store.RecordChart(ctx, st, chartSnap("us", "iphone", "7012", "top-free", store.SourceWebChart, 200, at,
			map[string]int{"2200000001": 1, s3bCandy: rank})))
	}
	puzzle(s3bAgo(base, 12, 9), 20)
	puzzle(s3bAgo(base, 10, 9), 15)
	puzzle(s3bAgo(base, 5, 9), 8)
	puzzle(s3bAgo(base, 3, 9), 10)

	_, err := store.UpsertReviews(ctx, st, store.ReviewUpsert{Storefront: "us", AppID: s3bCandy, Sort: appstore.SortMostRecent, Source: store.SourceRSS,
		SeenAt: s3bAgo(base, 1, 12), Reviews: []appstore.Review{
			synthReview("c1", 2, s3bAgo(base, 12, 9)),
			synthReview("c2", 4, s3bAgo(base, 9, 9)),
			synthReview("c3", 5, s3bAgo(base, 6, 9)),
			synthReview("c4", 5, s3bAgo(base, 4, 9)),
			synthReview("c5", 4, s3bAgo(base, 1, 9)),
		}})
	s3bMust(t, err)

	for _, k := range []struct {
		days, rank int
	}{{12, 4}, {9, 6}, {5, 2}, {2, 0}} {
		s3bMust(t, store.RecordKeywordRank(ctx, st, store.KeywordRank{CapturedAt: s3bAgo(base, k.days, 9), Storefront: "us", Platform: "iphone",
			Term: "candy", Source: store.SourceSearchAPI, AppID: s3bCandy, Rank: k.rank, ResultCount: 200, TopIDs: []string{"2200000001", "2200000002"}}))
	}

	// Today tab: absent, featured, featured, absent.
	other := store.FeaturedStory{Position: 1, StoryID: "s-picks", Title: "Daily Puzzle Picks", AppIDs: []string{"2200000099"}}
	gotd := store.FeaturedStory{Position: 2, StoryID: "s-gotd", Title: "Game of the Day", AppIDs: []string{s3bCandy}}
	for _, c := range []struct {
		days    int
		stories []store.FeaturedStory
	}{{8, []store.FeaturedStory{other}}, {6, []store.FeaturedStory{other, gotd}}, {5, []store.FeaturedStory{gotd}}, {3, []store.FeaturedStory{other}}} {
		s3bMust(t, store.RecordFeatured(ctx, st, store.FeaturedSnapshot{CapturedAt: s3bAgo(base, c.days, 9), Storefront: "us", Platform: "iphone", Stories: c.stories}))
	}

	// A second tracked app with a single snapshot: it must add no changes.
	single := appstore.LookupResult{TrackID: 850417475, TrackName: "Single Snapshot App", BundleID: "com.example.single", Version: "2.0", FormattedPrice: "Free", Currency: "USD"}
	s3bMust(t, store.RecordProductPage(ctx, st, "us", appstore.ProductPage{AdamID: "850417475", Subtitle: "Only once", FetchedAt: s3bAgo(base, 1, 8)}, single))
}

// --- keyword share (us) -------------------------------------------------------

func seedKeywordScenario(t testing.TB, st *store.Store, base time.Time) {
	t.Helper()
	ctx := context.Background()
	rec := func(days int, sf, term, target string, rank int, top []string) {
		s3bMust(t, store.RecordKeywordRank(ctx, st, store.KeywordRank{CapturedAt: s3bAgo(base, days, 9), Storefront: sf, Platform: "iphone",
			Term: term, Source: store.SourceSearchAPI, AppID: target, Rank: rank, ResultCount: 150, TopIDs: top}))
	}
	rec(8, "us", "music streaming", s3bSpotify, 1, []string{s3bSpotify, "1017492454", "336353151", "284035177", "510855668", "2300000001"})
	rec(1, "us", "music streaming", s3bSpotify, 2, []string{"1017492454", s3bSpotify, "284035177", "336353151", "510855668", "2300000001"})
	rec(8, "us", "podcast player", s3bSpotify, 4, []string{"525463029", "414834813", "888422857", s3bSpotify, "1243410543", "2300000002"})
	rec(1, "us", "podcast player", s3bSpotify, 9, []string{"525463029", "888422857", "414834813", "1243410543", "379693831", "2300000002"})
	rec(2, "us", "audiobooks", "", 0, []string{"379693831", s3bSpotify, "2300000003", "2300000004", "2300000005"})
	rec(1, "gb", "music streaming", s3bSpotify, 1, []string{s3bSpotify, "2300000009"}) // another storefront: never counted for us
}

// --- breakouts: iphone/6014/top-free across storefronts -----------------------

func seedBreakoutsScenario(t testing.TB, st *store.Store, base time.Time) {
	t.Helper()
	ctx := context.Background()
	games := func(sf, source string, rowLimit int, at time.Time, ranks map[string]int) {
		s3bMust(t, store.RecordChart(ctx, st, chartSnap(sf, "iphone", "6014", "top-free", source, rowLimit, at, ranks)))
	}
	b, e := s3bAgo(base, 6, 9), s3bAgo(base, 1, 9)
	const filler, hero, solo, two, faller = "2000000001", "1000000001", "1000000002", "1000000003", "1000000004"
	games("us", store.SourceWebChart, 200, b, map[string]int{filler: 1, faller: 5, two: 30, hero: 50, solo: 100})
	games("us", store.SourceWebChart, 200, e, map[string]int{filler: 1, hero: 10, two: 15, solo: 20, faller: 50})
	games("gb", store.SourceWebChart, 200, b, map[string]int{filler: 1, solo: 20})
	games("gb", store.SourceWebChart, 200, e, map[string]int{filler: 1, solo: 19, hero: 30})
	games("jp", store.SourceWebChart, 200, b, map[string]int{filler: 1, hero: 80})
	games("jp", store.SourceWebChart, 200, e, map[string]int{filler: 1, hero: 60})
	games("de", store.SourceWebChart, 200, b, map[string]int{filler: 1, two: 40})
	games("de", store.SourceWebChart, 200, e, map[string]int{filler: 1, two: 25})
	games("fr", store.SourceWebChart, 200, e, map[string]int{filler: 1, hero: 5})                    // one capture in window
	games("it", store.SourceWebChart, 200, s3bAgo(base, 20, 9), map[string]int{filler: 1, hero: 90}) // captures before the window only
	games("it", store.SourceWebChart, 200, s3bAgo(base, 15, 9), map[string]int{filler: 1, hero: 9})
	games("ca", store.SourceRSS, 100, b, map[string]int{filler: 1, hero: 99})     // rss then web:
	games("ca", store.SourceWebChart, 200, e, map[string]int{filler: 1, hero: 2}) // not comparable
}

// seedSingleCaptureScenario stores exactly one listing snapshot and one
// ratings snapshot for Spotify: nothing can be compared yet.
func seedSingleCaptureScenario(t testing.TB, st *store.Store, base time.Time) {
	t.Helper()
	ctx := context.Background()
	lk := appstore.LookupResult{TrackID: 324684580, TrackName: "Spotify: Music and Podcasts", BundleID: "com.spotify.client", Version: "9.1.92",
		FormattedPrice: "Free", Currency: "USD", UserRatingCount: 42719156, CurrentVersionReleaseDate: appstore.FormatTime(s3bAgo(base, 5, 13))}
	s3bMust(t, store.RecordLookupSnapshot(ctx, st, "us", lk, s3bAgo(base, 1, 9)))
	s3bMust(t, store.RecordRatings(ctx, st, s3bRatingsRow("us", s3bSpotify, store.SourceLookup, s3bAgo(base, 1, 9), 42719156)))
}

func seedAllS3BScenarios(t testing.TB, st *store.Store, base time.Time) {
	seedTractionScenario(t, st, base)
	seedCandyScenario(t, st, base)
	seedKeywordScenario(t, st, base)
	seedBreakoutsScenario(t, st, base)
}

// TestS3BSeedAcceptanceStore seeds the slice-3b scenarios into the store
// files named by S3B_SEED_DB (every scenario) and S3B_SEED_SINGLE_DB (one
// capture only), relative to the current hour, for binary acceptance runs.
// It is skipped unless one of those variables is set.
func TestS3BSeedAcceptanceStore(t *testing.T) {
	all, single := os.Getenv("S3B_SEED_DB"), os.Getenv("S3B_SEED_SINGLE_DB")
	if all == "" && single == "" {
		t.Skip("set S3B_SEED_DB and/or S3B_SEED_SINGLE_DB to seed acceptance stores")
	}
	testenv.Isolate(t)
	base := time.Now().UTC().Truncate(time.Hour)
	for _, job := range []struct {
		path string
		seed func(testing.TB, *store.Store, time.Time)
	}{{all, seedAllS3BScenarios}, {single, seedSingleCaptureScenario}} {
		if job.path == "" {
			continue
		}
		_ = os.Remove(job.path)
		st, err := store.Open(job.path)
		s3bMust(t, err)
		job.seed(t, st, base)
		s3bMust(t, st.Close())
		fmt.Fprintf(os.Stderr, "seeded %s (base %s)\n", job.path, appstore.FormatTime(base))
	}
}
