// Package booking — GraphQL transport for booking.com's internal /dml/graphql.
//
// The MapMarkersDesktop query body is captured from a live logged-in browser
// session (the operations/mapmarkers.json file embedded below). Booking.com
// does not publish this schema; the query string here is a frozen snapshot of
// the format the booking.com web app sent on 2026-05-19. When booking.com
// changes the schema, this file needs to be re-captured via the
// /printing-press browser-sniff flow and the embedded JSON regenerated.
package booking

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
)

//go:embed operations/mapmarkers.json
var mapMarkersOperationJSON []byte

// CSRFTokenRE matches `b_csrf_token: 'value'` (and the `"value"` variant) in
// the search-results page HTML. Booking.com renders this token as part of an
// inline JS bootstrap object on every signed-in or anonymous page.
var csrfTokenRE = regexp.MustCompile(`b_csrf_token['"]?\s*[:=]\s*['"]([a-zA-Z0-9_\-\.=+/]+)['"]`)

// ExtractCSRFToken finds b_csrf_token in raw HTML/JS source. Returns empty
// string when the token is not present (e.g., booking.com served an
// anti-bot/challenge page instead of real search results).
func ExtractCSRFToken(html []byte) string {
	m := csrfTokenRE.FindSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	return string(m[1])
}

// MapMarkersOptions are the user-supplied search params for a map-markers
// fetch. Everything else (markersInput, airportsInput, the include flags,
// metaContext, etc.) is taken from the captured operation defaults.
type MapMarkersOptions struct {
	DestID   int    // booking.com destination id (e.g., -1456928 for Paris). REQUIRED.
	DestType string // "city", "region", "district", "landmark", "airport". Default "city".
	Checkin  string // YYYY-MM-DD. REQUIRED.
	Checkout string // YYYY-MM-DD. REQUIRED.
	Adults   int    // default 2
	Rooms    int    // default 1
	Currency string // ISO. Default "USD".
}

// BuildMapMarkersRequest returns the JSON-encoded /dml/graphql POST body for
// the MapMarkersDesktop operation, patched with the caller's search params.
// The query string and the bulky default variable scaffolding come from the
// captured operation file.
func BuildMapMarkersRequest(opts MapMarkersOptions) ([]byte, error) {
	if opts.DestID == 0 {
		return nil, fmt.Errorf("booking: map markers requires DestID (use 'destinations search' to look up a city id)")
	}
	if opts.Checkin == "" || opts.Checkout == "" {
		return nil, fmt.Errorf("booking: map markers requires checkin and checkout dates")
	}
	if opts.DestType == "" {
		opts.DestType = "CITY"
	}
	if opts.Adults == 0 {
		opts.Adults = 2
	}
	if opts.Rooms == 0 {
		opts.Rooms = 1
	}
	if opts.Currency == "" {
		opts.Currency = "USD"
	}

	var canned struct {
		OperationName string                 `json:"operationName"`
		Query         string                 `json:"query"`
		Variables     map[string]interface{} `json:"variables"`
	}
	if err := json.Unmarshal(mapMarkersOperationJSON, &canned); err != nil {
		return nil, fmt.Errorf("booking: parse embedded map markers operation: %w", err)
	}

	// Patch the search input with the caller's params; keep the captured
	// markersInput / airportsInput / include flags as-is. The bounding box
	// from the captured request is Paris-tight; widen it so the city's
	// initialDestination drives marker selection regardless of city.
	input, _ := canned.Variables["input"].(map[string]interface{})
	if input == nil {
		return nil, fmt.Errorf("booking: embedded operation missing 'input'")
	}
	input["nbAdults"] = opts.Adults
	input["nbRooms"] = opts.Rooms
	if dates, ok := input["dates"].(map[string]interface{}); ok {
		dates["checkin"] = opts.Checkin
		dates["checkout"] = opts.Checkout
	}
	if flex, ok := input["flexibleDatesConfig"].(map[string]interface{}); ok {
		if dr, ok := flex["dateRangeCalendar"].(map[string]interface{}); ok {
			dr["checkin"] = []string{opts.Checkin}
			dr["checkout"] = []string{opts.Checkout}
		}
	}
	location, _ := input["location"].(map[string]interface{})
	if location == nil {
		return nil, fmt.Errorf("booking: embedded operation missing 'input.location'")
	}
	// Replace the captured BOUNDING_BOX location with a city-id lookup. The
	// CITY destType uses location.destId at the root (the captured request
	// had destId only inside initialDestination, but booking.com's resolver
	// reads it from the root). Verified live against -1456928 (Paris): the
	// initialDestination-only form returns destId: 0 with a guessed city.
	for k := range location {
		delete(location, k)
	}
	location["destType"] = "CITY"
	location["destId"] = opts.DestID
	location["initialDestination"] = map[string]interface{}{
		"destType": "CITY",
		"destId":   opts.DestID,
	}

	return json.Marshal(canned)
}

// GraphQLHeaders returns the static headers booking.com's web app sends on
// every /dml/graphql request. The caller adds the CSRF header and any
// auth-cookie state on top.
func GraphQLHeaders(csrfToken string) map[string]string {
	h := map[string]string{
		"content-type":                "application/json",
		"accept":                      "*/*",
		"apollographql-client-name":   "b-search-web-searchresults_rust",
		"apollographql-client-version": "1NLE7NhgZ4iqK7NN1iyo",
	}
	if csrfToken != "" {
		h["x-booking-csrf-token"] = csrfToken
	}
	return h
}
