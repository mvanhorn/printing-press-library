// Package store extension — hand-authored tables for ExchangeRate-API
// novel features (rates_snapshots, quota_snapshots, watchlist,
// conversions_log). Created on demand by each novel command via
// EnsureExrateTables(ctx); idempotent CREATE IF NOT EXISTS.
package store

// PATCH exchangerate-novel-store-tables: novel-feature SQLite tables (rates_snapshots, quota_snapshots, watchlist, conversions_log) with typed Inserts.

import (
	"context"
	"fmt"
)

// EnsureExrateTables creates the novel-feature tables if they don't exist.
// Idempotent. Safe to call from every novel command's RunE.
func (s *Store) EnsureExrateTables(ctx context.Context) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS rates_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			base_code TEXT NOT NULL,
			target_code TEXT NOT NULL,
			rate REAL NOT NULL,
			source TEXT NOT NULL,
			captured_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rates_snapshots_pair ON rates_snapshots(base_code, target_code, captured_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_rates_snapshots_captured ON rates_snapshots(captured_at DESC)`,

		`CREATE TABLE IF NOT EXISTS quota_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			plan_quota INTEGER NOT NULL,
			requests_remaining INTEGER NOT NULL,
			refresh_day_of_month INTEGER NOT NULL,
			captured_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_quota_snapshots_captured ON quota_snapshots(captured_at DESC)`,

		`CREATE TABLE IF NOT EXISTS watchlist (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			base_code TEXT NOT NULL,
			target_code TEXT NOT NULL,
			threshold_pct REAL NOT NULL,
			last_known_rate REAL,
			last_checked_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(base_code, target_code)
		)`,

		`CREATE TABLE IF NOT EXISTS conversions_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			base_code TEXT NOT NULL,
			target_code TEXT NOT NULL,
			amount REAL NOT NULL,
			result REAL NOT NULL,
			rate REAL NOT NULL,
			source TEXT NOT NULL,
			captured_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_conversions_log_captured ON conversions_log(captured_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_conversions_log_pair ON conversions_log(base_code, target_code)`,
	}
	for _, q := range stmts {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("ensure exrate tables: %w", err)
		}
	}
	return nil
}

// InsertRateSnapshots appends a batch of rate observations from one /latest
// fetch. Each (base, target, rate) row is inserted as a new snapshot row;
// the table is append-only so history accumulates over time.
func (s *Store) InsertRateSnapshots(ctx context.Context, base, source string, rates map[string]float64) (int, error) {
	if err := s.EnsureExrateTables(ctx); err != nil {
		return 0, err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO rates_snapshots(base_code, target_code, rate, source) VALUES(?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()
	count := 0
	for target, rate := range rates {
		if _, err := stmt.ExecContext(ctx, base, target, rate, source); err != nil {
			return count, fmt.Errorf("insert snapshot %s/%s: %w", base, target, err)
		}
		count++
	}
	if err := tx.Commit(); err != nil {
		return count, fmt.Errorf("commit: %w", err)
	}
	return count, nil
}

// InsertQuotaSnapshot appends one quota observation.
func (s *Store) InsertQuotaSnapshot(ctx context.Context, planQuota, remaining, refreshDay int) error {
	if err := s.EnsureExrateTables(ctx); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO quota_snapshots(plan_quota, requests_remaining, refresh_day_of_month) VALUES(?, ?, ?)`,
		planQuota, remaining, refreshDay)
	if err != nil {
		return fmt.Errorf("insert quota snapshot: %w", err)
	}
	return nil
}

// InsertConversionLog appends one conversion record.
func (s *Store) InsertConversionLog(ctx context.Context, base, target string, amount, result, rate float64, source string) error {
	if err := s.EnsureExrateTables(ctx); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO conversions_log(base_code, target_code, amount, result, rate, source) VALUES(?, ?, ?, ?, ?, ?)`,
		base, target, amount, result, rate, source)
	if err != nil {
		return fmt.Errorf("insert conversion log: %w", err)
	}
	return nil
}

// UpdateWatchObservation records the rate observed for a watched pair under
// the store's writeMu so concurrent InsertRateSnapshots cannot race the
// UPDATE on a busy/locked SQLite connection.
func (s *Store) UpdateWatchObservation(ctx context.Context, base, target string, rate float64, observedAt string) error {
	if err := s.EnsureExrateTables(ctx); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`UPDATE watchlist SET last_known_rate = ?, last_checked_at = ? WHERE base_code = ? AND target_code = ?`,
		rate, observedAt, base, target)
	if err != nil {
		return fmt.Errorf("update watch observation: %w", err)
	}
	return nil
}

// DB() is provided by store.go.
