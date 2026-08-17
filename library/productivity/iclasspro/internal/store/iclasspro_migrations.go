// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

// iClassPro-specific store extensions.
//
// These tables live beside the generated schema rather than inside it, so that
// regenerating the CLI never has to reconcile them. They exist because the
// iClassPro portal API is present-tense only: it exposes no updated-at cursor,
// no history, and no deletion feed. Everything the drift, fill-rate, and watch
// commands do is powered by observations this CLI records itself.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// icpStampFormat is fixed-width with nanosecond precision.
//
// Second-precision stamps are not safe here: openings history is keyed by
// (account, kind, entity_id, observed_at), so two observations landing in the
// same wall-clock second collapse into one via INSERT OR REPLACE and half the
// history disappears with no error. Two syncs a few hundred milliseconds apart
// is an ordinary thing to do. Fixed width also keeps the string comparisons in
// the range queries below lexicographically correct against a formatted cutoff.
const icpStampFormat = "2006-01-02T15:04:05.000000000Z07:00"

func icpStamp(t time.Time) string { return t.UTC().Format(icpStampFormat) }

// icpParseStamp accepts both the fixed-width form and any RFC3339 value written
// by an earlier version of this CLI.
func icpParseStamp(s string) (time.Time, bool) {
	if t, err := time.Parse(icpStampFormat, s); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// icpSchema is applied lazily the first time an iClassPro-specific table is
// needed. Every statement is IF NOT EXISTS so repeated calls are free.
var icpSchema = []string{
	`CREATE TABLE IF NOT EXISTS icp_sync_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		account TEXT NOT NULL,
		started_at DATETIME NOT NULL,
		entity_count INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE INDEX IF NOT EXISTS idx_icp_runs_account ON icp_sync_runs(account, started_at)`,
	`CREATE TABLE IF NOT EXISTS icp_snapshot (
		run_id INTEGER NOT NULL,
		account TEXT NOT NULL,
		kind TEXT NOT NULL,
		entity_id INTEGER NOT NULL,
		data JSON NOT NULL,
		PRIMARY KEY (run_id, account, kind, entity_id)
	)`,
	`CREATE TABLE IF NOT EXISTS icp_openings_history (
		account TEXT NOT NULL,
		kind TEXT NOT NULL,
		entity_id INTEGER NOT NULL,
		name TEXT NOT NULL DEFAULT '',
		observed_at DATETIME NOT NULL,
		openings INTEGER NOT NULL DEFAULT 0,
		future_openings INTEGER NOT NULL DEFAULT 0,
		has_openings INTEGER NOT NULL DEFAULT 0,
		allow_waitlist INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (account, kind, entity_id, observed_at)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_icp_openings_observed ON icp_openings_history(account, observed_at)`,
}

// EnsureICPSchema creates the iClassPro-specific tables if they do not exist.
func (s *Store) EnsureICPSchema(ctx context.Context) error {
	for _, stmt := range icpSchema {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("icp schema: %w", err)
		}
	}
	return nil
}

// ICPObservation is one recorded sighting of an entity's availability.
type ICPObservation struct {
	Account       string
	Kind          string
	EntityID      int
	Name          string
	Openings      int
	FutureOpen    int
	HasOpenings   bool
	AllowWaitlist bool
	Data          json.RawMessage
}

// StartICPRun opens a sync run and returns its id.
func (s *Store) StartICPRun(ctx context.Context, account string, at time.Time) (int64, error) {
	if err := s.EnsureICPSchema(ctx); err != nil {
		return 0, err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO icp_sync_runs (account, started_at, entity_count) VALUES (?, ?, 0)`,
		account, icpStamp(at))
	if err != nil {
		return 0, fmt.Errorf("start sync run: %w", err)
	}
	return res.LastInsertId()
}

// RecordICPObservations writes one run's worth of snapshot rows and openings
// history in a single transaction.
//
// This does not call Upsert: that helper opens its own write transaction, and
// SQLite permits only one writer, so nesting the two would deadlock or fail
// under a busy timeout. Callers that also want rows in the generic `resources`
// table must call Upsert separately, after this returns.
func (s *Store) RecordICPObservations(ctx context.Context, runID int64, at time.Time, obs []ICPObservation) error {
	if len(obs) == 0 {
		return nil
	}
	if err := s.EnsureICPSchema(ctx); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin observation tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	snap, err := tx.PrepareContext(ctx,
		`INSERT OR REPLACE INTO icp_snapshot (run_id, account, kind, entity_id, data) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare snapshot insert: %w", err)
	}
	defer func() { _ = snap.Close() }()

	hist, err := tx.PrepareContext(ctx,
		`INSERT OR REPLACE INTO icp_openings_history
		 (account, kind, entity_id, name, observed_at, openings, future_openings, has_openings, allow_waitlist)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare history insert: %w", err)
	}
	defer func() { _ = hist.Close() }()

	stamp := icpStamp(at)
	for _, o := range obs {
		if _, err := snap.ExecContext(ctx, runID, o.Account, o.Kind, o.EntityID, string(o.Data)); err != nil {
			return fmt.Errorf("write snapshot row: %w", err)
		}
		if _, err := hist.ExecContext(ctx, o.Account, o.Kind, o.EntityID, o.Name, stamp,
			o.Openings, o.FutureOpen, boolToInt(o.HasOpenings), boolToInt(o.AllowWaitlist)); err != nil {
			return fmt.Errorf("write openings history: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE icp_sync_runs SET entity_count = ? WHERE id = ?`, len(obs), runID); err != nil {
		return fmt.Errorf("update run count: %w", err)
	}
	return tx.Commit()
}

// RecordICPOpenings appends availability observations WITHOUT writing a catalog
// snapshot or opening a sync run.
//
// This is the path `watch` uses. Watch may be scoped to a single class, so the
// entities it sees are a filtered subset of the catalog. Writing those as a
// snapshot would make the newest run a partial one, and every command that
// reads "the latest snapshot" (drift, lint, calendar, opens-soon, compare)
// would then treat a one-class poll as the entire catalog — drift in particular
// would report every other class as removed. Only `sync` writes snapshots.
func (s *Store) RecordICPOpenings(ctx context.Context, at time.Time, obs []ICPObservation) error {
	if len(obs) == 0 {
		return nil
	}
	if err := s.EnsureICPSchema(ctx); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin openings tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	hist, err := tx.PrepareContext(ctx,
		`INSERT OR REPLACE INTO icp_openings_history
		 (account, kind, entity_id, name, observed_at, openings, future_openings, has_openings, allow_waitlist)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare history insert: %w", err)
	}
	defer func() { _ = hist.Close() }()

	stamp := icpStamp(at)
	for _, o := range obs {
		if _, err := hist.ExecContext(ctx, o.Account, o.Kind, o.EntityID, o.Name, stamp,
			o.Openings, o.FutureOpen, boolToInt(o.HasOpenings), boolToInt(o.AllowWaitlist)); err != nil {
			return fmt.Errorf("write openings history: %w", err)
		}
	}
	return tx.Commit()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ICPRun describes one completed sync.
type ICPRun struct {
	ID        int64
	Account   string
	StartedAt time.Time
	Entities  int
}

// ICPRuns returns runs for an account, newest first.
func (s *Store) ICPRuns(ctx context.Context, account string, limit int) ([]ICPRun, error) {
	if err := s.EnsureICPSchema(ctx); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, account, started_at, entity_count FROM icp_sync_runs
		 WHERE (? = '' OR account = ?) AND entity_count > 0
		 ORDER BY started_at DESC, id DESC LIMIT ?`, account, account, limit)
	if err != nil {
		return nil, fmt.Errorf("list sync runs: %w", err)
	}
	out := make([]ICPRun, 0)
	for rows.Next() {
		var (
			r       ICPRun
			started sql.NullString
			acct    sql.NullString
			count   sql.NullInt64
		)
		if err := rows.Scan(&r.ID, &acct, &started, &count); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan sync run: %w", err)
		}
		r.Account = acct.String
		r.Entities = int(count.Int64)
		if started.Valid {
			if t, ok := icpParseStamp(started.String); ok {
				r.StartedAt = t
			}
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate sync runs: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close sync runs: %w", err)
	}
	return out, nil
}

// ICPSnapshot returns the raw entity payloads recorded for one run.
func (s *Store) ICPSnapshot(ctx context.Context, runID int64) ([]json.RawMessage, error) {
	if err := s.EnsureICPSchema(ctx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT data FROM icp_snapshot WHERE run_id = ? ORDER BY kind, entity_id`, runID)
	if err != nil {
		return nil, fmt.Errorf("read snapshot: %w", err)
	}
	out := make([]json.RawMessage, 0)
	for rows.Next() {
		var data sql.NullString
		if err := rows.Scan(&data); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan snapshot row: %w", err)
		}
		if data.Valid && data.String != "" {
			out = append(out, json.RawMessage(data.String))
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate snapshot: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close snapshot: %w", err)
	}
	return out, nil
}

// ICPHistoryRow is one openings observation read back out of the store.
type ICPHistoryRow struct {
	Account    string
	Kind       string
	EntityID   int
	Name       string
	ObservedAt time.Time
	Openings   int
}

// ICPHistory returns openings observations for an account since a cutoff,
// oldest first.
func (s *Store) ICPHistory(ctx context.Context, account string, since time.Time) ([]ICPHistoryRow, error) {
	if err := s.EnsureICPSchema(ctx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT account, kind, entity_id, name, observed_at, openings
		 FROM icp_openings_history
		 WHERE (? = '' OR account = ?) AND observed_at >= ?
		 ORDER BY observed_at ASC`,
		account, account, icpStamp(since))
	if err != nil {
		return nil, fmt.Errorf("read openings history: %w", err)
	}
	out := make([]ICPHistoryRow, 0)
	for rows.Next() {
		var (
			r        ICPHistoryRow
			acct     sql.NullString
			kind     sql.NullString
			name     sql.NullString
			observed sql.NullString
			openings sql.NullInt64
			entityID sql.NullInt64
		)
		if err := rows.Scan(&acct, &kind, &entityID, &name, &observed, &openings); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan history row: %w", err)
		}
		r.Account = acct.String
		r.Kind = kind.String
		r.Name = name.String
		r.EntityID = int(entityID.Int64)
		r.Openings = int(openings.Int64)
		if observed.Valid {
			if t, ok := icpParseStamp(observed.String); ok {
				r.ObservedAt = t
			}
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate history: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close history: %w", err)
	}
	return out, nil
}

// PruneICPRuns keeps only the most recent `keep` runs per account, deleting the
// snapshot rows that belong to older runs. Openings history is retained: it is
// small, and the fill-rate command's value grows with its depth.
func (s *Store) PruneICPRuns(ctx context.Context, account string, keep int) error {
	if keep <= 0 {
		keep = 20
	}
	if err := s.EnsureICPSchema(ctx); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM icp_snapshot WHERE run_id IN (
			SELECT id FROM icp_sync_runs WHERE account = ?
			ORDER BY started_at DESC, id DESC LIMIT -1 OFFSET ?
		 )`, account, keep)
	if err != nil {
		return fmt.Errorf("prune snapshots: %w", err)
	}
	return nil
}
