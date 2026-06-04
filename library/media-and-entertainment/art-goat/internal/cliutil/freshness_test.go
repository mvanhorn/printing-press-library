// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

package cliutil

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

// openTestDB opens (and creates) a SQLite DB at path read-write. The
// freshness tests build the works schema directly via database/sql so
// the test does not import internal/store (which would create an
// import cycle through internal/cli).
func openTestDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path+"?_busy_timeout=2000")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// TestEnsureFresh_MissingDB asserts that a non-existent DB file is
// treated as "nothing to refresh yet" rather than stale. Returning
// stale=false here keeps a fresh-install CLI quiet on every command
// before the user has ever synced.
func TestEnsureFresh_MissingDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "nope.db")
	stale, last, err := EnsureFresh(dbPath, time.Hour)
	assert.NoError(t, err)
	assert.False(t, stale, "missing DB should report not stale")
	assert.True(t, last.IsZero(), "missing DB should return zero lastSync")
}

// TestEnsureFresh_NoWorksTable asserts that an existing DB file
// without a `works` table reports stale=true. The implementation
// swallows the Scan error from a missing table and returns
// (true, zero, nil); the contract is "stale, no last sync" so the
// caller can nudge a `sources sync`.
func TestEnsureFresh_NoWorksTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "empty.db")
	db := openTestDB(t, dbPath)
	// Create the file with at least one unrelated table so it is a
	// valid SQLite DB but no `works` table exists.
	_, err := db.Exec(`CREATE TABLE other_table (id INTEGER)`)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	stale, last, err := EnsureFresh(dbPath, time.Hour)
	assert.NoError(t, err)
	assert.True(t, stale, "DB without works table should report stale")
	assert.True(t, last.IsZero(), "no works table should return zero lastSync")
}

// TestEnsureFresh_WorksTableEmpty asserts that an existing works
// table with zero rows reports stale=true, zero lastSync. This is
// the "never synced" state — MAX(synced_at) is SQL NULL and
// sql.NullTime.Valid is false.
func TestEnsureFresh_WorksTableEmpty(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "noworks.db")
	db := openTestDB(t, dbPath)
	_, err := db.Exec(`CREATE TABLE works (synced_at TIMESTAMP)`)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	stale, last, err := EnsureFresh(dbPath, time.Hour)
	assert.NoError(t, err)
	assert.True(t, stale, "empty works table should report stale (never synced)")
	assert.True(t, last.IsZero(), "empty works table should return zero lastSync")
}

// TestEnsureFresh_WorksTableNullRow asserts that explicit NULL in
// synced_at also reports stale=true with zero lastSync. Catches the
// case where a partial migration left rows but none have been
// synced yet.
func TestEnsureFresh_WorksTableNullRow(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "nullrow.db")
	db := openTestDB(t, dbPath)
	_, err := db.Exec(`CREATE TABLE works (synced_at TIMESTAMP)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO works (synced_at) VALUES (NULL)`)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	stale, last, err := EnsureFresh(dbPath, time.Hour)
	assert.NoError(t, err)
	assert.True(t, stale, "row with NULL synced_at should report stale")
	assert.True(t, last.IsZero(), "NULL synced_at should return zero lastSync")
}

// TestEnsureFresh_PopulatedWorksTable asserts the freshness contract
// against rows actually present in the works table. modernc.org/sqlite
// returns TIMESTAMP columns as `driver.Value type string`, and
// sql.NullTime.Scan rejects strings — so EnsureFresh scans into a
// NullString and parses permissively (see parseStoredTime in
// freshness.go, which mirrors internal/store/sits.go's helper). With
// that in place, stale = (age > maxAge) and lastSync is the parsed
// MAX(synced_at).
func TestEnsureFresh_PopulatedWorksTable_RespectsAgeThreshold(t *testing.T) {
	cases := []struct {
		name      string
		offset    string // SQLite datetime() modifier
		wantStale bool
	}{
		{"recent row", "-1 minutes", false},
		{"old row", "-48 hours", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "works.db")
			db := openTestDB(t, dbPath)
			_, err := db.Exec(`CREATE TABLE works (synced_at TIMESTAMP)`)
			require.NoError(t, err)
			_, err = db.Exec(`INSERT INTO works (synced_at) VALUES (datetime('now', ?))`, tc.offset)
			require.NoError(t, err)
			require.NoError(t, db.Close())

			stale, last, err := EnsureFresh(dbPath, time.Hour)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantStale, stale, "stale should reflect whether MAX(synced_at) is older than maxAge")
			assert.False(t, last.IsZero(), "lastSync should be parsed from MAX(synced_at), not the zero value")
		})
	}
}
