// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package tock

// PATCH: cross-network-source-clients (search) — see .printing-press-patches.json.
// Tock's geo-search surface is fully server-side rendered. The URL
//   GET /city/<city-slug>/search?city=<CityName>&date=YYYY-MM-DD
//       &latlng=<lat>%2C<lng>&size=<n>&time=HH%3AMM&type=DINE_IN_EXPERIENCES
// returns HTML with `window.$REDUX_STATE = {...}` carrying the full result
// set inline. No client-side XHR fires, no auth headers are required, and
// no protobuf is involved — search is anonymous-public.
//
// SSR shape (captured 2026-05-09 via chrome-MCP, Seattle metro):
//   state.availability.result.offeringAvailability  → []entry
//     entry.business: {id, name, domainName, businessType, cuisines,
//                      neighborhood, location: {lat, lng, address, city, state}}
//     entry.offering[].ticketGroup[].availableDateTime  (slot times; v1 ignores)
//     entry.ranking: {distanceMeters, relevanceScore}
//   state.app.config.metroArea  → 253 metros worldwide (deferred follow-up).
//
// Any future Tock SPA refactor that drops $REDUX_STATE will surface as a
// sentinel error here — not a silent zero-result success.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// LocationInput is the location signal a Tock SearchCity call needs.
// City is the display name (drives ?city= query param), Slug is the
// path slug (drives /search/<slug> path segment), Lat/Lng anchor the
// ranking. All four are required for the SSR fetch to land on the
// right page. Callers construct this via cli.GeoContext.ForTock().
//
// PATCH: location-native-redesign — typed projection of GeoContext.
type LocationInput struct {
	City string
	Slug string
	Lat  float64
	Lng  float64
}

// SearchParams holds the geo-search inputs for SearchCity. City is the
// display name (e.g., "Seattle", "New York") — the city-slug path segment
// is derived by lowercasing and replacing spaces with dashes.
type SearchParams struct {
	City      string  // display name, e.g. "Seattle"
	Date      string  // YYYY-MM-DD
	Time      string  // HH:MM (24h)
	PartySize int     // 1..N
	Lat       float64 // metro center latitude
	Lng       float64 // metro center longitude
}

// TockBusiness is a single venue returned from a Tock city-search SSR.
// Slug is `business.domainName` (Tock's canonical URL slug); URL is composed
// as Origin + "/" + Slug. DistanceMeters and RelevanceScore are Tock's
// own ranking signals; callers may use them or layer their own scoring.
type TockBusiness struct {
	ID             int
	Name           string
	Slug           string
	BusinessType   string
	Cuisine        string
	Neighborhood   string
	City           string
	State          string
	Latitude       float64
	Longitude      float64
	URL            string
	DistanceMeters float64
	RelevanceScore float64
}

// offeringAvailEntry mirrors the JSON shape Tock emits at
// state.availability.result.offeringAvailability[]. Fields we don't use
// (offering[].ticketGroup, etc.) are intentionally elided.
type offeringAvailEntry struct {
	Business struct {
		ID           int             `json:"id"`
		Name         string          `json:"name"`
		DomainName   string          `json:"domainName"`
		BusinessType string          `json:"businessType"`
		Cuisines     json.RawMessage `json:"cuisines"`
		Neighborhood string          `json:"neighborhood"`
		Location     struct {
			Address string  `json:"address"`
			City    string  `json:"city"`
			State   string  `json:"state"`
			Country string  `json:"country"`
			Lat     float64 `json:"lat"`
			Lng     float64 `json:"lng"`
		} `json:"location"`
	} `json:"business"`
	Ranking struct {
		DistanceMeters float64 `json:"distanceMeters"`
		RelevanceScore float64 `json:"relevanceScore"`
	} `json:"ranking"`
}

// searchTypeDineInExperiences is the only `?type=` value v1 supports. Tock
// has other enum values (EVENT, TAKEOUT, etc.) but DINE_IN_EXPERIENCES
// matches the typical reservation use case.
const searchTypeDineInExperiences = "DINE_IN_EXPERIENCES"

// SearchCity returns Tock venues that match a city + date + time + party.
// On success returns a (possibly empty) slice and nil error. On extractor
// failure (Tock SPA-refactored away from $REDUX_STATE, or HTML missing the
// availability subtree entirely) returns a wrapped error so callers can
// distinguish "no results" from "extraction broken".
func (c *Client) SearchCity(ctx context.Context, p SearchParams) ([]TockBusiness, error) {
	path := buildSearchPath(p)
	state, err := c.FetchReduxState(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("tock SearchCity: %w", err)
	}

	availMap, ok := state["availability"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("tock SearchCity: state.availability missing or wrong type — Tock SPA may have changed")
	}
	resultRaw, hasResult := availMap["result"]
	if !hasResult || resultRaw == nil {
		return []TockBusiness{}, nil
	}
	resultMap, ok := resultRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("tock SearchCity: state.availability.result is not an object")
	}
	rawList, hasList := resultMap["offeringAvailability"]
	if !hasList || rawList == nil {
		return []TockBusiness{}, nil
	}

	// Round-trip the subtree through JSON to land it in typed structs.
	// The subtree is small (~10–100 KB) compared to the full state, so the
	// re-marshal cost is negligible and the typed handling is much clearer.
	listJSON, err := json.Marshal(rawList)
	if err != nil {
		return nil, fmt.Errorf("tock SearchCity: re-marshaling offeringAvailability: %w", err)
	}
	var entries []offeringAvailEntry
	if err := json.Unmarshal(listJSON, &entries); err != nil {
		return nil, fmt.Errorf("tock SearchCity: decoding offeringAvailability: %w", err)
	}

	out := make([]TockBusiness, 0, len(entries))
	for _, e := range entries {
		out = append(out, entryToBusiness(e))
	}
	return out, nil
}

// buildSearchPath produces the URL path + query string for the Tock
// geo-search SSR. City display name becomes the `?city=` query param;
// the path segment is the lowercased, dash-joined slug.
func buildSearchPath(p SearchParams) string {
	citySlug := citySlugFromName(p.City)
	q := url.Values{}
	q.Set("city", p.City)
	q.Set("date", p.Date)
	q.Set("latlng", fmt.Sprintf("%g,%g", p.Lat, p.Lng))
	if p.PartySize > 0 {
		q.Set("size", fmt.Sprintf("%d", p.PartySize))
	}
	q.Set("time", p.Time)
	q.Set("type", searchTypeDineInExperiences)
	return "/city/" + citySlug + "/search?" + q.Encode()
}

// citySlugFromName lowercases and dash-joins a display name. "Seattle" →
// "seattle"; "New York" → "new-york". Tock's city-slug path segments use
// this exact convention.
func citySlugFromName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

// entryToBusiness flattens the SSR entry into TockBusiness. Cuisines may
// arrive as either `"Indian"` or `["Indian","Vegetarian"]`; both are
// joined into a single comma-space-separated string for downstream
// passthrough.
func entryToBusiness(e offeringAvailEntry) TockBusiness {
	return TockBusiness{
		ID:             e.Business.ID,
		Name:           e.Business.Name,
		Slug:           e.Business.DomainName,
		BusinessType:   e.Business.BusinessType,
		Cuisine:        decodeCuisines(e.Business.Cuisines),
		Neighborhood:   e.Business.Neighborhood,
		City:           e.Business.Location.City,
		State:          e.Business.Location.State,
		Latitude:       e.Business.Location.Lat,
		Longitude:      e.Business.Location.Lng,
		URL:            Origin + "/" + e.Business.DomainName,
		DistanceMeters: e.Ranking.DistanceMeters,
		RelevanceScore: e.Ranking.RelevanceScore,
	}
}

// MetroArea is one Tock metro entry from state.app.config.metroArea[].
// Roughly 253 metros worldwide as of 2026-05; covers far more than the
// 20-entry static fallback in the CLI's metro registry. Slug is Tock's
// path segment (`bellevue`, `new-york-city`); Name is the display
// shape used in `?city=` query params.
type MetroArea struct {
	Slug          string  `json:"slug"`
	Name          string  `json:"name"`
	Lat           float64 `json:"lat"`
	Lng           float64 `json:"lng"`
	BusinessCount int     `json:"businessCount,omitempty"`
}

// FetchMetroAreas pulls Tock's full metro registry from any city-search
// SSR (the metroArea config is identical across all city-search pages,
// since it's a config-tier value not derived from the path). Issue
// #406 deferred-TODO: replaces the static 20-entry CLI fallback with
// the 253-metro live list.
//
// Strategy: seed the SSR with a known-stable metro (Seattle) so a
// fresh install — with no prior queries — can still hydrate. The
// returned slice carries the full Tock-canonical shape; callers may
// project into a leaner Metro form for their own use.
//
// On HTML-fetch or extractor failure, returns the error so callers can
// retain their pre-#406 static fallback rather than masking the
// regression.
func (c *Client) FetchMetroAreas(ctx context.Context) ([]MetroArea, error) {
	// Pick the smallest plausible payload — `/city/<slug>/search` with
	// minimal query params. The metroArea config rides in the same
	// $REDUX_STATE regardless of city/date/time, so any city works.
	seed := SearchParams{
		City: "Seattle", Date: "2099-01-01", Time: "19:00",
		PartySize: 2, Lat: 47.6062, Lng: -122.3321,
	}
	path := buildSearchPath(seed)
	state, err := c.FetchReduxState(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("tock metroArea: fetch %s: %w", path, err)
	}
	return extractMetroAreas(state)
}

// extractMetroAreas pulls the metroArea array out of a parsed
// $REDUX_STATE tree. Separated from the HTTP fetch so tests can pin
// behavior with a fixture map.
func extractMetroAreas(state map[string]any) ([]MetroArea, error) {
	app, ok := state["app"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("tock metroArea: state.app missing or wrong type — Tock SPA may have refactored")
	}
	cfg, ok := app["config"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("tock metroArea: state.app.config missing or wrong type")
	}
	raw, ok := cfg["metroArea"]
	if !ok || raw == nil {
		return nil, fmt.Errorf("tock metroArea: state.app.config.metroArea absent")
	}
	rawJSON, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("tock metroArea: re-marshal: %w", err)
	}
	var metros []MetroArea
	if err := json.Unmarshal(rawJSON, &metros); err != nil {
		return nil, fmt.Errorf("tock metroArea: decoding: %w", err)
	}
	// Filter out malformed/empty entries — a metro with zero centroid is
	// useless for geo math and probably a Tock-side data hiccup.
	out := metros[:0]
	for _, m := range metros {
		if m.Slug == "" || m.Name == "" {
			continue
		}
		if m.Lat == 0 && m.Lng == 0 {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

// decodeCuisines accepts either a JSON string ("Indian") or a JSON array
// (["Indian","Vegetarian"]) and returns a flat string. Empty/null returns "".
func decodeCuisines(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return single
	}
	var multi []string
	if err := json.Unmarshal(raw, &multi); err == nil {
		return strings.Join(multi, ", ")
	}
	return ""
}
