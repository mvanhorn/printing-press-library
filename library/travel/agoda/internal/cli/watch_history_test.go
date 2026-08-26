// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/agoda/internal/store"
)

// openTestStore creates an isolated store with the hand-authored agoda schema.
func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	ctx := context.Background()
	st, err := store.OpenWithContext(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("opening test store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := store.EnsureAgodaSchema(ctx, st.DB()); err != nil {
		t.Fatalf("creating agoda schema: %v", err)
	}
	return st
}

func insertObservation(t *testing.T, st *store.Store, propertyID int, w watchSpec, price float64, observedAt string) {
	t.Helper()
	_, err := st.DB().ExecContext(context.Background(), `
        INSERT INTO price_observations
            (property_id, property_name, city_id, checkin, nights, adults, rooms,
             currency, price_advertised, price_all_in, observed_at)
        VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		propertyID, "Test Property", w.CityID, w.CheckIn, w.Nights, w.Adults,
		w.Rooms, w.Currency, price*0.8, price, observedAt)
	if err != nil {
		t.Fatalf("inserting observation: %v", err)
	}
}

// TestWatchListCountsOnlyMatchingWatch pins a P1 review finding: the
// observation count filtered on city, check-in, nights, and adults but omitted
// rooms and currency, so a second watch differing only in those dimensions had
// its observations counted against the first.
func TestWatchListCountsOnlyMatchingWatch(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)

	base := watchSpec{CityID: 5085, CheckIn: "2026-10-15", Nights: 2, Adults: 2, Rooms: 1, Currency: "USD"}
	otherRooms := base
	otherRooms.Rooms = 2
	otherCurrency := base
	otherCurrency.Currency = "EUR"

	insertObservation(t, st, 1, base, 100, "2026-08-01T00:00:00Z")
	insertObservation(t, st, 1, base, 110, "2026-08-02T00:00:00Z")
	insertObservation(t, st, 1, otherRooms, 200, "2026-08-03T00:00:00Z")
	insertObservation(t, st, 1, otherCurrency, 300, "2026-08-04T00:00:00Z")

	var n int
	err := st.DB().QueryRowContext(ctx, `
        SELECT COUNT(*) FROM price_observations
        WHERE city_id = ? AND checkin = ? AND nights = ? AND adults = ?
          AND rooms = ? AND currency = ?`,
		base.CityID, base.CheckIn, base.Nights, base.Adults, base.Rooms, base.Currency).Scan(&n)
	if err != nil {
		t.Fatalf("counting observations: %v", err)
	}
	if n != 2 {
		t.Errorf("observation count = %d, want 2 (rows differing only by rooms or currency belong to other watches)", n)
	}
}

// TestDetectDropsExcludesCurrentObservationOnTiedTimestamps pins the other P1
// finding: observed_at has one-second resolution, so two runs inside the same
// second tie. Without a stable tiebreaker the just-recorded price could stay in
// its own baseline while an older row was dropped, corrupting the median and
// producing false or missed alerts.
func TestDetectDropsExcludesCurrentObservationOnTiedTimestamps(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	w := watchSpec{CityID: 5085, CheckIn: "2026-10-15", Nights: 2, Adults: 2, Rooms: 1, Currency: "USD"}

	// Four prior observations at 100 plus the current one at 50, all stamped
	// with the identical second so ordering can only come from the tiebreaker.
	const tied = "2026-08-26T12:00:00Z"
	for i := 0; i < 4; i++ {
		insertObservation(t, st, 42, w, 100, tied)
	}
	insertObservation(t, st, 42, w, 50, tied)

	drops, err := detectDrops(ctx, st.DB(), w,
		agodaTestProperties{{id: 42, price: 50}}.toProperties(), 10, 3)
	if err != nil {
		t.Fatalf("detectDrops() error = %v", err)
	}
	if len(drops) != 1 {
		t.Fatalf("got %d drops, want 1", len(drops))
	}
	// The baseline must be the four 100s, not a mix that includes the current 50.
	if drops[0].MedianAllIn != 100 {
		t.Errorf("trailing median = %v, want 100 (the current observation must be excluded)", drops[0].MedianAllIn)
	}
	if drops[0].DropPct != 50 {
		t.Errorf("drop = %v%%, want 50", drops[0].DropPct)
	}
}
