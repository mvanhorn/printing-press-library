// Package gflights is flight-goat's Google Flights backend. It has two
// implementations wired in priority order:
//
//  1. Native: krisukox/google-flights-api, a pure-Go reverse-engineered client
//     that talks directly to Google's internal protobuf endpoints. No Python,
//     no subprocess, no API key.
//  2. Fallback: the fli Python CLI (pipx install flights). Used when the
//     native backend fails (e.g. Google's abuse-detection demands the
//     GOOGLE_ABUSE_EXEMPTION cookie that only a browser can solve).
//
// The two backends return the same normalized flight-goat types so callers
// don't care which one served the request. Each response carries a Source
// string so users can see which backend answered.
package gflights

import (
	"context"
	"fmt"
	"strings"
	"time"

	krisukox "github.com/krisukox/google-flights-api/flights"
	"golang.org/x/text/currency"
	"golang.org/x/text/language"
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
}

// SearchResult is the normalized envelope returned by Search.
type SearchResult struct {
	Success    bool     `json:"success"`
	Source     string   `json:"source"` // "native-go" — krisukox/google-flights-api
	DataSource string   `json:"data_source"`
	SearchType string   `json:"search_type"`
	TripType   string   `json:"trip_type"`
	Query      SearchQuery `json:"query"`
	Count      int      `json:"count"`
	Flights    []Flight `json:"flights"`
}

// SearchQuery echoes the user's query back in the response envelope.
type SearchQuery struct {
	Origin        string `json:"origin"`
	Destination   string `json:"destination"`
	DepartureDate string `json:"departure_date"`
	ReturnDate    string `json:"return_date,omitempty"`
	CabinClass    string `json:"cabin_class"`
	MaxStops      string `json:"max_stops"`
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
}

// SearchOptions are the knobs users can pass to a flight search.
type SearchOptions struct {
	Origin        string
	Destination   string
	DepartureDate string
	ReturnDate    string
	TimeWindow    string
	Airlines      []string
	CabinClass    string
	MaxStops      string
	SortBy        string
	Passengers    int
	ExcludeBasic  bool
}

// Search runs a flight search via the native Go backend (krisukox).
// Previously fell back to a fli Python subprocess on retryable errors;
// that fallback has been removed to keep the binary self-contained for
// MCPB packaging. If the native path fails on Google's abuse detection,
// users can rerun later or use the AeroAPI commands instead.
func Search(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	return searchNative(ctx, opts)
}

// searchNative uses krisukox/google-flights-api for a pure-Go call to
// Google Flights' internal protobuf endpoint.
func searchNative(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	session, err := krisukox.New()
	if err != nil {
		return nil, fmt.Errorf("native session: %w", err)
	}
	depDate, err := time.Parse("2006-01-02", opts.DepartureDate)
	if err != nil {
		return nil, fmt.Errorf("invalid date %q: want YYYY-MM-DD", opts.DepartureDate)
	}
	// krisukox insists on a return date even for OneWay; synthesize one if
	// the user didn't provide it.
	retDate := depDate.AddDate(0, 0, 7)
	if opts.ReturnDate != "" {
		rd, err := time.Parse("2006-01-02", opts.ReturnDate)
		if err != nil {
			return nil, fmt.Errorf("invalid return date %q: want YYYY-MM-DD", opts.ReturnDate)
		}
		retDate = rd
	}

	stops := krisukox.AnyStops
	switch strings.ToLower(strings.ReplaceAll(opts.MaxStops, "-", "_")) {
	case "0", "nonstop", "non_stop":
		stops = krisukox.Nonstop
	case "1", "one_stop":
		stops = krisukox.Stop1
	case "2", "two_plus_stops":
		stops = krisukox.Stop2
	}
	cabin := krisukox.Economy
	switch strings.ToLower(opts.CabinClass) {
	case "premium_economy", "premium-economy", "premium":
		cabin = krisukox.PremiumEconomy
	case "business":
		cabin = krisukox.Business
	case "first":
		cabin = krisukox.First
	}
	tripType := krisukox.OneWay
	if opts.ReturnDate != "" {
		tripType = krisukox.RoundTrip
	}

	passengers := 1
	if opts.Passengers > 0 {
		passengers = opts.Passengers
	}

	offers, _, err := session.GetOffers(ctx, krisukox.Args{
		Date:        depDate,
		ReturnDate:  retDate,
		SrcAirports: []string{opts.Origin},
		DstAirports: []string{opts.Destination},
		Options: krisukox.Options{
			Travelers: krisukox.Travelers{Adults: passengers},
			Currency:  currency.USD,
			Stops:     stops,
			Class:     cabin,
			TripType:  tripType,
			Lang:      language.English,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("native GetOffers: %w", err)
	}

	flights := make([]Flight, 0, len(offers))
	for _, o := range offers {
		f := Flight{
			DurationMinutes: int(o.FlightDuration.Minutes()),
			Stops:           len(o.Flight) - 1,
			Price:           o.Price,
			Currency:        "USD",
		}
		for _, leg := range o.Flight {
			f.Legs = append(f.Legs, Leg{
				DepartureAirport: Airport{Code: leg.DepAirportCode, Name: leg.DepAirportName},
				ArrivalAirport:   Airport{Code: leg.ArrAirportCode, Name: leg.ArrAirportName},
				DepartureTime:    leg.DepTime.Format(time.RFC3339),
				ArrivalTime:      leg.ArrTime.Format(time.RFC3339),
				DurationMinutes:  int(leg.Duration.Minutes()),
				Airline:          Airline{Name: leg.AirlineName, Code: leg.FlightNumber[:2]},
				FlightNumber:     leg.FlightNumber,
			})
		}
		flights = append(flights, f)
	}

	return &SearchResult{
		Success:    true,
		Source:     "native-go",
		DataSource: "google_flights",
		SearchType: "flights",
		TripType:   strings.ToUpper(strings.TrimSpace(string(tripTypeName(tripType)))),
		Query: SearchQuery{
			Origin:        opts.Origin,
			Destination:   opts.Destination,
			DepartureDate: opts.DepartureDate,
			ReturnDate:    opts.ReturnDate,
			CabinClass:    strings.ToUpper(opts.CabinClass),
			MaxStops:      strings.ToUpper(opts.MaxStops),
		},
		Count:   len(flights),
		Flights: flights,
	}, nil
}

func tripTypeName(t krisukox.TripType) string {
	if t == krisukox.RoundTrip {
		return "round_trip"
	}
	return "one_way"
}

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
	return datesNative(ctx, opts)
}

