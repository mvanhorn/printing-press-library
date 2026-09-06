// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.
//
// Package snapshots stores date-stamped captures of Bing Webmaster API
// responses in the same SQLite file the generated store uses. The Bing API
// only ever returns the current window for stats; persisting timestamped
// snapshots is what makes period-over-period deltas, ranking drift, feed
// health trends, and indexation regression detection possible. Hand-authored
// (not generated).
package snapshots

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Snapshot is one stored capture of an API response for a (site, kind) pair.
type Snapshot struct {
	CapturedAt time.Time
	Data       json.RawMessage
}

// DB wraps the snapshot table on the shared SQLite file.
type DB struct {
	db *sql.DB
}

// Open opens (creating if needed) the snapshot store at dbPath and ensures
// the bing_snapshots table exists. dbPath is the same path the generated
// store uses (defaultDBPath).
func Open(dbPath string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}
	sqldb, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("opening snapshot db: %w", err)
	}
	sqldb.SetMaxOpenConns(2)
	d := &DB{db: sqldb}
	if err := d.migrate(); err != nil {
		sqldb.Close()
		return nil, err
	}
	return d, nil
}

func (d *DB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS bing_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			site TEXT NOT NULL,
			kind TEXT NOT NULL,
			captured_at TEXT NOT NULL,
			data JSON NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_bing_snap ON bing_snapshots(site, kind, captured_at)`,
	}
	for _, s := range stmts {
		if _, err := d.db.Exec(s); err != nil {
			return fmt.Errorf("migrating snapshot store: %w", err)
		}
	}
	return nil
}

// Close closes the underlying database handle.
func (d *DB) Close() error { return d.db.Close() }

// Capture records a snapshot for (site, kind) at capturedAt (stored as RFC3339 UTC).
func (d *DB) Capture(site, kind string, data json.RawMessage, capturedAt time.Time) error {
	if len(data) == 0 {
		data = json.RawMessage("null")
	}
	_, err := d.db.Exec(
		`INSERT INTO bing_snapshots(site, kind, captured_at, data) VALUES (?, ?, ?, ?)`,
		site, kind, capturedAt.UTC().Format(time.RFC3339), []byte(data),
	)
	if err != nil {
		return fmt.Errorf("capturing %s snapshot: %w", kind, err)
	}
	return nil
}

// Latest returns the most recent snapshot for (site, kind). ok=false when none exist.
func (d *DB) Latest(site, kind string) (Snapshot, bool, error) {
	return d.queryOne(
		`SELECT captured_at, data FROM bing_snapshots WHERE site=? AND kind=? ORDER BY captured_at DESC, id DESC LIMIT 1`,
		site, kind,
	)
}

// Before returns the most recent snapshot at or before t. ok=false when none exist.
func (d *DB) Before(site, kind string, t time.Time) (Snapshot, bool, error) {
	return d.queryOne(
		`SELECT captured_at, data FROM bing_snapshots WHERE site=? AND kind=? AND captured_at<=? ORDER BY captured_at DESC, id DESC LIMIT 1`,
		site, kind, t.UTC().Format(time.RFC3339),
	)
}

// Prior returns the most recent snapshot strictly before t. Used to find the
// previous capture when an exact N-days-ago baseline does not exist yet, so a
// useful diff appears as soon as two snapshots are stored. ok=false when none.
func (d *DB) Prior(site, kind string, t time.Time) (Snapshot, bool, error) {
	return d.queryOne(
		`SELECT captured_at, data FROM bing_snapshots WHERE site=? AND kind=? AND captured_at<? ORDER BY captured_at DESC, id DESC LIMIT 1`,
		site, kind, t.UTC().Format(time.RFC3339),
	)
}

// All returns every snapshot for (site, kind) ordered oldest-first.
func (d *DB) All(site, kind string) ([]Snapshot, error) {
	rows, err := d.db.Query(
		`SELECT captured_at, data FROM bing_snapshots WHERE site=? AND kind=? ORDER BY captured_at ASC, id ASC`,
		site, kind,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Snapshot
	for rows.Next() {
		var ts string
		var data []byte
		if err := rows.Scan(&ts, &data); err != nil {
			return nil, err
		}
		t, _ := time.Parse(time.RFC3339, ts)
		out = append(out, Snapshot{CapturedAt: t, Data: append(json.RawMessage(nil), data...)})
	}
	return out, rows.Err()
}

func (d *DB) queryOne(q string, args ...any) (Snapshot, bool, error) {
	var ts string
	var data []byte
	err := d.db.QueryRow(q, args...).Scan(&ts, &data)
	if err == sql.ErrNoRows {
		return Snapshot{}, false, nil
	}
	if err != nil {
		return Snapshot{}, false, err
	}
	t, _ := time.Parse(time.RFC3339, ts)
	return Snapshot{CapturedAt: t, Data: append(json.RawMessage(nil), data...)}, true, nil
}
