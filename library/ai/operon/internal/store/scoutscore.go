// Copyright 2026 yaooooooooooooooo. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel store — not generated.

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// InsertScoutScore appends a trust-score observation. Score may be nil
// (for advertisers that have not yet accrued behavioral signal).
func (s *Store) InsertScoutScore(ctx context.Context, advertiserID string, score *float64) error {
	if advertiserID == "" {
		return errors.New("store: advertiser id required")
	}
	now := time.Now().UnixMilli()
	var arg any
	if score != nil {
		arg = *score
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO scoutscore_history (advertiser_id, trust_score, observed_at)
		VALUES (?, ?, ?)`, advertiserID, arg, now)
	if err != nil {
		return fmt.Errorf("store: insert scoutscore: %w", err)
	}
	return nil
}

// GetTrustHistory returns every score point for advertiserID, ordered by
// observed_at ascending so the caller can render a time series directly.
func (s *Store) GetTrustHistory(ctx context.Context, advertiserID string) ([]ScoutScorePoint, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT advertiser_id, trust_score, observed_at
		  FROM scoutscore_history
		 WHERE advertiser_id = ?
		 ORDER BY observed_at ASC`, advertiserID)
	if err != nil {
		return nil, fmt.Errorf("store: trust history: %w", err)
	}
	defer rows.Close()
	var out []ScoutScorePoint
	for rows.Next() {
		var (
			p     ScoutScorePoint
			score sql.NullFloat64
		)
		if err := rows.Scan(&p.AdvertiserID, &score, &p.ObservedAt); err != nil {
			return nil, err
		}
		if score.Valid {
			v := score.Float64
			p.TrustScore = &v
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
