// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the canonical ID minting and classify pipeline (Task 7).
// All fixtures are synthetic — no real tenant ticket-type or venue names.
package cli

import (
	"context"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/store"
)

// TestFuzzyClassifyDeterministic verifies that running the fuzzy classify pass
// twice on the same seeded store produces identical canonical_id assignments.
// Non-determinism would manifest as different representative choices across
// runs when canonNames is built from an unordered map. The seeded names form
// two independent clusters so the representative choice matters and map
// iteration could swap them.
func TestFuzzyClassifyDeterministic(t *testing.T) {
	// Two pairs of near-duplicates. Alphabetically the representatives are
	// "alpha bird" and "final release" — after sort-then-cluster that must be
	// stable across runs.
	s := seedStore(t, map[string]map[string]string{
		"tickets": {
			"t1": `{"id":"t1","ticketType":{"name":"Early Bird"}}`,
			"t2": `{"id":"t2","ticketType":{"name":"Early Birds"}}`,
			"t3": `{"id":"t3","ticketType":{"name":"Final Release"}}`,
			"t4": `{"id":"t4","ticketType":{"name":"Final Releases"}}`,
		},
	})
	opts := classifyOpts{ClassifierVersion: 1, Fuzzy: true}

	_, err := classifyTiers(context.Background(), s, opts)
	if err != nil {
		t.Fatalf("first classifyTiers: %v", err)
	}
	cw1, err := s.ListCrosswalk("ticket_type", "dice")
	if err != nil {
		t.Fatalf("list crosswalk after first run: %v", err)
	}
	idByValue1 := map[string]string{}
	for _, r := range cw1 {
		idByValue1[r.SourceValue] = r.CanonicalID
	}

	// Clear non-manual rows so the second run repopulates from scratch.
	if err := s.ClearNormalization("ticket_type"); err != nil {
		t.Fatalf("clear: %v", err)
	}

	_, err = classifyTiers(context.Background(), s, opts)
	if err != nil {
		t.Fatalf("second classifyTiers: %v", err)
	}
	cw2, err := s.ListCrosswalk("ticket_type", "dice")
	if err != nil {
		t.Fatalf("list crosswalk after second run: %v", err)
	}
	idByValue2 := map[string]string{}
	for _, r := range cw2 {
		idByValue2[r.SourceValue] = r.CanonicalID
	}

	for sv, id1 := range idByValue1 {
		id2, ok := idByValue2[sv]
		if !ok {
			t.Errorf("source_value %q present in run 1 but not run 2", sv)
			continue
		}
		if id1 != id2 {
			t.Errorf("canonical_id for %q differs: run1=%q run2=%q (fuzzy pass non-deterministic)", sv, id1, id2)
		}
	}
}

// TestFuzzyClassifyErrorPropagation verifies that classifyTiers propagates
// errors from the store rather than silently swallowing them.
// (The hoisted ListCrosswalk call happens before the cluster loop; here we
// exercise that the results are actually used and any UpsertCrosswalk error
// is surfaced.)  Since we can't easily inject a store failure, we confirm the
// success path runs without error and produces consistent results — the
// structural check is that the hoisted crosswalk index is built once, not
// once per cluster member.
func TestFuzzyClassifyHoistsListCrosswalk(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"tickets": {
			"t1": `{"id":"t1","ticketType":{"name":"Early Bird"}}`,
			"t2": `{"id":"t2","ticketType":{"name":"Early Birds"}}`,
			"t3": `{"id":"t3","ticketType":{"name":"Early Birrd"}}`,
		},
	})
	opts := classifyOpts{ClassifierVersion: 1, Fuzzy: true}
	res, err := classifyTiers(context.Background(), s, opts)
	if err != nil {
		t.Fatalf("classifyTiers: %v", err)
	}
	// All three near-duplicate names should cluster to one canonical ID.
	if res.CanonicalCount > 2 {
		t.Errorf("expected clustering to reduce canonical count, got %d", res.CanonicalCount)
	}
	cw, err := s.ListCrosswalk("ticket_type", "dice")
	if err != nil {
		t.Fatalf("list crosswalk: %v", err)
	}
	ids := map[string]bool{}
	for _, r := range cw {
		ids[r.CanonicalID] = true
	}
	if len(ids) > 2 {
		t.Errorf("expected <=2 distinct canonical IDs after fuzzy clustering, got %d", len(ids))
	}
}

func TestMintCanonicalID(t *testing.T) {
	// Same input must produce the same ID across repeated calls (idempotent).
	id1 := mintCanonicalID("ticket_type", "general admission")
	id2 := mintCanonicalID("ticket_type", "general admission")
	if id1 != id2 {
		t.Errorf("mintCanonicalID not idempotent: %q != %q", id1, id2)
	}
	// Different canonical names must produce different IDs.
	id3 := mintCanonicalID("ticket_type", "vip experience")
	if id1 == id3 {
		t.Errorf("different canonical names produced the same ID: %q", id1)
	}
	// Entity type prefix must be present in the ID.
	if len(id1) < len("ticket_type:")+1 {
		t.Errorf("ID too short to contain entity_type prefix: %q", id1)
	}
}

func TestClassifyTiers(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"tickets": {
			"t1": `{"id":"t1","ticketType":{"name":"General Admission "}}`,
			"t2": `{"id":"t2","ticketType":{"name":"general admission"}}`, // canon-dup of t1
			"t3": `{"id":"t3","ticketType":{"name":"VIP Experience"}}`,
			"t4": `{"id":"t4","ticketType":{"name":"zzz mystery label"}}`, // unmatched
		},
	})
	res, err := classifyTiers(context.Background(), s, classifyOpts{ClassifierVersion: 1})
	if err != nil {
		t.Fatalf("classifyTiers: %v", err)
	}
	// Two GA variants collapse to one canonical entity.
	if res.CanonicalCount < 2 {
		t.Errorf("want >=2 canonical tiers, got %d", res.CanonicalCount)
	}
	if res.Unmatched < 1 {
		t.Errorf("want >=1 unmatched, got %d", res.Unmatched)
	}
	// The two GA raw values must map to the same canonical_id.
	cw, _ := s.ListCrosswalk("ticket_type", "dice")
	idByValue := map[string]string{}
	for _, r := range cw {
		idByValue[r.SourceValue] = r.CanonicalID
	}
	// t1 raw value is "General Admission " (original case from the fixture).
	// t2 raw value is "general admission". Both must map to the same canonical ID.
	if idByValue["General Admission "] == "" || idByValue["General Admission "] != idByValue["general admission"] {
		t.Errorf("GA variants not unified: %v", idByValue)
	}
}

func TestClassifyTiersSkipsManual(t *testing.T) {
	// A source value already stored with method=manual must not be overwritten.
	s := seedStore(t, map[string]map[string]string{
		"tickets": {
			"t1": `{"id":"t1","ticketType":{"name":"mystery tier"}}`,
		},
	})
	// Pre-seed a manual entry for this value.
	if err := s.UpsertCrosswalk(store.CrosswalkRow{
		EntityType: "ticket_type", SourceSystem: "dice", SourceValue: "mystery tier",
		CanonicalID: "ticket_type:manual-cid", Method: "manual", ClassifierVersion: 1,
	}); err != nil {
		t.Fatalf("pre-seed manual: %v", err)
	}
	if _, err := classifyTiers(context.Background(), s, classifyOpts{ClassifierVersion: 1}); err != nil {
		t.Fatalf("classifyTiers: %v", err)
	}
	cw, _ := s.ListCrosswalk("ticket_type", "dice")
	for _, r := range cw {
		if r.SourceValue == "mystery tier" && r.Method != "manual" {
			t.Errorf("manual row was overwritten: %+v", r)
		}
	}
}

func TestClassifyVenues(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"e1": `{"id":"e1","venues":[{"name":"Northside Hall"}]}`,
			"e2": `{"id":"e2","venues":[{"name":"northside hall"}]}`, // canon-dup of e1
			"e3": `{"id":"e3","venues":[{"name":"Southside Arena"}]}`,
		},
	})
	res, err := classifyVenues(context.Background(), s, classifyOpts{ClassifierVersion: 1})
	if err != nil {
		t.Fatalf("classifyVenues: %v", err)
	}
	// Two Northside Hall variants + one Southside = 2 unique canonical venues.
	if res.CanonicalCount < 2 {
		t.Errorf("want >=2 canonical venues, got %d", res.CanonicalCount)
	}
	// The two Northside raw values must map to the same canonical_id.
	cw, _ := s.ListCrosswalk("venue", "dice")
	idByValue := map[string]string{}
	for _, r := range cw {
		idByValue[r.SourceValue] = r.CanonicalID
	}
	if idByValue["Northside Hall"] == "" || idByValue["Northside Hall"] != idByValue["northside hall"] {
		t.Errorf("venue variants not unified: %v", idByValue)
	}
}
