// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestScorePicks(t *testing.T) {
	rows := []restaurantRow{
		{ID: "1", Name: "Best", DeliveryFeeCents: 0, ETAMinutes: 10, Rating: 5.0, Deals: 3},
		{ID: "2", Name: "Worst", DeliveryFeeCents: 500, ETAMinutes: 60, Rating: 2.0, Deals: 0},
		{ID: "3", Name: "Mid", DeliveryFeeCents: 200, ETAMinutes: 30, Rating: 4.0, Deals: 1},
	}
	picks := scorePicks(rows, scoreWeights{fee: 1, eta: 1, rating: 1, deal: 1})
	if len(picks) != 3 {
		t.Fatalf("got %d picks, want 3", len(picks))
	}
	// "Best" wins on every dimension, so it must rank first with the top score.
	if picks[0].Name != "Best" {
		t.Errorf("top pick = %s, want Best", picks[0].Name)
	}
	if picks[0].Score < picks[1].Score || picks[1].Score < picks[2].Score {
		t.Errorf("picks not sorted by score desc: %v", picks)
	}
	// Best is the min-fee, min-eta, max-rating, max-deal entry -> ~100.
	if picks[0].Score < 99 {
		t.Errorf("best score = %.1f, want ~100", picks[0].Score)
	}
	// Breakdown components are 0-100 normalized.
	for k, v := range picks[0].Breakdown {
		if v < 0 || v > 100 {
			t.Errorf("breakdown[%s] = %.1f out of [0,100]", k, v)
		}
	}
}

func TestScorePicksWeightingShiftsWinner(t *testing.T) {
	rows := []restaurantRow{
		{ID: "1", Name: "Cheap", DeliveryFeeCents: 0, ETAMinutes: 50, Rating: 3.0, Deals: 0},
		{ID: "2", Name: "Dealy", DeliveryFeeCents: 500, ETAMinutes: 50, Rating: 3.0, Deals: 5},
	}
	// Heavy deal weight should make the deal-rich restaurant win.
	picks := scorePicks(rows, scoreWeights{fee: 0, eta: 0, rating: 0, deal: 1})
	if picks[0].Name != "Dealy" {
		t.Errorf("deal-weighted top = %s, want Dealy", picks[0].Name)
	}
}

func TestNormLowerBetter(t *testing.T) {
	if got := normLowerBetter(0, 0, 10); got != 1 {
		t.Errorf("min value should score 1, got %v", got)
	}
	if got := normLowerBetter(10, 0, 10); got != 0 {
		t.Errorf("max value should score 0, got %v", got)
	}
	if got := normLowerBetter(5, 5, 5); got != 1 {
		t.Errorf("degenerate range should score 1, got %v", got)
	}
}

func TestNormHigherBetter(t *testing.T) {
	if got := normHigherBetter(10, 0, 10); got != 1 {
		t.Errorf("max value should score 1, got %v", got)
	}
	if got := normHigherBetter(0, 0, 10); got != 0 {
		t.Errorf("min value should score 0, got %v", got)
	}
	if got := normHigherBetter(4.5, 4.5, 4.5); got != 1 {
		t.Errorf("degenerate range should score 1, got %v", got)
	}
}
