package store

import (
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// TestDeleteWatchlistItem_RemovesByURL is the F3 store-delete contract: an
// existing watchlist row is removed by its listing_url and the row count
// drops to zero; deleting a URL that isn't watched reports zero rows affected
// without error.
func TestDeleteWatchlistItem_RemovesByURL(t *testing.T) {
	s := openTestStore(t)
	url := "https://www.airbnb.com/rooms/37124493"
	if err := s.UpsertWatchlistItem(WatchlistItem{ListingURL: url, ListingID: "37124493", Platform: "airbnb", MaxPrice: 350}); err != nil {
		t.Fatalf("upsert watchlist: %v", err)
	}

	items, err := s.ListWatchlist(0)
	if err != nil {
		t.Fatalf("list watchlist: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("watchlist len = %d, want 1", len(items))
	}

	n, err := s.DeleteWatchlistItem(url)
	if err != nil {
		t.Fatalf("delete watchlist item: %v", err)
	}
	if n != 1 {
		t.Fatalf("rows affected = %d, want 1", n)
	}

	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM watchlist`).Scan(&count); err != nil {
		t.Fatalf("count watchlist: %v", err)
	}
	if count != 0 {
		t.Fatalf("watchlist count after delete = %d, want 0", count)
	}

	// Deleting again (now absent) is a no-op reporting zero rows.
	n, err = s.DeleteWatchlistItem(url)
	if err != nil {
		t.Fatalf("delete absent item: %v", err)
	}
	if n != 0 {
		t.Fatalf("rows affected for absent url = %d, want 0", n)
	}
}

// TestInsertPriceSnapshot_PopulatesTable confirms a snapshot lands in the
// price_snapshots table and is readable back via ListPriceSnapshotsSince —
// the F1 building block that search/get/cheapest/watch write through.
func TestInsertPriceSnapshot_PopulatesTable(t *testing.T) {
	s := openTestStore(t)
	if err := s.InsertPriceSnapshot(PriceSnapshot{
		ListingID:   "37124493",
		Platform:    "airbnb",
		Checkin:     "2026-07-10",
		Checkout:    "2026-07-14",
		TotalPrice:  1234.50,
		CleaningFee: 90,
		ServiceFee:  60,
	}); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}

	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM price_snapshots`).Scan(&count); err != nil {
		t.Fatalf("count snapshots: %v", err)
	}
	if count != 1 {
		t.Fatalf("snapshot count = %d, want 1", count)
	}

	snaps, err := s.ListPriceSnapshotsSince(0)
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("listed snapshots = %d, want 1", len(snaps))
	}
	if snaps[0].TotalPrice != 1234.50 {
		t.Fatalf("snapshot total = %v, want 1234.50", snaps[0].TotalPrice)
	}
}

// TestUpsertHostRecord_PopulatesHostsTable confirms a host record lands in the
// hosts table keyed on name — the F1 host-persist building block.
func TestUpsertHostRecord_PopulatesHostsTable(t *testing.T) {
	s := openTestStore(t)
	if err := s.UpsertHostRecord(HostRecord{Name: "RnR Vacation Rentals", Brand: "RnR Vacation Rentals", Type: "pmc"}); err != nil {
		t.Fatalf("upsert host: %v", err)
	}
	var name, typ string
	if err := s.DB().QueryRow(`SELECT name, type FROM hosts WHERE name = ?`, "RnR Vacation Rentals").Scan(&name, &typ); err != nil {
		t.Fatalf("select host: %v", err)
	}
	if typ != "pmc" {
		t.Fatalf("host type = %q, want pmc", typ)
	}
}
