// Package gflights is flight-goat's Google Flights backend.
//
// PATCH(upstream cli-printing-press): replaced krisukox/google-flights-api
// (which only exposed Stops / Class / TripType and silently dropped airlines,
// bags, emissions, layover, carry-on, exclude-basic-economy, show-all, and
// the expanded sort options) with a native f.req protobuf-shaped client that
// matches fli's Python implementation. See flights_native.go.
//
// Both Search() and Dates() now POST directly to the FlightsFrontendService
// endpoints through the utls-fingerprinted HTTP client used by
// dates_native.go. No upstream Go dependency for Google Flights.
package gflights

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/text/currency"
)

// Airport matches the nested shape used across the normalized return types.
type Airport struct {
	Code string `json:"code"`
	Name string `json:"name,omitempty"`
}

// Airline matches the nested airline object.
type Airline struct {
	Code string `json:"code"`
	Name string `json:"name,omitempty"`
}

// Leg is one hop of a multi-stop itinerary.
type Leg struct {
	DepartureAirport Airport `json:"departure_airport"`
	ArrivalAirport   Airport `json:"arrival_airport"`
	DepartureTime    string  `json:"departure_time"`
	ArrivalTime      string  `json:"arrival_time"`
	DurationMinutes  int     `json:"duration"`
	Airline          Airline `json:"airline"`
	FlightNumber     string  `json:"flight_number"`
}

// Flight is one itinerary (possibly multi-leg).
type Flight struct {
	DurationMinutes int     `json:"duration"`
	Stops           int     `json:"stops"`
	Legs            []Leg   `json:"legs"`
	Price           float64 `json:"price"`
	Currency        string  `json:"currency"`
	// PATCH(library): one-click handoff to a real booking surface. Google is
	// always populated; Airline is populated only when all legs are operated
	// by a single carrier in the airlineTemplates table. See booking_urls.go.
	BookingURLs BookingURLs `json:"booking_urls"`
}

// SearchResult is the normalized envelope returned by Search.
type SearchResult struct {
	Success    bool        `json:"success"`
	Source     string      `json:"source"` // "native-go" — direct f.req client, see flights_native.go
	DataSource string      `json:"data_source"`
	SearchType string      `json:"search_type"`
	TripType   string      `json:"trip_type"`
	Query      SearchQuery `json:"query"`
	Count      int         `json:"count"`
	Flights    []Flight    `json:"flights"`
	// PATCH(library): populated when one or both airport codes were remapped
	// from a retired IATA code. The Query echo above keeps the user's
	// original input; AirportRemapped is the only signal of substitution.
	AirportRemapped *AirportRemapNote `json:"airport_remapped,omitempty"`
}

// SearchQuery echoes the user's query back in the response envelope.
type SearchQuery struct {
	Origin        string `json:"origin"`
	Destination   string `json:"destination"`
	DepartureDate string `json:"departure_date"`
	ReturnDate    string `json:"return_date,omitempty"`
	CabinClass    string `json:"cabin_class"`
	MaxStops      string `json:"max_stops"`
	Currency      string `json:"currency,omitempty"`
}

// DatePrice is one row in the cheapest-dates output.
type DatePrice struct {
	DepartureDate string  `json:"departure_date"`
	ReturnDate    string  `json:"return_date,omitempty"`
	Price         float64 `json:"price"`
	Currency      string  `json:"currency,omitempty"`
}

// DatesResult is the normalized envelope returned by Dates.
type DatesResult struct {
	Success    bool        `json:"success"`
	Source     string      `json:"source"`
	DataSource string      `json:"data_source"`
	SearchType string      `json:"search_type"`
	Query      SearchQuery `json:"query"`
	Count      int         `json:"count"`
	Dates      []DatePrice `json:"dates"`
	// PATCH(library): see SearchResult.AirportRemapped.
	AirportRemapped *AirportRemapNote `json:"airport_remapped,omitempty"`
}

// SearchOptions are the knobs users can pass to a flight search.
//
// PATCH(upstream cli-printing-press): added Bags, Emissions, Layover,
// LimitedResults — Google Flights' API supports all of these but krisukox
// did not expose them. fli matches this same surface.
type SearchOptions struct {
	Origin         string
	Destination    string
	DepartureDate  string
	ReturnDate     string
	TimeWindow     string
	Airlines       []string
	CabinClass     string
	MaxStops       string
	SortBy         string
	Passengers     int
	ExcludeBasic   bool
	Currency       string
	Bags           *BagsFilter          // PATCH: include checked-bag + carry-on fees in returned prices
	Emissions      string               // PATCH: "ALL" (default) or "LESS" to filter low-emission itineraries
	Layover        *LayoverRestrictions // PATCH: restrict connections to specific airports
	LimitedResults bool                 // PATCH: when true, request the ~30 Google-curated set
}

// Search runs a flight search against Google Flights' GetShoppingResults.
//
// PATCH(upstream cli-printing-press): now calls the native f.req client in
// flights_native.go instead of the deprecated krisukox wrapper. The Python
// fli subprocess fallback was already removed (MCPB packaging requirement);
// no fallback exists today.
func Search(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	// PATCH(greptile P1): mirror Dates()'s defensive 90s fallback timeout so
	// callers that supply a context without a deadline don't hang forever on
	// a stuck Google request. The utls client uses ctx for per-request
	// deadlines but won't impose its own.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
	}
	// PATCH(library): normalize retired IATA codes before talking to Google.
	// Google's GetShoppingResults silently returns empty for decommissioned
	// codes; remap to the current code and surface the substitution in the
	// result envelope so callers can see what happened. The user's original
	// input is preserved in the echoed SearchQuery.
	userOrigin, userDest := opts.Origin, opts.Destination
	o, d, note := remapAirportPair(opts.Origin, opts.Destination)
	opts.Origin, opts.Destination = o.To, d.To
	result, err := searchNativeDirect(ctx, opts)
	if err != nil {
		return result, err
	}
	if result != nil {
		result.Query.Origin = userOrigin
		result.Query.Destination = userDest
		result.AirportRemapped = note
	}
	return result, nil
}

// (The old krisukox-backed searchNative + tripTypeName helper were removed
// in the PATCH(upstream cli-printing-press) Google Flights direct port — see
// flights_native.go for searchNativeDirect, which now serves Search().)

// DatesOptions drives a cheapest-dates query.
type DatesOptions struct {
	Origin      string
	Destination string
	From        string
	To          string
	Duration    int
	Airlines    []string
	RoundTrip   bool
	MaxStops    string
	CabinClass  string
	Sort        bool
	Currency    string
}

// Dates runs a cheapest-dates query against Google Flights' GetCalendarGraph
// endpoint via the native Go backend (see dates_native.go). Previously this
// shelled out to the fli Python library; that dependency was dropped to
// keep the binary self-contained for MCPB packaging.
func Dates(ctx context.Context, opts DatesOptions) (*DatesResult, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
	}
	// PATCH(library): same remap-and-echo flow as Search(); see comment there.
	userOrigin, userDest := opts.Origin, opts.Destination
	o, d, note := remapAirportPair(opts.Origin, opts.Destination)
	opts.Origin, opts.Destination = o.To, d.To
	result, err := datesNative(ctx, opts)
	if err != nil {
		return result, err
	}
	if result != nil {
		result.Query.Origin = userOrigin
		result.Query.Destination = userDest
		result.AirportRemapped = note
	}
	return result, nil
}

func normalizeCurrency(code string) (currency.Unit, string, error) {
	normalized, err := NormalizeCurrencyCode(code)
	if err != nil {
		return currency.Unit{}, "", err
	}
	unit, err := currency.ParseISO(normalized)
	if err != nil {
		return currency.Unit{}, "", fmt.Errorf("invalid currency %q: must be an ISO 4217 code (e.g. USD, GBP, EUR)", code)
	}
	return unit, normalized, nil
}

func NormalizeCurrencyCode(code string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	if normalized == "" {
		return "USD", nil
	}
	if _, err := currency.ParseISO(normalized); err != nil {
		return "", fmt.Errorf("invalid currency %q: must be an ISO 4217 code (e.g. USD, GBP, EUR)", code)
	}
	return normalized, nil
}

func googleFlightsCurrencyHeader(code string) string {
	return fmt.Sprintf(`["en-US","US","%s",1,null,[-120],null,[[48764689,47907128,48676280,48710756,48627726,48480739,48593234,48707380]],1,[]]`, code)
}
