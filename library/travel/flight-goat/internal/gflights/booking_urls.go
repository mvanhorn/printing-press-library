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

// BookingURLs lives on Flight. Google is always populated for valid queries;
// Airline is populated only when the itinerary qualifies.
type BookingURLs struct {
	Google  string `json:"google"`
	Airline string `json:"airline,omitempty"`
}

const googleFlightsSearchBase = "https://www.google.com/travel/flights/search"

// buildBookingURLs composes the per-flight booking URLs. The opts argument
// carries the user's original query (origin/destination/dates/pax). The
// flight argument supplies the operating airline(s) to decide whether a
// direct airline link is appropriate.
func buildBookingURLs(opts SearchOptions, fl Flight) BookingURLs {
	out := BookingURLs{
		Google: buildGoogleFlightsURL(opts),
	}
	if airlineURL, ok := buildAirlineURL(opts, fl); ok {
		out.Airline = airlineURL
	}
	return out
}

// buildGoogleFlightsURL constructs the tfs= deeplink. The protobuf shape is:
//
//	field 2 (varint): trip type — 1=one-way, 2=round-trip
//	field 3 (length-delimited, repeated): travel slice {
//	    field 2 (length-delimited, message): origin {
//	        field 2 (length-delimited, string): IATA code
//	    }
//	    field 6 (length-delimited, message): destination {
//	        field 2 (length-delimited, string): IATA code
//	    }
//	    field 13 (length-delimited, string): YYYY-MM-DD
//	}
//	field 4 (varint): adults
//
// The shape is the minimum Google needs to land a working search. Cabin
// class and additional filters are intentionally omitted — they default
// to the standard "any class" search, which is the right thing for a
// "view in Google Flights" handoff.
func buildGoogleFlightsURL(opts SearchOptions) string {
	if opts.Origin == "" || opts.Destination == "" || opts.DepartureDate == "" {
		return ""
	}

	tripType := 1 // one-way
	if opts.ReturnDate != "" {
		tripType = 2
	}

	var pb []byte
	pb = protowire.AppendTag(pb, 2, protowire.VarintType)
	pb = protowire.AppendVarint(pb, uint64(tripType))

	outbound := encodeTravelSlice(opts.Origin, opts.Destination, opts.DepartureDate)
	pb = protowire.AppendTag(pb, 3, protowire.BytesType)
	pb = protowire.AppendVarint(pb, uint64(len(outbound)))
	pb = append(pb, outbound...)

	if opts.ReturnDate != "" {
		inbound := encodeTravelSlice(opts.Destination, opts.Origin, opts.ReturnDate)
		pb = protowire.AppendTag(pb, 3, protowire.BytesType)
		pb = protowire.AppendVarint(pb, uint64(len(inbound)))
		pb = append(pb, inbound...)
	}

	pax := opts.Passengers
	if pax < 1 {
		pax = 1
	}
	pb = protowire.AppendTag(pb, 4, protowire.VarintType)
	pb = protowire.AppendVarint(pb, uint64(pax))

	tfs := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(pb)
	return fmt.Sprintf("%s?tfs=%s&hl=en", googleFlightsSearchBase, tfs)
}

func encodeTravelSlice(origin, destination, date string) []byte {
	var slice []byte

	// Origin airport object: field 2, message containing IATA at field 2.
	originMsg := encodeAirportObject(origin)
	slice = protowire.AppendTag(slice, 2, protowire.BytesType)
	slice = protowire.AppendVarint(slice, uint64(len(originMsg)))
	slice = append(slice, originMsg...)

	// Destination airport object: field 6.
	destMsg := encodeAirportObject(destination)
	slice = protowire.AppendTag(slice, 6, protowire.BytesType)
	slice = protowire.AppendVarint(slice, uint64(len(destMsg)))
	slice = append(slice, destMsg...)

	// Date string at field 13.
	slice = protowire.AppendTag(slice, 13, protowire.BytesType)
	slice = protowire.AppendString(slice, date)

	return slice
}

func encodeAirportObject(iata string) []byte {
	var msg []byte
	msg = protowire.AppendTag(msg, 2, protowire.BytesType)
	msg = protowire.AppendString(msg, strings.ToUpper(iata))
	return msg
}

// airlineTemplate maps an IATA airline code to a deeplink template plus
// flags for which substitutions the carrier accepts.
type airlineTemplate struct {
	// URL with placeholders {origin}, {destination}, {depart}, {return}, {pax}.
	urlTemplate string
	// roundTripOnly templates omit themselves for one-way; oneWayOnly the opposite.
	// Empty means "supports both".
	mode string
}

// airlineTemplates is a curated set of carrier booking-search-form URLs.
// Each entry was constructed from live carrier sites and the patterns the
// search forms emit when submitted. Patterns drift; when one breaks, the
// fix is one line and the Google fallback continues to serve users.
var airlineTemplates = map[string]airlineTemplate{
	// Alaska Airlines — standard alaskaair.com booking form.
	"AS": {
		urlTemplate: "https://www.alaskaair.com/planbook?from={origin}&to={destination}&departureDate={depart}&returnDate={return}&adults={pax}&tripType={trip_type}",
	},
	// American Airlines — aa.com search-flights endpoint.
	"AA": {
		urlTemplate: "https://www.aa.com/booking/find-flights?from={origin}&to={destination}&departDate={depart}&returnDate={return}&adultPassengerCount={pax}&type={trip_type}",
	},
	// United Airlines — united.com flight search.
	"UA": {
		urlTemplate: "https://www.united.com/en/us/fsr/choose-flights?f={origin}&t={destination}&d={depart}&r={return}&px={pax}&tt={trip_type_int}&sc=7&taxng=1&clm=7",
	},
	// JetBlue — jetblue.com booking entry.
	"B6": {
		urlTemplate: "https://www.jetblue.com/booking/flights?from={origin}&to={destination}&depart={depart}&return={return}&isMultiCity=false&noOfRoute=1&adults={pax}",
	},
	// Air Canada — aircanada.com flight finder.
	"AC": {
		urlTemplate: "https://www.aircanada.com/aco/en_us/aco-booking-flights/flight-search?orgCity1={origin}&destCity1={destination}&date1={depart}&date2={return}&numAdults={pax}",
	},
	// British Airways — britishairways.com flight search.
	"BA": {
		urlTemplate: "https://www.britishairways.com/travel/fx/public/en_us?eId=120001&depAirport={origin}&arrAirport={destination}&outboundDate={depart}&inboundDate={return}&adults={pax}",
	},
}

// buildAirlineURL returns an airline-direct URL when the itinerary
// qualifies. Qualification: all legs operated by a single carrier in
// airlineTemplates. Codeshare flights, regional carriers outside the
// table, and itineraries with mixed operators omit the airline URL
// (the Google fallback always populates).
func buildAirlineURL(opts SearchOptions, fl Flight) (string, bool) {
	if len(fl.Legs) == 0 {
		return "", false
	}
	first := strings.ToUpper(fl.Legs[0].Airline.Code)
	if first == "" {
		return "", false
	}
	for _, leg := range fl.Legs[1:] {
		if !strings.EqualFold(leg.Airline.Code, first) {
			return "", false
		}
	}
	tmpl, ok := airlineTemplates[first]
	if !ok {
		return "", false
	}
	// PATCH(greptile P2): enforce the documented mode contract so a future
	// roundTripOnly/oneWayOnly entry doesn't silently generate URLs for
	// the unsupported trip type.
	isRoundTrip := opts.ReturnDate != ""
	if tmpl.mode == "roundTripOnly" && !isRoundTrip {
		return "", false
	}
	if tmpl.mode == "oneWayOnly" && isRoundTrip {
		return "", false
	}

	tripType := "OneWay"
	tripTypeInt := "1"
	if isRoundTrip {
		tripType = "RoundTrip"
		tripTypeInt = "2"
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
	)
	built := r.Replace(tmpl.urlTemplate)
	// PATCH(greptile P2): one-way searches leave the {return} placeholder
	// expanding to an empty string. Carriers without an explicit trip-type
	// param (B6, BA) treat returnDate=/inboundDate= as round-trip with
	// missing dates and may default the form to a round-trip search.
	// Strip query params with empty values so the form sees a clean
	// one-way query.
	if !isRoundTrip {
		built = stripEmptyQueryParams(built)
	}
	return built, true
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
