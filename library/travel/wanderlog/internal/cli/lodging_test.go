// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestBuildLodgingSearchRequestMatchesBrowserShape(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Float64("min-guest-rating", 0, "")
	if err := cmd.Flags().Set("min-guest-rating", "8"); err != nil {
		t.Fatalf("set min rating: %v", err)
	}
	body, err := buildLodgingSearchRequest(cmd, lodgingSearchOptions{
		geoID:                 50,
		bounds:                []float64{127.63045, 26.17561, 127.73895, 26.24614},
		startDate:             "2026-08-30",
		endDate:               "2026-09-06",
		adultCount:            2,
		roomCount:             1,
		sortBy:                "ratings",
		propertyName:          "",
		hotelOrVacationRental: "both",
		minGuestRating:        8,
		sources:               []string{"airbnb", "expedia", "google", "kayak"},
	})
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req := body.(map[string]any)
	if req["geoId"] != 50 || req["sortBy"] != "ratings" {
		t.Fatalf("request = %#v", req)
	}
	bounds := req["bounds"].([]float64)
	if len(bounds) != 4 || bounds[0] != 127.63045 || bounds[3] != 26.24614 {
		t.Fatalf("bounds = %#v", bounds)
	}
	dates := req["dates"].(map[string]any)
	if dates["startDate"] != "2026-08-30" || dates["endDate"] != "2026-09-06" {
		t.Fatalf("dates = %#v", dates)
	}
	guests := req["guests"].(map[string]any)
	if guests["adultCount"] != 2 || guests["roomCount"] != 1 {
		t.Fatalf("guests = %#v", guests)
	}
	filters := req["filters"].(map[string]any)
	if filters["minGuestRating"] != 8.0 || filters["hotelOrVacationRental"] != "both" || filters["propertyName"] != "" {
		t.Fatalf("filters = %#v", filters)
	}
	propertyTypes := filters["propertyTypes"].(map[string]any)
	if propertyTypes["lodgingTypes"] != nil || propertyTypes["accommodationTypes"] != nil {
		t.Fatalf("propertyTypes = %#v", propertyTypes)
	}
	vacationRentalFilters := filters["vacationRentalFilters"].(map[string]any)
	amenities := vacationRentalFilters["amenities"].([]any)
	if len(amenities) != 0 {
		t.Fatalf("vacation rental amenities = %#v", amenities)
	}
}

func TestBuildLodgingSearchRequestRequiresBounds(t *testing.T) {
	cmd := &cobra.Command{}
	_, err := buildLodgingSearchRequest(cmd, lodgingSearchOptions{
		geoID:      50,
		bounds:     []float64{127.63045, 26.17561},
		startDate:  "2026-08-30",
		endDate:    "2026-09-06",
		adultCount: 2,
		roomCount:  1,
		sources:    []string{"google"},
	})
	if err == nil {
		t.Fatalf("short bounds accepted")
	}
}

func TestBuildLodgingSearchRequestAcceptsRawRequest(t *testing.T) {
	cmd := &cobra.Command{}
	body, err := buildLodgingSearchRequest(cmd, lodgingSearchOptions{request: `{"geoId":50,"sources":["google"]}`})
	if err != nil {
		t.Fatalf("raw request: %v", err)
	}
	req := body.(map[string]any)
	if req["geoId"] != float64(50) {
		t.Fatalf("request = %#v", req)
	}
}

func TestSummarizeLodgingSearchResponse(t *testing.T) {
	raw := map[string]any{
		"success": true,
		"data": map[string]any{
			"isComplete": true,
			"offers": []any{map[string]any{
				"source":  "airbnb",
				"offerId": "123",
				"lodging": map[string]any{
					"id":   map[string]any{"type": "airbnb", "listingId": "123"},
					"name": "Naha Base",
					"location": map[string]any{
						"latitude":  26.2,
						"longitude": 127.7,
					},
					"rating":      map[string]any{"source": "Airbnb", "value": 5.0},
					"ratingCount": 40,
				},
				"priceRate": map[string]any{
					"site":         "Airbnb",
					"amount":       200.0,
					"currencyCode": "SGD",
					"total":        map[string]any{"amount": 1400.0, "currencyCode": "SGD"},
					"bookingUrl":   "https://example.com",
				},
			}},
			"unfilteredStats": map[string]any{},
		},
	}
	summary := summarizeLodgingSearchResponse(raw, 10)
	offers := summary["offers"].([]any)
	if summary["offer_count"] != 1 || len(offers) != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	first := offers[0].(map[string]any)
	if first["name"] != "Naha Base" || first["booking_url"] != "https://example.com" || first["lat"] != 26.2 {
		t.Fatalf("first = %#v", first)
	}
}
