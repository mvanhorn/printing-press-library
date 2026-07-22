// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"
)

func newPotentialStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "potential.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestUpsertAndLookupPotentialByID(t *testing.T) {
	ctx := context.Background()
	s := newPotentialStore(t)
	if err := s.UpsertPotential(ctx, PotentialRow{EAID: 231747, Name: "Kylian Mbappé Lottin", Overall: 91, Potential: 94, Source: "dataset:fc24"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	row, ok, err := s.LookupPotential(ctx, 231747, "")
	if err != nil || !ok {
		t.Fatalf("lookup by id ok=%v err=%v", ok, err)
	}
	if row.Potential != 94 || row.Overall != 91 || row.Source != "dataset:fc24" {
		t.Fatalf("row = %+v, want potential 94 overall 91", row)
	}
}

func TestLookupPotentialByNameFallback(t *testing.T) {
	ctx := context.Background()
	s := newPotentialStore(t)
	// Stored with diacritics; name_normalized computed by the store (Go fold).
	if err := s.UpsertPotential(ctx, PotentialRow{EAID: 260952, Name: "Andreas Rædergård Schjelderup", Overall: 73, Potential: 84, Source: "dataset:fc24"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// Look up with a differently-cased, differently-accented report name and no id.
	row, ok, err := s.LookupPotential(ctx, 0, "andreas raedergard schjelderup")
	if err != nil || !ok {
		t.Fatalf("name fallback ok=%v err=%v", ok, err)
	}
	if row.Potential != 84 {
		t.Fatalf("row.Potential = %d, want 84", row.Potential)
	}
}

func TestLookupPotentialMiss(t *testing.T) {
	ctx := context.Background()
	s := newPotentialStore(t)
	row, ok, err := s.LookupPotential(ctx, 999999, "nobody at all")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatalf("expected miss, got row %+v", row)
	}
	if row.Potential != 0 {
		t.Fatalf("miss must return zero row, got potential %d", row.Potential)
	}
}

func TestUpsertPotentialOverwrites(t *testing.T) {
	ctx := context.Background()
	s := newPotentialStore(t)
	_ = s.UpsertPotential(ctx, PotentialRow{EAID: 158023, Name: "Lionel Messi", Overall: 90, Potential: 90, Source: "dataset:fc24"})
	_ = s.UpsertPotential(ctx, PotentialRow{EAID: 158023, Name: "Lionel Messi", Overall: 88, Potential: 90, Source: "live:fifacm", CapturedAt: "2026-07-12T00:00:00Z"})
	row, ok, _ := s.LookupPotential(ctx, 158023, "")
	if !ok || row.Overall != 88 || row.Source != "live:fifacm" {
		t.Fatalf("re-upsert should overwrite: row=%+v", row)
	}
	n, _ := s.PotentialCount(ctx)
	if n != 1 {
		t.Fatalf("count = %d, want 1 (no duplicate on same ea_id)", n)
	}
}

func TestUpsertPotentialBatchSkipsInvalid(t *testing.T) {
	ctx := context.Background()
	s := newPotentialStore(t)
	rows := []PotentialRow{
		{EAID: 1, Name: "A", Overall: 70, Potential: 80, Source: "dataset:fc24"},
		{EAID: 0, Name: "no id", Overall: 70, Potential: 80},   // skipped: no id
		{EAID: 2, Name: "B", Overall: 70, Potential: 0},        // skipped: no potential
		{EAID: 3, Name: "C", Overall: 60, Potential: 90, Source: "dataset:fc24"},
	}
	n, err := s.UpsertPotentialBatch(ctx, rows)
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if n != 2 {
		t.Fatalf("inserted %d, want 2 valid rows", n)
	}
	count, _ := s.PotentialCount(ctx)
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
}

func TestNormalizePotentialName(t *testing.T) {
	cases := map[string]string{
		"Kylian Mbappé":                 "kylian mbappe",
		"Andreas Rædergård Schjelderup": "andreas raedergard schjelderup",
		"João Félix":                    "joao felix",
		"N'Golo Kanté":                  "n golo kante",
	}
	for in, want := range cases {
		if got := NormalizePotentialName(in); got != want {
			t.Errorf("NormalizePotentialName(%q) = %q, want %q", in, got, want)
		}
	}
}
