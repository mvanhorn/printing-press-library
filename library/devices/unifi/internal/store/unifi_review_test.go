// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// A first sync that only reached some resource types used to spend the console's one
// baseline, so the next full sync reported every existing client as a newcomer.
func TestBaselineIsPerResourceType(t *testing.T) {
	ctx := context.Background()
	db, err := OpenWithContext(ctx, filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	track := func(string) bool { return true }
	start := time.Now().Add(-time.Hour)
	row := func(rt, id string) SnapshotRow {
		return SnapshotRow{ResourceType: rt, EntityID: id, Data: []byte(`{}`)}
	}
	if _, err := db.SaveSnapshot(ctx, "C1", start, []SnapshotRow{row("devices", "d1")}, nil, track); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SaveSnapshot(ctx, "C1", start.Add(time.Minute), []SnapshotRow{row("devices", "d1"), row("clients", "old-client")}, nil, track); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SaveSnapshot(ctx, "C1", start.Add(2*time.Minute), []SnapshotRow{row("devices", "d1"), row("devices", "d2"), row("clients", "old-client"), row("clients", "new-client")}, nil, track); err != nil {
		t.Fatal(err)
	}
	rows, err := db.FirstSeenSince(ctx, "C1", start.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, r := range rows {
		got[r.EntityID] = true
	}
	if got["old-client"] || got["d1"] {
		t.Fatalf("baseline entity reported as new: %v", got)
	}
	if !got["d2"] || !got["new-client"] {
		t.Fatalf("real newcomer missed: %v", got)
	}
}
