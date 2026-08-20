// Migration safety tests.
//
// Covers: pre-existing user DBs that drift from canonical schema (the
// classic "we added a column to the CREATE TABLE without an ALTER
// migration" hazard). Without reconciliation, those DBs fail at
// CREATE INDEX time when the index references a column that exists
// in canonical schema but not on disk.
//
// These tests construct synthetic old-schema DBs at the SQLite level
// (no ORM) and then open them with store.Open to verify the migration
// path heals them in place.

package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// makeStaleEventsDB creates a database file at dbPath with a pre-
// season_year `events` table. Simulates a user who installed espn-pp-cli
// before the season_year column was added to the canonical schema.
//
// The shape mirrors what an early-2026 espn-pp-cli user would have on
// disk: a subset of today's columns, no season_year, no season_type,
// no week, no notes. Every column we omit here is one the canonical
// migrate() would otherwise reference at CREATE INDEX time.
func makeStaleEventsDB(t *testing.T, dbPath string) {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open stale db: %v", err)
	}
	defer db.Close()

	// Old shape: missing season_year, season_type, week, notes.
	stmts := []string{
		`CREATE TABLE events (
			id TEXT PRIMARY KEY,
			sport TEXT NOT NULL,
			league TEXT NOT NULL,
			name TEXT,
			short_name TEXT,
			date TEXT,
			status TEXT,
			completed INTEGER DEFAULT 0,
			home_team_id TEXT,
			home_team_abbr TEXT,
			home_team_name TEXT,
			home_score TEXT,
			home_winner INTEGER DEFAULT 0,
			away_team_id TEXT,
			away_team_abbr TEXT,
			away_team_name TEXT,
			away_score TEXT,
			away_winner INTEGER DEFAULT 0,
			venue_name TEXT,
			venue_city TEXT,
			broadcast TEXT,
			attendance INTEGER,
			neutral_site INTEGER DEFAULT 0,
			data JSON NOT NULL
		)`,
		// Seed a row so we can verify reconciliation preserves user data.
		`INSERT INTO events (id, sport, league, data) VALUES ('legacy-1', 'football', 'nfl', '{}')`,
	}
	for _, q := range stmts {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("stale schema setup: %v", err)
		}
	}
}

func TestMigrate_StaleEventsTable_SelfHeals(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "stale-events.db")
	makeStaleEventsDB(t, dbPath)

	// Open via canonical store. Before the reconciliation fix this
	// fails on `CREATE INDEX idx_events_season ON events(season_year,
	// season_type)` because the columns don't exist.
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open stale db: %v (reconciliation should add missing columns)", err)
	}
	defer s.Close()

	// Verify the missing columns now exist on disk.
	got := columnsOf(t, s.DB(), "events")
	wantMissing := []string{"season_year", "season_type", "week", "notes"}
	for _, col := range wantMissing {
		if _, ok := got[col]; !ok {
			t.Errorf("events table missing %q after migration; want present", col)
		}
	}

	// Verify the legacy row survives (reconciliation must not destroy data).
	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM events WHERE id='legacy-1'`).Scan(&count); err != nil {
		t.Fatalf("query legacy row: %v", err)
	}
	if count != 1 {
		t.Errorf("legacy row count = %d, want 1 (data must survive ALTER)", count)
	}

	// Verify the season index exists (it's what was failing before).
	if !indexExists(t, s.DB(), "idx_events_season") {
		t.Errorf("idx_events_season missing after migration; CREATE INDEX should have run after column-add")
	}
}

func TestMigrate_FreshDB_NoOpReconciliation(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "fresh.db")

	// Open a fresh DB twice — second Open must produce the same shape
	// (reconciliation pass is idempotent).
	s1, err := Open(dbPath)
	if err != nil {
		t.Fatalf("fresh open: %v", err)
	}
	cols1 := columnsOf(t, s1.DB(), "events")
	s1.Close()

	s2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	defer s2.Close()
	cols2 := columnsOf(t, s2.DB(), "events")

	if len(cols1) != len(cols2) {
		t.Errorf("column count drift across opens: %d -> %d", len(cols1), len(cols2))
	}
	for col := range cols1 {
		if _, ok := cols2[col]; !ok {
			t.Errorf("column %q lost on second open", col)
		}
	}
}

func TestMigrate_ForwardCompat_PreservesExtraColumns(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "forward.db")
	makeStaleEventsDB(t, dbPath)

	// User added a custom column (simulates someone who hand-altered
	// their DB or a future version that added an experimental column).
	pre, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := pre.Exec(`ALTER TABLE events ADD COLUMN experimental_score INTEGER`); err != nil {
		t.Fatalf("add experimental col: %v", err)
	}
	pre.Close()

	// Open with canonical store. Reconciliation must add missing
	// canonical columns AND leave the user's experimental column alone.
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open forward-compat db: %v", err)
	}
	defer s.Close()

	got := columnsOf(t, s.DB(), "events")
	if _, ok := got["experimental_score"]; !ok {
		t.Errorf("experimental_score was dropped by migration (must preserve unknown columns)")
	}
	if _, ok := got["season_year"]; !ok {
		t.Errorf("season_year not added (reconciliation should heal missing canonical columns)")
	}
}

func TestMigrate_StaleSchema_LearnLoopUnblocked(t *testing.T) {
	t.Parallel()
	// The canonical bug from real ESPN dogfood: stale events table
	// blocks the learn-loop init (runLearnInitOnce calls store.Open).
	// After the fix, the learn-loop store path should be usable.
	dbPath := filepath.Join(t.TempDir(), "learn-blocked.db")
	makeStaleEventsDB(t, dbPath)

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	// Verify search_learnings exists (the learn schema landed alongside
	// the events reconciliation).
	if !tableExists(t, s.DB(), "search_learnings") {
		t.Errorf("search_learnings table not created; learn loop is still blocked")
	}
}

// --- helpers ---

func columnsOf(t *testing.T, db *sql.DB, table string) map[string]string {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatalf("pragma table_info(%s): %v", table, err)
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, dfltValue, pk sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out[name] = typ
	}
	return out
}

func indexExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var got string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='index' AND name=?`, name).Scan(&got)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Fatalf("query index: %v", err)
	}
	return got == name
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var got string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&got)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Fatalf("query table: %v", err)
	}
	return got == name
}

// avoid unused import: silence the context-import if some helpers in
// the file later drop the references that introduced it
var _ = context.Background
