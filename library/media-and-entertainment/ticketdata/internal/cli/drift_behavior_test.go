// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketdata/internal/store"
)

// seedDriftStore builds a store with watched events, their snapshots, and an
// unwatched target event, then closes it so the CLI under test owns the DB.
func seedDriftStore(t *testing.T, dbPath string) {
	t.Helper()
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	watch := func(id, title string) {
		if err := s.AddWatch(ctx, store.TDWatch{EventID: id, Title: title}); err != nil {
			t.Fatalf("add watch %s: %v", id, err)
		}
	}
	snap := func(id string, price float64, at string) {
		if err := s.InsertSnapshot(ctx, store.TDSnapshot{EventID: id, GetInPrice: price, CapturedAt: at}); err != nil {
			t.Fatalf("insert snapshot %s: %v", id, err)
		}
	}

	// A: moved +20% (past threshold). B: +1% (under). C: only one snapshot (skipped).
	watch("A", "Event A")
	watch("B", "Event B")
	watch("C", "Event C")
	snap("A", 100, "2026-07-01T00:00:00Z")
	snap("A", 120, "2026-07-02T00:00:00Z")
	snap("B", 100, "2026-07-01T00:00:00Z")
	snap("B", 101, "2026-07-02T00:00:00Z")
	snap("C", 100, "2026-07-01T00:00:00Z")

	// T1: unwatched target event, latest 90 (below the 150 target => hit).
	snap("T1", 90, "2026-07-01T00:00:00Z")
}

func TestDrift_BatchedReadPreservesBehavior(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "drift.db")
	seedDriftStore(t, dbPath)

	stdout, stderr, err := runRootArgs(t, "drift", "--db", dbPath, "--threshold", "5", "--target", "T1=150")
	if err != nil {
		t.Fatalf("drift: %v (stderr=%q)", err, stderr)
	}
	var view driftView
	if err := json.Unmarshal([]byte(stdout), &view); err != nil {
		t.Fatalf("drift JSON: %v (stdout=%q)", err, stdout)
	}

	// Only A moved past the 5% threshold; B is under, C has <2 snapshots.
	if len(view.Moved) != 1 {
		t.Fatalf("want 1 moved event, got %d: %+v", len(view.Moved), view.Moved)
	}
	m := view.Moved[0]
	if m.EventID != "A" || m.PreviousPrice != 100 || m.CurrentPrice != 120 || m.Direction != "up" {
		t.Fatalf("move A wrong: %+v", m)
	}

	// Unwatched target T1 still resolves via the batched union query.
	if len(view.TargetHits) != 1 {
		t.Fatalf("want 1 target hit, got %d: %+v", len(view.TargetHits), view.TargetHits)
	}
	if h := view.TargetHits[0]; h.EventID != "T1" || h.CurrentPrice != 90 || h.Target != 150 || !h.Hit {
		t.Fatalf("target hit wrong: %+v", h)
	}
}

func TestDrift_EmptyStoreShowsNothing(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "empty.db")
	stdout, stderr, err := runRootArgs(t, "drift", "--db", dbPath)
	if err != nil {
		t.Fatalf("drift empty: %v (stderr=%q)", err, stderr)
	}
	var view driftView
	if err := json.Unmarshal([]byte(stdout), &view); err != nil {
		t.Fatalf("drift JSON: %v (stdout=%q)", err, stdout)
	}
	if len(view.Moved) != 0 || len(view.TargetHits) != 0 {
		t.Fatalf("empty store should yield no moves/hits, got %+v", view)
	}
}
