// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// PATCH(library): booking-URL deeplinks for the Flight result rows.
//
// Two outputs per flight when possible:
//
//   google:  a Google Flights URL that loads the same search the CLI ran.
//            We encode the trip type, origin/destination IATA codes, dates,
//            and passenger count into Google's tfs= protobuf parameter and
//            wrap with base64. The exact field numbers are derived from
//            community reverse-engineering of the format and confirmed by
//            decoding URLs Google's own "share search" feature produces.
//            Worst case (encoding subtly drifts): Google lands the user on
//            its Flights UI with the route pre-filled — strictly better
//            than no link.
//
//   airline: an airline.com booking-form URL when all legs of the
//            itinerary are operated by a single carrier in the curated
//            table below. Codeshare itineraries omit this and rely on the
//            Google fallback. The table covers the carriers most commonly
//            surfaced by SEA/LAX/JFK origins; add entries as new patterns
//            are verified live.

package gflights

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"google.golang.org/protobuf/encoding/protowire"
)

// BookingURLs lives on Flight and carries one-tap handoff URLs.
//
// Primary is the recommended single URL for one-tap UX; PrimaryKind tells the
// agent or UI what to call it: "Book on Delta" (prefill), "Open Delta booking"
// (landing), "View on Google Flights" (search).
//
// AirlineURL and AirlineKind are populated only when the itinerary qualifies
// (single-carrier operator in airlineTemplates). GoogleURL and Primary are
// populated for any valid query (Origin, Destination, DepartureDate all
// non-empty); buildBookingURLs returns a zero value otherwise.
type BookingURLs struct {
	Primary      string `json:"primary"`
	PrimaryKind  string `json:"primary_kind"`
	AirlineURL   string `json:"airline_url,omitempty"`
	AirlineKind  string `json:"airline_kind,omitempty"`
	GoogleURL    string `json:"google_url"`
}

// PrimaryKind values: "prefill" and "landing" are airline-direct; "search"
// is the Google Flights fallback.
const (
	primaryKindPrefill = "prefill"
	primaryKindLanding = "landing"
	primaryKindSearch  = "search"
)

const googleFlightsSearchBase = "https://www.google.com/travel/flights/search"

// buildBookingURLs composes the per-flight booking URLs. Returns a zero
// value for degenerate queries (missing Origin / Destination / DepartureDate)
// so SKILL.md's "always populated" contract on Primary and GoogleURL holds
// for any non-zero return. Primary preference: airline (any kind) over google.
func buildBookingURLs(opts SearchOptions, fl Flight) BookingURLs {
	googleURL := buildGoogleFlightsURL(opts)
	if googleURL == "" {
		return BookingURLs{}
	}
	out := BookingURLs{
		GoogleURL:   googleURL,
		Primary:     googleURL,
		PrimaryKind: primaryKindSearch,
	}
	if airlineURL, kind, ok := buildAirlineURL(opts, fl); ok {
		out.AirlineURL = airlineURL
		out.AirlineKind = kind
		out.Primary = airlineURL
		out.PrimaryKind = kind
	}
	return out
}

// buildGoogleFlightsURL constructs the tfs= deeplink against Google Flights'
// documented protobuf schema. The canonical .proto lives at
// https://github.com/krisukox/google-flights-api/blob/main/flights/internal/urlpb/url.proto
//
//	message Url {
//	    repeated Flight flight = 3;
//	    repeated Traveler travelers = 8;   // one enum value per passenger
//	    Class class = 9;                    // 1=economy
//	    TripType tripType = 19;             // 1=round-trip, 2=one-way
//	}
//	message Flight {
//	    string date = 2;                    // YYYY-MM-DD
//	    optional Stops stops = 5;
//	    repeated Location srcLocations = 13;
//	    repeated Location dstLocations = 14;
//	}
//	message Location {
//	    LocationType type = 1;              // 1=AIRPORT
//	    string name = 2;                    // IATA code
//	}
//
// Base64: URL-safe, no padding. URL params: tfs, curr, hl.
//
// Round-trip emits two Flight messages with origin and destination swapped on
// the return slice. The travelers field is repeated; emit one enum entry per
// passenger (NOT a count). Class defaults to ECONOMY (1).
func buildGoogleFlightsURL(opts SearchOptions) string {
	if opts.Origin == "" || opts.Destination == "" || opts.DepartureDate == "" {
		return ""
	}

	tripType := googleTripTypeOneWay
	if opts.ReturnDate != "" {
		tripType = googleTripTypeRoundTrip
	}
	pax := opts.Passengers
	if pax < 1 {
		pax = 1
	}

	var pb []byte

	outbound := encodeFlightSlice(opts.Origin, opts.Destination, opts.DepartureDate)
	pb = protowire.AppendTag(pb, 3, protowire.BytesType)
	pb = protowire.AppendVarint(pb, uint64(len(outbound)))
	pb = append(pb, outbound...)

	if opts.ReturnDate != "" {
		inbound := encodeFlightSlice(opts.Destination, opts.Origin, opts.ReturnDate)
		pb = protowire.AppendTag(pb, 3, protowire.BytesType)
		pb = protowire.AppendVarint(pb, uint64(len(inbound)))
		pb = append(pb, inbound...)
	}

	for i := 0; i < pax; i++ {
		pb = protowire.AppendTag(pb, 8, protowire.VarintType)
		pb = protowire.AppendVarint(pb, uint64(googleTravelerAdult))
	}

	pb = protowire.AppendTag(pb, 9, protowire.VarintType)
	pb = protowire.AppendVarint(pb, uint64(googleClassEconomy))

	pb = protowire.AppendTag(pb, 19, protowire.VarintType)
	pb = protowire.AppendVarint(pb, uint64(tripType))

	tfs := base64.RawURLEncoding.EncodeToString(pb)
	return fmt.Sprintf("%s?tfs=%s&curr=USD&hl=en", googleFlightsSearchBase, tfs)
}

// Google Flights protobuf enum values, matching krisukox url.proto.
const (
	googleTripTypeRoundTrip = 1
	googleTripTypeOneWay    = 2

	googleTravelerAdult = 1

	googleClassEconomy = 1

	googleLocationTypeAirport = 1
)

// encodeFlightSlice builds the inner Flight message: date string at field 2,
// origin Location at field 13 (repeated), destination Location at field 14.
func encodeFlightSlice(origin, destination, date string) []byte {
	var slice []byte

	slice = protowire.AppendTag(slice, 2, protowire.BytesType)
	slice = protowire.AppendString(slice, date)

	originLoc := encodeLocation(origin)
	slice = protowire.AppendTag(slice, 13, protowire.BytesType)
	slice = protowire.AppendVarint(slice, uint64(len(originLoc)))
	slice = append(slice, originLoc...)

	destLoc := encodeLocation(destination)
	slice = protowire.AppendTag(slice, 14, protowire.BytesType)
	slice = protowire.AppendVarint(slice, uint64(len(destLoc)))
	slice = append(slice, destLoc...)

	return slice
}

// encodeLocation builds the Location sub-message: LocationType at field 1
// (1=AIRPORT), IATA name string at field 2.
func encodeLocation(iata string) []byte {
	var msg []byte
	msg = protowire.AppendTag(msg, 1, protowire.VarintType)
	msg = protowire.AppendVarint(msg, uint64(googleLocationTypeAirport))
	msg = protowire.AppendTag(msg, 2, protowire.BytesType)
	msg = protowire.AppendString(msg, strings.ToUpper(iata))
	return msg
}

// airlineTemplate maps an IATA airline code to a deeplink template plus
// metadata about what kind of URL it produces.
//
// kind classifies the URL's behavior when visited:
//
//	prefill — URL params survive page load and pre-fill the booking form.
//	          User clicks once and sees results ready.
//	landing — URL lands the user on the carrier's booking entry; route
//	          and dates may not pre-fill. Still a one-tap improvement
//	          since the user already chose this airline.
//
// See testdata/airline_url_captures.md for the source of truth on each entry.
type airlineTemplate struct {
	urlTemplate string
	kind        string // "prefill" or "landing"
	// roundTripOnly templates omit themselves for one-way; oneWayOnly the opposite.
	// Empty means "supports both".
	mode string
}

// airlineTemplates is a curated set of carrier booking URLs. Each entry is
// classified prefill or landing per testdata/airline_url_captures.md.
//
// Patterns drift; when one breaks, recapture by visiting the carrier's site
// and updating both the map and the captures log. The Google fallback in
// buildGoogleFlightsURL continues to serve users when any entry is broken.
var airlineTemplates = map[string]airlineTemplate{
	// ---- prefill (params survive and pre-fill the form) ----

	// Delta — directly observable URL params.
	"DL": {
		urlTemplate: "https://www.delta.com/flightsearch/book-a-flight?tripType={trip_type_dl}&originCity={origin}&destinationCity={destination}&departureDate={depart}&returnDate={return}&paxCount={pax}",
		kind:        airlineKindPrefill,
	},
	// Southwest — param names from mobile.southwest.com URL bar.
	"WN": {
		urlTemplate: "https://www.southwest.com/air/booking/?originationAirportCode={origin}&destinationAirportCode={destination}&departureDate={depart}&returnDate={return}&adultPassengersCount={pax}&tripType={trip_type_wn}",
		kind:        airlineKindPrefill,
	},
	// Lufthansa — officially documented at developer.lufthansa.com.
	"LH": {
		urlTemplate: "https://www.lufthansa.com/deeplink/partner?airlineCode=LH&originCode={origin}&destinationCode={destination}&travelDate={depart}&returnDate={return}&travelers=adult={pax}",
		kind:        airlineKindPrefill,
	},
	// Swiss — same Lufthansa Group spec, airlineCode=LX.
	"LX": {
		urlTemplate: "https://www.lufthansa.com/deeplink/partner?airlineCode=LX&originCode={origin}&destinationCode={destination}&travelDate={depart}&returnDate={return}&travelers=adult={pax}",
		kind:        airlineKindPrefill,
	},

	// ---- landing (URL lands user on carrier's booking surface) ----

	"AS": {urlTemplate: "https://www.alaskaair.com/search/flights?O={origin}&D={destination}&OD={depart}&RD={return}&A={pax}", kind: airlineKindLanding},
	"AA": {urlTemplate: "https://www.aa.com/booking/find-flights?from={origin}&to={destination}&departDate={depart}&returnDate={return}&adultPassengerCount={pax}&type={trip_type}", kind: airlineKindLanding},
	"UA": {urlTemplate: "https://www.united.com/en/us/fsr/choose-flights?f={origin}&t={destination}&d={depart}&r={return}&px={pax}&tt={trip_type_int}&sc=7&taxng=1&clm=7", kind: airlineKindLanding},
	"B6": {urlTemplate: "https://www.jetblue.com/booking/flights?from={origin}&to={destination}&depart={depart}&return={return}&isMultiCity=false&noOfRoute=1&adults={pax}", kind: airlineKindLanding},
	"F9": {urlTemplate: "https://booking.flyfrontier.com/", kind: airlineKindLanding},
	"AC": {urlTemplate: "https://www.aircanada.com/aco/en_us/aco-booking-flights/flight-search?orgCity1={origin}&destCity1={destination}&date1={depart}&date2={return}&numAdults={pax}", kind: airlineKindLanding},
	"BA": {urlTemplate: "https://www.britishairways.com/travel/fx/public/en_us?eId=120001&depAirport={origin}&arrAirport={destination}&outboundDate={depart}&inboundDate={return}&adults={pax}", kind: airlineKindLanding},
	"AF": {urlTemplate: "https://wwws.airfrance.us/", kind: airlineKindLanding},
	"KL": {urlTemplate: "https://www.klm.com/en-us/flights", kind: airlineKindLanding},
	"IB": {urlTemplate: "https://www.iberia.com/us/flight-search-engine/", kind: airlineKindLanding},
	"VS": {urlTemplate: "https://www.virginatlantic.com/en-US", kind: airlineKindLanding},
	"SK": {urlTemplate: "https://www.flysas.com/en", kind: airlineKindLanding},
	"AY": {urlTemplate: "https://www.finnair.com/us/en", kind: airlineKindLanding},
	"EI": {urlTemplate: "https://www.aerlingus.com/html/dashboard.html", kind: airlineKindLanding},
	"DE": {urlTemplate: "https://www.condor.com/us/", kind: airlineKindLanding},
	"EK": {urlTemplate: "https://www.emirates.com/english/book/", kind: airlineKindLanding},
	"QR": {urlTemplate: "https://booking.qatarairways.com/nsp/views/deepLinkLoader.xhtml", kind: airlineKindLanding},
	"EY": {urlTemplate: "https://www.etihad.com/en-us/book", kind: airlineKindLanding},
	"SQ": {urlTemplate: "https://www.singaporeair.com/en_UK/us/home", kind: airlineKindLanding},
	"BR": {urlTemplate: "https://booking.evaair.com/flyeva/eva/b2c/booking-online.aspx?lang=en-us", kind: airlineKindLanding},
	"CX": {urlTemplate: "https://www.cathaypacific.com/cx/en_US.html", kind: airlineKindLanding},
	"KE": {urlTemplate: "https://www.koreanair.com/booking/search", kind: airlineKindLanding},
	"NH": {urlTemplate: "https://www.ana.co.jp/en/us/plan-book/", kind: airlineKindLanding},
	"JL": {urlTemplate: "https://www.jal.co.jp/jp/en/inter/booking/", kind: airlineKindLanding},
	"TG": {urlTemplate: "https://www.thaiairways.com/en/book/booking.page", kind: airlineKindLanding},
	"PG": {urlTemplate: "https://www.bangkokair.com/flight/booking", kind: airlineKindLanding},
	"HU": {urlTemplate: "https://www.hainanairlines.com/US/US/Search", kind: airlineKindLanding},
	"CI": {urlTemplate: "https://www.china-airlines.com/us/en/booking/book-flights", kind: airlineKindLanding},
	"OZ": {urlTemplate: "https://flyasiana.com/C/US/EN/index", kind: airlineKindLanding},
	"JX": {urlTemplate: "https://www.starlux-airlines.com/en-US/booking/book-flight/search-a-flight", kind: airlineKindLanding},
	"ET": {urlTemplate: "https://www.ethiopianairlines.com/us/book/booking/flight", kind: airlineKindLanding},
}

// airlineKind* alias the matching primaryKind* constants so airlineTemplate.kind
// and BookingURLs.AirlineKind / PrimaryKind share one source of truth.
// PATCH(greptile P2): single set of string values prevents callers and tests
// from accidentally mixing two parallel const blocks.
const (
	airlineKindPrefill = primaryKindPrefill
	airlineKindLanding = primaryKindLanding
)

// buildAirlineURL returns an airline-direct URL when the itinerary
// qualifies. Returns (url, kind, ok) where kind is "prefill" or "landing".
// Qualification: all legs operated by a single carrier in airlineTemplates.
// Codeshare flights, regional carriers outside the table, and itineraries
// with mixed operators return ("", "", false). The Google fallback in
// buildGoogleFlightsURL always populates regardless.
func buildAirlineURL(opts SearchOptions, fl Flight) (string, string, bool) {
	if len(fl.Legs) == 0 {
		return "", "", false
	}
	first := strings.ToUpper(fl.Legs[0].Airline.Code)
	if first == "" {
		return "", "", false
	}
	for _, leg := range fl.Legs[1:] {
		if !strings.EqualFold(leg.Airline.Code, first) {
			return "", "", false
		}
	}
	tmpl, ok := airlineTemplates[first]
	if !ok {
		return "", "", false
	}
	// PATCH(greptile P2): enforce the documented mode contract so a future
	// roundTripOnly/oneWayOnly entry doesn't silently generate URLs for
	// the unsupported trip type.
	isRoundTrip := opts.ReturnDate != ""
	if tmpl.mode == "roundTripOnly" && !isRoundTrip {
		return "", "", false
	}
	if tmpl.mode == "oneWayOnly" && isRoundTrip {
		return "", "", false
	}

	tripType := "OneWay"
	tripTypeInt := "1"
	tripTypeDL := "ONE_WAY"
	tripTypeWN := "oneway"
	if isRoundTrip {
		tripType = "RoundTrip"
		tripTypeInt = "2"
		tripTypeDL = "ROUND_TRIP"
		tripTypeWN = "roundtrip"
	}
	pax := opts.Passengers
	if pax < 1 {
		pax = 1
	}

	r := strings.NewReplacer(
		"{origin}", url.QueryEscape(strings.ToUpper(opts.Origin)),
		"{destination}", url.QueryEscape(strings.ToUpper(opts.Destination)),
		"{depart}", url.QueryEscape(opts.DepartureDate),
		"{return}", url.QueryEscape(opts.ReturnDate),
		"{pax}", fmt.Sprintf("%d", pax),
		"{trip_type}", tripType,
		"{trip_type_int}", tripTypeInt,
		"{trip_type_dl}", tripTypeDL,
		"{trip_type_wn}", tripTypeWN,
	)
	built := r.Replace(tmpl.urlTemplate)
	// PATCH(greptile P2): one-way searches leave the {return} placeholder
	// expanding to an empty string. Strip query params with empty values
	// so carrier forms without an explicit trip-type indicator (B6, BA, etc.)
	// see a clean one-way query.
	if !isRoundTrip {
		built = stripEmptyQueryParams(built)
	}
	return built, tmpl.kind, true
}

// stripEmptyQueryParams removes query parameters whose value is empty
// from the URL, preserving order and fragment.
func stripEmptyQueryParams(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if u.RawQuery == "" {
		return rawURL
	}
	pairs := strings.Split(u.RawQuery, "&")
	kept := pairs[:0]
	for _, p := range pairs {
		if p == "" {
			continue
		}
		eq := strings.IndexByte(p, '=')
		if eq < 0 {
			kept = append(kept, p)
			continue
		}
		if eq == len(p)-1 {
			continue
		}
		kept = append(kept, p)
	}
	u.RawQuery = strings.Join(kept, "&")
	return u.String()
}
