// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

// Regression tests for the first public-library review (PR #2374).

// A --lang capture is printed but stays out of the listing history, so a
// translated listing never reads as an edit; its rating counts are kept.
func TestLangCapturesStayOutOfListingHistory(t *testing.T) {
	f := newS2AFake(t)
	const candy = "553834731"
	res := f.mustOK("app", candy, "--full", "--lang", "ja", "--json")
	if n := strings.Count(res.stderr, "not added to the listing history"); n != 1 {
		t.Fatalf("--lang note printed %d times, want once:\n%s", n, res.stderr)
	}
	f.mustOK("prices", candy, "--lang", "ja", "--json")

	ctx := context.Background()
	st := f.openStore()
	history := func() (snaps, iaps, privacy, events int) {
		t.Helper()
		s, err := store.AppSnapshots(ctx, st, candy, "us", time.Time{})
		if err != nil {
			t.Fatal(err)
		}
		i, err := store.IAPHistory(ctx, st, candy, "us")
		if err != nil {
			t.Fatal(err)
		}
		p, err := store.PrivacyHistory(ctx, st, candy, "us")
		if err != nil {
			t.Fatal(err)
		}
		e, err := store.EventsHistory(ctx, st, candy, "us")
		if err != nil {
			t.Fatal(err)
		}
		return len(s), len(i), len(p), len(e)
	}
	if s, i, p, e := history(); s+i+p+e != 0 {
		t.Fatalf("--lang captures wrote listing history: %d snapshots, %d IAP rows, %d privacy, %d events", s, i, p, e)
	}
	ratings, err := store.RatingsSeries(ctx, st, candy, "", "")
	if err != nil {
		t.Fatal(err)
	}
	pageRatings := 0
	for _, r := range ratings {
		if r.Source == store.SourcePage {
			pageRatings++
		}
	}
	if pageRatings == 0 {
		t.Fatalf("the --lang page's rating counts were not recorded: %+v", ratings)
	}

	// The same page in the storefront's default language enters the history.
	res = f.mustOK("app", candy, "--full", "--json")
	if strings.Contains(res.stderr, "not added to the listing history") {
		t.Fatalf("a default-language run printed the --lang note:\n%s", res.stderr)
	}
	if s, i, p, e := history(); s == 0 || i == 0 || p == 0 || e == 0 {
		t.Fatalf("default-language capture missing from the history: %d snapshots, %d IAP rows, %d privacy, %d events", s, i, p, e)
	}
}

// impact picks each storefront's rating source by coverage inside the event
// windows: a long older page series must not hide Lookup captures made
// around the event.
func TestImpactPicksRatingSourceByWindowCoverage(t *testing.T) {
	st, _ := s3bNewStore(t)
	ctx := context.Background()
	const app = "6448311069"
	day := func(d, hour int) time.Time { return time.Date(2026, 10, d, hour, 0, 0, 0, time.UTC) }
	var snaps []store.RatingsSnapshot
	for d := 1; d <= 10; d++ { // ten page captures, all before the windows
		snaps = append(snaps, store.RatingsSnapshot{CapturedAt: day(d, 12), Storefront: "us", AppID: app,
			AverageRating: 4.5, RatingCount: int64(1000 + 10*d), Source: store.SourcePage})
	}
	for i, d := range []int{17, 18, 21, 22} { // Lookup captures around the event
		snaps = append(snaps, store.RatingsSnapshot{CapturedAt: day(d, 12), Storefront: "us", AppID: app,
			AverageRating: 4.5, RatingCount: int64(2000 + 100*i), Source: store.SourceLookup})
	}
	if err := store.RecordRatings(ctx, st, snaps...); err != nil {
		t.Fatal(err)
	}
	rows, _, err := impactRows(ctx, st, app, day(20, 0), 7*24*time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := impactRowBy(rows, "us", impactMetricRatings)
	if r == nil || r.Series != store.SourceLookup || r.PointsBefore != 2 || r.PointsAfter != 2 || r.InsufficientData {
		t.Fatalf("ratings row = %+v, want the lookup series with 2 capture days on each side", r)
	}
}

// A --dry-run preview leaves local data alone: no store, no learn setup and
// no journal entry.
func TestDryRunLeavesLocalDataAlone(t *testing.T) {
	home := testenv.Isolate(t)
	root := RootCmd()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs([]string{"teach", "--query", "find items in category", "--resource-type", "items", "--resource", "GROUP-category", "--dry-run"})
	if err := root.Execute(); err != nil {
		t.Fatalf("teach --dry-run: %v\n%s", err, errb.String())
	}
	if _, err := os.Stat(defaultDBPath(appStoreCLIName)); !os.IsNotExist(err) {
		t.Fatalf("teach --dry-run created the store (stat err %v)", err)
	}
	files := func() int {
		t.Helper()
		n := 0
		_ = filepath.WalkDir(home, func(_ string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				n++
			}
			return nil
		})
		return n
	}
	before := files()
	journalInvocation(&rootFlags{dryRun: true}, root, root, nil, "", "")
	if after := files(); after != before {
		t.Fatalf("a --dry-run journal call wrote %d file(s)", after-before)
	}
	journalInvocation(&rootFlags{}, root, root, nil, "", "") // control: a real run journals
	if files() == before {
		t.Fatal("control: a non-dry-run journal call wrote nothing, so the dry-run check proves nothing")
	}
}

func TestArgsDryRun(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{[]string{"teach", "--dry-run"}, true},
		{[]string{"--dry-run=true", "teach"}, true},
		{[]string{"teach", "--dry-run=false"}, false},
		{[]string{"teach", "--dry-run=0"}, false},
		{[]string{"teach", "--query", "dry-run"}, false},
	} {
		if got := argsDryRun(tc.args); got != tc.want {
			t.Errorf("argsDryRun(%q) = %v, want %v", tc.args, got, tc.want)
		}
	}
}
