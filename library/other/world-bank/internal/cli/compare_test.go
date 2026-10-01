// Copyright 2026 Luke J and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func float64Ptr(v float64) *float64 { return &v }

func TestBuildCompareViewUsesNewestCommonYear(t *testing.T) {
	obs := []wbObservation{
		{CountryISO3Code: "USA", Country: wbCodeValue{ID: "US", Value: "United States"}, Date: "2024", Value: float64Ptr(120)},
		{CountryISO3Code: "USA", Country: wbCodeValue{ID: "US", Value: "United States"}, Date: "2023", Value: float64Ptr(100)},
		{CountryISO3Code: "CAN", Country: wbCodeValue{ID: "CA", Value: "Canada"}, Date: "2023", Value: float64Ptr(80)},
		{CountryISO3Code: "CAN", Country: wbCodeValue{ID: "CA", Value: "Canada"}, Date: "2022", Value: float64Ptr(70)},
	}
	view, err := buildCompareView("GDP", "USA;CAN", obs)
	if err != nil {
		t.Fatalf("buildCompareView: %v", err)
	}
	if view.Date != "2023" || len(view.Rows) != 2 {
		t.Fatalf("date = %q, rows = %d; want 2023 and 2", view.Date, len(view.Rows))
	}
	if view.Rows[0].Date != view.Rows[1].Date {
		t.Fatalf("mismatched row dates: %q vs %q", view.Rows[0].Date, view.Rows[1].Date)
	}
	if got := *view.Rows[1].DeltaVsBase; got != -20 {
		t.Fatalf("Canada delta = %v, want -20", got)
	}
}

func TestBuildCompareViewRejectsMismatchedYearsWithoutOverlap(t *testing.T) {
	obs := []wbObservation{
		{CountryISO3Code: "USA", Country: wbCodeValue{Value: "United States"}, Date: "2024", Value: float64Ptr(120)},
		{CountryISO3Code: "CAN", Country: wbCodeValue{Value: "Canada"}, Date: "2023", Value: float64Ptr(80)},
	}
	if _, err := buildCompareView("GDP", "USA;CAN", obs); err == nil {
		t.Fatal("buildCompareView() succeeded without a common year")
	}
}

func TestBuildCompareViewRejectsMissingRequestedCountry(t *testing.T) {
	obs := []wbObservation{
		{CountryISO3Code: "USA", Country: wbCodeValue{ID: "US", Value: "United States"}, Date: "2024", Value: float64Ptr(120)},
	}
	if _, err := buildCompareView("GDP", "USA;CAN", obs); err == nil {
		t.Fatal("buildCompareView() succeeded without observations for Canada")
	}
}
