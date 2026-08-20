package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/source/airbnb"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/source/vrbo"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/store"
)

// scrapeSyncResource names the synthetic resource that re-scrapes everything
// the store already knows (watchlist entries + previously-scraped listings)
// and persists fresh price snapshots. It is the default sync target: this
// CLI has no paginated REST API to mirror, so "sync" means "refresh what
// you've already seen", not "page an upstream collection".
const scrapeSyncResource = "scrape"

// scrapeTarget is one listing to re-scrape during a scrape sync, carrying the
// dates (when known from a watchlist entry) so the refreshed snapshot lands
// under the same (listing_id, platform, checkin, checkout) key the watch /
// wishlist-diff readers expect.
type scrapeTarget struct {
	URL      string
	Checkin  string
	Checkout string
}

// collectScrapeTargets gathers the listings to re-scrape from the store:
// every watchlist entry (with its saved dates) plus every previously-scraped
// airbnb_listing, de-duplicated by URL (watchlist dates win). Returns an
// empty slice when the store knows nothing — the caller reports that honestly
// instead of hitting any API.
//
// The second return value is a non-fatal warning: a read failure on the
// previously-scraped listings is surfaced (not swallowed) so the caller can
// emit a sync_warning, while still returning the watchlist targets it already
// gathered. Only a ListWatchlist failure is fatal — without it there is no
// safe target set at all.
func collectScrapeTargets(db *store.Store) ([]scrapeTarget, string, error) {
	seen := map[string]bool{}
	var targets []scrapeTarget

	watch, err := db.ListWatchlist(0)
	if err != nil {
		return nil, "", fmt.Errorf("reading watchlist: %w", err)
	}
	for _, w := range watch {
		if w.ListingURL == "" || seen[w.ListingURL] {
			continue
		}
		seen[w.ListingURL] = true
		targets = append(targets, scrapeTarget{URL: w.ListingURL, Checkin: w.Checkin, Checkout: w.Checkout})
	}

	// Previously-scraped listings (search/get/cheapest persisted these).
	// They carry no saved dates, so they refresh the listing record and only
	// produce a snapshot if the SSR happens to expose a dateless price.
	//
	// Do not swallow a read error here: dropping the listing table silently
	// would halve sync coverage (watchlist-only) with zero signal. Return the
	// watchlist targets we already have plus a warning the caller surfaces.
	known, err := db.List("airbnb_listing", 0)
	if err != nil {
		return targets, fmt.Sprintf("reading stored listings: %v", err), nil
	}
	for _, raw := range known {
		var obj struct {
			URL string `json:"url"`
			ID  string `json:"id"`
		}
		if json.Unmarshal(raw, &obj) != nil {
			continue
		}
		url := obj.URL
		if url == "" && obj.ID != "" {
			url = "https://www.airbnb.com/rooms/" + obj.ID
		}
		if url == "" || seen[url] {
			continue
		}
		seen[url] = true
		targets = append(targets, scrapeTarget{URL: url})
	}

	return targets, "", nil
}

// runScrapeSync re-scrapes every store-known listing and persists fresh
// snapshots via the PATCH persistence path inside computeCheapest. It reports
// honestly (no API call, no error) when the store is empty, and emits the
// same sync_start / sync_progress / sync_complete / sync_summary event shapes
// the API-mirror sync path uses so existing tooling keeps working. The return
// value is the number of listings for which a fresh price snapshot was
// written. Per-listing scrape failures (including disabled VRBO entries) are
// downgraded to sync_warning events and never fail the whole run.
func runScrapeSync(ctx context.Context, db *store.Store) (int, error) {
	started := time.Now()

	if !humanFriendly {
		fmt.Fprintf(os.Stderr, `{"event":"sync_start","resource":"%s"}`+"\n", scrapeSyncResource)
	}

	targets, warn, err := collectScrapeTargets(db)
	if err != nil {
		return 0, err
	}
	if warn != "" {
		// Non-fatal: previously-scraped listings could not be read, but the
		// watchlist targets are still synced. Surface it so coverage loss is
		// visible to both humans and machine consumers.
		if humanFriendly {
			fmt.Fprintf(os.Stderr, "Sync: warning: %s\n", warn)
		} else {
			fmt.Fprintf(os.Stderr, `{"event":"sync_warning","resource":"%s","reason":"listing_read_failed","message":"%s"}`+"\n", scrapeSyncResource, jsonEscape(warn))
		}
	}

	if len(targets) == 0 {
		// Honest empty-store report: nothing to re-scrape, and there is no
		// public collection endpoint to page. The user must seed the store
		// first (watch add / airbnb_listing search|get|cheapest) or sync the
		// auth-gated wishlist (--resources airbnb_wishlist).
		msg := "store is empty: nothing to re-scrape. Seed it with 'watch add', 'airbnb_listing search|get', or 'cheapest', or sync the auth-gated wishlist via '--resources airbnb_wishlist' (requires 'auth login --chrome')."
		if humanFriendly {
			fmt.Fprintf(os.Stderr, "Sync: %s\n", msg)
		} else {
			fmt.Fprintf(os.Stderr, `{"event":"sync_warning","resource":"%s","reason":"empty_store","message":"%s"}`+"\n", scrapeSyncResource, msg)
			fmt.Fprintf(os.Stderr, `{"event":"sync_complete","resource":"%s","total":0,"duration_ms":%d}`+"\n", scrapeSyncResource, time.Since(started).Milliseconds())
		}
		return 0, nil
	}

	priced := 0
	visited := 0
	warned := 0
	for _, t := range targets {
		select {
		case <-ctx.Done():
			return priced, ctx.Err()
		default:
		}
		visited++
		// PATCH: computeCheapest persists the listing + host + (price>0) snapshot
		// through db via the persistence path. We count a "priced" refresh when a real
		// positive platform total came back, so the summary reflects how many
		// listings actually produced a new snapshot rather than how many were
		// merely revisited.
		ch, scErr := computeCheapest(ctx, t.URL, cheapestParams{Checkin: t.Checkin, Checkout: t.Checkout, store: db})
		if scErr != nil {
			reason := scErr.Error()
			if vrbo.IsDisabled(scErr) {
				reason = "vrbo_disabled"
			}
			warned++
			if humanFriendly {
				fmt.Fprintf(os.Stderr, "  %s: warning: %s\n", t.URL, reason)
			} else {
				fmt.Fprintf(os.Stderr, `{"event":"sync_warning","resource":"%s","url":"%s","reason":"scrape_failed","message":"%s"}`+"\n", scrapeSyncResource, t.URL, jsonEscape(reason))
			}
			continue
		}
		if price, _ := firstPlatformTotals(ch); price > 0 {
			priced++
		}
		if !humanFriendly {
			fmt.Fprintf(os.Stderr, `{"event":"sync_progress","resource":"%s","fetched":%d}`+"\n", scrapeSyncResource, visited)
		}
	}

	elapsed := time.Since(started)
	if humanFriendly {
		fmt.Fprintf(os.Stderr, "Sync complete: re-scraped %d known listing(s), %d fresh price snapshot(s), %d warning(s) (%.1fs)\n", len(targets), priced, warned, elapsed.Seconds())
	} else {
		fmt.Fprintf(os.Stderr, `{"event":"sync_complete","resource":"%s","total":%d,"duration_ms":%d}`+"\n", scrapeSyncResource, priced, elapsed.Milliseconds())
		fmt.Fprintf(os.Stderr, `{"event":"sync_summary","total_records":%d,"resources":1,"success":1,"warned":%d,"errored":0,"duration_ms":%d}`+"\n", priced, warned, elapsed.Milliseconds())
	}
	return priced, nil
}

// runWishlistAuthSync runs the auth-gated wishlist sync (Airbnb GraphQL
// persisted query behind a cookie jar). It is reachable only when the user
// explicitly asks for it via `sync --resources airbnb_wishlist`; the default
// sync never touches it. Kept as a documented, auth-required path so the
// wishlist capability is not lost when the default became a scrape re-sync.
// Mirrors the standalone `wishlist sync` command (wishlist.go) but reports
// through sync's event/summary shape.
func runWishlistAuthSync(ctx context.Context, db *store.Store) error {
	if !humanFriendly {
		fmt.Fprintf(os.Stderr, `{"event":"sync_start","resource":"airbnb_wishlist"}`+"\n")
	}
	wishlists, err := airbnb.WishlistList(ctx)
	if err != nil {
		// Auth-gated: surface as an auth error (exit 4) with the actionable
		// hint rather than a bare 404 — the endpoint requires imported
		// cookies.
		return authErr(fmt.Errorf("wishlist sync requires authentication; run 'airbnb-pp-cli auth login --chrome': %w", err))
	}
	for _, w := range wishlists {
		b, _ := json.Marshal(w)
		_ = db.UpsertAirbnbWishlist(b)
	}
	if humanFriendly {
		fmt.Fprintf(os.Stderr, "Sync complete: %d wishlist(s)\n", len(wishlists))
	} else {
		fmt.Fprintf(os.Stderr, `{"event":"sync_complete","resource":"airbnb_wishlist","total":%d}`+"\n", len(wishlists))
	}
	return nil
}

// jsonEscape escapes a string for safe embedding inside the hand-built JSON
// sync event lines (which use fmt rather than json.Marshal for the envelope).
func jsonEscape(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	// Strip the surrounding quotes json.Marshal adds.
	return string(b[1 : len(b)-1])
}
