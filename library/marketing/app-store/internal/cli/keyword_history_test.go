// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

func TestKeywordHistoryHelpWires(t *testing.T) {
	r := s3aRun(t, "keyword-history", "--help")
	if r.code != 0 || !strings.Contains(r.stdout, "keyword-history <term> [flags]") || !strings.Contains(r.stdout, "--serp") {
		t.Fatalf("keyword-history --help: exit %d\n%s", r.code, r.stdout)
	}
}

func TestKeywordHistoryAppSeries(t *testing.T) {
	db := s3aSeededDB(t)
	var v keywordHistoryView
	s3aDecode(t, s3aRun(t, "keyword-history", "Spotify", "--app", "324684580", "--db", db, "--json"), &v)
	if v.Term != "spotify" || v.Captures != 3 || v.FoundIn != 3 || len(v.Series) != 3 {
		t.Fatalf("view = %+v", v)
	}
	ranks := []string{}
	for _, p := range v.Series {
		ranks = append(ranks, s3aRankText(p.Rank))
	}
	// Captures 1-2 targeted the app (stored rank); capture 3 was a plain
	// search whose stored result list puts the app first.
	if strings.Join(ranks, ",") != "1,2,1" {
		t.Fatalf("ranks = %v", ranks)
	}
	if v.Series[0].Depth != 50 || v.Series[2].Depth != 3 || v.Series[2].ResultCount != 48 {
		t.Fatalf("depths = %d / %d", v.Series[0].Depth, v.Series[2].Depth)
	}
	if v.BestRank == nil || v.BestRank.Rank != 1 || !v.BestRank.CapturedAt.Equal(s3aT0) {
		t.Fatalf("bestRank = %+v", v.BestRank)
	}
	if v.LatestRank == nil || *v.LatestRank != 1 || v.Change == nil || *v.Change != 1 {
		t.Fatalf("latest %v change %v", v.LatestRank, v.Change)
	}
	for _, p := range v.Series {
		if len(p.Top) != 0 {
			t.Fatal("top lists must only appear with --serp")
		}
	}
}

func TestKeywordHistorySerpReplay(t *testing.T) {
	db := s3aSeededDB(t)
	var v keywordHistoryView
	s3aDecode(t, s3aRun(t, "keyword-history", "spotify", "--serp", "2", "--db", db, "--json"), &v)
	if len(v.Series) != 3 || v.BestRank != nil || v.LatestRank != nil {
		t.Fatalf("serp view = %+v", v)
	}
	first := v.Series[0]
	if len(first.Top) != 2 || first.Top[0].AppID != "324684580" || first.Top[0].Title != "Spotify: Music and Podcasts" || first.Top[1].Position != 2 || first.Top[1].Title != "" {
		t.Fatalf("serp replay = %+v", first.Top)
	}
	// Without --app each point keeps the capture's own target.
	if first.AppID != "324684580" || *first.Rank != 1 || v.Series[2].AppID != "" || v.Series[2].Rank != nil {
		t.Fatalf("capture targets = %+v / %+v", first, v.Series[2])
	}
}

func TestKeywordHistoryNotFoundIsEmpty(t *testing.T) {
	db := s3aSeededDB(t)
	var v keywordHistoryView
	s3aDecode(t, s3aRun(t, "keyword-history", "podcast", "app", "--app", "553834731", "--db", db, "--json"), &v)
	if v.Term != "podcast app" || v.Captures != 1 || v.FoundIn != 0 || len(v.Series) != 0 || v.BestRank != nil {
		t.Fatalf("not-found view = %+v", v)
	}
	if !strings.Contains(v.Note, "not found in any of the 1 stored captures") || !strings.Contains(v.Note, "200 results deep") {
		t.Fatalf("note = %q", v.Note)
	}
	// An app that appears in no capture at all.
	v = keywordHistoryView{}
	s3aDecode(t, s3aRun(t, "keyword-history", "spotify", "--app", "999999999", "--db", db, "--json"), &v)
	if len(v.Series) != 0 || v.FoundIn != 0 || v.Captures != 3 {
		t.Fatalf("absent app = %+v", v)
	}
}

func TestKeywordHistoryMixedSourcesAndFilters(t *testing.T) {
	db := s3aSeededDB(t)
	var v keywordHistoryView
	s3aDecode(t, s3aRun(t, "keyword-history", "music", "--app", "324684580", "--db", db, "--json"), &v)
	if v.Captures != 2 || !strings.Contains(v.Note, "sources itunes, web") {
		t.Fatalf("mixed view = %+v", v)
	}
	v = keywordHistoryView{}
	s3aDecode(t, s3aRun(t, "keyword-history", "music", "--app", "324684580", "--source", "web", "--db", db, "--json"), &v)
	if v.Captures != 1 || *v.LatestRank != 4 || v.Note != "" || v.Change != nil {
		t.Fatalf("web-only view = %+v", v)
	}
	v = keywordHistoryView{}
	s3aDecode(t, s3aRun(t, "keyword-history", "spotify", "--app", "324684580", "--country", "gb", "--db", db, "--json"), &v)
	if v.Captures != 0 || len(v.Series) != 0 || !strings.Contains(v.Note, `no stored keyword captures for "spotify" in gb`) {
		t.Fatalf("other storefront view = %+v", v)
	}
	v = keywordHistoryView{}
	s3aDecode(t, s3aRun(t, "keyword-history", "spotify", "--app", "324684580", "--limit", "1", "--db", db, "--json"), &v)
	if len(v.Series) != 1 || v.Captures != 3 || v.BestRank.Rank != 1 {
		t.Fatalf("--limit view = %+v", v)
	}
}

func TestKeywordHistoryEmptyAndErrors(t *testing.T) {
	r := s3aRun(t, "keyword-history", "spotify", "--db", t.TempDir()+"/absent.db", "--json")
	var v keywordHistoryView
	s3aDecode(t, r, &v)
	if len(v.Series) != 0 || !strings.Contains(v.Note, "no local keyword captures") || !strings.Contains(r.stderr, "no local mirror at") {
		t.Fatalf("missing mirror = %+v / %q", v, r.stderr)
	}
	v = keywordHistoryView{}
	s3aDecode(t, s3aRun(t, "keyword-history", "spotify", "--db", s3aEmptyDB(t), "--json"), &v)
	if len(v.Series) != 0 || v.Captures != 0 || !strings.Contains(v.Note, "no stored keyword captures") {
		t.Fatalf("empty store = %+v", v)
	}
	for _, args := range [][]string{
		{"keyword-history", "--json"},
		{"keyword-history", "spotify", "--serp", "500"},
		{"keyword-history", "spotify", "--source", "bing"},
		{"keyword-history", "spotify", "--platform", "toaster"},
	} {
		if r := s3aRun(t, args...); r.code != 2 {
			t.Errorf("%v: exit %d, want 2 (%v)", args, r.code, r.err)
		}
	}
}
