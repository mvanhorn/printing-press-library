// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ResearchSessionMember is one bound artifact in a stitched research session.
type ResearchSessionMember struct {
	Kind  string `json:"kind"`
	RefID string `json:"ref_id"`
}

// BalanceSnapshot is a point-in-time Account balance capture.
type BalanceSnapshot struct {
	ID                       int64
	CapturedAt               time.Time
	OrgID                    string
	CreditBalanceCents       float64
	PendingDebitBalanceCents float64
	WillInvoice              bool
	RawJSON                  string
}

// TaskInteractionLink is one hop in a task run lineage chain.
type TaskInteractionLink struct {
	RunID                 string `json:"run_id"`
	PreviousInteractionID string `json:"previous_interaction_id"`
}

// UpsertResearchSession creates or updates a research session and its members.
func (s *Store) UpsertResearchSession(sessionID, createdAt, notes string, members []ResearchSessionMember) error {
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("session id is required")
	}
	if createdAt == "" {
		createdAt = time.Now().UTC().Format(time.RFC3339)
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(
		`INSERT INTO research_sessions (id, created_at, notes)
		 VALUES (?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET notes = COALESCE(excluded.notes, research_sessions.notes)`,
		sessionID, createdAt, nullIfEmpty(notes),
	); err != nil {
		return fmt.Errorf("upsert research session: %w", err)
	}

	for _, m := range members {
		if strings.TrimSpace(m.Kind) == "" || strings.TrimSpace(m.RefID) == "" {
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO research_session_members (session_id, kind, ref_id, created_at)
			 VALUES (?, ?, ?, ?)
			 ON CONFLICT(session_id, kind, ref_id) DO NOTHING`,
			sessionID, m.Kind, m.RefID, createdAt,
		); err != nil {
			return fmt.Errorf("upsert research session member: %w", err)
		}
	}

	return tx.Commit()
}

// ListResearchSessionMembers returns all members for a session.
func (s *Store) ListResearchSessionMembers(sessionID string) ([]ResearchSessionMember, error) {
	rows, err := s.db.Query(
		`SELECT kind, ref_id FROM research_session_members
		 WHERE session_id = ? ORDER BY created_at, kind, ref_id`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []ResearchSessionMember
	for rows.Next() {
		var m ResearchSessionMember
		if err := rows.Scan(&m.Kind, &m.RefID); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// GetResearchSessionCreatedAt returns the session created_at timestamp.
func (s *Store) GetResearchSessionCreatedAt(sessionID string) (string, error) {
	var createdAt string
	err := s.db.QueryRow(`SELECT created_at FROM research_sessions WHERE id = ?`, sessionID).Scan(&createdAt)
	if err != nil {
		return "", err
	}
	return createdAt, nil
}

// InsertBalanceSnapshot records a live balance fetch.
func (s *Store) InsertBalanceSnapshot(capturedAt time.Time, orgID string, credit, pendingDebit float64, willInvoice bool, raw json.RawMessage) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	will := 0
	if willInvoice {
		will = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO balance_snapshots
		 (captured_at, org_id, credit_balance_cents, pending_debit_balance_cents, will_invoice, raw_json)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		capturedAt.UTC().Format(time.RFC3339),
		nullIfEmpty(orgID),
		credit,
		pendingDebit,
		will,
		string(raw),
	)
	return err
}

// BalanceSnapshotWindow returns the oldest and newest snapshots since cutoff.
func (s *Store) BalanceSnapshotWindow(since time.Time) (oldest, newest *BalanceSnapshot, err error) {
	cutoff := since.UTC().Format(time.RFC3339)
	oldest, err = s.queryBalanceSnapshot(
		`SELECT id, captured_at, org_id, credit_balance_cents, pending_debit_balance_cents, will_invoice, raw_json
		 FROM balance_snapshots WHERE captured_at >= ? ORDER BY captured_at ASC LIMIT 1`,
		cutoff,
	)
	if err != nil {
		return nil, nil, err
	}
	newest, err = s.queryBalanceSnapshot(
		`SELECT id, captured_at, org_id, credit_balance_cents, pending_debit_balance_cents, will_invoice, raw_json
		 FROM balance_snapshots WHERE captured_at >= ? ORDER BY captured_at DESC LIMIT 1`,
		cutoff,
	)
	if err != nil {
		return oldest, nil, err
	}
	return oldest, newest, nil
}

func (s *Store) queryBalanceSnapshot(query string, args ...any) (*BalanceSnapshot, error) {
	var snap BalanceSnapshot
	var capturedAt string
	var will sql.NullInt64
	err := s.db.QueryRow(query, args...).Scan(
		&snap.ID,
		&capturedAt,
		&snap.OrgID,
		&snap.CreditBalanceCents,
		&snap.PendingDebitBalanceCents,
		&will,
		&snap.RawJSON,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if t, parseErr := time.Parse(time.RFC3339, capturedAt); parseErr == nil {
		snap.CapturedAt = t
	}
	snap.WillInvoice = will.Valid && will.Int64 != 0
	return &snap, nil
}

// CountResourcesCreatedSince counts rows in resources synced within the window.
func (s *Store) CountResourcesCreatedSince(since time.Time, resourceTypes []string) (int64, error) {
	cutoff := since.UTC().Format(time.RFC3339)
	if len(resourceTypes) == 0 {
		resourceTypes = []string{"tasks", "findall"}
	}
	placeholders := make([]string, len(resourceTypes))
	args := make([]any, 0, len(resourceTypes)+1)
	for i, rt := range resourceTypes {
		placeholders[i] = "?"
		args = append(args, rt)
	}
	args = append(args, cutoff)
	// Placeholders are fixed "?" tokens only; resource type values are bound via args.
	var b strings.Builder
	b.WriteString(`SELECT COUNT(*) FROM resources WHERE resource_type IN (`)
	b.WriteString(strings.Join(placeholders, ","))
	b.WriteString(`) AND synced_at >= ?`)
	var count int64
	err := s.db.QueryRow(b.String(), args...).Scan(&count)
	return count, err
}

// UpsertTaskInteraction stores lineage metadata for a task run.
func (s *Store) UpsertTaskInteraction(runID, previousInteractionID string, raw json.RawMessage) error {
	if strings.TrimSpace(runID) == "" {
		return fmt.Errorf("run id is required")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO task_interactions (run_id, previous_interaction_id, created_at, raw_json)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(run_id) DO UPDATE SET
		   previous_interaction_id = excluded.previous_interaction_id,
		   created_at = excluded.created_at,
		   raw_json = excluded.raw_json`,
		runID,
		nullIfEmpty(previousInteractionID),
		time.Now().UTC().Format(time.RFC3339),
		string(raw),
	)
	return err
}

// GetTaskInteractionPrevious returns previous_interaction_id for a run, if stored.
func (s *Store) GetTaskInteractionPrevious(runID string) (string, bool, error) {
	var prev sql.NullString
	err := s.db.QueryRow(
		`SELECT previous_interaction_id FROM task_interactions WHERE run_id = ?`,
		runID,
	).Scan(&prev)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if !prev.Valid || strings.TrimSpace(prev.String) == "" {
		return "", false, nil
	}
	return prev.String, true, nil
}

// WalkTaskLineage follows previous_interaction_id links starting at runID.
func (s *Store) WalkTaskLineage(startRunID string) ([]TaskInteractionLink, error) {
	visited := make(map[string]bool)
	var chain []TaskInteractionLink

	runID := strings.TrimSpace(startRunID)
	if runID == "" {
		return nil, nil
	}

	prev, found, err := s.lookupPreviousInteraction(runID)
	if err != nil {
		return nil, err
	}
	if !found {
		// Unknown run — empty chain so callers can surface not-found.
		return nil, nil
	}

	for runID != "" && !visited[runID] {
		visited[runID] = true
		link := TaskInteractionLink{RunID: runID, PreviousInteractionID: prev}
		chain = append(chain, link)
		if strings.TrimSpace(prev) == "" {
			break
		}
		runID, err = s.resolveRunIDForInteraction(prev)
		if err != nil {
			return chain, err
		}
		if runID == "" {
			break
		}
		prev, found, err = s.lookupPreviousInteraction(runID)
		if err != nil {
			return chain, err
		}
		if !found {
			chain = append(chain, TaskInteractionLink{RunID: runID})
			break
		}
	}
	return chain, nil
}

func (s *Store) lookupPreviousInteraction(runID string) (string, bool, error) {
	prev, ok, err := s.GetTaskInteractionPrevious(runID)
	if err != nil || ok {
		return prev, ok, err
	}
	raw, err := s.Get("tasks", runID)
	if err != nil {
		if err == sql.ErrNoRows {
			raw, err = s.lookupTaskByRunID(runID)
		}
		if err != nil {
			if err == sql.ErrNoRows {
				return "", false, nil
			}
			return "", false, err
		}
	}
	var obj map[string]any
	if json.Unmarshal(raw, &obj) != nil {
		return "", false, nil
	}
	if v, ok := obj["previous_interaction_id"].(string); ok && strings.TrimSpace(v) != "" {
		return v, true, nil
	}
	return "", false, nil
}

func (s *Store) lookupTaskByRunID(runID string) (json.RawMessage, error) {
	var data string
	err := s.db.QueryRow(`SELECT data FROM tasks WHERE run_id = ? LIMIT 1`, runID).Scan(&data)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func (s *Store) resolveRunIDForInteraction(interactionID string) (string, error) {
	var runID sql.NullString
	err := s.db.QueryRow(
		`SELECT run_id FROM tasks WHERE interaction_id = ? LIMIT 1`,
		interactionID,
	).Scan(&runID)
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}
	if runID.Valid && strings.TrimSpace(runID.String) != "" {
		return runID.String, nil
	}
	err = s.db.QueryRow(
		`SELECT run_id FROM task_interactions WHERE previous_interaction_id = ? LIMIT 1`,
		interactionID,
	).Scan(&runID)
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}
	if runID.Valid {
		return runID.String, nil
	}
	return "", nil
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
