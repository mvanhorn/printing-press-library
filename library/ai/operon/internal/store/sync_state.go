// Copyright 2026 yaooooooooooooooo. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel store — not generated.

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// RecordSync stamps the last_synced_at + rows_synced for a table name.
func (s *Store) RecordSync(ctx context.Context, table string, rowsSynced int) error {
	if table == "" {
		return errors.New("store: table name required")
	}
	now := time.Now().UnixMilli()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sync_state (table_name, last_synced_at, rows_synced)
		VALUES (?, ?, ?)
		ON CONFLICT(table_name) DO UPDATE SET
		  last_synced_at = excluded.last_synced_at,
		  rows_synced    = excluded.rows_synced`,
		table, now, rowsSynced)
	if err != nil {
		return fmt.Errorf("store: record sync: %w", err)
	}
	return nil
}

// LastSync returns (lastSyncedAtMs, rowsSynced) for the table, or (0,0)
// when there is no row.
func (s *Store) LastSync(ctx context.Context, table string) (int64, int, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT last_synced_at, rows_synced FROM sync_state WHERE table_name=?`, table)
	var (
		ts   int64
		rows int
	)
	switch err := row.Scan(&ts, &rows); err {
	case nil:
		return ts, rows, nil
	case sql.ErrNoRows:
		return 0, 0, nil
	default:
		return 0, 0, err
	}
}
