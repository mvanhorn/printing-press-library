// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestRecordEngagementAndLatest(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	// First capture: real likes, no comments sampled (pass -1 for unknown).
	if err := s.RecordEngagement(ctx, "selly9401", "224234460263", 5, -1, "reaction-api"); err != nil {
		t.Fatalf("RecordEngagement #1: %v", err)
	}
	// Sleep so captured_at differs.
	time.Sleep(1 * time.Second)
	// Second capture: same post, both metrics, different source.
	if err := s.RecordEngagement(ctx, "selly9401", "224234460263", 7, 12, "static-html"); err != nil {
		t.Fatalf("RecordEngagement #2: %v", err)
	}

	snap, err := s.LatestEngagement(ctx, "selly9401", "224234460263", time.Now().UTC())
	if err != nil {
		t.Fatalf("LatestEngagement: %v", err)
	}
	if snap == nil {
		t.Fatal("LatestEngagement returned nil snapshot")
	}
	if !snap.Likes.Valid || snap.Likes.Int64 != 7 {
		t.Errorf("Likes = %+v, want 7 valid", snap.Likes)
	}
	if !snap.Comments.Valid || snap.Comments.Int64 != 12 {
		t.Errorf("Comments = %+v, want 12 valid", snap.Comments)
	}
	if snap.Source != "static-html" {
		t.Errorf("Source = %q, want static-html", snap.Source)
	}

	// LatestEngagement bounded to BEFORE the second snapshot should
	// return the first snapshot (with NULL comments).
	cutoff := snap.CapturedAt.Add(-500 * time.Millisecond)
	older, err := s.LatestEngagement(ctx, "selly9401", "224234460263", cutoff)
	if err != nil {
		t.Fatalf("LatestEngagement(older): %v", err)
	}
	if older == nil {
		t.Fatal("older snapshot returned nil")
	}
	if !older.Likes.Valid || older.Likes.Int64 != 5 {
		t.Errorf("older Likes = %+v, want 5 valid", older.Likes)
	}
	if older.Comments.Valid {
		t.Errorf("older Comments = %+v, want invalid (NULL)", older.Comments)
	}
}

func TestLatestEngagementNoData(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	snap, err := s.LatestEngagement(context.Background(), "missing", "1", time.Now().UTC())
	if err != nil {
		t.Fatalf("LatestEngagement: %v", err)
	}
	if snap != nil {
		t.Errorf("snap = %+v, want nil for missing post", snap)
	}
}
