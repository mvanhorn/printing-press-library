// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

// Package snapshotstore is a tiny side-database that records per-workspace
// agent state every time we observe it. The main generator-emitted store
// owns the resource snapshots; this store owns the time-series of state
// transitions, which the generator does not model.
//
// Schema:
//
//	status_snapshots(workspace_ref, key, value, icon, color, canonical,
//	                 observed_at_unix REAL, transitioned BOOL, prev_value TEXT)
//	alert_rules(id INTEGER PK, workspace_ref TEXT, key TEXT, on_state TEXT,
//	            sink TEXT, label TEXT, created_at_unix REAL)
//	alert_fires(id INTEGER PK, rule_id INTEGER, workspace_ref TEXT, key TEXT,
//	            prev_value TEXT, new_value TEXT, fired_at_unix REAL, sink TEXT,
//	            outcome TEXT, error TEXT)
//	notification_seen(notification_id TEXT PRIMARY KEY, seen_at_unix REAL)
//	pane_content_samples(workspace_ref, surface_ref, text, sampled_at_unix)
//	pane_content_samples_fts(... FTS5 ...)
package snapshotstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

// DefaultPath returns the canonical SQLite path for the snapshot DB.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "cmux-pp-cli", "snapshots.db")
}

// Open opens (creating if needed) the snapshot store at path.
func Open(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating snapshot dir: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// DB returns the underlying *sql.DB for ad-hoc queries.
func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS status_snapshots (
			workspace_ref     TEXT NOT NULL,
			key               TEXT NOT NULL,
			value             TEXT NOT NULL,
			icon              TEXT,
			color             TEXT,
			canonical         TEXT NOT NULL,
			observed_at_unix  REAL NOT NULL,
			transitioned      INTEGER NOT NULL DEFAULT 0,
			prev_value        TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_status_snapshots_ws_key_ts
			ON status_snapshots(workspace_ref, key, observed_at_unix)`,
		`CREATE INDEX IF NOT EXISTS idx_status_snapshots_transitions
			ON status_snapshots(transitioned, observed_at_unix)`,
		`CREATE TABLE IF NOT EXISTS alert_rules (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			workspace_ref   TEXT,
			key             TEXT NOT NULL DEFAULT 'claude_code',
			on_state        TEXT NOT NULL,
			sink            TEXT NOT NULL,
			label           TEXT,
			created_at_unix REAL NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS alert_fires (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			rule_id         INTEGER NOT NULL,
			workspace_ref   TEXT,
			key             TEXT,
			prev_value      TEXT,
			new_value       TEXT,
			fired_at_unix   REAL NOT NULL,
			sink            TEXT,
			outcome         TEXT,
			error           TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS notification_seen (
			notification_id TEXT PRIMARY KEY,
			seen_at_unix    REAL NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS pane_content_samples (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			workspace_ref   TEXT NOT NULL,
			surface_ref     TEXT NOT NULL,
			text            TEXT NOT NULL,
			sampled_at_unix REAL NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pane_samples_ws_surface
			ON pane_content_samples(workspace_ref, surface_ref, sampled_at_unix)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS pane_content_samples_fts USING fts5(
			text, workspace_ref UNINDEXED, surface_ref UNINDEXED, content='pane_content_samples', content_rowid='id'
		)`,
		`CREATE TRIGGER IF NOT EXISTS pcs_after_insert AFTER INSERT ON pane_content_samples
			BEGIN INSERT INTO pane_content_samples_fts(rowid, text) VALUES (new.id, new.text); END`,
		`CREATE TRIGGER IF NOT EXISTS pcs_after_delete AFTER DELETE ON pane_content_samples
			BEGIN INSERT INTO pane_content_samples_fts(pane_content_samples_fts, rowid, text) VALUES('delete', old.id, old.text); END`,
	}
	for _, q := range stmts {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("migrating snapshot store: %w (%s)", err, q[:60])
		}
	}
	return nil
}

// StatusSnapshot is one row in status_snapshots.
type StatusSnapshot struct {
	WorkspaceRef   string  `json:"workspace_ref"`
	Key            string  `json:"key"`
	Value          string  `json:"value"`
	Icon           string  `json:"icon,omitempty"`
	Color          string  `json:"color,omitempty"`
	Canonical      string  `json:"canonical"`
	ObservedAtUnix float64 `json:"observed_at_unix"`
	Transitioned   bool    `json:"transitioned"`
	PrevValue      string  `json:"prev_value,omitempty"`
}

// LatestPerWorkspaceKey returns the most recent snapshot per (workspace_ref, key).
func (s *Store) LatestPerWorkspaceKey(ctx context.Context) ([]StatusSnapshot, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ss.workspace_ref, ss.key, ss.value, ss.icon, ss.color, ss.canonical,
		       ss.observed_at_unix, ss.transitioned, ss.prev_value
		FROM status_snapshots ss
		JOIN (
			SELECT workspace_ref, key, MAX(observed_at_unix) AS mt
			FROM status_snapshots
			GROUP BY workspace_ref, key
		) m ON m.workspace_ref = ss.workspace_ref AND m.key = ss.key AND m.mt = ss.observed_at_unix
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSnapshots(rows)
}

// Timeline returns snapshots for a workspace (or all) since a unix timestamp.
func (s *Store) Timeline(ctx context.Context, workspaceRef string, sinceUnix float64) ([]StatusSnapshot, error) {
	if workspaceRef == "" {
		rows, err := s.db.QueryContext(ctx, `
			SELECT workspace_ref, key, value, icon, color, canonical,
			       observed_at_unix, transitioned, prev_value
			FROM status_snapshots
			WHERE observed_at_unix >= ?
			ORDER BY observed_at_unix DESC
		`, sinceUnix)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return scanSnapshots(rows)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT workspace_ref, key, value, icon, color, canonical,
		       observed_at_unix, transitioned, prev_value
		FROM status_snapshots
		WHERE workspace_ref = ? AND observed_at_unix >= ?
		ORDER BY observed_at_unix DESC
	`, workspaceRef, sinceUnix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSnapshots(rows)
}

// Changes returns only transition rows (transitioned=1) since a unix timestamp.
func (s *Store) Changes(ctx context.Context, sinceUnix float64) ([]StatusSnapshot, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT workspace_ref, key, value, icon, color, canonical,
		       observed_at_unix, transitioned, prev_value
		FROM status_snapshots
		WHERE transitioned = 1 AND observed_at_unix >= ?
		ORDER BY observed_at_unix DESC
	`, sinceUnix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSnapshots(rows)
}

func scanSnapshots(rows *sql.Rows) ([]StatusSnapshot, error) {
	out := make([]StatusSnapshot, 0)
	for rows.Next() {
		var s StatusSnapshot
		var icon, color, prev sql.NullString
		var trans sql.NullInt64
		if err := rows.Scan(&s.WorkspaceRef, &s.Key, &s.Value, &icon, &color, &s.Canonical,
			&s.ObservedAtUnix, &trans, &prev); err != nil {
			return nil, err
		}
		s.Icon = icon.String
		s.Color = color.String
		s.Transitioned = trans.Int64 != 0
		s.PrevValue = prev.String
		out = append(out, s)
	}
	return out, rows.Err()
}

// RecordObservation writes one observation row. If the prior latest row for
// (workspace_ref, key) has the same value, transitioned=false and prev_value
// is empty; otherwise transitioned=true and prev_value carries the prior.
// Returns true when a transition was recorded.
func (s *Store) RecordObservation(ctx context.Context, ws, key, value, icon, color, canonical string, observedAt float64) (bool, error) {
	var prevValue string
	err := s.db.QueryRowContext(ctx, `
		SELECT value FROM status_snapshots
		WHERE workspace_ref = ? AND key = ?
		ORDER BY observed_at_unix DESC LIMIT 1
	`, ws, key).Scan(&prevValue)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("looking up prior snapshot: %w", err)
	}
	transitioned := err == nil && prevValue != value
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		// First observation — treat as a transition from empty to value.
		transitioned = value != ""
		prevValue = ""
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO status_snapshots(workspace_ref, key, value, icon, color, canonical,
			observed_at_unix, transitioned, prev_value)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, ws, key, value, icon, color, canonical, observedAt, boolToInt(transitioned), prevValue)
	if err != nil {
		return false, err
	}
	return transitioned, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Now returns the current unix-seconds time (float).
func Now() float64 { return float64(time.Now().UnixNano()) / 1e9 }

// AlertRule mirrors an alert_rules row.
type AlertRule struct {
	ID            int64   `json:"id"`
	WorkspaceRef  string  `json:"workspace_ref,omitempty"`
	Key           string  `json:"key"`
	OnState       string  `json:"on_state"`
	Sink          string  `json:"sink"`
	Label         string  `json:"label,omitempty"`
	CreatedAtUnix float64 `json:"created_at_unix"`
}

// ListAlertRules returns every alert rule.
func (s *Store) ListAlertRules(ctx context.Context) ([]AlertRule, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(workspace_ref, ''), key, on_state, sink, COALESCE(label, ''), created_at_unix
		FROM alert_rules ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AlertRule, 0)
	for rows.Next() {
		var r AlertRule
		if err := rows.Scan(&r.ID, &r.WorkspaceRef, &r.Key, &r.OnState, &r.Sink, &r.Label, &r.CreatedAtUnix); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AddAlertRule inserts a rule and returns its id.
func (s *Store) AddAlertRule(ctx context.Context, ws, key, onState, sink, label string) (int64, error) {
	if key == "" {
		key = "claude_code"
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO alert_rules(workspace_ref, key, on_state, sink, label, created_at_unix)
		VALUES(NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''), ?)
	`, ws, key, onState, sink, label, Now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// RemoveAlertRule deletes a rule by id.
func (s *Store) RemoveAlertRule(ctx context.Context, id int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM alert_rules WHERE id = ?`, id)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// RecordAlertFire writes an alert_fires row.
func (s *Store) RecordAlertFire(ctx context.Context, ruleID int64, ws, key, prev, newVal, sink, outcome, errMsg string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO alert_fires(rule_id, workspace_ref, key, prev_value, new_value, fired_at_unix, sink, outcome, error)
		VALUES(?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''))
	`, ruleID, ws, key, prev, newVal, Now(), sink, outcome, errMsg)
	return err
}

// AlertFire mirrors an alert_fires row.
type AlertFire struct {
	ID           int64   `json:"id"`
	RuleID       int64   `json:"rule_id"`
	WorkspaceRef string  `json:"workspace_ref,omitempty"`
	Key          string  `json:"key,omitempty"`
	PrevValue    string  `json:"prev_value,omitempty"`
	NewValue     string  `json:"new_value,omitempty"`
	FiredAtUnix  float64 `json:"fired_at_unix"`
	Sink         string  `json:"sink,omitempty"`
	Outcome      string  `json:"outcome,omitempty"`
	Error        string  `json:"error,omitempty"`
}

// ListAlertFires returns the most recent N fires.
func (s *Store) ListAlertFires(ctx context.Context, limit int) ([]AlertFire, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, rule_id, COALESCE(workspace_ref, ''), COALESCE(key, ''),
		       COALESCE(prev_value, ''), COALESCE(new_value, ''),
		       fired_at_unix, COALESCE(sink, ''), COALESCE(outcome, ''), COALESCE(error, '')
		FROM alert_fires ORDER BY fired_at_unix DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AlertFire, 0)
	for rows.Next() {
		var f AlertFire
		if err := rows.Scan(&f.ID, &f.RuleID, &f.WorkspaceRef, &f.Key, &f.PrevValue, &f.NewValue,
			&f.FiredAtUnix, &f.Sink, &f.Outcome, &f.Error); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// MarkNotificationSeen records a notification id as processed.
func (s *Store) MarkNotificationSeen(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO notification_seen(notification_id, seen_at_unix) VALUES(?, ?)`, id, Now())
	return err
}

// SeenNotifications returns the seen ids (used by watch to compute new ones).
func (s *Store) SeenNotifications(ctx context.Context) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT notification_id FROM notification_seen`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// PaneContentSample is a single sampled snippet of a surface's screen.
type PaneContentSample struct {
	WorkspaceRef  string  `json:"workspace_ref"`
	SurfaceRef    string  `json:"surface_ref"`
	Text          string  `json:"text"`
	SampledAtUnix float64 `json:"sampled_at_unix"`
}

// RecordPaneSample stores a sampled screen text in FTS-indexed pane_content_samples.
func (s *Store) RecordPaneSample(ctx context.Context, ws, surface, text string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO pane_content_samples(workspace_ref, surface_ref, text, sampled_at_unix) VALUES(?, ?, ?, ?)`,
		ws, surface, text, Now())
	return err
}

// SearchPaneContent runs a FTS5 MATCH query against pane content samples.
type PaneHit struct {
	WorkspaceRef  string  `json:"workspace_ref"`
	SurfaceRef    string  `json:"surface_ref"`
	Snippet       string  `json:"snippet"`
	SampledAtUnix float64 `json:"sampled_at_unix"`
}

func (s *Store) SearchPaneContent(ctx context.Context, q string, limit int) ([]PaneHit, error) {
	if strings.TrimSpace(q) == "" {
		return nil, fmt.Errorf("empty query")
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT pcs.workspace_ref, pcs.surface_ref,
		       snippet(pane_content_samples_fts, 0, '[', ']', '…', 12),
		       pcs.sampled_at_unix
		FROM pane_content_samples_fts
		JOIN pane_content_samples pcs ON pcs.id = pane_content_samples_fts.rowid
		WHERE pane_content_samples_fts MATCH ?
		ORDER BY pcs.sampled_at_unix DESC
		LIMIT ?
	`, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PaneHit, 0)
	for rows.Next() {
		var h PaneHit
		if err := rows.Scan(&h.WorkspaceRef, &h.SurfaceRef, &h.Snippet, &h.SampledAtUnix); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
