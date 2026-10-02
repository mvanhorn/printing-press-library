// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"strings"
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

func TestMalformedLegacySnapshotBlocksPruningWithoutChangingState(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.SaveCompletedDesignSync(ctx, 1, []SnapshotRow{{DesignID: "first"}}); err != nil {
		t.Fatal(err)
	}
	baseline := s.GetLastSyncedAt("designs")
	if _, err := s.DB().Exec(`INSERT INTO design_snapshots (sync_at, design_id) VALUES (?, ?)`, "legacy-unknown-time", "legacy"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveCompletedDesignSync(ctx, 2, []SnapshotRow{{DesignID: "second"}}); err == nil || !strings.Contains(err.Error(), "invalid local design snapshot timestamp") {
		t.Fatalf("completed sync error = %v, want malformed timestamp refusal", err)
	}
	if got := s.GetLastSyncedAt("designs"); got != baseline {
		t.Fatalf("failed sync changed watermark from %q to %q", baseline, got)
	}
	var first, legacy, second int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM design_snapshots WHERE design_id = 'first'`).Scan(&first); err != nil {
		t.Fatal(err)
	}
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM design_snapshots WHERE design_id = 'legacy'`).Scan(&legacy); err != nil {
		t.Fatal(err)
	}
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM design_snapshots WHERE design_id = 'second'`).Scan(&second); err != nil {
		t.Fatal(err)
	}
	if first != 1 || legacy != 1 || second != 0 {
		t.Fatalf("failed sync left snapshot counts first=%d legacy=%d second=%d", first, legacy, second)
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
