// Copyright 2026 Matthew Vassallo and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestSyncLeaseSerializesDatabaseWriters(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "nested", "data.db")
	first, err := AcquireSyncLease(dbPath)
	if err != nil {
		t.Fatalf("acquire first lease: %v", err)
	}
	defer first.Close()

	if _, err := AcquireSyncLease(dbPath); !errors.Is(err, ErrSyncLeaseHeld) {
		t.Fatalf("second lease error = %v, want ErrSyncLeaseHeld", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("release first lease: %v", err)
	}
	second, err := AcquireSyncLease(dbPath)
	if err != nil {
		t.Fatalf("acquire lease after release: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("release second lease: %v", err)
	}
}
