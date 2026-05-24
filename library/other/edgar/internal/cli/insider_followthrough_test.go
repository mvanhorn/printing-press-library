// Copyright 2026 magoo242. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/edgar/internal/store"
)

// TestFindEarliestMaterial8K_PicksChronologicallyFirst guards against the
// pre-PATCH bug where iterating ListEdgarFilings rows (DESC by filed_at) and
// breaking on the first material hit returned the most recent material 8-K
// instead of the earliest after the sale.
func TestFindEarliestMaterial8K_PicksChronologicallyFirst(t *testing.T) {
	// DESC by filed_at, as ListEdgarFilings returns:
	eightKs := []store.EdgarFiling{
		{Accession: "z-late-material", FiledAt: "2026-04-01"},
		{Accession: "y-mid-material", FiledAt: "2026-02-15"},
		{Accession: "x-early-immaterial", FiledAt: "2026-01-20"},
		{Accession: "w-early-material", FiledAt: "2026-01-10"},
	}
	itemsByAccession := map[string][]string{
		"z-late-material":    {"5.02"},
		"y-mid-material":     {"1.01"},
		"x-early-immaterial": {"9.01"},
		"w-early-material":   {"2.02"},
	}
	itemsFor := func(i int) []string { return itemsByAccession[eightKs[i].Accession] }

	saleDate := mustDate(t, "2026-01-01")
	endDate := saleDate.AddDate(0, 0, 90)

	idx, items, ok := findEarliestMaterial8K(eightKs, saleDate, endDate, itemsFor)
	if !ok {
		t.Fatalf("expected a match, got ok=false")
	}
	if got, want := eightKs[idx].Accession, "w-early-material"; got != want {
		t.Errorf("picked %s; want %s (earliest material after sale)", got, want)
	}
	if len(items) != 1 || items[0] != "2.02" {
		t.Errorf("items = %v; want [2.02]", items)
	}
}

func TestFindEarliestMaterial8K_SkipsImmaterialAndOutOfWindow(t *testing.T) {
	eightKs := []store.EdgarFiling{
		{Accession: "after-window", FiledAt: "2026-05-15"},
		{Accession: "in-window-immaterial", FiledAt: "2026-02-10"},
		{Accession: "before-sale", FiledAt: "2025-12-01"},
	}
	itemsByAccession := map[string][]string{
		"after-window":         {"1.01"},
		"in-window-immaterial": {"9.01"},
		"before-sale":          {"2.02"},
	}
	itemsFor := func(i int) []string { return itemsByAccession[eightKs[i].Accession] }

	saleDate := mustDate(t, "2026-01-01")
	endDate := saleDate.AddDate(0, 0, 90)

	if _, _, ok := findEarliestMaterial8K(eightKs, saleDate, endDate, itemsFor); ok {
		t.Errorf("expected no match (only 9.01 in window, others out of window)")
	}
}

func TestFindEarliestMaterial8K_EmptyInput(t *testing.T) {
	saleDate := mustDate(t, "2026-01-01")
	if _, _, ok := findEarliestMaterial8K(nil, saleDate, saleDate.AddDate(0, 0, 90), func(int) []string { return nil }); ok {
		t.Errorf("expected no match on empty input")
	}
}

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return d
}
