// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func dishFixture() []cachedMenuItem {
	return []cachedMenuItem{
		{ItemID: "1", RestaurantName: "Poke Co", Name: "Spicy Poke Bowl", Description: "ahi tuna", PriceCents: 1400},
		{ItemID: "2", RestaurantName: "Sweetgreen", Name: "Caesar Salad", Description: "vegan option available", PriceCents: 1100},
		{ItemID: "3", RestaurantName: "Poke Co", Name: "Salmon Bowl", Description: "poke style", PriceCents: 1800},
		{ItemID: "4", RestaurantName: "Burger Hut", Name: "Cheeseburger", Description: "beef", PriceCents: 900},
	}
}

func TestMatchDishes(t *testing.T) {
	items := dishFixture()

	// Query matches name or description.
	got := matchDishes(items, "poke", 0, "")
	if len(got) != 2 {
		t.Fatalf("query 'poke' matched %d, want 2 (name + description)", len(got))
	}
	// Sorted cheapest first.
	if got[0].PriceCents > got[1].PriceCents {
		t.Errorf("matches not sorted ascending by price: %+v", got)
	}

	// Max-price filter.
	got = matchDishes(items, "poke", 15.0, "")
	if len(got) != 1 || got[0].Item != "Spicy Poke Bowl" {
		t.Errorf("max-price $15 'poke' = %+v, want only Spicy Poke Bowl", got)
	}

	// Diet keyword narrows further.
	got = matchDishes(items, "salad", 0, "vegan")
	if len(got) != 1 || got[0].Restaurant != "Sweetgreen" {
		t.Errorf("diet vegan salad = %+v, want Sweetgreen", got)
	}

	// Negative: irrelevant query returns nothing.
	if got := matchDishes(items, "sushi", 0, ""); len(got) != 0 {
		t.Errorf("query 'sushi' should match nothing, got %+v", got)
	}

	// Multi-word queries match when all tokens are present, regardless of order
	// or contiguity. Both "Spicy Poke Bowl" (poke+bowl in name) and "Salmon
	// Bowl" (bowl in name, poke in description) qualify; the name-hit ranks
	// first. Reordering the tokens yields the same result.
	for _, q := range []string{"poke bowl", "bowl poke"} {
		got := matchDishes(items, q, 0, "")
		if len(got) != 2 || got[0].Item != "Spicy Poke Bowl" {
			t.Errorf("query %q = %+v, want 2 matches with Spicy Poke Bowl first", q, got)
		}
	}
}

func TestCapMatches(t *testing.T) {
	if got := capMatches(nil, 5); got == nil || len(got) != 0 {
		t.Errorf("capMatches(nil) should be empty non-nil slice, got %v", got)
	}
	m := []dishMatch{{Item: "a"}, {Item: "b"}, {Item: "c"}}
	if got := capMatches(m, 2); len(got) != 2 {
		t.Errorf("capMatches limit 2 = %d", len(got))
	}
	if got := capMatches(m, 0); len(got) != 3 {
		t.Errorf("capMatches limit 0 (unlimited) = %d, want 3", len(got))
	}
}

func TestDishNote(t *testing.T) {
	if dishNote(3, true, false) != "" {
		t.Error("note should be empty when there are matches")
	}
	if dishNote(0, true, false) == "" {
		t.Error("note should explain scan cap when zero matches and cap hit")
	}
	if dishNote(0, false, true) == "" {
		t.Error("note should explain empty cache in local mode")
	}
}
