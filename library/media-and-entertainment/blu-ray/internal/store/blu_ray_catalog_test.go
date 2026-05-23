package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestMigrateBluRayCatalogCreatesTables(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.MigrateBluRayCatalog(); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name IN ('releases_catalog', 'releases_fts', 'watchlist', 'price_history', 'sitemap_snapshot', 'upc_index')`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 6 {
		t.Fatalf("created table count = %d, want 6", count)
	}

	// PATCH: Price history index should cover retailer-filtered history lookups.
	var indexSQL string
	if err := s.DB().QueryRow(`SELECT sql FROM sqlite_master WHERE type='index' AND name='price_history_release_retailer_observed'`).Scan(&indexSQL); err != nil {
		t.Fatal(err)
	}
	if indexSQL != `CREATE INDEX price_history_release_retailer_observed ON price_history(release_id, retailer_id, observed_at)` {
		t.Fatalf("price history index SQL = %q", indexSQL)
	}
}

func testBluRayStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MigrateBluRayCatalog(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestBluRayCatalogDomainMethods(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		run  func(t *testing.T, s *Store)
	}{
		{
			name: "catalog rows search list get stats and empty upsert",
			run: func(t *testing.T, s *Store) {
				if err := s.UpsertCatalogRows(ctx, nil); err != nil {
					t.Fatalf("empty upsert: %v", err)
				}
				rows := []CatalogRow{
					{ID: 1, Kind: "bluray", Slug: "Fight-Club-Blu-ray", TitleNormalized: "Fight Club", Country: "US", YearHint: 1999, Lastmod: "2026-01-01"},
					{ID: 2, Kind: "4k", Slug: "Alien-4K-Blu-ray", TitleNormalized: "Alien", Country: "UK", YearHint: 1979, Lastmod: "2026-01-02"},
				}
				if err := s.UpsertCatalogRows(ctx, rows); err != nil {
					t.Fatalf("upsert catalog: %v", err)
				}
				found, err := s.SearchCatalog(ctx, CatalogSearchOpts{Query: "Fight*", Limit: 10})
				if err != nil {
					t.Fatalf("search catalog: %v", err)
				}
				if len(found) != 1 || found[0].ID != 1 {
					t.Fatalf("search rows = %#v, want id 1", found)
				}
				listed, err := s.ListCatalog(ctx, "4k", 10)
				if err != nil {
					t.Fatalf("list catalog: %v", err)
				}
				if len(listed) != 1 || listed[0].ID != 2 {
					t.Fatalf("list rows = %#v, want id 2", listed)
				}
				row, ok, err := s.GetRelease(ctx, 1)
				if err != nil || !ok || row.TitleNormalized != "Fight Club" {
					t.Fatalf("get release = %#v %v %v", row, ok, err)
				}
				if _, ok, err := s.GetRelease(ctx, 404); err != nil || ok {
					t.Fatalf("missing release ok=%v err=%v, want false nil", ok, err)
				}
				stats, err := s.CatalogStats(ctx)
				if err != nil {
					t.Fatalf("catalog stats: %v", err)
				}
				if stats.TotalRows != 2 || stats.RowsByKind["4k"] != 1 || stats.RowsByKind["bluray"] != 1 {
					t.Fatalf("stats = %#v", stats)
				}
			},
		},
		{
			name: "news rows empty and upsert",
			run: func(t *testing.T, s *Store) {
				if err := s.UpsertNewsRows(ctx, nil); err != nil {
					t.Fatalf("empty news upsert: %v", err)
				}
				if err := s.UpsertNewsRows(ctx, []NewsRow{{ID: 7, URL: "https://www.blu-ray.com/news/?id=7", Title: "Story", PublicationDate: "2026-05-17"}}); err != nil {
					t.Fatalf("news upsert: %v", err)
				}
				var title string
				if err := s.DB().QueryRow(`SELECT title FROM news_catalog WHERE id=7`).Scan(&title); err != nil {
					t.Fatal(err)
				}
				if title != "Story" {
					t.Fatalf("title = %q", title)
				}
			},
		},
		{
			name: "sitemap snapshot record and filtered list",
			run: func(t *testing.T, s *Store) {
				if err := s.RecordSitemapSnapshot(ctx, "sitemap_bluraymovies_0.xml.gz", 2, "hash-a"); err != nil {
					t.Fatalf("record snapshot: %v", err)
				}
				if err := s.RecordSitemapSnapshot(ctx, "sitemap_news.xml.gz", 1, "hash-b"); err != nil {
					t.Fatalf("record snapshot news: %v", err)
				}
				rows, err := s.ListSitemapSnapshots(ctx, "%bluray%", "")
				if err != nil {
					t.Fatalf("list snapshots: %v", err)
				}
				if len(rows) != 1 || rows[0].URLSetHash != "hash-a" {
					t.Fatalf("snapshots = %#v", rows)
				}
				rows, err = s.ListSitemapSnapshots(ctx, "%missing%", "")
				if err != nil || len(rows) != 0 {
					t.Fatalf("missing snapshots = %#v err=%v", rows, err)
				}
			},
		},
		{
			name: "watchlist add list remove mark",
			run: func(t *testing.T, s *Store) {
				if err := s.AddToWatchlist(ctx, 10, 14.99); err != nil {
					t.Fatalf("add watchlist: %v", err)
				}
				if err := s.UpdateWatchlistLow(ctx, 10, 12.50); err != nil {
					t.Fatalf("update low: %v", err)
				}
				if err := s.MarkWatchlistAlerted(ctx, 10, 11.50); err != nil {
					t.Fatalf("mark alerted: %v", err)
				}
				rows, err := s.ListWatchlist(ctx)
				if err != nil {
					t.Fatalf("list watchlist: %v", err)
				}
				if len(rows) != 1 || rows[0].ReleaseID != 10 || !rows[0].LowSeen.Valid || !rows[0].AlertedAt.Valid {
					t.Fatalf("watchlist rows = %#v", rows)
				}
				n, err := s.RemoveFromWatchlist(ctx, 10)
				if err != nil || n != 1 {
					t.Fatalf("remove n=%d err=%v", n, err)
				}
				n, err = s.RemoveFromWatchlist(ctx, 10)
				if err != nil || n != 0 {
					t.Fatalf("remove missing n=%d err=%v", n, err)
				}
			},
		},
		{
			name: "price history record and retailer filter",
			run: func(t *testing.T, s *Store) {
				if err := s.RecordPrice(ctx, PriceObservation{ReleaseID: 20, RetailerID: 1, ObservedAt: "2026-01-01T00:00:00Z", Price: 19.99}); err != nil {
					t.Fatalf("record price: %v", err)
				}
				if err := s.RecordPrice(ctx, PriceObservation{ReleaseID: 20, RetailerID: 2, ObservedAt: "2026-01-02T00:00:00Z", Price: 17.99}); err != nil {
					t.Fatalf("record price 2: %v", err)
				}
				rows, err := s.GetPriceHistory(ctx, 20, 2)
				if err != nil {
					t.Fatalf("history: %v", err)
				}
				if len(rows) != 1 || rows[0].RetailerID != 2 {
					t.Fatalf("history rows = %#v", rows)
				}
				rows, err = s.GetPriceHistory(ctx, 404, 0)
				if err != nil || len(rows) != 0 {
					t.Fatalf("missing history rows = %#v err=%v", rows, err)
				}
			},
		},
		{
			name: "resolve upc hit and miss",
			run: func(t *testing.T, s *Store) {
				if _, err := s.DB().Exec(`INSERT INTO upc_index(upc, release_id) VALUES(?, ?)`, "012345678905", 42); err != nil {
					t.Fatal(err)
				}
				id, ok, err := s.ResolveUPC(ctx, "012345678905")
				if err != nil || !ok || id != 42 {
					t.Fatalf("resolve upc id=%d ok=%v err=%v", id, ok, err)
				}
				id, ok, err = s.ResolveUPC(ctx, "000000000000")
				if err != nil || ok || id != 0 {
					t.Fatalf("resolve missing id=%d ok=%v err=%v", id, ok, err)
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, testBluRayStore(t))
		})
	}
}

func TestBluRayCatalogDomainMethodsMissingSchema(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_, err = s.GetPriceHistory(context.Background(), 1, 0)
	if err == nil || err == sql.ErrNoRows {
		t.Fatalf("missing schema err = %v, want table error", err)
	}
}
