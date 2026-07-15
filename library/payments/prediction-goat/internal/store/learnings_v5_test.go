// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// stampV4WithoutLookups constructs a synthetic v4 database that
// stamps user_version=4 and creates the v4-era search_learnings
// table shape, but DOES NOT create entity_lookups. This exercises
// the v4->v5 path: opening through the current binary must create
// the table and seed it.
func stampV4WithoutLookups(t *testing.T, dbPath string) {
	t.Helper()
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw v4 db: %v", err)
	}
	defer raw.Close()

	// v4 shape: search_learnings with query_entities column, no
	// entity_lookups table. We don't create the full v4 schema here
	// — only the bits the migration loop won't drop or rewrite.
	if _, err := raw.Exec(`CREATE TABLE search_learnings (
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
	)`); err != nil {
		t.Fatalf("create v4 search_learnings: %v", err)
	}
	if _, err := raw.Exec(`PRAGMA user_version = 4`); err != nil {
		t.Fatalf("stamp user_version = 4: %v", err)
	}
}

// TestMigrate_EntityLookups_FreshDB pins the fresh-DB guarantee:
// opening a brand-new database stamps version 5, creates the
// entity_lookups table, and seeds it with all the canonical
// country + sports rows.
func TestMigrate_EntityLookups_FreshDB(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "fresh.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open fresh db: %v", err)
	}
	defer s.Close()

	v, err := s.SchemaVersion()
	if err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if v != StoreSchemaVersion {
		t.Fatalf("fresh db version = %d, want %d", v, StoreSchemaVersion)
	}

	// entity_lookups table must exist with at least the country
	// seed rows (>250) populated.
	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM entity_lookups`).Scan(&count); err != nil {
		t.Fatalf("count entity_lookups: %v", err)
	}
	if count < 250 {
		t.Errorf("entity_lookups has %d rows on fresh DB, want >= 250", count)
	}

	// Pin the smoke-test row that the recipe engine will rely on
	// for the "England wins free after Portugal teach" story.
	var value string
	if err := s.DB().QueryRow(`SELECT value FROM entity_lookups WHERE kind = ? AND canonical = ?`,
		"country_iso2", "Portugal").Scan(&value); err != nil {
		t.Fatalf("lookup Portugal country_iso2: %v", err)
	}
	if value != "PT" {
		t.Errorf("Portugal country_iso2 = %q, want \"PT\"", value)
	}
	if err := s.DB().QueryRow(`SELECT value FROM entity_lookups WHERE kind = ? AND canonical = ?`,
		"country_iso2", "England").Scan(&value); err != nil {
		t.Fatalf("lookup England country_iso2: %v", err)
	}
	if value != "GB" {
		t.Errorf("England country_iso2 = %q, want \"GB\"", value)
	}
}

// TestMigrate_EntityLookups_UpgradeFromV4 exercises the v4->v5
// transition: a synthetic v4 DB with no entity_lookups table opens
// through the current binary and ends up at v5 with the table
// created and seeded.
func TestMigrate_EntityLookups_UpgradeFromV4(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "v4.db")
	stampV4WithoutLookups(t, dbPath)

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open upgraded db: %v", err)
	}
	defer s.Close()

	v, err := s.SchemaVersion()
	if err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if v != StoreSchemaVersion {
		t.Fatalf("upgraded db version = %d, want %d", v, StoreSchemaVersion)
	}

	// Table exists and is populated.
	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM entity_lookups`).Scan(&count); err != nil {
		t.Fatalf("count entity_lookups: %v", err)
	}
	if count < 250 {
		t.Errorf("entity_lookups has %d rows after v4->v5, want >= 250", count)
	}
}

// TestMigrate_EntityLookups_Idempotent verifies the migration is
// safe to re-run: opening a v5 DB twice in sequence must not
// duplicate seed rows or change the row count. Combined with the
// `current < 5` gate in migrate() this exercises both paths (gate
// skip on re-Open; OR IGNORE silencing if the gate ever drifts).
func TestMigrate_EntityLookups_Idempotent(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "idem.db")

	s1, err := Open(dbPath)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	var firstCount int
	if err := s1.DB().QueryRow(`SELECT COUNT(*) FROM entity_lookups`).Scan(&firstCount); err != nil {
		s1.Close()
		t.Fatalf("count after first open: %v", err)
	}
	s1.Close()

	s2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer s2.Close()
	var secondCount int
	if err := s2.DB().QueryRow(`SELECT COUNT(*) FROM entity_lookups`).Scan(&secondCount); err != nil {
		t.Fatalf("count after second open: %v", err)
	}

	if firstCount != secondCount {
		t.Errorf("entity_lookups row count drifted across opens: first=%d second=%d", firstCount, secondCount)
	}
}

// TestMigrate_EntityLookups_PreservesTaughtRows covers the
// scenario where a user has already added a `teach-lookup` row
// (source='taught') and the migration runs again on a re-Open. The
// taught row must survive — the seed batch is INSERT OR IGNORE on
// the (kind, canonical, value) PK, so a re-seed of "Portugal" -> "PT"
// can't overwrite a user's "Portugal" -> "PRT" taught row in the
// same kind because the values differ (so both coexist) and can't
// alter the taught row when the values match because OR IGNORE
// silences the conflict.
func TestMigrate_EntityLookups_PreservesTaughtRows(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "taught.db")

	// First open: seeds Portugal -> PT.
	s1, err := Open(dbPath)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}

	// User teaches a custom alias under the same kind.
	if _, err := s1.DB().Exec(`INSERT INTO entity_lookups (kind, canonical, value, source) VALUES (?, ?, ?, ?)`,
		"country_iso2", "Portugal", "CUSTOM", "taught"); err != nil {
		s1.Close()
		t.Fatalf("insert taught row: %v", err)
	}
	s1.Close()

	// Re-Open: migration is gated at v5 so it won't run, but even
	// if it did the OR IGNORE on the seeded Portugal->PT triple
	// would be a no-op and the taught Portugal->CUSTOM row would
	// stay untouched.
	s2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer s2.Close()

	var seededCount, taughtCount int
	if err := s2.DB().QueryRow(`SELECT COUNT(*) FROM entity_lookups WHERE kind = ? AND canonical = ? AND value = ?`,
		"country_iso2", "Portugal", "PT").Scan(&seededCount); err != nil {
		t.Fatalf("count seeded: %v", err)
	}
	if seededCount != 1 {
		t.Errorf("seeded Portugal->PT count = %d, want 1", seededCount)
	}
	if err := s2.DB().QueryRow(`SELECT COUNT(*) FROM entity_lookups WHERE kind = ? AND canonical = ? AND value = ?`,
		"country_iso2", "Portugal", "CUSTOM").Scan(&taughtCount); err != nil {
		t.Fatalf("count taught: %v", err)
	}
	if taughtCount != 1 {
		t.Errorf("taught Portugal->CUSTOM count = %d, want 1", taughtCount)
	}
}
