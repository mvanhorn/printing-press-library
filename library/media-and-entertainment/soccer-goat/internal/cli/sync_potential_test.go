// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/store"
)

func TestLoadBundledPotential(t *testing.T) {
	rows, err := loadBundledPotential()
	if err != nil {
		t.Fatalf("loadBundledPotential: %v", err)
	}
	if len(rows) < 10000 {
		t.Fatalf("expected the full FC24 set (>10k rows), got %d", len(rows))
	}
	// Every row is well-formed: positive id, potential in [1,99], non-empty name.
	byID := map[int]store.PotentialRow{}
	for _, r := range rows {
		if r.EAID <= 0 || r.Potential < 1 || r.Potential > 99 || r.Name == "" {
			t.Fatalf("malformed row: %+v", r)
		}
		if r.Source != "dataset:sofifa-2025" {
			t.Fatalf("row source = %q, want dataset:sofifa-2025", r.Source)
		}
		byID[r.EAID] = r
	}
	// Known anchors from the bundled snapshot.
	anchors := map[int]int{231747: 93, 239085: 92, 260952: 84} // Mbappé, Haaland, Schjelderup
	for id, wantPot := range anchors {
		got, ok := byID[id]
		if !ok {
			t.Fatalf("anchor ea_id %d missing from bundled dataset", id)
		}
		if got.Potential != wantPot {
			t.Errorf("ea_id %d potential = %d, want %d", id, got.Potential, wantPot)
		}
	}
}

func TestSyncPotentialLoadsThenLookup(t *testing.T) {
	ctx := context.Background()
	s, err := store.OpenWithContext(ctx, filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	// Cold store: Schjelderup potential is unavailable before sync.
	if _, ok, _ := s.LookupPotential(ctx, 260952, "Andreas Schjelderup"); ok {
		t.Fatal("expected miss before sync")
	}

	rows, err := loadBundledPotential()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	n, err := s.UpsertPotentialBatch(ctx, rows)
	if err != nil || n < 10000 {
		t.Fatalf("batch loaded %d (err %v)", n, err)
	}

	// After sync, the id lookup returns Schjelderup's 86.
	row, ok, err := s.LookupPotential(ctx, 260952, "Andreas Schjelderup")
	if err != nil || !ok {
		t.Fatalf("lookup after sync ok=%v err=%v", ok, err)
	}
	if row.Potential != 84 {
		t.Fatalf("Schjelderup potential = %d, want 84", row.Potential)
	}
}
