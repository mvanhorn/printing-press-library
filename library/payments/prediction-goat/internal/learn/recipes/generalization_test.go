// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package recipes_test

import (
	"context"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn/recipes"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

// TestGeneralization_PortugalPlusUSATeachesUnlocksEngland is the
// end-to-end regression for U10. It exercises the full pipeline:
//
//  1. Fresh v6 store with seeded entity_lookups.
//  2. Seed resources for Portugal + USA + England (Kalshi tickers
//     and Polymarket slugs) — the recall layer needs them present
//     to verify recipe substitutions.
//  3. Teach Portugal's pair of tickers.
//  4. Teach USA's pair of tickers.
//  5. Run Extract to produce recipes.
//  6. Recall for "odds England wins world cup" — England was never
//     directly taught. Result MUST be Found=true with both
//     Kalshi + Polymarket England tickers reached via recipe.
//
// This is the flagship "teach Portugal once, get every other country
// free" story the smart-learning plan promises.
func TestGeneralization_PortugalPlusUSATeachesUnlocksEngland(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	ctx := context.Background()

	// 2. Seed the resources table. The recall + recipe verification
	//    paths look up the substituted candidate here before
	//    returning a hit.
	seedResource(t, s, "kalshi_markets", "KXMENWORLDCUP-26-PT", map[string]any{
		"title": "Will Portugal win the 2026 FIFA Men's World Cup?", "ticker": "KXMENWORLDCUP-26-PT",
	})
	seedResource(t, s, "kalshi_markets", "KXMENWORLDCUP-26-US", map[string]any{
		"title": "Will the United States win the 2026 FIFA Men's World Cup?", "ticker": "KXMENWORLDCUP-26-US",
	})
	seedResource(t, s, "kalshi_markets", "KXMENWORLDCUP-26-GB", map[string]any{
		"title": "Will England win the 2026 FIFA Men's World Cup?", "ticker": "KXMENWORLDCUP-26-GB",
	})
	seedResource(t, s, "markets", "will-portugal-win-the-2026-fifa-world-cup-912", map[string]any{
		"question": "Will Portugal win the 2026 FIFA World Cup?",
	})
	seedResource(t, s, "markets", "will-usa-win-the-2026-fifa-world-cup-467", map[string]any{
		"question": "Will the USA win the 2026 FIFA World Cup?",
	})
	seedResource(t, s, "markets", "will-england-win-the-2026-fifa-world-cup-318", map[string]any{
		"question": "Will England win the 2026 FIFA World Cup?",
	})

	// 3 & 4. Teach Portugal and USA, both venues each. UpsertLearning
	//        accepts QueryEntities so the structural extractor sees
	//        the same shape it would in production.
	teachAt := func(query, resourceType, resourceID string) {
		t.Helper()
		n := learn.Normalize(query, learn.DefaultPredictionGoatConfig())
		if _, _, err := s.UpsertLearning(store.UpsertLearningInput{
			Query:         query,
			QueryEntities: n.Entities,
			ResourceID:    resourceID,
			ResourceType:  resourceType,
			Source:        store.LearningSourceTaught,
		}); err != nil {
			t.Fatalf("teach %s -> %s: %v", query, resourceID, err)
		}
	}
	teachAt("odds Portugal wins world cup", "kalshi_markets", "KXMENWORLDCUP-26-PT")
	teachAt("odds Portugal wins world cup", "markets", "will-portugal-win-the-2026-fifa-world-cup-912")
	teachAt("odds USA wins world cup", "kalshi_markets", "KXMENWORLDCUP-26-US")
	teachAt("odds USA wins world cup", "markets", "will-usa-win-the-2026-fifa-world-cup-467")

	// 5. Run Extract. The store-side teach loop will eventually
	//    call this for us via the post-teach hook (U10 wires the
	//    CLI seam), but exercising it directly here keeps the
	//    regression test independent of CLI orchestration.
	if _, err := recipes.Extract(s.DB()); err != nil {
		t.Fatalf("extract: %v", err)
	}

	// Assert recipes exist for both venues. The Kalshi side should
	// produce a substitute-strategy recipe; the Polymarket side
	// should produce a prefix-strategy recipe.
	rows, err := recipes.List(s.DB(), recipes.ListFilter{})
	if err != nil {
		t.Fatalf("list recipes: %v", err)
	}
	if len(rows) < 2 {
		t.Fatalf("expected at least 2 recipes (kalshi + polymarket), got %d: %+v", len(rows), rows)
	}
	var hasKalshi, hasPoly bool
	for _, r := range rows {
		if r.ResourceType == "kalshi_markets" && r.Strategy == recipes.StrategySubstitute {
			hasKalshi = true
		}
		if r.ResourceType == "markets" && r.Strategy == recipes.StrategySubstituteThenSearchPrefix {
			hasPoly = true
		}
	}
	if !hasKalshi {
		t.Errorf("no kalshi substitute recipe found in %+v", rows)
	}
	if !hasPoly {
		t.Errorf("no polymarket prefix recipe found in %+v", rows)
	}

	// 7 & 8. Recall England — never directly taught. The recipe
	//        layer must produce both venue's tickers.
	got, err := learn.Recall(ctx, s.DB(), "odds England wins world cup", learn.Opts{})
	if err != nil {
		t.Fatalf("recall England: %v", err)
	}
	if !got.Found {
		t.Fatalf("England query should be Found=true via recipe; got %+v", got)
	}

	// Collect the resource IDs from the recall result.
	resourceIDs := map[string]string{}
	for _, h := range got.Results {
		resourceIDs[h.ResourceID] = h.Source
	}

	if src, ok := resourceIDs["KXMENWORLDCUP-26-GB"]; !ok {
		t.Errorf("recall for England missing KXMENWORLDCUP-26-GB; got %+v", got.Results)
	} else if src != "recipe" {
		t.Errorf("KXMENWORLDCUP-26-GB.Source = %q, want recipe", src)
	}

	if src, ok := resourceIDs["will-england-win-the-2026-fifa-world-cup-318"]; !ok {
		t.Errorf("recall for England missing Polymarket slug; got %+v", got.Results)
	} else if src != "recipe" {
		t.Errorf("polymarket hit.Source = %q, want recipe", src)
	}
}
