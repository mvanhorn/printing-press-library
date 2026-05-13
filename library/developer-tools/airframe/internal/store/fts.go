// Copyright 2026 ChrisDrit. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"fmt"
)

// BuildFTS drops and rebuilds the FTS5 virtual tables from the current
// content of aircraft, narratives, and events. Called at the end of `sync`
// when --with-fts is passed. The contentless FTS pattern (`content=”`)
// is intentional: we want a standalone search index that we can rebuild
// from scratch every sync without triggers gluing it to the source tables.
//
// PATCH: all DDL + INSERT statements run inside a single transaction so the
// rebuild is atomic. A mid-step failure (OOM, disk full, ctx cancel) used to
// leave one FTS table created-but-empty and the other absent, with HasFTS()
// returning true and `airframe search` yielding zero results.
func (s *Store) BuildFTS(ctx context.Context, progress func(string)) error {
	s.Lock()
	defer s.Unlock()

	if progress == nil {
		progress = func(string) {}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin FTS rebuild tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmts := []string{
		`DROP TABLE IF EXISTS aircraft_fts`,
		`DROP TABLE IF EXISTS narratives_fts`,
		`CREATE VIRTUAL TABLE aircraft_fts USING fts5(
			registration UNINDEXED, owner_name, manufacturer, model,
			tokenize = 'porter unicode61'
		)`,
		`CREATE VIRTUAL TABLE narratives_fts USING fts5(
			event_id UNINDEXED, event_date UNINDEXED, summary,
			tokenize = 'porter unicode61'
		)`,
	}
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("FTS schema: %w\n%s", err, stmt)
		}
	}

	progress("aircraft_fts_populate")
	if _, err := tx.ExecContext(ctx, `INSERT INTO aircraft_fts (registration, owner_name, manufacturer, model)
		SELECT a.registration, COALESCE(a.owner_name,''),
		       COALESCE(mm.manufacturer,''), COALESCE(mm.model,'')
		FROM aircraft a LEFT JOIN make_model mm ON mm.code = a.make_model_code`); err != nil {
		return fmt.Errorf("populate aircraft_fts: %w", err)
	}

	progress("narratives_fts_populate")
	if _, err := tx.ExecContext(ctx, `INSERT INTO narratives_fts (event_id, event_date, summary)
		SELECT n.event_id, COALESCE(e.event_date,''), COALESCE(n.summary,'')
		FROM narratives n LEFT JOIN events e ON e.event_id = n.event_id`); err != nil {
		return fmt.Errorf("populate narratives_fts: %w", err)
	}

	progress("fts_optimize")
	if _, err := tx.ExecContext(ctx, `INSERT INTO aircraft_fts(aircraft_fts) VALUES('optimize')`); err != nil {
		return fmt.Errorf("optimize aircraft_fts: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO narratives_fts(narratives_fts) VALUES('optimize')`); err != nil {
		return fmt.Errorf("optimize narratives_fts: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit FTS rebuild: %w", err)
	}
	return nil
}

// HasFTS reports whether the FTS5 virtual tables exist in the current DB.
// Used by `airframe search` to short-circuit with a precise install hint
// when --with-fts has not been run.
func (s *Store) HasFTS(ctx context.Context) (bool, error) {
	row := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master
		WHERE type='table' AND name IN ('aircraft_fts','narratives_fts')`)
	var n int
	if err := row.Scan(&n); err != nil {
		return false, err
	}
	return n == 2, nil
}

// FTSAircraftHit is one hit from aircraft_fts.
type FTSAircraftHit struct {
	Registration string
	OwnerName    string
	Manufacturer string
	Model        string
	Snippet      string
}

// FTSNarrativeHit is one hit from narratives_fts.
type FTSNarrativeHit struct {
	EventID   string
	EventDate string
	Snippet   string
}

// SearchAircraftFTS returns aircraft matching the FTS5 query.
func (s *Store) SearchAircraftFTS(ctx context.Context, query string, limit int) ([]FTSAircraftHit, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT registration, owner_name, manufacturer, model,
		snippet(aircraft_fts, -1, '«', '»', '…', 16) AS snip
		FROM aircraft_fts WHERE aircraft_fts MATCH ?
		ORDER BY rank LIMIT ?`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("aircraft fts query: %w", err)
	}
	defer rows.Close()
	var out []FTSAircraftHit
	for rows.Next() {
		var h FTSAircraftHit
		if err := rows.Scan(&h.Registration, &h.OwnerName, &h.Manufacturer, &h.Model, &h.Snippet); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// SearchNarrativesFTS returns NTSB narratives matching the FTS5 query.
func (s *Store) SearchNarrativesFTS(ctx context.Context, query string, limit int) ([]FTSNarrativeHit, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT event_id, event_date,
		snippet(narratives_fts, -1, '«', '»', '…', 32) AS snip
		FROM narratives_fts WHERE narratives_fts MATCH ?
		ORDER BY rank LIMIT ?`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("narratives fts query: %w", err)
	}
	defer rows.Close()
	var out []FTSNarrativeHit
	for rows.Next() {
		var h FTSNarrativeHit
		if err := rows.Scan(&h.EventID, &h.EventDate, &h.Snippet); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
