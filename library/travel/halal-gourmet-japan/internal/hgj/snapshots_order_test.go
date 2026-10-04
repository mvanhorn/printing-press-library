// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package hgj

import (
	"context"
	"github.com/mvanhorn/printing-press-library/library/travel/halal-gourmet-japan/internal/store"
	"path/filepath"
	"testing"
)

func TestSnapshotsRetainNewestObservationsWhenSavesArriveLate(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stamps []string
		want   []string
	}{
		{"late middle", []string{"2026-10-03T06:00:00Z", "2026-10-03T07:00:00Z", "2026-10-03T06:30:00Z"}, []string{"1", "2"}},
		{"late older discarded", []string{"2026-10-03T07:00:00Z", "2026-10-03T08:00:00Z", "2026-10-03T06:00:00Z"}, []string{"1", "0"}},
		{"offset instant", []string{"2026-10-03T15:00:00+08:00", "2026-10-03T06:30:00Z", "2026-10-03T07:30:00Z"}, []string{"2", "0"}},
		{"fractional instant", []string{"2026-10-03T07:00:00Z", "2026-10-03T07:00:00.001Z", "2026-10-03T07:00:00.0001Z"}, []string{"1", "2"}},
		{"tie uses latest save", []string{"2026-10-03T07:00:00Z", "2026-10-03T07:00:00Z", "2026-10-03T06:00:00Z"}, []string{"1", "0"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := store.Open(filepath.Join(t.TempDir(), "cache.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			ctx := context.Background()
			for i, stamp := range tc.stamps {
				p := examplePlace(Restaurant, "300739")
				p.Name = string(rune('0' + i))
				p.ObservedAt = stamp
				if err := SaveSnapshot(ctx, db.DB(), p); err != nil {
					t.Fatal(err)
				}
			}
			for position, name := range tc.want {
				got, ok, err := Snapshot(ctx, db.DB(), Selection{Restaurant, "300739"}, position)
				if err != nil || !ok || got.Name != name {
					t.Fatalf("position%d=%s, want%s; %v", position, got.Name, name, err)
				}
			}
			bad := examplePlace(Restaurant, "300739")
			bad.ObservedAt = "not-a-time"
			if SaveSnapshot(ctx, db.DB(), bad) == nil {
				t.Fatal("invalid observation accepted")
			}
			got, _, _ := Snapshot(ctx, db.DB(), Selection{Restaurant, "300739"}, 0)
			if got.Name != tc.want[0] {
				t.Fatal("invalid observation changed history")
			}
		})
	}
}
