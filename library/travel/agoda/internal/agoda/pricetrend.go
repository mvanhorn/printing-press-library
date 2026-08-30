// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.

package agoda

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"
)

// priceTrendTemplate is the captured priceTrendSearch operation. Unlike
// citySearch this one is small and ships its full query document.
//
//go:embed queries/pricetrend_request.json
var priceTrendTemplate []byte

// PriceTrend returns per-property prices across a whole date window in one call.
//
// This is Agoda's own operation, which is why a multi-week sweep costs a single
// request here rather than one search per candidate date.
func (c *Client) PriceTrend(ctx context.Context, cityID int, start, end time.Time, nights, occupancy int, currency string, authenticated bool) ([]TrendPoint, error) {
	var payload map[string]any
	if err := json.Unmarshal(priceTrendTemplate, &payload); err != nil {
		return nil, fmt.Errorf("decoding priceTrend template: %w", err)
	}
	vars, ok := payload["variables"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("priceTrend template missing variables")
	}
	req, ok := vars["PriceTrendSearchRequest"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("priceTrend template missing PriceTrendSearchRequest")
	}
	req["cityId"] = cityID
	req["startDate"] = start.UTC().Format("2006-01-02T15:04:05.000Z")
	req["endDate"] = end.UTC().Format("2006-01-02T15:04:05.000Z")
	req["los"] = nights
	req["occ"] = occupancy
	req["currency"] = currency
	req["isUserLoggedIn"] = authenticated

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	data, err := c.postGraphQL(ctx, "/graphql/search", body, searchPageTypeID)
	if err != nil {
		return nil, err
	}

	// PriceTrendSearchDetails is a single object for a city-level query and an
	// array when scoped to properties. Decode it as raw JSON and accept both
	// rather than guessing, since a shape mismatch would otherwise surface as
	// "no cheap dates found" instead of a decode error.
	var env struct {
		Data struct {
			PriceTrendSearch struct {
				Details json.RawMessage `json:"PriceTrendSearchDetails"`
			} `json:"priceTrendSearch"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("decoding priceTrend response: %w", err)
	}

	type trendDetail struct {
		CityID     int  `json:"cityId"`
		PropertyID *int `json:"propertyId"`
		Prices     []struct {
			CheckIn        string   `json:"checkIn"`
			Price          *float64 `json:"price"`
			PriceTrendType string   `json:"priceTrendType"`
		} `json:"prices"`
	}

	raw := env.Data.PriceTrendSearch.Details
	var details []trendDetail
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &details); err != nil {
			var single trendDetail
			if err2 := json.Unmarshal(raw, &single); err2 != nil {
				return nil, fmt.Errorf("decoding priceTrend details: %w", err)
			}
			details = []trendDetail{single}
		}
	}

	out := make([]TrendPoint, 0)
	for _, detail := range details {
		propertyID := 0
		if detail.PropertyID != nil {
			propertyID = *detail.PropertyID
		}
		for _, p := range detail.Prices {
			// A null or non-positive price means "no availability on this
			// date", which is meaningfully different from a price of zero.
			// Dropping these keeps an unavailable date from ranking as free.
			if p.Price == nil || *p.Price <= 0 {
				continue
			}
			out = append(out, TrendPoint{
				PropertyID: propertyID,
				CheckIn:    normalizeDate(p.CheckIn),
				Price:      round2(*p.Price),
				TrendType:  p.PriceTrendType,
			})
		}
	}
	return out, nil
}

func normalizeDate(s string) string {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05.000Z", "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return s
}
