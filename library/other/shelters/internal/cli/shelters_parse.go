// Copyright 2026 Abe Diaz (@abe238) and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Shared parsing/flattening helpers for the novel shelters commands (shelters,
// shelter, near, capacity, brief). Pure functions over already-fetched bytes so
// they unit-test against the real + synthetic fixtures without any network. The
// only network helpers (httpGetJSON, censusGeocode) are isolated here but are
// never reached by the parsers themselves.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// userAgent is the descriptive UA every request carries. The generated client
// already sends it for the OpenShelters feed; the geocoder helper below sets it
// explicitly. No contact email by design (GitHub URL only).
const userAgent = "shelters-pp-cli (https://github.com/mvanhorn/printing-press-library)"

// FEMA National Shelter System OpenShelters service (the authoritative feed).
const (
	openSheltersBase  = "https://gis.fema.gov"
	openSheltersQuery = "/arcgis/rest/services/NSS/OpenShelters/FeatureServer/0/query"
	// featureServerURL is the stable layer URL surfaced by `gis-links`.
	featureServerURL = "https://gis.fema.gov/arcgis/rest/services/NSS/OpenShelters/FeatureServer/0"
	// fullNSSInfoURL points to the broader NSS program (full access needs an MOU).
	fullNSSInfoURL = "https://www.fema.gov/emergency-managers/practitioners/national-mass-care-strategy"
	// censusGeocoderBase is the free, key-less US Census geocoder.
	censusGeocoderBase = "https://geocoding.geo.census.gov/geocoder/locations/onelineaddress"
)

// earthRadiusMiles is the mean Earth radius used for haversine. Verified against
// the canonical published vector (36.12,-86.67)->(33.94,-118.40) = 1793.56 mi.
const earthRadiusMiles = 3958.7613

// maxFeedBytes bounds a feed/geocoder response so a hostile or runaway body
// cannot exhaust memory. The live feed is a few KB; an active event is low MB.
const maxFeedBytes = 32 << 20 // 32 MiB

// envelope is the small machine envelope every novel command emits:
// {"source": "<url|fixture>", "fetched_at": "<ISO8601 UTC>", "data": {...}}.
// fetched_at is recorded client-side because the feed carries no server stamp.
type envelope struct {
	Source    string `json:"source"`
	FetchedAt string `json:"fetched_at"`
	Data      any    `json:"data"`
}

func newEnvelope(source string, data any) envelope {
	return envelope{Source: source, FetchedAt: time.Now().UTC().Format(time.RFC3339), Data: data}
}

// Shelter is one flattened OpenShelters record. Nullable numeric/geo fields are
// pointers so "unreported" (null) is distinct from a real zero. Coded string
// fields are normalized (trim + uppercase) so filters match regardless of the
// feed's NONE/None/" " and YES/blank inconsistencies.
type Shelter struct {
	ShelterID            int      `json:"shelter_id"`
	ObjectID             *int     `json:"objectid"`
	Name                 string   `json:"shelter_name"`
	Address              string   `json:"address"`
	City                 string   `json:"city"`
	State                string   `json:"state"`
	Zip                  string   `json:"zip"`
	Status               string   `json:"shelter_status"`
	EvacuationCapacity   *int     `json:"evacuation_capacity"`
	PostImpactCapacity   *int     `json:"post_impact_capacity"`
	TotalPopulation      *int     `json:"total_population"`
	HoursOpen            string   `json:"hours_open"`
	HoursClose           string   `json:"hours_close"`
	OrgName              string   `json:"org_name"`
	OrgID                *int     `json:"org_id"`
	MatchType            string   `json:"match_type"`
	SubfacilityCode      string   `json:"subfacility_code"`
	ADACompliant         string   `json:"ada_compliant"`
	PetAccommodations    string   `json:"pet_accommodations_code"`
	WheelchairAccessible string   `json:"wheelchair_accessible"`
	Latitude             *float64 `json:"latitude"`
	Longitude            *float64 `json:"longitude"`
}

// arcgisResponse decodes either the feature collection or an ArcGIS error. The
// service returns HTTP 200 even for query errors, carrying {"error": {...}},
// so the error object must be inspected explicitly.
type arcgisResponse struct {
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Features []struct {
		Attributes map[string]any `json:"attributes"`
	} `json:"features"`
}

// parseShelters decodes a raw OpenShelters query response into flattened,
// normalized Shelter records. The bytes may also be a bare features array (the
// shape after the generated response_path strips to "features"), so both the
// full envelope and the bare array are accepted.
func parseShelters(raw []byte) ([]Shelter, error) {
	var resp arcgisResponse
	// recognized is true once we have decoded a shape we understand: either an
	// error object, or a (possibly empty) features collection / bare array. A
	// valid-JSON-but-wrong-shape payload (`{}`, `null`, `{"features":null}`, a
	// CDN/WAF envelope) leaves it false so the caller fails loudly instead of
	// reporting a broken feed as "0 open shelters".
	recognized := false
	if err := json.Unmarshal(raw, &resp); err != nil {
		// Maybe the bytes are a bare [{"attributes": {...}}, ...] array.
		var bare []struct {
			Attributes map[string]any `json:"attributes"`
		}
		if berr := json.Unmarshal(raw, &bare); berr != nil {
			return nil, fmt.Errorf("parsing shelter feed: %w", err)
		}
		for i := range bare {
			resp.Features = append(resp.Features, struct {
				Attributes map[string]any `json:"attributes"`
			}{Attributes: bare[i].Attributes})
		}
		recognized = true // a JSON array (even empty) is a valid empty result
	} else if resp.Error != nil || resp.Features != nil {
		// resp.Features is non-nil for {"features":[...]} including [] (real
		// empty); it is nil for {} / null / {"features":null}.
		recognized = true
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("FEMA OpenShelters service error %d: %s", resp.Error.Code, resp.Error.Message)
	}
	if !recognized {
		return nil, fmt.Errorf("unrecognized OpenShelters response: valid JSON but no 'features' array and no 'error' (the feed shape may have changed)")
	}
	shelters := make([]Shelter, 0, len(resp.Features))
	for _, f := range resp.Features {
		a := f.Attributes
		if a == nil {
			continue
		}
		s := Shelter{
			ObjectID:             attrIntPtr(a, "objectid"),
			Name:                 attrStr(a, "shelter_name"),
			Address:              attrStr(a, "address"),
			City:                 attrStr(a, "city"),
			State:                normCode(attrStr(a, "state")),
			Zip:                  strings.TrimSpace(attrStr(a, "zip")),
			Status:               normCode(attrStr(a, "shelter_status")),
			EvacuationCapacity:   attrIntPtr(a, "evacuation_capacity"),
			PostImpactCapacity:   attrIntPtr(a, "post_impact_capacity"),
			TotalPopulation:      attrIntPtr(a, "total_population"),
			HoursOpen:            attrStr(a, "hours_open"),
			HoursClose:           attrStr(a, "hours_close"),
			OrgName:              attrStr(a, "org_name"),
			OrgID:                attrIntPtr(a, "org_id"),
			MatchType:            normCode(attrStr(a, "match_type")),
			SubfacilityCode:      attrStr(a, "subfacility_code"),
			ADACompliant:         normCode(attrStr(a, "ada_compliant")),
			PetAccommodations:    normCode(attrStr(a, "pet_accommodations_code")),
			WheelchairAccessible: normCode(attrStr(a, "wheelchair_accessible")),
			Latitude:             attrFloatPtr(a, "latitude"),
			Longitude:            attrFloatPtr(a, "longitude"),
		}
		if id := attrIntPtr(a, "shelter_id"); id != nil {
			s.ShelterID = *id
		}
		shelters = append(shelters, s)
	}
	return shelters, nil
}

// attrStr reads a string attribute, trimming surrounding whitespace. Missing or
// non-string values yield "".
func attrStr(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// attrIntPtr reads a numeric attribute as *int. JSON numbers decode to float64,
// so both 500 and 500.0 are handled; null/missing yields nil.
func attrIntPtr(m map[string]any, k string) *int {
	if v, ok := m[k].(float64); ok {
		i := int(math.Round(v))
		return &i
	}
	return nil
}

// attrFloatPtr reads a numeric attribute as *float64 (used for lat/lon).
func attrFloatPtr(m map[string]any, k string) *float64 {
	if v, ok := m[k].(float64); ok {
		return &v
	}
	return nil
}

// normCode normalizes a coded field for matching: trim then uppercase. This
// collapses the feed's "NONE"/"None"/" " and "YES"/"" inconsistencies. A blank
// or "NONE"-ish value is preserved as "" / "NONE" so callers can distinguish.
func normCode(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// allowsPets reports whether a pet_accommodations_code permits pets. Per FEMA's
// own "Pet Accommodations" layer the accepting codes are exactly COHABIT (pets
// stay with their owner) and ONSITE (pets sheltered onsite). NONE/blank do not.
func allowsPets(code string) bool {
	switch normCode(code) {
	case "COHABIT", "ONSITE":
		return true
	}
	return false
}

// isYes reports whether an ADA/wheelchair code is a confirmed YES. UNK, NO, and
// blank are all treated as "not confirmed" (never claim accessibility we lack).
func isYes(code string) bool {
	return normCode(code) == "YES"
}

// haversineMiles returns the great-circle distance in miles between two
// lat/lon points (decimal degrees). Verified against the canonical published
// vector in shelters_parse_test.go.
func haversineMiles(lat1, lon1, lat2, lon2 float64) float64 {
	p1 := lat1 * math.Pi / 180
	p2 := lat2 * math.Pi / 180
	dp := (lat2 - lat1) * math.Pi / 180
	dl := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return 2 * earthRadiusMiles * math.Asin(math.Sqrt(a))
}

// ---------------------------------------------------------------------------
// Input loading + geocoding (the only network code)
// ---------------------------------------------------------------------------

// loadFixture reads a fixture file or stdin (when path is "-").
func loadFixture(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(io.LimitReader(os.Stdin, maxFeedBytes))
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading fixture %q: %w", path, err)
	}
	return b, nil
}

// httpGetJSON fetches a URL with the descriptive UA and a size cap, returning
// the raw bytes. Used by the geocoder; the OpenShelters feed goes through the
// generated client. Timeouts ride the passed context.
func httpGetJSON(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json,*/*")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s returned HTTP %d", rawURL, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxFeedBytes {
		return nil, fmt.Errorf("GET %s exceeded %d-byte cap", rawURL, maxFeedBytes)
	}
	return body, nil
}

// latlon is a resolved coordinate.
type latlon struct {
	Lat float64 `json:"latitude"`
	Lon float64 `json:"longitude"`
}

// geocodeOneLine resolves a one-line address/place to coordinates. It is a
// package var so tests can stub it without touching the network. The default is
// the US Census geocoder (free, no key). ok is false (with nil error) when the
// geocoder simply returns no match, so callers can skip-with-a-count rather than
// failing the whole command.
var geocodeOneLine = censusGeocode

// censusGeocode queries the US Census one-line-address geocoder.
func censusGeocode(ctx context.Context, oneLine string) (latlon, bool, error) {
	oneLine = strings.TrimSpace(oneLine)
	if oneLine == "" {
		return latlon{}, false, nil
	}
	u := censusGeocoderBase + "?benchmark=Public_AR_Current&format=json&address=" + url.QueryEscape(oneLine)
	body, err := httpGetJSON(ctx, u)
	if err != nil {
		return latlon{}, false, err
	}
	var parsed struct {
		Result struct {
			AddressMatches []struct {
				Coordinates struct {
					X float64 `json:"x"`
					Y float64 `json:"y"`
				} `json:"coordinates"`
			} `json:"addressMatches"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return latlon{}, false, fmt.Errorf("parsing geocoder response: %w", err)
	}
	if len(parsed.Result.AddressMatches) == 0 {
		return latlon{}, false, nil
	}
	c := parsed.Result.AddressMatches[0].Coordinates
	return latlon{Lat: c.Y, Lon: c.X}, true, nil
}

// shelterOneLine builds a geocodable one-line address from a shelter's parts.
func shelterOneLine(s Shelter) string {
	parts := []string{}
	for _, p := range []string{s.Address, s.City, s.State, s.Zip} {
		if strings.TrimSpace(p) != "" {
			parts = append(parts, strings.TrimSpace(p))
		}
	}
	return strings.Join(parts, ", ")
}
