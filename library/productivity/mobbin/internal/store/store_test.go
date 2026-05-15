// Copyright 2026 darin-kishore. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"
)

// PATCH: Cover the local SQLite store migration, FTS, and read-only SQL guard.
func TestStoreSearchAndRawQuery(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if err := db.UpsertApp(ctx, map[string]any{
		"id":            "app_1",
		"slug":          "figma",
		"appName":       "Figma",
		"platform":      "web",
		"appCategories": []any{"design"},
	}); err != nil {
		t.Fatalf("UpsertApp() error = %v", err)
	}

	rows, err := db.SearchApps(ctx, "Figma", 5)
	if err != nil {
		t.Fatalf("SearchApps() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("SearchApps() rows = %d, want 1", len(rows))
	}

	rawRows, err := db.RawQuery(ctx, "select id, app_name from apps")
	if err != nil {
		t.Fatalf("RawQuery() error = %v", err)
	}
	if len(rawRows) != 1 || rawRows[0]["app_name"] != "Figma" {
		t.Fatalf("RawQuery() rows = %#v", rawRows)
	}

	if _, err := db.RawQuery(ctx, "delete from apps"); err == nil {
		t.Fatal("RawQuery(delete) error = nil, want error")
	}
}
