// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package gflights

// PATCH(library): Google Flights multi-city URL builder.
//
// Multi-city queries (trip_type=3) are gated server-side: the shopping POST
// endpoint requires an authenticated Google session (SAPISID cookie + XSRF
// hash). Anonymous POSTs — the only kind a server-side CLI can issue without
// a cookie jar — return travel.frontend.flights.ErrorResponse regardless of
// f.sid / build / token tweaks. Verified empirically: even with a session
// token scraped from the home page placed at inner[0][3], an anonymous POST
// rejects.
//
// What DOES work without auth: navigating to a URL whose `tfs` parameter
// encodes the multi-city query. Google's UI accepts the URL, renders the
// multi-city search form prefilled with our airports and dates, fires its
// own authenticated POST from the user's browser session, and renders the
// results. So for v1, multi-city flight-goat queries return a URL the user
// (or agent) opens in a browser — semantically equivalent to a deeplink.
//
// Schema of the tfs protobuf (reverse-engineered from a browser-captured
// multi-city query, fields decoded by inspection):
//
//   field 1, varint = 28          (some "include-all-stops" flag)
//   field 2, varint = 1           (page state; value 1 is what encodeTFSMultiCity
//                                  emits and what the live smoke test confirmed
//                                  renders the prefilled multi-city UI — do not
//                                  "correct" to 2 without a fresh browser capture)
//   field 3, len-delim segments   (repeated, one per leg)
//     within each segment:
//       field 2, len-delim "YYYY-MM-DD"
//       field 13, len-delim airport-block (origin)
//       field 14, len-delim airport-block (destination)
//     airport-block = 0x08 0x02 0x12 <len> "IATA"
//   field 8,  varint = 1          (currency/locale flag)
//   field 9,  varint = 1
//   field 14, varint = 1
//   field 16, len-delim, body 0x08 ff*9 01   (signed -1 ack)
//   field 19, varint = 3          (TRIP_TYPE_MULTI_CITY)
//
// When Google adds cookie/SAPISIDHASH-aware HTTP support to flight-goat,
// the POST shape we've already built (see buildOffersPayload + Segments
// handling) will Just Work without changes — see also the .printing-press-
// patches/multicity.json note. For programmatic multi-city flight prices
// today without cookies, use Kayak (separate PR).

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// MultiCityBookingURL returns the canonical Google Flights search URL for the
// given segments. Open this URL in a browser to see results (Google's UI
// handles authentication and POSTs the actual search from the user's session).
// Returns an error if segments are missing required fields.
func MultiCityBookingURL(segments []Segment) (string, error) {
	if len(segments) < 2 {
		return "", fmt.Errorf("multi-city requires >= 2 segments; got %d", len(segments))
	}
	for i, s := range segments {
		if _, err := time.Parse("2006-01-02", s.DepartureDate); err != nil {
			return "", fmt.Errorf("segment %d date %q: must be YYYY-MM-DD", i+1, s.DepartureDate)
		}
		if strings.TrimSpace(s.Origin) == "" || strings.TrimSpace(s.Destination) == "" {
			return "", fmt.Errorf("segment %d: origin and destination required", i+1)
		}
	}
	raw := encodeTFSMultiCity(segments)
	tfs := base64.RawURLEncoding.EncodeToString(raw)
	tfu := "KgIIAw" // protobuf bytes: field 5 length-delim with {field 1 varint = 3} → trip_type=3 marker
	return "https://www.google.com/travel/flights?" + url.Values{
		"tfs": {tfs},
		"tfu": {tfu},
	}.Encode(), nil
}

// encodeTFSMultiCity emits the protobuf bytes Google's UI parses out of the
// tfs URL parameter. Hand-rolled (no protoc dependency) because the only
// fields that matter are a small fixed set; see the package doc for the
// schema.
func encodeTFSMultiCity(segments []Segment) []byte {
	var out []byte
	out = append(out, pbVarintField(1, 28)...)
	out = append(out, pbVarintField(2, 1)...)
	for _, s := range segments {
		out = append(out, pbLenDelimField(3, pbSegment(s))...)
	}
	out = append(out, pbVarintField(8, 1)...)
	out = append(out, pbVarintField(9, 1)...)
	out = append(out, pbVarintField(14, 1)...)
	// field 16 length-delim, body 0x08 + 9× 0xff + 0x01 (Google's literal padding bytes)
	body16 := append([]byte{0x08}, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01}...)
	out = append(out, pbLenDelimField(16, body16)...)
	out = append(out, pbVarintField(19, 3)...) // trip_type = 3 (multi-city)
	return out
}

func pbSegment(s Segment) []byte {
	var b []byte
	// field 2 (date): length-delim string. Same shape as the other
	// length-delimited fields in this file; using the helper rather than
	// a raw 0x12 + byte-cast keeps the encoding style uniform.
	b = append(b, pbLenDelimField(2, []byte(s.DepartureDate))...)
	// field 13 (origin airport)
	b = append(b, pbAirportBlock(13, strings.ToUpper(s.Origin))...)
	// field 14 (destination airport)
	b = append(b, pbAirportBlock(14, strings.ToUpper(s.Destination))...)
	return b
}

func pbAirportBlock(field int, code string) []byte {
	// inner: 08 02 (field 1 varint 2) 12 <len> <code>
	inner := []byte{0x08, 0x02, 0x12, byte(len(code))}
	inner = append(inner, []byte(code)...)
	return pbLenDelimField(field, inner)
}

// pbVarint encodes a non-negative int as a base-128 varint.
func pbVarint(n int) []byte {
	if n < 0 {
		// negative ints would need 10-byte zig-zag; we don't use them here.
		return []byte{0}
	}
	var out []byte
	for {
		b := byte(n & 0x7F)
		n >>= 7
		if n == 0 {
			out = append(out, b)
			return out
		}
		out = append(out, b|0x80)
	}
}

func pbTag(field, wire int) []byte {
	return pbVarint((field << 3) | wire)
}

func pbVarintField(field, n int) []byte {
	return append(pbTag(field, 0), pbVarint(n)...)
}

func pbLenDelimField(field int, body []byte) []byte {
	out := pbTag(field, 2)
	out = append(out, pbVarint(len(body))...)
	out = append(out, body...)
	return out
}
