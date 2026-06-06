// Copyright 2026 David Bryson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored extended schema for the `drift` command: a lightweight
// rating/ranking snapshot history keyed by location_id. Lazily migrated so it
// imposes no cost on CLIs that never call drift. Preserved across regen.

package store

import (
	"context"
	"database/sql"
	"time"
)

// RatingSnapshot is one point-in-time capture of a location's headline metrics.
type RatingSnapshot struct {
	LocationID string  `json:"location_id"`
	Name       string  `json:"name"`
	Rating     float64 `json:"rating"`
	NumReviews int     `json:"num_reviews"`
	Ranking    int     `json:"ranking"`
	CapturedAt string  `json:"captured_at"`
}

func (s *Store) ensureRatingSnapshots(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS rating_snapshots (
    location_id TEXT NOT NULL,
    name        TEXT,
    rating      REAL,
    num_reviews INTEGER,
    ranking     INTEGER,
    captured_at TEXT NOT NULL
)`)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
CREATE INDEX IF NOT EXISTS idx_rating_snapshots_loc
    ON rating_snapshots(location_id, captured_at)`)
	return err
}

// RecordRatingSnapshot appends a snapshot for a location. captured_at is set to
// now (UTC, RFC3339) when empty.
func (s *Store) RecordRatingSnapshot(ctx context.Context, snap RatingSnapshot) error {
	if err := s.ensureRatingSnapshots(ctx); err != nil {
		return err
	}
	if snap.CapturedAt == "" {
		snap.CapturedAt = time.Now().UTC().Format(time.RFC3339)
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO rating_snapshots(location_id, name, rating, num_reviews, ranking, captured_at)
VALUES(?, ?, ?, ?, ?, ?)`,
		snap.LocationID, snap.Name, snap.Rating, snap.NumReviews, snap.Ranking, snap.CapturedAt)
	return err
}

// LastRatingSnapshot returns the most recent snapshot for a location strictly
// before `before` (pass the empty string for no upper bound). The bool is false
// when no prior snapshot exists.
func (s *Store) LastRatingSnapshot(ctx context.Context, locationID, before string) (RatingSnapshot, bool, error) {
	if err := s.ensureRatingSnapshots(ctx); err != nil {
		return RatingSnapshot{}, false, err
	}
	query := `
SELECT location_id, name, rating, num_reviews, ranking, captured_at
FROM rating_snapshots
WHERE location_id = ?`
	args := []any{locationID}
	if before != "" {
		query += " AND captured_at < ?"
		args = append(args, before)
	}
	query += " ORDER BY captured_at DESC LIMIT 1"

	row := s.db.QueryRowContext(ctx, query, args...)
	var (
		snap    RatingSnapshot
		name    sql.NullString
		rating  sql.NullFloat64
		reviews sql.NullInt64
		ranking sql.NullInt64
		capAt   sql.NullString
	)
	err := row.Scan(&snap.LocationID, &name, &rating, &reviews, &ranking, &capAt)
	if err == sql.ErrNoRows {
		return RatingSnapshot{}, false, nil
	}
	if err != nil {
		return RatingSnapshot{}, false, err
	}
	snap.Name = name.String
	snap.Rating = rating.Float64
	snap.NumReviews = int(reviews.Int64)
	snap.Ranking = int(ranking.Int64)
	snap.CapturedAt = capAt.String
	return snap, true, nil
}
