// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func openTestCarStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "car.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// backdateRating pushes a cached rating's updated_at into the past so TTL
// expiry can be exercised without waiting.
func backdateRating(t *testing.T, s *Store, airport, supplier string, age time.Duration) {
	t.Helper()
	ts := time.Now().UTC().Add(-age).Format(time.RFC3339)
	if _, err := s.DB().Exec(`UPDATE supplier_ratings SET updated_at=? WHERE airport=? AND supplier=?`, ts, airport, supplier); err != nil {
		t.Fatalf("backdate: %v", err)
	}
}

func TestSupplierRatingFreshness(t *testing.T) {
	ctx := context.Background()
	s := openTestCarStore(t)

	err := s.UpsertSupplierRatings(ctx, "BIO", []SupplierRating{
		{Supplier: "Sixt", Score: 8.8, Reviews: 1200, Source: "rentalcars"},
		{Supplier: "Goldcar", Score: 6.0, Reviews: 900, Source: "rentalcars"},
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// UpdatedAt is populated on read.
	got, err := s.GetSupplierRatings(ctx, "BIO")
	if err != nil || len(got) != 2 {
		t.Fatalf("get: %v (n=%d)", err, len(got))
	}
	for _, r := range got {
		if r.UpdatedAt.IsZero() {
			t.Errorf("%s has zero UpdatedAt", r.Supplier)
		}
	}

	// Age Goldcar past the TTL; a 14-day purge should drop only it.
	backdateRating(t, s, "BIO", "Goldcar", 20*24*time.Hour)
	removed, err := s.PurgeStaleSupplierRatings(ctx, 14*24*time.Hour)
	if err != nil || removed != 1 {
		t.Fatalf("purge: removed=%d err=%v", removed, err)
	}
	got, _ = s.GetSupplierRatings(ctx, "BIO")
	if len(got) != 1 || got[0].Supplier != "Sixt" {
		t.Fatalf("after purge expected only Sixt, got %+v", got)
	}

	// Purge with non-positive maxAge is a no-op.
	if n, _ := s.PurgeStaleSupplierRatings(ctx, 0); n != 0 {
		t.Errorf("purge(0) removed %d, want 0", n)
	}
}

func TestFXRatesCache(t *testing.T) {
	ctx := context.Background()
	s := openTestCarStore(t)

	// Empty cache → nil Rates.
	if c, err := s.GetFXRates(ctx); err != nil || c.Rates != nil {
		t.Fatalf("empty cache: %+v err=%v", c, err)
	}

	if err := s.UpsertFXRates(ctx, "2026-07-14", map[string]float64{"EUR": 1, "USD": 1.0856, "GBP": 0.8452}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	c, err := s.GetFXRates(ctx)
	if err != nil || c.Date != "2026-07-14" || len(c.Rates) != 3 || c.UpdatedAt.IsZero() {
		t.Fatalf("get: %+v err=%v", c, err)
	}
	if c.Rates["USD"] != 1.0856 {
		t.Errorf("USD rate = %v", c.Rates["USD"])
	}

	// Upsert replaces the whole set (fewer currencies, new date).
	if err := s.UpsertFXRates(ctx, "2026-07-15", map[string]float64{"EUR": 1, "USD": 1.09}); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	c, _ = s.GetFXRates(ctx)
	if len(c.Rates) != 2 || c.Date != "2026-07-15" || c.Rates["USD"] != 1.09 {
		t.Errorf("after replace: %+v", c)
	}
}

func TestSupplierRatingCacheStatsAndClear(t *testing.T) {
	ctx := context.Background()
	s := openTestCarStore(t)
	_ = s.UpsertSupplierRatings(ctx, "AGP", []SupplierRating{{Supplier: "Centauro", Score: 7.2, Reviews: 500, Source: "rentalcars"}})
	_ = s.UpsertSupplierRatings(ctx, "BIO", []SupplierRating{
		{Supplier: "Sixt", Score: 8.8, Reviews: 1200, Source: "rentalcars"},
		{Supplier: "Record Go", Score: 8.1, Reviews: 700, Source: "rentalcars"},
	})

	stats, err := s.SupplierRatingCacheStats(ctx)
	if err != nil || len(stats) != 2 {
		t.Fatalf("stats: %v (n=%d)", err, len(stats))
	}
	byAirport := map[string]RatingCacheStat{}
	for _, st := range stats {
		byAirport[st.Airport] = st
	}
	if byAirport["BIO"].Suppliers != 2 || byAirport["AGP"].Suppliers != 1 {
		t.Errorf("counts wrong: %+v", byAirport)
	}
	if byAirport["BIO"].Oldest.IsZero() || byAirport["BIO"].Newest.IsZero() {
		t.Error("BIO oldest/newest should be set")
	}

	// Clear one airport, leave the other.
	if n, err := s.ClearSupplierRatings(ctx, "BIO"); err != nil || n != 2 {
		t.Fatalf("clear BIO: n=%d err=%v", n, err)
	}
	if got, _ := s.GetSupplierRatings(ctx, "BIO"); len(got) != 0 {
		t.Errorf("BIO should be empty, got %d", len(got))
	}
	if got, _ := s.GetSupplierRatings(ctx, "AGP"); len(got) != 1 {
		t.Errorf("AGP should survive, got %d", len(got))
	}

	// Clear all.
	if n, err := s.ClearSupplierRatings(ctx, ""); err != nil || n != 1 {
		t.Fatalf("clear all: n=%d err=%v", n, err)
	}
}
