// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: local history tables for the Umami analysis commands
// (watch, new-referrers, pace). Lazy-initialized by the commands that need
// them; separate from the generated migration chain in store.go.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// EnsureUmamiHistory creates the per-day history tables when absent.
func (s *Store) EnsureUmamiHistory(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS umami_stats_daily (
			website_id TEXT NOT NULL,
			day        TEXT NOT NULL,
			pageviews  REAL NOT NULL DEFAULT 0,
			sessions   REAL NOT NULL DEFAULT 0,
			synced_at  TEXT NOT NULL,
			PRIMARY KEY (website_id, day)
		)`,
		`CREATE TABLE IF NOT EXISTS umami_metrics_daily (
			website_id  TEXT NOT NULL,
			day         TEXT NOT NULL,
			metric_type TEXT NOT NULL,
			name        TEXT NOT NULL,
			value       REAL NOT NULL DEFAULT 0,
			synced_at   TEXT NOT NULL,
			PRIMARY KEY (website_id, day, metric_type, name)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_umami_metrics_daily_type
			ON umami_metrics_daily (website_id, metric_type, day)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensuring umami history tables: %w", err)
		}
	}
	return nil
}

// UpsertUmamiStatsDay records one site-day pageviews/sessions row.
func (s *Store) UpsertUmamiStatsDay(ctx context.Context, tx *sql.Tx, websiteID, day string, pageviews, sessions float64) error {
	const q = `INSERT INTO umami_stats_daily (website_id, day, pageviews, sessions, synced_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(website_id, day) DO UPDATE SET
			pageviews = excluded.pageviews,
			sessions  = excluded.sessions,
			synced_at = excluded.synced_at`
	now := time.Now().UTC().Format(time.RFC3339)
	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, q, websiteID, day, pageviews, sessions, now)
	} else {
		_, err = s.db.ExecContext(ctx, q, websiteID, day, pageviews, sessions, now)
	}
	if err != nil {
		return fmt.Errorf("upserting stats day %s/%s: %w", websiteID, day, err)
	}
	return nil
}

// UpsertUmamiMetricDay records one site-day metric row (e.g. a referrer count).
func (s *Store) UpsertUmamiMetricDay(ctx context.Context, tx *sql.Tx, websiteID, day, metricType, name string, value float64) error {
	const q = `INSERT INTO umami_metrics_daily (website_id, day, metric_type, name, value, synced_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(website_id, day, metric_type, name) DO UPDATE SET
			value     = excluded.value,
			synced_at = excluded.synced_at`
	now := time.Now().UTC().Format(time.RFC3339)
	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, q, websiteID, day, metricType, name, value, now)
	} else {
		_, err = s.db.ExecContext(ctx, q, websiteID, day, metricType, name, value, now)
	}
	if err != nil {
		return fmt.Errorf("upserting metric day %s/%s/%s: %w", websiteID, day, metricType, err)
	}
	return nil
}

