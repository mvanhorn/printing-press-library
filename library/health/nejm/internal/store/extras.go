// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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

// PurgeArticlesByFeed removes every article row whose feed column matches one
// of feeds, cleaning all three storage surfaces consistently: the article
// domain table, the generic resources table, and the FTS index (which has no
// delete triggers — upserts maintain it row-by-row via ftsRowID, so a raw
// domain-table DELETE would orphan FTS entries). Returns the number of
// articles removed.
//
// Added for the RSS→OpenAlex source replacement: rows synced from the retired
// RSS transport (feed 'etoc'/'axatoc') carry no abstracts and use a different
// DOI casing than OpenAlex, so leaving them in place would duplicate every
// article the new source re-syncs. Idempotent — a second call finds no rows.
func (s *Store) PurgeArticlesByFeed(ctx context.Context, feeds ...string) (int, error) {
	if len(feeds) == 0 {
		return 0, nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(feeds)), ",")
	args := make([]any, len(feeds))
	for i, f := range feeds {
		args[i] = f
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `SELECT "id" FROM "article" WHERE "feed" IN (`+placeholders+`)`, args...)
	if err != nil {
		// The feed column is added by an idempotent CLI-side migration; a
		// brand-new store that has never synced has no column and nothing
		// to purge.
		if strings.Contains(err.Error(), "no such column") || strings.Contains(err.Error(), "no such table") {
			return 0, nil
		}
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	if len(ids) == 0 {
		return 0, nil
	}

	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `DELETE FROM resources_fts WHERE rowid = ?`, ftsRowID("article", id)); err != nil {
			return 0, fmt.Errorf("purging FTS row for %s: %w", id, err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM resources WHERE resource_type = 'article' AND id = ?`, id); err != nil {
			return 0, fmt.Errorf("purging resource row for %s: %w", id, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM "article" WHERE "feed" IN (`+placeholders+`)`, args...); err != nil {
		return 0, fmt.Errorf("purging article rows: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(ids), nil
}
