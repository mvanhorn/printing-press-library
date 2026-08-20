// Copyright 2026 omarshahine. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Blacklane-native place resolution (not generator output).
// PATCH: resolve addresses via Blacklane's own public GraphQL endpoint
// (graphql.blacklane.com/public, no auth) — locationsAutocomplete -> placeId,
// then locationsGeocode -> coords. This yields the placeId + airportIata that
// /prices uses for accurate fares, which OSM geocoding can't provide.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const blPublicGraphQLURL = "https://graphql.blacklane.com/public"

const gqlLocationsAutocomplete = `query locationsAutocomplete($location: String!, $locale: AllowedLocales, $biasedPlaceId: String) {
  locationsAutocomplete(input: $location, locale: $locale, overExternal: true, biasedPlaceId: $biasedPlaceId) {
    address airportIata latitude longitude name placeId types timezone
  }
}`

const gqlLocationsGeocode = `query locationsGeocode($input: String!, $locale: AllowedLocales) {
  locationsGeocode(input: $input, locale: $locale, inputType: PlaceId) {
    placeId name address airportIata latitude longitude city countryCode timezone types
  }
}`

// publicGraphQL runs an unauthenticated operation against the /public endpoint.
func publicGraphQL(operationName, query string, variables map[string]any, timeout time.Duration) (json.RawMessage, error) {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	payload, _ := json.Marshal(map[string]any{
		"operationName": operationName,
		"query":         query,
		"variables":     variables,
	})
	req, _ := http.NewRequest(http.MethodPost, blPublicGraphQLURL, strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apollographql-client-name", "web")
	req.Header.Set("apollographql-client-version", "0.0.0")
	req.Header.Set("Origin", "https://www.blacklane.com")
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var env struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("public graphql %s: bad response", operationName)
	}
	if len(env.Errors) > 0 {
		return nil, fmt.Errorf("public graphql %s: %s", operationName, env.Errors[0].Message)
	}
	return env.Data, nil
}

type blLocation struct {
	Address     string   `json:"address"`
	AirportIata string   `json:"airportIata"`
	Latitude    *float64 `json:"latitude"`
	Longitude   *float64 `json:"longitude"`
	Name        string   `json:"name"`
	PlaceID     string   `json:"placeId"`
	Types       []string `json:"types"`
}

// blacklaneResolve resolves a free-text query to a geoPoint via Blacklane's own
// autocomplete + geocode (two steps: text -> placeId, placeId -> coords).
func blacklaneResolve(query string, timeout time.Duration) (geoPoint, error) {
	// Step 1: autocomplete -> first suggestion (carries placeId + airportIata).
	data, err := publicGraphQL("locationsAutocomplete", gqlLocationsAutocomplete,
		map[string]any{"location": query, "locale": "en"}, timeout)
	if err != nil {
		return geoPoint{}, err
	}
	var ac struct {
		LocationsAutocomplete []blLocation `json:"locationsAutocomplete"`
	}
	if err := json.Unmarshal(data, &ac); err != nil || len(ac.LocationsAutocomplete) == 0 {
		return geoPoint{}, fmt.Errorf("no Blacklane match for %q", query)
	}
	top := ac.LocationsAutocomplete[0]

	// Step 2: geocode the placeId -> coordinates.
	gdata, err := publicGraphQL("locationsGeocode", gqlLocationsGeocode,
		map[string]any{"input": top.PlaceID, "locale": "en"}, timeout)
	if err != nil {
		return geoPoint{}, err
	}
	// locationsGeocode returns either an array or a single object depending on input.
	var arr struct {
		LocationsGeocode []blLocation `json:"locationsGeocode"`
	}
	var g blLocation
	if json.Unmarshal(gdata, &arr) == nil && len(arr.LocationsGeocode) > 0 {
		g = arr.LocationsGeocode[0]
	} else {
		var one struct {
			LocationsGeocode blLocation `json:"locationsGeocode"`
		}
		if err := json.Unmarshal(gdata, &one); err != nil {
			return geoPoint{}, fmt.Errorf("geocoding Blacklane place for %q failed", query)
		}
		g = one.LocationsGeocode
	}
	if g.Latitude == nil || g.Longitude == nil {
		return geoPoint{}, fmt.Errorf("no coordinates for %q", query)
	}
	addr := g.Address
	if addr == "" {
		addr = top.Address
	}
	iata := g.AirportIata
	if iata == "" {
		iata = top.AirportIata
	}
	pid := g.PlaceID
	if pid == "" {
		pid = top.PlaceID
	}
	return geoPoint{
		Address:     addr,
		Latitude:    *g.Latitude,
		Longitude:   *g.Longitude,
		PlaceID:     pid,
		AirportIata: iata,
	}, nil
}
