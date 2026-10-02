// Copyright 2026 Hunter Veltri and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestScryfallAlternateKeysBackfillAndUpdate(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "scryfall.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	first := json.RawMessage(`{"id":"card-1","name":"Old Name","set":"LEA","collector_number":"1","multiverse_ids":[789,790],"arena_id":123}`)
	if err := db.Upsert("cards", "card-1", first); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`INSERT INTO resources(resource_type,id,data) VALUES('cards','malformed','{')`); err != nil {
		t.Fatal(err)
	}
	// Simulate a v9 mirror, before alternate keys existed.
	if _, err := db.DB().Exec(`DROP TABLE resource_alt_keys`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`PRAGMA user_version = 9`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(dbPath)
	if err != nil {
		t.Fatalf("upgrade v9 mirror: %v", err)
	}
	defer db.Close()
	assertAlias := func(kind, value string, want int) {
		t.Helper()
		var got int
		if err := db.DB().QueryRow(`SELECT count(*) FROM resource_alt_keys WHERE resource_type='cards' AND kind=? AND value=? AND resource_id='card-1'`, kind, value).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%s=%q count %d, want %d", kind, value, got, want)
		}
	}
	assertAlias("name", "old name", 1)
	assertAlias("multiverse_ids", "790", 1)
	assertAlias("set_collector", "lea\x001", 1)
	assertAlias("arena_id", "123", 1)
	if err := db.Upsert("cards", "card-1", json.RawMessage(`{"id":"card-1","name":"New Name","set":"LEB","collector_number":"2","arena_id":124}`)); err != nil {
		t.Fatal(err)
	}
	assertAlias("name", "old name", 0)
	assertAlias("name", "new name", 1)
	assertAlias("multiverse_ids", "790", 0)
	assertAlias("set_collector", "lea\x001", 0)
	assertAlias("set_collector", "leb\x002", 1)
}
