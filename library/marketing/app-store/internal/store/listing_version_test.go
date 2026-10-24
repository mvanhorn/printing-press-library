// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package store

// A product page and the Lookup row fetched with it can describe different
// versions: Apple's Lookup CDN serves per-URL copies up to 24 h old. Live on
// 2026-10-23, `app 324684580` stored a page+lookup snapshot with the stale
// copy's 9.1.92 (data as of 06:35:19Z) although the page fetched at 21:53:30Z
// listed 9.1.94, and the page ratings row paired the page's fresh total with
// 9.1.92.

import (
	"context"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
)

func TestRecordProductPageTakesTheNewerPagesVersion(t *testing.T) {
	ctx := context.Background()
	fetch := time.Date(2026, 10, 23, 21, 53, 30, 0, time.UTC)
	spotify := func(t *testing.T) appstore.ProductPage {
		page := fixturePage(t, "product_spotify_us.html") // newest entry 9.1.92, 2026-10-19T13:57:56Z
		page.FetchedAt, page.DataAsOf = fetch, fetch
		return page
	}
	lookup := func(version, released string, dataAsOf time.Time) appstore.LookupResult {
		return appstore.LookupResult{
			WrapperType: "software", TrackID: 324684580, TrackName: "Spotify: Music and Podcasts", BundleID: "com.spotify.client",
			Version: version, CurrentVersionReleaseDate: released, Currency: "USD", FormattedPrice: "Free",
			UserRatingCount: 42719156, AverageUserRating: 4.77, FetchedAt: fetch.Add(-time.Second), DataAsOf: dataAsOf,
			TrackViewURL: "https://apps.apple.com/us/app/spotify-music-and-podcasts/id324684580?uo=4",
		}
	}

	t.Run("stale Lookup copy: the page's newer version wins", func(t *testing.T) {
		st, _ := openTestStore(t)
		page := spotify(t)
		if page.Versions[0].Version != "9.1.92" {
			t.Fatalf("fixture newest version = %s", page.Versions[0].Version)
		}
		stale := lookup("9.1.90", "2026-10-08T07:37:17Z", fetch.Add(-15*time.Hour))
		if err := RecordProductPage(ctx, st, "us", page, stale); err != nil {
			t.Fatal(err)
		}
		snaps, err := AppSnapshots(ctx, st, "324684580", "us", time.Time{})
		if err != nil || len(snaps) != 1 {
			t.Fatalf("snapshots = %v %d", err, len(snaps))
		}
		s := snaps[0]
		if s.Version != "9.1.92" || s.Fields.Version != "9.1.92" || s.Fields.VersionReleasedAt != "2026-10-19T13:57:56Z" {
			t.Fatalf("snapshot version = %q / %q released %q, want the page's 9.1.92", s.Version, s.Fields.Version, s.Fields.VersionReleasedAt)
		}
		if s.Fields.VersionSource != "page" || s.Fields.LookupVersion != "9.1.90" || s.Fields.LookupDataAsOf != "2026-10-23T06:53:30Z" {
			t.Fatalf("snapshot provenance = source %q lookupVersion %q lookupDataAsOf %q", s.Fields.VersionSource, s.Fields.LookupVersion, s.Fields.LookupDataAsOf)
		}
		ratings, err := RatingsSeries(ctx, st, "324684580", "us", SourcePage)
		if err != nil || len(ratings) != 1 || ratings[0].Version != "9.1.92" || ratings[0].RatingCount != page.TotalRatings {
			t.Fatalf("page ratings = %v %+v, want the page's total with the page's version 9.1.92", err, ratings)
		}
	})

	t.Run("fresher Lookup row keeps its version; page ratings keep the page's", func(t *testing.T) {
		st, _ := openTestStore(t)
		page := spotify(t)
		page.DataAsOf = fetch.Add(-2 * time.Hour) // an old page copy
		fresh := lookup("9.1.94", "2026-10-23T17:59:53Z", fetch)
		if err := RecordProductPage(ctx, st, "us", page, fresh); err != nil {
			t.Fatal(err)
		}
		snaps, _ := AppSnapshots(ctx, st, "324684580", "us", time.Time{})
		if len(snaps) != 1 || snaps[0].Version != "9.1.94" || snaps[0].Fields.VersionSource != "" || snaps[0].Fields.LookupVersion != "" {
			t.Fatalf("snapshot = %+v", snaps)
		}
		ratings, _ := RatingsSeries(ctx, st, "324684580", "us", SourcePage)
		if len(ratings) != 1 || ratings[0].Version != "9.1.92" {
			t.Fatalf("page ratings = %+v, want the page's own newest version 9.1.92", ratings)
		}
	})

	t.Run("same version: nothing changes", func(t *testing.T) {
		st, _ := openTestStore(t)
		page := spotify(t)
		if err := RecordProductPage(ctx, st, "us", page, lookup("9.1.92", "2026-10-19T13:57:56Z", fetch.Add(-15*time.Hour))); err != nil {
			t.Fatal(err)
		}
		snaps, _ := AppSnapshots(ctx, st, "324684580", "us", time.Time{})
		if len(snaps) != 1 || snaps[0].Version != "9.1.92" || snaps[0].Fields.VersionSource != "" {
			t.Fatalf("snapshot = %+v", snaps)
		}
	})
}
