// Copyright 2026 "Chris Carson" and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// ninjaoneHistorySchema is the table backing the patch-stuck and
// alert-flappers novel commands. Each row is one observation of an entity
// (a patch on a device, or an alert condition on a device) at a point in
// time. patch-stuck counts distinct run snapshots per entity to find KBs
// that stay broken across multiple syncs; alert-flappers counts events
// within a time window to find conditions that fire repeatedly.
const ninjaoneHistorySchema = `
CREATE TABLE IF NOT EXISTS ninjaone_history (
  kind TEXT NOT NULL,           -- 'patch' | 'alert'
  entity_key TEXT NOT NULL,     -- e.g. "deviceId:kbNumber" or "deviceId:condition"
  run_id TEXT NOT NULL,         -- snapshot id (timestamp string)
  observed_at INTEGER NOT NULL, -- epoch seconds
  detail JSON,
  PRIMARY KEY (kind, entity_key, run_id)
)`

// ensureNinjaoneHistory lazily creates the history table. It is safe to call
// repeatedly; CREATE TABLE IF NOT EXISTS is a no-op once the table exists.
func (s *Store) ensureNinjaoneHistory(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, ninjaoneHistorySchema); err != nil {
		return fmt.Errorf("creating ninjaone_history table: %w", err)
	}
	return nil
}

// RecordHistory inserts one observation. detail is JSON-marshaled and stored
// for later inspection. Re-recording the same (kind, entity_key, run_id)
// triple is idempotent (INSERT OR IGNORE) so re-running a command inside the
// same run snapshot does not inflate counts.
func (s *Store) RecordHistory(ctx context.Context, kind, entityKey, runID string, observedAt int64, detail any) error {
	if err := s.ensureNinjaoneHistory(ctx); err != nil {
		return err
	}
	var detailJSON []byte
	if detail != nil {
		b, err := json.Marshal(detail)
		if err != nil {
			return fmt.Errorf("marshaling history detail: %w", err)
		}
		detailJSON = b
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO ninjaone_history (kind, entity_key, run_id, observed_at, detail)
		 VALUES (?, ?, ?, ?, ?)`,
		kind, entityKey, runID, observedAt, string(detailJSON))
	if err != nil {
		return fmt.Errorf("recording history: %w", err)
	}
	return nil
}

// HistoryEntity is one aggregated row returned by the query methods.
type HistoryEntity struct {
	EntityKey string
	Count     int   // distinct run snapshots (patch) or event count (alert)
	FirstSeen int64 // epoch seconds
	LastSeen  int64 // epoch seconds
}

// EntitiesWithMinRuns returns entities of the given kind that appear in at
// least minRuns DISTINCT run snapshots, ordered by run count descending. Used
// by patch-stuck to find patches that stay broken across multiple syncs.
func (s *Store) EntitiesWithMinRuns(ctx context.Context, kind string, minRuns int) ([]HistoryEntity, error) {
	if err := s.ensureNinjaoneHistory(ctx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT entity_key, COUNT(DISTINCT run_id) AS runs,
		        MIN(observed_at) AS first_seen, MAX(observed_at) AS last_seen
		 FROM ninjaone_history
		 WHERE kind = ?
		 GROUP BY entity_key
		 HAVING runs >= ?
		 ORDER BY runs DESC, entity_key ASC`,
		kind, minRuns)
	if err != nil {
		return nil, fmt.Errorf("querying history runs: %w", err)
	}
	defer rows.Close()
	return scanHistoryEntities(rows)
}

// EventCountsSince returns entities of the given kind with at least minEvents
// observations recorded at or after sinceEpoch, ordered by event count
// descending. Used by alert-flappers to find conditions that fire repeatedly
// inside a window.
func (s *Store) EventCountsSince(ctx context.Context, kind string, sinceEpoch int64, minEvents int) ([]HistoryEntity, error) {
	if err := s.ensureNinjaoneHistory(ctx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT entity_key, COUNT(*) AS events,
		        MIN(observed_at) AS first_seen, MAX(observed_at) AS last_seen
		 FROM ninjaone_history
		 WHERE kind = ? AND observed_at >= ?
		 GROUP BY entity_key
		 HAVING events >= ?
		 ORDER BY events DESC, entity_key ASC`,
		kind, sinceEpoch, minEvents)
	if err != nil {
		return nil, fmt.Errorf("querying history events: %w", err)
	}
	defer rows.Close()
	return scanHistoryEntities(rows)
}

func scanHistoryEntities(rows *sql.Rows) ([]HistoryEntity, error) {
	out := make([]HistoryEntity, 0)
	for rows.Next() {
		var e HistoryEntity
		var first, last sql.NullInt64
		if err := rows.Scan(&e.EntityKey, &e.Count, &first, &last); err != nil {
			return nil, fmt.Errorf("scanning history row: %w", err)
		}
		e.FirstSeen = first.Int64
		e.LastSeen = last.Int64
		out = append(out, e)
	}
	return out, rows.Err()
}
