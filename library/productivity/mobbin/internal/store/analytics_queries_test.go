// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

// Exercise the SELECTs the analytics and tail commands run against the domain
// tables, proving they are valid over a populated store. No network.
func TestAnalyticsAndTailQueries(t *testing.T) {
	ctx := context.Background()
	db, err := OpenWithContext(ctx, filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("OpenWithContext() error = %v", err)
	}
	defer db.Close()

	if err := db.UpsertApp(ctx, map[string]any{"id": "app_1", "appName": "Stripe", "platform": "web"}); err != nil {
		t.Fatalf("UpsertApp() error = %v", err)
	}
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("scr_%d", i)
		if err := db.UpsertScreen(ctx, map[string]any{
			"id": id, "appId": "app_1", "platform": "web",
			"capturedAt": fmt.Sprintf("2026-01-0%dT00:00:00Z", i+1),
		}); err != nil {
			t.Fatalf("UpsertScreen() error = %v", err)
		}
		if err := db.UpsertScreenPattern(ctx, id, "paywall"); err != nil {
			t.Fatalf("UpsertScreenPattern() error = %v", err)
		}
	}

	// analytics: top patterns by screen_patterns count.
	top, err := db.RawQuery(ctx, `SELECT pattern_slug, COUNT(*) AS screens FROM screen_patterns
GROUP BY pattern_slug ORDER BY screens DESC LIMIT 5`)
	if err != nil {
		t.Fatalf("top patterns query error = %v", err)
	}
	if len(top) != 1 || top[0]["pattern_slug"] != "paywall" || fmt.Sprint(top[0]["screens"]) != "3" {
		t.Fatalf("top patterns = %#v", top)
	}

	// tail: recent rows across app_versions/screens/flows, newest first.
	recent, err := db.RawQuery(ctx, `SELECT 'app_version' AS kind, id, app_id, captured_at FROM app_versions
UNION ALL
SELECT 'screen' AS kind, id, app_id, captured_at FROM screens
UNION ALL
SELECT 'flow' AS kind, id, app_id, captured_at FROM flows
ORDER BY captured_at DESC LIMIT 20`)
	if err != nil {
		t.Fatalf("tail query error = %v", err)
	}
	if len(recent) != 3 || recent[0]["id"] != "scr_2" {
		t.Fatalf("tail rows = %#v", recent)
	}
}
