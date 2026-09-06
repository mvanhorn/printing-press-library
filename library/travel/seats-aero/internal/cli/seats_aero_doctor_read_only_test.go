package cli

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/seats-aero/internal/store"
)

func TestCollectCacheReportDoesNotMigrateStore(t *testing.T) {
	home := isolateNovelTest(t)
	_ = home
	dbPath := defaultDBPath("seats-aero-pp-cli")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`PRAGMA user_version = 1`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	report := collectCacheReport(context.Background(), "168h")
	if pending, _ := report["migration_pending"].(bool); !pending {
		t.Fatalf("report=%v", report)
	}
	ro, err := store.OpenReadOnlyContext(context.Background(), filepath.Clean(dbPath))
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	version, err := ro.SchemaVersion()
	if err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Fatalf("user_version=%d, want 1", version)
	}
}

func TestCollectCacheReportCurrentStoreHasNoMigrationPending(t *testing.T) {
	isolateNovelTest(t)
	db, err := store.Open(defaultDBPath("seats-aero-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	report := collectCacheReport(context.Background(), "168h")
	if pending, _ := report["migration_pending"].(bool); pending {
		t.Fatalf("report=%v", report)
	}
}
