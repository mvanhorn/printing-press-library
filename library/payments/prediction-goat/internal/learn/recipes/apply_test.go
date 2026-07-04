// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package recipes_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn/recipes"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

// seedResource upserts a row into the resources table with the
// given JSON payload. Apply's verifyCandidate path reads from this
// table to confirm a substituted candidate is real.
func seedResource(t *testing.T, s *store.Store, resourceType, id string, payload map[string]any) {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if err := s.Upsert(resourceType, id, data); err != nil {
		t.Fatalf("upsert resource (%s/%s): %v", resourceType, id, err)
	}
}

// normalizedFor returns the (NonEntityNormalized, Entities) pair
// the Apply path expects from its caller. Use the production
// normalize helper so the test stays in sync with the recall
// path's contract.
func normalizedFor(query string) (string, []string) {
	n := learn.Normalize(query, learn.DefaultPredictionGoatConfig())
	return n.NonEntityNormalized, n.Entities
}

// TestApply_KalshiSubstituteVerified is the flagship apply story:
// a Portugal + USA recipe is in the table; the live query asks
// about England; Apply should substitute England->GB and verify
// against the seeded KXMENWORLDCUP-26-GB row.
func TestApply_KalshiSubstituteVerified(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)

	// Seed England's resource (the recall layer's lookup target).
	seedResource(t, s, "kalshi_markets", "KXMENWORLDCUP-26-GB", map[string]any{
		"title":  "FIFA Men's World Cup 2026 Winner",
		"ticker": "KXMENWORLDCUP-26-GB",
	})

	// Plant the recipe directly (Extract is tested separately).
	if _, _, err := recipes.Upsert(s.DB(), recipes.Recipe{
		QueryTemplate:    "{entity} cup wins world",
		ResourceTemplate: "KXMENWORLDCUP-26-{entity:country_iso2}",
		ResourceType:     "kalshi_markets",
		Venue:            "kalshi",
		Strategy:         recipes.StrategySubstitute,
		EntityKind:       "country_iso2",
		Confidence:       2,
		Source:           recipes.SourceInferred,
	}); err != nil {
		t.Fatalf("upsert recipe: %v", err)
	}

	non, ents := normalizedFor("odds England wins world cup")
	hits, err := recipes.Apply(context.Background(), s.DB(), "odds England wins world cup", non, ents, recipes.Opts{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("want 1 hit, got %d: %+v", len(hits), hits)
	}
	if hits[0].ResourceID != "KXMENWORLDCUP-26-GB" {
		t.Errorf("hit.ResourceID = %q, want KXMENWORLDCUP-26-GB", hits[0].ResourceID)
	}
	if hits[0].Source != "recipe" {
		t.Errorf("hit.Source = %q, want recipe", hits[0].Source)
	}
	if hits[0].EntityMatch != "exact" {
		t.Errorf("hit.EntityMatch = %q, want exact", hits[0].EntityMatch)
	}
}

// TestApply_LookupMiss_NoHit covers the structured recipe_miss path:
// the recipe exists, the live query has a matching entity token, but
// the entity_lookups table has no value for that token under the
// recipe's kind. Apply must skip silently rather than emit a
// hallucinated candidate.
func TestApply_LookupMiss_NoHit(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)

	if _, _, err := recipes.Upsert(s.DB(), recipes.Recipe{
		QueryTemplate:    "{entity} cup wins world",
		ResourceTemplate: "KXMENWORLDCUP-26-{entity:country_iso2}",
		ResourceType:     "kalshi_markets",
		Strategy:         recipes.StrategySubstitute,
		EntityKind:       "country_iso2",
		Confidence:       2,
		Source:           recipes.SourceInferred,
	}); err != nil {
		t.Fatalf("upsert recipe: %v", err)
	}

	// Use an entity that has no country_iso2 row in entity_lookups.
	non, ents := normalizedFor("odds Atlantis wins world cup")
	hits, err := recipes.Apply(context.Background(), s.DB(), "odds Atlantis wins world cup", non, ents, recipes.Opts{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("want 0 hits on lookup miss, got %d: %+v", len(hits), hits)
	}
}

// TestApply_NoVerify_SkipsResourcesLookup confirms the test/dry-run
// path: Apply returns the substituted candidate verbatim without
// consulting the resources table.
func TestApply_NoVerify_SkipsResourcesLookup(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)

	// No seeded resource row for KXMENWORLDCUP-26-GB. With verify
	// on this would return zero hits; with NoVerify it should still
	// substitute.
	if _, _, err := recipes.Upsert(s.DB(), recipes.Recipe{
		QueryTemplate:    "{entity} cup wins world",
		ResourceTemplate: "KXMENWORLDCUP-26-{entity:country_iso2}",
		ResourceType:     "kalshi_markets",
		Strategy:         recipes.StrategySubstitute,
		EntityKind:       "country_iso2",
		Confidence:       2,
		Source:           recipes.SourceInferred,
	}); err != nil {
		t.Fatalf("upsert recipe: %v", err)
	}

	non, ents := normalizedFor("odds England wins world cup")
	hits, err := recipes.Apply(context.Background(), s.DB(), "odds England wins world cup", non, ents, recipes.Opts{NoVerify: true})
	if err != nil {
		t.Fatalf("apply noverify: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("noverify should produce 1 hit, got %d", len(hits))
	}
	if hits[0].ResourceID != "KXMENWORLDCUP-26-GB" {
		t.Errorf("noverify hit.ResourceID = %q", hits[0].ResourceID)
	}
}

// TestApply_PrefixSearchRecipe covers the substitute-then-search-
// prefix strategy: the candidate ends with "*" and Apply runs a
// prefix LIKE search against the resources table.
func TestApply_PrefixSearchRecipe(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)

	// Seed England's Polymarket slug (carries a trailing numeric ID).
	seedResource(t, s, "markets", "will-england-win-the-2026-fifa-world-cup-318", map[string]any{
		"question": "Will England win the 2026 FIFA World Cup?",
		"slug":     "will-england-win-the-2026-fifa-world-cup-318",
	})

	if _, _, err := recipes.Upsert(s.DB(), recipes.Recipe{
		QueryTemplate:    "{entity} cup wins world",
		ResourceTemplate: "will-{entity:country_lowercase}-win-the-2026-fifa-world-cup-*",
		ResourceType:     "markets",
		Venue:            "polymarket",
		Strategy:         recipes.StrategySubstituteThenSearchPrefix,
		EntityKind:       "country_lowercase",
		Confidence:       2,
		Source:           recipes.SourceInferred,
	}); err != nil {
		t.Fatalf("upsert prefix recipe: %v", err)
	}

	non, ents := normalizedFor("odds England wins world cup")
	hits, err := recipes.Apply(context.Background(), s.DB(), "odds England wins world cup", non, ents, recipes.Opts{})
	if err != nil {
		t.Fatalf("apply prefix: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("want 1 hit, got %d", len(hits))
	}
	if hits[0].ResourceID != "will-england-win-the-2026-fifa-world-cup-318" {
		t.Errorf("prefix hit.ResourceID = %q", hits[0].ResourceID)
	}
	if hits[0].Source != "recipe" {
		t.Errorf("hit.Source = %q, want recipe", hits[0].Source)
	}
}

// TestApply_NoEntities_NoHits covers the early exit: a query with
// no entity tokens (just non-entity content like "trending markets")
// can't trigger any recipe substitution.
func TestApply_NoEntities_NoHits(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)

	if _, _, err := recipes.Upsert(s.DB(), recipes.Recipe{
		QueryTemplate:    "{entity} cup wins world",
		ResourceTemplate: "KXMENWORLDCUP-26-{entity:country_iso2}",
		ResourceType:     "kalshi_markets",
		Strategy:         recipes.StrategySubstitute,
		EntityKind:       "country_iso2",
		Confidence:       2,
		Source:           recipes.SourceInferred,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	hits, err := recipes.Apply(context.Background(), s.DB(), "trending markets", "markets trending", nil, recipes.Opts{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("want 0 hits with no entities, got %d", len(hits))
	}
}

// TestApply_BelowJaccardThreshold_NoHit asserts that a query whose
// non-entity tokens don't overlap with the recipe's query_template
// doesn't trigger a substitution even when the entity has a lookup.
func TestApply_BelowJaccardThreshold_NoHit(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)

	if _, _, err := recipes.Upsert(s.DB(), recipes.Recipe{
		QueryTemplate:    "{entity} cup wins world",
		ResourceTemplate: "KXMENWORLDCUP-26-{entity:country_iso2}",
		ResourceType:     "kalshi_markets",
		Strategy:         recipes.StrategySubstitute,
		EntityKind:       "country_iso2",
		Confidence:       2,
		Source:           recipes.SourceInferred,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Query about Portugal's election odds — entity overlap (Portugal
	// has country_iso2=PT), but no world-cup-shaped non-entity
	// overlap with the recipe.
	non, ents := normalizedFor("Portugal election results today")
	hits, err := recipes.Apply(context.Background(), s.DB(), "Portugal election results today", non, ents, recipes.Opts{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("want 0 hits below Jaccard threshold, got %d: %+v", len(hits), hits)
	}
}
