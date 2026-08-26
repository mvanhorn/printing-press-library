// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.

package agoda

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// suggestPath is Agoda's destination autocomplete. The numeric path segments are
// storefront/language/platform routing constants observed in live traffic; they
// are not user-tunable.
const suggestPath = "/api/cronos/search/GetUnifiedSuggestResult/3/16/1/0/en-us/"

// suggestEnvelope is the subset of the autocomplete response we consume. Agoda
// returns PascalCase keys here (a different convention from its GraphQL surface).
type suggestEnvelope struct {
	ViewModelList []struct {
		ObjectId   int     `json:"ObjectId"`
		CityId     int     `json:"CityId"`
		CountryId  int     `json:"CountryId"`
		Name       string  `json:"Name"`
		ResultText string  `json:"ResultText"`
		NoOfHotels int     `json:"NoOfHotels"`
		IsHotel    bool    `json:"IsHotel"`
		Latitude   float64 `json:"Latitude"`
		Longtitude float64 `json:"Longtitude"`
	} `json:"ViewModelList"`
}

// ResolveDestination turns free text into the numeric city id every Agoda search
// requires. Agoda publishes no city-id table, so this call is the only way to go
// from "Tokyo" to 5085.
func (c *Client) ResolveDestination(ctx context.Context, text string) ([]Destination, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("destination text is required")
	}
	q := url.Values{}
	q.Set("searchText", text)
	q.Set("origin", "US")
	q.Set("cid", "-1")
	q.Set("pageTypeId", "1")
	q.Set("logTypeId", "1")
	q.Set("isHotelLandSearch", "true")

	target := c.baseURL() + suggestPath + "?" + q.Encode()
	data, err := c.do(ctx, http.MethodGet, target, nil, map[string]string{
		"X-Requested-With": "XMLHttpRequest",
	})
	if err != nil {
		return nil, err
	}
	var env suggestEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("decoding suggest response: %w", err)
	}
	out := make([]Destination, 0, len(env.ViewModelList))
	for _, v := range env.ViewModelList {
		id := v.CityId
		if id == 0 {
			// Agoda reports the city id in ObjectId for city-typed suggestions
			// and in CityId for narrower ones (areas, landmarks, properties).
			id = v.ObjectId
		}
		if id == 0 || v.Name == "" {
			continue
		}
		out = append(out, Destination{
			CityID:     id,
			Name:       v.Name,
			ResultText: v.ResultText,
			CountryID:  v.CountryId,
			HotelCount: v.NoOfHotels,
			IsHotel:    v.IsHotel,
			Latitude:   v.Latitude,
			Longitude:  v.Longtitude,
		})
	}
	return out, nil
}

// ResolveCityID returns the single best city id for a destination string.
//
// It prefers a city-shaped suggestion over a property-shaped one, because
// searching "Tokyo" should return a city of hotels rather than a hotel named
// after the city.
func (c *Client) ResolveCityID(ctx context.Context, text string) (Destination, error) {
	results, err := c.ResolveDestination(ctx, text)
	if err != nil {
		return Destination{}, err
	}
	if len(results) == 0 {
		return Destination{}, fmt.Errorf("no Agoda destination matched %q", text)
	}
	for _, r := range results {
		if !r.IsHotel {
			return r, nil
		}
	}
	return results[0], nil
}
