// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// PATCH(library): tests for the booking-urls-google-and-airline patch.
// See booking_urls.go for the production code.

package gflights

import (
	"encoding/base64"
	"encoding/json"
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

func TestBuildAirlineURLSingleCarrierDeltaPrefill(t *testing.T) {
	opts := SearchOptions{
		Origin:        "SEA",
		Destination:   "LAX",
		DepartureDate: "2026-06-15",
		ReturnDate:    "2026-06-22",
		Passengers:    2,
	}
	flight := Flight{Legs: []Leg{
		{Airline: Airline{Code: "DL"}},
		{Airline: Airline{Code: "DL"}},
	}}
	got, kind, ok := buildAirlineURL(opts, flight)
	if !ok {
		t.Fatal("expected single-carrier DL to qualify")
	}
	if kind != airlineKindPrefill {
		t.Errorf("DL kind = %q, want prefill", kind)
	}
	if !strings.Contains(got, "delta.com") {
		t.Errorf("URL not from delta.com: %s", got)
	}
	if !strings.Contains(got, "originCity=SEA") || !strings.Contains(got, "destinationCity=LAX") {
		t.Errorf("expected origin/destination params: %s", got)
	}
	if !strings.Contains(got, "departureDate=2026-06-15") || !strings.Contains(got, "returnDate=2026-06-22") {
		t.Errorf("expected dates: %s", got)
	}
	if !strings.Contains(got, "paxCount=2") {
		t.Errorf("expected paxCount=2: %s", got)
	}
	if !strings.Contains(got, "tripType=ROUND_TRIP") {
		t.Errorf("expected tripType=ROUND_TRIP: %s", got)
	}
}

func TestBuildAirlineURLLandingKindCondor(t *testing.T) {
	// DE (Condor) is the actual operator for SEA-PNH searches and is in
	// the table as landing. Verify it returns a non-empty URL.
	opts := SearchOptions{Origin: "SEA", Destination: "KTI", DepartureDate: "2026-12-24"}
	flight := Flight{Legs: []Leg{{Airline: Airline{Code: "DE"}}}}
	got, kind, ok := buildAirlineURL(opts, flight)
	if !ok {
		t.Fatal("expected DE to qualify")
	}
	if kind != airlineKindLanding {
		t.Errorf("DE kind = %q, want landing", kind)
	}
	if !strings.Contains(got, "condor.com") {
		t.Errorf("expected condor.com in URL: %s", got)
	}
}

func TestBuildAirlineURLCodeshareRejected(t *testing.T) {
	opts := SearchOptions{Origin: "SEA", Destination: "BKK", DepartureDate: "2026-12-24"}
	flight := Flight{Legs: []Leg{
		{Airline: Airline{Code: "AS"}},
		{Airline: Airline{Code: "TG"}},
	}}
	_, _, ok := buildAirlineURL(opts, flight)
	if ok {
		t.Error("expected codeshare itinerary to omit airline URL")
	}
}

func TestBuildAirlineURLUnknownCarrier(t *testing.T) {
	opts := SearchOptions{Origin: "SEA", Destination: "PEK", DepartureDate: "2026-09-01"}
	flight := Flight{Legs: []Leg{{Airline: Airline{Code: "ZZ"}}}}
	_, _, ok := buildAirlineURL(opts, flight)
	if ok {
		t.Error("expected unknown carrier (ZZ) to omit airline URL")
	}
}

func TestBuildAirlineURLEmptyAirlineCode(t *testing.T) {
	opts := SearchOptions{Origin: "SEA", Destination: "LAX", DepartureDate: "2026-06-15"}
	flight := Flight{Legs: []Leg{{Airline: Airline{Code: ""}}}}
	_, _, ok := buildAirlineURL(opts, flight)
	if ok {
		t.Error("expected empty airline code to omit airline URL")
	}
}

func TestBuildAirlineURLNoLegs(t *testing.T) {
	_, _, ok := buildAirlineURL(SearchOptions{}, Flight{})
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
	got, _, ok := buildAirlineURL(opts, flight)
	if !ok {
		t.Fatal("expected B6 to qualify")
	}
	if strings.Contains(got, "return=&") || strings.HasSuffix(got, "return=") {
		t.Errorf("URL should not contain empty return= param: %s", got)
	}
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
	got, _, ok := buildAirlineURL(opts, flight)
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
	got, _, ok := buildAirlineURL(opts, flight)
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
		kind:        airlineKindPrefill,
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

	_, _, ok := buildAirlineURL(SearchOptions{Origin: "A", Destination: "B", DepartureDate: "2026-01-01"}, flight)
	if ok {
		t.Error("roundTripOnly template should reject one-way query")
	}
	_, _, ok = buildAirlineURL(SearchOptions{Origin: "A", Destination: "B", DepartureDate: "2026-01-01", ReturnDate: "2026-01-08"}, flight)
	if !ok {
		t.Error("roundTripOnly template should accept round-trip query")
	}
}

func TestBuildAirlineURLModeOneWayOnlyRejectsRoundTrip(t *testing.T) {
	saved := airlineTemplates["TESTOW"]
	airlineTemplates["TESTOW"] = airlineTemplate{
		urlTemplate: "https://example.com/?o={origin}&d={destination}&dep={depart}",
		kind:        airlineKindPrefill,
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

	_, _, ok := buildAirlineURL(SearchOptions{Origin: "A", Destination: "B", DepartureDate: "2026-01-01", ReturnDate: "2026-01-08"}, flight)
	if ok {
		t.Error("oneWayOnly template should reject round-trip query")
	}
	_, _, ok = buildAirlineURL(SearchOptions{Origin: "A", Destination: "B", DepartureDate: "2026-01-01"}, flight)
	if !ok {
		t.Error("oneWayOnly template should accept one-way query")
	}
}

func TestBuildAirlineURLTableSize(t *testing.T) {
	// Locks in the curated coverage target so accidental deletions get caught.
	if len(airlineTemplates) < 30 {
		t.Errorf("airlineTemplates has %d entries, want at least 30", len(airlineTemplates))
	}
}

func TestBuildAirlineURLEveryEntryHasKind(t *testing.T) {
	for code, tmpl := range airlineTemplates {
		if tmpl.kind != airlineKindPrefill && tmpl.kind != airlineKindLanding {
			t.Errorf("airline %q has invalid kind %q", code, tmpl.kind)
		}
		if tmpl.urlTemplate == "" {
			t.Errorf("airline %q has empty urlTemplate", code)
		}
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

func TestBuildBookingURLsUnknownCarrierFallsBackToGoogle(t *testing.T) {
	opts := SearchOptions{Origin: "SEA", Destination: "LAX", DepartureDate: "2026-06-15", Passengers: 1}
	flight := Flight{Legs: []Leg{{Airline: Airline{Code: "ZZ"}}}} // ZZ not in table
	out := buildBookingURLs(opts, flight)
	if out.GoogleURL == "" {
		t.Error("GoogleURL should always be populated")
	}
	if out.Primary != out.GoogleURL {
		t.Errorf("Primary should equal GoogleURL for unknown carrier; got %q", out.Primary)
	}
	if out.PrimaryKind != primaryKindSearch {
		t.Errorf("PrimaryKind = %q, want %q", out.PrimaryKind, primaryKindSearch)
	}
	if out.AirlineURL != "" || out.AirlineKind != "" {
		t.Errorf("Airline fields should be empty for unknown carrier; got %q / %q", out.AirlineURL, out.AirlineKind)
	}
}

func TestBuildBookingURLsPrefillPicksAirlineAsPrimary(t *testing.T) {
	opts := SearchOptions{Origin: "SEA", Destination: "LAX", DepartureDate: "2026-06-15", Passengers: 2}
	flight := Flight{Legs: []Leg{{Airline: Airline{Code: "DL"}}}}
	out := buildBookingURLs(opts, flight)
	if out.AirlineURL == "" {
		t.Error("AirlineURL should be populated for DL")
	}
	if out.AirlineKind != primaryKindPrefill {
		t.Errorf("AirlineKind = %q, want %q", out.AirlineKind, primaryKindPrefill)
	}
	if out.Primary != out.AirlineURL {
		t.Error("Primary should equal AirlineURL when prefill is available")
	}
	if out.PrimaryKind != primaryKindPrefill {
		t.Errorf("PrimaryKind = %q, want %q", out.PrimaryKind, primaryKindPrefill)
	}
	if out.GoogleURL == "" {
		t.Error("GoogleURL should still be populated as a secondary option")
	}
}

func TestBuildBookingURLsLandingPicksAirlineAsPrimary(t *testing.T) {
	// Condor (DE) is the actual operator for SEA-PNH queries — landing kind.
	opts := SearchOptions{Origin: "SEA", Destination: "KTI", DepartureDate: "2026-12-24", ReturnDate: "2027-01-01", Passengers: 4}
	flight := Flight{Legs: []Leg{{Airline: Airline{Code: "DE"}}}}
	out := buildBookingURLs(opts, flight)
	if out.AirlineURL == "" {
		t.Error("AirlineURL should be populated for DE")
	}
	if out.Primary != out.AirlineURL {
		t.Error("Primary should equal AirlineURL when landing is available")
	}
	if out.PrimaryKind != primaryKindLanding {
		t.Errorf("PrimaryKind = %q, want %q", out.PrimaryKind, primaryKindLanding)
	}
}

func TestBuildBookingURLsJSONOmitsAbsentAirlineFields(t *testing.T) {
	opts := SearchOptions{Origin: "SEA", Destination: "LAX", DepartureDate: "2026-06-15", Passengers: 1}
	flight := Flight{Legs: []Leg{{Airline: Airline{Code: "ZZ"}}}}
	out := buildBookingURLs(opts, flight)
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	if strings.Contains(got, "airline_url") || strings.Contains(got, "airline_kind") {
		t.Errorf("expected airline_url and airline_kind omitted; got %s", got)
	}
	if !strings.Contains(got, "primary") || !strings.Contains(got, "google_url") {
		t.Errorf("expected primary and google_url present; got %s", got)
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
