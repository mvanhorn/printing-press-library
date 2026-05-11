// Package yclocal provides local-only logic for the YC companies CLI:
// snapshot history, watch lists, similarity ranking, and aggregations.
package yclocal

import (
	"database/sql"
	"fmt"
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS companies_history (
    snapshot_id TEXT NOT NULL,
    slug TEXT NOT NULL,
    name TEXT,
    status TEXT,
    team_size INTEGER,
    is_hiring INTEGER,
    top_company INTEGER,
    batch TEXT,
    captured_at TEXT NOT NULL,
    PRIMARY KEY (snapshot_id, slug)
);

CREATE INDEX IF NOT EXISTS idx_history_slug ON companies_history(slug);
CREATE INDEX IF NOT EXISTS idx_history_snapshot ON companies_history(snapshot_id);
CREATE INDEX IF NOT EXISTS idx_history_captured ON companies_history(captured_at);

CREATE TABLE IF NOT EXISTS watch (
    slug TEXT PRIMARY KEY,
    added_at TEXT NOT NULL DEFAULT (datetime('now'))
);
`

// EnsureSchema creates the yclocal tables if they do not already exist.
func EnsureSchema(db *sql.DB) error {
	if _, err := db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("yclocal: ensure schema: %w", err)
	}
	return nil
}
