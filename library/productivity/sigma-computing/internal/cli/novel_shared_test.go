// Copyright 2026 Chris Hatton and contributors. Licensed under Apache-2.0. See LICENSE.
// Shared test helpers for the novel feature commands.

package cli

import (
	"database/sql"
	"testing"
)

// mustExec runs an INSERT/UPDATE in a test, failing on error.
func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}
