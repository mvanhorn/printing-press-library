// Copyright 2026 Isaac Marks and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Clarify side tables: stage history for velocity, transcript
// cache for prep. Lazy-initialized by the commands that need them so the
// generated migration slice in store.go stays untouched.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// EnsureClarifySideTables creates the Clarify-specific side tables if they do
// not exist. Safe to call on every command run.
func (s *Store) EnsureClarifySideTables(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS clarify_stage_history (
			deal_id TEXT NOT NULL,
			stage TEXT NOT NULL,
			observed_at DATETIME NOT NULL,
			PRIMARY KEY (deal_id, stage, observed_at)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_clarify_stage_history_deal ON clarify_stage_history(deal_id, observed_at)`,
		`CREATE TABLE IF NOT EXISTS clarify_transcripts (
			meeting_id TEXT PRIMARY KEY,
			content TEXT NOT NULL,
			fetched_at DATETIME NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensuring clarify side tables: %w", err)
		}
	}
	return nil
}

// DealStageObservation is one deal's current pipeline stage at observation
// time, fed to RecordStageObservations by the analytics commands.
type DealStageObservation struct {
	DealID string
	Stage  string
}

// RecordStageObservations appends a history row for every deal whose current
// stage differs from its most recently recorded stage. History accrues each
// time an analytics command runs, which is what velocity's dwell-time and
// conversion computations read.
func (s *Store) RecordStageObservations(ctx context.Context, observations []DealStageObservation, now time.Time) (int, error) {
	if len(observations) == 0 {
		return 0, nil
	}
	latest := map[string]string{}
	rows, err := s.db.QueryContext(ctx, `
		SELECT h.deal_id, h.stage FROM clarify_stage_history h
		JOIN (
			SELECT deal_id, MAX(observed_at) AS mo FROM clarify_stage_history GROUP BY deal_id
		) m ON m.deal_id = h.deal_id AND m.mo = h.observed_at`)
	if err != nil {
		return 0, fmt.Errorf("reading stage history: %w", err)
	}
	for rows.Next() {
		var id, stage string
		if err := rows.Scan(&id, &stage); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("scanning stage history: %w", err)
		}
		latest[id] = stage
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, fmt.Errorf("iterating stage history: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("closing stage history rows: %w", err)
	}

	s.lockForWrite()
	defer s.unlockAfterWrite()
	inserted := 0
	ts := now.UTC().Format(time.RFC3339)
	for _, o := range observations {
		if o.DealID == "" || o.Stage == "" {
			continue
		}
		if latest[o.DealID] == o.Stage {
			continue
		}
		if _, err := s.db.ExecContext(ctx,
			`INSERT OR IGNORE INTO clarify_stage_history (deal_id, stage, observed_at) VALUES (?, ?, ?)`,
			o.DealID, o.Stage, ts); err != nil {
			return inserted, fmt.Errorf("recording stage observation for deal %s: %w", o.DealID, err)
		}
		latest[o.DealID] = o.Stage
		inserted++
	}
	return inserted, nil
}

// StageTransition is one observed stage change for a deal, ordered by time.
type StageTransition struct {
	DealID     string
	Stage      string
	ObservedAt time.Time
}

// StageHistory returns the full observed stage history ordered by deal then
// time, plus a count of rows whose timestamp failed to parse (those rows are
// excluded so velocity's transition math never runs on zero times).
func (s *Store) StageHistory(ctx context.Context) ([]StageTransition, int, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT deal_id, stage, observed_at FROM clarify_stage_history ORDER BY deal_id, observed_at`)
	if err != nil {
		return nil, 0, fmt.Errorf("reading stage history: %w", err)
	}
	out := make([]StageTransition, 0)
	badRows := 0
	for rows.Next() {
		var t StageTransition
		var ts string
		if err := rows.Scan(&t.DealID, &t.Stage, &ts); err != nil {
			_ = rows.Close()
			return nil, badRows, fmt.Errorf("scanning stage transition: %w", err)
		}
		parsed, perr := time.Parse(time.RFC3339, ts)
		if perr != nil {
			badRows++
			continue
		}
		t.ObservedAt = parsed
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, badRows, fmt.Errorf("iterating stage transitions: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, badRows, fmt.Errorf("closing stage transition rows: %w", err)
	}
	return out, badRows, nil
}

// CachedTranscript returns the cached transcript for a meeting, if present.
func (s *Store) CachedTranscript(ctx context.Context, meetingID string) (string, bool, error) {
	var content string
	err := s.db.QueryRowContext(ctx,
		`SELECT content FROM clarify_transcripts WHERE meeting_id = ?`, meetingID).Scan(&content)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("reading cached transcript: %w", err)
	}
	return content, true, nil
}

// CacheTranscript stores a fetched transcript for reuse by later prep runs.
func (s *Store) CacheTranscript(ctx context.Context, meetingID, content string, now time.Time) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO clarify_transcripts (meeting_id, content, fetched_at) VALUES (?, ?, ?)
		 ON CONFLICT(meeting_id) DO UPDATE SET content = excluded.content, fetched_at = excluded.fetched_at`,
		meetingID, content, now.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("caching transcript for meeting %s: %w", meetingID, err)
	}
	return nil
}
