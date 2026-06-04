// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestKeywordSignalSnapshotsRoundTrip(t *testing.T) {
	ctx := context.Background()
	db, err := OpenWithContext(ctx, filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatalf("OpenWithContext() error = %v", err)
	}
	defer db.Close()

	capturedAt := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	if err := db.InsertKeywordSignalSnapshot(ctx, KeywordSignalSnapshot{
		Keyword:    "ceramic mug",
		Source:     "live",
		Country:    "us",
		Score:      72.5,
		Rating:     "strong",
		CapturedAt: capturedAt,
	}); err != nil {
		t.Fatalf("InsertKeywordSignalSnapshot() error = %v", err)
	}
	if err := db.InsertKeywordSignalSnapshot(ctx, KeywordSignalSnapshot{
		Keyword:    "ceramic mug",
		Source:     "cached",
		Country:    "us",
		Score:      55,
		Rating:     "mixed",
		CapturedAt: capturedAt.Add(time.Second),
	}); err != nil {
		t.Fatalf("InsertKeywordSignalSnapshot() second source error = %v", err)
	}

	snapshots, err := db.ListKeywordSignalSnapshots(ctx, KeywordSignalSnapshotFilter{
		Keyword: "ceramic mug",
		Source:  "live",
		Country: "us",
	})
	if err != nil {
		t.Fatalf("ListKeywordSignalSnapshots() error = %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("ListKeywordSignalSnapshots() len = %d, want 1", len(snapshots))
	}
	if snapshots[0].Score != 72.5 {
		t.Fatalf("Score = %v, want 72.5", snapshots[0].Score)
	}
	if snapshots[0].Source != "live" {
		t.Fatalf("Source = %q, want live", snapshots[0].Source)
	}
	if !snapshots[0].CapturedAt.Equal(capturedAt) {
		t.Fatalf("CapturedAt = %v, want %v", snapshots[0].CapturedAt, capturedAt)
	}
}

func TestKeywordSignalSnapshotsSinceAndOrdering(t *testing.T) {
	ctx := context.Background()
	db, err := OpenWithContext(ctx, filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatalf("OpenWithContext() error = %v", err)
	}
	defer db.Close()

	exactSecond := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	fractionalSecond := exactSecond.Add(500 * time.Millisecond)
	for _, snapshot := range []KeywordSignalSnapshot{
		{Keyword: "ceramic mug", Source: "live", Country: "us", Score: 70, Rating: "mixed", CapturedAt: exactSecond},
		{Keyword: "ceramic mug", Source: "live", Country: "us", Score: 80, Rating: "strong", CapturedAt: fractionalSecond},
	} {
		if err := db.InsertKeywordSignalSnapshot(ctx, snapshot); err != nil {
			t.Fatalf("InsertKeywordSignalSnapshot() error = %v", err)
		}
	}

	snapshots, err := db.ListKeywordSignalSnapshots(ctx, KeywordSignalSnapshotFilter{
		Keyword: "ceramic mug",
		Source:  "live",
		Country: "us",
		Since:   exactSecond,
	})
	if err != nil {
		t.Fatalf("ListKeywordSignalSnapshots() error = %v", err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("ListKeywordSignalSnapshots() len = %d, want 2", len(snapshots))
	}
	if !snapshots[0].CapturedAt.Equal(fractionalSecond) {
		t.Fatalf("first CapturedAt = %v, want %v", snapshots[0].CapturedAt, fractionalSecond)
	}
	if !snapshots[1].CapturedAt.Equal(exactSecond) {
		t.Fatalf("second CapturedAt = %v, want %v", snapshots[1].CapturedAt, exactSecond)
	}
}

func TestKeywordSignalSnapshotsDefaultCapturedAtAndMigrationReopen(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "store.db")
	db, err := OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenWithContext() error = %v", err)
	}

	before := time.Now().UTC()
	if err := db.InsertKeywordSignalSnapshot(ctx, KeywordSignalSnapshot{
		Keyword: "ceramic mug",
		Source:  "live",
		Country: "us",
		Score:   72.5,
		Rating:  "strong",
	}); err != nil {
		t.Fatalf("InsertKeywordSignalSnapshot() error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	db, err = OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenWithContext() after reopen error = %v", err)
	}
	defer db.Close()

	snapshots, err := db.ListKeywordSignalSnapshots(ctx, KeywordSignalSnapshotFilter{
		Keyword: "ceramic mug",
		Source:  "live",
		Country: "us",
	})
	if err != nil {
		t.Fatalf("ListKeywordSignalSnapshots() error = %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("ListKeywordSignalSnapshots() len = %d, want 1", len(snapshots))
	}
	if snapshots[0].CapturedAt.Before(before) {
		t.Fatalf("CapturedAt = %v, want not before %v", snapshots[0].CapturedAt, before)
	}
}

func TestKeywordSignalSnapshotsMigrateLegacyTextCapturedAt(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "store.db")
	legacyDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	capturedAt := time.Date(2026, 6, 1, 12, 0, 0, 123456789, time.UTC)
	if _, err := legacyDB.ExecContext(ctx, `CREATE TABLE keyword_signal_snapshots (
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
		captured_at TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create legacy keyword_signal_snapshots error = %v", err)
	}
	if _, err := legacyDB.ExecContext(ctx, `CREATE INDEX idx_keyword_signal_snapshots_lookup
		ON keyword_signal_snapshots(keyword, country, source, captured_at DESC)`); err != nil {
		t.Fatalf("create legacy keyword_signal_snapshots index error = %v", err)
	}
	if _, err := legacyDB.ExecContext(ctx, `INSERT INTO keyword_signal_snapshots (
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
		"ceramic mug",
		"live",
		"us",
		72.5,
		"strong",
		10,
		20,
		30,
		4,
		5,
		capturedAt.Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("insert legacy keyword_signal_snapshots error = %v", err)
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatalf("legacy Close() error = %v", err)
	}

	db, err := OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenWithContext() error = %v", err)
	}
	defer db.Close()

	snapshots, err := db.ListKeywordSignalSnapshots(ctx, KeywordSignalSnapshotFilter{
		Keyword: "ceramic mug",
		Source:  "live",
		Country: "us",
	})
	if err != nil {
		t.Fatalf("ListKeywordSignalSnapshots() error = %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("ListKeywordSignalSnapshots() len = %d, want 1", len(snapshots))
	}
	if !snapshots[0].CapturedAt.Equal(capturedAt) {
		t.Fatalf("CapturedAt = %v, want %v", snapshots[0].CapturedAt, capturedAt)
	}

	var capturedAtType string
	if err := db.DB().QueryRowContext(ctx, `SELECT type FROM pragma_table_info('keyword_signal_snapshots') WHERE name = 'captured_at'`).Scan(&capturedAtType); err != nil {
		t.Fatalf("query captured_at type error = %v", err)
	}
	if capturedAtType != "INTEGER" {
		t.Fatalf("captured_at type = %q, want INTEGER", capturedAtType)
	}
}
