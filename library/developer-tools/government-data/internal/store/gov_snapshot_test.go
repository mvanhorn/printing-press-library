package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/govcache"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/govdata"
)

func TestCatalogSaveRollsBackEverySurfaceAndCanRetry(t *testing.T) {
	for _, failure := range []string{"generic insert", "full-text index"} {
		t.Run(failure, func(t *testing.T) {
			ctx := context.Background()
			db, err := Open(filepath.Join(t.TempDir(), "catalog.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			snapshot := govdata.Snapshot{ID: "retryable", CapturedAt: "2026-11-03T00:00:00Z", Datasets: []govdata.Dataset{{Source: "california", ID: "ca1", Title: "Water evidence"}, {Source: "federal", ID: "us1", Title: "Water evidence"}}}
			if failure == "generic insert" {
				_, err = db.db.Exec(`CREATE TRIGGER reject_catalog_index BEFORE INSERT ON resources WHEN NEW.resource_type='datasets' BEGIN SELECT RAISE(ABORT,'injected index failure'); END;`)
			} else {
				_, err = db.db.Exec(`DROP TABLE resources_fts`)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := db.SaveCatalogSnapshot(ctx, snapshot); err == nil {
				t.Fatal("injected failure reported success")
			}
			for _, table := range []string{"gov_snapshots", "gov_snapshot_records", "resources", "california", "federal"} {
				var n int
				if err := db.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil || n != 0 {
					t.Fatalf("partial %s: count=%d error=%v", table, n, err)
				}
			}
			if failure == "generic insert" {
				_, err = db.db.Exec(`DROP TRIGGER reject_catalog_index`)
			} else {
				_, err = db.db.Exec(resourcesFTSCreateSQL)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := db.SaveCatalogSnapshot(ctx, snapshot); err != nil {
				t.Fatalf("same ID could not retry: %v", err)
			}
			loaded, err := govcache.Load(ctx, db.db, "retryable")
			if err != nil || len(loaded.Datasets) != 2 {
				t.Fatalf("snapshot: %+v %v", loaded, err)
			}
			for source, search := range map[string]func(string, int) ([]json.RawMessage, error){"california": db.SearchCalifornia, "federal": db.SearchFederal} {
				rows, err := search("Water", 10)
				if err != nil || len(rows) != 1 {
					t.Fatalf("%s search=%d %v", source, len(rows), err)
				}
			}
			rows, err := db.Search("Water", 10, "datasets")
			if err != nil || len(rows) != 2 {
				t.Fatalf("generic search=%d %v", len(rows), err)
			}
			if err := db.SaveCatalogSnapshot(ctx, snapshot); err == nil {
				t.Fatal("successful snapshot lost immutability")
			}
		})
	}
}
