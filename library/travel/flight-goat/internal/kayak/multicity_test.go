// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package kayak

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

// hostRewrite redirects kayak.com requests to a local test server without
// mutating the caller's request (RoundTrippers must not modify the original).
type hostRewrite struct {
	host string
	base http.RoundTripper
}

func (h hostRewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	r2 := req.Clone(req.Context())
	r2.URL.Scheme = "http"
	r2.URL.Host = h.host
	return h.base.RoundTrip(r2)
}

const oneItineraryPoll = `{
	"searchId": "S1",
	"totalCount": 1567,
	"results": [{
		"legs": [{"id":"LEG1"}],
		"bookingOptions": [{
			"displayPrice": {"price":1161,"currency":"USD","localizedPrice":"$1,161"},
			"bookingUrl": {"url":"/book/x","urlType":"relative"}
		}],
		"totalDuration": 600
	}],
	"legs": {"LEG1":{"arrival":"2026-08-15T22:00:00","departure":"2026-08-15T08:00:00","duration":600,"segments":[{"id":"SEG1"}]}},
	"segments": {"SEG1":{"airline":"UA","origin":"SFO","destination":"NRT","departure":"2026-08-15T08:00:00","arrival":"2026-08-15T22:00:00","duration":600,"flightNumber":"UA837","equipmentTypeName":"Boeing 787"}}
}`

// newKayakStub returns a test server that serves the shell HTML (with a
// formToken) on GET and a poll body whose status is set by statusFor(pollNum)
// on POST, plus the client wired to reach it. pollSpacing is zeroed for the
// duration of the test so the exhausted-polls path runs without real sleeps.
func newKayakStub(t *testing.T, statusFor func(poll int64) string) (*MultiCityClient, *httptest.Server, *int64) {
	t.Helper()
	old := pollSpacing
	pollSpacing = 0
	t.Cleanup(func() { pollSpacing = old })

	var polls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			io.WriteString(w, `<html><script>var x = 1; formToken = 'TESTTOKEN'; var y = 2;</script></html>`)
			return
		}
		n := atomic.AddInt64(&polls, 1)
		var body map[string]any
		if err := json.Unmarshal([]byte(oneItineraryPoll), &body); err != nil {
			t.Errorf("stub: bad fixture: %v", err)
		}
		body["status"] = statusFor(n)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)

	c, err := NewMultiCityClient()
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	u, _ := url.Parse(srv.URL)
	c.HTTPClient.Transport = hostRewrite{host: u.Host, base: http.DefaultTransport}
	return c, srv, &polls
}

var multiCityStubOpts = MultiCityOptions{
	Segments: []Segment{
		{Origin: "SFO", Destination: "NRT", DepartureDate: "2026-08-15"},
		{Origin: "NRT", Destination: "ICN", DepartureDate: "2026-08-28"},
		{Origin: "ICN", Destination: "SFO", DepartureDate: "2026-09-05"},
	},
	Passengers: 1,
}

func TestSearchMultiCity_IncompleteFlagsPartialResults(t *testing.T) {
	// Poll never reports "complete" → loop exhausts maxPolls and the result
	// must be flagged incomplete so the caller can warn (the P1 fix).
	c, _, polls := newKayakStub(t, func(int64) string { return "searching" })
	res, err := c.SearchMultiCity(context.Background(), multiCityStubOpts)
	if err != nil {
		t.Fatalf("SearchMultiCity: %v", err)
	}
	if res.Complete {
		t.Error("Complete = true, want false when status never reaches \"complete\"")
	}
	if *polls != maxPolls {
		t.Errorf("polls = %d, want %d (should exhaust the budget)", *polls, maxPolls)
	}
	if res.TotalCount != 1567 || res.Count != 1 {
		t.Errorf("count = %d of %d, want 1 of 1567 (partial set preserved)", res.Count, res.TotalCount)
	}
}

func TestSearchMultiCity_CompleteStopsEarly(t *testing.T) {
	// First poll reports "complete" → loop breaks immediately, Complete=true.
	c, _, polls := newKayakStub(t, func(int64) string { return "complete" })
	res, err := c.SearchMultiCity(context.Background(), multiCityStubOpts)
	if err != nil {
		t.Fatalf("SearchMultiCity: %v", err)
	}
	if !res.Complete {
		t.Error("Complete = false, want true when status reaches \"complete\"")
	}
	if *polls != 1 {
		t.Errorf("polls = %d, want 1 (should stop on first complete)", *polls)
	}
}

func TestBuildShellURL_Shape(t *testing.T) {
	segs := []Segment{
		{Origin: "sfo", Destination: "nrt", DepartureDate: "2026-08-15"},
		{Origin: "NRT", Destination: "ICN", DepartureDate: "2026-08-28"},
		{Origin: "icn", Destination: "sfo", DepartureDate: "2026-09-05"},
	}
	got := buildShellURL(MultiCityOptions{Segments: segs, Passengers: 1})
	want := "https://www.kayak.com/flights/SFO-NRT/2026-08-15/NRT-ICN/2026-08-28/ICN-SFO/2026-09-05?sort=bestflight_a"
	if got != want {
		t.Errorf("URL = %q\nwant     %q", got, want)
	}
}

func TestBuildShellURL_WithCabinPaxNonstop(t *testing.T) {
	got := buildShellURL(MultiCityOptions{
		Segments: []Segment{
			{Origin: "BOS", Destination: "LHR", DepartureDate: "2026-10-05"},
			{Origin: "LHR", Destination: "BOS", DepartureDate: "2026-10-19"},
		},
		Passengers: 2,
		Cabin:      "business",
		Nonstop:    true,
	})
	if !strings.Contains(got, "/business") {
		t.Errorf("URL missing /business: %s", got)
	}
	if !strings.Contains(got, "/2adults") {
		t.Errorf("URL missing /2adults: %s", got)
	}
	if !strings.Contains(got, "/nonstop") {
		t.Errorf("URL missing /nonstop: %s", got)
	}
}

func TestBuildPollBody_LegsAndPaxShape(t *testing.T) {
	raw := buildPollBody(MultiCityOptions{
		Segments: []Segment{
			{Origin: "SFO", Destination: "NRT", DepartureDate: "2026-08-15"},
			{Origin: "NRT", Destination: "ICN", DepartureDate: "2026-08-28"},
		},
		Passengers: 2,
	}, "")
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	user, ok := got["userSearchParams"].(map[string]any)
	if !ok {
		t.Fatal("userSearchParams missing")
	}
	legs, _ := user["legs"].([]any)
	if len(legs) != 2 {
		t.Errorf("legs len = %d, want 2", len(legs))
	}
	pax, _ := user["passengers"].([]any)
	if len(pax) != 2 || pax[0] != "ADT" {
		t.Errorf("passengers = %v, want [ADT, ADT]", pax)
	}
	if user["sortMode"] != "bestflight_a" {
		t.Errorf("sortMode = %v, want bestflight_a", user["sortMode"])
	}
}

func TestBuildPollBody_IncludesSearchIDWhenSet(t *testing.T) {
	raw := buildPollBody(MultiCityOptions{
		Segments:   []Segment{{Origin: "A", Destination: "B", DepartureDate: "2026-01-01"}, {Origin: "B", Destination: "A", DepartureDate: "2026-01-08"}},
		Passengers: 1,
	}, "myID123")
	var got map[string]any
	_ = json.Unmarshal(raw, &got)
	user := got["userSearchParams"].(map[string]any)
	if user["searchId"] != "myID123" {
		t.Errorf("searchId = %v, want myID123", user["searchId"])
	}
}

func TestResolveKayakBookingURL_RelativeGetsOrigin(t *testing.T) {
	got := resolveKayakBookingURL("/book/flight?code=abc", "relative")
	if got != "https://www.kayak.com/book/flight?code=abc" {
		t.Errorf("got %q", got)
	}
}

func TestResolveKayakBookingURL_AbsolutePassesThrough(t *testing.T) {
	got := resolveKayakBookingURL("https://other.com/book", "absolute")
	if got != "https://other.com/book" {
		t.Errorf("got %q", got)
	}
}

func TestParseItineraries_HydratesFromLookupMaps(t *testing.T) {
	// Synthesize a minimal poll-response shape with lookup maps.
	legLookup := map[string]pollLegRef{
		"LEG1": {
			Arrival: "2026-08-15T22:00:00", Departure: "2026-08-15T08:00:00", Duration: 600,
			Segments: []struct {
				ID string `json:"id"`
			}{{ID: "SEG1"}},
		},
	}
	segLookup := map[string]pollSegment{
		"SEG1": {
			Airline: "UA", Origin: "SFO", Destination: "NRT",
			Departure: "2026-08-15T08:00:00", Arrival: "2026-08-15T22:00:00",
			Duration: 600, FlightNumber: "UA837", Equipment: "Boeing 787",
		},
	}
	result := json.RawMessage(`{
		"legs": [{"id":"LEG1"}],
		"bookingOptions": [{
			"displayPrice": {"price": 1239, "currency": "USD", "localizedPrice": "$1,239"},
			"bookingUrl": {"url": "/book/abc", "urlType": "relative"}
		}],
		"totalDuration": 600
	}`)
	got := parseItineraries([]json.RawMessage{result}, legLookup, segLookup)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	f := got[0]
	if f.Price != 1239 || f.Currency != "USD" {
		t.Errorf("price = %v %s, want 1239 USD", f.Price, f.Currency)
	}
	if !strings.HasPrefix(f.BookingURL, "https://www.kayak.com/") {
		t.Errorf("booking URL not resolved: %s", f.BookingURL)
	}
	if len(f.Legs) != 1 {
		t.Fatalf("legs = %d, want 1", len(f.Legs))
	}
	if f.Legs[0].Origin != "SFO" || f.Legs[0].Destination != "NRT" {
		t.Errorf("leg endpoints = %s->%s", f.Legs[0].Origin, f.Legs[0].Destination)
	}
	if len(f.Legs[0].Segments) != 1 || f.Legs[0].Segments[0].Carrier != "UA" {
		t.Errorf("segment hydration failed: %+v", f.Legs[0].Segments)
	}
}

func TestParseItineraries_SkipsResultsWithMissingLegs(t *testing.T) {
	// Result references a leg ID that isn't in the lookup map → skip.
	result := json.RawMessage(`{
		"legs": [{"id":"NONEXISTENT"}],
		"bookingOptions": [{"displayPrice":{"price":100,"currency":"USD"}}]
	}`)
	got := parseItineraries([]json.RawMessage{result}, map[string]pollLegRef{}, map[string]pollSegment{})
	if len(got) != 0 {
		t.Errorf("expected 0 results when leg lookup misses, got %d", len(got))
	}
}
