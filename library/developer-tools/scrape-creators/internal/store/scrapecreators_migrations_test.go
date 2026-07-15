// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func openTempStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestCreatorSnapshotTrajectory(t *testing.T) {
	ctx := context.Background()
	s := openTempStore(t)
	base := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	for i, fc := range []int64{100, 150, 130} {
		snap := CreatorSnapshot{Handle: "mrbeast", Platform: "tiktok", FollowerCount: fc, CapturedAt: base.Add(time.Duration(i) * time.Hour)}
		if err := InsertCreatorSnapshot(ctx, s.DB(), snap); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}
	// Different platform should not bleed into the trajectory.
	if err := InsertCreatorSnapshot(ctx, s.DB(), CreatorSnapshot{Handle: "mrbeast", Platform: "youtube", FollowerCount: 9, CapturedAt: base}); err != nil {
		t.Fatalf("insert other platform: %v", err)
	}

	traj, err := CreatorTrajectory(ctx, s.DB(), "mrbeast", "tiktok")
	if err != nil {
		t.Fatalf("trajectory: %v", err)
	}
	if len(traj) != 3 {
		t.Fatalf("len = %d, want 3", len(traj))
	}
	wantCounts := []int64{100, 150, 130}
	for i, p := range traj {
		if p.FollowerCount != wantCounts[i] {
			t.Fatalf("point %d follower = %d, want %d", i, p.FollowerCount, wantCounts[i])
		}
	}
}

func TestAdSnapshotDiff(t *testing.T) {
	ctx := context.Background()
	s := openTempStore(t)

	// First run: no prior.
	batch, prior, err := LatestAdSnapshot(ctx, s.DB(), "nike")
	if err != nil {
		t.Fatalf("latest(first): %v", err)
	}
	if batch != "" || len(prior) != 0 {
		t.Fatalf("expected empty first run, got batch=%q prior=%v", batch, prior)
	}

	run1 := map[string][]string{"facebook": {"a", "b"}, "tiktok": {"x"}}
	if err := InsertAdSnapshotBatch(ctx, s.DB(), "nike", run1, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("insert run1: %v", err)
	}

	batch, prior, err = LatestAdSnapshot(ctx, s.DB(), "nike")
	if err != nil {
		t.Fatalf("latest(after run1): %v", err)
	}
	if batch == "" {
		t.Fatalf("expected non-empty batch after run1")
	}
	if len(prior["facebook"]) != 2 || len(prior["tiktok"]) != 1 {
		t.Fatalf("prior mismatch: %v", prior)
	}

	// Second run later in time must become the new "latest".
	run2 := map[string][]string{"facebook": {"b", "c"}}
	if err := InsertAdSnapshotBatch(ctx, s.DB(), "nike", run2, time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("insert run2: %v", err)
	}
	batch2, prior2, err := LatestAdSnapshot(ctx, s.DB(), "nike")
	if err != nil {
		t.Fatalf("latest(after run2): %v", err)
	}
	if batch2 == batch {
		t.Fatalf("latest batch did not advance")
	}
	if len(prior2["facebook"]) != 2 || len(prior2["tiktok"]) != 0 {
		t.Fatalf("prior2 mismatch: %v", prior2)
	}
}
