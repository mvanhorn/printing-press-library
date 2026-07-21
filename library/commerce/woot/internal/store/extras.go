// Copyright 2026 Matthew Vassallo and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
)

// migrateExtras runs after the generated store migrations and before the
// schema-version stamp. It is the canonical place for novel-feature auxiliary
// tables that need to live in the local store.
//
// Edit this file when adding tables for novel commands. Keep migrations
// idempotent with CREATE TABLE IF NOT EXISTS / CREATE INDEX IF NOT EXISTS so
// every store open can safely re-run them.
func (s *Store) migrateExtras(ctx context.Context, conn *sql.Conn) error {
	migrations := []string{
		// Add CREATE TABLE IF NOT EXISTS statements here.
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}

// PruneResource removes generic rows that were not present in a complete
// snapshot. Callers must only use it after fully enumerating the resource.
func (s *Store) PruneResource(resourceType string, seenIDs []string) (int, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("starting resource prune: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.Query(`SELECT id FROM resources WHERE resource_type = ?`, resourceType)
	if err != nil {
		return 0, fmt.Errorf("listing %s rows for prune: %w", resourceType, err)
	}
	defer func() { _ = rows.Close() }()
	var existing []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("reading %s row for prune: %w", resourceType, err)
		}
		existing = append(existing, id)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterating %s rows for prune: %w", resourceType, err)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("closing %s prune rows: %w", resourceType, err)
	}

	seen := make(map[string]struct{}, len(seenIDs))
	for _, id := range seenIDs {
		seen[id] = struct{}{}
	}
	deleted := 0
	for _, id := range existing {
		if _, ok := seen[id]; ok {
			continue
		}
		if _, err := tx.Exec(`DELETE FROM resources_fts WHERE rowid = ?`, ftsRowID(resourceType, id)); err != nil {
			return deleted, fmt.Errorf("deleting %s/%s from search index: %w", resourceType, id, err)
		}
		if _, err := tx.Exec(`DELETE FROM resources WHERE resource_type = ? AND id = ?`, resourceType, id); err != nil {
			return deleted, fmt.Errorf("deleting %s/%s: %w", resourceType, id, err)
		}
		deleted++
	}
	if err := tx.Commit(); err != nil {
		return deleted, fmt.Errorf("committing %s prune: %w", resourceType, err)
	}
	return deleted, nil
}
