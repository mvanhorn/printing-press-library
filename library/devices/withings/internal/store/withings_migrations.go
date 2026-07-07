// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored Withings-specific store extensions (no "DO NOT EDIT" header).
// Holds lazily-created auxiliary tables that the analytics commands own —
// currently the `bp_notes` annotation table used by `bp-report`. These are
// created on demand (CREATE TABLE IF NOT EXISTS) rather than in the generated
// migrate() path so they survive a `generate --force` regen as a whole unit.

package store

import (
	"database/sql"
	"fmt"
	"strings"
)

// EnsureBPNotesTable creates the bp_notes(date, note) annotation table if it
// does not already exist. Idempotent; safe to call on every bp-report run.
// Requires a read-write store handle (opened via Open / OpenWithContext).
func (s *Store) EnsureBPNotesTable() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS bp_notes (
		date TEXT PRIMARY KEY,
		note TEXT NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("creating bp_notes table: %w", err)
	}
	return nil
}

// UpsertBPNote stores (or replaces) a free-text annotation for a given
// YYYY-MM-DD date. An empty note string deletes any existing note for that
// date so callers can clear an annotation by passing DATE=.
func (s *Store) UpsertBPNote(date, note string) error {
	date = strings.TrimSpace(date)
	if date == "" {
		return fmt.Errorf("bp note requires a non-empty date")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if strings.TrimSpace(note) == "" {
		_, err := s.db.Exec(`DELETE FROM bp_notes WHERE date = ?`, date)
		return err
	}
	_, err := s.db.Exec(
		`INSERT INTO bp_notes (date, note) VALUES (?, ?)
		 ON CONFLICT(date) DO UPDATE SET note = excluded.note`,
		date, note,
	)
	if err != nil {
		return fmt.Errorf("upserting bp note for %s: %w", date, err)
	}
	return nil
}

// BPNotes returns all stored annotations keyed by YYYY-MM-DD date. Returns an
// empty (non-nil) map when the table is absent or empty so callers can index it
// without a nil check.
func (s *Store) BPNotes() (map[string]string, error) {
	out := map[string]string{}
	// Tolerate the table not existing yet (read-only callers that never
	// created it): treat "no such table" as an empty annotation set.
	rows, err := s.db.Query(`SELECT date, note FROM bp_notes`)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return out, nil
		}
		return nil, fmt.Errorf("querying bp_notes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var date string
		var note sql.NullString
		if err := rows.Scan(&date, &note); err != nil {
			return nil, fmt.Errorf("scanning bp_notes row: %w", err)
		}
		if note.Valid {
			out[date] = note.String
		}
	}
	return out, rows.Err()
}
