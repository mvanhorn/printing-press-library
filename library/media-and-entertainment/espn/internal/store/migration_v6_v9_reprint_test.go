// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package store_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn/internal/store"
)

// TestMigrateV6PopulatedStoreUpgradesToV9 pins the store migration contract for
// a reprint: a database created at schema v6 (the published ESPN shape) with
// real learnings and playbooks must upgrade cleanly through to v9 with every
// row preserved and the new self-healing tables (learn_candidates, learn_events)
// created. This is the load-bearing guarantee behind reprinting a live,
// hand-evolved store: users' ~30 accumulated learnings survive the upgrade.
func TestMigrateV6PopulatedStoreUpgradesToV9(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "espn-v6.db")

	// --- Build a populated v6 fixture with the exact published v6 DDL. ---
	seedV6Fixture(t, dbPath)

	// --- Open under the current (v9) store, which runs migrate(). ---
	ctx := context.Background()
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenWithContext on a populated v6 db: %v", err)
	}
	defer s.Close()

	// Schema version must have advanced to the current constant (9).
	got, err := s.SchemaVersion()
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if got != store.StoreSchemaVersion {
		t.Fatalf("post-migration schema version = %d, want %d", got, store.StoreSchemaVersion)
	}
	if store.StoreSchemaVersion != 9 {
		t.Fatalf("StoreSchemaVersion = %d, want 9 (this test pins the v6->v9 upgrade)", store.StoreSchemaVersion)
	}

	// Learnings seeded at v6 must survive the upgrade with content intact.
	rows, err := s.ListLearnings(ctx, store.ListLearningsFilter{})
	if err != nil {
		t.Fatalf("ListLearnings after migration: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("learnings after migration = %d, want 3 (seeded v6 rows must survive)", len(rows))
	}
	wantQueries := map[string]bool{
		"portugal world cup":  false,
		"lakers game tonight": false,
		"chiefs schedule":     false,
	}
	for _, r := range rows {
		if _, ok := wantQueries[r.QueryPattern]; ok {
			wantQueries[r.QueryPattern] = true
		}
	}
	for q, seen := range wantQueries {
		if !seen {
			t.Errorf("seeded v6 learning %q did not survive migration to v9", q)
		}
	}

	// Playbooks seeded at v6 must survive too.
	pbs, err := s.ListPlaybooks()
	if err != nil {
		t.Fatalf("ListPlaybooks after migration: %v", err)
	}
	if len(pbs) != 2 {
		t.Fatalf("playbooks after migration = %d, want 2 (seeded v6 rows must survive)", len(pbs))
	}
	if _, ok, _ := s.GetPlaybookByFamily("world cup"); !ok {
		t.Error("seeded v6 playbook family \"world cup\" did not survive migration")
	}

	// The v9 self-healing tables must now exist.
	for _, tbl := range []string{"learn_candidates", "learn_events"} {
		if !tableExistsInDB(t, s.DB(), tbl) {
			t.Errorf("v9 table %q not created during v6->v9 migration", tbl)
		}
	}
}

// seedV6Fixture writes a SQLite database at dbPath carrying the published v6
// schema for search_learnings and learning_playbooks, stamps user_version = 6,
// and inserts real-shaped rows. The DDL is copied verbatim from the published
// v6 store so the fixture is a faithful pre-upgrade snapshot.
func seedV6Fixture(t *testing.T, dbPath string) {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw v6 db: %v", err)
	}
	defer db.Close()

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS search_learnings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			query_pattern TEXT NOT NULL,
			query_entities TEXT,
			venue TEXT,
			resource_type TEXT,
			resource_id TEXT NOT NULL,
			action TEXT NOT NULL,
			alias_target TEXT,
			source TEXT NOT NULL,
			confidence INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_observed_at DATETIME,
			notes TEXT
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_learn_unique ON search_learnings(query_pattern, resource_id, action)`,
		`CREATE TABLE IF NOT EXISTS learning_playbooks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			query_family TEXT NOT NULL,
			playbook_json TEXT,
			notes_text TEXT,
			source TEXT NOT NULL DEFAULT 'taught',
			confidence INTEGER NOT NULL DEFAULT 2,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_observed_at DATETIME
		)`,
	}
	for _, q := range stmts {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("v6 DDL exec failed: %v\nstmt: %s", err, q)
		}
	}

	learnings := []struct{ pattern, rid, action, source string }{
		{"portugal world cup", "soccer/fifa.world/portugal", "boost", "taught"},
		{"lakers game tonight", "basketball/nba/lakers", "boost", "observed"},
		{"chiefs schedule", "football/nfl/chiefs", "boost", "taught"},
	}
	for _, l := range learnings {
		if _, err := db.Exec(
			`INSERT INTO search_learnings (query_pattern, resource_id, action, source, confidence)
			 VALUES (?, ?, ?, ?, ?)`,
			l.pattern, l.rid, l.action, l.source, 3,
		); err != nil {
			t.Fatalf("seed learning %q: %v", l.pattern, err)
		}
	}

	playbooks := []struct{ family, pbjson, notes string }{
		{"world cup", `{"steps":[{"cmd":"scoreboard soccer fifa.world"}]}`, "use --dates not --date for historical soccer results"},
		{"nba standings", `{"steps":[{"cmd":"standings basketball nba"}]}`, ""},
	}
	for _, p := range playbooks {
		if _, err := db.Exec(
			`INSERT INTO learning_playbooks (query_family, playbook_json, notes_text, source, confidence)
			 VALUES (?, ?, ?, ?, ?)`,
			p.family, p.pbjson, p.notes, "taught", 2,
		); err != nil {
			t.Fatalf("seed playbook %q: %v", p.family, err)
		}
	}

	if _, err := db.Exec(`PRAGMA user_version = 6`); err != nil {
		t.Fatalf("stamp user_version = 6: %v", err)
	}
}

func tableExistsInDB(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var found string
	err := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name = ?`, name,
	).Scan(&found)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Fatalf("querying sqlite_master for %q: %v", name, err)
	}
	return found == name
}
