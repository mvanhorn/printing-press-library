package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/hostextract"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/source/airbnb"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/store"
)

// persistWarn reports a best-effort persistence failure without ever polluting
// the machine-mode event stream. In human mode it prints the familiar
// "warning: <msg>" line; in machine mode it emits a single structured
// persist_warning JSON event so consumers parsing stderr keep seeing exactly
// one JSON object per line (the same stream-purity contract scrape_sync.go's
// sync_* events honor). These helpers run on every default `sync` iteration via
// runScrapeSync -> computeCheapest, so an unguarded plain-text warning here
// would inject free-form text between the JSON sync events. id is optional
// context (listing id or host name) and is omitted from the event when empty.
func persistWarn(reason, id, msg string) {
	if humanFriendly {
		fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
		return
	}
	if id != "" {
		fmt.Fprintf(os.Stderr, `{"event":"persist_warning","reason":"%s","id":"%s","message":"%s"}`+"\n", jsonEscape(reason), jsonEscape(id), jsonEscape(msg))
		return
	}
	fmt.Fprintf(os.Stderr, `{"event":"persist_warning","reason":"%s","message":"%s"}`+"\n", jsonEscape(reason), jsonEscape(msg))
}

// openScrapeStore opens the local SQLite store for the best-effort
// persistence side-effect of the scrape commands (search/get/cheapest/
// compare/plan/watch). Returns (nil, nil) when the store cannot be opened
// so callers can persist-if-present without ever degrading the live scrape
// result: a store miss must never turn a successful scrape into an error.
// Callers own Close on a non-nil handle.
func openScrapeStore(ctx context.Context) *store.Store {
	db, err := store.OpenWithContext(ctx, defaultDBPath("airbnb-pp-cli"))
	if err != nil {
		persistWarn("store_unavailable", "", fmt.Sprintf("store unavailable, scrape not persisted: %v", err))
		return nil
	}
	return db
}

// persistAirbnbListing writes one scraped Airbnb listing into the local
// store, best-effort. It is a no-op when db is nil or the listing carries
// no extractable ID (Airbnb SSR search cards sometimes lack a stable id),
// and it swallows/stderr-warns store errors so a successful scrape is never
// degraded by a persistence failure. Mirrors the write-through-cache
// pattern: persistence is additive, never load-bearing for the response.
func persistAirbnbListing(db *store.Store, l *airbnb.Listing) {
	if db == nil || l == nil || l.ID == "" {
		return
	}
	data, err := json.Marshal(l)
	if err != nil {
		return
	}
	if err := db.UpsertAirbnbListing(data); err != nil {
		persistWarn("persist_listing_failed", l.ID, fmt.Sprintf("persist listing %s failed: %v", l.ID, err))
	}
}

// persistHost writes an extracted host record into the local store,
// best-effort. No-op when db is nil, host is nil, or the host has no name
// (UpsertHostRecord keys on name). Errors are swallowed/stderr-warned.
func persistHost(db *store.Store, h *hostextract.HostInfo) {
	if db == nil || h == nil || h.Name == "" {
		return
	}
	if err := db.UpsertHostRecord(store.HostRecord{
		Name:  h.Name,
		Brand: h.Brand,
		Type:  h.Type,
	}); err != nil {
		persistWarn("persist_host_failed", h.Name, fmt.Sprintf("persist host %q failed: %v", h.Name, err))
	}
}

// persistPriceSnapshot appends a price snapshot for a scraped listing,
// best-effort. It is GUARDED on total > 0: a zero/unavailable price is "no
// price data", not a $0 snapshot, so we never pollute the price history (and
// `wishlist diff`) with phantom zero-totals. No-op when db is nil. Errors
// are swallowed/stderr-warned. fees is the airbnbTotals fee map; cleaning
// and service fees are projected into their dedicated columns when present.
func persistPriceSnapshot(db *store.Store, listingID, platform, checkin, checkout string, total float64, fees map[string]float64) {
	if db == nil || listingID == "" || total <= 0 {
		return
	}
	snap := store.PriceSnapshot{
		ListingID:  listingID,
		Platform:   platform,
		Checkin:    checkin,
		Checkout:   checkout,
		TotalPrice: total,
	}
	if fees != nil {
		snap.CleaningFee = feeLookup(fees, "cleaning", "cleaning_fee", "cleaningFee")
		snap.ServiceFee = feeLookup(fees, "service", "service_fee", "serviceFee", "guest_service_fee")
		snap.Tax = feeLookup(fees, "tax", "taxes", "occupancy_tax")
	}
	if err := db.InsertPriceSnapshot(snap); err != nil {
		persistWarn("persist_snapshot_failed", listingID, fmt.Sprintf("persist snapshot for %s failed: %v", listingID, err))
	}
}

// feeLookup returns the first matching fee value from a fee map, trying each
// candidate key in order. Airbnb's SSR fee map keys are not stable across
// listings, so callers pass several aliases for one logical fee.
func feeLookup(fees map[string]float64, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := fees[k]; ok {
			return v
		}
	}
	return 0
}
