// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

// Every local history command prints a stderr hint when the newest capture
// it read is older than --max-age (0 disables it); stdout, and so the JSON
// output, is the same either way.
func TestLocalCommandsHintWhenDataIsOlderThanMaxAge(t *testing.T) {
	testenv.Isolate(t)
	s3bPinClock(t)
	st, db := s3bNewStore(t)
	ctx := context.Background()
	t0 := time.Date(2020, 6, 1, 9, 0, 0, 0, time.UTC) // long before any test clock
	for i := 0; i < 2; i++ {
		at := t0.Add(time.Duration(i) * 24 * time.Hour)
		for _, sf := range []string{"us", "gb"} {
			if err := store.RecordChart(ctx, st, store.ChartSnapshot{CapturedAt: at, Storefront: sf, Platform: "iphone", GenreID: "36", Chart: "top-free",
				Source: store.SourceWebChart, RowLimit: 200, Rows: []store.ChartRow{{Rank: 1, AppID: "1"}, {Rank: 40 - 30*i, AppID: "324684580"}}}); err != nil {
				t.Fatal(err)
			}
		}
		lk := appstore.LookupResult{TrackID: 324684580, TrackName: "Spotify", Version: []string{"9.1.90", "9.1.92"}[i], UserRatingCount: int64(1000 + 100*i)}
		if err := store.RecordLookupSnapshot(ctx, st, "us", lk, at); err != nil {
			t.Fatal(err)
		}
		if err := store.RecordRatings(ctx, st, store.RatingsSnapshot{CapturedAt: at, Storefront: "us", AppID: "324684580", RatingCount: lk.UserRatingCount, Source: store.SourceLookup}); err != nil {
			t.Fatal(err)
		}
		if err := store.RecordKeywordRank(ctx, st, store.KeywordRank{CapturedAt: at, Storefront: "us", Platform: "iphone", Term: "spotify", Source: store.SourceSearchAPI, AppID: "324684580", Rank: 1 + i, ResultCount: 10, TopIDs: []string{"324684580"}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.UpsertReviews(ctx, st, store.ReviewUpsert{Storefront: "us", AppID: "324684580", Sort: appstore.SortMostRecent, Source: store.SourceRSS, SeenAt: t0,
		Reviews: []appstore.Review{{ID: "r1", Rating: 2, Version: "9.1.90", Title: "Sample title 1", Body: "Sample body 1", Author: "reviewer-1", ReviewedAt: appstore.FormatTime(t0)}}}); err != nil {
		t.Fatal(err)
	}

	commands := [][]string{
		{"movers"},
		{"rank-history", "324684580"},
		{"keyword-history", "spotify"},
		{"watch-listing", "324684580", "--source", "lookup"},
		{"review-digest", "324684580"},
		{"search", "sample"},
		{"traction", "324684580", "--period", "all-time"},
		{"impact", "324684580", "--at", "2020-06-02"},
		{"changes", "--since", "all"},
		{"keyword-share", "spotify"},
		{"breakouts", "--since", "all"},
	}
	for _, args := range commands {
		staleOut, staleErr, err := runS3B(t, db, append(append([]string{}, args...), "--json", "--max-age", "1h")...)
		if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, staleErr)
		}
		if !strings.Contains(staleErr, "hint: the newest stored") || !strings.Contains(staleErr, "older than --max-age 1h0m0s") {
			t.Errorf("%v --max-age 1h: no staleness hint on stderr:\n%s", args, staleErr)
		}
		offOut, offErr, err := runS3B(t, db, append(append([]string{}, args...), "--json", "--max-age", "0")...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if strings.Contains(offErr, "hint: the newest stored") {
			t.Errorf("%v --max-age 0 still printed the hint:\n%s", args, offErr)
		}
		if offOut != staleOut {
			t.Errorf("%v: the hint changed stdout:\n%s\n---\n%s", args, staleOut, offOut)
		}
		_, freshErr, err := runS3B(t, db, append(append([]string{}, args...), "--json", "--max-age", "200000h")...)
		if err != nil || strings.Contains(freshErr, "hint: the newest stored") {
			t.Errorf("%v: data younger than --max-age got a hint (%v):\n%s", args, err, freshErr)
		}
	}
}

func TestStaleAgeFormat(t *testing.T) {
	for d, want := range map[time.Duration]string{
		45 * time.Minute:                 "45m",
		5*time.Hour + 12*time.Minute:     "5h12m",
		3*24*time.Hour + 4*time.Hour + 9: "3d4h",
	} {
		if got := staleAge(d); got != want {
			t.Errorf("staleAge(%v) = %q, want %q", d, got, want)
		}
	}
}
