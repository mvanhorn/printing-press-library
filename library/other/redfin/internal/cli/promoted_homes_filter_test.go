package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/redfin"
)

// TestFilterListings_PriceBounds locks the client-side enforcement of
// price-min / price-max against the Stingray response. Server-side filters
// are documented but not honored (verified against multiple city regions
// 2026-05-17): a gis query with min_price=700000&max_price=900000 still
// returns $220k listings. Without this filter, --price-min and --price-max
// are silently advisory.
func TestFilterListings_PriceBounds(t *testing.T) {
	in := []redfin.Listing{
		{Price: 0},
		{Price: 220000},
		{Price: 750000},
		{Price: 850000},
		{Price: 1_200_000},
	}
	got := filterListings(redfin.SearchOptions{PriceMin: 700000, PriceMax: 900000}, in)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (the 750k and 850k rows)", len(got))
	}
	for _, l := range got {
		if l.Price < 700000 || l.Price > 900000 {
			t.Errorf("listing price %d outside [700k, 900k]", l.Price)
		}
	}
}

// TestFilterListings_ZeroMeansUnbounded confirms that a zero-valued
// bound is treated as "no filter" rather than ">= 0" — otherwise every
// listing would be in-bounds for max_* and out-of-bounds for min_*.
func TestFilterListings_ZeroMeansUnbounded(t *testing.T) {
	in := []redfin.Listing{
		{Price: 100, Beds: 3, Baths: 2, Sqft: 1000, YearBuilt: 2000, LotSize: 5000},
		{Price: 999_999_999, Beds: 5, Baths: 4, Sqft: 9000, YearBuilt: 2024, LotSize: 99999},
	}
	got := filterListings(redfin.SearchOptions{}, in)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (no bounds = pass-through)", len(got))
	}
}

// TestFilterListings_ZeroFieldsArePreserved confirms that sparse rows missing
// non-price fields (e.g. an off-MLS row with no parsed beds count) aren't
// dropped just because the field is the Go zero value. Price and lot-size bounds
// are stricter because returning unknown-price/unknown-lot rows would violate
// the promise that those explicit bounds are satisfied.
func TestFilterListings_ZeroFieldsArePreserved(t *testing.T) {
	in := []redfin.Listing{
		{Price: 800000, Beds: 0, Baths: 0, Sqft: 0}, // unknown beds/baths/sqft
	}
	got := filterListings(redfin.SearchOptions{BedsMin: 3, BathsMin: 2.5, SqftMin: 1500}, in)
	if len(got) != 1 {
		t.Errorf("listings with unknown beds/baths/sqft were dropped; want pass-through")
	}
}

// TestFilterListings_MaxBoundsApplyEvenWhenZero locks that max_price=N
// drops listings priced above N. The zero-means-unbounded rule applies
// to the OPTION value, not the listing value.
func TestFilterListings_MaxBoundsApplyEvenWhenZero(t *testing.T) {
	in := []redfin.Listing{
		{Price: 500000, Sqft: 2000, YearBuilt: 2020},
		{Price: 1_500_000, Sqft: 5000, YearBuilt: 2024},
	}
	got := filterListings(redfin.SearchOptions{PriceMax: 1_000_000, SqftMax: 3000, YearMax: 2022}, in)
	if len(got) != 1 || got[0].Price != 500000 {
		t.Errorf("max-bound filter not applied; got %+v", got)
	}
}

// TestFilterListings_LotMinRequiresKnownLotSize confirms --lot-min returns only
// listings whose parsed gis lotSize satisfies the requested floor. Rows with an
// omitted lotSize cannot prove they satisfy the bound, so they are dropped.
func TestFilterListings_LotMinRequiresKnownLotSize(t *testing.T) {
	in := []redfin.Listing{
		{Price: 500000, LotSize: 0},
		{Price: 600000, LotSize: 5000},
		{Price: 700000, LotSize: 12000},
	}
	got := filterListings(redfin.SearchOptions{LotMin: 10000}, in)
	if len(got) != 1 || got[0].LotSize != 12000 {
		t.Errorf("lot-min filter not applied; got %+v", got)
	}
}

// TestFilterListings_UIPropertyType_DropsLandLot locks that a --type house
// (UIPropertyTypes=[1]) request drops listings the gis response tagged as
// non-house (uiPropertyType=5/6 in observed responses). Server-side `uipt=1`
// is silently advisory despite the CLI sending it.
func TestFilterListings_UIPropertyType_DropsLandLot(t *testing.T) {
	in := []redfin.Listing{
		{Price: 800000, UIPropertyType: 1, Beds: 4}, // house
		{Price: 900000, UIPropertyType: 5},          // non-house
		{Price: 750000, UIPropertyType: 6},          // non-house
		{Price: 850000, UIPropertyType: 1, Beds: 3}, // house
	}
	got := filterListings(redfin.SearchOptions{UIPropertyTypes: []int{1}}, in)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (only uiPropertyType=1 rows)", len(got))
	}
	for _, l := range got {
		if l.UIPropertyType != 1 {
			t.Errorf("kept non-house row: ui=%d", l.UIPropertyType)
		}
	}
}

// TestFilterListings_UIPropertyType_PreservesUnknown confirms that a row
// whose gis response omitted uiPropertyType (UIPropertyType=0) is NOT
// dropped when --type is set. Treating zero as "land/condo/anything-not-1"
// would silently exclude rows the server sent us where the field was
// genuinely missing.
func TestFilterListings_UIPropertyType_PreservesUnknown(t *testing.T) {
	in := []redfin.Listing{
		{Price: 800000, UIPropertyType: 0, Beds: 4},
	}
	got := filterListings(redfin.SearchOptions{UIPropertyTypes: []int{1}}, in)
	if len(got) != 1 {
		t.Errorf("dropped a row whose property type was unknown; want pass-through")
	}
}

// TestFilterListings_UIPropertyType_NoFilterPassesThrough confirms the
// type filter only applies when --type was explicitly set.
func TestFilterListings_UIPropertyType_NoFilterPassesThrough(t *testing.T) {
	in := []redfin.Listing{
		{Price: 800000, UIPropertyType: 1},
		{Price: 900000, UIPropertyType: 5},
		{Price: 750000, UIPropertyType: 6},
	}
	got := filterListings(redfin.SearchOptions{}, in)
	if len(got) != 3 {
		t.Errorf("type filter applied without --type; got %d, want 3", len(got))
	}
}
