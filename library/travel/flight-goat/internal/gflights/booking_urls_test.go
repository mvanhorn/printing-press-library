// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// PATCH(library): tests for the booking-urls-google-and-airline patch.
// See booking_urls.go for the production code.

package gflights

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func TestBuildGoogleFlightsURLOneWay(t *testing.T) {
	opts := SearchOptions{
		Origin:        "SEA",
		Destination:   "LHR",
		DepartureDate: "2026-06-15",
		Passengers:    1,
	}
	got := buildGoogleFlightsURL(opts)
	if !strings.HasPrefix(got, "https://www.google.com/travel/flights/search?tfs=") {
		t.Fatalf("unexpected prefix: %s", got)
	}
	if !strings.Contains(got, "&curr=USD") || !strings.Contains(got, "&hl=en") {
		t.Errorf("expected &curr=USD&hl=en suffix; got %s", got)
	}
	tripType, slices, pax, class := decodeTfs(t, got)
	// In Google Flights' protobuf schema, ONE_WAY = 2 (ROUND_TRIP = 1).
	if tripType != googleTripTypeOneWay {
		t.Errorf("trip type = %d, want %d (one-way)", tripType, googleTripTypeOneWay)
	}
	if len(slices) != 1 {
		t.Fatalf("slices = %d, want 1", len(slices))
	}
	if slices[0].origin != "SEA" || slices[0].dest != "LHR" || slices[0].date != "2026-06-15" {
		t.Errorf("slice mismatch: %+v", slices[0])
	}
	if pax != 1 {
		t.Errorf("pax (Traveler enum count) = %d, want 1", pax)
	}
	if class != googleClassEconomy {
		t.Errorf("class = %d, want %d (economy)", class, googleClassEconomy)
	}
}

func TestBuildGoogleFlightsURLRoundTripMultiPax(t *testing.T) {
	opts := SearchOptions{
		Origin:        "SEA",
		Destination:   "HND",
		DepartureDate: "2026-12-24",
		ReturnDate:    "2027-01-01",
		Passengers:    4,
	}
	got := buildGoogleFlightsURL(opts)
	tripType, slices, pax, _ := decodeTfs(t, got)
	if tripType != googleTripTypeRoundTrip {
		t.Errorf("trip type = %d, want %d (round-trip)", tripType, googleTripTypeRoundTrip)
	}
	if len(slices) != 2 {
		t.Fatalf("slices = %d, want 2", len(slices))
	}
	out, ret := slices[0], slices[1]
	if out.origin != "SEA" || out.dest != "HND" || out.date != "2026-12-24" {
		t.Errorf("outbound slice mismatch: %+v", out)
	}
	if ret.origin != "HND" || ret.dest != "SEA" || ret.date != "2027-01-01" {
		t.Errorf("return slice mismatch: %+v", ret)
	}
	if pax != 4 {
		t.Errorf("pax (Traveler enum count) = %d, want 4", pax)
	}
}

func TestBuildGoogleFlightsURLEmptyOriginYieldsEmpty(t *testing.T) {
	got := buildGoogleFlightsURL(SearchOptions{Destination: "LAX", DepartureDate: "2026-06-15"})
	if got != "" {
		t.Errorf("expected empty URL for missing origin, got %q", got)
	}
}

func TestBuildGoogleFlightsURLZeroPaxDefaultsToOne(t *testing.T) {
	opts := SearchOptions{Origin: "SEA", Destination: "LAX", DepartureDate: "2026-06-15", Passengers: 0}
	got := buildGoogleFlightsURL(opts)
	_, _, pax, _ := decodeTfs(t, got)
	if pax != 1 {
		t.Errorf("pax = %d, want 1 (default)", pax)
	}
}

func TestBuildAirlineURLSingleCarrierAlaska(t *testing.T) {
	opts := SearchOptions{
		Origin:        "SEA",
		Destination:   "LAX",
		DepartureDate: "2026-06-15",
		ReturnDate:    "2026-06-22",
		Passengers:    2,
	}
	flight := Flight{Legs: []Leg{
		{Airline: Airline{Code: "AS"}},
		{Airline: Airline{Code: "AS"}},
	}}
	got, ok := buildAirlineURL(opts, flight)
	if !ok {
		t.Fatal("expected single-carrier AS to qualify")
	}
	if !strings.Contains(got, "alaskaair.com") {
		t.Errorf("URL not from alaskaair.com: %s", got)
	}
	if !strings.Contains(got, "from=SEA") {
		t.Errorf("origin not in URL: %s", got)
	}
	if !strings.Contains(got, "to=LAX") {
		t.Errorf("destination not in URL: %s", got)
	}
	if !strings.Contains(got, "departureDate=2026-06-15") {
		t.Errorf("departure date not in URL: %s", got)
	}
	if !strings.Contains(got, "returnDate=2026-06-22") {
		t.Errorf("return date not in URL: %s", got)
	}
	if !strings.Contains(got, "adults=2") {
		t.Errorf("pax count not in URL: %s", got)
	}
}

func TestBuildAirlineURLCodeshareRejected(t *testing.T) {
	opts := SearchOptions{Origin: "SEA", Destination: "BKK", DepartureDate: "2026-12-24"}
	flight := Flight{Legs: []Leg{
		{Airline: Airline{Code: "AS"}},
		{Airline: Airline{Code: "TG"}},
	}}
	_, ok := buildAirlineURL(opts, flight)
	if ok {
		t.Error("expected codeshare itinerary to omit airline URL")
	}
}

func TestBuildAirlineURLUnknownCarrier(t *testing.T) {
	opts := SearchOptions{Origin: "SEA", Destination: "PEK", DepartureDate: "2026-09-01"}
	flight := Flight{Legs: []Leg{{Airline: Airline{Code: "HU"}}}}
	_, ok := buildAirlineURL(opts, flight)
	if ok {
		t.Error("expected unknown carrier (HU) to omit airline URL")
	}
}

func TestBuildAirlineURLEmptyAirlineCode(t *testing.T) {
	opts := SearchOptions{Origin: "SEA", Destination: "LAX", DepartureDate: "2026-06-15"}
	flight := Flight{Legs: []Leg{{Airline: Airline{Code: ""}}}}
	_, ok := buildAirlineURL(opts, flight)
	if ok {
		t.Error("expected empty airline code to omit airline URL")
	}
}

func TestBuildAirlineURLNoLegs(t *testing.T) {
	_, ok := buildAirlineURL(SearchOptions{}, Flight{})
	if ok {
		t.Error("expected flight with no legs to omit airline URL")
	}
}

func TestBuildAirlineURLOneWayStripsEmptyReturnParam(t *testing.T) {
	opts := SearchOptions{
		Origin:        "SEA",
		Destination:   "JFK",
		DepartureDate: "2026-07-15",
		Passengers:    1,
	}
	flight := Flight{Legs: []Leg{{Airline: Airline{Code: "B6"}}}}
	got, ok := buildAirlineURL(opts, flight)
	if !ok {
		t.Fatal("expected B6 to qualify")
	}
	if strings.Contains(got, "return=&") || strings.HasSuffix(got, "return=") {
		t.Errorf("URL should not contain empty return= param: %s", got)
	}
	// Other params should still be present and non-empty.
	if !strings.Contains(got, "from=SEA") || !strings.Contains(got, "to=JFK") {
		t.Errorf("expected origin/destination params in: %s", got)
	}
}

func TestBuildAirlineURLOneWayStripsEmptyInboundDateBA(t *testing.T) {
	opts := SearchOptions{
		Origin:        "SEA",
		Destination:   "LHR",
		DepartureDate: "2026-07-15",
		Passengers:    1,
	}
	flight := Flight{Legs: []Leg{{Airline: Airline{Code: "BA"}}}}
	got, ok := buildAirlineURL(opts, flight)
	if !ok {
		t.Fatal("expected BA to qualify")
	}
	if strings.Contains(got, "inboundDate=&") || strings.HasSuffix(got, "inboundDate=") {
		t.Errorf("URL should not contain empty inboundDate= param: %s", got)
	}
}

func TestBuildAirlineURLRoundTripKeepsReturnParam(t *testing.T) {
	opts := SearchOptions{
		Origin:        "SEA",
		Destination:   "JFK",
		DepartureDate: "2026-07-15",
		ReturnDate:    "2026-07-22",
		Passengers:    1,
	}
	flight := Flight{Legs: []Leg{{Airline: Airline{Code: "B6"}}}}
	got, ok := buildAirlineURL(opts, flight)
	if !ok {
		t.Fatal("expected B6 to qualify")
	}
	if !strings.Contains(got, "return=2026-07-22") {
		t.Errorf("round-trip URL should preserve return param: %s", got)
	}
}

func TestBuildAirlineURLModeRoundTripOnlyRejectsOneWay(t *testing.T) {
	saved := airlineTemplates["TESTRT"]
	airlineTemplates["TESTRT"] = airlineTemplate{
		urlTemplate: "https://example.com/?o={origin}&d={destination}&dep={depart}&ret={return}",
		mode:        "roundTripOnly",
	}
	defer func() {
		if saved.urlTemplate == "" {
			delete(airlineTemplates, "TESTRT")
		} else {
			airlineTemplates["TESTRT"] = saved
		}
	}()

	flight := Flight{Legs: []Leg{{Airline: Airline{Code: "TESTRT"}}}}

	_, ok := buildAirlineURL(SearchOptions{Origin: "A", Destination: "B", DepartureDate: "2026-01-01"}, flight)
	if ok {
		t.Error("roundTripOnly template should reject one-way query")
	}
	_, ok = buildAirlineURL(SearchOptions{Origin: "A", Destination: "B", DepartureDate: "2026-01-01", ReturnDate: "2026-01-08"}, flight)
	if !ok {
		t.Error("roundTripOnly template should accept round-trip query")
	}
}

func TestBuildAirlineURLModeOneWayOnlyRejectsRoundTrip(t *testing.T) {
	saved := airlineTemplates["TESTOW"]
	airlineTemplates["TESTOW"] = airlineTemplate{
		urlTemplate: "https://example.com/?o={origin}&d={destination}&dep={depart}",
		mode:        "oneWayOnly",
	}
	defer func() {
		if saved.urlTemplate == "" {
			delete(airlineTemplates, "TESTOW")
		} else {
			airlineTemplates["TESTOW"] = saved
		}
	}()

	flight := Flight{Legs: []Leg{{Airline: Airline{Code: "TESTOW"}}}}

	_, ok := buildAirlineURL(SearchOptions{Origin: "A", Destination: "B", DepartureDate: "2026-01-01", ReturnDate: "2026-01-08"}, flight)
	if ok {
		t.Error("oneWayOnly template should reject round-trip query")
	}
	_, ok = buildAirlineURL(SearchOptions{Origin: "A", Destination: "B", DepartureDate: "2026-01-01"}, flight)
	if !ok {
		t.Error("oneWayOnly template should accept one-way query")
	}
}

func TestStripEmptyQueryParams(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://x.test/?a=1&b=&c=3", "https://x.test/?a=1&c=3"},
		{"https://x.test/?a=&b=&c=", "https://x.test/"},
		{"https://x.test/?a=1", "https://x.test/?a=1"},
		{"https://x.test/", "https://x.test/"},
		{"https://x.test/?a=1&flag&c=3", "https://x.test/?a=1&flag&c=3"}, // bare flag preserved
	}
	for _, tc := range cases {
		got := stripEmptyQueryParams(tc.in)
		if got != tc.want {
			t.Errorf("stripEmptyQueryParams(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBuildBookingURLsAlwaysPopulatesGoogle(t *testing.T) {
	opts := SearchOptions{Origin: "SEA", Destination: "LAX", DepartureDate: "2026-06-15", Passengers: 1}
	flight := Flight{Legs: []Leg{{Airline: Airline{Code: "HU"}}}} // HU not in table
	out := buildBookingURLs(opts, flight)
	if out.Google == "" {
		t.Error("Google URL should always be populated")
	}
	if out.Airline != "" {
		t.Errorf("Airline URL should be empty for unknown carrier; got %q", out.Airline)
	}
}

func TestBuildBookingURLsPopulatesBothWhenQualifying(t *testing.T) {
	opts := SearchOptions{Origin: "SEA", Destination: "LAX", DepartureDate: "2026-06-15", Passengers: 2}
	flight := Flight{Legs: []Leg{{Airline: Airline{Code: "AS"}}}}
	out := buildBookingURLs(opts, flight)
	if out.Google == "" {
		t.Error("Google URL should be populated")
	}
	if out.Airline == "" {
		t.Error("Airline URL should be populated for AS")
	}
}

// --- decoder helpers used by tests ---

type decodedSlice struct {
	origin, dest, date string
}

// decodeTfs decodes the tfs= protobuf using the canonical schema from
// krisukox/google-flights-api url.proto. Outer fields: 3 (Flight, repeated),
// 8 (Traveler enum, repeated — count = passenger count), 9 (Class enum),
// 19 (TripType enum).
func decodeTfs(t *testing.T, urlStr string) (tripType int, slices []decodedSlice, pax int, class int) {
	t.Helper()
	u, err := url.Parse(urlStr)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	tfs := u.Query().Get("tfs")
	if tfs == "" {
		t.Fatal("tfs param missing")
	}
	pb, err := base64.RawURLEncoding.DecodeString(tfs)
	if err != nil {
		t.Fatalf("decode base64: %v", err)
	}
	for len(pb) > 0 {
		field, wireType, tagLen := protowire.ConsumeTag(pb)
		if tagLen < 0 {
			t.Fatalf("consume tag: %d", tagLen)
		}
		pb = pb[tagLen:]
		switch field {
		case 3:
			if wireType != protowire.BytesType {
				t.Fatalf("field 3 wire type = %d, want bytes", wireType)
			}
			data, n := protowire.ConsumeBytes(pb)
			pb = pb[n:]
			slices = append(slices, decodeFlightSlice(t, data))
		case 8:
			if wireType != protowire.VarintType {
				t.Fatalf("field 8 wire type = %d, want varint", wireType)
			}
			_, n := protowire.ConsumeVarint(pb)
			pax++
			pb = pb[n:]
		case 9:
			v, n := protowire.ConsumeVarint(pb)
			class = int(v)
			pb = pb[n:]
		case 19:
			v, n := protowire.ConsumeVarint(pb)
			tripType = int(v)
			pb = pb[n:]
		default:
			n := protowire.ConsumeFieldValue(field, wireType, pb)
			if n < 0 {
				t.Fatalf("consume unknown field: %d", n)
			}
			pb = pb[n:]
		}
	}
	return tripType, slices, pax, class
}

// decodeFlightSlice decodes a Flight message: field 2 (date string),
// field 13 (origin Location, repeated — first only), field 14 (destination
// Location, repeated — first only).
func decodeFlightSlice(t *testing.T, data []byte) decodedSlice {
	t.Helper()
	var s decodedSlice
	for len(data) > 0 {
		field, wireType, tagLen := protowire.ConsumeTag(data)
		data = data[tagLen:]
		switch field {
		case 2:
			str, n := protowire.ConsumeBytes(data)
			data = data[n:]
			s.date = string(str)
		case 13:
			inner, n := protowire.ConsumeBytes(data)
			data = data[n:]
			if s.origin == "" {
				s.origin = decodeLocation(t, inner)
			}
		case 14:
			inner, n := protowire.ConsumeBytes(data)
			data = data[n:]
			if s.dest == "" {
				s.dest = decodeLocation(t, inner)
			}
		default:
			n := protowire.ConsumeFieldValue(field, wireType, data)
			data = data[n:]
		}
	}
	return s
}

// decodeLocation reads the Location sub-message: field 1 (LocationType, skipped),
// field 2 (IATA name string — what we return).
func decodeLocation(t *testing.T, data []byte) string {
	t.Helper()
	for len(data) > 0 {
		field, wireType, tagLen := protowire.ConsumeTag(data)
		data = data[tagLen:]
		if field == 2 && wireType == protowire.BytesType {
			str, _ := protowire.ConsumeBytes(data)
			return string(str)
		}
		n := protowire.ConsumeFieldValue(field, wireType, data)
		data = data[n:]
	}
	return ""
}
