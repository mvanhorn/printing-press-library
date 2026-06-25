package store

import (
	"context"
	"path/filepath"
	"testing"
)

// TestChartSnapshotDedup verifies that inserting the same chart twice at the
// same captured_at does not produce duplicate rows (UNIQUE ... ON CONFLICT
// REPLACE). Two `top` runs in the same Unix second previously double-inserted
// every row and corrupted `movers` output.
func TestChartSnapshotDedup(t *testing.T) {
	db := filepath.Join(t.TempDir(), "gp.db")
	s, err := Open(db)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()
	rows := []ChartRow{
		{Rank: 1, AppID: "a", Title: "A", Score: 4.5},
		{Rank: 2, AppID: "b", Title: "B", Score: 4.2},
	}
	const at = int64(1781179000)
	for i := 0; i < 2; i++ { // same captured_at twice
		if err := s.InsertChartSnapshot(ctx, "topselling_free", "GAME", "us", at, rows); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}
	got, err := s.ChartSnapshotAt(ctx, "topselling_free", "GAME", "us", at)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 rows after double-insert, got %d (duplicate-row regression)", len(got))
	}
}

func TestKeywordRankRoundTrip(t *testing.T) {
	db := filepath.Join(t.TempDir(), "gp.db")
	s, err := Open(db)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.InsertKeywordRank(ctx, "puzzle", "us", "com.x", 1781179000, 5, 50); err != nil {
		t.Fatalf("insert keyword rank: %v", err)
	}
	// Re-insert at the same captured_at with a corrected rank: must replace,
	// not duplicate (UNIQUE ... ON CONFLICT REPLACE).
	if err := s.InsertKeywordRank(ctx, "puzzle", "us", "com.x", 1781179000, 3, 80); err != nil {
		t.Fatalf("re-insert keyword rank: %v", err)
	}
	pts, err := s.KeywordRankSeries(ctx, "puzzle", "us", "com.x")
	if err != nil {
		t.Fatalf("series: %v", err)
	}
	if len(pts) != 1 || pts[0].Rank != 3 {
		t.Fatalf("expected one point rank 3 after same-second replace, got %+v", pts)
	}
}
