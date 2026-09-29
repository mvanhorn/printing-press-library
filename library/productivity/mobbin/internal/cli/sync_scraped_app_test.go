// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/mobbin/internal/appscrape"
	"github.com/mvanhorn/printing-press-library/library/productivity/mobbin/internal/store"
)

// The fixture's only version was published 2026-08-03 while its screens were
// created 2026-07-13. Sync must store the published date from
// appInfo.appVersions, not a screen's createdAt, and keep screen image URLs.
func TestStoreScrapedApp_UsesPageVersionsAndScreenURLs(t *testing.T) {
	ctx := context.Background()
	html, err := os.ReadFile(filepath.Join("..", "appscrape", "testdata", "app_page_rsc.html"))
	if err != nil {
		t.Fatal(err)
	}
	const appID = "72304e20-0d6a-4030-be48-f53a9831e891"
	payload, err := appscrape.Parse(string(html), "brick-ios-"+appID)
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenWithContext(ctx, filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	counts := storeScrapedApp(ctx, db, map[string]any{"id": appID, "appName": "Brick"}, "ios", payload)
	if counts.Flows != 2 || counts.Screens != 4 || counts.AppVersions != 1 {
		t.Fatalf("counts = %+v; want 2 flows, 4 screens, 1 app version", counts)
	}

	versions, err := db.RawQuery(ctx, `SELECT id, app_id, captured_at FROM app_versions`)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 {
		t.Fatalf("app_versions = %#v; want one row", versions)
	}
	v := versions[0]
	if v["id"] != "897f091c-52a5-4c24-bf75-73f4eaef6fca" || v["app_id"] != appID || v["captured_at"] != "2026-08-03T10:08:38.818+00:00" {
		t.Fatalf("app version = %#v; want published date 2026-08-03T10:08:38.818+00:00", v)
	}

	screens, err := db.RawQuery(ctx, `SELECT id, image_url, image_url_full, app_version_id FROM screens ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	if len(screens) != 4 {
		t.Fatalf("screens = %d rows; want 4", len(screens))
	}
	for _, s := range screens {
		if s["image_url"] == nil || s["image_url"] == "" || s["image_url_full"] == nil || s["image_url_full"] == "" {
			t.Fatalf("screen %v missing image URLs: %#v", s["id"], s)
		}
		if s["app_version_id"] != "897f091c-52a5-4c24-bf75-73f4eaef6fca" {
			t.Fatalf("screen %v app_version_id = %v", s["id"], s["app_version_id"])
		}
	}
}

// Without a page version list, sync still derives versions from rows.
func TestStoreScrapedApp_FallsBackToRowVersions(t *testing.T) {
	ctx := context.Background()
	db, err := store.OpenWithContext(ctx, filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	payload := &appscrape.AppPagePayload{
		Screens: []map[string]any{{"id": "s1", "appVersionId": "v1", "createdAt": "2026-01-02T00:00:00Z"}},
	}
	counts := storeScrapedApp(ctx, db, map[string]any{"id": "app_1"}, "ios", payload)
	if counts.AppVersions != 1 {
		t.Fatalf("counts = %+v; want 1 app version", counts)
	}
	rows, err := db.RawQuery(ctx, `SELECT id, captured_at FROM app_versions`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["id"] != "v1" || rows[0]["captured_at"] != "2026-01-02T00:00:00Z" {
		t.Fatalf("app_versions = %#v", rows)
	}
}
