// Copyright 2026 Matthew Vassallo and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestUpsertBatchWithSyncStateCommitsRowsAndCheckpoint(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	stored, failed, total, err := db.UpsertBatchWithSyncState("deals", []json.RawMessage{
		json.RawMessage(`{"id":"offer-1","title":"Offer 1"}`),
	}, "100")
	if err != nil {
		t.Fatalf("checkpointed upsert: %v", err)
	}
	if stored != 1 || failed != 0 || total != 1 {
		t.Fatalf("checkpointed upsert = stored %d failed %d total %d, want 1, 0, 1", stored, failed, total)
	}
	cursor, syncedAt, count, err := db.GetSyncState("deals")
	if err != nil {
		t.Fatalf("read sync state: %v", err)
	}
	if cursor != "100" || syncedAt.IsZero() || count != 1 {
		t.Fatalf("sync state = cursor %q time %v count %d, want 100, nonzero, 1", cursor, syncedAt, count)
	}
}

func TestUpsertBatchWithSyncStateRollsBackRowsWhenCheckpointFails(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	if _, err := db.DB().Exec(`
		CREATE TRIGGER reject_sync_state
		BEFORE INSERT ON sync_state
		BEGIN
			SELECT RAISE(ABORT, 'forced checkpoint failure');
		END`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	if _, _, _, err := db.UpsertBatchWithSyncState("deals", []json.RawMessage{
		json.RawMessage(`{"id":"offer-1","title":"Offer 1"}`),
	}, "100"); err == nil {
		t.Fatal("checkpointed upsert succeeded, want forced checkpoint failure")
	}
	count, err := db.Count("deals")
	if err != nil {
		t.Fatalf("count deals: %v", err)
	}
	if count != 0 {
		t.Fatalf("stored rows after rolled-back checkpoint = %d, want 0", count)
	}
}

func TestHasIncompleteSyncContextDetectsRowsWithoutSyncState(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	if err := db.Upsert("deals", "offer-1", json.RawMessage(`{"id":"offer-1"}`)); err != nil {
		t.Fatalf("seed deal: %v", err)
	}

	incomplete, err := db.HasIncompleteSyncContext(context.Background())
	if err != nil {
		t.Fatalf("read incomplete state: %v", err)
	}
	if !incomplete {
		t.Fatal("rows without sync_state were reported complete")
	}
	if err := db.SaveSyncState("deals", "", 1); err != nil {
		t.Fatalf("save complete state: %v", err)
	}
	incomplete, err = db.HasIncompleteSyncContext(context.Background())
	if err != nil {
		t.Fatalf("read complete state: %v", err)
	}
	if incomplete {
		t.Fatal("completed sync_state was reported incomplete")
	}
}

func TestGetSyncStateWorksThroughReadOnlyConnection(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := db.SaveSyncState("deals", "0", 42); err != nil {
		db.Close()
		t.Fatalf("save sync state: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close writable store: %v", err)
	}

	readOnly, err := OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("open read-only store: %v", err)
	}
	defer readOnly.Close()
	cursor, syncedAt, count, err := readOnly.GetSyncState("deals")
	if err != nil {
		t.Fatalf("read sync state: %v", err)
	}
	if cursor != "0" || syncedAt.IsZero() || count != 42 {
		t.Fatalf("read-only sync state = cursor %q time %v count %d, want 0, nonzero, 42", cursor, syncedAt, count)
	}
}
