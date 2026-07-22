// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored regression tests for the novel commands' rank-history scoping.
// Markerless on purpose: `generate --force` must not clobber these.

package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/sensortower/internal/store"
)

// writeSnap mirrors exactly what movers stores, including the composite id.
func writeSnap(t *testing.T, db *store.Store, category, date string, rank int, capturedAt string) {
	t.Helper()
	snap := rankSnapshot{
		AppID:      json.RawMessage(`"460177396"`),
		AppIDKey:   "460177396",
		Name:       "Twitch",
		OS:         "ios",
		Category:   category,
		Country:    "US",
		Chart:      "free",
		Device:     "iphone",
		Date:       date,
		Rank:       rank,
		CapturedAt: capturedAt,
		Limit:      25,
	}
	payload, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert(rankSnapshotResource, rankSnapshotID("ios", category, "US", "free", date, "460177396"), payload); err != nil {
		t.Fatal(err)
	}
}

// An app can sit on several category charts at once — `movers 6016` and
// `movers 36` both snapshot it — and a rank on one chart is not comparable to a
// rank on another. The digest reports one category per app, so both of the rows
// it diffs must come from that same category. Before this was scoped, the two
// newest rows could be the same date in different categories and their
// difference was printed as a move the app never made.
func TestReadLatestSnapshotsScopesToOneCategory(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Same app, same date, two categories, wildly different ladders.
	writeSnap(t, db, "6016", "2026-07-17", 3, "2026-07-17T10:00:00Z")
	writeSnap(t, db, "36", "2026-07-17", 45, "2026-07-17T10:05:00Z")

	snaps, err := readLatestSnapshots(context.Background(), db, "ios", "460177396", "US", "free")
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 {
		t.Fatalf("got %d snapshots, want 1: two same-date rows from different categories must not\n"+
			"be returned as a current/previous pair (that yields a fabricated delta of 42)", len(snaps))
	}
	// Category 36 wins the tie on captured_at, deterministically.
	if snaps[0].Category != "36" {
		t.Errorf("category = %q, want %q (newest captured_at must win the tie)", snaps[0].Category, "36")
	}
}

// A genuine within-category move across two dates must still produce a delta —
// the scoping must not cost the feature.
func TestReadLatestSnapshotsKeepsRealWithinCategoryMove(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	writeSnap(t, db, "36", "2026-07-16", 50, "2026-07-16T10:00:00Z")
	writeSnap(t, db, "36", "2026-07-17", 45, "2026-07-17T10:00:00Z")
	// Noise on another ladder that must be ignored entirely.
	writeSnap(t, db, "6016", "2026-07-17", 3, "2026-07-17T09:00:00Z")

	snaps, err := readLatestSnapshots(context.Background(), db, "ios", "460177396", "US", "free")
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 2 {
		t.Fatalf("got %d snapshots, want 2", len(snaps))
	}
	if snaps[0].Rank != 45 || snaps[1].Rank != 50 {
		t.Fatalf("got ranks %d/%d, want 45/50 (newest first, category 36 only)", snaps[0].Rank, snaps[1].Rank)
	}
	if delta := snaps[1].Rank - snaps[0].Rank; delta != 5 {
		t.Errorf("delta = %d, want 5", delta)
	}
	for _, s := range snaps {
		if s.Category != "36" {
			t.Errorf("category = %q, want %q — the 6016 row must never enter the pair", s.Category, "36")
		}
	}
}

// teardown aligns releases against rank history; mixing two categories' ladders
// would let it read "before" off one and "after" off another and print the
// difference as a post-release move that never happened.
func TestReadAppRankHistoryScopesToOneCategory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SENSORTOWER_DATA_DIR", dir)

	dbPath := defaultDBPath("sensortower-pp-cli")
	if !strings.HasPrefix(dbPath, dir) {
		t.Skipf("data dir override not honored (resolved %q); scoping is covered by the readLatestSnapshots tests", dbPath)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	writeSnap(t, db, "36", "2026-07-10", 50, "2026-07-10T10:00:00Z")
	writeSnap(t, db, "36", "2026-07-14", 49, "2026-07-14T10:00:00Z")
	writeSnap(t, db, "6016", "2026-07-14", 3, "2026-07-14T09:00:00Z")
	db.Close()

	points, category, err := readAppRankHistory(context.Background(), "460177396", "US")
	if err != nil {
		t.Fatal(err)
	}
	if category != "36" {
		t.Fatalf("category = %q, want %q", category, "36")
	}
	if len(points) != 2 {
		t.Fatalf("got %d points, want 2 (the 6016 row must be excluded)", len(points))
	}
	for _, p := range points {
		if p.rank == 3 {
			t.Fatalf("rank 3 came from category 6016 and must never enter category 36's history: "+
				"aligning it against a 07-10 rank of 50 fabricates a rank_delta of 47, got points %+v", points)
		}
	}
	// Oldest first.
	if points[0].rank != 50 || points[1].rank != 49 {
		t.Errorf("got ranks %d/%d, want 50/49 oldest-first", points[0].rank, points[1].rank)
	}
}
