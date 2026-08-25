// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored store extension for the `log` novel command (daily food diary
// with targets). Kept in a separate file with lazy CREATE TABLE IF NOT EXISTS
// so a generator regen does not disturb it and does not need to know about
// these tables.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// LogEntry is one logged food consumption record.
type LogEntry struct {
	ID       int64   `json:"id"`
	Date     string  `json:"date"` // YYYY-MM-DD
	FdcID    string  `json:"fdc_id"`
	Name     string  `json:"name"`
	Grams    float64 `json:"grams"`
	Calories float64 `json:"calories"`
	Protein  float64 `json:"protein_g"`
	Fat      float64 `json:"fat_g"`
	Carbs    float64 `json:"carbs_g"`
	Fiber    float64 `json:"fiber_g"`
}

// EnsureLogSchema creates the diary tables if they do not exist. Safe to call
// on every log command invocation.
func EnsureLogSchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS log_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date TEXT NOT NULL,
			fdc_id TEXT NOT NULL,
			name TEXT NOT NULL,
			grams REAL NOT NULL,
			calories REAL NOT NULL DEFAULT 0,
			protein_g REAL NOT NULL DEFAULT 0,
			fat_g REAL NOT NULL DEFAULT 0,
			carbs_g REAL NOT NULL DEFAULT 0,
			fiber_g REAL NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_log_entries_date ON log_entries(date)`,
		`CREATE TABLE IF NOT EXISTS log_targets (
			nutrient TEXT PRIMARY KEY,
			amount REAL NOT NULL
		)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("ensuring log schema: %w", err)
		}
	}
	return nil
}

// AddLogEntry inserts a diary entry and returns its id.
func (s *Store) AddLogEntry(ctx context.Context, e LogEntry) (int64, error) {
	if err := EnsureLogSchema(ctx, s.DB()); err != nil {
		return 0, err
	}
	if e.Date == "" {
		e.Date = time.Now().Format("2006-01-02")
	}
	res, err := s.DB().ExecContext(ctx,
		`INSERT INTO log_entries (date, fdc_id, name, grams, calories, protein_g, fat_g, carbs_g, fiber_g)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Date, e.FdcID, e.Name, e.Grams, e.Calories, e.Protein, e.Fat, e.Carbs, e.Fiber)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// LogEntriesForDate returns diary entries for a given YYYY-MM-DD date.
func (s *Store) LogEntriesForDate(ctx context.Context, date string) ([]LogEntry, error) {
	if err := EnsureLogSchema(ctx, s.DB()); err != nil {
		return nil, err
	}
	rows, err := s.DB().QueryContext(ctx,
		`SELECT id, date, fdc_id, name, grams, calories, protein_g, fat_g, carbs_g, fiber_g
		 FROM log_entries WHERE date = ? ORDER BY id`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]LogEntry, 0)
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.ID, &e.Date, &e.FdcID, &e.Name, &e.Grams, &e.Calories, &e.Protein, &e.Fat, &e.Carbs, &e.Fiber); err != nil {
			continue
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// LogEntriesSince returns diary entries on or after a start date (YYYY-MM-DD),
// ordered by date then id.
func (s *Store) LogEntriesSince(ctx context.Context, startDate string) ([]LogEntry, error) {
	if err := EnsureLogSchema(ctx, s.DB()); err != nil {
		return nil, err
	}
	rows, err := s.DB().QueryContext(ctx,
		`SELECT id, date, fdc_id, name, grams, calories, protein_g, fat_g, carbs_g, fiber_g
		 FROM log_entries WHERE date >= ? ORDER BY date, id`, startDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]LogEntry, 0)
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.ID, &e.Date, &e.FdcID, &e.Name, &e.Grams, &e.Calories, &e.Protein, &e.Fat, &e.Carbs, &e.Fiber); err != nil {
			continue
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// DeleteLogEntry removes a diary entry by id. Returns the number of rows removed.
func (s *Store) DeleteLogEntry(ctx context.Context, id int64) (int64, error) {
	if err := EnsureLogSchema(ctx, s.DB()); err != nil {
		return 0, err
	}
	res, err := s.DB().ExecContext(ctx, `DELETE FROM log_entries WHERE id = ?`, id)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// SetTarget upserts a daily target for a nutrient (e.g. "calories", "protein_g").
func (s *Store) SetTarget(ctx context.Context, nutrient string, amount float64) error {
	if err := EnsureLogSchema(ctx, s.DB()); err != nil {
		return err
	}
	_, err := s.DB().ExecContext(ctx,
		`INSERT INTO log_targets (nutrient, amount) VALUES (?, ?)
		 ON CONFLICT(nutrient) DO UPDATE SET amount = excluded.amount`, nutrient, amount)
	return err
}

// Targets returns all configured targets as a map.
func (s *Store) Targets(ctx context.Context) (map[string]float64, error) {
	if err := EnsureLogSchema(ctx, s.DB()); err != nil {
		return nil, err
	}
	rows, err := s.DB().QueryContext(ctx, `SELECT nutrient, amount FROM log_targets`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]float64{}
	for rows.Next() {
		var n string
		var a float64
		if err := rows.Scan(&n, &a); err != nil {
			continue
		}
		out[n] = a
	}
	return out, rows.Err()
}

// marshalLogEntries is a small helper so command code can emit entries as JSON
// without re-declaring the struct.
func marshalLogEntries(entries []LogEntry) (json.RawMessage, error) {
	return json.Marshal(entries)
}

var _ = marshalLogEntries
