// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// PlaybookRow is one row in the learning_playbooks table. Either
// PlaybookJSON or NotesText (or both) carries content; an all-empty
// row is rejected at upsert time.
type PlaybookRow struct {
	ID             int64      `json:"id"`
	QueryFamily    string     `json:"query_family"`
	PlaybookJSON   string     `json:"playbook_json,omitempty"`
	NotesText      string     `json:"notes_text,omitempty"`
	Source         string     `json:"source"`
	Confidence     int        `json:"confidence"`
	CreatedAt      time.Time  `json:"created_at"`
	LastObservedAt *time.Time `json:"last_observed_at,omitempty"`
}

// UpsertPlaybookInput is the call-shape UpsertPlaybook accepts.
// QueryFamily is required; at least one of PlaybookJSON / NotesText
// must be non-empty.
//
// PreserveExistingNotes is a seed-loop affordance: when set, the
// UPDATE branch only fills notes_text if the stored value is empty.
// This protects `playbook amend` notes (which share the seed's family
// keys) across binary upgrades that bump SeedVersion and re-run the
// install path. Insert paths ignore the flag — a brand-new row gets
// the supplied notes unconditionally.
type UpsertPlaybookInput struct {
	QueryFamily           string
	PlaybookJSON          string
	NotesText             string
	Source                string
	PreserveExistingNotes bool
}

// UpsertPlaybook inserts a playbook row for a query family or, on
// conflict, refreshes content + bumps last_observed_at. Source on the
// existing row is preserved across re-teaches; only the content +
// timestamp update. Returns the row id and a bool indicating insert
// vs. update.
func (s *Store) UpsertPlaybook(in UpsertPlaybookInput) (int64, bool, error) {
	family := strings.TrimSpace(in.QueryFamily)
	if family == "" {
		return 0, false, fmt.Errorf("upsert playbook: query_family is required")
	}
	if strings.TrimSpace(in.PlaybookJSON) == "" && strings.TrimSpace(in.NotesText) == "" {
		return 0, false, fmt.Errorf("upsert playbook: at least one of playbook_json or notes_text must be non-empty")
	}
	source := in.Source
	if source == "" {
		source = LearningSourceTaught
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	now := time.Now().UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback()

	var existingID int64
	err = tx.QueryRow(
		`SELECT id FROM learning_playbooks WHERE query_family = ?`,
		family,
	).Scan(&existingID)
	if err == nil {
		// Partial-update semantics: an empty PlaybookJSON or NotesText
		// from the caller means "leave this column alone", not "wipe
		// it". This lets `teach-playbook --notes "..."` and
		// `playbook amend --add-note ...` append guidance without
		// destroying the stored choreography, and lets a playbook-only
		// re-teach preserve existing notes. PreserveExistingNotes
		// additionally protects existing non-empty notes_text from
		// being overwritten by a seed-loop upsert.
		var notesUpdate string
		notesArgs := []any{in.NotesText, in.NotesText}
		if in.PreserveExistingNotes {
			notesUpdate = `notes_text = CASE WHEN ? != '' AND (notes_text IS NULL OR notes_text = '') THEN ? ELSE notes_text END`
		} else {
			notesUpdate = `notes_text = CASE WHEN ? != '' THEN ? ELSE notes_text END`
		}
		query := `UPDATE learning_playbooks
			 SET playbook_json = CASE WHEN ? != '' THEN ? ELSE playbook_json END,
			     ` + notesUpdate + `,
			     last_observed_at = ?
			 WHERE id = ?`
		args := []any{in.PlaybookJSON, in.PlaybookJSON}
		args = append(args, notesArgs...)
		args = append(args, now, existingID)
		if _, err := tx.Exec(query, args...); err != nil {
			return 0, false, fmt.Errorf("upsert playbook update: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return 0, false, err
		}
		return existingID, false, nil
	}
	if err != sql.ErrNoRows {
		return 0, false, fmt.Errorf("upsert playbook lookup: %w", err)
	}

	res, err := tx.Exec(
		`INSERT INTO learning_playbooks
		 (query_family, playbook_json, notes_text, source, confidence, last_observed_at)
		 VALUES (?, ?, ?, ?, 2, ?)`,
		family, in.PlaybookJSON, in.NotesText, source, now,
	)
	if err != nil {
		return 0, false, fmt.Errorf("upsert playbook insert: %w", err)
	}
	id, _ := res.LastInsertId()
	if err := tx.Commit(); err != nil {
		return 0, false, err
	}
	return id, true, nil
}

// GetPlaybookByFamily returns the row for a given family, or
// (PlaybookRow{}, false, nil) when nothing matches. The bool
// distinguishes "no row" from "lookup error".
func (s *Store) GetPlaybookByFamily(family string) (PlaybookRow, bool, error) {
	family = strings.TrimSpace(family)
	if family == "" {
		return PlaybookRow{}, false, nil
	}
	var row PlaybookRow
	var playbook, notes sql.NullString
	var lastObserved sql.NullTime
	err := s.db.QueryRow(
		`SELECT id, query_family, COALESCE(playbook_json, ''), COALESCE(notes_text, ''),
			source, confidence, created_at, last_observed_at
		 FROM learning_playbooks WHERE query_family = ?`,
		family,
	).Scan(&row.ID, &row.QueryFamily, &playbook, &notes, &row.Source, &row.Confidence, &row.CreatedAt, &lastObserved)
	if err == sql.ErrNoRows {
		return PlaybookRow{}, false, nil
	}
	if err != nil {
		return PlaybookRow{}, false, fmt.Errorf("get playbook: %w", err)
	}
	row.PlaybookJSON = playbook.String
	row.NotesText = notes.String
	if lastObserved.Valid {
		t := lastObserved.Time
		row.LastObservedAt = &t
	}
	return row, true, nil
}

// ListPlaybooks returns all rows ordered by last_observed_at DESC.
// Useful for `playbook list` inspection.
func (s *Store) ListPlaybooks() ([]PlaybookRow, error) {
	rows, err := s.db.Query(
		`SELECT id, query_family, COALESCE(playbook_json, ''), COALESCE(notes_text, ''),
			source, confidence, created_at, last_observed_at
		 FROM learning_playbooks
		 ORDER BY COALESCE(last_observed_at, created_at) DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list playbooks: %w", err)
	}
	defer rows.Close()
	var out []PlaybookRow
	for rows.Next() {
		var row PlaybookRow
		var playbook, notes sql.NullString
		var lastObserved sql.NullTime
		if err := rows.Scan(&row.ID, &row.QueryFamily, &playbook, &notes, &row.Source, &row.Confidence, &row.CreatedAt, &lastObserved); err != nil {
			return nil, fmt.Errorf("scan playbook: %w", err)
		}
		row.PlaybookJSON = playbook.String
		row.NotesText = notes.String
		if lastObserved.Valid {
			t := lastObserved.Time
			row.LastObservedAt = &t
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
