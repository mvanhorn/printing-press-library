package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestRebuildPurchasedItemsFrequencyTiebreak(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	older := now.AddDate(0, -2, 0)
	mustUpsertOrder(t, s, Order{OrderID: "O-OLD", PlacedAt: older, TotalCents: 1000})
	mustUpsertOrder(t, s, Order{OrderID: "O-RECENT", PlacedAt: now, TotalCents: 2000})

	// ASIN A: bought 3x across the older order, 1x recent.
	mustUpsertItem(t, s, OrderItem{OrderID: "O-OLD", ASIN: "ASIN-A", Title: "Frequent Bath Tissue 24-pack", Quantity: 3})
	mustUpsertItem(t, s, OrderItem{OrderID: "O-RECENT", ASIN: "ASIN-A", Title: "Frequent Bath Tissue 24-pack", Quantity: 1})
	// ASIN B: bought once, but more recently.
	mustUpsertItem(t, s, OrderItem{OrderID: "O-RECENT", ASIN: "ASIN-B", Title: "Trial Bath Tissue Single", Quantity: 1})

	if err := s.RebuildPurchasedItems(ctx); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	rows, quality, err := s.SearchPurchasedItems("bath tissue", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 matches, got %d", len(rows))
	}
	if quality != MatchStrict {
		t.Fatalf("want strict match (both tokens present), got %q", quality)
	}
	if rows[0].ASIN != "ASIN-A" {
		t.Fatalf("most-frequent should win the tiebreak, got %s", rows[0].ASIN)
	}
	if rows[0].PurchaseCount != 4 {
		t.Fatalf("want purchase_count=4 (3+1), got %d", rows[0].PurchaseCount)
	}
}

func TestSearchEmptyQueryReturnsNoRows(t *testing.T) {
	s := newTestStore(t)
	rows, quality, err := s.SearchPurchasedItems("   ", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("empty query should yield zero rows, got %d", len(rows))
	}
	if quality != "" {
		t.Fatalf("empty query should have empty quality, got %q", quality)
	}
}

func TestSearchLooseFallbackForCompoundWord(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	mustUpsertOrder(t, s, Order{OrderID: "O-1", PlacedAt: time.Now().UTC(), TotalCents: 100})
	mustUpsertItem(t, s, OrderItem{OrderID: "O-1", ASIN: "ASIN-PUFFS", Title: "CauliPuffs Variety Pack", Quantity: 1})
	if err := s.RebuildPurchasedItems(ctx); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	// "cauli puffs" can't AND-match "CauliPuffs" (one indexed token), so OR fallback rescues.
	rows, quality, err := s.SearchPurchasedItems("cauli puffs", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(rows) != 1 || rows[0].ASIN != "ASIN-PUFFS" {
		t.Fatalf("want loose match on CauliPuffs, got %+v", rows)
	}
	if quality != MatchLoose {
		t.Fatalf("compound-word match should be loose, got %q", quality)
	}
}

func TestNormalizeFTSQuery(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"bath tissue", "bath* AND tissue*"},
		{"Bath-Tissue!", "Bath* AND Tissue*"},
		{"   ", ""},
		{"detergent", "detergent*"},
	}
	for _, c := range cases {
		if got := NormalizeFTSQuery(c.in); got != c.want {
			t.Errorf("NormalizeFTSQuery(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestMostRecentOrderEmpty(t *testing.T) {
	s := newTestStore(t)
	o, err := s.MostRecentOrder()
	if err != nil {
		t.Fatalf("most recent: %v", err)
	}
	if o.OrderID != "" {
		t.Fatalf("empty store should have no most-recent order, got %q", o.OrderID)
	}
}

func TestMostRecentOrderWinner(t *testing.T) {
	s := newTestStore(t)
	older := time.Now().UTC().Add(-72 * time.Hour).Truncate(time.Second)
	newer := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	mustUpsertOrder(t, s, Order{OrderID: "O-1", PlacedAt: older})
	mustUpsertOrder(t, s, Order{OrderID: "O-2", PlacedAt: newer})
	o, err := s.MostRecentOrder()
	if err != nil {
		t.Fatalf("most recent: %v", err)
	}
	if o.OrderID != "O-2" {
		t.Fatalf("expected O-2 (newer), got %q", o.OrderID)
	}
}

func mustUpsertOrder(t *testing.T, s *Store, o Order) {
	t.Helper()
	if err := s.UpsertOrder(context.Background(), o, ""); err != nil {
		t.Fatalf("upsert order: %v", err)
	}
}

func mustUpsertItem(t *testing.T, s *Store, oi OrderItem) {
	t.Helper()
	if err := s.UpsertOrderItem(context.Background(), oi, ""); err != nil {
		t.Fatalf("upsert item: %v", err)
	}
}
