// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH(amend-2026-06-11): regression tests for the gated-RPC detection and
// the server-rendered HTML fallback. testdata/error_response_blocked.json is
// a live capture (2026-06-11, trace tokens redacted) of the ErrorResponse
// envelope Google now returns to non-interactive RPC clients;
// testdata/aus_lax_embedded_ds1.json is the live AF_initDataCallback ds:1
// payload from the server-rendered AUS->LAX search page the same day.

package gflights

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return raw
}

// The blocked envelope must surface as errShoppingBlocked from the offers
// parser — never as a silent empty result (the bug users hit: success with
// count 0 on routes that obviously have flights).
func TestParseOffersResponseDetectsBlockedEnvelope(t *testing.T) {
	body := loadFixture(t, "error_response_blocked.json")
	flights, err := parseOffersResponse(body, "USD")
	if !errors.Is(err, errShoppingBlocked) {
		t.Fatalf("parseOffersResponse error = %v, want errShoppingBlocked", err)
	}
	if flights != nil {
		t.Fatalf("parseOffersResponse returned %d flights alongside the blocked error", len(flights))
	}
}

// Same detection on the dates parser (previously died with the bare
// "response wrb.fr payload is not a string" error).
func TestParseDatesResponseDetectsBlockedEnvelope(t *testing.T) {
	body := loadFixture(t, "error_response_blocked.json")
	_, err := parseDatesResponse(body, "USD", DatesOptions{})
	if !errors.Is(err, errShoppingBlocked) {
		t.Fatalf("parseDatesResponse error = %v, want errShoppingBlocked", err)
	}
}

func TestParseDatesResponseEmptyStringIsNotBlockedEnvelope(t *testing.T) {
	body := []byte(googleResponsePrefix + ` [["wrb.fr","opaque",""]]`)
	_, err := parseDatesResponse(body, "USD", DatesOptions{})
	if err == nil {
		t.Fatal("expected parseDatesResponse to reject an empty inner payload")
	}
	if errors.Is(err, errShoppingBlocked) {
		t.Fatalf("empty string payload misclassified as blocked envelope: %v", err)
	}
}

// A non-string payload that is NOT the known ErrorResponse shape must still
// error loudly (format drift should never look like an empty result), but
// must not be classified as the gated-RPC condition.
func TestEnvelopeBlockedErrUnrecognizedShape(t *testing.T) {
	err := envelopeBlockedErr(`[["wrb.fr",null,null]]`)
	if errors.Is(err, errShoppingBlocked) {
		t.Fatalf("unrecognized envelope misclassified as blocked: %v", err)
	}
	if err == nil {
		t.Fatal("expected a non-nil error for unrecognized envelope")
	}
}

// Google now sometimes returns a compact gated-RPC envelope with only the
// status-code metadata slot (`[13]`) and no ErrorResponse type URL. It should
// trigger the same HTML fallback as the older verbose ErrorResponse envelope.
func TestParseOffersResponseDetectsCompactBlockedEnvelope(t *testing.T) {
	body := []byte(googleResponsePrefix + `
[["wrb.fr",null,null,null,null,[13]],["di",39],["af.httprm",38,"redacted",6]]`)
	flights, err := parseOffersResponse(body, "USD")
	if !errors.Is(err, errShoppingBlocked) {
		t.Fatalf("parseOffersResponse error = %v, want errShoppingBlocked", err)
	}
	if flights != nil {
		t.Fatalf("parseOffersResponse returned %d flights alongside the blocked error", len(flights))
	}
}

func TestParseDatesResponseDetectsCompactBlockedEnvelope(t *testing.T) {
	body := []byte(googleResponsePrefix + `
[["wrb.fr",null,null,null,null,[13]],["di",39],["af.httprm",38,"redacted",6]]`)
	_, err := parseDatesResponse(body, "USD", DatesOptions{})
	if !errors.Is(err, errShoppingBlocked) {
		t.Fatalf("parseDatesResponse error = %v, want errShoppingBlocked", err)
	}
}

// The existing old-format fixtures must keep parsing — the blocked-envelope
// detection must not regress the happy path.
func TestParseOffersResponseOldFormatStillParses(t *testing.T) {
	for _, name := range []string{"sea_kti_2026-12-24_response.json", "sea_bkk_2026-12-24_response.json"} {
		body := loadFixture(t, name)
		flights, err := parseOffersResponse(body, "USD")
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", name, err)
		}
		if len(flights) == 0 {
			t.Fatalf("%s: parsed 0 flights from known-good fixture", name)
		}
	}
}

// wrapDs1HTML embeds the captured ds:1 payload in a minimal page skeleton
// shaped like the live search page (several callbacks, only one carrying
// flights, brackets inside string literals).
func wrapDs1HTML(payload []byte) string {
	return `<!doctype html><html><body>
<script>AF_initDataCallback({key: 'ds:0', hash: '1', data:[null,"decoy ] with bracket",[]], sideChannel: {}});</script>
<script>AF_initDataCallback({key: 'ds:1', hash: '2', data:` + string(payload) + `, sideChannel: {}});</script>
<script>AF_initDataCallback({key: 'ds:2', hash: '3', data:[1,2,3], sideChannel: {}});</script>
</body></html>`
}

func TestFlightsFromHTMLParsesEmbeddedPayload(t *testing.T) {
	html := wrapDs1HTML(loadFixture(t, "aus_lax_embedded_ds1.json"))

	blobs := extractInitDataBlobs(html)
	if len(blobs) != 3 {
		t.Fatalf("extractInitDataBlobs found %d blobs, want 3", len(blobs))
	}

	flights, _ := flightsFromHTML(html, "USD")
	if len(flights) == 0 {
		t.Fatal("flightsFromHTML parsed 0 flights from live-captured page payload")
	}
	// Live values captured 2026-06-11: 13 itineraries; cheapest nonstops at
	// $134 (WN and DL). Assert the structural invariants, not the exact
	// market prices, so a future fixture refresh doesn't need test edits.
	for i, f := range flights {
		if f.Price <= 0 {
			t.Fatalf("flight[%d] has non-positive price %.2f", i, f.Price)
		}
		if len(f.Legs) == 0 {
			t.Fatalf("flight[%d] has no legs", i)
		}
		for j, leg := range f.Legs {
			if leg.DepartureAirport.Code == "" || leg.ArrivalAirport.Code == "" {
				t.Fatalf("flight[%d] leg[%d] missing airport codes", i, j)
			}
			if leg.Airline.Code == "" {
				t.Fatalf("flight[%d] leg[%d] missing airline code", i, j)
			}
			if !strings.HasPrefix(leg.DepartureTime, "2026-") {
				t.Fatalf("flight[%d] leg[%d] departure time %q not parsed", i, j, leg.DepartureTime)
			}
		}
		if f.Legs[0].DepartureAirport.Code != "AUS" {
			t.Fatalf("flight[%d] originates at %s, want AUS", i, f.Legs[0].DepartureAirport.Code)
		}
	}
	// A busy nonstop market should always yield multiple itineraries; any
	// stricter count would couple the test to one specific capture.
	if got := len(flights); got < 2 {
		t.Fatalf("parsed %d flights from the captured page payload, want at least 2", got)
	}
}

func TestFlightsFromHTMLParsesStandaloneDS1ScriptPayload(t *testing.T) {
	html := `<!doctype html><html><body>
<script class="ds:1">window._unused = {data:` + string(loadFixture(t, "aus_lax_embedded_ds1.json")) + `, sideChannel:{}};</script>
</body></html>`

	flights, _ := flightsFromHTML(html, "USD")
	if len(flights) == 0 {
		t.Fatal("flightsFromHTML parsed 0 flights from standalone script.ds:1 payload")
	}
	if pageMissingFlightData(html) {
		t.Fatal("script.ds:1 payload misclassified as missing flight data")
	}
}

func TestFlightsFromHTMLSkipsUnclosedScriptBeforeDS1Payload(t *testing.T) {
	html := `<!doctype html><html><body>
<script class="decoy">var unfinished = true;
<script class="ds:1">window._unused = {data:` + string(loadFixture(t, "aus_lax_embedded_ds1.json")) + `, sideChannel:{}};</script>
</body></html>`

	flights, _ := flightsFromHTML(html, "USD")
	if len(flights) == 0 {
		t.Fatal("flightsFromHTML parsed 0 flights after unclosed decoy script")
	}
}

func TestSortFlightsClientSide(t *testing.T) {
	mk := func(price float64, duration int, dep, arr string) Flight {
		return Flight{Price: price, DurationMinutes: duration, Legs: []Leg{{
			DepartureTime: dep,
			ArrivalTime:   arr,
		}}}
	}
	base := []Flight{
		mk(300, 180, "2026-07-15T16:15:00", "2026-07-15T19:15:00"),
		mk(100, 240, "2026-07-15T20:43:00", "2026-07-16T00:43:00"),
		mk(200, 120, "2026-07-15T06:10:00", "2026-07-15T08:10:00"),
	}
	cases := []struct {
		sortBy    string
		honored   bool
		wantFirst float64 // price of the expected first flight
	}{
		{"", true, 100},
		{"cheapest", true, 100},
		{"Cheapest", true, 100},
		{"duration", true, 200},
		{"departure_time", true, 200},
		{"arrival_time", true, 200},
		{"best", false, 300},
		{"top_flights", false, 300},
		{"emissions", false, 300},
	}
	for _, c := range cases {
		flights := append([]Flight(nil), base...)
		if got := sortFlightsClientSide(flights, c.sortBy); got != c.honored {
			t.Fatalf("sortFlightsClientSide(%q) honored = %v, want %v", c.sortBy, got, c.honored)
		}
		if flights[0].Price != c.wantFirst {
			t.Fatalf("sort %q: first flight price = %.0f, want %.0f", c.sortBy, flights[0].Price, c.wantFirst)
		}
	}
}

// A page with no embedded payload at all (consent interstitial, redesign)
// must be distinguishable from a legitimately empty result set.
func TestPageMissingFlightData(t *testing.T) {
	if !pageMissingFlightData(`<html><body>redirecting to https://consent.google.com/m?continue=...</body></html>`) {
		t.Fatal("consent interstitial not detected")
	}
	if !pageMissingFlightData(`<html><body>no callbacks here</body></html>`) {
		t.Fatal("payload-free page not detected")
	}
	if pageMissingFlightData(wrapDs1HTML([]byte(`[null,[],[]]`))) {
		t.Fatal("page with embedded callbacks misclassified as missing data")
	}
}

func TestPageErrorStatusIsMissingFlightData(t *testing.T) {
	html := `<html><body><script class="ds:1">AF_initDataCallback({key:'ds:1', data:[null,[],[]], errorHasStatus: true});</script></body></html>`
	if !pageMissingFlightData(html) {
		t.Fatal("embedded ds:1 errorHasStatus page not detected")
	}
}

func TestDatesViaHTMLDisclosesPartialDayFailures(t *testing.T) {
	origFetch := fetchSearchPage
	defer func() { fetchSearchPage = origFetch }()

	html := wrapDs1HTML(loadFixture(t, "aus_lax_embedded_ds1.json"))
	var calls atomic.Int64
	fetchSearchPage = func(context.Context, string) (string, error) {
		if calls.Add(1) == 1 {
			return "", fmt.Errorf("temporary page fetch failure")
		}
		return html, nil
	}

	from, err := time.Parse("2006-01-02", "2026-07-13")
	if err != nil {
		t.Fatal(err)
	}
	to, err := time.Parse("2006-01-02", "2026-07-15")
	if err != nil {
		t.Fatal(err)
	}

	rows, note, err := datesViaHTML(context.Background(), DatesOptions{
		Origin:      "AUS",
		Destination: "LAX",
	}, from, to, "USD")
	if err != nil {
		t.Fatalf("datesViaHTML returned error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("datesViaHTML returned %d rows, want 2 successful days", len(rows))
	}
	if !strings.Contains(note, "1 day(s) in range could not be fetched") {
		t.Fatalf("datesViaHTML note did not disclose partial failure: %q", note)
	}
}

func TestScanBalancedArrayRespectsStrings(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{`[1,2,3] trailing`, `[1,2,3]`, true},
		{`[1,"a ] b",[2]]x`, `[1,"a ] b",[2]]`, true},
		{`[1,"esc \" ]",2]y`, `[1,"esc \" ]",2]`, true},
		{`[1,2`, ``, false},
	}
	for _, c := range cases {
		end, ok := scanBalancedArray(c.in)
		if ok != c.ok {
			t.Fatalf("scanBalancedArray(%q) ok = %v, want %v", c.in, ok, c.ok)
		}
		if ok && c.in[:end] != c.want {
			t.Fatalf("scanBalancedArray(%q) = %q, want %q", c.in, c.in[:end], c.want)
		}
	}
}

func TestGoogleSearchPageURLEncoding(t *testing.T) {
	got, err := googleSearchPageURL(SearchOptions{
		Origin:        "AUS",
		Destination:   "LAX",
		DepartureDate: "2026-07-15",
		Passengers:    2,
		MaxStops:      "non_stop",
		CabinClass:    "business",
	}, "EUR")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("curr") != "EUR" {
		t.Fatalf("curr = %q, want EUR", q.Get("curr"))
	}
	if q.Get("tfs") == "" {
		t.Fatal("tfs param missing")
	}
	if !strings.HasPrefix(got, googleFlightsSearchBase) {
		t.Fatalf("URL %q not rooted at %q", got, googleFlightsSearchBase)
	}

	// Errors must propagate from the filter mappers.
	if _, err := googleSearchPageURL(SearchOptions{
		Origin: "AUS", Destination: "LAX", DepartureDate: "2026-07-15",
		CabinClass: "bogus",
	}, "USD"); err == nil {
		t.Fatal("expected error for bogus cabin class")
	}
}

func TestFilterFlightsClientSide(t *testing.T) {
	mk := func(airline, dep string) Flight {
		return Flight{Price: 100, Legs: []Leg{{
			Airline:       Airline{Code: airline},
			DepartureTime: dep,
		}}}
	}
	flights := []Flight{
		mk("WN", "2026-07-15T06:10:00"),
		mk("DL", "2026-07-15T16:15:00"),
		mk("AA", "2026-07-15T20:43:00"),
	}

	byAirline := filterFlightsClientSide(append([]Flight(nil), flights...), SearchOptions{Airlines: []string{"dl"}})
	if len(byAirline) != 1 || byAirline[0].Legs[0].Airline.Code != "DL" {
		t.Fatalf("airline filter returned %+v", byAirline)
	}

	byTime := filterFlightsClientSide(append([]Flight(nil), flights...), SearchOptions{TimeWindow: "6-17"})
	if len(byTime) != 2 {
		t.Fatalf("time filter returned %d flights, want 2", len(byTime))
	}

	passthrough := filterFlightsClientSide(append([]Flight(nil), flights...), SearchOptions{})
	if len(passthrough) != 3 {
		t.Fatalf("no-filter passthrough returned %d flights, want 3", len(passthrough))
	}
}

// PATCH(review-2026-07-31): a 429 across the whole per-day fallback fan-out
// must surface as the typed rate-limit error, not a generic all-days failure.
func TestDatesViaHTMLAllRateLimitedReturnsErrRateLimited(t *testing.T) {
	origFetch := fetchSearchPage
	defer func() { fetchSearchPage = origFetch }()
	fetchSearchPage = func(context.Context, string) (string, error) {
		return "", fmt.Errorf("fallback search page: %w", ErrRateLimited)
	}
	from, _ := time.Parse("2006-01-02", "2026-09-14")
	to, _ := time.Parse("2006-01-02", "2026-09-17")
	_, _, err := datesViaHTML(context.Background(), DatesOptions{Origin: "SEA", Destination: "DEN"}, from, to, "EUR")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v, want ErrRateLimited", err)
	}
}

// TestFlightsFromEmbeddedPayload_OutboundReturnSplit verifies that the
// outbound bucket (inner[2]) and return bucket (inner[3]) are counted
// separately via outboundCount, which searchViaHTML relies on to apply
// ReturnTimeWindow only to the return leg.
func TestFlightsFromEmbeddedPayload_OutboundReturnSplit(t *testing.T) {
	mkHead := func(duration float64) []any {
		head := make([]any, 10)
		head[2] = []any{} // no legs — irrelevant to this test
		head[9] = duration
		return head
	}
	mkRow := func(duration float64) []any {
		return []any{mkHead(duration)}
	}

	inner := make([]any, 4)
	inner[2] = []any{[]any{mkRow(100), mkRow(200)}} // 2 outbound offers
	inner[3] = []any{[]any{mkRow(300)}}             // 1 return offer

	flights, outboundCount := flightsFromEmbeddedPayload(inner, "USD")
	if len(flights) != 3 {
		t.Fatalf("got %d flights, want 3", len(flights))
	}
	if outboundCount != 2 {
		t.Fatalf("outboundCount = %d, want 2", outboundCount)
	}
	if flights[0].DurationMinutes != 100 || flights[1].DurationMinutes != 200 {
		t.Errorf("outbound flights out of order: %+v", flights[:2])
	}
	if flights[2].DurationMinutes != 300 {
		t.Errorf("return flight wrong: %+v", flights[2])
	}
}

// TestFilterFlightsClientSide_ReturnTimeWindowAppliesOnlyToReturnBucket
// exercises the searchViaHTML split: outbound flights are filtered against
// TimeWindow, return flights against ReturnTimeWindow, independently.
func TestFilterFlightsClientSide_ReturnTimeWindowAppliesOnlyToReturnBucket(t *testing.T) {
	mk := func(dep string) Flight {
		return Flight{Price: 100, Legs: []Leg{{DepartureTime: dep}}}
	}
	outbound := []Flight{mk("2026-07-15T06:10:00"), mk("2026-07-15T20:00:00")}
	inbound := []Flight{mk("2026-07-20T18:00:00"), mk("2026-07-20T06:00:00")}

	outboundOpts := SearchOptions{TimeWindow: "6-12"}
	returnOpts := SearchOptions{TimeWindow: "17-23"} // simulates ReturnTimeWindow substitution

	filteredOutbound := filterFlightsClientSide(append([]Flight(nil), outbound...), outboundOpts)
	filteredInbound := filterFlightsClientSide(append([]Flight(nil), inbound...), returnOpts)

	if len(filteredOutbound) != 1 || filteredOutbound[0].Legs[0].DepartureTime != "2026-07-15T06:10:00" {
		t.Fatalf("outbound filter = %+v, want only the 06:10 departure", filteredOutbound)
	}
	if len(filteredInbound) != 1 || filteredInbound[0].Legs[0].DepartureTime != "2026-07-20T18:00:00" {
		t.Fatalf("inbound filter = %+v, want only the 18:00 departure", filteredInbound)
	}
}

// TestCheapestFallbackCandidateRoundTripIgnoresReturnBucket guards against
// picking a cheap "return" bucket entry (an incremental fare priced against
// whatever outbound the page pre-selected, not a standalone round-trip
// total) over a pricier but real round-trip total from the outbound bucket.
func TestCheapestFallbackCandidateRoundTripIgnoresReturnBucket(t *testing.T) {
	flights := []Flight{
		{Price: 400, Direction: "outbound"},
		{Price: 350, Direction: "outbound"},
		{Price: 40, Direction: "return"}, // incremental fare, not a real total
	}
	got := cheapestFallbackCandidate(flights, true)
	if got == nil || got.Price != 350 {
		t.Fatalf("cheapestFallbackCandidate(roundTrip) = %+v, want outbound Price=350", got)
	}
}

// One-way fallback pages have no Direction tagging (untagged/"") — every
// flight is eligible.
func TestCheapestFallbackCandidateOneWayConsidersAllFlights(t *testing.T) {
	flights := []Flight{{Price: 120}, {Price: 90}, {Price: 200}}
	got := cheapestFallbackCandidate(flights, false)
	if got == nil || got.Price != 90 {
		t.Fatalf("cheapestFallbackCandidate(oneWay) = %+v, want Price=90", got)
	}
}

func TestCheapestFallbackCandidateSkipsNonPositivePrices(t *testing.T) {
	flights := []Flight{{Price: 0}, {Price: -5}}
	if got := cheapestFallbackCandidate(flights, false); got != nil {
		t.Fatalf("cheapestFallbackCandidate = %+v, want nil (no positive-priced flights)", got)
	}
}

// TestDatesViaHTMLRoundTripSetsReturnDate exercises the full per-day
// fallback path: ReturnDate must be passed into the round-trip search page
// request (day + Duration) and echoed back on the resulting DatePrice.
func TestDatesViaHTMLRoundTripSetsReturnDate(t *testing.T) {
	origFetch := fetchSearchPage
	defer func() { fetchSearchPage = origFetch }()

	var gotReturnDates []string
	var mu sync.Mutex
	html := wrapDs1HTML(loadFixture(t, "aus_lax_embedded_ds1.json"))
	fetchSearchPage = func(_ context.Context, pageURL string) (string, error) {
		u, err := url.Parse(pageURL)
		if err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		gotReturnDates = append(gotReturnDates, u.Query().Get("tfs"))
		mu.Unlock()
		return html, nil
	}

	from, _ := time.Parse("2006-01-02", "2026-07-13")
	to, _ := time.Parse("2006-01-02", "2026-07-13")
	rows, _, err := datesViaHTML(context.Background(), DatesOptions{
		Origin:      "AUS",
		Destination: "LAX",
		RoundTrip:   true,
		Duration:    3,
	}, from, to, "USD")
	if err != nil {
		t.Fatalf("datesViaHTML returned error: %v", err)
	}
	if len(gotReturnDates) != 1 || gotReturnDates[0] == "" {
		t.Fatalf("expected one request with a non-empty tfs (round-trip) param, got %v", gotReturnDates)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].ReturnDate != "2026-07-16" {
		t.Fatalf("ReturnDate = %q, want 2026-07-16 (departure + 3 days)", rows[0].ReturnDate)
	}
}
