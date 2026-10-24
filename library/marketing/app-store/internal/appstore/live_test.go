// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

//go:build live

package appstore

// Live acceptance for the foundation client. It sends exactly 5 sequential
// requests to Apple through the paced client and stops at the first error
// (a 403/429 trips the governor). It never runs in `go test ./...`:
//
//	go test -tags live -run TestLiveFoundation -v ./internal/appstore/

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"
)

type liveSink struct{ rows []Capture }

func (s *liveSink) RecordCapture(_ context.Context, c Capture) error {
	s.rows = append(s.rows, c)
	return nil
}

func TestLiveFoundation(t *testing.T) {
	sink := &liveSink{}
	c, err := New(Options{Country: "us", CacheDir: t.TempDir(), Capture: sink, Timeout: 45 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	stop := func(step string, err error) {
		if err != nil {
			t.Fatalf("%s: %v (requests sent: %d)", step, err, c.Requests())
		}
	}

	// 1. Lookup 324684580 (Spotify) in us.
	lk, err := c.Lookup(ctx, LookupQuery{IDs: []string{"324684580"}})
	stop("lookup", err)
	if len(lk.Results) != 1 {
		t.Fatalf("lookup results = %d missing = %v", len(lk.Results), lk.Missing)
	}
	app := lk.Results[0]
	t.Logf("LIVE lookup: trackId=%d trackName=%q version=%s userRatingCount=%d averageUserRating=%.4f price=%v %s raw_keys=%d url=%s",
		app.TrackID, app.TrackName, app.Version, app.UserRatingCount, app.AverageUserRating, app.Price, app.Currency, len(app.Raw), app.CanonicalURL())
	if !strings.HasPrefix(app.TrackName, "Spotify") || app.UserRatingCount < 1_000_000 {
		t.Fatalf("unexpected lookup result %+v", app)
	}

	// 2. Chart page: us iphone genre 36 (expect 25 lockups + 175 ids per segment).
	charts, err := c.ChartsPage(ctx, ChartsPageQuery{Country: "us", Platform: "iphone", GenreID: "36", Chart: "top-free"})
	stop("charts page", err)
	for _, seg := range charts.Segments {
		t.Logf("LIVE chart %s: lockups=%d remaining=%d ranked=%d #1=%s %q price=%q", seg.Chart, len(seg.Lockups), len(seg.RemainingIDs), len(seg.RankedIDs()), seg.Lockups[0].AdamID, seg.Lockups[0].Title, seg.Lockups[0].PriceText)
		if len(seg.Lockups) != 25 || len(seg.RemainingIDs) != 175 {
			t.Fatalf("segment %s = %d + %d, want 25 + 175", seg.Chart, len(seg.Lockups), len(seg.RemainingIDs))
		}
	}
	t.Logf("LIVE chart categories=%d (flat %d)", len(charts.Categories), len(charts.FlattenCategories()))

	// 3. Spotify product page via the canonical URL (no slug redirect).
	page, err := c.ProductPage(ctx, ProductPageQuery{URL: app.CanonicalURL()})
	stop("product page", err)
	sum := page.HistogramSum()
	diff := math.Abs(float64(sum-page.TotalRatings)) / math.Max(1, float64(page.TotalRatings))
	t.Logf("LIVE product: adamId=%s title=%q subtitle=%q ratingAverage=%.2f totalRatings=%d histogram=%v sum=%d diff=%.5f versions=%d newest=%+v privacyTypes=%d accessibilityDeclared=%v chart=%s",
		page.AdamID, page.Title, page.Subtitle, page.RatingAverage, page.TotalRatings, page.Histogram, sum, diff, len(page.Versions), page.Versions[0], len(page.Privacy.Types), page.Accessibility.Declared, page.ChartPosition.String())
	if len(page.Histogram) != 5 || diff > 0.01 {
		t.Fatalf("histogram %v sums to %d vs totalRatings %d", page.Histogram, sum, page.TotalRatings)
	}

	// 4. Reviews RSS: us 6448311069 page 1 mostrecent (expect 50).
	feed, err := c.ReviewsRSS(ctx, "us", "6448311069", 1, "recent")
	stop("reviews rss", err)
	ratings := map[int]int{}
	for _, r := range feed.Reviews {
		ratings[r.Rating]++
	}
	t.Logf("LIVE reviews: entries=%d outcome=%s oldest=%s newest=%s ratings=%v", len(feed.Reviews), feed.Outcome, feed.Oldest, feed.Newest, ratings)
	if len(feed.Reviews) != 50 {
		t.Fatalf("reviews = %d, want 50", len(feed.Reviews))
	}

	// 5. Search hints: "insta" in us (expect >= 5).
	hints, err := c.Hints(ctx, "us", "insta")
	stop("hints", err)
	t.Logf("LIVE hints: count=%d suggestions=%v", len(hints.Suggestions), hints.Suggestions)
	if len(hints.Suggestions) < 5 {
		t.Fatalf("hints = %v, want >= 5", hints.Suggestions)
	}

	t.Logf("LIVE requests sent=%d stats=%v captures=%d", c.Requests(), c.Stats(), len(sink.rows))
	for _, row := range sink.rows {
		t.Logf("LIVE capture: status=%d outcome=%s entries=%d cache-control=%q x-cache=%q host=%s url=%s", row.Status, row.Outcome, row.EntryCount, row.CacheControl, row.XCache, row.Host, row.URL)
	}
	if c.Requests() > 10 {
		t.Fatalf("request budget exceeded: %d", c.Requests())
	}
}

// TestLiveEmptyEnvelopes checks that Apple's real "nothing here" answers
// still satisfy the envelope rules (parser generation 2): they must come back
// verified-empty, not failed. Exactly 3 sequential requests, stopping at the
// first error:
//
//	go test -tags live -run TestLiveEmptyEnvelopes -v ./internal/appstore/
func TestLiveEmptyEnvelopes(t *testing.T) {
	sink := &liveSink{}
	c, err := New(Options{Country: "us", CacheDir: t.TempDir(), Capture: sink, Timeout: 45 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	defer func() {
		t.Logf("LIVE requests sent=%d stats=%v", c.Requests(), c.Stats())
		for _, row := range sink.rows {
			t.Logf("LIVE capture: status=%d outcome=%s entries=%d cache-control=%q host=%s url=%s error=%q", row.Status, row.Outcome, row.EntryCount, row.CacheControl, row.Host, row.URL, row.Error)
		}
	}()

	// 1. Lookup of an id no app has: {"resultCount":0,"results":[]}.
	lk, err := c.Lookup(ctx, LookupQuery{IDs: []string{"1"}})
	if err != nil {
		t.Fatalf("lookup id 1: %v", err)
	}
	t.Logf("LIVE lookup id=1: results=%d missing=%v outcome=%s fetchedAt=%s", len(lk.Results), lk.Missing, lk.Meta[0].Outcome, FormatTime(lk.FetchedAt))
	if len(lk.Missing) != 1 || lk.Meta[0].Outcome != OutcomeVerifiedEmpty {
		t.Fatalf("lookup id 1 = %+v", lk.Meta)
	}

	// 2. Customer-reviews RSS of that id: an empty feed that still carries
	// its id/updated/author metadata.
	feed, err := c.ReviewsRSS(ctx, "us", "1", 1, "recent")
	if err != nil {
		t.Fatalf("reviews rss id 1: %v", err)
	}
	t.Logf("LIVE reviews id=1: reviews=%d outcome=%s fetchedAt=%s", len(feed.Reviews), feed.Outcome, FormatTime(feed.FetchedAt))
	if feed.Outcome != OutcomeVerifiedEmpty {
		t.Fatalf("reviews id 1 outcome = %s", feed.Outcome)
	}

	// 3. Search hints for a term nothing starts with: the hints array is
	// present and empty.
	hints, err := c.Hints(ctx, "us", "zqxjvkwpqz")
	if err != nil {
		t.Fatalf("hints: %v", err)
	}
	t.Logf("LIVE hints zqxjvkwpqz: suggestions=%v outcome=%s", hints.Suggestions, hints.Meta.Outcome)
	if c.Requests() != 3 {
		t.Fatalf("requests = %d, want 3", c.Requests())
	}
}
