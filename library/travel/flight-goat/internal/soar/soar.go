// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Package soar is flight-goat's FlySoar.ai backend. FlySoar (https://flysoar.ai)
// is a Duffel-backed flight metasearch that exposes an anonymous streaming
// search endpoint — no API key, no auth. It sits alongside the Google Flights
// (internal/gflights) and Kayak (internal/kayak) price sources as a third
// independent way to shop a route/date, and is the only source here whose
// prices come straight off Duffel's aggregated content (NDC + GDS).
//
// The endpoint (POST /api/search/stream) answers with a Server-Sent Events
// stream of Duffel offer objects rather than a single JSON body, so Search
// consumes the SSE stream to completion, normalizes each offer into the same
// leg/itinerary shape the other backends use, dedupes identical itineraries
// keeping the cheapest fare, and returns them cheapest-first.
package soar

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	searchURL = "https://flysoar.ai/api/search/stream"
	baseURL   = "https://flysoar.ai"
	// browserUA mirrors the desktop Chrome string the FlySoar web app sends.
	// FlySoar gates the endpoint on Origin/Referer/User-Agent looking like the
	// site's own browser traffic; a stdlib default UA gets a 403. This is a
	// moving target — a regen or a FlySoar frontend refresh may require
	// re-capturing the current string.
	browserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
		"AppleWebKit/537.36 (KHTML, like Gecko) " +
		"Chrome/124.0.0.0 Safari/537.36"
	// defaultTimeout bounds a full SSE stream. FlySoar streams typically run
	// a few seconds; the generous ceiling matches the reference tool's 90s.
	defaultTimeout = 90 * time.Second
	// imessageAgent is FlySoar's conversational booking agent, reachable over
	// iMessage/SMS. FlySoar's own site links to it as sms:+14156299322. The
	// stream endpoint only prices; the actual purchase (saved travelers, saved
	// cards, SMS identity verification) happens through this agent or the web
	// app. This is a moving target — a regen should re-capture the number from
	// flysoar.ai rather than trust this constant blindly.
	imessageAgent = "+14156299322"
)

// Airport is the nested airport shape shared across the normalized types.
type Airport struct {
	Code string `json:"code"`
	Name string `json:"name,omitempty"`
}

// Airline is the nested carrier shape.
type Airline struct {
	Code string `json:"code"`
	Name string `json:"name,omitempty"`
}

// Leg is one operated segment of an itinerary.
type Leg struct {
	DepartureAirport Airport `json:"departure_airport"`
	ArrivalAirport   Airport `json:"arrival_airport"`
	DepartureTime    string  `json:"departure_time"`
	ArrivalTime      string  `json:"arrival_time"`
	DurationMinutes  int     `json:"duration"`
	Airline          Airline `json:"airline"`
	FlightNumber     string  `json:"flight_number,omitempty"`
	CabinClass       string  `json:"cabin_class,omitempty"`
	AircraftType     string  `json:"aircraft_type,omitempty"`
}

// Flight is one shoppable itinerary. For round-trips, Legs concatenates the
// outbound then the return slice; SliceCount records the trip's slice count so
// callers can tell one-way (1) from round-trip (2) itineraries apart.
type Flight struct {
	OfferID         string  `json:"offer_id,omitempty"`
	Price           float64 `json:"price"`
	Currency        string  `json:"currency"`
	DurationMinutes int     `json:"duration"`
	// Stops is the total number of connections across all slices.
	Stops int `json:"stops"`
	// SliceStops is the connection count per slice (per direction), e.g. [0,1]
	// for a round-trip that is nonstop out and one-stop back. FlySoar's stops
	// filter applies per direction, so this drives the --stops filter.
	SliceStops []int  `json:"slice_stops,omitempty"`
	SliceCount int    `json:"slice_count"`
	Owner      string `json:"owner,omitempty"`
	Legs       []Leg  `json:"legs"`
}

// Slice is one flown leg of a multi-city trip: a route on a date. The cabin
// applies trip-wide, mirroring FlySoar's GUI, which composes multicity
// searches with a single cabin repeated into every slice.
type Slice struct {
	Origin        string `json:"origin"`
	Destination   string `json:"destination"`
	DepartureDate string `json:"departure_date"` // YYYY-MM-DD
}

// SearchQuery echoes the request back in the response envelope. Currency is
// always USD — FlySoar's anonymous endpoint ignores any requested currency and
// prices every offer in USD (verified live across US and intra-EU routes), so
// the CLI deliberately does not expose a currency knob.
type SearchQuery struct {
	// For multi-city searches Origin/Destination/DepartureDate carry the trip's
	// first origin, final destination, and first date (matching the endpoint's
	// own created-event echo); Slices carries the full leg list.
	Origin        string `json:"origin"`
	Destination   string `json:"destination"`
	DepartureDate string `json:"departure_date"`
	ReturnDate    string `json:"return_date,omitempty"`
	// Slices echoes the requested legs when the search was multi-city.
	Slices     []Slice `json:"slices,omitempty"`
	CabinClass string  `json:"cabin_class"`
	Passengers int     `json:"passengers"`
	Currency   string  `json:"currency"`
	// Stops echoes the allowed stop counts (0 = nonstop, 1, 2, …); empty = any.
	Stops []int `json:"stops,omitempty"`
	// Airlines echoes the airline whitelist (IATA codes), empty when unset.
	Airlines []string `json:"airlines,omitempty"`
}

// SearchResult is the normalized envelope returned by Search.
type SearchResult struct {
	Success    bool        `json:"success"`
	Source     string      `json:"source"` // "flysoar"
	SearchType string      `json:"search_type"`
	TripType   string      `json:"trip_type"`
	Query      SearchQuery `json:"query"`
	Count      int         `json:"count"`
	Flights    []Flight    `json:"flights"`
	// SearchURL is a deep link to the same search on flysoar.ai, so a user can
	// open, refine, and book in the browser (the stream endpoint prices but
	// does not book). Always populated.
	SearchURL string `json:"search_url"`
	// Booking is the handoff for completing a purchase. FlySoar has no
	// anonymous booking API — the actual buy runs through its conversational
	// agent (iMessage) or the authenticated web app. Always populated.
	Booking BookingHandoff `json:"booking"`
	Note    string         `json:"note,omitempty"`
}

// BookingHandoff carries everything needed to move from pricing to purchase.
// flight-goat does not place bookings itself (that needs a FlySoar account,
// saved payment, and SMS identity verification); it hands off to FlySoar's
// booking surfaces with the search pre-composed.
type BookingHandoff struct {
	// Method is always "handoff" — a reminder that no purchase happens here.
	Method string `json:"method"`
	// Request is a natural-language booking seed the iMessage agent understands,
	// e.g. "DEN -> SEA on 2026-09-28, first class". Refine it in the chat.
	Request string `json:"request"`
	// IMessageAgent is FlySoar's booking agent handle (a phone number reachable
	// over iMessage/SMS).
	IMessageAgent string `json:"imessage_agent"`
	// IMessageURL opens a new iMessage/SMS to the agent with Request pre-filled.
	IMessageURL string `json:"imessage_url"`
	// WebURL is the browser deep link (same as SearchResult.SearchURL) for
	// completing the booking in the FlySoar web app.
	WebURL string `json:"web_url"`
	Note   string `json:"note"`
}

// SearchOptions are the knobs a caller can pass to a FlySoar search. There is
// deliberately no currency option: FlySoar's anonymous endpoint always prices in
// USD and ignores any requested currency, so exposing one would be misleading.
type SearchOptions struct {
	Origin        string
	Destination   string
	DepartureDate string // YYYY-MM-DD
	ReturnDate    string // YYYY-MM-DD, empty for one-way
	CabinClass    string // economy | premium_economy | business | first
	Passengers    int    // defaults to 1 when <= 0
	// Stops filters results and the deep link to itineraries whose stop count
	// is in this set (0 = nonstop, 1, 2, …). Empty means no stops filter.
	// Matches FlySoar's ?stops=0,1 GUI param (a set of allowed counts).
	Stops []int
	// Airlines whitelists marketing carriers by IATA code (e.g. UA, DL).
	// Empty means no airline filter. Matches FlySoar's ?airlines=DL,UA param.
	Airlines []string
	// Slices, when set (>= 2 entries), switches the search to multi-city mode.
	// Origin/Destination/DepartureDate are ignored and ReturnDate must be empty
	// (a return leg is just another slice). CabinClass applies to every slice.
	Slices []Slice
}

// priceCurrency is the only currency FlySoar's anonymous endpoint returns.
const priceCurrency = "USD"

// searchPayload is the JSON body POSTed to the stream endpoint for one-way and
// round-trip searches.
type searchPayload struct {
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
	Date        string `json:"date"`
	ReturnDate  string `json:"return_date"`
	Cabin       string `json:"cabin"`
	Currency    string `json:"currency"`
	Passengers  int    `json:"passengers"`
}

// multiSearchPayload is the JSON body for a multi-city search. Verified live:
// the endpoint accepts a bare slices array with no top-level origin/destination
// (it derives the trip's endpoints itself) and echoes is_multi_city=true in the
// created event.
type multiSearchPayload struct {
	Slices     []payloadSlice `json:"slices"`
	Cabin      string         `json:"cabin"`
	Currency   string         `json:"currency"`
	Passengers int            `json:"passengers"`
}

// payloadSlice is one requested multi-city leg. The endpoint accepts "date"
// and echoes it back as departure_date.
type payloadSlice struct {
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
	Date        string `json:"date"`
}

// --- raw Duffel-shaped offer decoding ---------------------------------------

type rawOffer struct {
	ID          string     `json:"id"`
	Owner       rawOwner   `json:"owner"`
	TotalAmount string     `json:"total_amount"`
	Currency    string     `json:"total_currency"`
	Slices      []rawSlice `json:"slices"`
}

type rawOwner struct {
	IATACode string `json:"iata_code"`
	Name     string `json:"name"`
}

type rawSlice struct {
	Segments []rawSegment `json:"segments"`
}

type rawSegment struct {
	CarrierIATA  string   `json:"carrier_iata"`
	CarrierName  string   `json:"carrier_name"`
	FlightNumber string   `json:"flight_number"`
	Departure    string   `json:"departure"`
	Arrival      string   `json:"arrival"`
	CabinClass   string   `json:"cabin_class"`
	Duration     string   `json:"duration"` // ISO 8601 duration, e.g. "PT5H30M"
	Aircraft     string   `json:"aircraft"`
	Origin       rawPlace `json:"origin"`
	Destination  rawPlace `json:"destination"`
	// Some serializations nest carrier/aircraft as objects; captured below and
	// merged in normalizeOffer when the flat fields are empty.
	MarketingCarrier rawCarrier `json:"marketing_carrier"`
	OperatingCarrier rawCarrier `json:"operating_carrier"`
}

// rawPlace tolerates either a bare IATA string or a nested {iata_code,name}
// object, which is how Duffel serializes origin/destination.
type rawPlace struct {
	IATACode string
	Name     string
}

func (p *rawPlace) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		p.IATACode = s
		return nil
	}
	var obj struct {
		IATACode string `json:"iata_code"`
		Name     string `json:"name"`
	}
	if err := json.Unmarshal(b, &obj); err != nil {
		return err
	}
	p.IATACode = obj.IATACode
	p.Name = obj.Name
	return nil
}

type rawCarrier struct {
	IATACode string `json:"iata_code"`
	Name     string `json:"name"`
}

// Search runs a FlySoar search and returns normalized, cheapest-first offers.
func Search(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	multi := len(opts.Slices) > 0
	if multi {
		if len(opts.Slices) < 2 {
			return nil, fmt.Errorf("soar: multi-city needs at least 2 slices; use origin/destination for a one-way")
		}
		if strings.TrimSpace(opts.ReturnDate) != "" {
			return nil, fmt.Errorf("soar: return date cannot be combined with multi-city slices; add the return as its own slice")
		}
	} else {
		if strings.TrimSpace(opts.Origin) == "" || strings.TrimSpace(opts.Destination) == "" {
			return nil, fmt.Errorf("soar: origin and destination are required")
		}
		if strings.TrimSpace(opts.DepartureDate) == "" {
			return nil, fmt.Errorf("soar: departure date is required")
		}
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
	}

	// Airport codes are forwarded raw: FlySoar's Duffel backend resolves
	// IATA/renamed codes itself, so (unlike the gflights/kayak backends) soar
	// keeps no retired-code alias table and emits no airport_remapped note. See
	// the "Airport alias maintenance" section in AGENTS.md.
	origin := strings.ToUpper(strings.TrimSpace(opts.Origin))
	dest := strings.ToUpper(strings.TrimSpace(opts.Destination))
	depDate := strings.TrimSpace(opts.DepartureDate)
	var slices []Slice
	if multi {
		slices = make([]Slice, len(opts.Slices))
		for i, s := range opts.Slices {
			slices[i] = Slice{
				Origin:        strings.ToUpper(strings.TrimSpace(s.Origin)),
				Destination:   strings.ToUpper(strings.TrimSpace(s.Destination)),
				DepartureDate: strings.TrimSpace(s.DepartureDate),
			}
			if slices[i].Origin == "" || slices[i].Destination == "" || slices[i].DepartureDate == "" {
				return nil, fmt.Errorf("soar: slice %d needs origin, destination, and date", i+1)
			}
		}
		// The query echo mirrors the endpoint's created event: first origin,
		// final destination, first date.
		origin = slices[0].Origin
		dest = slices[len(slices)-1].Destination
		depDate = slices[0].DepartureDate
	}
	cabin := normalizeCabin(opts.CabinClass)
	passengers := opts.Passengers
	if passengers <= 0 {
		passengers = 1
	}
	stops := normalizeStops(opts.Stops)
	airlines := normalizeAirlines(opts.Airlines)

	var body []byte
	var err error
	if multi {
		ps := make([]payloadSlice, len(slices))
		for i, s := range slices {
			ps[i] = payloadSlice{Origin: s.Origin, Destination: s.Destination, Date: s.DepartureDate}
		}
		body, err = json.Marshal(multiSearchPayload{
			Slices:     ps,
			Cabin:      cabin,
			Currency:   priceCurrency,
			Passengers: passengers,
		})
	} else {
		body, err = json.Marshal(searchPayload{
			Origin:      origin,
			Destination: dest,
			Date:        opts.DepartureDate,
			ReturnDate:  opts.ReturnDate,
			Cabin:       cabin,
			Currency:    priceCurrency,
			Passengers:  passengers,
		})
	}
	if err != nil {
		return nil, fmt.Errorf("soar: encoding request: %w", err)
	}

	// The deep link doubles as the Referer: it reconstructs the site's own
	// search URL, which FlySoar checks as part of its anonymous-request gating
	// (both the path-only classic form and the multicity query form are
	// verified live to pass the gate).
	deepLink := SearchURL(origin, dest, depDate, opts.ReturnDate, cabin, stops, airlines)
	referer := refererURL(origin, dest, depDate, opts.ReturnDate)
	if multi {
		deepLink = MultiCitySearchURL(slices, cabin, stops, airlines)
		referer = deepLink
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, searchURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("soar: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Origin", baseURL)
	req.Header.Set("Referer", referer)
	req.Header.Set("User-Agent", browserUA)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("soar: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("soar: search endpoint returned HTTP %d", resp.StatusCode)
	}

	offers, err := parseSSEOffers(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("soar: reading stream: %w", err)
	}

	// Apply the same stop/airline filters the deep link encodes, so the CLI's
	// results match what opening search_url in the browser would show. FlySoar's
	// stream endpoint returns the unfiltered set; these are GUI-side filters.
	unfiltered := normalizeOffers(offers, priceCurrency, passengers)
	flights := filterFlights(unfiltered, stops, airlines)
	// Cheapest-first, matching the other price backends' default ordering.
	sort.SliceStable(flights, func(i, j int) bool {
		return flights[i].Price < flights[j].Price
	})

	tripType := "one_way"
	switch {
	case multi:
		tripType = "multi_city"
	case strings.TrimSpace(opts.ReturnDate) != "":
		tripType = "round_trip"
	}

	res := &SearchResult{
		Success:    true,
		Source:     "flysoar",
		SearchType: "flights",
		TripType:   tripType,
		Query: SearchQuery{
			Origin:        origin,
			Destination:   dest,
			DepartureDate: depDate,
			ReturnDate:    opts.ReturnDate,
			Slices:        slices,
			CabinClass:    cabin,
			Passengers:    passengers,
			Currency:      priceCurrency,
			Stops:         stops,
			Airlines:      airlines,
		},
		Count:     len(flights),
		Flights:   flights,
		SearchURL: deepLink,
	}
	res.Booking = buildBookingHandoff(res.Query, res.SearchURL)
	if len(flights) == 0 {
		if len(unfiltered) > 0 && (len(stops) > 0 || len(airlines) > 0) {
			// Offers existed but the stops/airlines filters removed them all —
			// don't mislead the caller into thinking the route has no service.
			res.Note = fmt.Sprintf("FlySoar returned %d offer(s), but none matched the selected filters (stops/airlines); widen or drop --stops/--airlines", len(unfiltered))
		} else {
			res.Note = "FlySoar returned no offers for this search"
		}
	}
	return res, nil
}

// flightPath builds the site's path segment for a route/date:
// /flights/<orig>/<dest>/<YYMMDD>[/<returnYYMMDD>]/ — shared by the Referer
// header and the user-facing deep link.
func flightPath(origin, dest, depDate, retDate string) string {
	parts := []string{strings.ToLower(origin), strings.ToLower(dest), yymmdd(depDate)}
	if strings.TrimSpace(retDate) != "" {
		parts = append(parts, yymmdd(retDate))
	}
	return fmt.Sprintf("/flights/%s/", strings.Join(parts, "/"))
}

// refererURL reconstructs the site's own flight-search URL, which FlySoar
// checks as part of its anonymous-request gating.
func refererURL(origin, dest, depDate, retDate string) string {
	return baseURL + flightPath(origin, dest, depDate, retDate)
}

// buildBookingHandoff composes the booking handoff for a search. FlySoar has no
// anonymous booking API, so this points at the two surfaces that can actually
// complete a purchase — the iMessage agent and the web app — with the request
// pre-composed.
func buildBookingHandoff(q SearchQuery, webURL string) BookingHandoff {
	request := BookingRequest(q)
	return BookingHandoff{
		Method:        "handoff",
		Request:       request,
		IMessageAgent: imessageAgent,
		IMessageURL:   IMessageURL(imessageAgent, request),
		WebURL:        webURL,
		Note: "flight-goat prices via FlySoar but does not book. Send the request " +
			"to FlySoar's iMessage agent, or open the web link, to complete the " +
			"purchase (both require a FlySoar account with saved traveler + payment).",
	}
}

// BookingRequest builds a natural-language booking seed the FlySoar agent
// understands, mirroring how a user phrases it in chat, e.g.
// "DEN -> SEA on 2026-09-28, first class, 2 passengers". Multi-city queries
// render every leg: "multi-city: IAH -> FCO on 2026-09-27, then CAI -> SEA on
// 2026-10-04, first class".
func BookingRequest(q SearchQuery) string {
	var b string
	if len(q.Slices) > 0 {
		legs := make([]string, len(q.Slices))
		for i, s := range q.Slices {
			legs[i] = fmt.Sprintf("%s -> %s on %s", s.Origin, s.Destination, s.DepartureDate)
		}
		b = "multi-city: " + strings.Join(legs, ", then ")
	} else {
		b = fmt.Sprintf("%s -> %s on %s", q.Origin, q.Destination, q.DepartureDate)
		if strings.TrimSpace(q.ReturnDate) != "" {
			b += ", returning " + q.ReturnDate
		}
	}
	if cabin := prettyCabin(q.CabinClass); cabin != "" {
		b += ", " + cabin
	}
	if s := stopsPhrase(q.Stops); s != "" {
		b += ", " + s
	}
	if len(q.Airlines) > 0 {
		b += ", on " + strings.Join(q.Airlines, "/")
	}
	if q.Passengers > 1 {
		b += fmt.Sprintf(", %d passengers", q.Passengers)
	}
	return b
}

// stopsPhrase renders the allowed stop counts in natural language for the
// booking request, e.g. [0] -> "nonstop", [0,1] -> "nonstop or 1 stop".
func stopsPhrase(stops []int) string {
	if len(stops) == 0 {
		return ""
	}
	parts := make([]string, len(stops))
	for i, s := range stops {
		switch s {
		case 0:
			parts[i] = "nonstop"
		case 1:
			parts[i] = "1 stop"
		default:
			parts[i] = fmt.Sprintf("%d stops", s)
		}
	}
	return strings.Join(parts, " or ")
}

// prettyCabin turns a normalized cabin token into agent-friendly prose, e.g.
// "premium_economy" -> "premium economy class". Empty/economy returns "".
func prettyCabin(cabin string) string {
	switch normalizeCabin(cabin) {
	case "first":
		return "first class"
	case "business":
		return "business class"
	case "premium_economy":
		return "premium economy class"
	default:
		return ""
	}
}

// IMessageURL builds an sms: URL that opens a new iMessage/SMS to number with
// body pre-filled. Per RFC 5724 the query string starts with '?' (a leading
// '&' would fold body into the recipient path and defeat the pre-fill).
func IMessageURL(number, body string) string {
	return fmt.Sprintf("sms:%s?body=%s", number, url.QueryEscape(body))
}

// SearchURL builds a browser deep link to a FlySoar search, e.g.
// https://flysoar.ai/flights/sea/den/260921/?cabin=first&trip=oneway .
// It mirrors FlySoar's GUI filter params: cabin, trip, stops (a set of allowed
// stop counts, e.g. stops=0,1), and airlines (an IATA whitelist, e.g.
// airlines=DL,UA). cabin/stops/airlines should already be normalized; empty
// values omit their param. Commas are URL-encoded as %2C by url.Values.
func SearchURL(origin, dest, depDate, retDate, cabin string, stops []int, airlines []string) string {
	trip := "oneway"
	if strings.TrimSpace(retDate) != "" {
		trip = "roundtrip"
	}
	q := url.Values{}
	if c := strings.ToLower(strings.TrimSpace(cabin)); c != "" {
		q.Set("cabin", c)
	}
	q.Set("trip", trip)
	setFilterParams(q, stops, airlines)
	return fmt.Sprintf("%s%s?%s", baseURL, flightPath(origin, dest, depDate, retDate), q.Encode())
}

// MultiCitySearchURL builds the browser deep link for a multi-city search,
// mirroring FlySoar's GUI URL shape, e.g.
// https://flysoar.ai/flights/iah/sea/260927/?cabin=first&slices=IAH-FCO-260927-first%3BCAI-SEA-261004-first&trip=multicity .
// The path carries first origin / final destination / first date; the slices
// param carries every leg as ORIG-DEST-YYMMDD-<cabin>, ';'-joined (the cabin
// token repeats because the GUI applies one cabin trip-wide). stops/airlines
// mirror SearchURL's GUI filter params.
func MultiCitySearchURL(slices []Slice, cabin string, stops []int, airlines []string) string {
	if len(slices) == 0 {
		return baseURL
	}
	c := normalizeCabin(cabin)
	parts := make([]string, len(slices))
	for i, s := range slices {
		parts[i] = fmt.Sprintf("%s-%s-%s-%s",
			strings.ToUpper(strings.TrimSpace(s.Origin)),
			strings.ToUpper(strings.TrimSpace(s.Destination)),
			yymmdd(s.DepartureDate), c)
	}
	q := url.Values{}
	q.Set("cabin", c)
	q.Set("trip", "multicity")
	q.Set("slices", strings.Join(parts, ";"))
	setFilterParams(q, stops, airlines)
	path := flightPath(slices[0].Origin, slices[len(slices)-1].Destination, slices[0].DepartureDate, "")
	return fmt.Sprintf("%s%s?%s", baseURL, path, q.Encode())
}

// setFilterParams adds the stops/airlines GUI filter params shared by the
// one-way/round-trip and multi-city deep links.
func setFilterParams(q url.Values, stops []int, airlines []string) {
	if len(stops) > 0 {
		parts := make([]string, len(stops))
		for i, s := range stops {
			parts[i] = strconv.Itoa(s)
		}
		q.Set("stops", strings.Join(parts, ","))
	}
	if len(airlines) > 0 {
		q.Set("airlines", strings.Join(airlines, ","))
	}
}

// normalizeStops sorts and de-dupes the allowed stop counts, dropping negatives.
// Returns nil for empty/all-invalid input (no stops filter).
func normalizeStops(stops []int) []int {
	if len(stops) == 0 {
		return nil
	}
	seen := make(map[int]bool, len(stops))
	out := make([]int, 0, len(stops))
	for _, s := range stops {
		if s < 0 || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Ints(out)
	return out
}

// normalizeAirlines upper-cases, trims, and de-dupes IATA codes, preserving
// input order. Returns nil for empty input (no airline filter).
func normalizeAirlines(airlines []string) []string {
	if len(airlines) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(airlines))
	out := make([]string, 0, len(airlines))
	for _, a := range airlines {
		code := strings.ToUpper(strings.TrimSpace(a))
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		out = append(out, code)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// filterFlights keeps itineraries matching the stop-count set (if any) and the
// airline whitelist (if any). The stops filter is per direction — FlySoar's
// stops=N applies to each slice — so an itinerary passes only when every slice's
// connection count is in the set. The airline filter passes when every leg's
// marketing carrier is in the whitelist.
func filterFlights(flights []Flight, stops []int, airlines []string) []Flight {
	if len(stops) == 0 && len(airlines) == 0 {
		return flights
	}
	stopSet := make(map[int]bool, len(stops))
	for _, s := range stops {
		stopSet[s] = true
	}
	airlineSet := make(map[string]bool, len(airlines))
	for _, a := range airlines {
		airlineSet[a] = true
	}
	out := make([]Flight, 0, len(flights))
	for _, f := range flights {
		if len(stopSet) > 0 && !sliceStopsAllowed(f, stopSet) {
			continue
		}
		if len(airlineSet) > 0 && !allLegsInAirlines(f, airlineSet) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// sliceStopsAllowed reports whether every slice's connection count is in the
// allowed set (FlySoar's stops filter is per direction). Falls back to the total
// stop count when per-slice data is unavailable.
func sliceStopsAllowed(f Flight, set map[int]bool) bool {
	if len(f.SliceStops) == 0 {
		return set[f.Stops]
	}
	for _, c := range f.SliceStops {
		if !set[c] {
			return false
		}
	}
	return true
}

// allLegsInAirlines reports whether every leg's airline code is in the set.
func allLegsInAirlines(f Flight, set map[string]bool) bool {
	if len(f.Legs) == 0 {
		return false
	}
	for _, l := range f.Legs {
		if !set[strings.ToUpper(l.Airline.Code)] {
			return false
		}
	}
	return true
}

// yymmdd turns "2026-07-15" into "260715". Falls back to the digit-stripped
// input when the date isn't in the expected form.
func yymmdd(date string) string {
	digits := strings.ReplaceAll(date, "-", "")
	if len(digits) == 8 {
		return digits[2:]
	}
	return digits
}

// normalizeCabin maps the CLI's cabin vocabulary to FlySoar's request value.
// FlySoar accepts the lowercase Duffel cabin names; unknown values pass through
// lowercased so a caller can reach a cabin we don't yet enumerate.
func normalizeCabin(cabin string) string {
	c := strings.ToLower(strings.TrimSpace(cabin))
	switch c {
	case "", "economy", "coach":
		return "economy"
	case "premium_economy", "premium-economy", "premium economy", "premium":
		return "premium_economy"
	case "business":
		return "business"
	case "first":
		return "first"
	default:
		return c
	}
}

// parseSSEOffers consumes a Server-Sent Events stream and returns the decoded
// payloads of every `event: offer`. Non-offer events (created, batch, done)
// are ignored; the loop stops on `event: done` or EOF. A single un-decodable
// offer payload is skipped rather than failing the whole stream.
func parseSSEOffers(r interface{ Read([]byte) (int, error) }) ([]rawOffer, error) {
	scanner := bufio.NewScanner(r)
	// Offers can be large (multi-slice itineraries); lift the line cap to 1 MiB.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var offers []rawOffer
	var currentEvent string
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			currentEvent = ""
			continue
		}
		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(line[len("event:"):])
			if currentEvent == "done" {
				break
			}
			continue
		}
		if strings.HasPrefix(line, "data:") && currentEvent == "offer" {
			payload := strings.TrimSpace(line[len("data:"):])
			var offer rawOffer
			if err := json.Unmarshal([]byte(payload), &offer); err != nil {
				continue
			}
			offers = append(offers, offer)
		}
	}
	if err := scanner.Err(); err != nil {
		return offers, err
	}
	return offers, nil
}

// normalizeOffers converts raw Duffel offers into cheapest-per-itinerary
// Flights. Identical itineraries (same per-leg carrier + flight number +
// departure) collapse to a single entry keeping the lowest fare. passengers
// divides the party total into a per-seat price (see normalizeOffer).
func normalizeOffers(offers []rawOffer, fallbackCurrency string, passengers int) []Flight {
	byKey := make(map[string]Flight)
	order := make([]string, 0, len(offers))
	for _, o := range offers {
		f, ok := normalizeOffer(o, fallbackCurrency, passengers)
		if !ok {
			continue
		}
		key := itineraryKey(f)
		if existing, seen := byKey[key]; seen {
			if f.Price < existing.Price {
				byKey[key] = f
			}
			continue
		}
		byKey[key] = f
		order = append(order, key)
	}
	out := make([]Flight, 0, len(order))
	for _, k := range order {
		out = append(out, byKey[k])
	}
	return out
}

// normalizeOffer flattens one Duffel offer into a Flight. Returns ok=false when
// the offer has no priced, non-empty itinerary. FlySoar/Duffel quotes
// total_amount for the whole party, so with passengers > 1 the fare is divided
// to a per-seat price — matching the Google Flights backend's contract that
// Flight.Price is per-seat (see the flights-passengers-per-seat-price patch).
func normalizeOffer(o rawOffer, fallbackCurrency string, passengers int) (Flight, bool) {
	if len(o.Slices) == 0 {
		return Flight{}, false
	}
	price, perr := strconv.ParseFloat(strings.TrimSpace(o.TotalAmount), 64)
	if perr != nil || price <= 0 {
		return Flight{}, false
	}
	if passengers > 1 {
		// Round to cents so a 3-way split doesn't produce a repeating decimal.
		price = math.Round(price/float64(passengers)*100) / 100
	}
	cur := strings.ToUpper(strings.TrimSpace(o.Currency))
	if cur == "" {
		cur = fallbackCurrency
	}

	var legs []Leg
	var sliceStops []int
	segCount := 0
	for _, s := range o.Slices {
		n := len(s.Segments)
		for _, seg := range s.Segments {
			legs = append(legs, normalizeSegment(seg))
			segCount++
		}
		// Connections within this slice (this direction): segments - 1.
		conns := n - 1
		if conns < 0 {
			conns = 0
		}
		sliceStops = append(sliceStops, conns)
	}
	if len(legs) == 0 {
		return Flight{}, false
	}

	// Stops are total connections within the trip: sum of per-slice connections.
	stops := 0
	for _, c := range sliceStops {
		stops += c
	}

	total := 0
	for _, l := range legs {
		total += l.DurationMinutes
	}

	f := Flight{
		OfferID:         o.ID,
		Price:           price,
		Currency:        cur,
		DurationMinutes: total,
		Stops:           stops,
		SliceStops:      sliceStops,
		SliceCount:      len(o.Slices),
		Owner:           firstNonEmpty(o.Owner.Name, o.Owner.IATACode),
		Legs:            legs,
	}
	return f, true
}

func normalizeSegment(seg rawSegment) Leg {
	carrierCode := firstNonEmpty(seg.CarrierIATA, seg.MarketingCarrier.IATACode, seg.OperatingCarrier.IATACode)
	carrierName := firstNonEmpty(seg.CarrierName, seg.MarketingCarrier.Name, seg.OperatingCarrier.Name)
	return Leg{
		DepartureAirport: Airport{Code: strings.ToUpper(seg.Origin.IATACode), Name: seg.Origin.Name},
		ArrivalAirport:   Airport{Code: strings.ToUpper(seg.Destination.IATACode), Name: seg.Destination.Name},
		DepartureTime:    seg.Departure,
		ArrivalTime:      seg.Arrival,
		DurationMinutes:  parseISODurationMinutes(seg.Duration),
		Airline:          Airline{Code: strings.ToUpper(carrierCode), Name: carrierName},
		FlightNumber:     normalizeFlightNumber(carrierCode, seg.FlightNumber),
		CabinClass:       seg.CabinClass,
		AircraftType:     seg.Aircraft,
	}
}

// itineraryKey builds a stable identity for an itinerary from its per-leg
// carrier + flight number + departure timestamp, so equivalent offers dedupe.
func itineraryKey(f Flight) string {
	parts := make([]string, 0, len(f.Legs))
	for _, l := range f.Legs {
		parts = append(parts, fmt.Sprintf("%s%s@%s", l.Airline.Code, l.FlightNumber, l.DepartureTime))
	}
	return strings.Join(parts, "+")
}

// normalizeFlightNumber returns a "AA475"-style number. FlySoar sometimes
// returns the bare number and sometimes the carrier-prefixed form; this
// produces a consistent prefixed value and strips leading zeros.
func normalizeFlightNumber(carrier, num string) string {
	num = strings.TrimSpace(num)
	if num == "" {
		return ""
	}
	num = strings.ToUpper(num)
	carrier = strings.ToUpper(strings.TrimSpace(carrier))
	if carrier != "" && strings.HasPrefix(num, carrier) {
		num = strings.TrimSpace(num[len(carrier):])
	}
	num = strings.TrimLeft(num, "0")
	if num == "" {
		num = "0"
	}
	if carrier == "" {
		return num
	}
	return carrier + num
}

// parseISODurationMinutes parses an ISO 8601 duration like "PT5H30M" into
// minutes. Returns 0 for empty or unparseable input.
func parseISODurationMinutes(d string) int {
	d = strings.TrimSpace(strings.ToUpper(d))
	if d == "" || !strings.HasPrefix(d, "PT") {
		return 0
	}
	d = d[len("PT"):]
	var minutes, cur int
	for _, r := range d {
		switch {
		case r >= '0' && r <= '9':
			cur = cur*10 + int(r-'0')
		case r == 'H':
			minutes += cur * 60
			cur = 0
		case r == 'M':
			minutes += cur
			cur = 0
		default:
			cur = 0
		}
	}
	return minutes
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
