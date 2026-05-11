// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// Native Go implementation of Google Flights' GetShoppingResults endpoint —
// the per-day flight-search call (origin/destination/date + filters).
//
// PATCH(upstream cli-printing-press): replaces github.com/krisukox/google-
// flights-api, which only exposed stops / class / trip-type and silently
// ignored airlines, bags, emissions, layover, carry-on, exclude-basic-
// economy, show-all-results, and the expanded sort options. This native
// backend matches what fli (the Python library) sends and exposes every
// filter Google Flights' internal API understands.
//
// Field map ported from fli/models/google_flights/flights.py format().
// Uses the same utls-fingerprinted HTTP client from utls_client.go that
// dates_native.go already used for the calendar endpoint.

package gflights

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const offersEndpoint = "https://www.google.com/_/FlightsFrontendUi/data/travel.frontend.flights.FlightsFrontendService/GetShoppingResults"

// Sort-by enum values mirror fli's SortBy in fli/models/google_flights/base.py.
const (
	sortByTopFlights    = 0
	sortByBest          = 1
	sortByCheapest      = 2
	sortByDepartureTime = 3
	sortByArrivalTime   = 4
	sortByDuration      = 5
	sortByEmissions     = 6
)

// emissionsLess is the only non-default emissions value Google Flights honors.
const emissionsLess = 1

// BagsFilter is the user-facing knob for including bag fees in returned prices.
// checked_bags clamps 0..2; carry_on is boolean. fli mirrors this struct.
type BagsFilter struct {
	CheckedBags int
	CarryOn     bool
}

// LayoverRestrictions narrows search to connections via specific airports.
type LayoverRestrictions struct {
	Airports    []string // IATA codes
	MaxDuration int      // minutes; 0 = no constraint
}

// searchNativeDirect is the post-krisukox native backend.
func searchNativeDirect(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	_, currencyCode, err := normalizeCurrency(opts.Currency)
	if err != nil {
		return nil, err
	}

	depDate, err := time.Parse("2006-01-02", opts.DepartureDate)
	if err != nil {
		return nil, fmt.Errorf("invalid date %q: want YYYY-MM-DD", opts.DepartureDate)
	}
	var retDate time.Time
	tripType := tripTypeOneWay
	if opts.ReturnDate != "" {
		rd, err := time.Parse("2006-01-02", opts.ReturnDate)
		if err != nil {
			return nil, fmt.Errorf("invalid return date %q: want YYYY-MM-DD", opts.ReturnDate)
		}
		retDate = rd
		tripType = tripTypeRoundTrip
	}

	payload, err := buildOffersPayload(opts, depDate, retDate, tripType)
	if err != nil {
		return nil, fmt.Errorf("building payload: %w", err)
	}
	body := "f.req=" + payload

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, offersEndpoint, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Set("User-Agent", chromeUserAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("x-goog-ext-259736195-jspb", googleFlightsCurrencyHeader(currencyCode))

	resp, err := utlsClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling shopping endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		snippet := string(respBody)
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		return nil, fmt.Errorf("shopping endpoint returned HTTP %d: %s", resp.StatusCode, snippet)
	}

	flights, err := parseOffersResponse(respBody, currencyCode)
	if err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	tripTypeName := "ONE_WAY"
	if tripType == tripTypeRoundTrip {
		tripTypeName = "ROUND_TRIP"
	}

	return &SearchResult{
		Success:    true,
		Source:     "native-go",
		DataSource: "google_flights",
		SearchType: "flights",
		TripType:   tripTypeName,
		Query: SearchQuery{
			Origin:        opts.Origin,
			Destination:   opts.Destination,
			DepartureDate: opts.DepartureDate,
			ReturnDate:    opts.ReturnDate,
			MaxStops:      strings.ToUpper(opts.MaxStops),
			CabinClass:    strings.ToUpper(opts.CabinClass),
			Currency:      currencyCode,
		},
		Count:   len(flights),
		Flights: flights,
	}, nil
}

// buildOffersPayload constructs the URL-encoded `f.req` value mirroring
// fli's FlightSearchFilters.format(). Field positions documented inline.
func buildOffersPayload(opts SearchOptions, depDate, retDate time.Time, tripType int) (string, error) {
	seat, err := mapSeatType(opts.CabinClass)
	if err != nil {
		return "", err
	}
	stops, err := mapMaxStops(opts.MaxStops)
	if err != nil {
		return "", err
	}
	sortBy, err := mapSortBy(opts.SortBy)
	if err != nil {
		return "", err
	}

	segments, err := buildOfferSegments(opts, depDate, retDate, tripType, stops)
	if err != nil {
		return "", err
	}

	passengers := opts.Passengers
	if passengers < 1 {
		passengers = 1
	}

	var bagsField any
	if opts.Bags != nil {
		// PATCH(greptile P2): clamp checked-bags to the documented 0..2 range.
		// Google's response to out-of-range values is undefined; passing a
		// negative or >2 integer silently built a malformed payload before.
		checked := opts.Bags.CheckedBags
		if checked < 0 {
			checked = 0
		} else if checked > 2 {
			checked = 2
		}
		carryOnInt := 0
		if opts.Bags.CarryOn {
			carryOnInt = 1
		}
		bagsField = []any{checked, carryOnInt}
	}

	excludeBasic := 0
	if opts.ExcludeBasic {
		excludeBasic = 1
	}

	main := []any{
		nil, nil,                   // [0..1]
		tripType,                   // [2]
		nil,                        // [3]
		[]any{},                    // [4]
		seat,                       // [5]
		[]any{passengers, 0, 0, 0}, // [6] [adults, children, infants_lap, infants_seat]
		nil, nil, nil,              // [7..9]
		bagsField,                  // [10] [checked_bags, carry_on]
		nil, nil,                   // [11..12]
		segments,                   // [13]
		nil, nil, nil,              // [14..16]
		1,                          // [17] hardcoded
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, // [18..27]
		excludeBasic,               // [28]
	}

	showAll := 1
	if opts.LimitedResults {
		showAll = 0
	}

	outer := []any{[]any{}, main, sortBy, showAll, 0, 1}

	innerJSON, err := json.Marshal(outer)
	if err != nil {
		return "", err
	}
	wrapped := []any{nil, string(innerJSON)}
	wrappedJSON, err := json.Marshal(wrapped)
	if err != nil {
		return "", err
	}
	return url.QueryEscape(string(wrappedJSON)), nil
}

func buildOfferSegments(opts SearchOptions, depDate, retDate time.Time, tripType int, stops int) ([]any, error) {
	var segments []any
	outbound, err := buildOneSegment(opts, depDate, opts.Origin, opts.Destination, stops)
	if err != nil {
		return nil, err
	}
	segments = append(segments, outbound)
	if tripType == tripTypeRoundTrip {
		inbound, err := buildOneSegment(opts, retDate, opts.Destination, opts.Origin, stops)
		if err != nil {
			return nil, err
		}
		segments = append(segments, inbound)
	}
	return segments, nil
}

func buildOneSegment(opts SearchOptions, date time.Time, origin, dest string, stops int) ([]any, error) {
	var timeField any
	if opts.TimeWindow != "" {
		earliest, latest, err := parseTimeWindow(opts.TimeWindow)
		if err != nil {
			return nil, err
		}
		timeField = []any{earliest, latest, nil, nil}
	}

	var airlinesField any
	if len(opts.Airlines) > 0 {
		airlines := make([]any, 0, len(opts.Airlines))
		for _, a := range opts.Airlines {
			airlines = append(airlines, strings.ToUpper(strings.TrimSpace(a)))
		}
		airlinesField = airlines
	}

	var layoverAirports any
	var layoverDuration any
	if opts.Layover != nil {
		if len(opts.Layover.Airports) > 0 {
			lo := make([]any, 0, len(opts.Layover.Airports))
			for _, a := range opts.Layover.Airports {
				lo = append(lo, strings.ToUpper(strings.TrimSpace(a)))
			}
			layoverAirports = lo
		}
		if opts.Layover.MaxDuration > 0 {
			layoverDuration = opts.Layover.MaxDuration
		}
	}

	// PATCH(greptile P2): validate emissions enum at build time so typos
	// fail loudly instead of silently degrading to "ALL".
	emissionsField, err := mapEmissions(opts.Emissions)
	if err != nil {
		return nil, err
	}

	return []any{
		[]any{[]any{[]any{strings.ToUpper(origin), 0}}}, // [0] departure airport
		[]any{[]any{[]any{strings.ToUpper(dest), 0}}},   // [1] arrival airport
		timeField,                                       // [2] time restrictions
		stops,                                           // [3] stops
		airlinesField,                                   // [4] airlines
		nil,                                             // [5]
		date.Format("2006-01-02"),                       // [6] travel date
		nil,                                             // [7] max duration
		nil,                                             // [8] selected_flight
		layoverAirports,                                 // [9] layover airports
		nil,                                             // [10]
		nil,                                             // [11]
		layoverDuration,                                 // [12]
		emissionsField,                                  // [13]
		3,                                               // [14] no observable effect
	}, nil
}

func mapEmissions(s string) (any, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "all":
		return nil, nil
	case "less":
		return []any{emissionsLess}, nil
	default:
		return nil, fmt.Errorf("unknown --emissions %q (valid: ALL, LESS)", s)
	}
}

func mapSortBy(s string) (int, error) {
	switch strings.ToLower(strings.ReplaceAll(s, "-", "_")) {
	case "", "cheapest":
		return sortByCheapest, nil
	case "top_flights", "top":
		return sortByTopFlights, nil
	case "best":
		return sortByBest, nil
	case "departure_time", "departure":
		return sortByDepartureTime, nil
	case "arrival_time", "arrival":
		return sortByArrivalTime, nil
	case "duration":
		return sortByDuration, nil
	case "emissions":
		return sortByEmissions, nil
	default:
		return 0, fmt.Errorf("unknown --sort %q (valid: cheapest, top_flights, best, departure_time, arrival_time, duration, emissions)", s)
	}
}

func parseTimeWindow(tw string) (earliest, latest int, err error) {
	parts := strings.SplitN(tw, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid --time %q: want 'H-H' like '6-20'", tw)
	}
	if _, err := fmt.Sscanf(parts[0], "%d", &earliest); err != nil {
		return 0, 0, fmt.Errorf("invalid --time start %q: %w", parts[0], err)
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &latest); err != nil {
		return 0, 0, fmt.Errorf("invalid --time end %q: %w", parts[1], err)
	}
	if earliest < 0 || earliest > 23 || latest < 0 || latest > 23 || latest <= earliest {
		return 0, 0, fmt.Errorf("invalid --time %q: hours must be 0-23 with start < end", tw)
	}
	return earliest, latest, nil
}

func parseOffersResponse(body []byte, currency string) ([]Flight, error) {
	text := strings.TrimSpace(string(body))
	text = strings.TrimPrefix(text, googleResponsePrefix)
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, errors.New("empty response body after prefix strip")
	}
	var outer [][]any
	if err := json.Unmarshal([]byte(text), &outer); err != nil {
		return nil, fmt.Errorf("decoding outer envelope: %w", err)
	}
	if len(outer) == 0 || len(outer[0]) < 3 {
		return nil, errors.New("envelope missing inner payload")
	}
	innerStr, ok := outer[0][2].(string)
	if !ok {
		return []Flight{}, nil
	}
	var inner []any
	if err := json.Unmarshal([]byte(innerStr), &inner); err != nil {
		return nil, fmt.Errorf("decoding inner payload: %w", err)
	}

	var flights []Flight
	for _, idx := range []int{2, 3} {
		if idx >= len(inner) {
			continue
		}
		bucket, ok := inner[idx].([]any)
		if !ok || len(bucket) == 0 {
			continue
		}
		rows, ok := bucket[0].([]any)
		if !ok {
			continue
		}
		for _, row := range rows {
			f, ok := parseOfferRow(row, currency)
			if !ok {
				continue
			}
			flights = append(flights, f)
		}
	}
	return flights, nil
}

func parseOfferRow(row any, currency string) (Flight, bool) {
	r, ok := row.([]any)
	if !ok || len(r) < 1 {
		return Flight{}, false
	}
	head, ok := r[0].([]any)
	if !ok {
		return Flight{}, false
	}
	duration := numericInt(head, 9)
	legsRaw, _ := indexSlice(head, 2)
	legs := make([]Leg, 0, len(legsRaw))
	for _, legRaw := range legsRaw {
		leg, ok := parseOfferLeg(legRaw)
		if ok {
			legs = append(legs, leg)
		}
	}
	price := parseOfferPrice(r)
	return Flight{
		DurationMinutes: duration,
		Stops:           max0(len(legs) - 1),
		Price:           price,
		Currency:        currency,
		Legs:            legs,
	}, true
}

func parseOfferLeg(legRaw any) (Leg, bool) {
	leg, ok := legRaw.([]any)
	if !ok || len(leg) < 23 {
		return Leg{}, false
	}
	// PATCH(greptile P1): the airline + airport NAME fields fli's parser
	// drops live at adjacent slots Google sends in the leg array. Probed
	// against a real GetShoppingResults response:
	//   leg[4]      = dep airport name ("Seattle-Tacoma International Airport")
	//   leg[5]      = arr airport name ("Newark Liberty International Airport")
	//   leg[22][3]  = airline name      ("United")
	// Restores parity with the prior krisukox output for Airline.Name and
	// Airport.Name; addresses the P1 dropped-name regression on PR #440.
	airlineCode := ""
	flightNumber := ""
	airlineName := ""
	if al, ok := leg[22].([]any); ok {
		if len(al) >= 1 {
			airlineCode, _ = al[0].(string)
		}
		if len(al) >= 2 {
			flightNumber, _ = al[1].(string)
		}
		if len(al) >= 4 {
			airlineName, _ = al[3].(string)
		}
	}
	depAirport, _ := leg[3].(string)
	arrAirport, _ := leg[6].(string)
	depAirportName, _ := leg[4].(string)
	arrAirportName, _ := leg[5].(string)
	depTime := formatLegDateTime(indexAny(leg, 20), indexAny(leg, 8))
	arrTime := formatLegDateTime(indexAny(leg, 21), indexAny(leg, 10))
	return Leg{
		DepartureAirport: Airport{Code: strings.ToUpper(depAirport), Name: depAirportName},
		ArrivalAirport:   Airport{Code: strings.ToUpper(arrAirport), Name: arrAirportName},
		DepartureTime:    depTime,
		ArrivalTime:      arrTime,
		DurationMinutes:  numericInt(leg, 11),
		Airline:          Airline{Code: airlineCode, Name: airlineName},
		FlightNumber:     flightNumber,
	}, true
}

// parseOfferPrice returns the numeric price from the flight row.
//
// PATCH(greptile P1): the row's priceBlock[1] is Google's opaque price token
// (e.g. "CJRIDJNH..."), not an ISO currency code. Earlier versions returned
// it as a currency string, which then overwrote the user-requested ISO code
// downstream. Now this returns only the price; callers preserve the ISO
// code resolved from `--currency` / normalizeCurrency.
func parseOfferPrice(row []any) float64 {
	if len(row) < 2 {
		return 0
	}
	priceBlock, ok := row[1].([]any)
	if !ok {
		return 0
	}
	if len(priceBlock) > 0 {
		if outer, ok := priceBlock[0].([]any); ok && len(outer) > 0 {
			return numericFloat(outer[len(outer)-1])
		}
	}
	return 0
}

// --- small helpers ---

func indexSlice(a []any, i int) ([]any, bool) {
	if i >= len(a) {
		return nil, false
	}
	x, ok := a[i].([]any)
	return x, ok
}

func indexAny(a []any, i int) any {
	if i >= len(a) {
		return nil
	}
	return a[i]
}

func numericInt(a []any, i int) int {
	if i >= len(a) {
		return 0
	}
	return int(numericFloat(a[i]))
}

func numericFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

func formatLegDateTime(dateAny, timeAny any) string {
	d, _ := dateAny.([]any)
	t, _ := timeAny.([]any)
	year, month, day := 0, 0, 0
	hour, min := 0, 0
	if len(d) >= 3 {
		year = int(numericFloat(d[0]))
		month = int(numericFloat(d[1]))
		day = int(numericFloat(d[2]))
	}
	if len(t) >= 2 {
		hour = int(numericFloat(t[0]))
		min = int(numericFloat(t[1]))
	}
	if year == 0 && month == 0 && day == 0 && hour == 0 && min == 0 {
		return ""
	}
	if month < 1 {
		month = 1
	}
	if day < 1 {
		day = 1
	}
	// PATCH(codex P2): Google's date/time arrays are local to the airport
	// (UTC offset varies per leg and isn't carried in the response). Use a
	// time.Location with zero offset so the RFC3339 string ends without 'Z'
	// — preventing downstream consumers from treating local times as UTC.
	loc := time.FixedZone("", 0)
	t0 := time.Date(year, time.Month(month), day, hour, min, 0, 0, loc)
	return t0.Format("2006-01-02T15:04:05")
}
