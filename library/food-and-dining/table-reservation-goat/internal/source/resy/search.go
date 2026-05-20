// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

// PATCH: resy-source-port — see .printing-press-patches.json for the change-set rationale.

package resy

import (
	"context"
	"encoding/json"
	"fmt"
)

// SearchParams are the inputs to Search. City is the Resy city CODE (e.g.
// "ny", "la", "sea"), not a display name — matches the TS client behavior
// and resy-cli's `location` parameter.
type SearchParams struct {
	Query string
	City  string
	Limit int
}

// resySearchHit mirrors the wire shape. Both `id.resy` and `objectID`
// surfaces have been observed in real responses, and both can carry the
// venue ID — we accept either.
//
// Coordinates: live Resy responses (verified 2026-05-11) carry venue
// coordinates under `_geoloc.{lat,lng}` (Algolia-style indexing convention),
// NOT under top-level `latitude` / `longitude`. The top-level fields are
// kept for forward compatibility but the parser prefers `_geoloc` when
// present.
type resySearchHit struct {
	ID *struct {
		Resy json.RawMessage `json:"resy"`
	} `json:"id"`
	ObjectID string `json:"objectID"`
	Name     string `json:"name"`
	City     string `json:"city"`
	Region   string `json:"region"`
	URLSlug  string `json:"url_slug"`
	Location *struct {
		Name    string `json:"name"`
		Code    string `json:"code"`
		URLSlug string `json:"url_slug"`
	} `json:"location"`
	GeoLoc *struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	} `json:"_geoloc"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type resySearchResponse struct {
	Search *struct {
		Hits []resySearchHit `json:"hits"`
	} `json:"search"`
	Hits []resySearchHit `json:"hits"`
}

// Search runs a venue search and returns the parsed Venue list.
func (c *Client) Search(ctx context.Context, params SearchParams) ([]Venue, error) {
	body, err := c.rawSearch(ctx, params.Query, params.City, params.Limit)
	if err != nil {
		return nil, err
	}
	return ParseSearchResponse(body)
}

// ParseSearchResponse turns a raw /3/venuesearch/search body into Venue
// rows. Exported so tests can pin the shape without an HTTP stub.
func ParseSearchResponse(raw []byte) ([]Venue, error) {
	var r resySearchResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("resy: parse search response: %w", err)
	}
	// Prefer the nested `search.hits` envelope (modern shape) when it
	// carries rows, but DON'T blindly overwrite `r.Hits` with an empty
	// nested array — Resy has been observed returning either envelope
	// alone, and a payload carrying both with the nested one empty
	// would silently lose the top-level data.
	hits := r.Hits
	if r.Search != nil && len(r.Search.Hits) > 0 {
		hits = r.Search.Hits
	}
	out := make([]Venue, 0, len(hits))
	for _, h := range hits {
		id := h.ObjectID
		if id == "" && h.ID != nil && len(h.ID.Resy) > 0 {
			id = unquoteJSON(h.ID.Resy)
		}
		if id == "" || h.Name == "" {
			continue
		}
		cityCode := h.City
		cityName := h.City
		if h.Location != nil {
			if h.Location.Code != "" {
				cityCode = h.Location.Code
			}
			if h.Location.Name != "" {
				cityName = h.Location.Name
			}
		}
		lat := h.Latitude
		lng := h.Longitude
		if h.GeoLoc != nil && (h.GeoLoc.Lat != 0 || h.GeoLoc.Lng != 0) {
			lat = h.GeoLoc.Lat
			lng = h.GeoLoc.Lng
		}
		v := Venue{
			ID:        id,
			Name:      h.Name,
			City:      cityName,
			CityCode:  cityCode,
			Region:    h.Region,
			Slug:      h.URLSlug,
			Latitude:  lat,
			Longitude: lng,
		}
		if cityCode != "" && h.URLSlug != "" {
			v.URL = fmt.Sprintf("%s/cities/%s/%s", Origin, cityCode, h.URLSlug)
		}
		out = append(out, v)
	}
	return out, nil
}

// unquoteJSON pulls the inner value out of either a quoted string or a bare
// number. Resy's search payload sends id.resy as a number on some endpoints
// and a string on others.
func unquoteJSON(raw json.RawMessage) string {
	s := string(raw)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
