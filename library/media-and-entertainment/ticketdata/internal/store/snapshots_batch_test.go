// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
)

func newSnapshotStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "snaps.db")
	s, err := OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// seedSnapshot inserts one snapshot with an explicit captured_at so ordering is
// deterministic across the batch/single-event comparison.
func seedSnapshot(t *testing.T, s *Store, eventID string, price float64, capturedAt string) {
	t.Helper()
	if err := s.InsertSnapshot(context.Background(), TDSnapshot{
		EventID:    eventID,
		GetInPrice: price,
		CapturedAt: capturedAt,
	}); err != nil {
		t.Fatalf("insert snapshot %s@%s: %v", eventID, capturedAt, err)
	}
}

func TestLatestSnapshotsForEvents_CapsAndOrdersPerEvent(t *testing.T) {
	s := newSnapshotStore(t)
	ctx := context.Background()

	// Event A: three snapshots; Event B: two. Insert oldest-first.
	seedSnapshot(t, s, "A", 100, "2026-07-01T00:00:00Z")
	seedSnapshot(t, s, "A", 110, "2026-07-02T00:00:00Z")
	seedSnapshot(t, s, "A", 120, "2026-07-03T00:00:00Z")
	seedSnapshot(t, s, "B", 200, "2026-07-01T00:00:00Z")
	seedSnapshot(t, s, "B", 210, "2026-07-02T00:00:00Z")

	got, err := s.LatestSnapshotsForEvents(ctx, []string{"A", "B"}, 2)
	if err != nil {
		t.Fatalf("LatestSnapshotsForEvents: %v", err)
	}
	if len(got["A"]) != 2 {
		t.Fatalf("event A: want 2 snapshots, got %d", len(got["A"]))
	}
	// Most-recent first, oldest (100) excluded by the n=2 cap.
	if got["A"][0].GetInPrice != 120 || got["A"][1].GetInPrice != 110 {
		t.Fatalf("event A ordering wrong: %+v", got["A"])
	}
	if len(got["B"]) != 2 || got["B"][0].GetInPrice != 210 {
		t.Fatalf("event B wrong: %+v", got["B"])
	}
}

func TestLatestSnapshotsForEvents_TiebreakByIDWhenCapturedAtEqual(t *testing.T) {
	s := newSnapshotStore(t)
	ctx := context.Background()

	// Same captured_at → the later-inserted row (higher id) must sort first,
	// matching single-event LatestSnapshots.
	seedSnapshot(t, s, "A", 100, "2026-07-01T00:00:00Z")
	seedSnapshot(t, s, "A", 999, "2026-07-01T00:00:00Z")

	got, err := s.LatestSnapshotsForEvents(ctx, []string{"A"}, 2)
	if err != nil {
		t.Fatalf("LatestSnapshotsForEvents: %v", err)
	}
	if len(got["A"]) != 2 || got["A"][0].GetInPrice != 999 {
		t.Fatalf("tiebreak wrong: %+v", got["A"])
	}
}

func TestLatestSnapshotsForEvents_ParityWithSingleEvent(t *testing.T) {
	s := newSnapshotStore(t)
	ctx := context.Background()

	seedSnapshot(t, s, "A", 100, "2026-07-01T00:00:00Z")
	seedSnapshot(t, s, "A", 110, "2026-07-02T00:00:00Z")
	seedSnapshot(t, s, "A", 120, "2026-07-03T00:00:00Z")

	single, err := s.LatestSnapshots(ctx, "A", 2)
	if err != nil {
		t.Fatalf("LatestSnapshots: %v", err)
	}
	batch, err := s.LatestSnapshotsForEvents(ctx, []string{"A"}, 2)
	if err != nil {
		t.Fatalf("LatestSnapshotsForEvents: %v", err)
	}
	if !reflect.DeepEqual(single, batch["A"]) {
		t.Fatalf("parity mismatch:\n single=%+v\n batch =%+v", single, batch["A"])
	}
}

func TestLatestSnapshotsForEvents_EmptyAndMissingInputs(t *testing.T) {
	s := newSnapshotStore(t)
	ctx := context.Background()
	seedSnapshot(t, s, "A", 100, "2026-07-01T00:00:00Z")

	// Empty input: non-nil, empty map, no error.
	got, err := s.LatestSnapshotsForEvents(ctx, nil, 2)
	if err != nil {
		t.Fatalf("empty input error: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("empty input: want empty non-nil map, got %#v", got)
	}

	// n <= 0: empty map.
	if got, err := s.LatestSnapshotsForEvents(ctx, []string{"A"}, 0); err != nil || len(got) != 0 {
		t.Fatalf("n<=0: want empty map no error, got %#v err=%v", got, err)
	}

	// Unknown event: simply absent from the map (no panic, no empty-slice key).
	got, err = s.LatestSnapshotsForEvents(ctx, []string{"A", "does-not-exist"}, 2)
	if err != nil {
		t.Fatalf("mixed known/unknown error: %v", err)
	}
	if _, ok := got["does-not-exist"]; ok {
		t.Fatalf("unknown event should be absent from map, got %#v", got["does-not-exist"])
	}
	if len(got["A"]) != 1 {
		t.Fatalf("known event A: want 1 snapshot, got %d", len(got["A"]))
	}
}

func TestLatestSnapshotsForEvents_FreshStoreNoPanic(t *testing.T) {
	s := newSnapshotStore(t)
	// No snapshots inserted at all: tables are ensured, query returns empty map.
	got, err := s.LatestSnapshotsForEvents(context.Background(), []string{"A", "B"}, 2)
	if err != nil {
		t.Fatalf("fresh store error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("fresh store: want empty map, got %#v", got)
	}
}
