// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Flighty store extension: the airport snapshot-history table
// that powers `airports diff`. Not generator-emitted; keep in its own file.

package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// EnsureFlightySnapshotTable creates the airport_snapshots table if needed.
// Each sync records one row per tracked airport so `airports diff` can
// compare the two most recent snapshots. Flighty's site keeps no history
// (every page is a live snapshot), so this table is the only source of
// "what changed since last sync".
func (s *Store) EnsureFlightySnapshotTable(ctx context.Context) error {
	_, err := s.DB().ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS airport_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			synced_at TEXT NOT NULL,
			iata TEXT NOT NULL,
			data TEXT NOT NULL
		)`)
	if err != nil {
		return fmt.Errorf("creating airport_snapshots table: %w", err)
	}
	_, err = s.DB().ExecContext(ctx,
		`CREATE INDEX IF NOT EXISTS idx_airport_snapshots_synced ON airport_snapshots (synced_at)`)
	return err
}

// FlightySnapshotRow is one recorded airport state.
type FlightySnapshotRow struct {
	SyncedAt time.Time       `json:"synced_at"`
	IATA     string          `json:"iata"`
	Data     json.RawMessage `json:"data"`
}

// InsertFlightySnapshot stores one airport row under the given timestamp.
func (s *Store) InsertFlightySnapshot(ctx context.Context, syncedAt time.Time, iata string, data json.RawMessage) error {
	if iata == "" {
		return nil
	}
	_, err := s.DB().ExecContext(ctx,
		`INSERT INTO airport_snapshots (synced_at, iata, data) VALUES (?, ?, ?)`,
		syncedAt.UTC().Format(time.RFC3339Nano), iata, string(data))
	return err
}

// LatestFlightySnapshotTimes returns the two most recent distinct snapshot
// timestamps, newest first. ok is false when fewer than two exist.
func (s *Store) LatestFlightySnapshotTimes(ctx context.Context) (newest, previous time.Time, ok bool) {
	rows, err := s.DB().QueryContext(ctx, `
		SELECT synced_at FROM airport_snapshots GROUP BY synced_at ORDER BY MAX(id) DESC LIMIT 2`)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	defer rows.Close()
	var times []time.Time
	for rows.Next() {
		var ts string
		if err := rows.Scan(&ts); err != nil {
			continue
		}
		if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			times = append(times, t)
		}
	}
	if len(times) < 2 {
		return time.Time{}, time.Time{}, false
	}
	return times[0], times[1], true
}

// FlightySnapshotsAt returns all airport rows recorded at the given timestamp.
func (s *Store) FlightySnapshotsAt(ctx context.Context, at time.Time) (map[string]json.RawMessage, error) {
	rows, err := s.DB().QueryContext(ctx,
		`SELECT iata, data FROM airport_snapshots WHERE synced_at = ?`,
		at.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]json.RawMessage{}
	for rows.Next() {
		var iata, data string
		if err := rows.Scan(&iata, &data); err != nil {
			continue
		}
		out[iata] = json.RawMessage(data)
	}
	return out, rows.Err()
}
