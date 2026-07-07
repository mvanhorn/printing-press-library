package cli

import "testing"

func TestNovelGrowthAbsAndPctChange(t *testing.T) {
	dbPath := newTestDB(t)
	// Two snapshots within an 8w window: start 1000, end 1250.
	insertAccountSnapshot(t, dbPath, "alpha", 1000, 500, 100, 200, rfcAgo(6*week))
	insertAccountSnapshot(t, dbPath, "alpha", 1250, 600, 120, 240, rfcAgo(1*week))

	out := execTestJSON(t, newNovelGrowthCmd, dbPath, "--since", "8w")
	brands := asSlice(t, out, "brands")
	if len(brands) != 1 {
		t.Fatalf("want 1 brand, got %d", len(brands))
	}
	g := brands[0]
	if got := num(t, g, "start_followers"); got != 1000 {
		t.Errorf("start_followers = %v, want 1000", got)
	}
	if got := num(t, g, "end_followers"); got != 1250 {
		t.Errorf("end_followers = %v, want 1250", got)
	}
	if got := num(t, g, "abs_change"); got != 250 {
		t.Errorf("abs_change = %v, want 250 (end-start)", got)
	}
	if pct := num(t, g, "pct_change"); pct <= 0 {
		t.Errorf("pct_change = %v, want positive (growth)", pct)
	}
}

func TestNovelGrowthSingleSnapshotNote(t *testing.T) {
	dbPath := newTestDB(t)
	insertAccountSnapshot(t, dbPath, "solo", 1000, 500, 100, 200, rfcAgo(1*week))
	out := execTestJSON(t, newNovelGrowthCmd, dbPath, "--since", "8w")
	brands := asSlice(t, out, "brands")
	if len(brands) != 1 {
		t.Fatalf("want 1 brand, got %d", len(brands))
	}
	if got := num(t, brands[0], "abs_change"); got != 0 {
		t.Errorf("single snapshot abs_change = %v, want 0", got)
	}
	if _, ok := brands[0]["note"]; !ok {
		t.Errorf("expected a note for single-snapshot brand")
	}
}
