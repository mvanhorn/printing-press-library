// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// CaptureSnowJapan records complete normalized projections after the generic
// resource upsert has committed. The current directory and its history share
// one transaction; no pooled upsert is called inside this transaction.
func (s *Store) CaptureSnowJapan(ctx context.Context, facts []json.RawMessage, fullCatalog bool) error {
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS snowjapan_catalog (id TEXT PRIMARY KEY, data TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS snowjapan_snapshots (seq INTEGER PRIMARY KEY AUTOINCREMENT, id TEXT NOT NULL, projection TEXT NOT NULL, observed_at TEXT NOT NULL, data TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS snowjapan_snapshot_identity ON snowjapan_snapshots(id, projection, seq DESC)`,
	} {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if fullCatalog {
		if _, err = tx.ExecContext(ctx, `DELETE FROM snowjapan_catalog`); err != nil {
			return err
		}
	}
	for _, raw := range facts {
		var v map[string]any
		if err = json.Unmarshal(raw, &v); err != nil {
			return err
		}
		id, _ := v["id"].(string)
		projection, _ := v["projection"].(string)
		at, _ := v["observed_at"].(string)
		if id == "" || at == "" || (projection != "catalog-v1" && projection != "detail-v1") {
			return fmt.Errorf("incomplete SnowJapan snapshot projection")
		}
		if fullCatalog {
			if projection != "catalog-v1" {
				return fmt.Errorf("full catalog capture requires catalog projections")
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO snowjapan_catalog(id,data) VALUES(?,?)`, id, string(raw)); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO snowjapan_snapshots(id,projection,observed_at,data) VALUES(?,?,?,?)`, id, projection, at, string(raw)); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM snowjapan_snapshots WHERE id=? AND projection=? AND seq NOT IN (SELECT seq FROM snowjapan_snapshots WHERE id=? AND projection=? ORDER BY seq DESC LIMIT 2)`, id, projection, id, projection); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) SnowJapanCatalog(ctx context.Context) ([]json.RawMessage, bool, error) {
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name='snowjapan_catalog'`).Scan(&exists); err != nil {
		return nil, false, err
	}
	if exists == 0 {
		return []json.RawMessage{}, false, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT data FROM snowjapan_catalog ORDER BY id LIMIT 1501`)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out := make([]json.RawMessage, 0)
	for rows.Next() {
		var v string
		if err = rows.Scan(&v); err != nil {
			return nil, false, err
		}
		out = append(out, json.RawMessage(v))
	}
	if err = rows.Err(); err != nil {
		return nil, false, err
	}
	if len(out) > 1500 {
		return nil, false, fmt.Errorf("saved catalog exceeds the source record bound")
	}
	return out, len(out) > 0, nil
}

// CaptureSnowJapanSeason replaces one complete source-season population atomically.
// Generic resource rows alone cannot prove that a particular winter was captured.
func (s *Store) CaptureSnowJapanSeason(ctx context.Context, season string, facts []json.RawMessage) error {
	if season == "" || len(facts) == 0 || len(facts) > 1500 {
		return fmt.Errorf("a bounded, nonempty complete season capture is required")
	}
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS snowjapan_season_current (season TEXT NOT NULL, id TEXT NOT NULL, data TEXT NOT NULL, PRIMARY KEY(season,id))`,
		`CREATE TABLE IF NOT EXISTS snowjapan_season_capture (season TEXT PRIMARY KEY, observed_at TEXT NOT NULL, record_count INTEGER NOT NULL)`,
	} {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM snowjapan_season_current WHERE season=?`, season); err != nil {
		return err
	}
	var observed string
	for _, raw := range facts {
		var v map[string]any
		if err = json.Unmarshal(raw, &v); err != nil {
			return err
		}
		id, _ := v["id"].(string)
		at, _ := v["observed_at"].(string)
		if id == "" || at == "" || v["season"] != season {
			return fmt.Errorf("incomplete or mismatched SnowJapan season projection")
		}
		if observed == "" {
			observed = at
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO snowjapan_season_current(season,id,data) VALUES(?,?,?)`, season, id, string(raw)); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO snowjapan_season_capture(season,observed_at,record_count) VALUES(?,?,?) ON CONFLICT(season) DO UPDATE SET observed_at=excluded.observed_at,record_count=excluded.record_count`, season, observed, len(facts)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SnowJapanSeason(ctx context.Context, season string) ([]json.RawMessage, bool, string, error) {
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('snowjapan_season_current','snowjapan_season_capture')`).Scan(&exists); err != nil {
		return nil, false, "", err
	}
	if exists != 2 {
		return []json.RawMessage{}, false, "", nil
	}
	var observed string
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT observed_at,record_count FROM snowjapan_season_capture WHERE season=?`, season).Scan(&observed, &count)
	if err == sql.ErrNoRows {
		return []json.RawMessage{}, false, "", nil
	}
	if err != nil {
		return nil, false, "", err
	}
	if count < 1 || count > 1500 {
		return nil, false, "", fmt.Errorf("saved SnowJapan season capture is incomplete")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT data FROM snowjapan_season_current WHERE season=? ORDER BY id LIMIT 1501`, season)
	if err != nil {
		return nil, false, "", err
	}
	defer rows.Close()
	out := make([]json.RawMessage, 0, count)
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return nil, false, "", err
		}
		out = append(out, json.RawMessage(raw))
	}
	if err = rows.Err(); err != nil {
		return nil, false, "", err
	}
	if len(out) != count || count < 1 || count > 1500 {
		return nil, false, "", fmt.Errorf("saved SnowJapan season capture is incomplete")
	}
	return out, true, observed, nil
}

// SnowJapanSnapshotPair returns the latest two observations of the same
// projection, choosing the most recently captured compatible pair. A newer
// single observation cannot replace an available two-observation baseline.
func (s *Store) SnowJapanSnapshotPair(ctx context.Context, id string) ([]json.RawMessage, error) {
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name='snowjapan_snapshots'`).Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return []json.RawMessage{}, nil
	}
	var projection string
	err := s.db.QueryRowContext(ctx, `SELECT projection FROM snowjapan_snapshots WHERE id=? GROUP BY projection ORDER BY (COUNT(*)>=2) DESC, MAX(seq) DESC LIMIT 1`, id).Scan(&projection)
	if err == sql.ErrNoRows {
		return []json.RawMessage{}, nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT data FROM snowjapan_snapshots WHERE id=? AND projection=? ORDER BY seq DESC LIMIT 2`, id, projection)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]json.RawMessage, 0, 2)
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		out = append(out, json.RawMessage(raw))
	}
	return out, rows.Err()
}
