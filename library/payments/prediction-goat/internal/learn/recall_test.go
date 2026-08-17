// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package learn_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn/recipes"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

// openRecallStore opens a fresh store at a temp path so each test runs
// against an isolated DB. Returns the open store; the cleanup hook
// closes it at test end.
func openRecallStore(t *testing.T) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "recall.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// seedResource upserts a row into the resources table with the supplied
// resource_type, id, and JSON content. The JSON content is built from
// the provided map so individual tests can vary the entity-bearing
// fields without rebuilding the marshal call by hand.
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

// seedLearning writes one learning row via UpsertLearning. The store's
// UpsertLearning normalizes the query before storage; the recall path
// re-normalizes the live query against the same vocabulary so the
// stored query_pattern is the comparison surface.
func seedLearning(t *testing.T, s *store.Store, query, resourceType, resourceID string) {
	t.Helper()
	if _, _, err := s.UpsertLearning(store.UpsertLearningInput{
		Query:        query,
		ResourceID:   resourceID,
		ResourceType: resourceType,
		Source:       store.LearningSourceTaught,
	}); err != nil {
		t.Fatalf("upsert learning (%s -> %s): %v", query, resourceID, err)
	}
}

func TestRecall_ColdQueryReturnsEmptyEnvelope(t *testing.T) {
	s := openRecallStore(t)
	got, err := learn.Recall(context.Background(), s.DB(), "what are the odds USA wins world cup", learn.Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if got.Found {
		t.Errorf("cold query should be found=false; got %+v", got)
	}
	if len(got.Results) != 0 {
		t.Errorf("cold query should have empty results; got %+v", got.Results)
	}
	if got.Normalized == "" {
		t.Errorf("normalized should be populated even on cold query")
	}
	if !sliceContains(got.QueryEntities, "USA") {
		t.Errorf("query entities should contain USA; got %v", got.QueryEntities)
	}
}

func TestRecall_EnglandAgainstPortugalLearning_FiltersToMismatches(t *testing.T) {
	// The flagship England-vs-Portugal regression from the plan.
	s := openRecallStore(t)
	// Seed: the resource the Portugal learning points at.
	seedResource(t, s, "kalshi_markets", "KXMENWORLDCUP-26-PT", map[string]any{
		"title":         "FIFA Men's World Cup 2026 Winner",
		"yes_sub_title": "Portugal",
		"ticker":        "KXMENWORLDCUP-26-PT",
		"event_ticker":  "KXMENWORLDCUP-26",
	})
	// Seed: the Portugal learning.
	seedLearning(t, s, "odds Portugal wins world cup", "kalshi_markets", "KXMENWORLDCUP-26-PT")

	// Default (mismatches hidden): England query returns found=false.
	got, err := learn.Recall(context.Background(), s.DB(), "odds England wins world cup", learn.Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if got.Found {
		t.Errorf("England query against Portugal learning: want found=false, got %+v", got)
	}
	if len(got.Results) != 0 {
		t.Errorf("default results should be empty (Portugal hit filtered to mismatches); got %+v", got.Results)
	}
	if len(got.Mismatches) != 0 {
		t.Errorf("default envelope should hide mismatches; got %+v", got.Mismatches)
	}

	// With DebugMismatches: Portugal row surfaces under mismatches.
	gotDebug, err := learn.Recall(context.Background(), s.DB(), "odds England wins world cup", learn.Opts{DebugMismatches: true})
	if err != nil {
		t.Fatalf("recall debug: %v", err)
	}
	if gotDebug.Found {
		t.Errorf("debug-on: still want found=false (Results bucket unchanged); got %+v", gotDebug)
	}
	if len(gotDebug.Mismatches) != 1 {
		t.Fatalf("debug-on: want 1 mismatch row, got %d (%+v)", len(gotDebug.Mismatches), gotDebug.Mismatches)
	}
	if gotDebug.Mismatches[0].EntityMatch != learn.EntityMatchMismatch {
		t.Errorf("mismatch row: want entity_match=mismatch, got %q", gotDebug.Mismatches[0].EntityMatch)
	}
}

func TestRecall_ParentEventWithMatchingChild_AttachesWarning(t *testing.T) {
	s := openRecallStore(t)
	// Parent event resource.
	seedResource(t, s, "kalshi_events", "KXMENWORLDCUP-26", map[string]any{
		"title":         "2026 Men's World Cup Winner",
		"event_ticker":  "KXMENWORLDCUP-26",
		"series_ticker": "KXMENWORLDCUP",
	})
	// Child market with USA subtitle.
	seedResource(t, s, "kalshi_markets", "KXMENWORLDCUP-26-US", map[string]any{
		"title":         "FIFA Men's World Cup 2026 Winner",
		"yes_sub_title": "USA",
		"ticker":        "KXMENWORLDCUP-26-US",
		"event_ticker":  "KXMENWORLDCUP-26",
	})
	// The (wrong-shape) learning points at the parent.
	seedLearning(t, s, "odds USA wins world cup", "kalshi_events", "KXMENWORLDCUP-26")

	got, err := learn.Recall(context.Background(), s.DB(), "odds USA wins world cup", learn.Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if !got.Found || len(got.Results) != 1 {
		t.Fatalf("want one result for parent event with matching child; got %+v", got)
	}
	hit := got.Results[0]
	if hit.EntityMatch != learn.EntityMatchExact {
		t.Errorf("parent w/ matching child: want entity_match=exact (promoted), got %q", hit.EntityMatch)
	}
	// Warning should reference the child ticker.
	if !hasWarningPrefix(hit.Warnings, learn.WarningParentEventWhenChildExists) {
		t.Errorf("want %s warning naming child; got warnings=%v", learn.WarningParentEventWhenChildExists, hit.Warnings)
	}
}

func TestRecall_NoEntitiesEitherSide_PartialMatch(t *testing.T) {
	s := openRecallStore(t)
	seedResource(t, s, "kalshi_markets", "KXFED-RATECUT", map[string]any{
		"title":         "Federal Reserve Rate Cut",
		"yes_sub_title": "",
		"ticker":        "KXFED-RATECUT",
	})
	seedLearning(t, s, "rate cut", "kalshi_markets", "KXFED-RATECUT")

	got, err := learn.Recall(context.Background(), s.DB(), "rate cut", learn.Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if !got.Found || len(got.Results) == 0 {
		t.Fatalf("want at least one result for matching categorical query; got %+v", got)
	}
	// The KXFED-RATECUT title carries "Federal" + "Reserve" + "Rate" + "Cut" as entities (capitalized).
	// Query "rate cut" has no entities. Expect partial.
	if got.Results[0].EntityMatch != learn.EntityMatchPartial {
		t.Errorf("query w/o entities against resource w/ entities: want partial, got %q", got.Results[0].EntityMatch)
	}
}

func TestRecall_RankingPrefersExactThenConfidence(t *testing.T) {
	s := openRecallStore(t)
	// Three resources. KXA + KXB are both USA-tagged (exact match for the
	// query); KXC has no entity-bearing fields (partial fallback).
	seedResource(t, s, "kalshi_markets", "KXA-USA", map[string]any{
		"title":         "World Cup Winner",
		"yes_sub_title": "USA",
		"ticker":        "KXA-USA",
	})
	seedResource(t, s, "kalshi_markets", "KXB-USA", map[string]any{
		"title":         "World Cup Winner",
		"yes_sub_title": "USA",
		"ticker":        "KXB-USA",
	})
	// Categorical-only resource: no entity fields at all (no title, no
	// subtitle, just an opaque id). Entity extraction yields nothing,
	// so the resource side is empty and ClassifyEntityMatch returns
	// partial against the query's [USA].
	seedResource(t, s, "kalshi_markets", "KXC-NUMERIC-1", map[string]any{
		// Title written intentionally as digits-only so it carries no
		// entity tokens. Empty string for yes_sub_title.
		"yes_sub_title": "",
	})
	// KXA gets a single teach (confidence=1, entity_match=exact).
	seedLearning(t, s, "USA wins world cup", "kalshi_markets", "KXA-USA")
	// KXB gets three teaches (confidence=3, entity_match=exact). Should
	// rank above KXA on confidence within the exact bucket.
	seedLearning(t, s, "USA wins world cup", "kalshi_markets", "KXB-USA")
	seedLearning(t, s, "USA wins world cup", "kalshi_markets", "KXB-USA")
	seedLearning(t, s, "USA wins world cup", "kalshi_markets", "KXB-USA")
	// KXC gets three teaches (confidence=3, entity_match=partial). Even
	// at higher confidence, it should rank below exact matches.
	seedLearning(t, s, "USA wins world cup", "kalshi_markets", "KXC-NUMERIC-1")
	seedLearning(t, s, "USA wins world cup", "kalshi_markets", "KXC-NUMERIC-1")
	seedLearning(t, s, "USA wins world cup", "kalshi_markets", "KXC-NUMERIC-1")

	got, err := learn.Recall(context.Background(), s.DB(), "USA wins world cup", learn.Opts{Limit: 10})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if len(got.Results) < 3 {
		t.Fatalf("want 3 hits in results; got %d (%+v)", len(got.Results), got.Results)
	}
	// Ranking contract: exact > partial; within exact, confidence DESC.
	if got.Results[0].ResourceID != "KXB-USA" {
		t.Errorf("position 0: want KXB-USA (exact, conf=3), got %s (entity_match=%s confidence=%d)",
			got.Results[0].ResourceID, got.Results[0].EntityMatch, got.Results[0].Confidence)
	}
	if got.Results[1].ResourceID != "KXA-USA" {
		t.Errorf("position 1: want KXA-USA (exact, conf=1), got %s (entity_match=%s confidence=%d)",
			got.Results[1].ResourceID, got.Results[1].EntityMatch, got.Results[1].Confidence)
	}
	if got.Results[2].ResourceID != "KXC-NUMERIC-1" {
		t.Errorf("position 2: want KXC-NUMERIC-1 (partial, conf=3 — ranks below exact), got %s (entity_match=%s confidence=%d)",
			got.Results[2].ResourceID, got.Results[2].EntityMatch, got.Results[2].Confidence)
	}
}

func TestRecall_NoLearn_ReturnsEmptyEnvelope(t *testing.T) {
	s := openRecallStore(t)
	seedResource(t, s, "kalshi_markets", "KX-X", map[string]any{
		"yes_sub_title": "USA",
	})
	seedLearning(t, s, "USA wins world cup", "kalshi_markets", "KX-X")

	got, err := learn.Recall(context.Background(), s.DB(), "USA wins world cup", learn.Opts{NoLearn: true})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if got.Found || len(got.Results) != 0 {
		t.Errorf("NoLearn should suppress results; got %+v", got)
	}
}

func TestRecall_ResourceMissingFromStore_UnknownAndWarning(t *testing.T) {
	s := openRecallStore(t)
	// Learning points at a resource that was never synced.
	seedLearning(t, s, "odds Portugal wins world cup", "kalshi_markets", "KXMENWORLDCUP-26-PT")

	got, err := learn.Recall(context.Background(), s.DB(), "odds Portugal wins world cup", learn.Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	// The learning's query_entities was populated at write time with
	// [Portugal] -- so even with the resource missing, the validator
	// uses storedEntitySlice to keep classification coherent.
	if !got.Found || len(got.Results) != 1 {
		t.Fatalf("want one result; got %+v", got)
	}
	hit := got.Results[0]
	if !sliceContains(hit.Warnings, learn.WarningResourceNotInStore) {
		t.Errorf("want %s warning when resource missing; got %v", learn.WarningResourceNotInStore, hit.Warnings)
	}
}

func TestRecall_LowConfidenceWarning(t *testing.T) {
	s := openRecallStore(t)
	seedResource(t, s, "kalshi_markets", "KX-Y", map[string]any{
		"yes_sub_title": "USA",
	})
	seedLearning(t, s, "USA wins world cup", "kalshi_markets", "KX-Y")

	got, err := learn.Recall(context.Background(), s.DB(), "USA wins world cup", learn.Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if !got.Found || len(got.Results) != 1 {
		t.Fatalf("want one result; got %+v", got)
	}
	// Post-U4, first teach lands at confidence=2, which clears the
	// skill's >=2 skip threshold immediately. The low_confidence warning
	// only attaches to rows below that floor (legacy v3 data or rows
	// downgraded by external mutation), not to fresh teaches.
	if sliceContains(got.Results[0].Warnings, learn.WarningLowConfidence) {
		t.Errorf("first teach should NOT carry low_confidence warning (U4 floor=2); got %v", got.Results[0].Warnings)
	}
}

func TestRecall_MinConfidenceFiltersBelowThreshold(t *testing.T) {
	s := openRecallStore(t)
	seedResource(t, s, "kalshi_markets", "KX-Z", map[string]any{
		"yes_sub_title": "USA",
	})
	seedLearning(t, s, "USA wins world cup", "kalshi_markets", "KX-Z")

	// Post-U4, the single-teach floor is 2. To verify min-confidence
	// filtering, use a threshold ABOVE the floor.
	got, err := learn.Recall(context.Background(), s.DB(), "USA wins world cup", learn.Opts{MinConfidence: 3})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	// One teach => confidence=2 => below MinConfidence=3 floor => no results.
	if got.Found || len(got.Results) != 0 {
		t.Errorf("MinConfidence=3 should filter the single-teach row; got %+v", got)
	}
	// Top-level warning should signal empty result set.
	if !sliceContains(got.Warnings, learn.TopWarningNoLearningsForQueryFamily) {
		t.Errorf("want %s top-level warning on empty results; got %v",
			learn.TopWarningNoLearningsForQueryFamily, got.Warnings)
	}
}

func TestRecall_EnvelopeShape_IncludesNormalizedAndQueryEntities(t *testing.T) {
	s := openRecallStore(t)
	got, err := learn.Recall(context.Background(), s.DB(), "odds Portugal wins world cup", learn.Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if got.Normalized != "cup world" {
		t.Errorf("normalized: want 'cup world' (sorted, stopwords stripped), got %q", got.Normalized)
	}
	if !sliceContains(got.QueryEntities, "Portugal") {
		t.Errorf("query_entities: want Portugal, got %v", got.QueryEntities)
	}
}

func TestRecall_EnvelopeShape_JSONStableNullsAndArrays(t *testing.T) {
	// Guard the JSON contract the LLM consumes: query_entities is
	// always an array (never null), results is always an array,
	// mismatches is omitted when empty AND DebugMismatches is false.
	s := openRecallStore(t)
	got, err := learn.Recall(context.Background(), s.DB(), "rate cut", learn.Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	js := string(raw)
	if !containsStr(js, `"query_entities":[]`) && !containsStr(js, `"query_entities":["`) {
		t.Errorf("envelope should serialize query_entities as a non-null array; got %s", js)
	}
	if !containsStr(js, `"results":[]`) && !containsStr(js, `"results":[{`) {
		t.Errorf("envelope should serialize results as a non-null array; got %s", js)
	}
	if containsStr(js, `"mismatches":`) {
		t.Errorf("envelope should omit mismatches when DebugMismatches=false and empty; got %s", js)
	}
}

// TestRecall_RecipeFallback_FoundWithSourceRecipe asserts the U10
// generalization path: a cold England query against a corpus that
// has a Portugal+USA recipe should return Found=true via the recipe
// engine, with the recipe-derived hit tagged source="recipe".
func TestRecall_RecipeFallback_FoundWithSourceRecipe(t *testing.T) {
	s := openRecallStore(t)
	// Seed Portugal + USA + England Kalshi resources, then teach the
	// first two so Extract derives a recipe.
	seedResource(t, s, "kalshi_markets", "KXMENWORLDCUP-26-PT", map[string]any{
		"title": "Portugal cup", "ticker": "KXMENWORLDCUP-26-PT",
	})
	seedResource(t, s, "kalshi_markets", "KXMENWORLDCUP-26-US", map[string]any{
		"title": "USA cup", "ticker": "KXMENWORLDCUP-26-US",
	})
	seedResource(t, s, "kalshi_markets", "KXMENWORLDCUP-26-GB", map[string]any{
		"title": "England cup", "ticker": "KXMENWORLDCUP-26-GB",
	})

	teachOneRecall := func(query, resourceID string) {
		t.Helper()
		n := learn.Normalize(query, learn.DefaultPredictionGoatConfig())
		if _, _, err := s.UpsertLearning(store.UpsertLearningInput{
			Query: query, QueryEntities: n.Entities,
			ResourceID: resourceID, ResourceType: "kalshi_markets",
			Source: store.LearningSourceTaught,
		}); err != nil {
			t.Fatalf("teach %q -> %s: %v", query, resourceID, err)
		}
	}
	teachOneRecall("odds Portugal wins world cup", "KXMENWORLDCUP-26-PT")
	teachOneRecall("odds USA wins world cup", "KXMENWORLDCUP-26-US")

	// Trigger extraction explicitly. In production this fires from
	// the CLI teach hook; the recall layer reads recipes either way.
	if _, err := recipes.Extract(s.DB()); err != nil {
		t.Fatalf("extract: %v", err)
	}

	got, err := learn.Recall(context.Background(), s.DB(), "odds England wins world cup", learn.Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if !got.Found {
		t.Fatalf("England recall should be Found=true via recipe; got %+v", got)
	}
	var found bool
	for _, h := range got.Results {
		if h.ResourceID == "KXMENWORLDCUP-26-GB" && h.Source == "recipe" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("recall results missing recipe-source GB hit; results=%+v", got.Results)
	}
}

// TestRecall_DirectAndRecipeMerge_DirectRanksFirst asserts that
// when a query has both a direct teach AND a recipe match, the
// direct hit ranks first by source priority (within the same
// entity_match tier).
func TestRecall_DirectAndRecipeMerge_DirectRanksFirst(t *testing.T) {
	s := openRecallStore(t)
	// Seed Portugal + USA + Spain resources. Teach Portugal+USA so a
	// recipe forms; also directly teach Spain so the query for Spain
	// has both a direct match AND a recipe match.
	seedResource(t, s, "kalshi_markets", "KXMENWORLDCUP-26-PT", map[string]any{
		"title": "Portugal cup", "ticker": "KXMENWORLDCUP-26-PT",
	})
	seedResource(t, s, "kalshi_markets", "KXMENWORLDCUP-26-US", map[string]any{
		"title": "USA cup", "ticker": "KXMENWORLDCUP-26-US",
	})
	seedResource(t, s, "kalshi_markets", "KXMENWORLDCUP-26-ES", map[string]any{
		"title": "Spain cup", "yes_sub_title": "Spain", "ticker": "KXMENWORLDCUP-26-ES",
	})

	teachOneRecall := func(query, resourceID string) {
		t.Helper()
		n := learn.Normalize(query, learn.DefaultPredictionGoatConfig())
		if _, _, err := s.UpsertLearning(store.UpsertLearningInput{
			Query: query, QueryEntities: n.Entities,
			ResourceID: resourceID, ResourceType: "kalshi_markets",
			Source: store.LearningSourceTaught,
		}); err != nil {
			t.Fatalf("teach %q -> %s: %v", query, resourceID, err)
		}
	}
	teachOneRecall("odds Portugal wins world cup", "KXMENWORLDCUP-26-PT")
	teachOneRecall("odds USA wins world cup", "KXMENWORLDCUP-26-US")
	teachOneRecall("odds Spain wins world cup", "KXMENWORLDCUP-26-ES")
	if _, err := recipes.Extract(s.DB()); err != nil {
		t.Fatalf("extract: %v", err)
	}

	got, err := learn.Recall(context.Background(), s.DB(), "odds Spain wins world cup", learn.Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if !got.Found {
		t.Fatalf("Spain recall should be Found=true; got %+v", got)
	}
	if len(got.Results) == 0 {
		t.Fatalf("Spain recall has no results")
	}
	if got.Results[0].Source == "recipe" {
		t.Errorf("first hit is recipe-source; direct teach should outrank it. results=%+v", got.Results)
	}
	// The first hit should be the direct teach for ES. Recipe hits
	// (if any) would target the same ticker and be deduped, so we
	// expect exactly one Spain hit total.
	if got.Results[0].ResourceID != "KXMENWORLDCUP-26-ES" {
		t.Errorf("first hit.ResourceID = %q, want KXMENWORLDCUP-26-ES", got.Results[0].ResourceID)
	}
}

// hasWarningPrefix reports whether any warning in the slice begins
// with the supplied prefix. parent_event_when_child_exists is emitted
// as "<prefix>:<child_ticker>", so callers test by prefix.
func hasWarningPrefix(warnings []string, prefix string) bool {
	for _, w := range warnings {
		if len(w) >= len(prefix) && w[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func sliceContains(haystack []string, needle string) bool {
	for _, w := range haystack {
		if w == needle {
			return true
		}
	}
	return false
}

func containsStr(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
