// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package recipes_test

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn/recipes"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

// teachOne writes one search_learnings row at source="taught" with
// the QueryEntities pre-populated to match what the live teach path
// would do (run normalize.Normalize over the query and pass through
// the entities). This is the same shape the U10 Extract walks.
func teachOne(t *testing.T, s *store.Store, query, resourceType, resourceID string) {
	t.Helper()
	normalized := learn.Normalize(query, learn.DefaultPredictionGoatConfig())
	if _, _, err := s.UpsertLearning(store.UpsertLearningInput{
		Query:         query,
		QueryEntities: normalized.Entities,
		ResourceID:    resourceID,
		ResourceType:  resourceType,
		Source:        store.LearningSourceTaught,
	}); err != nil {
		t.Fatalf("teach %q -> %s: %v", query, resourceID, err)
	}
}

// TestExtract_KalshiCountryTicker_ExactSubstitute is the flagship
// "Portugal + USA generalize to England" story. Two teaches with the
// same query shape and a country-iso2 swap in the ticker should
// produce a substitute-strategy recipe bound to country_iso2.
func TestExtract_KalshiCountryTicker_ExactSubstitute(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)

	teachOne(t, s, "odds Portugal wins world cup", "kalshi_markets", "KXMENWORLDCUP-26-PT")
	teachOne(t, s, "odds USA wins world cup", "kalshi_markets", "KXMENWORLDCUP-26-US")

	created, err := recipes.Extract(s.DB())
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if created < 1 {
		t.Errorf("Extract created %d recipes, want >= 1", created)
	}

	rows, err := recipes.List(s.DB(), recipes.ListFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) < 1 {
		t.Fatalf("want at least 1 recipe row, got %d", len(rows))
	}

	// Find the country_iso2 substitute recipe.
	var got *recipes.Recipe
	for i := range rows {
		if rows[i].EntityKind == "country_iso2" && rows[i].Strategy == recipes.StrategySubstitute {
			got = &rows[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("no country_iso2/substitute recipe found in %+v", rows)
	}
	if got.ResourceTemplate != "KXMENWORLDCUP-26-{entity:country_iso2}" {
		t.Errorf("resource_template = %q, want KXMENWORLDCUP-26-{entity:country_iso2}", got.ResourceTemplate)
	}
	if got.ResourceType != "kalshi_markets" {
		t.Errorf("resource_type = %q, want kalshi_markets", got.ResourceType)
	}
	if got.Source != recipes.SourceInferred {
		t.Errorf("source = %q, want inferred", got.Source)
	}
}

// TestExtract_PolymarketSlug_PrefixSearch covers the second strategy:
// two Polymarket slugs that share a literal core but carry an
// unpredictable trailing numeric segment. Extract should bind the
// lowercase kind and emit substitute-then-search-prefix.
func TestExtract_PolymarketSlug_PrefixSearch(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)

	teachOne(t, s, "odds Portugal wins world cup", "markets", "will-portugal-win-the-2026-fifa-world-cup-912")
	teachOne(t, s, "odds USA wins world cup", "markets", "will-usa-win-the-2026-fifa-world-cup-467")

	if _, err := recipes.Extract(s.DB()); err != nil {
		t.Fatalf("extract: %v", err)
	}

	rows, err := recipes.List(s.DB(), recipes.ListFilter{Strategy: recipes.StrategySubstituteThenSearchPrefix})
	if err != nil {
		t.Fatalf("list prefix: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want exactly 1 prefix recipe, got %d (rows=%+v)", len(rows), rows)
	}
	got := rows[0]
	// Either the table-backed country_lowercase kind or the
	// computed lowercase kind produces the right substitution
	// value for these countries; the inference engine prefers
	// table-backed when both work because it carries stronger
	// "user meant this specific alias" semantics.
	if got.EntityKind != "lowercase" && got.EntityKind != "country_lowercase" {
		t.Errorf("entity_kind = %q, want lowercase or country_lowercase", got.EntityKind)
	}
	if got.ResourceTemplate == "" || got.ResourceTemplate[len(got.ResourceTemplate)-1] != '*' {
		t.Errorf("resource_template should end with '*'; got %q", got.ResourceTemplate)
	}
}

// TestExtract_NoPattern_NoRecipe asserts that two unrelated teaches
// don't get fused into a bogus recipe. Different query patterns
// AND different resource shapes = no group.
func TestExtract_NoPattern_NoRecipe(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)

	teachOne(t, s, "odds Portugal wins world cup", "kalshi_markets", "KXMENWORLDCUP-26-PT")
	teachOne(t, s, "trending crypto markets today", "markets", "trending-crypto-2026")

	if _, err := recipes.Extract(s.DB()); err != nil {
		t.Fatalf("extract: %v", err)
	}
	rows, err := recipes.List(s.DB(), recipes.ListFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("unrelated teaches should not yield a recipe; got %+v", rows)
	}
}

// TestExtract_SingleTeach_NoRecipe asserts a lone teach doesn't
// spawn a one-sample recipe.
func TestExtract_SingleTeach_NoRecipe(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)

	teachOne(t, s, "odds Portugal wins world cup", "kalshi_markets", "KXMENWORLDCUP-26-PT")

	if _, err := recipes.Extract(s.DB()); err != nil {
		t.Fatalf("extract: %v", err)
	}
	rows, err := recipes.List(s.DB(), recipes.ListFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("single teach should not yield a recipe; got %+v", rows)
	}
}

// TestExtract_MultiEntity_NoSingleSlotRecipe is the multi-entity
// anyMulti guard: a pair of multi-entity rows that share a structural
// stem must not synthesize a single-slot template. tryExactBinding /
// tryPrefixBinding both index queryEntities[0] only — emitting a
// single-slot recipe against a multi-entity row would silently bake
// the second entity into prefix/suffix as a literal and over-match
// future queries. The guard skips inference for groups whose members
// carry more than one entity; multi-entity templating is future work.
//
// Multi-entity rows are still allowed past the entity-count check at
// collection time so they participate in grouping (the structural form
// strips all entities, so a multi-entity row shares the same stem as
// the single-entity rows in its bucket). That's what enables future
// multi-entity binding without re-bucketing teaches.
func TestExtract_MultiEntity_NoSingleSlotRecipe(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)

	// Two multi-entity teaches with the same shape. Both have len
	// (QueryEntities) > 1, so the anyMulti guard fires and inference
	// is skipped.
	if _, _, err := s.UpsertLearning(store.UpsertLearningInput{
		Query:         "tonight Mariners vs Yankees",
		QueryEntities: []string{"Mariners", "Yankees"},
		ResourceID:    "event-mariners-yankees-2026-05-26",
		ResourceType:  "events",
		Source:        store.LearningSourceTaught,
	}); err != nil {
		t.Fatalf("teach 1: %v", err)
	}
	if _, _, err := s.UpsertLearning(store.UpsertLearningInput{
		Query:         "tonight Mets vs Cubs",
		QueryEntities: []string{"Mets", "Cubs"},
		ResourceID:    "event-mets-cubs-2026-05-26",
		ResourceType:  "events",
		Source:        store.LearningSourceTaught,
	}); err != nil {
		t.Fatalf("teach 2: %v", err)
	}

	if _, err := recipes.Extract(s.DB()); err != nil {
		t.Fatalf("extract: %v", err)
	}
	rows, err := recipes.List(s.DB(), recipes.ListFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("multi-entity teaches should not emit a single-slot recipe; got %+v", rows)
	}
}

// TestExtract_MultiEntityPoisonsSingleEntityGroup verifies the
// anyMulti guard checks every member of a group, not just the first.
// A multi-entity row that lands in the same structural bucket as two
// single-entity rows must block inference for that whole group.
// Otherwise a stray multi-entity teach in a shape-peer cluster would
// produce a single-slot template that drops the multi-entity row's
// second entity on the floor.
func TestExtract_MultiEntityPoisonsSingleEntityGroup(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)

	// Two single-entity rows that would normally pair into a
	// substitute-strategy recipe.
	teachOne(t, s, "odds Portugal wins world cup", "kalshi_markets", "KXMENWORLDCUP-26-PT")
	teachOne(t, s, "odds USA wins world cup", "kalshi_markets", "KXMENWORLDCUP-26-US")

	// A multi-entity teach in the same structural shape: after
	// stripping both entities from the query_pattern, the stem is
	// identical to the two single-entity rows ("cup wins world"
	// after the "odds" stopword drops). Hand-build the entities
	// slice so the row is unambiguously multi-entity regardless of
	// what the extractor would have produced. The structural-form
	// equality forces it into the same bucket; the anyMulti guard
	// must still fire and skip inference for the whole group.
	if _, _, err := s.UpsertLearning(store.UpsertLearningInput{
		Query:         "odds England Brazil wins world cup",
		QueryEntities: []string{"England", "Brazil"},
		ResourceID:    "KXMENWORLDCUP-26-GB-BR",
		ResourceType:  "kalshi_markets",
		Source:        store.LearningSourceTaught,
	}); err != nil {
		t.Fatalf("teach multi: %v", err)
	}

	if _, err := recipes.Extract(s.DB()); err != nil {
		t.Fatalf("extract: %v", err)
	}

	// The kalshi_markets group now has 3 members but one is multi-
	// entity, so anyMulti fires and we get no recipe rows.
	rows, err := recipes.List(s.DB(), recipes.ListFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, r := range rows {
		if r.ResourceType == "kalshi_markets" {
			t.Errorf("kalshi_markets group should be skipped due to multi-entity member; got %+v", r)
		}
	}
}

// TestExtract_SingleEntityPathStillWorks is the regression guard for
// U4: after generalizing queryStructural / buildQueryTemplate to take
// an entity slice, two single-entity teaches in the same shape must
// still produce a template that a fresh single-entity query in the
// same shape would match via the apply path. This is structurally
// covered by TestExtract_KalshiCountryTicker_ExactSubstitute today
// but called out explicitly here as the "single-slot path is not
// regressed" check the U4 plan asks for.
func TestExtract_SingleEntityPathStillWorks(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)

	teachOne(t, s, "odds Portugal wins world cup", "kalshi_markets", "KXMENWORLDCUP-26-PT")
	teachOne(t, s, "odds USA wins world cup", "kalshi_markets", "KXMENWORLDCUP-26-US")

	if _, err := recipes.Extract(s.DB()); err != nil {
		t.Fatalf("extract: %v", err)
	}

	rows, err := recipes.List(s.DB(), recipes.ListFilter{ResourceType: "kalshi_markets"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) == 0 {
		t.Fatalf("two single-entity teaches should produce >= 1 recipe (regression: U4 multi-entity refactor must not break single-entity path)")
	}
	// The query template must carry exactly one {entity} placeholder
	// and the non-entity stem tokens, sorted. We assert the shape
	// rather than an exact string so the test stays resilient to
	// future stem additions.
	var got *recipes.Recipe
	for i := range rows {
		if rows[i].EntityKind == "country_iso2" && rows[i].Strategy == recipes.StrategySubstitute {
			got = &rows[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("expected a country_iso2 substitute recipe; got %+v", rows)
	}
	// The normalize layer drops "odds" as a domain stopword, so the
	// non-entity stem is "cup wins world" after stripping the
	// entity. Template is the stem + a single {entity} placeholder,
	// all alphabetized.
	wantTemplate := "cup wins world {entity}"
	if got.QueryTemplate != wantTemplate {
		t.Errorf("query_template = %q, want %q", got.QueryTemplate, wantTemplate)
	}
}

// TestExtract_Idempotent asserts that running Extract twice over the
// same data does not create duplicate rows. The second pass should
// bump confidence on the existing recipe and leave the row count
// unchanged.
func TestExtract_Idempotent(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)

	teachOne(t, s, "odds Portugal wins world cup", "kalshi_markets", "KXMENWORLDCUP-26-PT")
	teachOne(t, s, "odds USA wins world cup", "kalshi_markets", "KXMENWORLDCUP-26-US")

	if _, err := recipes.Extract(s.DB()); err != nil {
		t.Fatalf("first extract: %v", err)
	}
	firstRows, err := recipes.List(s.DB(), recipes.ListFilter{})
	if err != nil {
		t.Fatalf("list after first extract: %v", err)
	}
	firstCount := len(firstRows)
	if firstCount == 0 {
		t.Fatalf("first extract produced 0 rows; subsequent assertions meaningless")
	}

	if _, err := recipes.Extract(s.DB()); err != nil {
		t.Fatalf("second extract: %v", err)
	}
	secondRows, err := recipes.List(s.DB(), recipes.ListFilter{})
	if err != nil {
		t.Fatalf("list after second extract: %v", err)
	}
	if len(secondRows) != firstCount {
		t.Errorf("recipe count drifted across Extract calls: first=%d second=%d", firstCount, len(secondRows))
	}
	// Confidence should bump on the matching row.
	for _, r := range secondRows {
		if r.Confidence < firstRows[0].Confidence {
			t.Errorf("expected confidence to increase or stay, got %d (first=%d)", r.Confidence, firstRows[0].Confidence)
		}
	}
}
