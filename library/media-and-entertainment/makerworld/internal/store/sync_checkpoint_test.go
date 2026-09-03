// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"path/filepath"
	"testing"
)

func TestSaveSyncCheckpointDoesNotAdvanceLastSyncedAt(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	if err := s.SaveSyncState("designs", "", 10); err != nil {
		t.Fatalf("SaveSyncState: %v", err)
	}
	const sentinel = "2000-01-01T00:00:00Z"
	if _, err := s.DB().Exec(`UPDATE sync_state SET last_synced_at = ? WHERE resource_type = ?`, sentinel, "designs"); err != nil {
		t.Fatalf("seed last_synced_at: %v", err)
	}

	if err := s.SaveSyncCheckpoint("designs", "2", 20); err != nil {
		t.Fatalf("SaveSyncCheckpoint: %v", err)
	}
	if got := s.GetLastSyncedAt("designs"); got != sentinel {
		t.Fatalf("checkpoint advanced last_synced_at: got %q want %q", got, sentinel)
	}
	cursor, _, count, err := s.GetSyncState("designs")
	if err != nil {
		t.Fatalf("GetSyncState: %v", err)
	}
	if cursor != "2" || count != 20 {
		t.Fatalf("checkpoint cursor/count = %q/%d want 2/20", cursor, count)
	}
}

func TestSaveSyncCheckpointFirstInsertLeavesLastSyncedAtEmpty(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	if err := s.SaveSyncCheckpoint("designs", "1", 5); err != nil {
		t.Fatalf("SaveSyncCheckpoint: %v", err)
	}
	cursor, watermark, count, err := s.GetSyncState("designs")
	if err != nil {
		t.Fatalf("GetSyncState after first checkpoint: %v", err)
	}
	if cursor != "1" || count != 5 {
		t.Fatalf("cursor/count = %q/%d want 1/5", cursor, count)
	}
	if !watermark.IsZero() {
		t.Fatalf("first checkpoint stamped last_synced_at = %v", watermark)
	}
	if got := s.GetLastSyncedAt("designs"); got != "" {
		t.Fatalf("GetLastSyncedAt after first checkpoint = %q, want empty", got)
	}
}
