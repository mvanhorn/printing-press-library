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
	"strconv"
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
	// femaNSSQuery / femaNSSLayerURL are the richer FEMA_NSS layer the enrichment
	// step reads (county, incident, generator, populations, ...). Same host as
	// OpenShelters; see shelters_enrich.go.
	femaNSSQuery    = "/arcgis/rest/services/NSS/FEMA_NSS/FeatureServer/0/query"
	femaNSSLayerURL = "https://gis.fema.gov/arcgis/rest/services/NSS/FEMA_NSS/FeatureServer/0"
	// Red Cross Emergency-Action view: the feed the public redcross.org
	// "find an open shelter" map uses. A DIFFERENT org (services.arcgis.com),
	// fields are active_*, geometry is Web Mercator unless outSR=4326 is set, and
	// the endpoint gates on a Referer header. FEMA's feed is synchronized from the
	// Red Cross database (every morning, then every 20 min), so a freshly opened
	// shelter can appear in Red Cross up to a day before FEMA: unioning both is the
	// only way to get every currently-open shelter. See redcross.go.
	redCrossBase     = "https://services.arcgis.com"
	redCrossQuery    = "/pGfbNJoYypmNq86F/arcgis/rest/services/Shelters_Prod_EA_view/FeatureServer/0/query"
	redCrossLayerURL = "https://services.arcgis.com/pGfbNJoYypmNq86F/arcgis/rest/services/Shelters_Prod_EA_view/FeatureServer/0"
	// ARC Open_Shelters operational layer: same ARC org (services.arcgis.com) as
	// the Red Cross EA view above, but this is the ONLY feed that carries live
	// occupancy -- total_population, the general/medical/other/pet breakdown -- plus
	// the driving incident. Its SHELTER_ID is an ARC-internal id (NOT FEMA's stable
	// shelter_id), so it is joined onto the union by physical identity
	// (name+state+ZIP), not by id. No Referer is needed (unlike the EA view) and the
	// attribute keys are UPPERCASE. See occupancy.go.
	openSheltersOccQuery    = "/pGfbNJoYypmNq86F/arcgis/rest/services/Open_Shelters/FeatureServer/0/query"
	openSheltersOccLayerURL = "https://services.arcgis.com/pGfbNJoYypmNq86F/arcgis/rest/services/Open_Shelters/FeatureServer/0"
	// fullNSSInfoURL points to the broader NSS program (full access needs an MOU).
	fullNSSInfoURL = "https://www.fema.gov/emergency-managers/practitioners/national-mass-care-strategy"
	// censusGeocoderBase is the free, key-less US Census geocoder (street-level;
	// it does NOT resolve a bare ZIP).
	censusGeocoderBase = "https://geocoding.geo.census.gov/geocoder/locations/onelineaddress"
	// censusZCTAQuery is the free, key-less US Census TIGERweb ZIP Code Tabulation
	// Area layer, used to resolve a bare 5-digit ZIP to its area interior-point
	// centroid (the onelineaddress geocoder above returns nothing for a bare ZIP).
	// Government source, same trust tier as the FEMA feed and the address geocoder.
	censusZCTAQuery = "https://tigerweb.geo.census.gov/arcgis/rest/services/TIGERweb/PUMA_TAD_TAZ_UGA_ZCTA/MapServer/1/query"
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

	// --- FEMA_NSS/0 enrichment (best-effort, joined by shelter_id) ---
	// These come from FEMA's richer FEMA_NSS/FeatureServer/0 layer, merged onto
	// the OpenShelters spine in shelters_enrich.go. They are empty/nil when
	// enrichment is skipped (--no-enrich, --data-source local, --fixture) or the
	// enrichment fetch failed; coded fields keep the feed's literal UNK/NONE so a
	// real "unreported" is never silently rendered as a confirmed value. The
	// enrichment status (attempted / ok / note) rides alongside in each command's
	// envelope so a consumer can tell genuine nulls from a missed fetch.
	CountyParish              string `json:"county_parish"`
	FacilityType              string `json:"facility_type"`
	ShelterStatusCode         string `json:"shelter_status_code"`
	IncidentName              string `json:"incident_name"`
	IncidentNumber            string `json:"incident_number"`
	ShelterOpenDate           string `json:"shelter_open_date"`
	ShelterClosedDate         string `json:"shelter_closed_date"`
	GeneratorOnsite           string `json:"generator_onsite"`
	SelfSufficientElectricity string `json:"self_sufficient_electricity"`
	In100YrFloodplain         string `json:"in_100_yr_floodplain"`
	In500YrFloodplain         string `json:"in_500_yr_floodplain"`
	InSurgeSloshArea          string `json:"in_surge_slosh_area"`
	PreLandfallShelter        string `json:"pre_landfall_shelter"`
	PopulationCode            string `json:"population_code"`
	GeneralPopulation         *int   `json:"general_population"`
	MedicalNeedsPopulation    *int   `json:"medical_needs_population"`
	OtherPopulation           *int   `json:"other_population"`
	PetPopulation             *int   `json:"pet_population"`
	PetAccommodationsDesc     string `json:"pet_accommodations_desc"`
	OrgMainPhone              string `json:"org_main_phone"`
	OrgHotlinePhone           string `json:"org_hotline_phone"`

	// --- provenance ---
	// Source is which feed this record came from: "fema" (OpenShelters spine),
	// "redcross" (Red Cross EA view), or "fema+redcross" (present in both and
	// merged). RedCrossID carries the Red Cross site id for rows that have one
	// (RC has no FEMA shelter_id), so a consumer can cross-reference the RC map.
	Source     string `json:"source"`
	RedCrossID string `json:"red_cross_id,omitempty"`
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

// decodeFeatures decodes a raw ArcGIS query response (or a bare features array)
// into the slice of per-feature attribute maps, failing loudly on a
// valid-JSON-but-wrong-shape payload. Both parseShelters (OpenShelters spine)
// and parseEnrichment (FEMA_NSS layer) share it so the fail-loud contract lives
// in exactly one place.
func decodeFeatures(raw []byte) ([]map[string]any, error) {
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
		return nil, fmt.Errorf("ArcGIS service error %d: %s", resp.Error.Code, resp.Error.Message)
	}
	if !recognized {
		return nil, fmt.Errorf("unrecognized ArcGIS response: valid JSON but no 'features' array and no 'error' (the feed shape may have changed)")
	}
	attrs := make([]map[string]any, 0, len(resp.Features))
	for _, f := range resp.Features {
		if f.Attributes != nil {
			attrs = append(attrs, f.Attributes)
		}
	}
	return attrs, nil
}

// parseShelters decodes a raw OpenShelters query response into flattened,
// normalized Shelter records. The bytes may also be a bare features array (the
// shape after the generated response_path strips to "features"), so both the
// full envelope and the bare array are accepted.
func parseShelters(raw []byte) ([]Shelter, error) {
	attrs, err := decodeFeatures(raw)
	if err != nil {
		return nil, err
	}
	shelters := make([]Shelter, 0, len(attrs))
	for _, a := range attrs {
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
			Source:               "fema",
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
	return httpGet(ctx, rawURL, "")
}

// httpGet is httpGetJSON with an optional Referer header. The Red Cross EA view
// gates on Referer, so its fetch passes "https://www.redcross.org/"; an empty
// referer omits the header (the FEMA/Census endpoints need none).
func httpGet(ctx context.Context, rawURL, referer string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json,*/*")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
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

// zipToLatLon resolves a 5-digit US ZIP to coordinates. It is a package var so
// tests can stub it without touching the network. The default is the US Census
// ZCTA layer (free, no key, government source); the returned point is the ZIP
// area's interior-point centroid, so it is coarser than a street-level geocode.
// ok is false (with nil error) when the ZIP has no matching ZCTA.
var zipToLatLon = censusZCTALookup

// censusZCTALookup resolves a bare ZIP via the Census TIGERweb ZCTA layer. zipCode
// must be exactly five digits; the caller (resolveOrigin) guarantees this via a
// regex, and this function re-validates before interpolating it into the ArcGIS
// where clause so a non-numeric value can never reach the query.
func censusZCTALookup(ctx context.Context, zipCode string) (latlon, bool, error) {
	zipCode = strings.TrimSpace(zipCode)
	if len(zipCode) != 5 {
		return latlon{}, false, nil
	}
	for _, r := range zipCode {
		if r < '0' || r > '9' {
			return latlon{}, false, nil
		}
	}
	v := url.Values{}
	v.Set("where", "ZCTA5='"+zipCode+"'")
	v.Set("outFields", "INTPTLAT,INTPTLON")
	v.Set("returnGeometry", "false")
	v.Set("f", "json")
	body, err := httpGetJSON(ctx, censusZCTAQuery+"?"+v.Encode())
	if err != nil {
		return latlon{}, false, err
	}
	var parsed struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Features []struct {
			Attributes struct {
				Lat string `json:"INTPTLAT"`
				Lon string `json:"INTPTLON"`
			} `json:"attributes"`
		} `json:"features"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return latlon{}, false, fmt.Errorf("parsing ZCTA response: %w", err)
	}
	if parsed.Error != nil {
		return latlon{}, false, fmt.Errorf("Census ZCTA service error: %s", parsed.Error.Message)
	}
	if len(parsed.Features) == 0 {
		return latlon{}, false, nil
	}
	lat, err1 := strconv.ParseFloat(strings.TrimSpace(parsed.Features[0].Attributes.Lat), 64)
	lon, err2 := strconv.ParseFloat(strings.TrimSpace(parsed.Features[0].Attributes.Lon), 64)
	if err1 != nil || err2 != nil {
		return latlon{}, false, fmt.Errorf("parsing ZCTA coordinates for %s", zipCode)
	}
	return latlon{Lat: lat, Lon: lon}, true, nil
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
