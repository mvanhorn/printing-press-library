// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil/testenv"
)

// TestNovelImpactHelpWires smoke-tests that the impact command resolves at
// runtime and renders useful --help output.
func TestNovelImpactHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"impact", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("impact --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "impact <app>", "--at", "--window", "version:<v>"} {
		if !strings.Contains(help, want) {
			t.Fatalf("impact --help missing %q in output:\n%s", want, help)
		}
	}
}

func impactRowBy(rows []impactRow, sf, metric string) *impactRow {
	for i := range rows {
		if rows[i].Storefront == sf && rows[i].Metric == metric {
			return &rows[i]
		}
	}
	return nil
}

func wantFloat(t *testing.T, label string, got *float64, want float64) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("%s = %s, want %g", label, s3bOptFloat(got), want)
	}
}

func TestImpactLatestReleaseWindows(t *testing.T) {
	testenv.Isolate(t)
	s3bPinClock(t)
	st, db := s3bNewStore(t)
	seedCandyScenario(t, st, s3bTestNow)

	out, _, err := runS3B(t, db, "impact", s3bCandy, "--at", "latest", "--window", "7d", "--countries", "us", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v impactView
	decodeS3B(t, out, &v)
	ev := v.Meta.Event
	if ev == nil || ev.Kind != "version" || ev.Version != "1.301.0" || ev.At != "2026-10-17T10:00:00Z" {
		t.Fatalf("event = %+v, want version 1.301.0 at 2026-10-17T10:00:00Z", ev)
	}
	if v.Meta.Before.From != "2026-10-10T10:00:00Z" || v.Meta.After.To != "2026-10-24T10:00:00Z" || v.Meta.AfterComplete == nil || !*v.Meta.AfterComplete {
		t.Fatalf("windows = %+v %+v complete=%v", v.Meta.Before, v.Meta.After, v.Meta.AfterComplete)
	}
	rank := impactRowBy(v.Results, "us", impactMetricRank)
	if rank == nil || rank.Series != "iphone/7012/top-free" || rank.PointsBefore != 2 || rank.PointsAfter != 2 || rank.InsufficientData {
		t.Fatalf("rank row = %+v", rank)
	}
	wantFloat(t, "rank before", rank.Before, 15)
	wantFloat(t, "rank after", rank.After, 8)
	wantFloat(t, "rank delta", rank.Delta, -7)

	ratings := impactRowBy(v.Results, "us", impactMetricRatings)
	if ratings == nil || ratings.Series != "lookup" || ratings.PointsBefore != 3 || ratings.PointsAfter != 3 || ratings.InsufficientData {
		t.Fatalf("ratings row = %+v", ratings)
	}
	wantFloat(t, "ratings before", ratings.Before, 75)
	wantFloat(t, "ratings after", ratings.After, 150)
	wantFloat(t, "ratings delta", ratings.Delta, 75)

	stars := impactRowBy(v.Results, "us", impactMetricReviews)
	if stars == nil || stars.PointsBefore != 2 || stars.PointsAfter != 3 {
		t.Fatalf("stars row = %+v", stars)
	}
	wantFloat(t, "stars before", stars.Before, 3)
	wantFloat(t, "stars after", stars.After, 4.67)
	wantFloat(t, "stars delta", stars.Delta, 1.67)

	kw := impactRowBy(v.Results, "us", impactMetricKeywords)
	if kw == nil || kw.Series != "candy (itunes, iphone)" || kw.PointsBefore != 2 || kw.PointsAfter != 2 || kw.Detail != "found in 2 of 2 captures before and 1 of 2 after" {
		t.Fatalf("keyword row = %+v", kw)
	}
	wantFloat(t, "keyword before", kw.Before, 5)
	wantFloat(t, "keyword after", kw.After, 2)
	wantFloat(t, "keyword delta", kw.Delta, -3)

	if !v.Meta.CoMovementOnly {
		t.Fatal("co_movement_only must be true")
	}
	lower := strings.ToLower(v.Meta.Note + " " + out)
	for _, causal := range []string{"caused", "because of", "drove", "led to", "resulted in"} {
		if strings.Contains(lower, causal) {
			t.Fatalf("output uses causal wording %q:\n%s", causal, out)
		}
	}
}

func TestImpactInsufficientDataWithOneCapture(t *testing.T) {
	testenv.Isolate(t)
	s3bPinClock(t)

	// gb has a single rating capture: insufficient_data and no delta.
	st, db := s3bNewStore(t)
	seedCandyScenario(t, st, s3bTestNow)
	out, _, err := runS3B(t, db, "impact", s3bCandy, "--at", "latest", "--countries", "gb", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v impactView
	decodeS3B(t, out, &v)
	r := impactRowBy(v.Results, "gb", impactMetricRatings)
	if r == nil || !r.InsufficientData || r.Delta != nil || r.PointsBefore != 0 || r.PointsAfter != 1 || r.Before != nil || r.After != nil {
		t.Fatalf("gb ratings row = %+v, want insufficient_data with 0/1 points and no values", r)
	}
	for _, row := range v.Results {
		if row.Storefront != "gb" {
			t.Fatalf("--countries gb leaked %s", row.Storefront)
		}
		if !row.InsufficientData || row.Delta != nil {
			t.Fatalf("gb has one capture; every row must be insufficient_data: %+v", row)
		}
	}

	// A store holding exactly one capture of the app.
	st2, db2 := s3bNewStore(t)
	seedSingleCaptureScenario(t, st2, s3bTestNow)
	out, _, err = runS3B(t, db2, "impact", s3bSpotify, "--at", "latest", "--json")
	if err != nil {
		t.Fatal(err)
	}
	v = impactView{}
	decodeS3B(t, out, &v)
	if v.Meta.Event == nil || v.Meta.Event.Version != "9.1.92" {
		t.Fatalf("latest release from the snapshot's release date: %+v", v.Meta.Event)
	}
	if len(v.Results) == 0 {
		t.Fatalf("expected insufficient rows, got none:\n%s", out)
	}
	for _, row := range v.Results {
		if !row.InsufficientData || row.Delta != nil {
			t.Fatalf("one capture must be insufficient_data with no delta: %+v", row)
		}
	}
}

func TestImpactEventKinds(t *testing.T) {
	testenv.Isolate(t)
	s3bPinClock(t)
	st, db := s3bNewStore(t)
	seedCandyScenario(t, st, s3bTestNow)

	run := func(at string) impactView {
		out, _, err := runS3B(t, db, "impact", s3bCandy, "--at", at, "--json")
		if err != nil {
			t.Fatalf("--at %s: %v", at, err)
		}
		var v impactView
		decodeS3B(t, out, &v)
		return v
	}
	listing := run("listing")
	if ev := listing.Meta.Event; ev == nil || ev.Kind != "listing" || ev.At != "2026-10-22T08:00:00Z" || ev.Storefront != "us" ||
		strings.Join(ev.ChangedFields, ",") != "title,version,price" {
		t.Fatalf("listing event = %+v", listing.Meta.Event)
	}
	featured := run("featured")
	if ev := featured.Meta.Event; ev == nil || ev.Kind != "featured" || ev.At != "2026-10-18T09:00:00Z" || !strings.Contains(ev.Detail, "Game of the Day") {
		t.Fatalf("featured event = %+v", featured.Meta.Event)
	}
	version := run("version:1.300.0")
	if ev := version.Meta.Event; ev == nil || ev.At != "2026-10-10T10:00:00Z" || ev.Version != "1.300.0" {
		t.Fatalf("version event = %+v", version.Meta.Event)
	}
	date := run("2026-10-15")
	if ev := date.Meta.Event; ev == nil || ev.Kind != "date" || ev.At != "2026-10-15T00:00:00Z" {
		t.Fatalf("date event = %+v", date.Meta.Event)
	}
	unknown := run("version:9.9.9")
	if unknown.Meta.Event != nil || len(unknown.Results) != 0 || !strings.Contains(unknown.Meta.Note, "not in the stored history") || !strings.Contains(unknown.Meta.Note, "1.301.0") {
		t.Fatalf("unknown version = %+v", unknown.Meta)
	}
}

func TestImpactEmptyAndNegative(t *testing.T) {
	testenv.Isolate(t)
	s3bPinClock(t)
	st, db := s3bNewStore(t)

	out, _, err := runS3B(t, db, "impact", s3bSpotify, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v impactView
	decodeS3B(t, out, &v)
	if v.Meta.Event != nil || len(v.Results) != 0 || !strings.Contains(v.Meta.Note, "No stored version release") || !strings.Contains(out, `"results": []`) {
		t.Fatalf("empty store = %s", out)
	}

	seedCandyScenario(t, st, s3bTestNow)
	out, _, err = runS3B(t, db, "impact", "999999999", "--at", "2026-10-17", "--json")
	if err != nil {
		t.Fatal(err)
	}
	v = impactView{}
	decodeS3B(t, out, &v)
	if v.Meta.Event == nil || len(v.Results) != 0 || !strings.Contains(v.Meta.Note, "nothing to compare") {
		t.Fatalf("app without data must not borrow another app's rows: %s", out)
	}

	for _, args := range [][]string{
		{"impact", "--json"},
		{"impact", s3bCandy, s3bSpotify},
		{"impact", s3bCandy, "--at", "yesterday"},
		{"impact", s3bCandy, "--at", "version:"},
		{"impact", s3bCandy, "--window", "0d"},
		{"impact", s3bCandy, "--window", "soon"},
		{"impact", "bad app ref"},
	} {
		if _, _, err := runS3B(t, db, args...); ExitCode(err) != 2 {
			t.Fatalf("%v exit = %d (%v), want 2", args, ExitCode(err), err)
		}
	}
	if spec, err := parseImpactAt("2026-10-01T05:00:00Z"); err != nil || spec.Kind != "date" || appstore.FormatTime(spec.Date) != "2026-10-01T05:00:00Z" {
		t.Fatalf("RFC3339 --at = %+v %v", spec, err)
	}
}
