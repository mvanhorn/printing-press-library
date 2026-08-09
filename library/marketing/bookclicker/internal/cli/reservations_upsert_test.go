// Copyright 2026 wmiles81 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: regression test for the reservation_mirror upsert. Kept in its
// own file so `generate --force` preserves it.

package cli

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/bookclicker/internal/store"
)

// TestUpsertReservation_RefreshesMutableFields pins that a re-pull converges on
// what the launch page now says, rather than keeping the first-seen values.
//
// The original ON CONFLICT branch refreshed only status and the numeric metrics.
// That silently corrupted two commands, because both aggregate on string keys
// that were never updated:
//
//   - partner-roi groups by list_name (falling back to counterparty), and
//     swap-balance groups by counterparty (falling back to list_name). A renamed
//     newsletter kept its old name on existing rows while new rows carried the
//     new one, so one partner split into two identities.
//
//   - swap-balance counts a swap as paired when swap_reservation_id > 0. That id
//     is assigned when the swap is matched, typically after the reservation was
//     first mirrored, so leaving the column stale kept paired swaps counted as
//     unpaired forever.
//
// date and inv_type feed capacity and launch health and go stale the same way.
func TestUpsertReservation_RefreshesMutableFields(t *testing.T) {
	ctx := context.Background()
	s, err := store.OpenWithContext(ctx, filepath.Join(t.TempDir(), "mirror.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	// reservation_mirror lives in the hand-owned migration, not the generated
	// schema, so it is created explicitly the same way every novel command does.
	if err := store.EnsureBookclickerTables(ctx, s); err != nil {
		t.Fatalf("ensure tables: %v", err)
	}

	firstPrice := int64(40)
	first := novelReservation{
		ID:         "res-1",
		BookID:     11,
		BookTitle:  "Working Title",
		ListID:     22,
		ListName:   "Old Newsletter Name",
		ListSize:   1000,
		Date:       "2026-09-01",
		InvType:    "feature",
		Status:     "pending",
		IsSwap:     true,
		Price:      &firstPrice,
		SwapPairID: 0, // not yet matched
		Counterpar: "Old Partner",
		CreatedAt:  "2026-08-01",
	}
	isNew, err := upsertReservation(ctx, s, first)
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if !isNew {
		t.Fatal("first upsert should report the row as new")
	}

	// The launch page now reports the same reservation with a renamed
	// newsletter, a rescheduled date, a changed slot type, and a swap that has
	// since been matched.
	secondPrice := int64(55)
	second := first
	second.BookTitle = "Final Title"
	second.ListName = "New Newsletter Name"
	second.Date = "2026-09-14"
	second.InvType = "solo"
	second.Status = "sent"
	second.Price = &secondPrice
	second.SwapPairID = 987
	second.Counterpar = "New Partner"

	isNew, err = upsertReservation(ctx, s, second)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if isNew {
		t.Fatal("second upsert should not report the row as new")
	}

	var (
		bookTitle, listName, date, invType, status, counterparty string
		swapPairID, price                                        int64
		rows                                                     int
	)
	if err := s.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM reservation_mirror`).Scan(&rows); err != nil {
		t.Fatalf("count: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected the upsert to update in place, got %d rows", rows)
	}
	if err := s.DB().QueryRowContext(ctx, `
		SELECT book_title, list_name, date, inv_type, status,
		       counterparty, swap_reservation_id, price
		FROM reservation_mirror WHERE id = ?`, "res-1").
		Scan(&bookTitle, &listName, &date, &invType, &status,
			&counterparty, &swapPairID, &price); err != nil {
		t.Fatalf("select: %v", err)
	}

	for _, c := range []struct{ field, got, want string }{
		{"book_title", bookTitle, "Final Title"},
		{"list_name", listName, "New Newsletter Name"},
		{"date", date, "2026-09-14"},
		{"inv_type", invType, "solo"},
		{"status", status, "sent"},
		{"counterparty", counterparty, "New Partner"},
	} {
		if c.got != c.want {
			t.Errorf("%s not refreshed: got %q, want %q", c.field, c.got, c.want)
		}
	}
	if swapPairID != 987 {
		t.Errorf("swap_reservation_id not refreshed: got %d, want 987 "+
			"(a swap matched after first mirror would stay counted as unpaired)", swapPairID)
	}
	if price != 55 {
		t.Errorf("price not refreshed: got %d, want 55", price)
	}
}
