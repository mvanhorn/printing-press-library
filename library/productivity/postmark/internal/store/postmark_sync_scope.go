// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Sync checkpoints in sync_state are keyed by resource only, while one
// archive can hold several Postmark servers. postmark_sync_scope records which
// server's token wrote them, so a sync from another server resets them instead
// of resuming a cursor or timestamp that belongs elsewhere.
const postmarkSyncScopeDDL = `CREATE TABLE IF NOT EXISTS postmark_sync_scope (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	scope TEXT NOT NULL,
	updated_at TEXT NOT NULL
)`

// ClaimPostmarkSyncScope makes scope the owner of the sync checkpoints. When
// another scope (or no recorded scope) owns existing checkpoints, every
// checkpoint is reset in the same transaction, so each resource starts from
// the beginning without pruning anything. It reports whether a reset happened.
func (s *Store) ClaimPostmarkSyncScope(ctx context.Context, scope string) (bool, error) {
	if scope == "" {
		return false, errors.New("sync scope is empty")
	}
	s.lockForWrite()
	defer s.unlockAfterWrite()
	if _, err := s.db.ExecContext(ctx, postmarkSyncScopeDDL); err != nil {
		return false, fmt.Errorf("creating sync scope table: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("claiming sync scope: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var previous string
	if err := tx.QueryRowContext(ctx, `SELECT scope FROM postmark_sync_scope WHERE id = 1`).Scan(&previous); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("reading sync scope: %w", err)
	}
	reset := false
	if previous != scope {
		res, err := tx.ExecContext(ctx, `UPDATE sync_state
			SET last_cursor = '', last_synced_at = NULL, last_attempt_complete = 0
			WHERE last_synced_at IS NOT NULL OR COALESCE(last_cursor, '') != ''`)
		if err != nil {
			return false, fmt.Errorf("resetting sync checkpoints: %w", err)
		}
		n, _ := res.RowsAffected()
		reset = n > 0
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO postmark_sync_scope (id, scope, updated_at) VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET scope = excluded.scope, updated_at = excluded.updated_at`,
		scope, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return false, fmt.Errorf("writing sync scope: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("claiming sync scope: %w", err)
	}
	return reset, nil
}
