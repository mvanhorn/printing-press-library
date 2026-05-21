// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
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

func TestChoosePlanReturnsEmptyWhenNoCommonCurrency(t *testing.T) {
	t.Parallel()

	got, err := choosePlan([][]planPick{
		{{Leg: "Paris:2026-06-01:2026-06-03", Price: 80, Currency: "EUR"}},
		{{Leg: "London:2026-06-03:2026-06-05", Price: 70, Currency: "GBP"}},
	}, 250)
	if err != nil {
		t.Fatalf("choosePlan returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("choosePlan returned mixed-currency plan, want empty: %+v", got)
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
