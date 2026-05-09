// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

// Package judicial holds the Taiwan-judicial-specific store schema and
// repository helpers that sit on top of the generator-emitted store.Store.
//
// The generator's store handles the generic `judgments` and `knowledge`
// tables. This package adds the four extra tables that power the novel
// features (citations, sentences, watchlist, change_log) and exposes
// repository methods scoped to those tables.
package judicial

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// EnsureSchema creates the judicial-specific tables if they don't exist.
// Safe to call repeatedly; uses IF NOT EXISTS.
//
// Tables:
//   - citations: extracted statute references per judgment (drives `cites`, `cited-by`)
//   - sentences: extracted 主文 sentence patterns per judgment (drives `sentences`)
//   - jid_refs:  reverse JID references inside judgment bodies (drives `cited-by`)
//   - watchlist: saved queries with last-seen cursor (drives `watch query`, `watch case`)
//   - change_log: append-only sync log + privacy-purge audit
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS citations (
			jid       TEXT NOT NULL,
			statute   TEXT NOT NULL,
			article   INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (jid, statute, article)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_citations_statute ON citations(statute, article)`,
		`CREATE INDEX IF NOT EXISTS idx_citations_jid     ON citations(jid)`,

		`CREATE TABLE IF NOT EXISTS sentences (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			jid           TEXT NOT NULL,
			kind          TEXT NOT NULL,
			prison_months INTEGER NOT NULL DEFAULT 0,
			fine_ntd      INTEGER NOT NULL DEFAULT 0,
			probation     INTEGER NOT NULL DEFAULT 0,
			raw           TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sentences_jid  ON sentences(jid)`,
		`CREATE INDEX IF NOT EXISTS idx_sentences_kind ON sentences(kind)`,

		`CREATE TABLE IF NOT EXISTS jid_refs (
			from_jid TEXT NOT NULL,
			to_jid   TEXT NOT NULL,
			PRIMARY KEY (from_jid, to_jid)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_jid_refs_to ON jid_refs(to_jid)`,

		`CREATE TABLE IF NOT EXISTS watchlist (
			name        TEXT PRIMARY KEY,
			kind        TEXT NOT NULL,
			query_json  TEXT NOT NULL,
			last_seen   TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL,
			updated_at  TEXT NOT NULL
		)`,

		`CREATE TABLE IF NOT EXISTS change_log (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			at         TEXT NOT NULL,
			action     TEXT NOT NULL,
			jid        TEXT NOT NULL DEFAULT '',
			detail     TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_change_log_jid ON change_log(jid)`,
		`CREATE INDEX IF NOT EXISTS idx_change_log_at  ON change_log(at)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("ensure schema: %s: %w", firstLine(s), err)
		}
	}
	return nil
}

func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	return s
}

// LogEvent appends a row to change_log. action examples: "synced", "purged",
// "watch_run", "search". jid may be empty for non-judgment events.
func LogEvent(ctx context.Context, db *sql.DB, action, jid, detail string) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO change_log (at, action, jid, detail) VALUES (?, ?, ?, ?)`,
		time.Now().UTC().Format(time.RFC3339), action, jid, detail)
	return err
}
