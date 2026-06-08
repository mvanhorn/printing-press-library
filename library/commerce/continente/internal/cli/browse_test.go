package cli

import (
	"testing"

	"continente-pp-cli/internal/acquisition/storefront"
)

func TestEnrichSearchResponse_AddsPaginationAndFilters(t *testing.T) {
	t.Parallel()

	payload := searchResponse{Start: 24, Count: 24}
	enrichSearchResponse(&payload, storefront.SearchParams{
		PageSize: 24,
		SortRule: "price-asc",
		Prefn1:   "brand",
		Prefv1:   "Mimosa",
		Prefn2:   "diet",
		Prefv2:   "Sem Lactose",
	})

	if payload.PageSize != 24 {
		t.Fatalf("page size = %d, want 24", payload.PageSize)
	}
	if payload.NextStart == nil || *payload.NextStart != 48 {
		t.Fatalf("next start = %v, want 48", payload.NextStart)
	}
	if payload.SortRule != "price-asc" {
		t.Fatalf("sort rule = %q, want price-asc", payload.SortRule)
	}
	if payload.ActiveFilters["brand"] != "Mimosa" || payload.ActiveFilters["diet"] != "Sem Lactose" {
		t.Fatalf("active filters = %#v", payload.ActiveFilters)
	}
}

func TestEnrichSearchResponse_DoesNotInventNextStartOnShortPage(t *testing.T) {
	t.Parallel()

	payload := searchResponse{Start: 24, Count: 10}
	enrichSearchResponse(&payload, storefront.SearchParams{PageSize: 24})
	if payload.NextStart != nil {
		t.Fatalf("next start = %v, want nil", payload.NextStart)
	}
}

func TestApplyDealSort_SortsByUnitPrice(t *testing.T) {
	t.Parallel()

	items := []storefrontItem{
		{ID: "a", Price: 10, UnitPrice: 2.0},
		{ID: "b", Price: 8, UnitPrice: 1.2},
		{ID: "c", Price: 5},
	}
	applyDealSort(items, "unit-price")

	if items[0].ID != "b" || items[1].ID != "a" || items[2].ID != "c" {
		t.Fatalf("unexpected unit-price order: %+v", items)
	}
}

func TestApplyDealSort_SortsBySavingsPercent(t *testing.T) {
	t.Parallel()

	items := []storefrontItem{
		{ID: "a", Price: 10, SavingsPercent: 10},
		{ID: "b", Price: 9, SavingsPercent: 25},
		{ID: "c", Price: 7},
	}
	applyDealSort(items, "savings-percent")

	if items[0].ID != "b" || items[1].ID != "a" || items[2].ID != "c" {
		t.Fatalf("unexpected savings-percent order: %+v", items)
	}
}
