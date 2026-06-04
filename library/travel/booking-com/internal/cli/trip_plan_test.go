// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/booking-com/internal/booking"
)

func TestChoosePlanRequiresSingleCurrency(t *testing.T) {
	t.Parallel()

	got, err := choosePlan([][]planPick{
		{
			{Leg: "Paris:2026-06-01:2026-06-03", Price: 80, Currency: "EUR"},
			{Leg: "Paris:2026-06-01:2026-06-03", Price: 120, Currency: "USD"},
		},
		{
			{Leg: "London:2026-06-03:2026-06-05", Price: 70, Currency: "GBP"},
			{Leg: "London:2026-06-03:2026-06-05", Price: 100, Currency: "USD"},
		},
	}, 250)
	if err != nil {
		t.Fatalf("choosePlan returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("choosePlan returned %d picks, want 2: %+v", len(got), got)
	}
	for _, pick := range got {
		if pick.Currency != "USD" {
			t.Fatalf("choosePlan mixed currencies or selected wrong common currency: %+v", got)
		}
	}
}

func TestChoosePlanRejectsNoCommonCurrency(t *testing.T) {
	t.Parallel()

	_, err := choosePlan([][]planPick{
		{{Leg: "Paris:2026-06-01:2026-06-03", Price: 80, Currency: "EUR"}},
		{{Leg: "London:2026-06-03:2026-06-05", Price: 70, Currency: "GBP"}},
	}, 250)
	if err == nil {
		t.Fatal("choosePlan returned nil error for legs with no common currency")
	}
	if !strings.Contains(err.Error(), "no common currency") {
		t.Fatalf("choosePlan error = %q, want common-currency diagnostic", err)
	}
}

func TestChoosePlanRejectsOversizedSearchSpace(t *testing.T) {
	t.Parallel()

	options := make([][]planPick, 7)
	for i := range options {
		for j := 0; j < 10; j++ {
			options[i] = append(options[i], planPick{Leg: "leg", Price: float64(j + 1), Currency: "USD"})
		}
	}

	_, err := choosePlan(options, 10_000)
	if err == nil {
		t.Fatal("choosePlan returned nil error for oversized search space")
	}
	if !strings.Contains(err.Error(), "too many plan combinations") {
		t.Fatalf("choosePlan error = %q, want search-space cap error", err)
	}
}

func TestValidatePlanSearchSpaceChecksAfterEmptyLeg(t *testing.T) {
	t.Parallel()

	options := make([][]planPick, 8)
	for i := 1; i < len(options); i++ {
		for j := 0; j < 10; j++ {
			options[i] = append(options[i], planPick{Leg: "leg", Price: float64(j + 1), Currency: "USD"})
		}
	}

	err := validatePlanSearchSpace(options)
	if err == nil {
		t.Fatal("validatePlanSearchSpace returned nil error for oversized search space after empty leg")
	}
	if !strings.Contains(err.Error(), "too many plan combinations") {
		t.Fatalf("validatePlanSearchSpace error = %q, want search-space cap error", err)
	}
}

func TestPlanPicksForLegDropsUnpricedCards(t *testing.T) {
	t.Parallel()

	got := planPicksForLeg("Paris:2026-06-01:2026-06-03", []booking.PropertyCard{
		{Name: "No parse", Slug: "no-parse", Price: 0, Currency: "EUR"},
		{Name: "Higher", Slug: "higher", Price: 200, Currency: "EUR"},
		{Name: "Lower", Slug: "lower", Price: 100, Currency: "EUR"},
	})

	if len(got) != 2 {
		t.Fatalf("planPicksForLeg returned %d picks, want 2: %+v", len(got), got)
	}
	if got[0].Slug != "lower" || got[1].Slug != "higher" {
		t.Fatalf("planPicksForLeg did not sort by price: %+v", got)
	}
}

func TestPlanPicksForLegReturnsEmptyForNoParseablePrices(t *testing.T) {
	t.Parallel()

	got := planPicksForLeg("Paris:2026-06-01:2026-06-03", []booking.PropertyCard{
		{Name: "No parse", Slug: "no-parse", Price: 0, Currency: "EUR"},
	})
	if len(got) != 0 {
		t.Fatalf("planPicksForLeg returned priced picks for unpriced cards: %+v", got)
	}
}
