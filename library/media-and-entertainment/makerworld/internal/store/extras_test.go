// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDesignSnapshotRetentionUsesTimeOrder(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Older RFC3339Nano rows have variable-width fractions. Text sorting
	// would put .9 after .91 even though .91 is later.
	for _, stamp := range []string{
		"2026-10-01T00:00:00.9Z",
		"2026-10-01T00:00:00.901Z",
		"2026-10-01T00:00:00.91Z",
	} {
		if err := s.RecordDesignSnapshots(context.Background(), stamp, []SnapshotRow{{DesignID: stamp}}); err != nil {
			t.Fatalf("record %s: %v", stamp, err)
		}
	}
	stamps, err := RecentDesignSnapshotTimes(context.Background(), s.DB())
	if err != nil {
		t.Fatal(err)
	}
	if len(stamps) != 2 || stamps[0] != "2026-10-01T00:00:00.91Z" || stamps[1] != "2026-10-01T00:00:00.901Z" {
		t.Fatalf("retained timestamps = %v, want latest two in time order", stamps)
	}
}

func TestCompletedDesignSyncRetainsTwoBatches(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	for i := 0; i < 3; i++ {
		if err := s.SaveCompletedDesignSync(context.Background(), 1, []SnapshotRow{{DesignID: "design-1", Like: i}}); err != nil {
			t.Fatalf("complete sync %d: %v", i, err)
		}
	}
	stamps, err := RecentDesignSnapshotTimes(context.Background(), s.DB())
	if err != nil {
		t.Fatal(err)
	}
	if len(stamps) != 2 {
		t.Fatalf("retained %d batches, want 2", len(stamps))
	}
	if watermark := s.GetLastSyncedAt("designs"); watermark != stamps[0] {
		t.Fatalf("watermark %q differs from latest snapshot %q", watermark, stamps[0])
	}
	if err := s.RecordDesignSnapshots(context.Background(), s.GetLastSyncedAt("designs"), []SnapshotRow{{DesignID: "design-1"}}); err != nil {
		t.Fatalf("record current watermark: %v", err)
	}
	afterRead, err := RecentDesignSnapshotTimes(context.Background(), s.DB())
	if err != nil {
		t.Fatal(err)
	}
	if len(afterRead) != 2 || afterRead[0] != stamps[0] || afterRead[1] != stamps[1] {
		t.Fatalf("reading current snapshot changed batches from %v to %v", stamps, afterRead)
	}
}
