// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// HAND-WRITTEN — not generated. Adds the per-food-entry table and the FTS5
// index that the generator's resource-scoped migrations don't cover.
//
// The generator emits a row-per-day `diary` table that stores each day as one
// JSON blob. Per-food queries — which are the whole point of this CLI — need
// row-per-food granularity. EnsureDiaryEntries adds:
//
//   - diary_entry: one row per (date, meal, food_name, position) with a
//     denormalized nutrient panel snapshotted at log time.
//   - diary_entries_fts: an FTS5 virtual table over food_name + meal so
//     `diary find` can answer "every time I logged X" instantly.
//   - foods_fts: an FTS5 virtual table over the existing food.id table's
//     description and brand columns.
//
// Idempotent — uses CREATE TABLE / CREATE VIRTUAL TABLE IF NOT EXISTS.
// Safe to call on every Open after migrate().

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// EnsureDiaryEntries creates the per-entry diary table and FTS5 indexes.
// Call after store.Open returns; it's safe across repeated invocations.
func (s *Store) EnsureDiaryEntries(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS diary_entry (
			date TEXT NOT NULL,
			meal TEXT NOT NULL,
			position INTEGER NOT NULL,
			food_name TEXT NOT NULL,
			calories REAL,
			carbohydrates REAL,
			fat REAL,
			protein REAL,
			sodium REAL,
			sugar REAL,
			fiber REAL,
			cholesterol REAL,
			extras_json TEXT,
			synced_at INTEGER NOT NULL,
			PRIMARY KEY (date, meal, position)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_diary_entry_date ON diary_entry(date)`,
		`CREATE INDEX IF NOT EXISTS idx_diary_entry_meal ON diary_entry(meal)`,
		`CREATE INDEX IF NOT EXISTS idx_diary_entry_food ON diary_entry(food_name)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS diary_entries_fts USING fts5(
			food_name, meal, content='diary_entry', content_rowid='rowid'
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS foods_fts USING fts5(
			description, brand, content='food', content_rowid='rowid'
		)`,
		`CREATE TABLE IF NOT EXISTS diary_day_meta (
			date TEXT PRIMARY KEY,
			complete INTEGER NOT NULL DEFAULT 0,
			totals_json TEXT,
			goals_json TEXT,
			synced_at INTEGER NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensuring diary schema: %w (stmt: %s)", err, oneLine(stmt))
		}
	}
	return nil
}

// DiaryEntryRow is the storage shape for a single food log entry. Mirrors the
// columns in diary_entry; extras_json holds nutrient fields that vary across
// days (potassium, calcium, iron, etc.) without bloating the schema.
type DiaryEntryRow struct {
	Date          string
	Meal          string
	Position      int
	FoodName      string
	Calories      float64
	Carbohydrates float64
	Fat           float64
	Protein       float64
	Sodium        float64
	Sugar         float64
	Fiber         float64
	Cholesterol   float64
	Extras        map[string]float64
}

// UpsertDiaryDay replaces all entries for the given date with the supplied
// rows. This matches MFP's edit semantics: the user can reorder, add, or
// delete entries within a day, so a per-day replace beats trying to diff.
//
// totalsJSON and goalsJSON may be nil; complete is recorded in diary_day_meta.
func (s *Store) UpsertDiaryDay(ctx context.Context, date string, rows []DiaryEntryRow, totalsJSON, goalsJSON json.RawMessage, complete bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM diary_entry WHERE date = ?`, date); err != nil {
		return fmt.Errorf("clearing diary_entry for %s: %w", date, err)
	}

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO diary_entry
		(date, meal, position, food_name, calories, carbohydrates, fat, protein, sodium, sugar, fiber, cholesterol, extras_json, synced_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for _, r := range rows {
		extrasBlob, _ := json.Marshal(r.Extras)
		if _, err := stmt.ExecContext(ctx,
			r.Date, r.Meal, r.Position, r.FoodName,
			r.Calories, r.Carbohydrates, r.Fat, r.Protein,
			r.Sodium, r.Sugar, r.Fiber, r.Cholesterol,
			string(extrasBlob), now,
		); err != nil {
			return fmt.Errorf("inserting diary_entry %s/%s/%d: %w", r.Date, r.Meal, r.Position, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO diary_day_meta (date, complete, totals_json, goals_json, synced_at)
		VALUES (?, ?, ?, ?, ?)`, date, boolToInt(complete), string(totalsJSON), string(goalsJSON), now); err != nil {
		return fmt.Errorf("upserting diary_day_meta %s: %w", date, err)
	}

	// Rebuild FTS for this date's entries. A delete-then-insert keeps the
	// virtual table in sync without external triggers.
	if _, err := tx.ExecContext(ctx, `DELETE FROM diary_entries_fts WHERE meal IN (SELECT meal FROM diary_entry WHERE date = ?)`, date); err != nil {
		return fmt.Errorf("clearing FTS for %s: %w", date, err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO diary_entries_fts (rowid, food_name, meal)
		SELECT rowid, food_name, meal FROM diary_entry WHERE date = ?`, date); err != nil {
		return fmt.Errorf("rebuilding FTS for %s: %w", date, err)
	}

	return tx.Commit()
}

// QueryDiaryEntries returns rows from diary_entry over the given inclusive
// date range. Results are ordered by date, meal position so callers can
// stream them into CSV/JSON without re-sorting.
func (s *Store) QueryDiaryEntries(ctx context.Context, fromDate, toDate string) ([]DiaryEntryRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT date, meal, position, food_name,
		calories, carbohydrates, fat, protein, sodium, sugar, fiber, cholesterol, extras_json
		FROM diary_entry WHERE date BETWEEN ? AND ?
		ORDER BY date, meal, position`, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DiaryEntryRow
	for rows.Next() {
		var r DiaryEntryRow
		var extrasJSON sql.NullString
		if err := rows.Scan(&r.Date, &r.Meal, &r.Position, &r.FoodName,
			&r.Calories, &r.Carbohydrates, &r.Fat, &r.Protein,
			&r.Sodium, &r.Sugar, &r.Fiber, &r.Cholesterol, &extrasJSON); err != nil {
			return nil, err
		}
		if extrasJSON.Valid && extrasJSON.String != "" && extrasJSON.String != "null" {
			r.Extras = map[string]float64{}
			_ = json.Unmarshal([]byte(extrasJSON.String), &r.Extras)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// FindDiaryEntries runs an FTS query against food_name + meal in
// diary_entries_fts. Returns rows with date+meal+nutrients for context.
func (s *Store) FindDiaryEntries(ctx context.Context, query string, fromDate, toDate string, limit int) ([]DiaryEntryRow, error) {
	if limit <= 0 {
		limit = 50
	}
	if fromDate == "" {
		fromDate = "0000-00-00"
	}
	if toDate == "" {
		toDate = "9999-12-31"
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT de.date, de.meal, de.position, de.food_name,
		       de.calories, de.carbohydrates, de.fat, de.protein,
		       de.sodium, de.sugar, de.fiber, de.cholesterol, de.extras_json
		FROM diary_entries_fts fts
		JOIN diary_entry de ON de.rowid = fts.rowid
		WHERE diary_entries_fts MATCH ? AND de.date BETWEEN ? AND ?
		ORDER BY de.date DESC, de.meal, de.position
		LIMIT ?`, query, fromDate, toDate, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DiaryEntryRow
	for rows.Next() {
		var r DiaryEntryRow
		var extrasJSON sql.NullString
		if err := rows.Scan(&r.Date, &r.Meal, &r.Position, &r.FoodName,
			&r.Calories, &r.Carbohydrates, &r.Fat, &r.Protein,
			&r.Sodium, &r.Sugar, &r.Fiber, &r.Cholesterol, &extrasJSON); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DiaryDayMeta describes the day-level metadata persisted alongside per-food
// entries — daily totals, daily goals, and completion state.
type DiaryDayMeta struct {
	Date       string
	Complete   bool
	TotalsJSON json.RawMessage
	GoalsJSON  json.RawMessage
	SyncedAt   time.Time
}

// QueryDiaryDayMeta returns per-day metadata for an inclusive date range.
func (s *Store) QueryDiaryDayMeta(ctx context.Context, fromDate, toDate string) ([]DiaryDayMeta, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT date, complete, totals_json, goals_json, synced_at
		FROM diary_day_meta WHERE date BETWEEN ? AND ? ORDER BY date`, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DiaryDayMeta
	for rows.Next() {
		var m DiaryDayMeta
		var complete int
		var totals, goals sql.NullString
		var ts int64
		if err := rows.Scan(&m.Date, &complete, &totals, &goals, &ts); err != nil {
			return nil, err
		}
		m.Complete = complete != 0
		if totals.Valid {
			m.TotalsJSON = json.RawMessage(totals.String)
		}
		if goals.Valid {
			m.GoalsJSON = json.RawMessage(goals.String)
		}
		m.SyncedAt = time.Unix(ts, 0)
		out = append(out, m)
	}
	return out, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func oneLine(s string) string {
	parts := strings.Fields(s)
	out := strings.Join(parts, " ")
	if len(out) > 80 {
		return out[:77] + "..."
	}
	return out
}
