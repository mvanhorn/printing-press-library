// Copyright 2026 Charles Garrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored, NOT generator-emitted. Lives alongside the generated
// store.go file but carries its own header so regen does not wipe it.
// Lazy migration: every novel-feature command calls EnsureNovelTables
// once at the start of RunE before issuing its own SQL. Idempotent —
// CREATE TABLE IF NOT EXISTS / CREATE INDEX IF NOT EXISTS are no-ops on
// subsequent calls.

package store

import "context"

// EnsureNovelTables creates the tables required by the hand-authored novel
// features (drift / snapshot / budget). Idempotent — safe to call from every
// novel command's RunE without coordinating with the main migration runner.
//
// snapshot_labels: tag the current local-store state of a resource so the
// `snapshot diff <a> <b>` command can compute the set difference between
// two tagged points in time. taken_at carries the wall-clock cut-off; the
// diff query selects resources whose synced_at <= label.taken_at.
//
// credit_log: per-call ledger of Semrush API unit balances. Each row is a
// snapshot of units_remaining at a point in time, tagged with the command
// path that triggered the probe. The budget report computes deltas between
// consecutive rows grouped by day or command.
func (s *Store) EnsureNovelTables(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS snapshot_labels (
			label TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			taken_at INTEGER NOT NULL,
			created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
			PRIMARY KEY (label, resource_type)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshot_labels_label ON snapshot_labels(label)`,
		`CREATE TABLE IF NOT EXISTS credit_log (
			ts INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
			command TEXT NOT NULL,
			units_remaining INTEGER NOT NULL,
			balance_source TEXT NOT NULL DEFAULT 'api'
		)`,
		`CREATE INDEX IF NOT EXISTS idx_credit_log_ts ON credit_log(ts)`,
		`CREATE INDEX IF NOT EXISTS idx_credit_log_command ON credit_log(command)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
