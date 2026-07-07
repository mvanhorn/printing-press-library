// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// Custom store tables for the Scrape Creators novel commands. These tables sit
// alongside the generated generic resources table and are created lazily so a
// command works the first time it runs without a separate migration step. They
// are intentionally NOT added to the generated migration slice in store.go.

package store

import (
	"context"
	"database/sql"
	"time"
)

// CreatorSnapshot is one follower-count reading captured by `creator track`.
type CreatorSnapshot struct {
	Handle        string    `json:"handle"`
	Platform      string    `json:"platform"`
	FollowerCount int64     `json:"follower_count"`
	CapturedAt    time.Time `json:"captured_at"`
}

// EnsureCreatorSnapshots lazily creates the creator_snapshots table.
func EnsureCreatorSnapshots(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS creator_snapshots (
			handle         TEXT    NOT NULL,
			platform       TEXT    NOT NULL,
			follower_count INTEGER NOT NULL,
			captured_at    TEXT    NOT NULL
		)`)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_creator_snapshots_lookup
			ON creator_snapshots(handle, platform, captured_at)`)
	return err
}

// InsertCreatorSnapshot appends one follower reading.
func InsertCreatorSnapshot(ctx context.Context, db *sql.DB, s CreatorSnapshot) error {
	if err := EnsureCreatorSnapshots(ctx, db); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx,
		`INSERT INTO creator_snapshots (handle, platform, follower_count, captured_at)
		 VALUES (?, ?, ?, ?)`,
		s.Handle, s.Platform, s.FollowerCount, s.CapturedAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

// CreatorTrajectory returns every snapshot for handle+platform, oldest first.
func CreatorTrajectory(ctx context.Context, db *sql.DB, handle, platform string) ([]CreatorSnapshot, error) {
	if err := EnsureCreatorSnapshots(ctx, db); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		`SELECT handle, platform, follower_count, captured_at
		   FROM creator_snapshots
		  WHERE handle = ? AND platform = ?
		  ORDER BY captured_at ASC`,
		handle, platform,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]CreatorSnapshot, 0)
	for rows.Next() {
		var s CreatorSnapshot
		var captured string
		if err := rows.Scan(&s.Handle, &s.Platform, &s.FollowerCount, &captured); err != nil {
			return nil, err
		}
		s.CapturedAt, _ = time.Parse(time.RFC3339Nano, captured)
		out = append(out, s)
	}
	return out, rows.Err()
}

// EnsureAdSnapshots lazily creates the ad_snapshots table. Each row is one ad
// id seen for a brand on a network within a single captured batch.
func EnsureAdSnapshots(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS ad_snapshots (
			brand       TEXT NOT NULL,
			batch_id    TEXT NOT NULL,
			network     TEXT NOT NULL,
			ad_id       TEXT NOT NULL,
			captured_at TEXT NOT NULL
		)`)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_ad_snapshots_lookup
			ON ad_snapshots(brand, batch_id, network)`)
	return err
}

// LatestAdSnapshot returns the ad ids per network from the most recent batch
// stored for brand. batchID is empty and the map is empty when no prior
// snapshot exists (first run).
func LatestAdSnapshot(ctx context.Context, db *sql.DB, brand string) (batchID string, idsByNetwork map[string][]string, err error) {
	if err = EnsureAdSnapshots(ctx, db); err != nil {
		return "", nil, err
	}
	// Drain the single-row batch_id query fully before the follow-up query;
	// the store caps connections low, so an open cursor would block it.
	// Order by rowid (monotonic insertion order), not the RFC3339Nano batch_id
	// string: Nano formatting drops trailing-zero fractional seconds, so two
	// batches in the same whole second can sort wrong lexically. rowid always
	// reflects true insertion order, so the highest rowid is the latest batch.
	if err = db.QueryRowContext(ctx,
		`SELECT batch_id FROM ad_snapshots WHERE brand = ? ORDER BY rowid DESC LIMIT 1`,
		brand,
	).Scan(&batchID); err != nil {
		if err == sql.ErrNoRows {
			return "", map[string][]string{}, nil
		}
		return "", nil, err
	}

	rows, err := db.QueryContext(ctx,
		`SELECT network, ad_id FROM ad_snapshots WHERE brand = ? AND batch_id = ?`,
		brand, batchID,
	)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()

	idsByNetwork = map[string][]string{}
	for rows.Next() {
		var network, adID string
		if err := rows.Scan(&network, &adID); err != nil {
			return "", nil, err
		}
		idsByNetwork[network] = append(idsByNetwork[network], adID)
	}
	return batchID, idsByNetwork, rows.Err()
}

// InsertAdSnapshotBatch records the current ad ids per network for brand under
// a new batch keyed by capturedAt.
func InsertAdSnapshotBatch(ctx context.Context, db *sql.DB, brand string, idsByNetwork map[string][]string, capturedAt time.Time) error {
	if err := EnsureAdSnapshots(ctx, db); err != nil {
		return err
	}
	batchID := capturedAt.UTC().Format(time.RFC3339Nano)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO ad_snapshots (brand, batch_id, network, ad_id, captured_at)
		 VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for network, ids := range idsByNetwork {
		for _, id := range ids {
			if _, err := stmt.ExecContext(ctx, brand, batchID, network, id, batchID); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
