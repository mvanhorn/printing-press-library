// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
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

	if err := insertDesignSnapshots(ctx, tx, syncAt, rows); err != nil {
		return err
	}
	if err := retainRecentDesignSnapshots(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

// SaveCompletedDesignSync commits a completed designs watermark and its
// analytics snapshot together. If either write fails, neither becomes visible,
// so a retry cannot lose the failed run's comparison baseline.
func (s *Store) SaveCompletedDesignSync(ctx context.Context, count int, rows []SnapshotRow) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	syncAt := time.Now().UTC().Format(time.RFC3339Nano)
	if err := insertDesignSnapshots(ctx, tx, syncAt, rows); err != nil {
		return err
	}
	if err := retainRecentDesignSnapshots(ctx, tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO sync_state (resource_type, last_cursor, last_synced_at, total_count)
		 VALUES ('designs', '', ?, ?)
		 ON CONFLICT(resource_type) DO UPDATE SET last_cursor = excluded.last_cursor,
		 last_synced_at = excluded.last_synced_at, total_count = excluded.total_count`,
		syncAt, count,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func insertDesignSnapshots(ctx context.Context, tx *sql.Tx, syncAt string, rows []SnapshotRow) error {
	if syncAt == "" || len(rows) == 0 {
		return nil
	}
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
	return nil
}

type snapshotQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

// RecentDesignSnapshotTimes returns snapshot batches in actual time order.
// RFC3339Nano's variable-width fractional seconds are not text-sortable.
func RecentDesignSnapshotTimes(ctx context.Context, db snapshotQuerier) ([]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT DISTINCT sync_at FROM design_snapshots`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type snapshotTime struct {
		stamp string
		when  time.Time
	}
	var times []snapshotTime
	for rows.Next() {
		var stamp string
		if err := rows.Scan(&stamp); err != nil {
			return nil, err
		}
		when, err := time.Parse(time.RFC3339Nano, stamp)
		if err != nil {
			// Unknown legacy data cannot be ordered or pruned safely.
			return nil, fmt.Errorf("invalid local design snapshot timestamp; repair snapshot history before resync")
		}
		times = append(times, snapshotTime{stamp: stamp, when: when})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(times, func(i, j int) bool {
		if !times[i].when.Equal(times[j].when) {
			return times[i].when.After(times[j].when)
		}
		return times[i].stamp > times[j].stamp
	})
	stamps := make([]string, len(times))
	for i, snapshot := range times {
		stamps[i] = snapshot.stamp
	}
	return stamps, nil
}

// Movers and designer deltas compare only the latest two complete batches.
// Retain them in the same transaction as the new batch and its watermark.
func retainRecentDesignSnapshots(ctx context.Context, tx *sql.Tx) error {
	stamps, err := RecentDesignSnapshotTimes(ctx, tx)
	if err != nil {
		return err
	}
	if len(stamps) <= 2 {
		return nil
	}
	for _, stamp := range stamps[2:] {
		if _, err := tx.ExecContext(ctx, `DELETE FROM design_snapshots WHERE sync_at = ?`, stamp); err != nil {
			return err
		}
	}
	return nil
}
