// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package taskrabbit

import (
	"errors"
	"math"
	"testing"
)

func TestUnwrapTRPCSuccess(t *testing.T) {
	got, err := unwrapTRPC("page.book.recommendations", []byte(`[{"result":{"data":{"json":{"bff":{"recommendations":[]}}}}}]`))
	if err != nil {
		t.Fatalf("unwrapTRPC() error = %v", err)
	}
	if string(got) != `{"bff":{"recommendations":[]}}` {
		t.Fatalf("unwrapTRPC() = %s, want inner json", got)
	}
}

func TestUnwrapTRPCError(t *testing.T) {
	_, err := unwrapTRPC("page.book.recommendations", []byte(`[{"error":{"json":{"message":"[{\"path\":[\"locale\"]}]","data":{"code":"BAD_REQUEST"}}}}]`))
	if err == nil {
		t.Fatal("unwrapTRPC() error = nil, want TRPCError")
	}
	var trpcErr *TRPCError
	if !errors.As(err, &trpcErr) {
		t.Fatalf("unwrapTRPC() error = %T, want *TRPCError", err)
	}
	if trpcErr.Code != "BAD_REQUEST" {
		t.Fatalf("TRPCError.Code = %q, want %q", trpcErr.Code, "BAD_REQUEST")
	}
}

func TestParseRecommendationsPayload(t *testing.T) {
	taskers, histogram, err := parseRecommendationsPayload([]byte(`{
		"bff": {
			"recommendations": [
				{
					"id": "tasker-1",
					"user_id": 123,
					"first_name": "Ada",
					"display_name": "Ada L.",
					"poster_hourly_rate_cents": 3333,
					"poster_hourly_rate_currency": "USD",
					"rabbit_rating":                       "100%",
					"category_family_average_star_rating": 5.0,
					"rabbit_number_of_reviews": 17,
					"category_invoices_count": 8,
					"hours_worked": 42.5,
					"elite": true,
					"reliability_rate": "99%",
					"next_available_at": "2026-07-04T09:00:00Z",
					"is_favorite": true,
					"past_tasker": false,
					"two_hour_minimum_required_display": "2 hr minimum"
				}
			],
			"histogram": {
				"minimum_price_cents": 2500,
				"median_price_cents": 3333,
				"maximum_price_cents": 5000,
				"currency_code": "USD"
			}
		}
	}`))
	if err != nil {
		t.Fatalf("parseRecommendationsPayload() error = %v", err)
	}
	if len(taskers) != 1 {
		t.Fatalf("len(taskers) = %d, want 1", len(taskers))
	}
	if taskers[0].PosterHourlyRateCents != 3333 {
		t.Fatalf("PosterHourlyRateCents = %d, want 3333", taskers[0].PosterHourlyRateCents)
	}
	if math.Abs(taskers[0].RabbitRating-5.0) > 0.0001 {
		t.Fatalf("RabbitRating = %v, want 5.0", taskers[0].RabbitRating)
	}
	if histogram.MedianPriceCents != 3333 {
		t.Fatalf("MedianPriceCents = %d, want 3333", histogram.MedianPriceCents)
	}
}
