// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const keywordSignalSnapshotsTableSQL = `CREATE TABLE IF NOT EXISTS keyword_signal_snapshots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	keyword TEXT NOT NULL,
	source TEXT NOT NULL,
	country TEXT NOT NULL,
	score REAL NOT NULL,
	rating TEXT NOT NULL,
	search_signal REAL NOT NULL DEFAULT 0,
	competition_signal REAL NOT NULL DEFAULT 0,
	difficulty_signal REAL NOT NULL DEFAULT 0,
	tag_count INTEGER NOT NULL DEFAULT 0,
	top_listing_count INTEGER NOT NULL DEFAULT 0,
	captured_at INTEGER NOT NULL
)`

const keywordSignalSnapshotsIndexSQL = `CREATE INDEX IF NOT EXISTS idx_keyword_signal_snapshots_lookup
	ON keyword_signal_snapshots(keyword, country, source, captured_at DESC)`

// migrateExtras runs after the generated store migrations and before the
// schema-version stamp. It is the canonical place for novel-feature auxiliary
// tables that need to live in the local store.
//
// Edit this file when adding tables for novel commands. Keep migrations
// idempotent with CREATE TABLE IF NOT EXISTS / CREATE INDEX IF NOT EXISTS so
// every store open can safely re-run them.
func (s *Store) migrateExtras(ctx context.Context, conn *sql.Conn) error {
	if err := s.migrateKeywordSignalSnapshots(ctx, conn); err != nil {
		return fmt.Errorf("migrating keyword signal snapshots: %w", err)
	}
	migrations := []string{keywordSignalSnapshotsIndexSQL}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}

func (s *Store) migrateKeywordSignalSnapshots(ctx context.Context, conn *sql.Conn) error {
	exists, err := tableExists(ctx, conn, "keyword_signal_snapshots")
	if err != nil {
		return err
	}
	if !exists {
		if _, err := conn.ExecContext(ctx, keywordSignalSnapshotsTableSQL); err != nil {
			return fmt.Errorf("creating keyword_signal_snapshots: %w", err)
		}
		return nil
	}

	capturedAtType, err := keywordSignalSnapshotsCapturedAtType(ctx, conn)
	if err != nil {
		return err
	}
	if strings.EqualFold(capturedAtType, "INTEGER") {
		return nil
	}
	if !strings.EqualFold(capturedAtType, "TEXT") {
		return fmt.Errorf("unsupported keyword_signal_snapshots captured_at type %q", capturedAtType)
	}
	return rebuildKeywordSignalSnapshotsWithIntegerTimestamp(ctx, conn)
}

func keywordSignalSnapshotsCapturedAtType(ctx context.Context, conn *sql.Conn) (string, error) {
	rows, err := conn.QueryContext(ctx, `PRAGMA table_info(keyword_signal_snapshots)`)
	if err != nil {
		return "", fmt.Errorf("reading keyword_signal_snapshots table info: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pkOrder int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pkOrder); err != nil {
			return "", fmt.Errorf("scanning keyword_signal_snapshots table info: %w", err)
		}
		if name == "captured_at" {
			return strings.TrimSpace(typ), nil
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("reading keyword_signal_snapshots table info rows: %w", err)
	}
	return "", fmt.Errorf("keyword_signal_snapshots captured_at column missing")
}

func rebuildKeywordSignalSnapshotsWithIntegerTimestamp(ctx context.Context, conn *sql.Conn) error {
	type legacySnapshot struct {
		id                int64
		keyword           string
		source            string
		country           string
		score             float64
		rating            string
		searchSignal      float64
		competitionSignal float64
		difficultySignal  float64
		tagCount          int
		topListingCount   int
		capturedAt        string
	}

	rows, err := conn.QueryContext(ctx, `SELECT
		id,
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
		ORDER BY id`)
	if err != nil {
		return fmt.Errorf("reading legacy keyword_signal_snapshots: %w", err)
	}

	var snapshots []legacySnapshot
	for rows.Next() {
		var snapshot legacySnapshot
		if err := rows.Scan(
			&snapshot.id,
			&snapshot.keyword,
			&snapshot.source,
			&snapshot.country,
			&snapshot.score,
			&snapshot.rating,
			&snapshot.searchSignal,
			&snapshot.competitionSignal,
			&snapshot.difficultySignal,
			&snapshot.tagCount,
			&snapshot.topListingCount,
			&snapshot.capturedAt,
		); err != nil {
			rows.Close()
			return fmt.Errorf("scanning legacy keyword_signal_snapshots: %w", err)
		}
		snapshots = append(snapshots, snapshot)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("closing legacy keyword_signal_snapshots rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("reading legacy keyword_signal_snapshots rows: %w", err)
	}

	if _, err := conn.ExecContext(ctx, `DROP INDEX IF EXISTS idx_keyword_signal_snapshots_lookup`); err != nil {
		return fmt.Errorf("dropping keyword_signal_snapshots lookup index: %w", err)
	}
	if _, err := conn.ExecContext(ctx, strings.Replace(keywordSignalSnapshotsTableSQL, "keyword_signal_snapshots", "keyword_signal_snapshots_v2", 1)); err != nil {
		return fmt.Errorf("creating keyword_signal_snapshots_v2: %w", err)
	}
	for _, snapshot := range snapshots {
		capturedAt, err := time.Parse(time.RFC3339Nano, snapshot.capturedAt)
		if err != nil {
			return fmt.Errorf("parsing legacy keyword_signal_snapshots captured_at %q: %w", snapshot.capturedAt, err)
		}
		if _, err := conn.ExecContext(ctx, `INSERT INTO keyword_signal_snapshots_v2 (
			id,
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
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			snapshot.id,
			snapshot.keyword,
			snapshot.source,
			snapshot.country,
			snapshot.score,
			snapshot.rating,
			snapshot.searchSignal,
			snapshot.competitionSignal,
			snapshot.difficultySignal,
			snapshot.tagCount,
			snapshot.topListingCount,
			capturedAt.UTC().UnixNano(),
		); err != nil {
			return fmt.Errorf("inserting converted keyword_signal_snapshots row: %w", err)
		}
	}
	if _, err := conn.ExecContext(ctx, `DROP TABLE keyword_signal_snapshots`); err != nil {
		return fmt.Errorf("dropping legacy keyword_signal_snapshots: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `ALTER TABLE keyword_signal_snapshots_v2 RENAME TO keyword_signal_snapshots`); err != nil {
		return fmt.Errorf("renaming keyword_signal_snapshots_v2: %w", err)
	}
	return nil
}
