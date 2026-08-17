// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored SimpleFIN-specific tables. Not generator-emitted.
//
// Accounts, transactions, holdings, and connections live in the generic
// `resources` table (so framework search/sql/provenance work). These dedicated
// tables hold the things the protocol cannot give:
//   - balance_snapshots: a per-sync balance time series (the protocol only
//     returns point-in-time balances).
//   - categories / categorization_rules: local categories (SimpleFIN has none).
//   - transaction_categories: category assignments keyed by transaction id,
//     decoupled from `resources` so they survive a re-sync that overwrites the
//     transaction row.

package store

import "context"

const simplefinSchema = `
CREATE TABLE IF NOT EXISTS balance_snapshots (
	account_id        TEXT NOT NULL,
	snapshot_at       INTEGER NOT NULL,
	balance           REAL NOT NULL,
	available_balance REAL,
	PRIMARY KEY (account_id, snapshot_at)
);
CREATE INDEX IF NOT EXISTS idx_snap_at ON balance_snapshots(snapshot_at DESC);

CREATE TABLE IF NOT EXISTS categorization_rules (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	pattern    TEXT NOT NULL,
	field      TEXT NOT NULL DEFAULT 'description',
	category   TEXT NOT NULL,
	priority   INTEGER NOT NULL DEFAULT 100,
	created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_rules_priority ON categorization_rules(priority);

CREATE TABLE IF NOT EXISTS transaction_categories (
	txn_id     TEXT PRIMARY KEY,
	category   TEXT NOT NULL,
	source     TEXT NOT NULL DEFAULT 'rule',
	updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_txncat_category ON transaction_categories(category);
`

// EnsureSimpleFINSchema creates the SimpleFIN-specific tables if they do not
// exist. Idempotent and cheap; call it from sync and any store-reading command
// before touching these tables.
func (s *Store) EnsureSimpleFINSchema(ctx context.Context) error {
	_, err := s.DB().ExecContext(ctx, simplefinSchema)
	return err
}

// FTSRowID exposes the package-internal resources_fts rowid derivation so
// hand-authored commands that delete from the generic resources table (e.g.
// reconcile --fix) can pair the delete with the matching resources_fts delete
// and avoid leaving stale full-text-search entries.
func FTSRowID(resourceType, id string) int64 {
	return ftsRowID(resourceType, id)
}
