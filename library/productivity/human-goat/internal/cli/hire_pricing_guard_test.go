// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/source/taskrabbit"
)

func TestPickTaskRabbitCategoryPrefersExactTitle(t *testing.T) {
	templates := []taskrabbitCategoryMatch{
		{Title: "IKEA Furniture Assembly", CategoryName: "Furniture Assembly", CategoryID: 1104},
		{Title: "Furniture Assembly", CategoryName: "Furniture Assembly", CategoryID: 33},
	}
	got, err := pickTaskRabbitCategory(templates, "Furniture assembly")
	if err != nil {
		t.Fatal(err)
	}
	if got.CategoryID != 33 {
		t.Fatalf("picked category %d, want 33", got.CategoryID)
	}
	got, err = pickTaskRabbitCategory(templates, "ikea")
	if err != nil || got.CategoryID != 1104 {
		t.Fatalf("substring fallback: got %d, %v", got.CategoryID, err)
	}
}

func TestSelectHireTaskerSkipsUnpricedTaskers(t *testing.T) {
	taskers := []taskrabbit.Tasker{
		{FirstName: "Free", PosterHourlyRateCents: 0, RabbitRating: 5},
		{FirstName: "Priced", PosterHourlyRateCents: 4200, RabbitRating: 4.9},
	}
	got, ok := selectHireTasker(taskers, 4.8, "CA")
	if !ok || got.FirstName != "Priced" {
		t.Fatalf("got %q ok=%v, want the priced Tasker", got.FirstName, ok)
	}
	if _, ok := selectHireTasker(taskers[:1], 0, "CA"); ok {
		t.Fatal("selected a Tasker with no hourly rate")
	}
}
