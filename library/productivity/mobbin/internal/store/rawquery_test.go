// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"
)

// Cover the novel Mobbin store surface: migrateExtras creates the domain
// tables, RawQuery reads them, and RawQuery rejects non-read statements.
func TestRawQueryAndExtraTables(t *testing.T) {
	ctx := context.Background()
	db, err := OpenWithContext(ctx, filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("OpenWithContext() error = %v", err)
	}
	defer db.Close()

	if _, err := db.DB().ExecContext(ctx,
		`INSERT INTO screens (id, app_id, platform) VALUES ('scr_1', 'app_1', 'web')`); err != nil {
		t.Fatalf("insert into screens error = %v (migrateExtras should have created it)", err)
	}

	rows, err := db.RawQuery(ctx, "SELECT id, app_id, platform FROM screens")
	if err != nil {
		t.Fatalf("RawQuery() error = %v", err)
	}
	if len(rows) != 1 || rows[0]["id"] != "scr_1" || rows[0]["platform"] != "web" {
		t.Fatalf("RawQuery() rows = %#v", rows)
	}

	if _, err := db.RawQuery(ctx, "DELETE FROM screens"); err == nil {
		t.Fatal("RawQuery(delete) error = nil, want read-only guard error")
	}
}
