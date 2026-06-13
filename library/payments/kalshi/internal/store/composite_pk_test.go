// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Regression tests for the audit-2026-06-09 composite-PK migration: a market
// and a settlement sharing a ticker must coexist as separate rows.

package store

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// Fresh databases must get the composite PK and keep colliding-ticker rows apart.
func TestResources_CompositePK_FreshDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	const ticker = "KXMLBGAME-26JUN09-BOSTB"
	if err := s.Upsert("markets", ticker, json.RawMessage(`{"ticker":"`+ticker+`","status":"finalized","category":"sports"}`)); err != nil {
		t.Fatalf("upsert market: %v", err)
	}
	if err := s.Upsert("portfolio-settlements", ticker, json.RawMessage(`{"ticker":"`+ticker+`","revenue":3400,"settled_time":"2026-06-09T03:00:00Z"}`)); err != nil {
		t.Fatalf("upsert settlement: %v", err)
	}

	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM resources WHERE id = ?`, ticker).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 2 {
		t.Fatalf("colliding ticker rows = %d, want 2 (market + settlement must coexist; the v1 id-only PK let one overwrite the other)", n)
	}

	// Each row must retain ITS data, keyed by type.
	var marketData string
	if err := s.DB().QueryRow(`SELECT data FROM resources WHERE resource_type='markets' AND id=?`, ticker).Scan(&marketData); err != nil {
		t.Fatalf("read market row: %v", err)
	}
	if !strings.Contains(marketData, `"status"`) || strings.Contains(marketData, `"revenue"`) {
		t.Fatalf("market row holds wrong payload: %s", marketData)
	}
}

// v1 databases (id-only PK) must be rebuilt on open, preserving rows.
func TestResources_CompositePK_MigratesV1(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")

	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	stmts := []string{
		`CREATE TABLE resources (
			id TEXT PRIMARY KEY,
			resource_type TEXT NOT NULL,
			data JSON NOT NULL,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE VIRTUAL TABLE resources_fts USING fts5(id, resource_type, content, tokenize='porter unicode61')`,
		`INSERT INTO resources (id, resource_type, data) VALUES
			('TICK-A', 'markets', '{"ticker":"TICK-A","status":"open"}'),
			('TICK-B', 'portfolio-settlements', '{"ticker":"TICK-B","revenue":100}')`,
		`PRAGMA user_version = 1`,
	}
	for _, q := range stmts {
		if _, err := raw.Exec(q); err != nil {
			raw.Close()
			t.Fatalf("seed v1 db (%s): %v", q[:30], err)
		}
	}
	raw.Close()

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open v1 db with v2 binary: %v", err)
	}
	defer s.Close()

	// Both seeded rows survive the rebuild.
	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM resources`).Scan(&n); err != nil {
		t.Fatalf("count after migration: %v", err)
	}
	if n != 2 {
		t.Fatalf("rows after migration = %d, want 2", n)
	}

	// The DDL is now composite.
	var ddl string
	if err := s.DB().QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='resources'`).Scan(&ddl); err != nil {
		t.Fatalf("read ddl: %v", err)
	}
	if !strings.Contains(ddl, "PRIMARY KEY (resource_type, id)") {
		t.Fatalf("resources still on v1 PK after migration: %s", ddl)
	}

	// And the collision case now works going forward.
	if err := s.Upsert("portfolio-settlements", "TICK-A", json.RawMessage(`{"ticker":"TICK-A","revenue":500}`)); err != nil {
		t.Fatalf("post-migration settlement upsert: %v", err)
	}
	var marketStatus string
	if err := s.DB().QueryRow(`SELECT json_extract(data,'$.status') FROM resources WHERE resource_type='markets' AND id='TICK-A'`).Scan(&marketStatus); err != nil {
		t.Fatalf("market row gone after settlement upsert: %v", err)
	}
	if marketStatus != "open" {
		t.Fatalf("market row clobbered: status=%q", marketStatus)
	}
}

// v1 databases created before updated_at was added must still migrate. The
// composite-PK rebuild reads updated_at, and SQLite resolves that column name
// at prepare time, so the column has to be backfilled before the rebuild runs.
// (The earlier COALESCE/WHERE-1=0 guard did not dodge that prepare-time check;
// this test fails against that version and passes once updated_at is backfilled.)
func TestResources_CompositePK_MigratesV1_NoUpdatedAt(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")

	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	stmts := []string{
		// Legacy shape: id-only PK AND no updated_at column.
		`CREATE TABLE resources (
			id TEXT PRIMARY KEY,
			resource_type TEXT NOT NULL,
			data JSON NOT NULL,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE VIRTUAL TABLE resources_fts USING fts5(id, resource_type, content, tokenize='porter unicode61')`,
		`INSERT INTO resources (id, resource_type, data) VALUES
			('TICK-A', 'markets', '{"ticker":"TICK-A","status":"open"}'),
			('TICK-B', 'portfolio-settlements', '{"ticker":"TICK-B","revenue":100}')`,
		`PRAGMA user_version = 1`,
	}
	for _, q := range stmts {
		if _, err := raw.Exec(q); err != nil {
			raw.Close()
			t.Fatalf("seed pre-updated_at v1 db: %v (stmt: %s)", err, q)
		}
	}
	raw.Close()

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open pre-updated_at v1 db with v2 binary: %v", err)
	}
	defer s.Close()

	// Both seeded rows survive the rebuild.
	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM resources`).Scan(&n); err != nil {
		t.Fatalf("count after migration: %v", err)
	}
	if n != 2 {
		t.Fatalf("rows after migration = %d, want 2", n)
	}

	// The table is now composite and carries the backfilled updated_at column.
	var ddl string
	if err := s.DB().QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='resources'`).Scan(&ddl); err != nil {
		t.Fatalf("read ddl: %v", err)
	}
	if !strings.Contains(ddl, "PRIMARY KEY (resource_type, id)") {
		t.Fatalf("resources still on v1 PK after migration: %s", ddl)
	}
	if !strings.Contains(ddl, "updated_at") {
		t.Fatalf("updated_at column missing after migration: %s", ddl)
	}
}

// Reopening a migrated DB must be a no-op (idempotence).
func TestResources_CompositePK_MigrationIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s1, err := Open(dbPath)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	if err := s1.Upsert("markets", "T1", json.RawMessage(`{"a":1}`)); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	s1.Close()

	s2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer s2.Close()
	var n int
	if err := s2.DB().QueryRow(`SELECT COUNT(*) FROM resources`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("rows after reopen = %d (err %v), want 1", n, err)
	}
}
