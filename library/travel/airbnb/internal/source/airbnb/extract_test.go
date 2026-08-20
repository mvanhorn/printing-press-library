package airbnb

import "testing"

// TestListingFromSearchFlatShape covers the current Airbnb SSR Apollo cache
// shape where each searchResults entry is a flat listing card (no `listing`
// or `pricingQuote` envelope), price lives at `structuredDisplayPrice`, and
// `reviewsCount` is null with the count buried in `avgRatingLocalized`.
func TestListingFromSearchFlatShape(t *testing.T) {
	entry := map[string]any{
		"title":              "Guest suite in Mercer Island",
		"subtitle":           "Secluded Private Guest Suite",
		"avgRatingLocalized": "4.98 (576)",
		"avgRatingA11yLabel": "4.98 out of 5 average rating,  576 reviews",
		"reviewsCount":       nil,
		"demandStayListing": map[string]any{
			"id": "RGVtYW5kU3RheUxpc3Rpbmc6MTg0MTMxODY=",
		},
		"structuredDisplayPrice": map[string]any{
			"primaryLine": map[string]any{
				"price":     "$206",
				"qualifier": "for 1 night",
			},
			"secondaryLine": nil,
		},
		"structuredContent": map[string]any{
			"primaryLine":   []any{map[string]any{"body": "$206 night"}},
			"secondaryLine": []any{map[string]any{"body": "Free cancellation"}},
		},
	}

	// The flat shape has no `listing` or `pricingQuote` envelope — `client.Search`
	// falls back to passing the entry itself as both arguments.
	l := listingFromSearch(entry, entry)

	if l.Title != "Guest suite in Mercer Island" {
		t.Fatalf("Title = %q", l.Title)
	}
	if l.PrimaryPrice == nil || l.PrimaryPrice.Amount != 206 {
		t.Fatalf("PrimaryPrice = %#v, want amount 206", l.PrimaryPrice)
	}
	if l.PerNightPrice != 206 {
		t.Fatalf("PerNightPrice = %v, want 206", l.PerNightPrice)
	}
	if l.ReviewsCount != 576 {
		t.Fatalf("ReviewsCount = %d, want 576 (parsed from avgRatingLocalized)", l.ReviewsCount)
	}
	if len(l.PrimaryLine) == 0 || l.PrimaryLine[0] != "$206 night" {
		t.Fatalf("PrimaryLine = %v", l.PrimaryLine)
	}
	if len(l.SecondaryLine) == 0 || l.SecondaryLine[0] != "Free cancellation" {
		t.Fatalf("SecondaryLine = %v", l.SecondaryLine)
	}
}

// TestListingFromSearchReviewsCountFromA11y exercises the avgRatingA11yLabel
// fallback when avgRatingLocalized doesn't match the "X (Y)" pattern.
func TestListingFromSearchReviewsCountFromA11y(t *testing.T) {
	entry := map[string]any{
		"title":              "Some listing",
		"avgRatingLocalized": "New",
		"avgRatingA11yLabel": "4.9 out of 5 average rating, 1,234 reviews",
		"reviewsCount":       nil,
	}
	l := listingFromSearch(entry, entry)
	if l.ReviewsCount != 1234 {
		t.Fatalf("ReviewsCount = %d, want 1234", l.ReviewsCount)
	}
}

// TestListingFromSearchLegacyEnvelopeStillWorks ensures the original
// `structuredStayDisplayPrice` lookup remains as a fallback for any route
// that still ships the old envelope shape.
func TestListingFromSearchLegacyEnvelopeStillWorks(t *testing.T) {
	priceQuote := map[string]any{
		"structuredStayDisplayPrice": map[string]any{
			"primaryLine": map[string]any{"price": "$300"},
		},
	}
	l := listingFromSearch(map[string]any{"title": "Legacy", "name": "Legacy"}, priceQuote)
	if l.PrimaryPrice == nil || l.PrimaryPrice.Amount != 300 {
		t.Fatalf("PrimaryPrice = %#v, want amount 300 from legacy shape", l.PrimaryPrice)
	}
}
