// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package soar

import (
	"strings"
	"testing"
)

func TestParseSSEOffers(t *testing.T) {
	stream := strings.Join([]string{
		"event: created",
		`data: {"search_id":"abc"}`,
		"",
		"event: offer",
		`data: {"id":"off_1","total_amount":"352.40","total_currency":"USD","owner":{"iata_code":"AA","name":"American Airlines"},"slices":[{"segments":[{"carrier_iata":"AA","carrier_name":"American Airlines","flight_number":"475","departure":"2026-07-15T08:00:00","arrival":"2026-07-15T16:30:00","cabin_class":"Economy","duration":"PT5H30M","origin":{"iata_code":"SFO"},"destination":{"iata_code":"JFK"}}]}]}`,
		"",
		"event: offer",
		`data: {invalid json`,
		"",
		"event: done",
		"data: {}",
		"",
	}, "\n")

	offers, err := parseSSEOffers(strings.NewReader(stream))
	if err != nil {
		t.Fatalf("parseSSEOffers: %v", err)
	}
	if len(offers) != 1 {
		t.Fatalf("want 1 valid offer (bad JSON skipped), got %d", len(offers))
	}
	if offers[0].ID != "off_1" || offers[0].TotalAmount != "352.40" {
		t.Fatalf("unexpected offer: %+v", offers[0])
	}
}

func TestParseSSEOffersStopsOnDone(t *testing.T) {
	stream := strings.Join([]string{
		"event: offer",
		`data: {"id":"off_1","total_amount":"100","slices":[{"segments":[{"carrier_iata":"DL","flight_number":"1","departure":"d","arrival":"a"}]}]}`,
		"",
		"event: done",
		"",
		"event: offer",
		`data: {"id":"off_after_done","total_amount":"50","slices":[{"segments":[{}]}]}`,
		"",
	}, "\n")

	offers, err := parseSSEOffers(strings.NewReader(stream))
	if err != nil {
		t.Fatalf("parseSSEOffers: %v", err)
	}
	if len(offers) != 1 || offers[0].ID != "off_1" {
		t.Fatalf("should stop at done event, got %d offers: %+v", len(offers), offers)
	}
}

func TestNormalizeOfferOneWay(t *testing.T) {
	raw := rawOffer{
		ID:          "off_1",
		TotalAmount: "352.40",
		Currency:    "usd",
		Owner:       rawOwner{IATACode: "AA", Name: "American Airlines"},
		Slices: []rawSlice{{Segments: []rawSegment{
			{CarrierIATA: "AA", CarrierName: "American Airlines", FlightNumber: "0475",
				Departure: "2026-07-15T08:00:00", Arrival: "2026-07-15T14:30:00",
				CabinClass: "Economy", Duration: "PT6H30M",
				Origin: rawPlace{IATACode: "SFO"}, Destination: rawPlace{IATACode: "ORD"}},
			{CarrierIATA: "AA", FlightNumber: "88",
				Departure: "2026-07-15T16:00:00", Arrival: "2026-07-15T19:00:00",
				Duration: "PT3H0M",
				Origin: rawPlace{IATACode: "ORD"}, Destination: rawPlace{IATACode: "JFK"}},
		}}},
	}
	f, ok := normalizeOffer(raw, "USD", 1)
	if !ok {
		t.Fatal("expected offer to normalize")
	}
	if f.Price != 352.40 || f.Currency != "USD" {
		t.Fatalf("price/currency wrong: %+v", f)
	}
	if f.SliceCount != 1 {
		t.Fatalf("slice count want 1, got %d", f.SliceCount)
	}
	if f.Stops != 1 {
		t.Fatalf("stops want 1 (2 segments, 1 slice), got %d", f.Stops)
	}
	if f.DurationMinutes != 6*60+30+3*60 {
		t.Fatalf("duration sum wrong: %d", f.DurationMinutes)
	}
	if len(f.Legs) != 2 {
		t.Fatalf("want 2 legs, got %d", len(f.Legs))
	}
	if f.Legs[0].FlightNumber != "AA475" {
		t.Fatalf("flight number normalization: want AA475, got %q", f.Legs[0].FlightNumber)
	}
	if f.Legs[0].DepartureAirport.Code != "SFO" || f.Legs[1].ArrivalAirport.Code != "JFK" {
		t.Fatalf("airport codes wrong: %+v %+v", f.Legs[0].DepartureAirport, f.Legs[1].ArrivalAirport)
	}
	if f.Owner != "American Airlines" {
		t.Fatalf("owner want American Airlines, got %q", f.Owner)
	}
}

func TestNormalizeOfferRoundTripStops(t *testing.T) {
	// Two slices, one nonstop segment each → 0 stops.
	raw := rawOffer{
		ID:          "off_rt",
		TotalAmount: "900",
		Currency:    "USD",
		Slices: []rawSlice{
			{Segments: []rawSegment{{CarrierIATA: "DL", FlightNumber: "1", Departure: "2026-07-15T08:00:00", Arrival: "2026-07-15T16:00:00"}}},
			{Segments: []rawSegment{{CarrierIATA: "DL", FlightNumber: "2", Departure: "2026-07-22T09:00:00", Arrival: "2026-07-22T12:00:00"}}},
		},
	}
	f, ok := normalizeOffer(raw, "USD", 1)
	if !ok {
		t.Fatal("expected normalize")
	}
	if f.SliceCount != 2 {
		t.Fatalf("slice count want 2, got %d", f.SliceCount)
	}
	if f.Stops != 0 {
		t.Fatalf("round-trip nonstop each way want 0 stops, got %d", f.Stops)
	}
}

func TestNormalizeOfferDropsUnpriced(t *testing.T) {
	for _, amt := range []string{"", "0", "not-a-number"} {
		raw := rawOffer{TotalAmount: amt, Slices: []rawSlice{{Segments: []rawSegment{{CarrierIATA: "AA"}}}}}
		if _, ok := normalizeOffer(raw, "USD", 1); ok {
			t.Fatalf("expected unpriced offer (amount %q) to be dropped", amt)
		}
	}
}

func TestNormalizeOfferDividesToPerSeat(t *testing.T) {
	// FlySoar quotes the party total; Flight.Price must be per-seat.
	raw := rawOffer{
		ID: "off_party", TotalAmount: "189.96", Currency: "USD",
		Slices: []rawSlice{{Segments: []rawSegment{{CarrierIATA: "AS", FlightNumber: "1", Departure: "d", Arrival: "a"}}}},
	}
	if f, _ := normalizeOffer(raw, "USD", 1); f.Price != 189.96 {
		t.Fatalf("1 pax: want 189.96, got %v", f.Price)
	}
	if f, _ := normalizeOffer(raw, "USD", 2); f.Price != 94.98 {
		t.Fatalf("2 pax: want per-seat 94.98, got %v", f.Price)
	}
	// 3-way split rounds to cents (100/3 = 33.333... -> 33.33).
	raw3 := rawOffer{
		ID: "off3", TotalAmount: "100", Currency: "USD",
		Slices: []rawSlice{{Segments: []rawSegment{{CarrierIATA: "AS", FlightNumber: "1", Departure: "d", Arrival: "a"}}}},
	}
	if f, _ := normalizeOffer(raw3, "USD", 3); f.Price != 33.33 {
		t.Fatalf("3 pax: want 33.33, got %v", f.Price)
	}
}

func TestNormalizeOffersDedupesCheapest(t *testing.T) {
	mk := func(id, amt string) rawOffer {
		return rawOffer{ID: id, TotalAmount: amt, Currency: "USD", Slices: []rawSlice{{Segments: []rawSegment{
			{CarrierIATA: "UA", FlightNumber: "100", Departure: "2026-07-15T08:00:00", Arrival: "2026-07-15T16:00:00"},
		}}}}
	}
	got := normalizeOffers([]rawOffer{mk("a", "500"), mk("b", "420"), mk("c", "480")}, "USD", 1)
	if len(got) != 1 {
		t.Fatalf("identical itineraries should collapse to 1, got %d", len(got))
	}
	if got[0].Price != 420 {
		t.Fatalf("dedupe should keep cheapest (420), got %v", got[0].Price)
	}
}

func TestParseISODurationMinutes(t *testing.T) {
	cases := map[string]int{
		"PT5H30M": 330,
		"PT45M":   45,
		"PT2H":    120,
		"":        0,
		"garbage": 0,
		"PT0H0M":  0,
	}
	for in, want := range cases {
		if got := parseISODurationMinutes(in); got != want {
			t.Errorf("parseISODurationMinutes(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestNormalizeFlightNumber(t *testing.T) {
	cases := []struct{ carrier, num, want string }{
		{"AA", "0475", "AA475"},
		{"AA", "AA475", "AA475"},
		{"dl", "88", "DL88"},
		{"", "123", "123"},
		{"UA", "", ""},
		{"BA", "007", "BA7"},
	}
	for _, c := range cases {
		if got := normalizeFlightNumber(c.carrier, c.num); got != c.want {
			t.Errorf("normalizeFlightNumber(%q,%q) = %q, want %q", c.carrier, c.num, got, c.want)
		}
	}
}

func TestSearchURL(t *testing.T) {
	got := SearchURL("SEA", "DEN", "2026-09-21", "", "first", nil, nil)
	want := "https://flysoar.ai/flights/sea/den/260921/?cabin=first&trip=oneway"
	if got != want {
		t.Fatalf("SearchURL one-way:\n got %q\nwant %q", got, want)
	}
	rt := SearchURL("JFK", "LHR", "2026-07-15", "2026-07-22", "business", nil, nil)
	if !strings.Contains(rt, "/flights/jfk/lhr/260715/260722/?") || !strings.Contains(rt, "trip=roundtrip") || !strings.Contains(rt, "cabin=business") {
		t.Fatalf("SearchURL round-trip wrong: %q", rt)
	}
}

func TestSearchURLStopsAirlines(t *testing.T) {
	// Mirrors the FlySoar GUI URLs: commas URL-encoded as %2C.
	u := SearchURL("DCA", "IAH", "2026-09-23", "", "first", []int{0, 1}, []string{"DL", "UA"})
	for _, want := range []string{"/flights/dca/iah/260923/?", "cabin=first", "trip=oneway", "stops=0%2C1", "airlines=DL%2CUA"} {
		if !strings.Contains(u, want) {
			t.Fatalf("SearchURL missing %q in %q", want, u)
		}
	}
	// Single airline, single stop count.
	u2 := SearchURL("DCA", "IAH", "2026-09-23", "", "first", []int{0}, []string{"UA"})
	if !strings.Contains(u2, "stops=0") || !strings.Contains(u2, "airlines=UA") {
		t.Fatalf("single-value stops/airlines wrong: %q", u2)
	}
	// No filters → no stops/airlines params.
	u3 := SearchURL("SEA", "DEN", "2026-09-21", "", "economy", nil, nil)
	if strings.Contains(u3, "stops=") || strings.Contains(u3, "airlines=") {
		t.Fatalf("expected no stops/airlines params: %q", u3)
	}
}

func TestNormalizeStops(t *testing.T) {
	cases := []struct {
		in   []int
		want []int
	}{
		{nil, nil},
		{[]int{}, nil},
		{[]int{1, 2}, []int{1, 2}},
		{[]int{2, 0, 1}, []int{0, 1, 2}},   // sorted
		{[]int{1, 1, 0}, []int{0, 1}},      // de-duped
		{[]int{-1, 0, -3, 2}, []int{0, 2}}, // negatives dropped
		{[]int{-1}, nil},
	}
	for _, c := range cases {
		got := normalizeStops(c.in)
		if len(got) != len(c.want) {
			t.Fatalf("normalizeStops(%v) = %v, want %v", c.in, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("normalizeStops(%v) = %v, want %v", c.in, got, c.want)
			}
		}
	}
}

func TestNormalizeAirlines(t *testing.T) {
	got := normalizeAirlines([]string{"dl", " ua ", "DL", ""})
	want := []string{"DL", "UA"} // upper, trimmed, de-duped, order preserved
	if len(got) != len(want) {
		t.Fatalf("normalizeAirlines = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("normalizeAirlines = %v, want %v", got, want)
		}
	}
	if normalizeAirlines(nil) != nil {
		t.Fatal("normalizeAirlines(nil) should be nil")
	}
}

func TestFilterFlights(t *testing.T) {
	// One-way helper: a single slice, so SliceStops = [stops].
	mk := func(stops int, carriers ...string) Flight {
		legs := make([]Leg, len(carriers))
		for i, c := range carriers {
			legs[i] = Leg{Airline: Airline{Code: c}}
		}
		return Flight{Stops: stops, SliceStops: []int{stops}, Legs: legs}
	}
	flights := []Flight{
		mk(0, "UA"),       // nonstop UA
		mk(1, "UA", "UA"), // 1-stop UA
		mk(1, "AA", "DL"), // 1-stop AA+DL
		mk(2, "DL", "DL", "DL"),
	}
	// stops {0,1}, airlines {UA}: only the nonstop UA and 1-stop UA.
	got := filterFlights(flights, []int{0, 1}, []string{"UA"})
	if len(got) != 2 {
		t.Fatalf("stops+airlines filter: want 2, got %d", len(got))
	}
	// stops {1} only.
	if got := filterFlights(flights, []int{1}, nil); len(got) != 2 {
		t.Fatalf("stops-only filter: want 2, got %d", len(got))
	}
	// airlines {DL} only: itineraries whose every leg is DL → the 2-stop one.
	if got := filterFlights(flights, nil, []string{"DL"}); len(got) != 1 {
		t.Fatalf("airlines-only filter: want 1, got %d", len(got))
	}
	// no filters → unchanged.
	if got := filterFlights(flights, nil, nil); len(got) != len(flights) {
		t.Fatalf("no filter should pass all, got %d", len(got))
	}
}

func TestFilterFlightsRoundTripPerSlice(t *testing.T) {
	// Round trip, one connection each way: total Stops == 2 but each direction
	// is a single stop. FlySoar's stops=1 (per direction) should keep it.
	rt := Flight{Stops: 2, SliceStops: []int{1, 1}, Legs: []Leg{
		{Airline: Airline{Code: "UA"}}, {Airline: Airline{Code: "UA"}},
		{Airline: Airline{Code: "UA"}}, {Airline: Airline{Code: "UA"}},
	}}
	// nonstop-out, one-stop-back mixed itinerary.
	mixed := Flight{Stops: 1, SliceStops: []int{0, 1}}
	flights := []Flight{rt, mixed}

	if got := filterFlights(flights, []int{1}, nil); len(got) != 1 || got[0].Stops != 2 {
		t.Fatalf("stops=1 per-slice should keep the 1-each-way round trip, got %d", len(got))
	}
	// stops=0 (nonstop both ways) keeps neither.
	if got := filterFlights(flights, []int{0}, nil); len(got) != 0 {
		t.Fatalf("stops=0 should drop both, got %d", len(got))
	}
	// stops={0,1} keeps both (each slice is 0 or 1).
	if got := filterFlights(flights, []int{0, 1}, nil); len(got) != 2 {
		t.Fatalf("stops=0,1 should keep both, got %d", len(got))
	}
}

func TestBookingRequest(t *testing.T) {
	cases := []struct {
		q    SearchQuery
		want string
	}{
		{SearchQuery{Origin: "DEN", Destination: "SEA", DepartureDate: "2026-09-28", CabinClass: "first", Passengers: 1},
			"DEN -> SEA on 2026-09-28, first class"},
		{SearchQuery{Origin: "JFK", Destination: "LHR", DepartureDate: "2026-07-15", ReturnDate: "2026-07-22", CabinClass: "business", Passengers: 2},
			"JFK -> LHR on 2026-07-15, returning 2026-07-22, business class, 2 passengers"},
		{SearchQuery{Origin: "SEA", Destination: "DEN", DepartureDate: "2026-09-21", CabinClass: "economy", Passengers: 1},
			"SEA -> DEN on 2026-09-21"},
		{SearchQuery{Origin: "DCA", Destination: "IAH", DepartureDate: "2026-09-23", CabinClass: "first", Passengers: 1, Stops: []int{0}, Airlines: []string{"UA"}},
			"DCA -> IAH on 2026-09-23, first class, nonstop, on UA"},
		{SearchQuery{Origin: "DCA", Destination: "IAH", DepartureDate: "2026-09-23", CabinClass: "first", Passengers: 2, Stops: []int{0, 1}, Airlines: []string{"DL", "UA"}},
			"DCA -> IAH on 2026-09-23, first class, nonstop or 1 stop, on DL/UA, 2 passengers"},
	}
	for _, c := range cases {
		if got := BookingRequest(c.q); got != c.want {
			t.Errorf("BookingRequest(%+v)\n got %q\nwant %q", c.q, got, c.want)
		}
	}
}

func TestIMessageURL(t *testing.T) {
	got := IMessageURL("+14156299322", "DEN -> SEA on 2026-09-28, first class")
	want := "sms:+14156299322?body=DEN+-%3E+SEA+on+2026-09-28%2C+first+class"
	if got != want {
		t.Fatalf("IMessageURL:\n got %q\nwant %q", got, want)
	}
}

func TestBuildBookingHandoff(t *testing.T) {
	q := SearchQuery{Origin: "DEN", Destination: "SEA", DepartureDate: "2026-09-28", CabinClass: "first", Passengers: 1}
	h := buildBookingHandoff(q, "https://flysoar.ai/flights/den/sea/260928/?cabin=first&trip=oneway")
	if h.Method != "handoff" {
		t.Errorf("method = %q, want handoff", h.Method)
	}
	if h.IMessageAgent == "" || !strings.HasPrefix(h.IMessageURL, "sms:") {
		t.Errorf("imessage handoff not populated: %+v", h)
	}
	if !strings.Contains(h.WebURL, "flysoar.ai") {
		t.Errorf("web url wrong: %q", h.WebURL)
	}
	if !strings.Contains(h.Request, "DEN -> SEA") {
		t.Errorf("request seed wrong: %q", h.Request)
	}
}

func TestPlaceUnmarshalStringOrObject(t *testing.T) {
	var p1 rawPlace
	if err := p1.UnmarshalJSON([]byte(`"SEA"`)); err != nil || p1.IATACode != "SEA" {
		t.Fatalf("string form: %+v err=%v", p1, err)
	}
	var p2 rawPlace
	if err := p2.UnmarshalJSON([]byte(`{"iata_code":"DEN","name":"Denver Intl"}`)); err != nil || p2.IATACode != "DEN" || p2.Name != "Denver Intl" {
		t.Fatalf("object form: %+v err=%v", p2, err)
	}
	var p3 rawPlace
	if err := p3.UnmarshalJSON([]byte(`null`)); err != nil || p3.IATACode != "" {
		t.Fatalf("null form: %+v err=%v", p3, err)
	}
}
