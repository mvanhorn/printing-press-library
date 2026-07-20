// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
package practice

import (
	"context"
	"path/filepath"
	"testing"
)

func TestLogReadClearsStaleFinishedAtAndPreservesRating(t *testing.T) {
	p, err := Open(context.Background(), filepath.Join(t.TempDir(), "passage.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer p.Close()

	if err := p.LogRead(context.Background(), "OL1W", "A Book", "read", 5); err != nil {
		t.Fatalf("LogRead(read): %v", err)
	}
	if err := p.LogRead(context.Background(), "OL1W", "", "reading", 0); err != nil {
		t.Fatalf("LogRead(reading): %v", err)
	}

	var title, status, finishedAt string
	var rating int
	if err := p.db.QueryRow(`SELECT title, status, rating, finished_at FROM reading_log WHERE work_key=?`, "OL1W").Scan(&title, &status, &rating, &finishedAt); err != nil {
		t.Fatalf("query reading_log: %v", err)
	}
	if title != "A Book" || status != "reading" || rating != 5 || finishedAt != "" {
		t.Fatalf("row = title %q, status %q, rating %d, finished_at %q; want A Book, reading, 5, empty", title, status, rating, finishedAt)
	}
}

func TestRecentSitIDsReturnsDatabaseErrors(t *testing.T) {
	p, err := Open(context.Background(), filepath.Join(t.TempDir(), "passage.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := p.RecentSitIDs(context.Background(), 14); err == nil {
		t.Fatal("RecentSitIDs returned nil error after the database was closed")
	}
}
