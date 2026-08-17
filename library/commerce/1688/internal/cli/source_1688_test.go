package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/1688/internal/mtop"
	"github.com/mvanhorn/printing-press-library/library/commerce/1688/internal/store"
)

func TestPersistSearchReturnsSnapshotWriteErrors(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "1688.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.DB().ExecContext(ctx, `CREATE TABLE offer_snapshots (
		offer_id TEXT NOT NULL,
		synced_at TEXT NOT NULL,
		PRIMARY KEY (offer_id, synced_at)
	)`); err != nil {
		t.Fatal(err)
	}

	err = persistSearch(ctx, db, &mtop.SearchResult{
		Offers: []mtop.Offer{{
			OfferID:  "offer-1",
			SyncedAt: "2026-07-08T12:00:00.000000001Z",
			Keyword:  "手机壳",
		}},
	})
	if err == nil {
		t.Fatal("persistSearch returned nil for a failed snapshot insert")
	}
	if !strings.Contains(err.Error(), "keyword") {
		t.Fatalf("persistSearch error = %v, want missing snapshot column", err)
	}
}
