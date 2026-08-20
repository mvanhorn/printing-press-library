// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"
)

// expectedCoffeeTables lists the regular (non-virtual) tables the
// coffee-goat schema must create. Virtual FTS5 tables are checked
// separately because PRAGMA table_list reports them with `type=table`
// but doesn't appear in some sqlite_master scans depending on driver
// build flags.
var expectedCoffeeTables = []string{
	"roasters",
	"roaster_products",
	"reviews",
	"youtube_reviews",
	"beans",
	"brews",
	"watchlist",
	"palate_profiles",
	"coffee_sync_state",
}

var expectedCoffeeFTSTables = []string{
	"roaster_products_fts",
	"youtube_reviews_fts",
}

var expectedCoffeeIndexes = []string{
	"idx_roaster_products_origin",
	"idx_roaster_products_process",
	"idx_roaster_products_in_stock",
	"idx_roaster_products_first_seen",
	"idx_reviews_roaster",
	"idx_reviews_score",
	"idx_youtube_reviews_creator",
	"idx_youtube_reviews_published",
	"idx_beans_roaster",
	"idx_beans_product",
	"idx_brews_bean",
	"idx_brews_method",
	"idx_brews_rating",
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := OpenWithContext(context.Background(), filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestCoffeeSchemaCreatesAllTables(t *testing.T) {
	s := openTestStore(t)

	for _, table := range expectedCoffeeTables {
		var got string
		err := s.DB().QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`,
			table,
		).Scan(&got)
		if err != nil {
			t.Errorf("table %q missing: %v", table, err)
		}
	}

	for _, fts := range expectedCoffeeFTSTables {
		var got string
		err := s.DB().QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`,
			fts,
		).Scan(&got)
		if err != nil {
			t.Errorf("FTS table %q missing: %v", fts, err)
		}
	}

	for _, idx := range expectedCoffeeIndexes {
		var got string
		err := s.DB().QueryRow(
			`SELECT name FROM sqlite_master WHERE type='index' AND name=?`,
			idx,
		).Scan(&got)
		if err != nil {
			t.Errorf("index %q missing: %v", idx, err)
		}
	}
}

func TestCoffeeSchemaIsIdempotent(t *testing.T) {
	s := openTestStore(t)
	// Re-running the schema must not error.
	if err := s.EnsureCoffeeSchema(context.Background()); err != nil {
		t.Fatalf("second EnsureCoffeeSchema call failed: %v", err)
	}
	if err := s.EnsureCoffeeSchema(context.Background()); err != nil {
		t.Fatalf("third EnsureCoffeeSchema call failed: %v", err)
	}
}

func TestUpsertRoasterProductRoundtrips(t *testing.T) {
	s := openTestStore(t)
	err := s.UpsertRoasterProduct("onyx", "test-handle", map[string]any{
		"title":     "Test Bean Ethiopia Natural",
		"origin":    "Ethiopia",
		"process":   "natural",
		"in_stock":  1,
		"body_text": "Bright florals and stonefruit",
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	var title string
	err = s.DB().QueryRow(
		`SELECT title FROM roaster_products WHERE roaster_slug=? AND handle=?`,
		"onyx", "test-handle",
	).Scan(&title)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if title != "Test Bean Ethiopia Natural" {
		t.Errorf("title = %q, want %q", title, "Test Bean Ethiopia Natural")
	}

	// FTS5 should match.
	var ftsTitle string
	err = s.DB().QueryRow(
		`SELECT title FROM roaster_products_fts WHERE roaster_products_fts MATCH 'ethiopia'`,
	).Scan(&ftsTitle)
	if err != nil {
		t.Fatalf("FTS match: %v", err)
	}
}

func TestSaveCoffeeSyncStateUpserts(t *testing.T) {
	s := openTestStore(t)
	if err := s.SaveCoffeeSyncState("shopify", "ok", 42); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := s.SaveCoffeeSyncState("shopify", "ok", 100); err != nil {
		t.Fatalf("second save: %v", err)
	}
	var n int
	if err := s.DB().QueryRow(`SELECT item_count FROM coffee_sync_state WHERE source=?`, "shopify").Scan(&n); err != nil {
		t.Fatalf("read: %v", err)
	}
	if n != 100 {
		t.Errorf("item_count = %d, want 100", n)
	}
}
