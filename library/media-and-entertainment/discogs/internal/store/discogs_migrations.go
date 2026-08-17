// Copyright 2026 ganes-j and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// discogsTableStmts are the hand-authored tables that back the price-history
// moat: the Discogs API keeps no price history, so the CLI persists its own
// marketplace snapshots plus the per-want limit prices that drive `fills`.
var discogsTableStmts = []string{
	`CREATE TABLE IF NOT EXISTS price_snapshots (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		release_id   INTEGER NOT NULL,
		captured_at  TEXT NOT NULL,
		currency     TEXT,
		lowest_price REAL,
		num_for_sale INTEGER,
		blocked      INTEGER DEFAULT 0,
		source       TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_price_snapshots_release ON price_snapshots(release_id, captured_at)`,
	`CREATE TABLE IF NOT EXISTS wantlist_limits (
		release_id INTEGER PRIMARY KEY,
		max_price  REAL NOT NULL,
		currency   TEXT,
		note       TEXT,
		set_at     TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS discogs_meta (
		key   TEXT PRIMARY KEY,
		value TEXT
	)`,
}

// EnsureDiscogsTables lazily creates the hand-authored Discogs tables. It is
// safe to call on every command entry (CREATE TABLE IF NOT EXISTS is a no-op
// once the table exists).
func (s *Store) EnsureDiscogsTables(ctx context.Context) error {
	for _, stmt := range discogsTableStmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("discogs migration: %w", err)
		}
	}
	return nil
}

// MetaGet reads a cached key (e.g. the resolved username). Returns ("", false)
// when the key is unset. A missing table is treated as unset, not an error.
func (s *Store) MetaGet(ctx context.Context, key string) (string, bool) {
	var v sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT value FROM discogs_meta WHERE key = ?`, key).Scan(&v)
	if err != nil || !v.Valid {
		return "", false
	}
	return v.String, v.String != ""
}

// MetaSet upserts a cached key/value.
func (s *Store) MetaSet(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO discogs_meta (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// InsertPriceSnapshot records one marketplace snapshot for a release.
func (s *Store) InsertPriceSnapshot(ctx context.Context, releaseID int64, currency string, lowest sql.NullFloat64, numForSale sql.NullInt64, blocked bool, source string) error {
	blockedInt := 0
	if blocked {
		blockedInt = 1
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO price_snapshots (release_id, captured_at, currency, lowest_price, num_for_sale, blocked, source)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		releaseID, time.Now().UTC().Format(time.RFC3339), currency, lowest, numForSale, blockedInt, source)
	return err
}

// SetWantlistLimit upserts a per-release limit price.
func (s *Store) SetWantlistLimit(ctx context.Context, releaseID int64, maxPrice float64, currency, note string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO wantlist_limits (release_id, max_price, currency, note, set_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(release_id) DO UPDATE SET max_price = excluded.max_price, currency = excluded.currency, note = excluded.note, set_at = excluded.set_at`,
		releaseID, maxPrice, currency, note, time.Now().UTC().Format(time.RFC3339))
	return err
}

// DeleteWantlistLimit removes a limit. Returns whether a row was deleted.
func (s *Store) DeleteWantlistLimit(ctx context.Context, releaseID int64) (bool, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM wantlist_limits WHERE release_id = ?`, releaseID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
