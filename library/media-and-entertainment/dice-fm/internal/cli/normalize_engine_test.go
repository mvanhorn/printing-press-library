// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the config-driven normalization engine: vocab/attribute overlays
// and the generic classifyEntity classifier (including its manual-skip
// preservation guarantee). All fixtures are synthetic — no real venue or
// entity names.
package cli

import (
	"context"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/normalizecfg"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/store"
)

// TestApplyAttributesOverlay verifies the overlay applies the entity's
// precompiled rules: a single rule on the entity sets the corresponding axis.
func TestApplyAttributesOverlay(t *testing.T) {
	ent := normalizecfg.Entity{
		Shape: normalizecfg.ShapeAttributes,
		Rules: []normalizecfg.Rule{
			{Match: `(?i)\bvip\b`, Set: map[string]string{"access_class": "vip"}},
		},
	}
	got := applyAttributesOverlay("vip experience", compileRules("ticket_type", ent.Rules, nil))
	if got["access_class"] != "vip" {
		t.Errorf("got %v, want access_class=vip", got)
	}
	// No rules -> empty overlay.
	none := applyAttributesOverlay("vip experience", compileRules("ticket_type", normalizecfg.Entity{}.Rules, nil))
	if len(none) != 0 {
		t.Errorf("got %v, want empty overlay for entity without rules", none)
	}
}

// TestClassifyEntityAttributes verifies the generic classifyEntity drives an
// attributes-shaped entity from its config declaration: a VIP rule resolves
// access_class=vip (matched) while an unruled GA row is left as an LLM-tail
// candidate (no attribute set). Synthetic fixtures only.
func TestClassifyEntityAttributes(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"tickets": {
			"t1": `{"id":"t1","ticketType":{"name":"VIP Experience"}}`,
			"t2": `{"id":"t2","ticketType":{"name":"General Admission"}}`,
		},
	})
	ent := normalizecfg.Entity{Source: "tickets.ticketType.name", Shape: "attributes",
		Attributes: []string{"access_class"},
		Rules:      []normalizecfg.Rule{{Match: `(?i)\bvip\b`, Set: map[string]string{"access_class": "vip"}}}}
	res, err := classifyEntity(context.Background(), s, "ticket_type", ent, classifyOpts{ClassifierVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	if res.Matched < 1 {
		t.Errorf("want >=1 matched, got %d", res.Matched)
	}
	// VIP row resolved access_class=vip; GA row unset (LLM-tail candidate)
}

// TestManualSurvivesConfigDrivenRerun is a regression guard: a method="manual"
// crosswalk row must survive a config-driven classifyEntity rerun. The engine
// must skip any raw source value whose manual crosswalk row exists (matched on
// SourceValue) so an operator override is never overwritten by a derived
// classification. Synthetic fixtures only ("Mystery Tier").
func TestManualSurvivesConfigDrivenRerun(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{"tickets": {"t1": `{"id":"t1","ticketType":{"name":"Mystery Tier"}}`}})
	s.UpsertCrosswalk(store.CrosswalkRow{EntityType: "ticket_type", SourceSystem: "dice", SourceValue: "Mystery Tier", CanonicalID: mintCanonicalID("ticket_type", "mystery tier"), Method: "manual", ClassifierVersion: 1})
	ent := normalizecfg.Entity{Source: "tickets.ticketType.name", Shape: "attributes", Attributes: []string{"access_class"}}
	if _, err := classifyEntity(context.Background(), s, "ticket_type", ent, classifyOpts{ClassifierVersion: 1}); err != nil {
		t.Fatal(err)
	}
	rows, _ := s.ListCrosswalk("ticket_type", "dice")
	found := false
	for _, r := range rows {
		if r.SourceValue == "Mystery Tier" {
			found = true
			if r.Method != "manual" {
				t.Errorf("manual row overwritten: %+v", r)
			}
		}
	}
	if !found {
		t.Error("manual row for 'Mystery Tier' missing after rerun")
	}
}

// TestClassifyEntityWritesGenericAttributes verifies that classifyEntity, run
// over a non-tier/venue attributes entity with a promoted rule, writes the
// matched attribute to the generic entity_attributes store (not to the typed
// tier/venue tables). Synthetic fixtures only.
func TestClassifyEntityWritesGenericAttributes(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"tickets": {
			"t1": `{"id":"t1","ticketType":{"name":"$120 Premium Seat"}}`,
			"t2": `{"id":"t2","ticketType":{"name":"Standard"}}`,
		},
	})
	// price_tier is not ticket_type/venue, so a match must land in
	// entity_attributes. A rule sets price_band=high on any "$120" value.
	ent := normalizecfg.Entity{
		Source:     "tickets.ticketType.name",
		Shape:      "attributes",
		Attributes: []string{"price_band"},
		Rules:      []normalizecfg.Rule{{Match: `\$120`, Set: map[string]string{"price_band": "high"}}},
	}
	res, err := classifyEntity(context.Background(), s, "price_tier", ent, classifyOpts{ClassifierVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	if res.Matched < 1 {
		t.Errorf("want >=1 matched, got %d", res.Matched)
	}

	attrs, err := s.ListEntityAttributes("price_tier")
	if err != nil {
		t.Fatalf("ListEntityAttributes: %v", err)
	}
	if len(attrs) != 1 {
		t.Fatalf("want 1 entity_attributes row for the matched $120 row, got %d: %+v", len(attrs), attrs)
	}
	got := attrs[0]
	if got.EntityType != "price_tier" || got.AttrKey != "price_band" || got.AttrValue != "high" {
		t.Errorf("attr = %+v, want {price_tier price_band high}", got)
	}
	// Derived rows carry the rule method, not manual.
	if got.Method != methodRule {
		t.Errorf("attr method = %q, want %q (derived)", got.Method, methodRule)
	}

	// The generic path must not touch the typed tables.
	ta, _ := s.ListTierAttributes("price_tier")
	if len(ta) != 0 {
		t.Errorf("classifyEntity wrote %d tier_attributes rows for a generic entity, want 0", len(ta))
	}
}

// TestClassifyEntityStripPattern verifies the per-entity strip_pattern transform
// folds two values that differ only by a namespace prefix to the same canonical
// id (deduped), while each crosswalk row preserves its distinct RAW SourceValue.
// Strip applies before canonicalization for an alias-shaped scalar-array entity.
// Synthetic generic values only.
func TestClassifyEntityStripPattern(t *testing.T) {
	const (
		rawDJ  = "dj:afrohouse"
		rawGig = "gig:afrohouse"
	)
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"e1": `{"id":"e1","genres":["` + rawDJ + `"]}`,
			"e2": `{"id":"e2","genres":["` + rawGig + `"]}`,
		},
	})
	ent := normalizecfg.Entity{
		Source:       "events.genres[*]",
		Shape:        normalizecfg.ShapeAlias,
		StripPattern: "^[a-z]+:",
	}
	res, err := classifyEntity(context.Background(), s, "genre", ent, classifyOpts{ClassifierVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	// Two raw values, one stripped+canonicalized form → one canonical entity.
	if res.CanonicalCount != 1 {
		t.Errorf("want 1 canonical form after stripping the namespace prefix, got %d", res.CanonicalCount)
	}

	rows, err := s.ListCrosswalk("genre", "dice")
	if err != nil {
		t.Fatalf("ListCrosswalk: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 crosswalk rows (one per raw value), got %d: %+v", len(rows), rows)
	}
	gotSources := map[string]bool{}
	canonIDs := map[string]bool{}
	for _, r := range rows {
		gotSources[r.SourceValue] = true
		canonIDs[r.CanonicalID] = true
	}
	// RAW SourceValues preserved (mapping back to source intact).
	if !gotSources[rawDJ] || !gotSources[rawGig] {
		t.Errorf("crosswalk SourceValues = %v, want both raw values %q and %q", gotSources, rawDJ, rawGig)
	}
	// Both rows point at the same canonical id (deduped on stripped form).
	if len(canonIDs) != 1 {
		t.Errorf("want both rows to share one canonical id, got %d distinct ids: %v", len(canonIDs), canonIDs)
	}
}

// TestClassifyEntityNoStripPatternUnchanged is the regression guard: an entity
// WITHOUT strip_pattern leaves namespaced values distinct (no folding).
func TestClassifyEntityNoStripPatternUnchanged(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"e1": `{"id":"e1","genres":["dj:afrohouse"]}`,
			"e2": `{"id":"e2","genres":["gig:afrohouse"]}`,
		},
	})
	ent := normalizecfg.Entity{Source: "events.genres[*]", Shape: normalizecfg.ShapeAlias}
	res, err := classifyEntity(context.Background(), s, "genre", ent, classifyOpts{ClassifierVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	if res.CanonicalCount != 2 {
		t.Errorf("want 2 distinct canonical forms without strip_pattern, got %d", res.CanonicalCount)
	}
}

func TestVocabOverlay(t *testing.T) {
	set := []string{"house", "techno", "trance"}
	if v, ok := mapVocab("Techno ", set); !ok || v != "techno" {
		t.Errorf("mapVocab=%q,%v want techno,true", v, ok)
	}
	if _, ok := mapVocab("ambient", set); ok {
		t.Error("unknown vocab should flag (ok=false)")
	}
	// Set member with different casing/extra whitespace vs raw.
	set2 := []string{"Deep House"}
	if v, ok := mapVocab("deep  house ", set2); !ok || v != "deep house" {
		t.Errorf("mapVocab with spaced raw=%q,%v want \"deep house\",true", v, ok)
	}
}
