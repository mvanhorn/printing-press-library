// Copyright 2026 servosity. Licensed under Apache-2.0. See LICENSE.

// Package snapshot persists time-keyed JSON payloads for trend & drift commands.
// Used by the `attention` and `drift` transcendence commands to record what
// the CLI saw on each run and to compare two points in time.
package snapshot

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

const createTableSQL = `
CREATE TABLE IF NOT EXISTS pp_snapshots (
	metric     TEXT NOT NULL,
	taken_at   TEXT NOT NULL,
	data       BLOB NOT NULL,
	PRIMARY KEY (metric, taken_at)
);
CREATE INDEX IF NOT EXISTS pp_snapshots_metric_idx ON pp_snapshots(metric, taken_at DESC);
`

// Snapshot is one time-keyed JSON payload.
type Snapshot struct {
	Metric  string          `json:"metric"`
	TakenAt time.Time       `json:"taken_at"`
	Data    json.RawMessage `json:"data"`
}

// EnsureTable creates the snapshots table if it does not exist. Callers that
// know the store is fresh-open may call this once at startup; callers that
// only sometimes write snapshots may call it inline before Save.
func EnsureTable(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, createTableSQL); err != nil {
		return fmt.Errorf("snapshot.EnsureTable: %w", err)
	}
	return nil
}

// Save persists a snapshot. Idempotent on (metric, taken_at).
func Save(ctx context.Context, db *sql.DB, metric string, takenAt time.Time, data json.RawMessage) error {
	if err := EnsureTable(ctx, db); err != nil {
		return err
	}
	stamp := takenAt.UTC().Format(time.RFC3339Nano)
	_, err := db.ExecContext(ctx,
		`INSERT OR REPLACE INTO pp_snapshots(metric, taken_at, data) VALUES (?, ?, ?)`,
		metric, stamp, []byte(data))
	if err != nil {
		return fmt.Errorf("snapshot.Save(%s): %w", metric, err)
	}
	return nil
}

// Latest returns the most-recent snapshot for the given metric, or
// (nil, nil) if none exist.
func Latest(ctx context.Context, db *sql.DB, metric string) (*Snapshot, error) {
	if err := EnsureTable(ctx, db); err != nil {
		return nil, err
	}
	row := db.QueryRowContext(ctx,
		`SELECT metric, taken_at, data FROM pp_snapshots WHERE metric = ? ORDER BY taken_at DESC LIMIT 1`,
		metric)
	return scanOne(row)
}

// At returns the most-recent snapshot at-or-before the given time, or
// (nil, nil) if none exist. Used by drift's --from anchor resolution.
func At(ctx context.Context, db *sql.DB, metric string, at time.Time) (*Snapshot, error) {
	if err := EnsureTable(ctx, db); err != nil {
		return nil, err
	}
	stamp := at.UTC().Format(time.RFC3339Nano)
	row := db.QueryRowContext(ctx,
		`SELECT metric, taken_at, data FROM pp_snapshots WHERE metric = ? AND taken_at <= ? ORDER BY taken_at DESC LIMIT 1`,
		metric, stamp)
	return scanOne(row)
}

// List returns all snapshots for a metric, newest first, up to `limit`
// (use 0 for unbounded).
func List(ctx context.Context, db *sql.DB, metric string, limit int) ([]Snapshot, error) {
	if err := EnsureTable(ctx, db); err != nil {
		return nil, err
	}
	q := `SELECT metric, taken_at, data FROM pp_snapshots WHERE metric = ? ORDER BY taken_at DESC`
	args := []any{metric}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("snapshot.List(%s): %w", metric, err)
	}
	defer rows.Close()
	var out []Snapshot
	for rows.Next() {
		s, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

func scanOne(row *sql.Row) (*Snapshot, error) {
	var s Snapshot
	var stamp string
	var data []byte
	if err := row.Scan(&s.Metric, &stamp, &data); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("snapshot scan: %w", err)
	}
	t, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		return nil, fmt.Errorf("snapshot parse taken_at: %w", err)
	}
	s.TakenAt = t
	s.Data = json.RawMessage(data)
	return &s, nil
}

func scanRow(rows *sql.Rows) (*Snapshot, error) {
	var s Snapshot
	var stamp string
	var data []byte
	if err := rows.Scan(&s.Metric, &stamp, &data); err != nil {
		return nil, fmt.Errorf("snapshot scan: %w", err)
	}
	t, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		return nil, fmt.Errorf("snapshot parse taken_at: %w", err)
	}
	s.TakenAt = t
	s.Data = json.RawMessage(data)
	return &s, nil
}

// ResolveAnchor parses a time anchor like "yesterday", "now", "2026-05-21",
// "2h ago", "30m ago" relative to `now`. Returns (anchor, true) on success.
func ResolveAnchor(now time.Time, s string) (time.Time, bool) {
	switch s {
	case "now":
		return now, true
	case "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), true
	case "yesterday":
		y := now.AddDate(0, 0, -1)
		return time.Date(y.Year(), y.Month(), y.Day(), 0, 0, 0, 0, now.Location()), true
	}
	// Relative ("2h ago", "30m ago", "7d ago")
	if len(s) > 4 && s[len(s)-4:] == " ago" {
		quant := s[:len(s)-4]
		if d, err := time.ParseDuration(quant); err == nil {
			return now.Add(-d), true
		}
		// Try days: "7d"
		if len(quant) >= 2 && quant[len(quant)-1] == 'd' {
			var n int
			if _, err := fmt.Sscanf(quant[:len(quant)-1], "%d", &n); err == nil {
				return now.AddDate(0, 0, -n), true
			}
		}
	}
	// Absolute RFC3339 or YYYY-MM-DD
	for _, layout := range []string{time.RFC3339, "2006-01-02", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
