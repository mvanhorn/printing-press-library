// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.

package venuex

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// EnsureShortlistSnapshots creates the shortlist_snapshots table if needed.
func EnsureShortlistSnapshots(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS shortlist_snapshots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	captured_at TEXT NOT NULL,
	fav_ids_json TEXT NOT NULL,
	attrs_json TEXT NOT NULL DEFAULT '{}'
)`); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, `
CREATE INDEX IF NOT EXISTS idx_shortlist_snapshots_captured ON shortlist_snapshots(captured_at)
`)
	return err
}

// SnapshotRow is one stored shortlist snapshot.
type SnapshotRow struct {
	ID         int64
	CapturedAt time.Time
	FavIDs     []string
	Attrs      map[string]SnapshotAttrs
}

// InsertSnapshot stores current favorite ids + attrs.
func InsertSnapshot(ctx context.Context, db *sql.DB, ids []string, attrs map[string]SnapshotAttrs) error {
	if err := EnsureShortlistSnapshots(ctx, db); err != nil {
		return err
	}
	if ids == nil {
		ids = make([]string, 0)
	}
	if attrs == nil {
		attrs = map[string]SnapshotAttrs{}
	}
	idsJSON, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	attrsJSON, err := json.Marshal(attrs)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx,
		`INSERT INTO shortlist_snapshots(captured_at, fav_ids_json, attrs_json) VALUES(?,?,?)`,
		time.Now().UTC().Format(time.RFC3339Nano), string(idsJSON), string(attrsJSON),
	)
	return err
}

// LatestSnapshot returns the most recent snapshot, or ok=false if none.
func LatestSnapshot(ctx context.Context, db *sql.DB) (SnapshotRow, bool, error) {
	if err := EnsureShortlistSnapshots(ctx, db); err != nil {
		return SnapshotRow{}, false, err
	}
	var id int64
	var captured, favJSON, attrsJSON sql.NullString
	err := db.QueryRowContext(ctx,
		`SELECT id, captured_at, fav_ids_json, attrs_json FROM shortlist_snapshots ORDER BY id DESC LIMIT 1`,
	).Scan(&id, &captured, &favJSON, &attrsJSON)
	if err == sql.ErrNoRows {
		return SnapshotRow{}, false, nil
	}
	if err != nil {
		return SnapshotRow{}, false, err
	}
	return parseSnapshotRow(id, captured, favJSON, attrsJSON)
}

// SnapshotSince returns the oldest snapshot at or before now-since, falling back to oldest overall.
func SnapshotSince(ctx context.Context, db *sql.DB, since time.Duration) (SnapshotRow, bool, error) {
	if err := EnsureShortlistSnapshots(ctx, db); err != nil {
		return SnapshotRow{}, false, err
	}
	cutoff := time.Now().UTC().Add(-since).Format(time.RFC3339Nano)
	var id int64
	var captured, favJSON, attrsJSON sql.NullString
	// Prefer the newest snapshot that is still older than cutoff (i.e. at least `since` ago).
	err := db.QueryRowContext(ctx, `
SELECT id, captured_at, fav_ids_json, attrs_json
FROM shortlist_snapshots
WHERE captured_at <= ?
ORDER BY captured_at DESC
LIMIT 1`, cutoff).Scan(&id, &captured, &favJSON, &attrsJSON)
	if err == sql.ErrNoRows {
		// Fall back to oldest snapshot if none old enough.
		err = db.QueryRowContext(ctx, `
SELECT id, captured_at, fav_ids_json, attrs_json
FROM shortlist_snapshots
ORDER BY id ASC
LIMIT 1`).Scan(&id, &captured, &favJSON, &attrsJSON)
		if err == sql.ErrNoRows {
			return SnapshotRow{}, false, nil
		}
	}
	if err != nil {
		return SnapshotRow{}, false, err
	}
	return parseSnapshotRow(id, captured, favJSON, attrsJSON)
}

func parseSnapshotRow(id int64, captured, favJSON, attrsJSON sql.NullString) (SnapshotRow, bool, error) {
	row := SnapshotRow{
		ID:     id,
		FavIDs: make([]string, 0),
		Attrs:  map[string]SnapshotAttrs{},
	}
	if captured.Valid {
		if t, err := time.Parse(time.RFC3339Nano, captured.String); err == nil {
			row.CapturedAt = t
		} else if t, err := time.Parse(time.RFC3339, captured.String); err == nil {
			row.CapturedAt = t
		}
	}
	if favJSON.Valid && favJSON.String != "" {
		if err := json.Unmarshal([]byte(favJSON.String), &row.FavIDs); err != nil {
			return SnapshotRow{}, false, fmt.Errorf("decode fav_ids_json: %w", err)
		}
	}
	if attrsJSON.Valid && attrsJSON.String != "" && attrsJSON.String != "{}" {
		if err := json.Unmarshal([]byte(attrsJSON.String), &row.Attrs); err != nil {
			return SnapshotRow{}, false, fmt.Errorf("decode attrs_json: %w", err)
		}
	}
	if row.FavIDs == nil {
		row.FavIDs = make([]string, 0)
	}
	if row.Attrs == nil {
		row.Attrs = map[string]SnapshotAttrs{}
	}
	return row, true, nil
}
