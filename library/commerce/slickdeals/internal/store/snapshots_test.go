// Copyright 2026 david. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"path/filepath"
	"testing"
	"time"
)

func openTempStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "snap.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func mustInsert(t *testing.T, s *Store, snap DealSnapshot) DealSnapshot {
	t.Helper()
	if err := s.InsertSnapshot(&snap); err != nil {
		t.Fatalf("InsertSnapshot: %v", err)
	}
	if snap.ID == 0 {
		t.Fatalf("InsertSnapshot did not set ID")
	}
	return snap
}

func TestEnsureSnapshotsSchema_Idempotent(t *testing.T) {
	s := openTempStore(t)
	for i := 0; i < 3; i++ {
		if err := s.EnsureSnapshotsSchema(); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	// Verify table exists.
	var name string
	if err := s.DB().QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='deal_snapshots'`,
	).Scan(&name); err != nil {
		t.Fatalf("table not created: %v", err)
	}
	if name != "deal_snapshots" {
		t.Fatalf("wrong table: %q", name)
	}
}

func TestInsertSnapshot_RoundTrip(t *testing.T) {
	s := openTempStore(t)
	at := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	mustInsert(t, s, DealSnapshot{
		DealID:     "19510173",
		CapturedAt: at,
		Price:      19.99,
		ListPrice:  29.99,
		Thumbs:     55,
		Comments:   12,
		Views:      300,
		Merchant:   "costco",
		Category:   "tech",
		Title:      "Test Deal",
		Link:       "https://slickdeals.net/f/19510173-test",
		Raw:        `{"raw":"json"}`,
	})

	got, err := s.QueryDeals(DealFilter{DealID: "19510173"})
	if err != nil {
		t.Fatalf("QueryDeals: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len=%d want 1", len(got))
	}
	row := got[0]
	if row.DealID != "19510173" || row.Thumbs != 55 || row.Merchant != "costco" {
		t.Fatalf("round-trip mismatch: %+v", row)
	}
	if row.Title != "Test Deal" || row.Price != 19.99 {
		t.Fatalf("scalar mismatch: %+v", row)
	}
	if !row.CapturedAt.Equal(at) {
		t.Fatalf("time mismatch: got %s want %s", row.CapturedAt, at)
	}
}

func TestInsertSnapshot_DefaultsCapturedAt(t *testing.T) {
	s := openTempStore(t)
	before := time.Now().Add(-time.Second)
	snap := DealSnapshot{DealID: "abc", Title: "x", Link: "y"}
	if err := s.InsertSnapshot(&snap); err != nil {
		t.Fatalf("InsertSnapshot: %v", err)
	}
	after := time.Now().Add(time.Second)
	if snap.CapturedAt.Before(before) || snap.CapturedAt.After(after) {
		t.Fatalf("CapturedAt not defaulted to now: %s", snap.CapturedAt)
	}
}

func TestQueryDeals_FilterCombinations(t *testing.T) {
	s := openTempStore(t)
	now := time.Now().UTC()
	mustInsert(t, s, DealSnapshot{DealID: "1", CapturedAt: now.Add(-3 * time.Hour), Merchant: "costco", Category: "tech", Thumbs: 100, Title: "a", Link: "l1"})
	mustInsert(t, s, DealSnapshot{DealID: "2", CapturedAt: now.Add(-2 * time.Hour), Merchant: "amazon", Category: "home", Thumbs: 50, Title: "b", Link: "l2"})
	mustInsert(t, s, DealSnapshot{DealID: "3", CapturedAt: now.Add(-1 * time.Hour), Merchant: "Costco", Category: "tech", Thumbs: 20, Title: "c", Link: "l3"})

	// Case-insensitive merchant match — should hit both "costco" and "Costco".
	res, err := s.QueryDeals(DealFilter{Store: "costco"})
	if err != nil {
		t.Fatalf("QueryDeals: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("store filter len=%d want 2", len(res))
	}

	// Min thumbs filter.
	res, err = s.QueryDeals(DealFilter{MinThumbs: 60})
	if err != nil {
		t.Fatalf("QueryDeals: %v", err)
	}
	if len(res) != 1 || res[0].DealID != "1" {
		t.Fatalf("min thumbs filter: %+v", res)
	}

	// Category filter (case-insensitive).
	res, err = s.QueryDeals(DealFilter{Category: "TECH"})
	if err != nil {
		t.Fatalf("QueryDeals: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("category filter len=%d want 2", len(res))
	}

	// Since filter.
	res, err = s.QueryDeals(DealFilter{Since: now.Add(-90 * time.Minute)})
	if err != nil {
		t.Fatalf("QueryDeals: %v", err)
	}
	if len(res) != 1 || res[0].DealID != "3" {
		t.Fatalf("since filter: %+v", res)
	}

	// Limit.
	res, err = s.QueryDeals(DealFilter{Limit: 2})
	if err != nil {
		t.Fatalf("QueryDeals: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("limit: %d want 2", len(res))
	}

	// Combined filter.
	res, err = s.QueryDeals(DealFilter{Store: "costco", Category: "tech", MinThumbs: 50})
	if err != nil {
		t.Fatalf("QueryDeals: %v", err)
	}
	if len(res) != 1 || res[0].DealID != "1" {
		t.Fatalf("combined filter: %+v", res)
	}
}

func TestQueryDeals_LatestDedupes(t *testing.T) {
	s := openTempStore(t)
	base := time.Now().UTC()

	// Three observations of deal_id=1, two of deal_id=2.
	mustInsert(t, s, DealSnapshot{DealID: "1", CapturedAt: base.Add(-3 * time.Hour), Thumbs: 10, Title: "t", Link: "l"})
	mustInsert(t, s, DealSnapshot{DealID: "1", CapturedAt: base.Add(-2 * time.Hour), Thumbs: 25, Title: "t", Link: "l"})
	mustInsert(t, s, DealSnapshot{DealID: "1", CapturedAt: base.Add(-1 * time.Hour), Thumbs: 50, Title: "t", Link: "l"})
	mustInsert(t, s, DealSnapshot{DealID: "2", CapturedAt: base.Add(-90 * time.Minute), Thumbs: 5, Title: "t", Link: "l"})
	mustInsert(t, s, DealSnapshot{DealID: "2", CapturedAt: base.Add(-30 * time.Minute), Thumbs: 7, Title: "t", Link: "l"})

	res, err := s.QueryDeals(DealFilter{Latest: true})
	if err != nil {
		t.Fatalf("QueryDeals: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("latest dedupe len=%d want 2", len(res))
	}
	byID := map[string]int{}
	for _, r := range res {
		byID[r.DealID] = r.Thumbs
	}
	if byID["1"] != 50 {
		t.Fatalf("expected latest thumbs for deal 1 to be 50, got %d", byID["1"])
	}
	if byID["2"] != 7 {
		t.Fatalf("expected latest thumbs for deal 2 to be 7, got %d", byID["2"])
	}

	// Latest + filter combination.
	res, err = s.QueryDeals(DealFilter{Latest: true, MinThumbs: 10})
	if err != nil {
		t.Fatalf("QueryDeals: %v", err)
	}
	if len(res) != 1 || res[0].DealID != "1" {
		t.Fatalf("latest+min-thumbs filter: %+v", res)
	}
}

func TestQuerySnapshotsSince_LatestPerDeal(t *testing.T) {
	s := openTempStore(t)
	now := time.Now().UTC()
	mustInsert(t, s, DealSnapshot{DealID: "1", CapturedAt: now.Add(-3 * time.Hour), Thumbs: 5, Title: "t", Link: "l"})
	mustInsert(t, s, DealSnapshot{DealID: "1", CapturedAt: now.Add(-30 * time.Minute), Thumbs: 9, Title: "t", Link: "l"})
	mustInsert(t, s, DealSnapshot{DealID: "2", CapturedAt: now.Add(-10 * time.Minute), Thumbs: 3, Title: "t", Link: "l"})

	res, err := s.QuerySnapshotsSince(now.Add(-time.Hour), 0)
	if err != nil {
		t.Fatalf("QuerySnapshotsSince: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("len=%d want 2", len(res))
	}
}

func TestTopStores_Aggregation(t *testing.T) {
	s := openTempStore(t)
	now := time.Now().UTC()
	// costco: 3 distinct deals; max thumbs 100; avg includes all rows.
	mustInsert(t, s, DealSnapshot{DealID: "a", CapturedAt: now.Add(-time.Hour), Merchant: "costco", Thumbs: 100, Title: "t", Link: "l"})
	mustInsert(t, s, DealSnapshot{DealID: "b", CapturedAt: now.Add(-2 * time.Hour), Merchant: "costco", Thumbs: 50, Title: "t", Link: "l"})
	mustInsert(t, s, DealSnapshot{DealID: "c", CapturedAt: now.Add(-3 * time.Hour), Merchant: "costco", Thumbs: 30, Title: "t", Link: "l"})
	// amazon: 1 deal.
	mustInsert(t, s, DealSnapshot{DealID: "x", CapturedAt: now.Add(-time.Hour), Merchant: "amazon", Thumbs: 200, Title: "t", Link: "l"})
	// Outside window.
	mustInsert(t, s, DealSnapshot{DealID: "y", CapturedAt: now.Add(-30 * 24 * time.Hour), Merchant: "stale", Thumbs: 1, Title: "t", Link: "l"})

	stats, err := s.TopStores(24*time.Hour, 0)
	if err != nil {
		t.Fatalf("TopStores: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("len=%d want 2 (stale outside window)", len(stats))
	}
	byMerchant := map[string]StoreStats{}
	for _, s := range stats {
		byMerchant[s.Merchant] = s
	}
	c, ok := byMerchant["costco"]
	if !ok {
		t.Fatalf("costco missing: %+v", stats)
	}
	if c.DealCount != 3 {
		t.Fatalf("costco deal_count=%d want 3", c.DealCount)
	}
	if c.MaxThumbs != 100 {
		t.Fatalf("costco max_thumbs=%d want 100", c.MaxThumbs)
	}
	wantAvg := float64(100+50+30) / 3
	if c.AvgThumbs < wantAvg-0.01 || c.AvgThumbs > wantAvg+0.01 {
		t.Fatalf("costco avg_thumbs=%f want ~%f", c.AvgThumbs, wantAvg)
	}

	a, ok := byMerchant["amazon"]
	if !ok {
		t.Fatalf("amazon missing")
	}
	if a.DealCount != 1 || a.MaxThumbs != 200 {
		t.Fatalf("amazon stats wrong: %+v", a)
	}

	// Ordering: costco has 3 deals, amazon 1 — costco first.
	if stats[0].Merchant != "costco" {
		t.Fatalf("expected costco first, got %s", stats[0].Merchant)
	}

	// Limit.
	stats, err = s.TopStores(24*time.Hour, 1)
	if err != nil {
		t.Fatalf("TopStores limit: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("limit len=%d want 1", len(stats))
	}
}

func TestTopStores_WindowZeroAllTime(t *testing.T) {
	s := openTempStore(t)
	now := time.Now().UTC()
	mustInsert(t, s, DealSnapshot{DealID: "a", CapturedAt: now, Merchant: "x", Thumbs: 1, Title: "t", Link: "l"})
	mustInsert(t, s, DealSnapshot{DealID: "b", CapturedAt: now.Add(-365 * 24 * time.Hour), Merchant: "y", Thumbs: 1, Title: "t", Link: "l"})

	stats, err := s.TopStores(0, 0)
	if err != nil {
		t.Fatalf("TopStores: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("len=%d want 2 (window=0 means all time)", len(stats))
	}
}

func TestThumbsVelocity_DeltaComputation(t *testing.T) {
	s := openTempStore(t)
	base := time.Now().UTC().Truncate(time.Second)
	mustInsert(t, s, DealSnapshot{DealID: "v1", CapturedAt: base.Add(-3 * time.Hour), Thumbs: 10, Title: "t", Link: "l"})
	mustInsert(t, s, DealSnapshot{DealID: "v1", CapturedAt: base.Add(-2 * time.Hour), Thumbs: 25, Title: "t", Link: "l"})
	mustInsert(t, s, DealSnapshot{DealID: "v1", CapturedAt: base.Add(-1 * time.Hour), Thumbs: 22, Title: "t", Link: "l"})

	pts, err := s.ThumbsVelocity("v1")
	if err != nil {
		t.Fatalf("ThumbsVelocity: %v", err)
	}
	if len(pts) != 3 {
		t.Fatalf("len=%d want 3", len(pts))
	}
	if pts[0].Delta != 0 {
		t.Fatalf("first delta=%d want 0", pts[0].Delta)
	}
	if pts[1].Thumbs != 25 || pts[1].Delta != 15 {
		t.Fatalf("second point: %+v want thumbs=25 delta=15", pts[1])
	}
	if pts[2].Thumbs != 22 || pts[2].Delta != -3 {
		t.Fatalf("third point: %+v want thumbs=22 delta=-3", pts[2])
	}
}

func TestThumbsVelocity_NoDataNotError(t *testing.T) {
	s := openTempStore(t)
	pts, err := s.ThumbsVelocity("does-not-exist")
	if err != nil {
		t.Fatalf("ThumbsVelocity empty: %v", err)
	}
	if len(pts) != 0 {
		t.Fatalf("len=%d want 0", len(pts))
	}
}

func TestThumbsVelocity_SinglePoint(t *testing.T) {
	s := openTempStore(t)
	mustInsert(t, s, DealSnapshot{DealID: "solo", Thumbs: 42, Title: "t", Link: "l"})
	pts, err := s.ThumbsVelocity("solo")
	if err != nil {
		t.Fatalf("ThumbsVelocity: %v", err)
	}
	if len(pts) != 1 {
		t.Fatalf("len=%d want 1", len(pts))
	}
	if pts[0].Thumbs != 42 || pts[0].Delta != 0 {
		t.Fatalf("single point delta should be 0, got %+v", pts[0])
	}
}
