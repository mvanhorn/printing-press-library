// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
	"time"
)

func TestRankHistoryHelpWires(t *testing.T) {
	r := s3aRun(t, "rank-history", "--help")
	if r.code != 0 || !strings.Contains(r.stdout, "rank-history <app> [flags]") {
		t.Fatalf("rank-history --help: exit %d\n%s", r.code, r.stdout)
	}
}

func TestRankHistorySeriesWithGap(t *testing.T) {
	db := s3aSeededDB(t)
	var v rankHistoryView
	s3aDecode(t, s3aRun(t, "rank-history", "553834731", "--db", db, "--chart", "top-grossing", "--genre", "36", "--country", "us", "--json"), &v)
	if v.Captures != 3 || v.Charted != 2 || len(v.Series) != 3 {
		t.Fatalf("captures %d charted %d series %d", v.Captures, v.Charted, len(v.Series))
	}
	ranks := []string{}
	for _, p := range v.Series {
		ranks = append(ranks, s3aRankText(p.Rank))
		if p.Source != "rss" || p.RowLimit != 100 {
			t.Fatalf("point source = %s/%d", p.Source, p.RowLimit)
		}
	}
	if strings.Join(ranks, ",") != "5,-,3" || v.Series[1].InChart {
		t.Fatalf("series ranks = %v (middle capture must be null, not invented)", ranks)
	}
	if v.FirstSeen == nil || v.FirstSeen.Rank != 5 || !v.FirstSeen.CapturedAt.Equal(s3aT0.Add(-24*time.Hour)) {
		t.Fatalf("firstSeen = %+v", v.FirstSeen)
	}
	if v.Peak == nil || v.Peak.Rank != 3 || v.LastSeen == nil || v.LastSeen.Rank != 3 || !v.LastSeen.CapturedAt.Equal(s3aT0.Add(24*time.Hour)) {
		t.Fatalf("peak %+v lastSeen %+v", v.Peak, v.LastSeen)
	}
	if v.Latest == nil || !v.Latest.InChart || *v.Latest.Rank != 3 || v.Title != "Candy Crush Saga" || v.Note != "" {
		t.Fatalf("latest %+v title %q note %q", v.Latest, v.Title, v.Note)
	}
	// --limit keeps the newest points but the summary still covers the window.
	v = rankHistoryView{}
	s3aDecode(t, s3aRun(t, "rank-history", "553834731", "--db", db, "--chart", "top-grossing", "--limit", "1", "--json"), &v)
	if len(v.Series) != 1 || *v.Series[0].Rank != 3 || v.FirstSeen.Rank != 5 || v.Captures != 3 {
		t.Fatalf("--limit 1: %+v", v)
	}
}

func TestRankHistoryDroppedOutNote(t *testing.T) {
	db := s3aSeededDB(t)
	var v rankHistoryView
	s3aDecode(t, s3aRun(t, "rank-history", "id431946152", "--db", db, "--genre", "games", "--json"), &v)
	if v.Charted != 1 || len(v.Series) != 2 || v.Latest == nil || v.Latest.InChart || v.Latest.Rank != nil {
		t.Fatalf("dropped-out app: %+v", v)
	}
	if !strings.Contains(v.Note, "not in the latest capture") || !strings.Contains(v.Note, "#4") {
		t.Fatalf("note = %q", v.Note)
	}
}

func TestRankHistoryNeverChartedIsEmptyWithElsewhere(t *testing.T) {
	db := s3aSeededDB(t)
	// The new game charted only in the Games top-free chart, never in All Apps.
	var v rankHistoryView
	s3aDecode(t, s3aRun(t, "rank-history", "6446308106", "--db", db, "--genre", "36", "--json"), &v)
	if len(v.Series) != 0 || v.Charted != 0 || v.FirstSeen != nil || v.Peak != nil {
		t.Fatalf("expected an empty series: %+v", v)
	}
	if !strings.Contains(v.Note, "no stored captures of us iphone 36") {
		t.Fatalf("note = %q", v.Note)
	}
	if len(v.Elsewhere) != 1 || v.Elsewhere[0].GenreID != "6014" || v.Elsewhere[0].BestRank != 4 || v.Elsewhere[0].Captures != 1 {
		t.Fatalf("elsewhere = %+v", v.Elsewhere)
	}
	// An app absent from every capture of an existing chart: empty series, counts kept.
	v = rankHistoryView{}
	s3aDecode(t, s3aRun(t, "rank-history", "324684580", "--db", db, "--genre", "6014", "--json"), &v)
	if len(v.Series) != 0 || v.Captures != 2 || !strings.Contains(v.Note, "not in any of the 2 stored captures") {
		t.Fatalf("absent app: %+v", v)
	}
	if len(v.Elsewhere) != 1 || v.Elsewhere[0].Chart != "top-grossing" || v.Elsewhere[0].BestRank != 2 {
		t.Fatalf("elsewhere for spotify = %+v", v.Elsewhere)
	}
}

func TestRankHistorySinceWindow(t *testing.T) {
	db := s3aSeededDB(t)
	var v rankHistoryView
	s3aDecode(t, s3aRun(t, "rank-history", "553834731", "--db", db, "--chart", "top-grossing", "--since", "2026-10-20", "--json"), &v)
	if v.Captures != 2 || v.Charted != 1 || v.FirstSeen.Rank != 3 || v.Since == nil {
		t.Fatalf("--since window: %+v", v)
	}
}

func TestRankHistoryEmptyAndErrors(t *testing.T) {
	r := s3aRun(t, "rank-history", "324684580", "--db", t.TempDir()+"/absent.db", "--json")
	var v rankHistoryView
	s3aDecode(t, r, &v)
	if len(v.Series) != 0 || v.Captures != 0 || !strings.Contains(v.Note, "no local chart snapshots") || !strings.Contains(r.stderr, "no local mirror at") {
		t.Fatalf("missing mirror: %+v / %q", v, r.stderr)
	}
	if !strings.Contains(r.stdout, `"series": []`) || !strings.Contains(r.stdout, `"peak": null`) {
		t.Fatalf("empty JSON shape:\n%s", r.stdout)
	}
	v = rankHistoryView{}
	s3aDecode(t, s3aRun(t, "rank-history", "324684580", "--db", s3aEmptyDB(t), "--json"), &v)
	if len(v.Series) != 0 || !strings.Contains(v.Note, "no stored captures") {
		t.Fatalf("empty store: %+v", v)
	}
	for _, args := range [][]string{
		{"rank-history", "--json"},
		{"rank-history", "__printing_press_invalid__"},
		{"rank-history", "1", "2"},
		{"rank-history", "324684580", "--since", "soon"},
		{"rank-history", "324684580", "--limit", "-2"},
	} {
		if r := s3aRun(t, args...); r.code != 2 {
			t.Errorf("%v: exit %d, want 2 (%v)", args, r.code, r.err)
		}
	}
	if r := s3aRun(t, "rank-history", "--dry-run", "--json"); r.code != 0 || !strings.Contains(r.stdout, `"dry_run":true`) {
		t.Fatalf("dry-run: %d %q", r.code, r.stdout)
	}
}
