package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestOfferSnapshotRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "snap.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	for _, sn := range []OfferSnapshot{
		{OfferID: "o1", SyncedAt: "2026-01-01T00:00:00Z", Keyword: "kw", PriceCNY: 7.99, RepurchasePct: 11, BookedCount: 100},
		{OfferID: "o1", SyncedAt: "2026-01-02T00:00:00Z", Keyword: "kw", PriceCNY: 6.99, RepurchasePct: 15, BookedCount: 150},
	} {
		if err := s.InsertOfferSnapshot(ctx, sn); err != nil {
			t.Fatal(err)
		}
	}

	snaps, err := s.OfferSnapshots(ctx, "o1")
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 2 {
		t.Fatalf("want 2 snapshots, got %d", len(snaps))
	}
	// Oldest-first ordering so drift can diff first vs last.
	if snaps[0].PriceCNY != 7.99 || snaps[1].PriceCNY != 6.99 {
		t.Errorf("snapshot order/values wrong: %+v", snaps)
	}
	// Keyword target matches too.
	byKw, err := s.OfferSnapshots(ctx, "kw")
	if err != nil {
		t.Fatal(err)
	}
	if len(byKw) != 2 {
		t.Fatalf("keyword snapshot lookup want 2, got %d", len(byKw))
	}
}

func TestOfferQueries(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	data, _ := json.Marshal(map[string]any{
		"offer_id":           "o1",
		"title":              "透明硅胶手机壳",
		"keyword":            "手机壳",
		"supplier_member_id": "m1",
		"supplier_name":      "示例供应商",
	})
	if err := s.Upsert("offer", "o1", data); err != nil {
		t.Fatal(err)
	}

	if rows, err := s.OffersByKeyword(ctx, "手机壳", 0); err != nil || len(rows) != 1 {
		t.Fatalf("OffersByKeyword want 1 (err=%v), got %d", err, len(rows))
	}
	if rows, err := s.OffersByKeyword(ctx, "nomatch", 0); err != nil || len(rows) != 0 {
		t.Fatalf("OffersByKeyword nomatch want 0 (err=%v), got %d", err, len(rows))
	}
	// LIKE substring (CJK), since FTS does not tokenize Chinese.
	if rows, err := s.FindOffers(ctx, "硅胶", 10); err != nil || len(rows) != 1 {
		t.Fatalf("FindOffers want 1 (err=%v), got %d", err, len(rows))
	}
	if rows, err := s.OffersBySupplier(ctx, "m1"); err != nil || len(rows) != 1 {
		t.Fatalf("OffersBySupplier want 1 (err=%v), got %d", err, len(rows))
	}
}
