// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

// Local history commands accept a bundle id and resolve it from the stored
// listing snapshots (store.AppIDForBundle), without a live lookup.
func TestLocalCommandsAcceptBundleIDs(t *testing.T) {
	testenv.Isolate(t)
	ctx := context.Background()
	db := filepath.Join(t.TempDir(), "store.db")
	st, err := store.OpenWithContext(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	lk := appstore.LookupResult{TrackID: 324684580, TrackName: "Spotify: Music and Podcasts", BundleID: "com.spotify.client", Version: "9.1.90", Currency: "USD", FormattedPrice: "Free"}
	for i, version := range []string{"9.1.90", "9.1.92"} {
		lk.Version = version
		if err := store.RecordLookupSnapshot(ctx, st, "us", lk, t0.Add(time.Duration(i)*24*time.Hour)); err != nil {
			t.Fatal(err)
		}
		if err := store.RecordChart(ctx, st, store.ChartSnapshot{
			CapturedAt: t0.Add(time.Duration(i) * 24 * time.Hour), Storefront: "us", Platform: "iphone", GenreID: "36", Chart: "top-free",
			Source: store.SourceWebChart, RowLimit: 200, Rows: []store.ChartRow{{Rank: 1, AppID: "1"}, {Rank: 3 - i, AppID: "324684580"}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.UpsertReviews(ctx, st, store.ReviewUpsert{Storefront: "us", AppID: "324684580", Sort: appstore.SortMostRecent, Source: store.SourceRSS, SeenAt: t0,
		Reviews: []appstore.Review{{ID: "r1", Rating: 1, Version: "9.1.92", Title: "Sample title 1", Body: "Sample body that crashes 1", Author: "reviewer-1", ReviewedAt: appstore.FormatTime(t0)}}}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordKeywordRank(ctx, st, store.KeywordRank{CapturedAt: t0, Storefront: "us", Platform: "iphone", Term: "spotify", Source: store.SourceSearchAPI, AppID: "324684580", Rank: 1, ResultCount: 10, TopIDs: []string{"324684580"}}); err != nil {
		t.Fatal(err)
	}
	// An app captured only through a page record (no listing snapshot): its
	// IAP list changed between two captures.
	for i, price := range []string{"$0.99", "$1.99"} {
		page := appstore.ProductPage{AdamID: "553834731", Title: "Candy Crush Saga", InAppPurchases: []appstore.IAPItem{{Position: 1, Name: "Gold Bars", Price: price}}, HasInAppPurchases: true, FetchedAt: t0.Add(time.Duration(i) * 24 * time.Hour)}
		if err := store.RecordProductPage(ctx, st, "us", page, appstore.LookupResult{}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	var hist rankHistoryView
	for _, bundle := range []string{"com.spotify.client", "COM.Spotify.Client"} {
		hist = rankHistoryView{}
		s3aDecode(t, s3aRun(t, "rank-history", bundle, "--db", db, "--json"), &hist)
		if hist.AppID != "324684580" || hist.Charted != 2 || hist.Peak == nil || hist.Peak.Rank != 2 {
			t.Fatalf("rank-history %s = %+v", bundle, hist)
		}
	}
	var watch watchListingView
	s3aDecode(t, s3aRun(t, "watch-listing", "com.spotify.client", "--source", "lookup", "--db", db, "--json"), &watch)
	if watch.AppID != "324684580" || !watch.Changed {
		t.Fatalf("watch-listing by bundle id = %+v", watch)
	}
	var digest reviewDigestView
	s3aDecode(t, s3aRun(t, "review-digest", "com.spotify.client", "--db", db, "--json"), &digest)
	if digest.AppID != "324684580" || digest.Total != 1 {
		t.Fatalf("review-digest by bundle id = %+v", digest)
	}
	var kw keywordHistoryView
	s3aDecode(t, s3aRun(t, "keyword-history", "spotify", "--app", "com.spotify.client", "--db", db, "--json"), &kw)
	if kw.AppID != "324684580" || kw.FoundIn != 1 {
		t.Fatalf("keyword-history --app bundle id = %+v", kw)
	}
	var hits []searchHit
	s3aDecode(t, s3aRun(t, "search", "crashes", "--app", "com.spotify.client", "--db", db, "--json"), &hits)
	if len(hits) != 1 || hits[0].AppID != "324684580" {
		t.Fatalf("search --app bundle id = %+v", hits)
	}
	var ch changesView
	s3aDecode(t, s3aRun(t, "changes", "--since", "all", "--apps", "com.spotify.client", "--db", db, "--json"), &ch)
	if r := changeRowsOf(ch.Results, "version"); len(r) != 1 || r[0].AppID != "324684580" {
		t.Fatalf("changes --apps bundle id = %+v", ch.Results)
	}

	// changes without --apps covers every app with stored history, including
	// the app known only from page records (no listing snapshot).
	ch = changesView{}
	s3aDecode(t, s3aRun(t, "changes", "--since", "all", "--db", db, "--json"), &ch)
	if strings.Join(ch.Meta.Apps, ",") != "324684580,553834731" {
		t.Fatalf("changes default app set = %v, want both apps", ch.Meta.Apps)
	}
	if r := changeRowsOf(ch.Results, "iap"); len(r) != 1 || r[0].AppID != "553834731" || r[0].From != "$0.99" || r[0].To != "$1.99" {
		t.Fatalf("IAP change of the page-only app = %+v", r)
	}

	// A bundle id the store has never seen is an empty local result, not an
	// error.
	r := s3aRun(t, "rank-history", "com.example.unknown", "--db", db, "--json")
	hist = rankHistoryView{}
	s3aDecode(t, r, &hist)
	if len(hist.Series) != 0 || !strings.Contains(hist.Note, "not in the local store") {
		t.Fatalf("unknown bundle id = %+v", hist)
	}
}
