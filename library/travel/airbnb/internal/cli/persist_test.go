package cli

import (
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/source/airbnb"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/store"
	_ "modernc.org/sqlite"
)

func newPersistTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// TestPersistAirbnbListing_WritesRowAndSkipsEmptyID covers the F1 listing-
// persist path: a listing with an id lands in the airbnb_listing table, and a
// card with no id is skipped (Airbnb SSR search cards sometimes lack a stable
// id) rather than erroring.
func TestPersistAirbnbListing_WritesRowAndSkipsEmptyID(t *testing.T) {
	db := newPersistTestStore(t)

	persistAirbnbListing(db, &airbnb.Listing{ID: "37124493", Title: "Lakefront cabin", City: "South Lake Tahoe"})

	var count int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM airbnb_listing`).Scan(&count); err != nil {
		t.Fatalf("count airbnb_listing: %v", err)
	}
	if count != 1 {
		t.Fatalf("airbnb_listing count = %d, want 1", count)
	}

	// A card with no id must not error and must not add a row.
	persistAirbnbListing(db, &airbnb.Listing{Title: "no id card"})
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM airbnb_listing`).Scan(&count); err != nil {
		t.Fatalf("recount airbnb_listing: %v", err)
	}
	if count != 1 {
		t.Fatalf("airbnb_listing count after empty-id persist = %d, want 1", count)
	}

	// nil store is a no-op (must not panic).
	persistAirbnbListing(nil, &airbnb.Listing{ID: "1"})
}

// TestPersistPriceSnapshot_GuardsOnPositivePrice is the F1/F2 shared
// invariant: a positive total writes a snapshot, while a zero or negative
// total writes NOTHING — an unavailable price is "no price data", never a $0
// snapshot that would pollute price history and wishlist diff.
func TestPersistPriceSnapshot_GuardsOnPositivePrice(t *testing.T) {
	db := newPersistTestStore(t)

	persistPriceSnapshot(db, "37124493", "airbnb", "2026-07-10", "2026-07-14", 1500, map[string]float64{"cleaning": 90, "service": 60})

	var count int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM price_snapshots`).Scan(&count); err != nil {
		t.Fatalf("count snapshots: %v", err)
	}
	if count != 1 {
		t.Fatalf("snapshot count = %d, want 1", count)
	}

	// Zero and negative prices are no-ops: no phantom snapshots.
	persistPriceSnapshot(db, "37124493", "airbnb", "2026-08-01", "2026-08-05", 0, nil)
	persistPriceSnapshot(db, "37124493", "airbnb", "2026-09-01", "2026-09-05", -10, nil)
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM price_snapshots`).Scan(&count); err != nil {
		t.Fatalf("recount snapshots: %v", err)
	}
	if count != 1 {
		t.Fatalf("snapshot count after zero/neg persist = %d, want 1 (no phantom rows)", count)
	}

	// Empty listing id is a no-op even with a positive price.
	persistPriceSnapshot(db, "", "airbnb", "2026-10-01", "2026-10-05", 999, nil)
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM price_snapshots`).Scan(&count); err != nil {
		t.Fatalf("recount snapshots after empty-id: %v", err)
	}
	if count != 1 {
		t.Fatalf("snapshot count after empty-id persist = %d, want 1", count)
	}

	// nil store is a no-op (must not panic).
	persistPriceSnapshot(nil, "1", "airbnb", "", "", 100, nil)

	// Fee aliases project into the dedicated columns.
	snaps, err := db.ListPriceSnapshotsSince(0)
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("listed snapshots = %d, want 1", len(snaps))
	}
	if snaps[0].CleaningFee != 90 || snaps[0].ServiceFee != 60 {
		t.Fatalf("fees = (cleaning %v, service %v), want (90, 60)", snaps[0].CleaningFee, snaps[0].ServiceFee)
	}
}

// TestFeeLookup_TriesAliasesInOrder confirms the fee-alias resolution used
// when mapping Airbnb's unstable SSR fee-map keys into snapshot columns.
func TestFeeLookup_TriesAliasesInOrder(t *testing.T) {
	fees := map[string]float64{"serviceFee": 42}
	if got := feeLookup(fees, "service", "service_fee", "serviceFee"); got != 42 {
		t.Fatalf("feeLookup = %v, want 42", got)
	}
	if got := feeLookup(fees, "nope", "missing"); got != 0 {
		t.Fatalf("feeLookup miss = %v, want 0", got)
	}
}
