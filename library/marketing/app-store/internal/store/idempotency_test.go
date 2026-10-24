// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
)

// Every snapshot writer is idempotent on the observation: recording the same
// response again (what a cache-served rerun does, since a cached response
// keeps its FetchedAt) writes nothing, and an older observation applied
// after a newer one never moves first/last-seen times or change points.
func TestWritersAreIdempotentOnTheObservation(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()
	t1 := time.Date(2026, 10, 23, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)

	// Lookup snapshots.
	lk := appstore.LookupResult{TrackID: 324684580, TrackName: "Spotify: Music and Podcasts", Version: "9.1.92", UserRatingCount: 42703709, FetchedAt: t1}
	for i := 0; i < 2; i++ {
		if err := RecordLookupSnapshot(ctx, st, "us", lk, time.Time{}); err != nil {
			t.Fatal(err)
		}
	}
	if n := countRows(t, st, "app_snapshots", "app_id = '324684580' AND source = 'lookup'"); n != 1 {
		t.Fatalf("lookup snapshots after a repeated observation = %d, want 1", n)
	}

	// Product pages: snapshot, page ratings, privacy, IAP, events.
	page := fixturePage(t, "product_candycrush_us.html")
	cc := appstore.LookupResult{
		TrackID: 553834731, TrackName: "Candy Crush Saga", Version: page.Versions[0].Version, Currency: "USD",
		UserRatingCount: 4030718, TrackViewURL: "https://apps.apple.com/us/app/candy-crush-saga/id553834731?uo=4", FetchedAt: t1,
	}
	page.FetchedAt = t1
	for i := 0; i < 2; i++ {
		if err := RecordProductPage(ctx, st, "us", page, cc); err != nil {
			t.Fatal(err)
		}
	}
	for table, want := range map[string]int{"app_snapshots": 1, "ratings_snapshots": 1, "privacy_snapshots": 1, "iap_prices": 10, "app_events": 1, "version_entries": 25} {
		if n := countRows(t, st, table, "app_id = '553834731'"); n != want {
			t.Fatalf("%s after a repeated page observation = %d, want %d", table, n, want)
		}
	}
	// A newer observation, then the older one again (a stale cached copy).
	page.FetchedAt = t2
	if err := RecordProductPage(ctx, st, "us", page, cc); err != nil {
		t.Fatal(err)
	}
	page.FetchedAt = t1
	if err := RecordProductPage(ctx, st, "us", page, cc); err != nil {
		t.Fatal(err)
	}
	for table, want := range map[string]int{"app_snapshots": 2, "ratings_snapshots": 2, "privacy_snapshots": 1, "iap_prices": 10} {
		if n := countRows(t, st, table, "app_id = '553834731'"); n != want {
			t.Fatalf("%s after t1, t2, t1 = %d, want %d", table, n, want)
		}
	}
	events, err := EventsHistory(ctx, st, "553834731", "us")
	if err != nil || len(events) != 1 || !events[0].FirstSeenAt.Equal(t1) || !events[0].LastSeenAt.Equal(t2) {
		t.Fatalf("events after an older observation = %v %+v, want first %s last %s", err, events, t1, t2)
	}

	// Ratings.
	snap := RatingsSnapshot{CapturedAt: t1, Storefront: "us", AppID: "1", RatingCount: 10, Source: SourceLookup}
	if n, err := RecordRatingsN(ctx, st, snap, snap); err != nil || n != 1 {
		t.Fatalf("RecordRatingsN(same observation twice) = %d %v, want 1", n, err)
	}
	if n, err := RecordRatingsN(ctx, st, snap); err != nil || n != 0 {
		t.Fatalf("RecordRatingsN(stored observation) = %d %v, want 0", n, err)
	}
	other := snap
	other.Source = SourcePage
	if n, _ := RecordRatingsN(ctx, st, other); n != 1 {
		t.Fatal("another source at the same time is another observation")
	}

	// Featured.
	feat := FeaturedSnapshot{CapturedAt: t1, Storefront: "us", Platform: "iphone", Stories: []FeaturedStory{{Position: 1, StoryID: "s1", Title: "Story", AppIDs: []string{"1", "2"}}}}
	if ok, err := RecordFeaturedOnce(ctx, st, feat); err != nil || !ok {
		t.Fatalf("first featured capture = %v %v", ok, err)
	}
	if ok, err := RecordFeaturedOnce(ctx, st, feat); err != nil || ok {
		t.Fatalf("repeated featured capture written = %v %v", ok, err)
	}
	if n := countRows(t, st, "featured_snapshots", ""); n != 2 {
		t.Fatalf("featured rows = %d, want 2", n)
	}

	// Keyword ranks: the target app is part of the observation.
	k := KeywordRank{CapturedAt: t1, Storefront: "us", Platform: "iphone", Term: "Spotify", Source: SourceSearchAPI, AppID: "324684580", Rank: 1, TopIDs: []string{"324684580"}}
	if ok, err := RecordKeywordRankOnce(ctx, st, k); err != nil || !ok {
		t.Fatalf("first keyword capture = %v %v", ok, err)
	}
	k.Term = " spotify "
	if ok, err := RecordKeywordRankOnce(ctx, st, k); err != nil || ok {
		t.Fatalf("repeated keyword capture written = %v %v", ok, err)
	}
	k.AppID = ""
	if ok, _ := RecordKeywordRankOnce(ctx, st, k); !ok {
		t.Fatal("a capture without a target is a different data point")
	}

	// Reviews: an older batch never moves last_seen_at back or overwrites
	// the newer text.
	reviews := fixtureReviews(t, "reviews_rss_spotify_us_p1_recent.json")[:3]
	newer := append([]appstore.Review(nil), reviews...)
	newer[0].Body = "Sample body edited"
	if _, err := UpsertReviews(ctx, st, ReviewUpsert{Storefront: "us", AppID: "324684580", Sort: appstore.SortMostRecent, Source: SourceRSS, SeenAt: t2, Reviews: newer}); err != nil {
		t.Fatal(err)
	}
	stats, err := UpsertReviews(ctx, st, ReviewUpsert{Storefront: "us", AppID: "324684580", Sort: appstore.SortMostRecent, Source: SourceRSS, SeenAt: t1, Reviews: reviews})
	if err != nil || stats.Inserted != 0 {
		t.Fatalf("older batch = %+v %v", stats, err)
	}
	stored, err := Reviews(ctx, st, ReviewFilter{AppID: "324684580"})
	if err != nil || len(stored) != 3 {
		t.Fatalf("reviews = %v %d", err, len(stored))
	}
	for _, r := range stored {
		if !r.LastSeenAt.Equal(t2) || !r.FirstSeenAt.Equal(t1) {
			t.Fatalf("review %s seen %s..%s, want %s..%s", r.ReviewID, r.FirstSeenAt, r.LastSeenAt, t1, t2)
		}
		if r.ReviewID == newer[0].ID && r.Body != "Sample body edited" {
			t.Fatalf("an older observation overwrote the newer body: %q", r.Body)
		}
	}
	stats, err = UpsertReviews(ctx, st, ReviewUpsert{Storefront: "us", AppID: "324684580", Sort: appstore.SortMostRecent, Source: SourceRSS, SeenAt: t2, Reviews: newer})
	if err != nil || stats.Unchanged != 3 {
		t.Fatalf("repeated newest batch = %+v %v, want 3 unchanged", stats, err)
	}
}

// Stores created before app_snapshots.source existed get the column and a
// backfill from fields_json, so idempotency checks see the old rows.
func TestAppSnapshotsSourceColumnMigration(t *testing.T) {
	st, path := openTestStore(t)
	db := st.DB()
	for _, stmt := range []string{
		`DROP TABLE app_snapshots`,
		`CREATE TABLE app_snapshots (id INTEGER PRIMARY KEY, captured_at TEXT NOT NULL, storefront TEXT NOT NULL,
			app_id TEXT NOT NULL, bundle_id TEXT, content_hash TEXT, title TEXT, subtitle TEXT, version TEXT, price REAL,
			currency TEXT, formatted_price TEXT, average_rating REAL, rating_count INTEGER, editors_choice INTEGER,
			has_iap INTEGER, screenshots_hash TEXT, privacy_hash TEXT, iap_hash TEXT, events_hash TEXT,
			chart_position TEXT, fields_json TEXT)`,
		`INSERT INTO app_snapshots (captured_at, storefront, app_id, title, fields_json)
			VALUES ('2026-10-23T10:00:00Z','us','324684580','Spotify','{"source":"lookup","appId":"324684580"}')`,
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
		if n := countRows(t, st2, "app_snapshots", "source = 'lookup'"); n != 1 {
			t.Fatalf("open %d: source not backfilled", i)
		}
		lk := appstore.LookupResult{TrackID: 324684580, TrackName: "Spotify", FetchedAt: time.Date(2026, 10, 23, 10, 0, 0, 0, time.UTC)}
		if err := RecordLookupSnapshot(context.Background(), st2, "us", lk, time.Time{}); err != nil {
			t.Fatal(err)
		}
		if n := countRows(t, st2, "app_snapshots", ""); n != 1 {
			t.Fatalf("open %d: the migrated observation was recorded again (%d rows)", i, n)
		}
		_ = st2.Close()
	}
}

// /api reviews carry no helpfulness votes: seeing a review there after the
// RSS feed must keep its stored votes, while a later RSS sighting still
// updates them.
func TestReviewVotesSurviveASourceWithoutVotes(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()
	t1 := time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)
	rss := []appstore.Review{{ID: "r1", Rating: 4, Title: "Sample title 1", Body: "Sample body 1", Author: "reviewer-1", VoteSum: 5, VoteCount: 7}}
	if _, err := UpsertReviews(ctx, st, ReviewUpsert{Storefront: "us", AppID: "1", Sort: appstore.SortMostHelpful, Source: SourceRSS, SeenAt: t1, Reviews: rss}); err != nil {
		t.Fatal(err)
	}
	api := []appstore.Review{{ID: "r1", Rating: 4, Title: "Sample title 1", Body: "Sample body 1", Author: "reviewer-1", DeveloperResponse: "Sample response 1"}}
	if _, err := UpsertReviews(ctx, st, ReviewUpsert{Storefront: "us", AppID: "1", Sort: "recent", Source: SourceAPI, SeenAt: t1.Add(time.Hour), Reviews: api}); err != nil {
		t.Fatal(err)
	}
	got, err := Reviews(ctx, st, ReviewFilter{AppID: "1"})
	if err != nil || len(got) != 1 {
		t.Fatalf("reviews = %v %d", err, len(got))
	}
	if got[0].VoteSum != 5 || got[0].VoteCount != 7 || got[0].DeveloperResponse != "Sample response 1" || got[0].Source != "api,rss" {
		t.Fatalf("after an /api sighting: %+v, want votes 5/7 kept and the response added", got[0])
	}
	rss[0].VoteSum, rss[0].VoteCount = 6, 9
	if _, err := UpsertReviews(ctx, st, ReviewUpsert{Storefront: "us", AppID: "1", Sort: appstore.SortMostHelpful, Source: SourceRSS, SeenAt: t1.Add(2 * time.Hour), Reviews: rss}); err != nil {
		t.Fatal(err)
	}
	got, _ = Reviews(ctx, st, ReviewFilter{AppID: "1"})
	if got[0].VoteSum != 6 || got[0].VoteCount != 9 || got[0].DeveloperResponse != "Sample response 1" {
		t.Fatalf("after a newer RSS sighting: %+v, want votes 6/9 and the response kept", got[0])
	}
	// A review first seen through /api has no votes yet (stored NULL, read 0).
	if _, err := UpsertReviews(ctx, st, ReviewUpsert{Storefront: "us", AppID: "1", Sort: "recent", Source: SourceAPI, SeenAt: t1, Reviews: []appstore.Review{{ID: "r2", Rating: 2, Title: "Sample title 2", Body: "Sample body 2"}}}); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, st, "reviews", "review_id = 'r2' AND vote_sum IS NULL AND vote_count IS NULL"); n != 1 {
		t.Fatal("an /api-only review must not store invented zero votes")
	}
}

// A privacy label or IAP list that disappears is a change point when the
// page positively showed none; a page the parser could not read (no
// NoneShown flag) records nothing, and a repeated empty observation adds
// nothing.
func TestEmptyPrivacyAndIAPAreChangePointsOnlyWhenShown(t *testing.T) {
	st, _ := openTestStore(t)
	ctx := context.Background()
	t1 := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	page := fixturePage(t, "product_candycrush_us.html")
	lk := appstore.LookupResult{TrackID: 553834731, TrackName: "Candy Crush Saga", Version: page.Versions[0].Version, Currency: "USD", FetchedAt: t1}
	record := func(p appstore.ProductPage, at time.Time) {
		t.Helper()
		p.FetchedAt = at
		if err := RecordProductPage(ctx, st, "us", p, lk); err != nil {
			t.Fatal(err)
		}
	}
	record(page, t1)

	// The parser could not read the sections (no NoneShown): nothing recorded.
	unread := page
	unread.InAppPurchases, unread.HasInAppPurchases = []appstore.IAPItem{}, false
	unread.Privacy = appstore.Privacy{Types: []appstore.PrivacyType{}}
	record(unread, t1.Add(24*time.Hour))
	iaps, _ := IAPHistory(ctx, st, "553834731", "us")
	privacy, _ := PrivacyHistory(ctx, st, "553834731", "us")
	if len(iaps) != 1 || len(privacy) != 1 {
		t.Fatalf("an unreadable page recorded a removal: %d IAP lists, %d labels", len(iaps), len(privacy))
	}

	// The page positively shows no list and no label: both are change points.
	removed := unread
	removed.IAPNoneShown = true
	removed.Privacy.NoneShown = true
	record(removed, t1.Add(48*time.Hour))
	record(removed, t1.Add(72*time.Hour)) // a repeated empty observation adds nothing
	iaps, _ = IAPHistory(ctx, st, "553834731", "us")
	privacy, _ = PrivacyHistory(ctx, st, "553834731", "us")
	if len(iaps) != 2 || len(iaps[1].Items) != 0 || !iaps[1].CapturedAt.Equal(t1.Add(48*time.Hour)) || len(iaps[0].Items) != 10 {
		t.Fatalf("IAP history = %+v, want the 10-item list then an empty list at +48h", iaps)
	}
	if len(privacy) != 2 || len(privacy[1].Privacy.Types) != 0 || len(privacy[0].Privacy.Types) == 0 {
		t.Fatalf("privacy history = %+v, want the label then an empty label", privacy)
	}
	if n := countRows(t, st, "iap_prices", "app_id = '553834731' AND position = 0"); n != 1 {
		t.Fatalf("empty-list marker rows = %d, want 1", n)
	}

	// The list and label come back: new change points again.
	record(page, t1.Add(96*time.Hour))
	iaps, _ = IAPHistory(ctx, st, "553834731", "us")
	privacy, _ = PrivacyHistory(ctx, st, "553834731", "us")
	if len(iaps) != 3 || len(iaps[2].Items) != 10 || len(privacy) != 3 {
		t.Fatalf("restored list/label not recorded: %d IAP lists, %d labels", len(iaps), len(privacy))
	}
	// The standalone writer cannot tell an empty list from a hidden one.
	if changed, err := RecordIAPs(ctx, st, "us", "553834731", nil, t1.Add(120*time.Hour)); err != nil || changed {
		t.Fatalf("RecordIAPs(empty) = %v %v, want no write", changed, err)
	}
}

// Ratings snapshots keep when Apple produced the counts; stores created
// before the column get it on open.
func TestRatingsSnapshotsKeepTheDataTime(t *testing.T) {
	st, path := openTestStore(t)
	ctx := context.Background()
	captured := time.Date(2026, 10, 23, 19, 28, 54, 0, time.UTC)
	asOf := captured.Add(-46415 * time.Second)
	if err := RecordRatings(ctx, st, RatingsSnapshot{CapturedAt: captured, DataAsOf: asOf, Storefront: "us", AppID: "324684580", RatingCount: 42719156, Source: SourceLookup}); err != nil {
		t.Fatal(err)
	}
	series, err := RatingsSeries(ctx, st, "324684580", "us", SourceLookup)
	if err != nil || len(series) != 1 || !series[0].DataAsOf.Equal(asOf) || !series[0].ObservedAt().Equal(asOf) {
		t.Fatalf("series = %+v %v", series, err)
	}
	// An old store without the column: the column is added, old rows read
	// their capture time as the observation time.
	for _, stmt := range []string{
		`DROP TABLE ratings_snapshots`,
		`CREATE TABLE ratings_snapshots (id INTEGER PRIMARY KEY, captured_at TEXT NOT NULL, storefront TEXT NOT NULL,
			app_id TEXT NOT NULL, average_rating REAL, rating_count INTEGER, histogram TEXT, version TEXT, source TEXT)`,
		`INSERT INTO ratings_snapshots (captured_at, storefront, app_id, rating_count, source) VALUES ('2026-10-20T00:00:00Z','us','1',10,'lookup')`,
	} {
		if _, err := st.DB().ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	st2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st2.Close() }()
	old, err := RatingsSeries(ctx, st2, "1", "", "")
	if err != nil || len(old) != 1 || !old[0].DataAsOf.IsZero() || !old[0].ObservedAt().Equal(time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("migrated row = %+v %v", old, err)
	}
	if err := RecordRatings(ctx, st2, RatingsSnapshot{CapturedAt: captured, DataAsOf: asOf, Storefront: "us", AppID: "1", RatingCount: 11}); err != nil {
		t.Fatalf("write after migration: %v", err)
	}
}
