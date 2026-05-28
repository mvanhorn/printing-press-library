// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

// art-goat daily-pick cache. Keyed by (pick_date, mode) so distinct
// rotation policies on the same day persist independently. Pure CRUD;
// schema lives in works.go alongside the other art-goat tables.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// TodayPick is one row of the today_picks cache.
type TodayPick struct {
	Date     string // YYYY-MM-DD in the writer's local time
	Mode     string // "" for default rotation, "bridge-from-last", etc.
	WorkID   string
	Why      string
	Prompt   string
	ChosenAt time.Time
}

// GetTodayPick returns the cached pick for (date, mode), or nil if no
// row exists. Callers pass the date string already formatted as
// YYYY-MM-DD so the cache key matches what SaveTodayPick wrote — no
// format-coupling in either direction.
func (s *Store) GetTodayPick(ctx context.Context, date, mode string) (*TodayPick, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT pick_date, mode, work_id, why, prompt, chosen_at
FROM today_picks WHERE pick_date = ? AND mode = ?`, date, mode)
	p := &TodayPick{}
	err := row.Scan(&p.Date, &p.Mode, &p.WorkID, &p.Why, &p.Prompt, &p.ChosenAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get today pick: %w", err)
	}
	return p, nil
}

// SaveTodayPick upserts the cached pick for (date, mode). Subsequent
// reads return this work + why + prompt verbatim until ClearTodayPick
// fires or the calendar date rolls over.
func (s *Store) SaveTodayPick(ctx context.Context, p TodayPick) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO today_picks (pick_date, mode, work_id, why, prompt, chosen_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(pick_date, mode) DO UPDATE SET
    work_id = excluded.work_id,
    why = excluded.why,
    prompt = excluded.prompt,
    chosen_at = excluded.chosen_at`,
		p.Date, p.Mode, p.WorkID, p.Why, p.Prompt, p.ChosenAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("save today pick: %w", err)
	}
	return nil
}

// ClearTodayPick removes the cached pick for (date, mode). Used by
// `today --reroll` to drop the existing pick before re-rolling.
// Returns the number of rows deleted so callers can distinguish a real
// re-roll (1) from a re-roll over an already-empty cache (0).
func (s *Store) ClearTodayPick(ctx context.Context, date, mode string) (int64, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.ExecContext(ctx, `
DELETE FROM today_picks WHERE pick_date = ? AND mode = ?`, date, mode)
	if err != nil {
		return 0, fmt.Errorf("clear today pick: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
