// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

// TestNovelTractionHelpWires smoke-tests that the traction command
// resolves at runtime and renders useful --help output.
func TestNovelTractionHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"traction", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("traction --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "traction <app> [app...]", "--period", "--countries", "never download counts"} {
		if !strings.Contains(help, want) {
			t.Fatalf("traction --help missing %q in output:\n%s", want, help)
		}
	}
}

func tractionRowsBy(rows []tractionRow, sf, period string) *tractionRow {
	for i := range rows {
		if rows[i].Storefront == sf && rows[i].Period == period {
			return &rows[i]
		}
	}
	return nil
}

func i64(v int64) *int64 { return &v }

func ptrStr(p *int64) string {
	if p == nil {
		return "null"
	}
	b, _ := json.Marshal(*p)
	return string(b)
}

func TestS3BRatingIncrements(t *testing.T) {
	day := func(n int, count int64) s3bDayCount {
		d := s3bAgo(s3bTestNow, 10-n, 0)
		return s3bDayCount{Day: d, At: d, Count: count}
	}
	cases := []struct {
		name  string
		days  []s3bDayCount
		want  []int64
		reset []bool
	}{
		{"rises count in full", []s3bDayCount{day(0, 100), day(1, 150), day(2, 175)}, []int64{50, 25}, []bool{false, false}},
		{"flat is zero", []s3bDayCount{day(0, 100), day(1, 100)}, []int64{0}, []bool{false}},
		{"drop within 1% is removed ratings, not negative", []s3bDayCount{day(0, 100000), day(1, 99001)}, []int64{0}, []bool{false}},
		{"drop over 1% is a reset counted from the reset day", []s3bDayCount{day(0, 100000), day(1, 98000), day(2, 98100)}, []int64{98000, 100}, []bool{true, false}},
		{"reset to zero", []s3bDayCount{day(0, 500), day(1, 0), day(2, 7)}, []int64{0, 7}, []bool{true, false}},
		{"single day has no increment", []s3bDayCount{day(0, 10)}, []int64{}, []bool{}},
	}
	for _, tc := range cases {
		got := s3bRatingIncrements(tc.days)
		if len(got) != len(tc.want) {
			t.Fatalf("%s: %d increments, want %d", tc.name, len(got), len(tc.want))
		}
		for i := range got {
			if got[i].New < 0 {
				t.Fatalf("%s: negative increment %d", tc.name, got[i].New)
			}
			if got[i].New != tc.want[i] || got[i].Reset != tc.reset[i] {
				t.Fatalf("%s: increment %d = %+v, want new=%d reset=%v", tc.name, i, got[i], tc.want[i], tc.reset[i])
			}
		}
	}
}

func TestS3BBucketsAndPeriods(t *testing.T) {
	at := s3bAgo(s3bTestNow, 0, 23)
	for period, want := range map[string]string{s3bPeriodDaily: "2026-10-24", s3bPeriodMonthly: "2026-10", s3bPeriodAnnual: "2026", s3bPeriodAll: "all-time"} {
		if got := s3bBucket(at, period); got != want {
			t.Fatalf("bucket %s = %s, want %s", period, got, want)
		}
	}
	for raw, want := range map[string]string{"": "monthly", "day": "daily", "Yearly": "annual", "all": "all-time", "ALL-TIME": "all-time"} {
		if got, err := s3bParsePeriod(raw); err != nil || got != want {
			t.Fatalf("period %q = %q %v, want %q", raw, got, err, want)
		}
	}
	if _, err := s3bParsePeriod("weekly"); ExitCode(err) != 2 {
		t.Fatalf("weekly period exit = %d, want 2", ExitCode(err))
	}
}

func TestTractionResetNeverNegativeDaily(t *testing.T) {
	testenv.Isolate(t)
	s3bPinClock(t)
	st, db := s3bNewStore(t)
	seedTractionScenario(t, st, s3bTestNow)

	out, _, err := runS3B(t, db, "traction", s3bSpotify, "--period", "daily", "--countries", "us", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var view tractionView
	decodeS3B(t, out, &view)
	for _, r := range view.Results {
		if r.NewRatings != nil && *r.NewRatings < 0 {
			t.Fatalf("negative newRatings in %+v", r)
		}
		if r.Storefront != "us" {
			t.Fatalf("--countries us leaked storefront %s", r.Storefront)
		}
	}
	checks := []struct {
		day   string
		new   *int64
		reset bool
		count int64
	}{
		{"2025-12-28", nil, false, 90000},
		{"2026-10-14", i64(10000), false, 100000},
		{"2026-10-15", i64(500), false, 100500},
		{"2026-10-16", i64(0), false, 100400},
		{"2026-10-17", i64(2000), true, 2000},
		{"2026-10-18", i64(600), false, 2600},
	}
	for _, c := range checks {
		r := tractionRowsBy(view.Results, "us", c.day)
		if r == nil {
			t.Fatalf("no us row for %s in %s", c.day, out)
		}
		if (c.new == nil) != (r.NewRatings == nil) || (c.new != nil && *c.new != *r.NewRatings) {
			t.Fatalf("%s newRatings = %v, want %v", c.day, ptrStr(r.NewRatings), ptrStr(c.new))
		}
		if r.ResetDetected != c.reset || r.RatingCount == nil || *r.RatingCount != c.count {
			t.Fatalf("%s row = %+v, want reset=%v count=%d", c.day, *r, c.reset, c.count)
		}
		if r.RatingsSource != "lookup" {
			t.Fatalf("%s ratingsSource = %q, want lookup (page row must be ignored)", c.day, r.RatingsSource)
		}
	}
	if r := tractionRowsBy(view.Results, "us", "2026-10-17"); r.BestRank != nil || r.DaysCharted != 0 || r.ChartDaysCaptured != 1 {
		t.Fatalf("10-17 chart fields = %+v, want captured but not charted", *r)
	}
}

func TestTractionPeriodRollups(t *testing.T) {
	testenv.Isolate(t)
	s3bPinClock(t)
	st, db := s3bNewStore(t)
	seedTractionScenario(t, st, s3bTestNow)

	run := func(period string) tractionView {
		out, _, err := runS3B(t, db, "traction", s3bSpotify, "--period", period, "--json")
		if err != nil {
			t.Fatalf("%s: %v", period, err)
		}
		var v tractionView
		decodeS3B(t, out, &v)
		return v
	}

	monthly := run("monthly")
	oct := tractionRowsBy(monthly.Results, "us", "2026-10")
	dec := tractionRowsBy(monthly.Results, "us", "2025-12")
	if oct == nil || dec == nil {
		t.Fatalf("monthly rows missing: %+v", monthly.Results)
	}
	if oct.NewRatings == nil || *oct.NewRatings != 13100 || oct.RatingDeltas != 5 || !oct.ResetDetected || *oct.RatingCount != 2600 {
		t.Fatalf("2026-10 = %+v, want newRatings 13100 over 5 deltas with a reset, count 2600", *oct)
	}
	if oct.BestRank == nil || *oct.BestRank != 3 || oct.BestRankChart != "iphone/6011/top-free" || oct.DaysCharted != 3 || oct.ChartDaysCaptured != 4 || oct.ReviewVolume != 3 {
		t.Fatalf("2026-10 chart/review fields = %+v", *oct)
	}
	if dec.NewRatings != nil || dec.ReviewVolume != 1 || *dec.RatingCount != 90000 {
		t.Fatalf("2025-12 = %+v, want baseline (null newRatings), 1 review, count 90000", *dec)
	}
	if gb := tractionRowsBy(monthly.Results, "gb", "2026-10"); gb == nil || gb.RatingsSource != "page" || *gb.NewRatings != 100 || !gb.ReviewCapTruncated || gb.ReviewVolume != 500 {
		t.Fatalf("gb 2026-10 = %+v, want page source, +100, 500 reviews flagged review_cap_truncated", gb)
	}
	if jp := tractionRowsBy(monthly.Results, "jp", "2026-10"); jp == nil || jp.NewRatings != nil || *jp.RatingCount != 3000 || jp.RatingDeltas != 0 {
		t.Fatalf("jp single capture = %+v, want null newRatings and count 3000", jp)
	}
	if oct.ReviewCapTruncated {
		t.Fatal("us has 4 reviews; review_cap_truncated must be false")
	}

	annual := run("annual")
	if r := tractionRowsBy(annual.Results, "us", "2026"); r == nil || *r.NewRatings != 13100 {
		t.Fatalf("annual 2026 = %+v", r)
	}
	if r := tractionRowsBy(annual.Results, "us", "2025"); r == nil || r.NewRatings != nil {
		t.Fatalf("annual 2025 = %+v, want baseline", r)
	}

	all := run("all-time")
	if len(all.Results) != 3 {
		t.Fatalf("all-time rows = %d, want one per storefront (gb, jp, us): %+v", len(all.Results), all.Results)
	}
	if r := tractionRowsBy(all.Results, "us", "all-time"); r == nil || *r.NewRatings != 13100 || !r.ResetDetected || r.DaysCharted != 3 || r.ReviewVolume != 4 {
		t.Fatalf("all-time us = %+v", r)
	}

	m := all.Meta
	if !m.Proxy || m.DownloadCounts {
		t.Fatalf("meta proxy=%v download_counts=%v, want true/false", m.Proxy, m.DownloadCounts)
	}
	if m.FirstCaptureAt == nil || *m.FirstCaptureAt != "2025-12-28T09:00:00Z" {
		t.Fatalf("first_capture_at = %v", m.FirstCaptureAt)
	}
	if strings.Join(m.StorefrontsWithData, ",") != "gb,jp,us" || !strings.Contains(m.Note, "not downloads") {
		t.Fatalf("meta = %+v", m)
	}
}

func TestTractionSinceWindowAndAgentEnvelope(t *testing.T) {
	testenv.Isolate(t)
	s3bPinClock(t)
	st, db := s3bNewStore(t)
	seedTractionScenario(t, st, s3bTestNow)

	out, _, err := runS3B(t, db, "traction", s3bSpotify, "--since", "7d", "--period", "monthly", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v tractionView
	decodeS3B(t, out, &v)
	us := tractionRowsBy(v.Results, "us", "2026-10")
	if us == nil || us.NewRatings != nil || *us.RatingCount != 2600 || *us.BestRank != 4 || us.DaysCharted != 1 || us.ReviewVolume != 0 {
		t.Fatalf("since 7d us = %+v, want one capture day (no increment), rank 4, no reviews", us)
	}
	if tractionRowsBy(v.Results, "gb", "2026-10") != nil {
		t.Fatal("gb has no captures inside the window and must not appear")
	}

	out, _, err = runS3B(t, db, "traction", s3bSpotify, "--period", "all-time", "--agent")
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		Meta    map[string]any   `json:"meta"`
		Results []map[string]any `json:"results"`
	}
	decodeS3B(t, out, &env)
	if env.Meta["proxy"] != true || env.Meta["download_counts"] != false || env.Meta["source"] != "local" {
		t.Fatalf("--agent meta = %v", env.Meta)
	}
	if len(env.Results) != 3 {
		t.Fatalf("--agent results = %d rows", len(env.Results))
	}
	for _, key := range []string{"newRatings", "reset_detected", "bestRank", "reviewVolume", "review_cap_truncated"} {
		if _, ok := env.Results[0][key]; !ok {
			t.Fatalf("--agent row lost %q: %v", key, env.Results[0])
		}
	}
}

func TestTractionEmptyAndNegative(t *testing.T) {
	testenv.Isolate(t)
	s3bPinClock(t)

	// No store file: exit 0, empty results, an honest note, hint on stderr.
	missing := filepath.Join(t.TempDir(), "absent.db")
	out, stderr, err := runS3B(t, missing, "traction", s3bSpotify, "--json")
	if err != nil {
		t.Fatalf("missing store must exit 0: %v", err)
	}
	var v tractionView
	decodeS3B(t, out, &v)
	if len(v.Results) != 0 || !strings.Contains(stderr, "no local mirror") || !strings.Contains(v.Meta.Note, "No local App Store snapshot store") {
		t.Fatalf("missing store: results=%d stderr=%q note=%q", len(v.Results), stderr, v.Meta.Note)
	}

	// Empty store and an app with no data: [] plus a note, never other apps' rows.
	st, db := s3bNewStore(t)
	out, _, err = runS3B(t, db, "traction", s3bSpotify, "--json")
	if err != nil {
		t.Fatal(err)
	}
	v = tractionView{}
	decodeS3B(t, out, &v)
	if len(v.Results) != 0 || !strings.Contains(v.Meta.Note, "No stored ratings") || strings.Join(v.Meta.AppsWithoutData, ",") != s3bSpotify {
		t.Fatalf("empty store view = %+v", v)
	}
	if !strings.Contains(out, `"results": []`) {
		t.Fatalf("empty results must print [] not null:\n%s", out)
	}
	seedTractionScenario(t, st, s3bTestNow)
	for _, tc := range []struct {
		args    []string
		missing string
		noData  string
	}{
		{[]string{"traction", "999999999", "--json"}, "999999999", ""},
		{[]string{"traction", "com.example.unknown", "--json"}, "com.example.unknown", ""},
		{[]string{"traction", s3bSpotify, "--countries", "fr", "--json"}, "", "fr"},
	} {
		out, _, err = runS3B(t, db, tc.args...)
		if err != nil {
			t.Fatal(err)
		}
		v = tractionView{}
		decodeS3B(t, out, &v)
		if len(v.Results) != 0 {
			t.Fatalf("%v must return no rows, got %+v", tc.args, v.Results)
		}
		if tc.missing != "" && strings.Join(v.Meta.AppsWithoutData, ",") != tc.missing {
			t.Fatalf("%v apps_without_data = %v", tc.args, v.Meta.AppsWithoutData)
		}
		if tc.noData != "" && strings.Join(v.Meta.StorefrontsWithoutData, ",") != tc.noData {
			t.Fatalf("%v storefronts_without_data = %v", tc.args, v.Meta.StorefrontsWithoutData)
		}
	}

	for _, args := range [][]string{
		{"traction", "--json"},
		{"traction", "not an app", "--json"},
		{"traction", s3bSpotify, "--period", "weekly"},
		{"traction", s3bSpotify, "--countries", "zz"},
		{"traction", s3bSpotify, "--since", "yesterday"},
	} {
		if _, _, err := runS3B(t, db, args...); ExitCode(err) != 2 {
			t.Fatalf("%v exit = %d (%v), want 2", args, ExitCode(err), err)
		}
	}
	out, _, err = runS3B(t, db, "traction", "--dry-run", "--json")
	if err != nil || !strings.Contains(out, `"dry_run":true`) {
		t.Fatalf("dry-run = %q %v", out, err)
	}
}

// A stale Lookup CDN copy captured just after a fresh one (Apple's edge can
// serve a copy up to 24 h old) holds a lower count: ordered by capture time
// it reads as a drop of more than 1% (a fake rating reset that counts the
// whole post-"reset" total as new); ordered by data time it is just an
// older count from the previous day.
func TestTractionOrdersCountsByDataTimeNotCaptureTime(t *testing.T) {
	testenv.Isolate(t)
	s3bPinClock(t)
	st, db := s3bNewStore(t)
	day := func(d, h, m int) time.Time { return time.Date(2026, 10, d, h, m, 0, 0, time.UTC) }
	snaps := []store.RatingsSnapshot{
		{CapturedAt: day(20, 10, 0), DataAsOf: day(20, 10, 0), RatingCount: 900},
		{CapturedAt: day(21, 23, 58), DataAsOf: day(21, 23, 58), RatingCount: 1000},
		// Captured minutes later, but a copy whose data is 12.9 h old.
		{CapturedAt: day(22, 0, 3), DataAsOf: day(21, 11, 10), RatingCount: 950},
		{CapturedAt: day(23, 12, 0), DataAsOf: day(23, 12, 0), RatingCount: 1200},
	}
	for i := range snaps {
		snaps[i].Storefront, snaps[i].AppID, snaps[i].Source = "us", "324684580", store.SourceLookup
	}
	if err := store.RecordRatings(context.Background(), st, snaps...); err != nil {
		t.Fatal(err)
	}
	out, _, err := runS3B(t, db, "traction", "324684580", "--countries", "us", "--period", "daily", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v tractionView
	decodeS3B(t, out, &v)
	var total int64
	for _, r := range v.Results {
		if r.ResetDetected {
			t.Fatalf("a stale CDN copy was read as a rating reset: %s", out)
		}
		if r.NewRatings != nil {
			total += *r.NewRatings
		}
	}
	if total != 300 {
		t.Fatalf("new ratings = %d, want 300 (900 -> 1000 -> 1200):\n%s", total, out)
	}
}
