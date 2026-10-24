// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

func TestS3BParseSinceAndWindow(t *testing.T) {
	now := s3bTestNow
	cases := []struct {
		raw  string
		want time.Time
	}{
		{"", time.Time{}},
		{"all", time.Time{}},
		{"7d", now.Add(-7 * 24 * time.Hour)},
		{"2w", now.Add(-14 * 24 * time.Hour)},
		{"36h", now.Add(-36 * time.Hour)},
		{"2026-10-01", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
		{"2026-10-01T05:30:00Z", time.Date(2026, 10, 1, 5, 30, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		got, err := s3bParseSince("since", tc.raw, now)
		if err != nil || !got.Equal(tc.want) {
			t.Fatalf("since %q = %v %v, want %v", tc.raw, got, err, tc.want)
		}
	}
	for _, bad := range []string{"-3d", "0d", "soon", "7 days"} {
		if _, err := s3bParseSince("since", bad, now); ExitCode(err) != 2 {
			t.Fatalf("since %q exit = %d, want 2", bad, ExitCode(err))
		}
	}
	if d, err := s3bParseWindow("window", "7d"); err != nil || d != 7*24*time.Hour {
		t.Fatalf("window 7d = %v %v", d, err)
	}
	for _, bad := range []string{"", "0d", "-1d", "401d", "a week"} {
		if _, err := s3bParseWindow("window", bad); ExitCode(err) != 2 {
			t.Fatalf("window %q exit = %d, want 2", bad, ExitCode(err))
		}
	}
}

func TestS3BStorefrontScope(t *testing.T) {
	testenv.Isolate(t)
	root := RootCmd()
	cmd, _, err := root.Find([]string{"breakouts"})
	if err != nil {
		t.Fatal(err)
	}
	scope, err := s3bStorefrontScope(cmd, &rootFlags{country: "us"}, "us,GB,uk")
	if err != nil || strings.Join(scope.list(), ",") != "gb,us" || !scope.has("gb") || scope.has("jp") {
		t.Fatalf("scope = %v %v", scope.list(), err)
	}
	for _, raw := range []string{"", "all", "ALL"} {
		if scope, err := s3bStorefrontScope(cmd, &rootFlags{country: "us"}, raw); err != nil || scope != nil || !scope.has("zz-anything") {
			t.Fatalf("%q must mean every storefront with data: %v %v", raw, scope, err)
		}
	}
	if _, err := s3bStorefrontScope(cmd, &rootFlags{}, "us,zz"); ExitCode(err) != 2 {
		t.Fatalf("bad storefront exit = %d", ExitCode(err))
	}
	if err := root.PersistentFlags().Set("country", "pk"); err != nil {
		t.Fatal(err)
	}
	if scope, err := s3bStorefrontScope(cmd, &rootFlags{country: "pk"}, ""); err != nil || strings.Join(scope.list(), ",") != "pk" {
		t.Fatalf("explicit --country = %v %v", scope.list(), err)
	}
}

func TestS3BChangedFieldsMedianAndFeatured(t *testing.T) {
	a := store.AppSnapshot{Title: "A", Subtitle: "one", Version: "1.0", Price: 0, ScreenshotsHash: "s1"}
	b := a
	b.Subtitle, b.Price, b.ScreenshotsHash = "two", 0.99, "s2"
	if got := strings.Join(s3bChangedFields(a, b), ","); got != "subtitle,price,screenshots" {
		t.Fatalf("changed fields = %s", got)
	}
	if got := s3bChangedFields(a, a); len(got) != 0 {
		t.Fatalf("identical snapshots differ: %v", got)
	}
	if s3bMedian([]float64{40, 171, 20}) != 40 || s3bMedian([]float64{15, 25}) != 20 || s3bMedian(nil) != 0 {
		t.Fatal("median wrong")
	}
	t1, t2 := s3bAgo(s3bTestNow, 2, 9), s3bAgo(s3bTestNow, 1, 9)
	caps := s3bFeaturedCaptures([]store.FeaturedRow{
		{CapturedAt: t2, Storefront: "us", Platform: "iphone", StoryID: "x", StoryTitle: "Story X", AppID: "1"},
		{CapturedAt: t1, Storefront: "us", Platform: "iphone", StoryID: "y", StoryTitle: "Story Y"}, // a story with no apps still marks the capture
		{CapturedAt: t2, Storefront: "us", Platform: "iphone", StoryID: "z", StoryTitle: "Story Z", AppID: "1"},
	})
	if len(caps) != 2 || !caps[0].At.Equal(t1) || len(caps[0].Apps) != 0 || strings.Join(caps[1].Apps["1"], "|") != "Story X|Story Z" {
		t.Fatalf("featured captures = %+v", caps)
	}
	if s3bTruncate("abcdef", 3) != "abc…" || s3bTruncate("abc", 3) != "abc" {
		t.Fatal("truncate wrong")
	}
}

// TestS3BDogfoodProbes mirrors the live-dogfood probes: the error-path
// sentinel is rejected (exit 2) by the app commands, and every command
// carries the annotations the matrix reads.
func TestS3BDogfoodProbes(t *testing.T) {
	testenv.Isolate(t)
	s3bPinClock(t)
	_, db := s3bNewStore(t)
	for _, name := range []string{"traction", "impact"} {
		if _, _, err := runS3B(t, db, name, "__printing_press_invalid__"); ExitCode(err) != 2 {
			t.Fatalf("%s sentinel exit = %d (%v), want 2", name, ExitCode(err), err)
		}
	}
	root := RootCmd()
	for _, name := range []string{"traction", "impact", "changes", "keyword-share", "breakouts"} {
		cmd, _, err := root.Find([]string{name})
		if err != nil || cmd.Name() != name {
			t.Fatalf("%s not registered: %v", name, err)
		}
		a := cmd.Annotations
		if a["pp:data-source"] != "local" || a["mcp:read-only"] != "true" || a["pp:novel-scaffold"] != "" || !strings.Contains(a["pp:happy-args"], "324684580") && name != "breakouts" {
			t.Fatalf("%s annotations = %v", name, a)
		}
		if a["pp:happy-args"] == "" || !strings.HasPrefix(cmd.Example, "  app-store-pp-cli "+name) || cmd.Long == "" {
			t.Fatalf("%s help metadata incomplete: example=%q", name, cmd.Example)
		}
	}
}

func TestBreakoutsCountriesAllMeansStoredStorefronts(t *testing.T) {
	testenv.Isolate(t)
	s3bPinClock(t)
	st, db := s3bNewStore(t)
	seedBreakoutsScenario(t, st, s3bTestNow)
	out, _, err := runS3B(t, db, "breakouts", "--genre", "6014", "--countries", "all", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v breakoutsView
	decodeS3B(t, out, &v)
	if len(v.Meta.StorefrontsSkipped) != 3 || len(v.Meta.StorefrontsCompared) != 4 || len(v.Results) != 2 {
		t.Fatalf("--countries all should behave like the default scope: %+v", v.Meta)
	}
}

// One stray Lookup capture must not hide a dense series of page captures.
func TestPickRatingSourceByCoverage(t *testing.T) {
	day := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	page := make([]store.RatingsSnapshot, 0, 10)
	for i := 0; i < 10; i++ {
		page = append(page, store.RatingsSnapshot{CapturedAt: day.AddDate(0, 0, i), Source: store.SourcePage})
	}
	lookup := []store.RatingsSnapshot{{CapturedAt: day.AddDate(0, 0, -10), Source: store.SourceLookup}}
	if got := s3bPickRatingSource(map[string][]store.RatingsSnapshot{store.SourceLookup: lookup, store.SourcePage: page}); got != store.SourcePage {
		t.Fatalf("1 lookup day vs 10 page days picked %q, want page", got)
	}
	lookup = append(lookup, page[0])
	lookup[1].Source = store.SourceLookup
	if got := s3bPickRatingSource(map[string][]store.RatingsSnapshot{store.SourceLookup: lookup, store.SourcePage: page[:2]}); got != store.SourceLookup {
		t.Fatalf("equal coverage picked %q, want lookup (exact counts)", got)
	}
}
