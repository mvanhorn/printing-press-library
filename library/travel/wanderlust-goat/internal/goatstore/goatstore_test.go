package goatstore

import (
	"context"
	"path/filepath"
	"testing"
)

func freshStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(context.Background(), filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestUpsertAndQueryRadius(t *testing.T) {
	ctx := context.Background()
	s := freshStore(t)
	p := Place{
		ID: "test:1", Source: "tabelog", Intent: "food", Name: "Test Cafe",
		NameLocal: "テスト", Lat: 35.6895, Lng: 139.6917, Country: "JP",
		CitySlug: "tokyo", Trust: 0.9, WhySpecial: "Brief test",
	}
	if err := s.UpsertPlace(ctx, p); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := s.QueryRadius(ctx, 35.6895, 139.6917, 200, "food")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if got[0].Name != "Test Cafe" {
		t.Errorf("name: got %q", got[0].Name)
	}
}

func TestQuotesForPlace(t *testing.T) {
	ctx := context.Background()
	s := freshStore(t)
	t1 := RedditThread{
		ID: "r1", Subreddit: "japantravel", Title: "Best Tokyo coffee",
		Score: 50, NumComments: 12,
		Body: "I went to Kohi Bibi yesterday and it was amazing",
	}
	t2 := RedditThread{
		ID: "r2", Subreddit: "tokyo", Title: "Cheap eats",
		Score: 30, NumComments: 5,
		Body: "Random ramen post nothing special",
	}
	if err := s.UpsertRedditThread(ctx, t1); err != nil {
		t.Fatalf("upsert thread 1: %v", err)
	}
	if err := s.UpsertRedditThread(ctx, t2); err != nil {
		t.Fatalf("upsert thread 2: %v", err)
	}
	got, err := s.QuotesForPlace(ctx, []string{"Kohi Bibi"})
	if err != nil {
		t.Fatalf("quotes: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 thread mentioning Kohi Bibi, got %d", len(got))
	}
	if got[0].ID != "r1" {
		t.Errorf("expected r1, got %s", got[0].ID)
	}
}

func TestRouteCache(t *testing.T) {
	ctx := context.Background()
	s := freshStore(t)

	_, _, _, found, err := s.LookupRoute(ctx, 35.0, 139.0, 35.1, 139.1)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if found {
		t.Error("expected not-found on empty cache")
	}

	if err := s.CacheRoute(ctx, 35.0, 139.0, 35.1, 139.1, 1234.5, 600.0, "polyline-data"); err != nil {
		t.Fatalf("cache: %v", err)
	}
	dist, dur, poly, found, err := s.LookupRoute(ctx, 35.0, 139.0, 35.1, 139.1)
	if err != nil || !found {
		t.Fatalf("expected found after cache: err=%v found=%v", err, found)
	}
	if dist != 1234.5 || dur != 600.0 || poly != "polyline-data" {
		t.Errorf("cache mismatch: dist=%v dur=%v poly=%q", dist, dur, poly)
	}
}

func TestCoverage(t *testing.T) {
	ctx := context.Background()
	s := freshStore(t)
	if err := s.SaveSync(ctx, "tabelog", "tokyo", "cursor-1", 42); err != nil {
		t.Fatalf("save sync: %v", err)
	}
	rows, err := s.Coverage(ctx, "tokyo")
	if err != nil {
		t.Fatalf("coverage: %v", err)
	}
	if len(rows) != 1 || rows[0].Source != "tabelog" || rows[0].RowCount != 42 {
		t.Errorf("coverage mismatch: %+v", rows)
	}
}
