// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

const firewallType = "network_integration_v1_sites_firewall_policies"

func openConsoleTestStore(t *testing.T) *Store {
	t.Helper()
	SetActiveConsole("")
	t.Cleanup(func() { SetActiveConsole("") })
	db, err := Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func policy(id, site, name string) json.RawMessage {
	return json.RawMessage(`{"id":"` + id + `","parent_id":"` + site + `","name":"` + name + `"}`)
}

func TestConsoleKeySeparatesIdenticalSiteAndEntityIDs(t *testing.T) {
	db := openConsoleTestStore(t)
	SetActiveConsole("console-a")
	if stored, _, err := db.UpsertBatch(firewallType, []json.RawMessage{policy("p1", "shared-site", "from A")}); err != nil || stored != 1 {
		t.Fatalf("stored %d err %v", stored, err)
	}
	SetActiveConsole("console-b")
	if stored, _, err := db.UpsertBatch(firewallType, []json.RawMessage{policy("p1", "shared-site", "from B")}); err != nil || stored != 1 {
		t.Fatalf("stored %d err %v", stored, err)
	}
	ids, err := db.ListIDs(firewallType)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("same site id on two consoles collapsed: %q", ids)
	}
	got, err := db.GetInActiveConsole(firewallType, "p1")
	if err != nil {
		t.Fatal(err)
	}
	var row map[string]any
	_ = json.Unmarshal(got, &row)
	if row["name"] != "from B" || row[ConsoleIDField] != "console-b" {
		t.Fatalf("active-console read returned %v", row)
	}
	SetActiveConsole("console-a")
	got, err = db.GetInActiveConsole(firewallType, "p1")
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(got, &row)
	if row["name"] != "from A" {
		t.Fatalf("console A read returned %v", row)
	}
}

func TestReconcileLeavesOtherConsolesAlone(t *testing.T) {
	db := openConsoleTestStore(t)
	SetActiveConsole("console-a")
	if _, _, err := db.UpsertBatch(firewallType, []json.RawMessage{policy("p1", "s", "a")}); err != nil {
		t.Fatal(err)
	}
	SetActiveConsole("console-b")
	if _, _, err := db.UpsertBatch(firewallType, []json.RawMessage{policy("p2", "s", "b")}); err != nil {
		t.Fatal(err)
	}
	// Console B's partition no longer lists p2; p1 belongs to A and must survive.
	removed, err := db.ReconcilePartition(firewallType, "$.parent_id", "s", nil, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed %d, want only console B's row", removed)
	}
	SetActiveConsole("console-a")
	if _, err := db.GetInActiveConsole(firewallType, "p1"); err != nil {
		t.Fatalf("console A row was reconciled away: %v", err)
	}
}

func TestSiteManagerResourcesStayUnkeyed(t *testing.T) {
	db := openConsoleTestStore(t)
	SetActiveConsole("console-a")
	if err := db.Upsert("hosts", "h1", json.RawMessage(`{"id":"h1"}`)); err != nil {
		t.Fatal(err)
	}
	SetActiveConsole("console-b")
	got, err := db.Get("hosts", "h1")
	if err != nil {
		t.Fatalf("host should be visible from every console: %v", err)
	}
	var row map[string]any
	_ = json.Unmarshal(got, &row)
	if _, stamped := row[ConsoleIDField]; stamped {
		t.Fatalf("account-wide row was stamped: %v", row)
	}
}

func TestDeleteResourceRemovesGenericAndFTSForActiveConsoleOnly(t *testing.T) {
	db := openConsoleTestStore(t)
	SetActiveConsole("console-a")
	if _, _, err := db.UpsertBatch(firewallType, []json.RawMessage{policy("p1", "s", "uniqueword")}); err != nil {
		t.Fatal(err)
	}
	SetActiveConsole("console-b")
	if _, _, err := db.UpsertBatch(firewallType, []json.RawMessage{policy("p1", "s", "uniqueword")}); err != nil {
		t.Fatal(err)
	}
	hits, err := db.Search("uniqueword", 10)
	if err != nil || len(hits) != 2 {
		t.Fatalf("search before delete: %d %v", len(hits), err)
	}
	n, err := db.DeleteResource(firewallType, "p1", "s")
	if err != nil || n != 1 {
		t.Fatalf("deleted %d err %v", n, err)
	}
	hits, err = db.Search("uniqueword", 10)
	if err != nil || len(hits) != 1 {
		t.Fatalf("FTS row not removed: %d %v", len(hits), err)
	}
	if n, err := db.DeleteResource(firewallType, "absent", ""); err != nil || n != 0 {
		t.Fatalf("absent id: %d %v", n, err)
	}
}

func TestDeleteResourceRemovesTypedRow(t *testing.T) {
	db := openConsoleTestStore(t)
	SetActiveConsole("console-a")
	if _, _, err := db.UpsertBatch("protect", []json.RawMessage{json.RawMessage(`{"id":"hub1","name":"hub"}`)}); err != nil {
		t.Fatal(err)
	}
	var before int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM protect`).Scan(&before); err != nil || before != 1 {
		t.Fatalf("typed before: %d %v", before, err)
	}
	if _, err := db.DeleteResource("protect", "hub1", ""); err != nil {
		t.Fatal(err)
	}
	var after int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM protect`).Scan(&after); err != nil || after != 0 {
		t.Fatalf("typed after: %d %v", after, err)
	}
}
