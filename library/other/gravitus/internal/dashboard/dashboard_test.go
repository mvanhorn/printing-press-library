// Copyright 2026 mvanhorn and contributors. Licensed under Apache-2.0. See LICENSE.

package dashboard

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func createTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(path)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	for _, stmt := range []string{
		`CREATE TABLE LiftingSession (
			id TEXT PRIMARY KEY,
			date TEXT NOT NULL,
			title TEXT NOT NULL,
			notes TEXT,
			source TEXT NOT NULL DEFAULT 'gravitus',
			createdAt TEXT NOT NULL,
			UNIQUE(date, source)
		)`,
		`CREATE TABLE Exercise (
			id TEXT PRIMARY KEY,
			sessionId TEXT NOT NULL,
			name TEXT NOT NULL,
			"order" INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE ExerciseSet (
			id TEXT PRIMARY KEY,
			exerciseId TEXT NOT NULL REFERENCES Exercise(id) ON DELETE CASCADE,
			reps INTEGER NOT NULL DEFAULT 0,
			weightLbs REAL NOT NULL DEFAULT 0,
			"order" INTEGER NOT NULL DEFAULT 0
		)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	return db
}

func TestUpsertAndExists(t *testing.T) {
	db := createTestDB(t)
	date := time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC)

	sess := LiftingSession{
		Date:  date,
		Title: "Full Body + Core",
		Exercises: []ExerciseEntry{
			{Name: "Bench Press", Sets: []ExerciseSet{{Reps: 10, WeightLbs: 135}}},
		},
		Source: "gravitus",
	}

	if err := Upsert(db, sess); err != nil {
		t.Fatalf("first Upsert: %v", err)
	}

	exists, err := ExistsOnDate(db, date, "gravitus")
	if err != nil {
		t.Fatalf("ExistsOnDate: %v", err)
	}
	if !exists {
		t.Error("ExistsOnDate = false after Upsert, want true")
	}

	// Upsert again — should update, not leave orphaned ExerciseSet rows
	sess.Title = "Updated"
	if err := Upsert(db, sess); err != nil {
		t.Fatalf("second Upsert: %v", err)
	}
	// Verify cascade: exactly one ExerciseSet row should exist (no orphans)
	var setCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ExerciseSet`).Scan(&setCount); err != nil {
		t.Fatalf("counting ExerciseSet: %v", err)
	}
	if setCount != 1 {
		t.Errorf("ExerciseSet count = %d after re-upsert, want 1 (orphaned rows from broken cascade)", setCount)
	}

	// Different date → not found
	other := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	exists2, err := ExistsOnDate(db, other, "gravitus")
	if err != nil {
		t.Fatalf("ExistsOnDate (miss): %v", err)
	}
	if exists2 {
		t.Error("ExistsOnDate = true for absent date, want false")
	}
}

func TestTableExists(t *testing.T) {
	db := createTestDB(t)
	ok, err := TableExists(db)
	if err != nil {
		t.Fatalf("TableExists: %v", err)
	}
	if !ok {
		t.Error("TableExists = false after creating schema, want true")
	}
}

func TestNewCUID(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		id := newCUID()
		if len(id) != 25 {
			t.Fatalf("newCUID length = %d, want 25: %q", len(id), id)
		}
		if id[0] != 'c' {
			t.Fatalf("newCUID does not start with 'c': %q", id)
		}
		if seen[id] {
			t.Fatalf("newCUID returned duplicate: %s", id)
		}
		seen[id] = true
	}
}
