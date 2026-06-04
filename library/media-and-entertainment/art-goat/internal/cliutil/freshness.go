// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

// Package cliutil exports the EnsureFresh helper that printed CLIs call
// from their root PersistentPreRunE to gate work on a freshness
// threshold.
package cliutil

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite" // SQLite driver registered as "sqlite"
)

// EnsureFresh opens the SQLite DB at dbPath read-only and reports
// whether the most-recent works.synced_at is older than maxAge.
//
// Returns (stale, lastSync, err):
//   - stale=true, lastSync=zero, err=nil when the DB file does not exist:
//     a fresh CLI install has nothing to refresh yet, callers should
//     treat this as "nothing to nag about" — we report stale=false here
//     to keep the no-DB case quiet. See the early return below.
//   - stale=true, lastSync=zero, err=nil when the works table has no rows
//     (a sync has never been run).
//   - stale=(age > maxAge), lastSync=MAX(synced_at), err=nil otherwise.
//
// This helper intentionally does NOT import internal/store; doing so
// would create an import cycle for callers in internal/cli. The schema
// it reads (the works table with a synced_at column) is owned by
// internal/store; if that schema changes, update this query too.
func EnsureFresh(dbPath string, maxAge time.Duration) (stale bool, lastSync time.Time, err error) {
	// Treat a missing DB as "nothing to refresh yet" rather than stale.
	// A passive nudge to run `sources sync` on every command before the
	// first sync would be noise; the user has already been told to
	// hydrate via doctor/sources output.
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		return false, time.Time{}, nil
	} else if statErr != nil {
		return false, time.Time{}, fmt.Errorf("stat %s: %w", dbPath, statErr)
	}

	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro&_busy_timeout=2000")
	if err != nil {
		return false, time.Time{}, fmt.Errorf("open %s: %w", dbPath, err)
	}
	defer db.Close()

	// modernc.org/sqlite returns TIMESTAMP columns as `driver.Value type
	// string`, and sql.NullTime.Scan rejects strings — so scan as
	// NullString and parse permissively. internal/store/sits.go uses the
	// same pattern for the same reason; if that helper moves we should
	// reuse it via a stable cliutil export rather than duplicating
	// layouts.
	var lastSyncNS sql.NullString
	row := db.QueryRow(`SELECT MAX(synced_at) FROM works`)
	if scanErr := row.Scan(&lastSyncNS); scanErr != nil {
		// works table may not exist yet on a partially-migrated DB.
		// Treat as "stale, no last sync" so callers can nudge.
		return true, time.Time{}, nil
	}
	if !lastSyncNS.Valid || strings.TrimSpace(lastSyncNS.String) == "" {
		// No rows in works yet.
		return true, time.Time{}, nil
	}
	t, ok := parseStoredTime(lastSyncNS.String)
	if !ok {
		// Unparseable timestamp — treat as stale so the user gets a nudge
		// rather than silently swallowing a schema-drift signal.
		return true, time.Time{}, nil
	}
	age := time.Since(t)
	return age > maxAge, t, nil
}

// parseStoredTime parses the wire formats SQLite stores time.Time in
// when written through the modernc.org/sqlite driver. Mirrors the
// helper of the same name in internal/store/sits.go; intentionally
// duplicated to avoid a store→cliutil dependency cycle.
func parseStoredTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999999 -0700",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05",
		time.RFC3339Nano,
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
