// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package watch

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "watches.db")
	s, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestStoreInsertAndGet(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	w := newSampleWatch()
	id, err := s.Insert(ctx, w)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if id == "" || id != w.ID {
		t.Fatalf("expected ID populated, got %q vs %q", id, w.ID)
	}

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Origin != "SFO" || got.Destination != "JFK" || got.Airline != "DL" || got.FlightNumber != "669" {
		t.Fatalf("round-trip lost fields: %+v", got)
	}
	if got.OriginalPrice != 428.20 || got.Threshold != 50 {
		t.Fatalf("round-trip lost numeric fields: %+v", got)
	}
	if !got.CreatedAt.Before(time.Now().Add(time.Minute)) || got.CreatedAt.IsZero() {
		t.Fatalf("CreatedAt looks wrong: %v", got.CreatedAt)
	}
	if got.LastCheckedAt != nil || got.LastSeenPrice != nil || got.LastAlertedPrice != nil {
		t.Fatalf("fresh watch should have nil tracking fields: %+v", got)
	}
}

func TestStoreListAndDelete(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	a, _ := s.Insert(ctx, newSampleWatch())
	b := newSampleWatch()
	b.FlightNumber = "100"
	bID, _ := s.Insert(ctx, b)

	rows, err := s.List(ctx, "")
	if err != nil || len(rows) != 2 {
		t.Fatalf("List: %v len=%d", err, len(rows))
	}

	if err := s.Delete(ctx, a); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := s.Delete(ctx, a); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second Delete should be ErrNotFound, got %v", err)
	}

	rows, _ = s.List(ctx, "")
	if len(rows) != 1 || rows[0].ID != bID {
		t.Fatalf("unexpected remaining rows: %+v", rows)
	}
}

func TestStoreRecordCheckUpdatesAlertedPrice(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	id, _ := s.Insert(ctx, newSampleWatch())

	now := time.Now().UTC()
	price := 354.10
	if err := s.RecordCheck(ctx, id, now, &price, true); err != nil {
		t.Fatalf("RecordCheck: %v", err)
	}
	got, _ := s.Get(ctx, id)
	if got.LastSeenPrice == nil || *got.LastSeenPrice != 354.10 {
		t.Fatalf("LastSeenPrice should be 354.10, got %v", got.LastSeenPrice)
	}
	if got.LastAlertedPrice == nil || *got.LastAlertedPrice != 354.10 {
		t.Fatalf("LastAlertedPrice should be 354.10, got %v", got.LastAlertedPrice)
	}

	// A subsequent check that doesn't alert should NOT overwrite last_alerted_price.
	lower := 200.0
	if err := s.RecordCheck(ctx, id, now, &lower, false); err != nil {
		t.Fatalf("RecordCheck (no alert): %v", err)
	}
	got, _ = s.Get(ctx, id)
	if *got.LastSeenPrice != 200.0 {
		t.Fatalf("LastSeenPrice should be 200, got %v", *got.LastSeenPrice)
	}
	if got.LastAlertedPrice == nil || *got.LastAlertedPrice != 354.10 {
		t.Fatalf("LastAlertedPrice should still be 354.10, got %v", got.LastAlertedPrice)
	}
}

func TestStoreReturnsErrNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, err := s.Get(ctx, "watch_doesnotexist"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
