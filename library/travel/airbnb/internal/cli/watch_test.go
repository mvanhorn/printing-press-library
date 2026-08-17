package cli

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/source/airbnb"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/store"
	_ "modernc.org/sqlite"
)

// TestClassifyWatchPrice is the F2 contract: a null/zero/negative scraped
// price is "no price data" (hasPrice=false) and is NEVER a hit, regardless of
// the threshold — so it can never trigger the exit-7 drop sentinel. A real
// positive price is a hit only when it is at or below a positive threshold.
func TestClassifyWatchPrice(t *testing.T) {
	cases := []struct {
		name         string
		price        float64
		maxPrice     float64
		wantHasPrice bool
		wantHit      bool
	}{
		{"zero price never hits even with threshold", 0, 350, false, false},
		{"negative price never hits", -1, 350, false, false},
		{"zero price zero threshold", 0, 0, false, false},
		{"positive price under threshold hits", 300, 350, true, true},
		{"positive price equal threshold hits", 350, 350, true, true},
		{"positive price over threshold no hit", 400, 350, true, false},
		{"positive price no threshold no hit", 300, 0, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hasPrice, hit := classifyWatchPrice(tc.price, tc.maxPrice)
			if hasPrice != tc.wantHasPrice || hit != tc.wantHit {
				t.Fatalf("classifyWatchPrice(%v,%v) = (hasPrice %v, hit %v), want (%v, %v)",
					tc.price, tc.maxPrice, hasPrice, hit, tc.wantHasPrice, tc.wantHit)
			}
		})
	}
}

// TestCollectScrapeTargets_FromWatchlistAndListings is the F4 store-read
// contract: the scrape resync gathers targets from both the watchlist (with
// saved dates) and previously-scraped listings, de-duplicated by URL with
// watchlist dates winning. An empty store yields zero targets so the caller
// reports honestly instead of calling any API.
func TestCollectScrapeTargets_FromWatchlistAndListings(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	// Empty store -> no targets, no warning.
	targets, warn, err := collectScrapeTargets(s)
	if err != nil {
		t.Fatalf("collect (empty): %v", err)
	}
	if warn != "" {
		t.Fatalf("empty-store warning = %q, want none", warn)
	}
	if len(targets) != 0 {
		t.Fatalf("empty-store targets = %d, want 0", len(targets))
	}

	// Watchlisted listing with dates.
	watchURL := "https://www.airbnb.com/rooms/111"
	if err := s.UpsertWatchlistItem(store.WatchlistItem{ListingURL: watchURL, ListingID: "111", Platform: "airbnb", Checkin: "2026-07-10", Checkout: "2026-07-14"}); err != nil {
		t.Fatalf("upsert watchlist: %v", err)
	}
	// A separately-scraped listing (no watchlist entry).
	knownData, _ := json.Marshal(&airbnb.Listing{ID: "222", URL: "https://www.airbnb.com/rooms/222"})
	if err := s.UpsertAirbnbListing(knownData); err != nil {
		t.Fatalf("upsert listing 222: %v", err)
	}
	// The watchlisted listing ALSO persisted as a listing row — must dedupe.
	dupData, _ := json.Marshal(&airbnb.Listing{ID: "111", URL: watchURL})
	if err := s.UpsertAirbnbListing(dupData); err != nil {
		t.Fatalf("upsert listing 111: %v", err)
	}

	targets, warn, err = collectScrapeTargets(s)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if warn != "" {
		t.Fatalf("warning = %q, want none", warn)
	}
	if len(targets) != 2 {
		t.Fatalf("targets = %d, want 2 (deduped by URL)", len(targets))
	}

	byURL := map[string]scrapeTarget{}
	for _, tg := range targets {
		byURL[tg.URL] = tg
	}
	w, ok := byURL[watchURL]
	if !ok {
		t.Fatalf("watch URL %q missing from targets", watchURL)
	}
	if w.Checkin != "2026-07-10" || w.Checkout != "2026-07-14" {
		t.Fatalf("watch target dates = (%q,%q), want (2026-07-10,2026-07-14) — watchlist dates must win", w.Checkin, w.Checkout)
	}
	if _, ok := byURL["https://www.airbnb.com/rooms/222"]; !ok {
		t.Fatalf("known listing 222 missing from targets")
	}
}
