// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type KeywordSignalSnapshot struct {
	Keyword           string
	Source            string
	Country           string
	Score             float64
	Rating            string
	SearchSignal      float64
	CompetitionSignal float64
	DifficultySignal  float64
	TagCount          int
	TopListingCount   int
	CapturedAt        time.Time
}

type KeywordSignalSnapshotFilter struct {
	Keyword string
	Source  string
	Country string
	Since   time.Time
	Limit   int
}

func (s *Store) InsertKeywordSignalSnapshot(ctx context.Context, snapshot KeywordSignalSnapshot) error {
	if snapshot.CapturedAt.IsZero() {
		snapshot.CapturedAt = time.Now().UTC()
	} else {
		snapshot.CapturedAt = snapshot.CapturedAt.UTC()
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	_, err := s.db.ExecContext(ctx, `INSERT INTO keyword_signal_snapshots (
		keyword,
		source,
		country,
		score,
		rating,
		search_signal,
		competition_signal,
		difficulty_signal,
		tag_count,
		top_listing_count,
		captured_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		snapshot.Keyword,
		snapshot.Source,
		snapshot.Country,
		snapshot.Score,
		snapshot.Rating,
		snapshot.SearchSignal,
		snapshot.CompetitionSignal,
		snapshot.DifficultySignal,
		snapshot.TagCount,
		snapshot.TopListingCount,
		snapshot.CapturedAt.UnixNano(),
	)
	if err != nil {
		return fmt.Errorf("insert keyword signal snapshot: %w", err)
	}
	return nil
}

func (s *Store) ListKeywordSignalSnapshots(ctx context.Context, filter KeywordSignalSnapshotFilter) ([]KeywordSignalSnapshot, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}

	conditions := []string{"1 = 1"}
	args := make([]any, 0, 5)
	if filter.Keyword != "" {
		conditions = append(conditions, "keyword = ?")
		args = append(args, filter.Keyword)
	}
	if filter.Source != "" {
		conditions = append(conditions, "source = ?")
		args = append(args, filter.Source)
	}
	if filter.Country != "" {
		conditions = append(conditions, "country = ?")
		args = append(args, filter.Country)
	}
	if !filter.Since.IsZero() {
		conditions = append(conditions, "captured_at >= ?")
		args = append(args, filter.Since.UTC().UnixNano())
	}
	args = append(args, filter.Limit)

	rows, err := s.db.QueryContext(ctx, `SELECT
		keyword,
		source,
		country,
		score,
		rating,
		search_signal,
		competition_signal,
		difficulty_signal,
		tag_count,
		top_listing_count,
		captured_at
		FROM keyword_signal_snapshots
		WHERE `+strings.Join(conditions, " AND ")+`
		ORDER BY captured_at DESC
		LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("list keyword signal snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []KeywordSignalSnapshot
	for rows.Next() {
		var snapshot KeywordSignalSnapshot
		var capturedAt int64
		if err := rows.Scan(
			&snapshot.Keyword,
			&snapshot.Source,
			&snapshot.Country,
			&snapshot.Score,
			&snapshot.Rating,
			&snapshot.SearchSignal,
			&snapshot.CompetitionSignal,
			&snapshot.DifficultySignal,
			&snapshot.TagCount,
			&snapshot.TopListingCount,
			&capturedAt,
		); err != nil {
			return nil, fmt.Errorf("list keyword signal snapshots scan: %w", err)
		}
		snapshot.CapturedAt = time.Unix(0, capturedAt).UTC()
		snapshots = append(snapshots, snapshot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list keyword signal snapshots rows: %w", err)
	}
	return snapshots, nil
}
