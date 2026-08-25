// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSaveSyncResumePreservesWatermark(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	if err := s.SaveSyncState("news", "", 100); err != nil {
		t.Fatalf("complete save: %v", err)
	}
	_, watermark, _, err := s.GetSyncState("news")
	if err != nil {
		t.Fatalf("get after complete: %v", err)
	}
	if watermark.IsZero() {
		t.Fatal("complete save should set last_synced_at")
	}

	time.Sleep(15 * time.Millisecond)
	if err := s.SaveSyncResume("news", "3", 145); err != nil {
		t.Fatalf("resume save: %v", err)
	}
	cursor, got, count, err := s.GetSyncState("news")
	if err != nil {
		t.Fatalf("get after resume: %v", err)
	}
	if cursor != "3" {
		t.Fatalf("cursor = %q, want 3", cursor)
	}
	if count != 145 {
		t.Fatalf("count = %d, want 145", count)
	}
	if !got.Equal(watermark) {
		t.Fatalf("last_synced_at changed from %v to %v", watermark, got)
	}
}

func TestSaveSyncResumeFirstInsertHasNoWatermark(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	if err := s.SaveSyncResume("news", "1", 15); err != nil {
		t.Fatalf("resume save: %v", err)
	}
	cursor, watermark, count, err := s.GetSyncState("news")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if cursor != "1" || count != 15 {
		t.Fatalf("cursor/count = %q/%d, want 1/15", cursor, count)
	}
	if !watermark.IsZero() {
		t.Fatalf("first incomplete save should leave last_synced_at unset, got %v", watermark)
	}
}
