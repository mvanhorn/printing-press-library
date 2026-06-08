// Copyright 2026 "Chris Carson" and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := OpenWithContext(context.Background(), filepath.Join(dir, "data.db"))
	if err != nil {
		t.Fatalf("OpenWithContext: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestRecordHistoryAndMinRuns(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// device 1 / KB001 appears in 3 distinct runs; device 1 / KB002 in 1.
	rec := func(key, run string, at int64) {
		if err := s.RecordHistory(ctx, "patch", key, run, at, map[string]any{"status": "FAILED"}); err != nil {
			t.Fatalf("RecordHistory: %v", err)
		}
	}
	rec("1:KB001", "r1", 100)
	rec("1:KB001", "r2", 200)
	rec("1:KB001", "r3", 300)
	rec("1:KB002", "r1", 100)
	// idempotent re-insert of same triple should not inflate count.
	rec("1:KB001", "r3", 300)

	tests := []struct {
		name    string
		minRuns int
		wantLen int
	}{
		{"threshold 3 matches KB001 only", 3, 1},
		{"threshold 2 matches KB001 only", 2, 1},
		{"threshold 1 matches both", 1, 2},
		{"threshold 4 matches none", 4, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.EntitiesWithMinRuns(ctx, "patch", tt.minRuns)
			if err != nil {
				t.Fatalf("EntitiesWithMinRuns: %v", err)
			}
			if len(got) != tt.wantLen {
				t.Fatalf("len = %d, want %d (%+v)", len(got), tt.wantLen, got)
			}
		})
	}

	got, err := s.EntitiesWithMinRuns(ctx, "patch", 3)
	if err != nil {
		t.Fatalf("EntitiesWithMinRuns: %v", err)
	}
	if got[0].EntityKey != "1:KB001" || got[0].Count != 3 {
		t.Fatalf("got %+v, want 1:KB001 count 3", got[0])
	}
	if got[0].FirstSeen != 100 || got[0].LastSeen != 300 {
		t.Fatalf("first/last = %d/%d, want 100/300", got[0].FirstSeen, got[0].LastSeen)
	}
}

func TestEventCountsSince(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	rec := func(key, run string, at int64) {
		if err := s.RecordHistory(ctx, "alert", key, run, at, nil); err != nil {
			t.Fatalf("RecordHistory: %v", err)
		}
	}
	// device 5 / cpu: 3 events, 2 of them after t=150.
	rec("5:cpu", "u1", 100)
	rec("5:cpu", "u2", 200)
	rec("5:cpu", "u3", 300)
	rec("5:mem", "u4", 250)

	tests := []struct {
		name      string
		since     int64
		minEvents int
		wantLen   int
	}{
		{"since 0, min 3 -> cpu", 0, 3, 1},
		{"since 150, min 2 -> cpu only", 150, 2, 1},
		{"since 0, min 1 -> both", 0, 1, 2},
		{"since 400 -> none", 400, 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.EventCountsSince(ctx, "alert", tt.since, tt.minEvents)
			if err != nil {
				t.Fatalf("EventCountsSince: %v", err)
			}
			if len(got) != tt.wantLen {
				t.Fatalf("len = %d, want %d (%+v)", len(got), tt.wantLen, got)
			}
		})
	}
}
