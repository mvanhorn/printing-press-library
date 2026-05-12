// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

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
	tripType, slices, pax := decodeTfs(t, got)
	if tripType != 1 {
		t.Errorf("trip type = %d, want 1", tripType)
	}
	if len(slices) != 1 {
		t.Fatalf("slices = %d, want 1", len(slices))
	}
	if slices[0].origin != "SEA" || slices[0].dest != "LHR" || slices[0].date != "2026-06-15" {
		t.Errorf("slice mismatch: %+v", slices[0])
	}
	if pax != 1 {
		t.Errorf("pax = %d, want 1", pax)
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
	tripType, slices, pax := decodeTfs(t, got)
	if tripType != 2 {
		t.Errorf("trip type = %d, want 2 (round-trip)", tripType)
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
		t.Errorf("pax = %d, want 4", pax)
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
	_, _, pax := decodeTfs(t, got)
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

func decodeTfs(t *testing.T, urlStr string) (tripType int, slices []decodedSlice, pax int) {
	t.Helper()
	u, err := url.Parse(urlStr)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	tfs := u.Query().Get("tfs")
	if tfs == "" {
		t.Fatal("tfs param missing")
	}
	pb, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(tfs)
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
		case 2:
			if wireType != protowire.VarintType {
				t.Fatalf("field 2 wire type = %d, want varint", wireType)
			}
			v, n := protowire.ConsumeVarint(pb)
			tripType = int(v)
			pb = pb[n:]
		case 3:
			if wireType != protowire.BytesType {
				t.Fatalf("field 3 wire type = %d, want bytes", wireType)
			}
			data, n := protowire.ConsumeBytes(pb)
			pb = pb[n:]
			slices = append(slices, decodeSliceBytes(t, data))
		case 4:
			v, n := protowire.ConsumeVarint(pb)
			pax = int(v)
			pb = pb[n:]
		default:
			// Skip unknown fields.
			n := protowire.ConsumeFieldValue(field, wireType, pb)
			if n < 0 {
				t.Fatalf("consume unknown field: %d", n)
			}
			pb = pb[n:]
		}
	}
	return tripType, slices, pax
}

func decodeSliceBytes(t *testing.T, data []byte) decodedSlice {
	t.Helper()
	var s decodedSlice
	for len(data) > 0 {
		field, wireType, tagLen := protowire.ConsumeTag(data)
		data = data[tagLen:]
		switch field {
		case 2, 6:
			inner, n := protowire.ConsumeBytes(data)
			data = data[n:]
			iata := decodeAirportObject(t, inner)
			if field == 2 {
				s.origin = iata
			} else {
				s.dest = iata
			}
		case 13:
			str, n := protowire.ConsumeBytes(data)
			data = data[n:]
			s.date = string(str)
		default:
			n := protowire.ConsumeFieldValue(field, wireType, data)
			data = data[n:]
		}
	}
	return s
}

func decodeAirportObject(t *testing.T, data []byte) string {
	t.Helper()
	for len(data) > 0 {
		field, wireType, tagLen := protowire.ConsumeTag(data)
		data = data[tagLen:]
		if field == 2 {
			str, _ := protowire.ConsumeBytes(data)
			return string(str)
		}
		n := protowire.ConsumeFieldValue(field, wireType, data)
		data = data[n:]
	}
	return ""
}
