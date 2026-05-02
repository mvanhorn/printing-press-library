// Hand-written novel-feature helpers. Not generator-emitted.

package cli

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/store"
)

// novelOpenStore opens the local SQLite store at the default path and ensures
// the snapshot tables this CLI's transcendence commands rely on exist. The
// generator-emitted store creates the per-platform tables and `resources_fts`;
// this layer adds the snapshot tables without touching the DO-NOT-EDIT
// generator file.
func novelOpenStore(ctx context.Context) (*store.Store, error) {
	dbPath := defaultDBPath("scrape-creators-pp-cli")
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening store: %w", err)
	}
	if err := ensureSnapshotTables(ctx, s); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// ensureSnapshotTables creates the snapshot tables that back creator track,
// trends delta, and ads monitor. Idempotent.
func ensureSnapshotTables(ctx context.Context, s *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS profile_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			handle TEXT NOT NULL,
			platform TEXT NOT NULL,
			follower_count INTEGER,
			following_count INTEGER,
			content_count INTEGER,
			data TEXT,
			snapshot_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_profile_snapshots_lookup
			ON profile_snapshots(handle, platform, snapshot_at)`,
		`CREATE TABLE IF NOT EXISTS trend_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			topic TEXT NOT NULL,
			platform TEXT NOT NULL,
			result_count INTEGER,
			snapshot_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_trend_snapshots_lookup
			ON trend_snapshots(topic, platform, snapshot_at)`,
		`CREATE TABLE IF NOT EXISTS ad_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			brand TEXT NOT NULL,
			platform TEXT NOT NULL,
			ad_id TEXT NOT NULL,
			fingerprint TEXT,
			data TEXT,
			snapshot_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ad_snapshots_lookup
			ON ad_snapshots(brand, platform, ad_id, snapshot_at)`,
		`CREATE TABLE IF NOT EXISTS usage_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			command TEXT NOT NULL,
			endpoint TEXT,
			credits INTEGER,
			logged_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_log_logged_at ON usage_log(logged_at)`,
	}
	for _, q := range stmts {
		if _, err := s.DB().ExecContext(ctx, q); err != nil {
			return fmt.Errorf("create snapshot tables: %w", err)
		}
	}
	return nil
}

// parseTimeFlexible accepts ISO8601, RFC3339, and Unix epoch (seconds or
// milliseconds, as integer or string) and returns a UTC time. Returns the
// zero value when the input doesn't parse.
func parseTimeFlexible(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		// Heuristic: 13-digit values are milliseconds, 10-digit are seconds.
		if n > 1_000_000_000_000 {
			return time.UnixMilli(n).UTC()
		}
		if n > 1_000_000_000 {
			return time.Unix(n, 0).UTC()
		}
	}
	return time.Time{}
}
