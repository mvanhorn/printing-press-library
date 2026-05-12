// Copyright 2026 kjuju600. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/seykota/internal/corpus"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := OpenWithContext(context.Background(), filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("OpenWithContext: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.EnsureCorpus(context.Background()); err != nil {
		t.Fatalf("EnsureCorpus: %v", err)
	}
	return s
}

func sampleDocs() []corpus.Doc {
	return []corpus.Doc{
		{ID: "tt/2007/Jul/01-31/default.html", Source: corpus.SourceFAQ, URL: "https://www.seykota.com/tt/2007/Jul/01-31/default.html",
			Title: "Ed's FAQ Jul 01-31, 2007", Year: "2007", Month: "Jul", MonthN: 7, Range: "01-31", Ord: 1,
			Contributors: []string{"Dave Druz", "Sam Q"}, Body: "A question about heat and pyramiding. Reply: mind your stops."},
		{ID: "tt/2019/Nov/01-30/default.html", Source: corpus.SourceFAQ, URL: "https://www.seykota.com/tt/2019/Nov/01-30/default.html",
			Title: "Ed's FAQ Nov 01-30, 2019", Year: "2019", Month: "Nov", MonthN: 11, Range: "01-30", Ord: 0,
			Contributors: []string{"Dave Druz"}, Body: "More on portfolio heat and diversification across markets."},
		{ID: "tribe/TSP/EA/index.htm", Source: corpus.SourceTSP, URL: "https://www.seykota.com/tribe/TSP/EA/index.htm",
			Title: "EA Crossover", Slug: "EA", Updated: "October 26, 2005", Ord: 0,
			Body: "The exponential average crossover system: go long when the fast EMA crosses above the slow EMA."},
		{ID: "tribe/TSP/SR/index.htm", Source: corpus.SourceTSP, URL: "https://www.seykota.com/tribe/TSP/SR/index.htm",
			Title: "SR", Slug: "SR", Updated: "October 26, 2005", Ord: 1,
			Body: "The support and resistance system: breakouts of recent highs and lows."},
		{ID: "tribe/risk/index.htm", Source: corpus.SourceRisk, URL: "https://www.seykota.com/tribe/risk/index.htm",
			Title: "Risk Management", Ord: 0,
			Body: "Risk is the possibility of loss. The Kelly Formula: K = W - (1-W)/R. The Uncle Point. The Lake Ratio."},
	}
}

func TestEnsureCorpusAndReplace(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if !s.CorpusEmpty(ctx) {
		t.Fatalf("fresh corpus should be empty")
	}
	n, err := s.ReplaceCorpus(ctx, sampleDocs())
	if err != nil {
		t.Fatalf("ReplaceCorpus: %v", err)
	}
	if n != 5 {
		t.Errorf("inserted %d; want 5", n)
	}
	if s.CorpusEmpty(ctx) {
		t.Errorf("corpus should be non-empty after ReplaceCorpus")
	}
	if c, _ := s.CorpusCount(""); c != 5 {
		t.Errorf("CorpusCount('') = %d; want 5", c)
	}
	if c, _ := s.CorpusCount("faq"); c != 2 {
		t.Errorf("CorpusCount('faq') = %d; want 2", c)
	}
	if c, _ := s.CorpusCount("tsp"); c != 2 {
		t.Errorf("CorpusCount('tsp') = %d; want 2", c)
	}
	// replace again with a subset — old rows should be gone
	n2, err := s.ReplaceCorpus(ctx, sampleDocs()[:1])
	if err != nil {
		t.Fatalf("ReplaceCorpus (subset): %v", err)
	}
	if n2 != 1 {
		t.Errorf("re-inserted %d; want 1", n2)
	}
	if c, _ := s.CorpusCount(""); c != 1 {
		t.Errorf("after re-replace CorpusCount('') = %d; want 1", c)
	}
}

func TestSearchCorpus(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if _, err := s.ReplaceCorpus(ctx, sampleDocs()); err != nil {
		t.Fatal(err)
	}
	hits, err := s.SearchCorpus("heat", SearchOpts{})
	if err != nil {
		t.Fatalf("SearchCorpus: %v", err)
	}
	if len(hits) < 2 {
		t.Errorf("search 'heat' returned %d hits; want >= 2", len(hits))
	}
	for _, h := range hits {
		if h.Snippet == "" || h.URL == "" {
			t.Errorf("hit missing snippet/url: %+v", h)
		}
	}
	// filter by source
	tspHits, _ := s.SearchCorpus("crossover OR resistance", SearchOpts{Source: "tsp"})
	for _, h := range tspHits {
		if h.Source != "tsp" {
			t.Errorf("--source=tsp returned a %s hit", h.Source)
		}
	}
	// filter by year
	yHits, _ := s.SearchCorpus("heat", SearchOpts{Source: "faq", Year: "2007"})
	for _, h := range yHits {
		if h.Year != "2007" {
			t.Errorf("--year=2007 returned a %s hit", h.Year)
		}
	}
	// empty query is an error
	if _, err := s.SearchCorpus("   ", SearchOpts{}); err == nil {
		t.Errorf("empty query should error")
	}
	// nonsense query -> no hits, no error
	none, err := s.SearchCorpus("zxcvbnmqwerty", SearchOpts{})
	if err != nil {
		t.Errorf("unexpected error for no-match query: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("expected 0 hits for nonsense query, got %d", len(none))
	}
}

func TestLookups(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if _, err := s.ReplaceCorpus(ctx, sampleDocs()); err != nil {
		t.Fatal(err)
	}
	d, err := s.FAQByYearMonth("2007", "Jul")
	if err != nil {
		t.Fatalf("FAQByYearMonth: %v", err)
	}
	if d.ID != "tt/2007/Jul/01-31/default.html" {
		t.Errorf("got %q", d.ID)
	}
	// case-insensitive + 3-letter prefix
	if d2, err := s.FAQByYearMonth("2019", "NOVEMBER"); err != nil || d2.Month != "Nov" {
		t.Errorf("FAQByYearMonth(2019,NOVEMBER) = %v, %v", d2, err)
	}
	// month-number fallback
	if d3, err := s.FAQByYearMonth("2007", "7"); err != nil || d3.MonthN != 7 {
		t.Errorf("FAQByYearMonth(2007,7) = %v, %v", d3, err)
	}
	if _, err := s.FAQByYearMonth("1999", "Jan"); err == nil {
		t.Errorf("expected error for absent month")
	}
	ts, err := s.TSPBySlug("ea")
	if err != nil || ts.Slug != "EA" {
		t.Errorf("TSPBySlug(ea) = %v, %v", ts, err)
	}
	if _, err := s.TSPBySlug("NOPE"); err == nil {
		t.Errorf("expected error for absent slug")
	}
	r, err := s.RiskDoc()
	if err != nil || r.Source != corpus.SourceRisk {
		t.Errorf("RiskDoc = %v, %v", r, err)
	}
	years, _ := s.FAQYears()
	if len(years) != 2 || years[0] != "2007" || years[1] != "2019" {
		t.Errorf("FAQYears = %v; want [2007 2019]", years)
	}
	faq, _ := s.ListDocs("faq")
	if len(faq) != 2 {
		t.Errorf("ListDocs(faq) = %d; want 2", len(faq))
	}
	// faq ordered newest first
	if faq[0].Year != "2019" {
		t.Errorf("ListDocs(faq) not newest-first: %q", faq[0].Year)
	}
}

func TestContributors(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if _, err := s.ReplaceCorpus(ctx, sampleDocs()); err != nil {
		t.Fatal(err)
	}
	all, err := s.Contributors("")
	if err != nil {
		t.Fatalf("Contributors: %v", err)
	}
	// Dave Druz appears in both FAQ months -> 2; Sam Q in one -> 1
	var druz, sam int
	for _, c := range all {
		switch c.Name {
		case "Dave Druz":
			druz = c.Months
		case "Sam Q":
			sam = c.Months
		}
	}
	if druz != 2 || sam != 1 {
		t.Errorf("contributor counts: Druz=%d (want 2), Sam Q=%d (want 1); all=%v", druz, sam, all)
	}
	// sorted: most months first
	if len(all) > 1 && all[0].Months < all[1].Months {
		t.Errorf("Contributors not sorted by month count desc: %v", all)
	}
	// filtered: months listed
	filt, _ := s.Contributors("druz")
	if len(filt) != 1 || filt[0].Months != 2 || len(filt[0].When) != 2 {
		t.Errorf("Contributors('druz') = %v", filt)
	}
}

func TestReadOnlyQuery(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if _, err := s.ReplaceCorpus(ctx, sampleDocs()); err != nil {
		t.Fatal(err)
	}
	cols, rows, err := s.ReadOnlyQuery(ctx, "SELECT source, COUNT(*) AS n FROM corpus GROUP BY source ORDER BY source", 100)
	if err != nil {
		t.Fatalf("ReadOnlyQuery: %v", err)
	}
	if len(cols) != 2 || cols[0] != "source" {
		t.Errorf("cols = %v", cols)
	}
	if len(rows) != 3 {
		t.Errorf("rows = %v; want 3 (faq, risk, tsp)", rows)
	}
	// rejects writes
	for _, bad := range []string{
		"INSERT INTO corpus(id,source,url,body,fetched_at) VALUES('x','x','x','x','x')",
		"UPDATE corpus SET body='x'",
		"DELETE FROM corpus",
		"DROP TABLE corpus",
		"PRAGMA user_version",
		"SELECT 1; SELECT 2",
		"WITH x AS (SELECT 1) DELETE FROM corpus",
	} {
		if _, _, err := s.ReadOnlyQuery(ctx, bad, 100); err == nil {
			t.Errorf("ReadOnlyQuery accepted a forbidden query: %q", bad)
		}
	}
}
