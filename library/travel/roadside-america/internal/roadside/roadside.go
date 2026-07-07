// Package roadside holds the hand-authored domain logic for the
// roadside-america CLI: parsing RoadsideAmerica.com HTML surfaces into
// structured attractions, local superlative/keyword classification, and
// keyless geocoding. Kept separate from package cli so it is unit-testable
// and survives regeneration as a whole hand-authored unit.
//
// All data produced here is scraped and community-sourced from
// RoadsideAmerica.com; every record carries a SourceURL back to its page.
package roadside

import (
	"fmt"
	"strings"
)

const (
	// BaseURL is the RoadsideAmerica.com origin. Kept here so URL builders
	// have a single source of truth independent of the runtime config.
	BaseURL = "https://www.roadsideamerica.com"

	// SourceLabel is attached to output envelopes so consumers and agents
	// always see the provenance of the data.
	SourceLabel = "community-sourced from RoadsideAmerica.com (scraped; not affiliated)"

	// ResourceType is the store resource_type key for cached attractions.
	ResourceType = "attraction"

	// GeocodeResourceType is the store resource_type key for cached geocodes.
	GeocodeResourceType = "geocode"
)

// Attraction is one offbeat stop. Distance fields are populated only for
// nearby queries; Categories is populated by local classification.
type Attraction struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Street     string   `json:"street,omitempty"`
	City       string   `json:"city,omitempty"`
	State      string   `json:"state,omitempty"`
	Distance   string   `json:"distance,omitempty"`    // raw label, e.g. "<1 mi. away"
	DistanceMi float64  `json:"distance_mi,omitempty"` // parsed miles; 0 when unknown
	DetailPath string   `json:"detail_path,omitempty"` // /tip/<id> or /story/<id>
	SourceURL  string   `json:"source_url"`
	Categories []string `json:"categories,omitempty"`
	CachedAt   string   `json:"cached_at,omitempty"` // RFC3339; when this row was fetched
}

// Detail is a full attraction writeup from a /tip/<id> or /story/<id> page.
type Detail struct {
	Attraction
	Summary    string `json:"summary,omitempty"`    // og:description
	Writeup    string `json:"writeup,omitempty"`    // editorial paragraph
	Directions string `json:"directions,omitempty"` // human directions blurb
	ImageURL   string `json:"image_url,omitempty"`
}

// AttractionURL returns the canonical source URL for a detail path or, when
// the path is empty, a /tip/<id> URL derived from the id.
func AttractionURL(detailPath, id string) string {
	p := strings.TrimSpace(detailPath)
	if p == "" && id != "" {
		p = "/tip/" + id
	}
	if p == "" {
		return BaseURL
	}
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return p
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return BaseURL + p
}

// NormalizeState lowercases and trims a US state / Canadian province code for
// use in attractionsByState.php?state=. It does not validate membership;
// validation lives in the command layer so the error message can list codes.
func NormalizeState(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// StateName maps a 2-letter code to a display name when known, else returns
// the upper-cased code. Used only for human-friendly output.
func StateName(code string) string {
	if n, ok := stateNames[strings.ToUpper(strings.TrimSpace(code))]; ok {
		return n
	}
	return strings.ToUpper(strings.TrimSpace(code))
}

// ValidState reports whether code is a recognized US state/DC or Canadian
// province abbreviation.
func ValidState(code string) bool {
	_, ok := stateNames[strings.ToUpper(strings.TrimSpace(code))]
	return ok
}

// StateCodes returns the sorted list of accepted codes for error messages.
func StateCodes() string {
	return strings.Join(orderedStateCodes, " ")
}

var orderedStateCodes = []string{
	"AL", "AK", "AZ", "AR", "CA", "CO", "CT", "DE", "DC", "FL", "GA", "HI",
	"ID", "IL", "IN", "IA", "KS", "KY", "LA", "ME", "MD", "MA", "MI", "MN",
	"MS", "MO", "MT", "NE", "NV", "NH", "NJ", "NM", "NY", "NC", "ND", "OH",
	"OK", "OR", "PA", "RI", "SC", "SD", "TN", "TX", "UT", "VT", "VA", "WA",
	"WV", "WI", "WY",
	"AB", "BC", "MB", "NB", "NF", "NT", "NS", "ON", "PE", "QC", "SK",
}

var stateNames = map[string]string{
	"AL": "Alabama", "AK": "Alaska", "AZ": "Arizona", "AR": "Arkansas",
	"CA": "California", "CO": "Colorado", "CT": "Connecticut", "DE": "Delaware",
	"DC": "District of Columbia", "FL": "Florida", "GA": "Georgia", "HI": "Hawaii",
	"ID": "Idaho", "IL": "Illinois", "IN": "Indiana", "IA": "Iowa",
	"KS": "Kansas", "KY": "Kentucky", "LA": "Louisiana", "ME": "Maine",
	"MD": "Maryland", "MA": "Massachusetts", "MI": "Michigan", "MN": "Minnesota",
	"MS": "Mississippi", "MO": "Missouri", "MT": "Montana", "NE": "Nebraska",
	"NV": "Nevada", "NH": "New Hampshire", "NJ": "New Jersey", "NM": "New Mexico",
	"NY": "New York", "NC": "North Carolina", "ND": "North Dakota", "OH": "Ohio",
	"OK": "Oklahoma", "OR": "Oregon", "PA": "Pennsylvania", "RI": "Rhode Island",
	"SC": "South Carolina", "SD": "South Dakota", "TN": "Tennessee", "TX": "Texas",
	"UT": "Utah", "VT": "Vermont", "VA": "Virginia", "WA": "Washington",
	"WV": "West Virginia", "WI": "Wisconsin", "WY": "Wyoming",
	"AB": "Alberta", "BC": "British Columbia", "MB": "Manitoba",
	"NB": "New Brunswick", "NF": "Newfoundland", "NT": "Northwest Territories",
	"NS": "Nova Scotia", "ON": "Ontario", "PE": "Prince Edward Island",
	"QC": "Quebec", "SK": "Saskatchewan",
}

// MilesToDelta converts a search radius in miles to a bounding half-size in
// degrees for nearbyAttractions.php?delta=. 1 degree of latitude is ~69 mi;
// the 1.4 fudge widens the bounding box so longitude compression at higher
// latitudes does not drop edge results before client-side distance filtering.
func MilesToDelta(miles float64) float64 {
	if miles <= 0 {
		miles = 25
	}
	d := (miles / 69.0) * 1.4
	if d < 0.05 {
		d = 0.05
	}
	if d > 5 {
		d = 5
	}
	return d
}

// FormatFloat renders a float without a trailing ".0" for clean query strings.
func FormatFloat(f float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.5f", f), "0"), ".")
}
