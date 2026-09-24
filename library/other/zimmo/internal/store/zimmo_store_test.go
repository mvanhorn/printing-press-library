// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/zimmo"
)

func TestAddrKey(t *testing.T) {
	cases := map[[2]string]string{
		{"1030", "Avenue Charbo 24-26"}:             "1030|charbo|24",
		{"1030", "Rue Anatole France 37"}:           "1030|anatole france|37",
		{"1000", "Rue du 11 Novembre 4"}:            "1000|du 11novembre|4",
		{"1060", "Théodore Verhaegenstraat 93"}:     "1060|theodore verhaegenstraat|93",
		{"1050", "Chaussée de Waterloo 1234 bte 5"}: "1050|de waterloo|1234",
	}
	for in, want := range cases {
		if got := AddrKey(in[0], in[1]); got != want {
			t.Errorf("AddrKey(%q,%q)=%q want %q", in[0], in[1], got, want)
		}
	}
	if AddrKey("1050", "Rue sans numéro") != "" || AddrKey("", "Rue X 1") != "" {
		t.Error("no number or no postcode must give an empty key")
	}
}

func TestZimmoStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, err := OpenWithContext(ctx, filepath.Join(t.TempDir(), "z.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.EnsureZimmoSchema(ctx); err != nil {
		t.Fatal(err)
	}
	p1, p2 := 300000.0, 280000.0
	l := zimmo.Listing{Code: "AAAAA", Status: "FOR_SALE", Type: "HOUSE", PostalCode: "1030", Street: "Rue X", Number: "5", Price: &p1, EPC: "D_MINUS"}
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if err := s.UpsertZimmoListings(ctx, []zimmo.Listing{l}, t0, false); err != nil {
		t.Fatal(err)
	}
	l.Price = &p2
	if err := s.UpsertZimmoListings(ctx, []zimmo.Listing{l}, t0.Add(24*time.Hour), true); err != nil {
		t.Fatal(err)
	}
	rows, err := s.QueryZimmoListings(ctx, ListingFilter{Postcodes: []string{"1030"}})
	if err != nil || len(rows) != 1 {
		t.Fatalf("query: %v %d", err, len(rows))
	}
	if *rows[0].Price != p2 || rows[0].DetailAt == "" || rows[0].FirstSeen != "2026-09-01T00:00:00Z" {
		t.Errorf("latest price, detail stamp and first_seen kept: %+v", rows[0])
	}
	if rows[0].EPC != "D-" {
		t.Errorf("stored labels are normalised on read, got %q", rows[0].EPC)
	}
	obs, _ := s.AllZimmoPriceHistories(ctx)
	if len(obs["AAAAA"]) != 2 {
		t.Errorf("two distinct prices = two observations, got %v", obs["AAAAA"])
	}
	l.Status = "SOLD"
	_ = s.UpsertZimmoListings(ctx, []zimmo.Listing{l}, t0.Add(48*time.Hour), false)
	if rows, _ := s.QueryZimmoListings(ctx, ListingFilter{}); len(rows) != 0 {
		t.Errorf("a sold listing leaves the active set")
	}
	if rows, _ := s.QueryZimmoListings(ctx, ListingFilter{OnlyGone: true}); len(rows) != 1 {
		t.Errorf("a sold listing is gone")
	}
	if err := s.SaveZimmoSearch(ctx, "w", zimmo.Criteria{Postcodes: []string{"1030"}}, t0); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceZimmoSearchSeen(ctx, "w", []zimmo.Listing{l}, t0); err != nil {
		t.Fatal(err)
	}
	seen, _ := s.ZimmoSearchSeen(ctx, "w")
	if len(seen) != 1 || seen["AAAAA"].LastPrice == nil {
		t.Errorf("seen-set stores the code and price: %+v", seen)
	}
	if err := s.ReplaceZimmoSearchSeen(ctx, "w", nil, t0); err != nil {
		t.Fatal(err)
	}
	if seen, _ := s.ZimmoSearchSeen(ctx, "w"); len(seen) != 0 {
		t.Errorf("empty result set clears the seen-set: %+v", seen)
	}
}

func TestMergeZimmoSearchSeenKeepsUnscannedHistory(t *testing.T) {
	ctx := context.Background()
	s, err := OpenWithContext(ctx, filepath.Join(t.TempDir(), "z.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.EnsureZimmoSchema(ctx); err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	later := t0.Add(31 * 24 * time.Hour)
	p1, p2 := 200000.0, 210000.0
	if err := s.SaveZimmoSearch(ctx, "w", zimmo.Criteria{Postcodes: []string{"1030"}}, t0); err != nil {
		t.Fatal(err)
	}
	if err := s.MergeZimmoSearchSeen(ctx, "w", []zimmo.Listing{{Code: "OLD01", Price: &p1}}, t0); err != nil {
		t.Fatal(err)
	}
	if err := s.MergeZimmoSearchSeen(ctx, "w", []zimmo.Listing{{Code: "PAGE1", Price: &p2}}, later); err != nil {
		t.Fatal(err)
	}
	seen, err := s.ZimmoSearchSeen(ctx, "w")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := seen["OLD01"]; !ok {
		t.Fatalf("an incomplete scan must keep listings on pages it did not revisit: %+v", seen)
	}
	if _, ok := seen["PAGE1"]; !ok {
		t.Fatalf("the scanned page must be recorded: %+v", seen)
	}
}
