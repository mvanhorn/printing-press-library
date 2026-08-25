// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"
)

// Prove the domain population -> query path end-to-end, offline: upsert an app,
// screen, and screen_pattern via the novel write methods, then run the same
// JOIN shape `bench` uses and assert the row comes back. No network.
func TestDomainUpsertFeedsBenchJoin(t *testing.T) {
	ctx := context.Background()
	db, err := OpenWithContext(ctx, filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("OpenWithContext() error = %v", err)
	}
	defer db.Close()

	if err := db.UpsertApp(ctx, map[string]any{
		"id":            "app_1",
		"appName":       "Stripe",
		"platform":      "web",
		"appCategories": []any{"fintech"},
	}); err != nil {
		t.Fatalf("UpsertApp() error = %v", err)
	}
	if err := db.UpsertScreen(ctx, map[string]any{
		"id":         "scr_1",
		"appId":      "app_1",
		"platform":   "web",
		"capturedAt": "2026-01-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("UpsertScreen() error = %v", err)
	}
	if err := db.UpsertScreenPattern(ctx, "scr_1", "paywall"); err != nil {
		t.Fatalf("UpsertScreenPattern() error = %v", err)
	}

	// apps.slug must be populated for drift/app lookups.
	slugRows, err := db.RawQuery(ctx, "SELECT slug FROM apps WHERE id = 'app_1'")
	if err != nil {
		t.Fatalf("slug RawQuery() error = %v", err)
	}
	if len(slugRows) != 1 || slugRows[0]["slug"] != "stripe-web-app_1" {
		t.Fatalf("apps.slug = %#v, want stripe-web-app_1", slugRows)
	}

	// The bench JOIN: screens x screen_patterns x apps, filtered by pattern.
	rows, err := db.RawQuery(ctx, `SELECT screens.app_id, apps.app_name, COUNT(*) AS n
FROM screens JOIN screen_patterns sp ON sp.screen_id=screens.id
LEFT JOIN apps ON apps.id=screens.app_id
WHERE sp.pattern_slug='paywall' AND screens.platform='web' AND apps.app_categories LIKE '%fintech%'
GROUP BY screens.app_id, apps.app_name ORDER BY n DESC`)
	if err != nil {
		t.Fatalf("bench JOIN RawQuery() error = %v", err)
	}
	if len(rows) != 1 || rows[0]["app_name"] != "Stripe" {
		t.Fatalf("bench JOIN rows = %#v, want one Stripe row", rows)
	}
}
