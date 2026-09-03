// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

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
		// design_snapshots backs the `movers` and `designers deltas` novel
		// commands: one row per design per sync timestamp, so cross-sync deltas
		// can be computed offline. Created here (under the migration lock) so it
		// exists before any command reaches RecordDesignSnapshots.
		`CREATE TABLE IF NOT EXISTS design_snapshots (
			sync_at          TEXT NOT NULL,
			design_id        TEXT NOT NULL,
			title            TEXT,
			creator_id       TEXT,
			creator_name     TEXT,
			like_count       INTEGER,
			download_count   INTEGER,
			print_count      INTEGER,
			collection_count INTEGER,
			comment_count    INTEGER,
			PRIMARY KEY (sync_at, design_id)
		);`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}

// SnapshotRow is one design's metrics captured at a single sync, written into
// the design_snapshots table by the movers / designers-deltas commands.
type SnapshotRow struct {
	DesignID    string
	Title       string
	CreatorID   string
	CreatorName string
	Like        int
	Download    int
	Print       int
	Collection  int
	Comment     int
}

// RecordDesignSnapshots inserts a sync's snapshot rows under the store write
// lock, serialized against all other store writers. INSERT OR IGNORE keeps the
// first capture per (sync_at, design_id) so repeated command runs against the
// same sync do not churn the snapshot. The design_snapshots table is created in
// migrateExtras, so callers do not create it.
func (s *Store) RecordDesignSnapshots(ctx context.Context, syncAt string, rows []SnapshotRow) error {
	if syncAt == "" || len(rows) == 0 {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO design_snapshots
		(sync_at, design_id, title, creator_id, creator_name, like_count, download_count, print_count, collection_count, comment_count)
		VALUES (?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.ExecContext(ctx, syncAt, r.DesignID, r.Title, r.CreatorID, r.CreatorName,
			r.Like, r.Download, r.Print, r.Collection, r.Comment); err != nil {
			return err
		}
	}
	return tx.Commit()
}
