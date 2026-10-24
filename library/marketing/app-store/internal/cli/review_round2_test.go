// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Regression tests for the round-2 code review (phase 4.95): defects the
// first round of fixes introduced.

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

// A throttle after a not-sold storefront is a stop (exit 7), not "not sold
// anywhere" (exit 3) for storefronts never asked.
func TestPricesThrottleAfterNotSoldExits7(t *testing.T) {
	f := newS2AFake(t)
	f.fail = func(host, path string, q url.Values) int {
		if host == "itunes.apple.com" && q.Get("country") == "gb" {
			return http.StatusTooManyRequests
		}
		return 0
	}
	res := f.run("prices", "324684580", "--countries", "cn,gb,us", "--json")
	if res.code != 7 || !strings.Contains(res.stdout, "not attempted: the run stopped at gb") {
		t.Fatalf("exit %d (%v), stdout %s; want 7 with us listed as not attempted", res.code, res.err, res.stdout)
	}
	for _, r := range f.requestLog() {
		if strings.Contains(r, "country=us") {
			t.Fatalf("the run continued after the throttle: %v", f.requestLog())
		}
	}
}

// An expired --timeout stops a fan-out like a throttle: the gathered rows
// print and later storefronts are not asked.
func TestRatingsDeadlineStopsTheFanOut(t *testing.T) {
	f := newS2AFake(t)
	f.fail = func(host, path string, q url.Values) int {
		if host == "itunes.apple.com" && q.Get("country") == "gb" {
			time.Sleep(700 * time.Millisecond)
		}
		return 0
	}
	res := f.run("ratings", "324684580", "--countries", "us,gb,cn", "--timeout", "400ms", "--json")
	if res.code == 0 || !strings.Contains(res.stdout, `"storefront": "us"`) {
		t.Fatalf("exit %d (%v), stdout %s; want a non-zero exit with the us row kept", res.code, res.err, res.stdout)
	}
	for _, r := range f.requestLog() {
		if strings.Contains(r, "country=cn") {
			t.Fatalf("the run continued after the deadline: %v", f.requestLog())
		}
	}
}

// Apps captured by the same batched Lookup share capture times; each app's
// pairs still count.
func TestChangesCountsCapturePairsPerApp(t *testing.T) {
	testenv.Isolate(t)
	s3bPinClock(t)
	st, db := s3bNewStore(t)
	for i := 0; i < 2; i++ {
		at := s3bTestNow.Add(time.Duration(i-2) * 24 * time.Hour)
		for _, id := range []int64{324684580, 553834731} {
			lk := appstore.LookupResult{TrackID: id, TrackName: "App", Version: "1.0", Currency: "USD"}
			if err := store.RecordLookupSnapshot(context.Background(), st, "us", lk, at); err != nil {
				t.Fatal(err)
			}
		}
	}
	out, _, err := runS3B(t, db, "changes", "--since", "all", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v changesView
	decodeS3B(t, out, &v)
	if v.Meta.Comparisons != 2 {
		t.Fatalf("comparisons = %d, want 2 (one pair per app)", v.Meta.Comparisons)
	}
}

// The 500-review cap is read from the capture log: a full page 10 of the
// reviews RSS means older reviews exist. It stays set when a later, smaller
// capture sees some of the same reviews again (their last-seen times move).
func TestTractionReviewCapFromTheCaptureLog(t *testing.T) {
	capture := func(t *testing.T, st *store.Store, page, entries int, at time.Time, idBase int) {
		revs := make([]appstore.Review, 0, entries)
		for i := 0; i < entries; i++ {
			n := idBase + i
			revs = append(revs, appstore.Review{ID: fmt.Sprintf("r%05d", n), Rating: 4, Title: "t", Body: "b", Author: "a",
				ReviewedAt: s3bTestNow.Add(-time.Duration(n) * time.Hour).Format(time.RFC3339)})
		}
		ctx := context.Background()
		if _, err := store.UpsertReviews(ctx, st, store.ReviewUpsert{Storefront: "us", AppID: "324684580", Sort: appstore.SortMostRecent,
			Source: store.SourceRSS, SeenAt: at, Reviews: revs}); err != nil {
			t.Fatal(err)
		}
		url := fmt.Sprintf("https://itunes.apple.com/us/rss/customerreviews/page=%d/id=324684580/sortby=mostrecent/json", page)
		if err := store.RecordCapture(ctx, st, appstore.Capture{CapturedAt: at, URL: url, Host: "itunes.apple.com", Status: 200,
			Outcome: appstore.OutcomeOK, EntryCount: entries}); err != nil {
			t.Fatal(err)
		}
	}
	fullCapture := func(t *testing.T, st *store.Store, start time.Time) {
		for page := 1; page <= 10; page++ {
			capture(t, st, page, 50, start.Add(time.Duration(page)*2*time.Second), (page-1)*50)
		}
	}
	for _, tc := range []struct {
		name string
		seed func(*testing.T, *store.Store)
		want bool
	}{
		{"one full capture", func(t *testing.T, st *store.Store) { fullCapture(t, st, s3bTestNow.Add(-time.Hour)) }, true},
		{"full capture, then a 1-page re-capture", func(t *testing.T, st *store.Store) {
			fullCapture(t, st, s3bTestNow.Add(-26*time.Hour))
			capture(t, st, 1, 30, s3bTestNow.Add(-2*time.Hour), 0)
		}, true},
		{"captures that never reach page 10", func(t *testing.T, st *store.Store) {
			for page := 1; page <= 6; page++ {
				capture(t, st, page, 50, s3bTestNow.Add(-26*time.Hour+time.Duration(page)*time.Second), (page-1)*50)
				capture(t, st, page, 50, s3bTestNow.Add(-2*time.Hour+time.Duration(page)*time.Second), 1000+(page-1)*50)
			}
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			s3bPinClock(t)
			st, db := s3bNewStore(t)
			tc.seed(t, st)
			out, _, err := runS3B(t, db, "traction", "324684580", "--json")
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(out, `"review_cap_truncated": true`); got != tc.want {
				t.Fatalf("review_cap_truncated = %v, want %v", got, tc.want)
			}
		})
	}
}

// The never-captured note counts only resolved apps without history; an
// unresolved bundle id does not make a captured app look uncaptured.
func TestChangesNeverCapturedNoteIgnoresUnresolvedRefs(t *testing.T) {
	testenv.Isolate(t)
	s3bPinClock(t)
	st, db := s3bNewStore(t)
	for i, v := range []string{"9.1", "9.2"} {
		lk := appstore.LookupResult{TrackID: 324684580, TrackName: "Spotify", Version: v, Currency: "USD"}
		if err := store.RecordLookupSnapshot(context.Background(), st, "us", lk, s3bTestNow.Add(time.Duration(i-2)*24*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	out, _, err := runS3B(t, db, "changes", "--since", "all", "--apps", "com.example.neverstored,324684580", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v changesView
	decodeS3B(t, out, &v)
	if len(v.Results) != 1 || strings.Contains(v.Meta.Note, "has been captured yet") {
		t.Fatalf("results %d, note %q", len(v.Results), v.Meta.Note)
	}
}
