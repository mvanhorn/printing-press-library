// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.

package agoda

import (
	"encoding/json"
	"testing"
)

// TestBuildCitySearchRewritesBothCurrencyFields guards the single most
// dangerous field in this client. Agoda accepts a request that sets only
// searchCriteria.currency and silently returns geo-default pricing, so a
// regression here produces plausible-looking prices in the wrong currency.
func TestBuildCitySearchRewritesBothCurrencyFields(t *testing.T) {
	body, err := buildCitySearch(SearchOptions{
		CityID: 5085, CheckIn: "2026-10-15", Nights: 2, Rooms: 1,
		Adults: 2, Currency: "USD", Locale: "en-us", Origin: "US",
	})
	if err != nil {
		t.Fatalf("buildCitySearch() error = %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("built payload is not valid JSON: %v", err)
	}
	vars := payload["variables"].(map[string]any)

	sc := vars["CitySearchRequest"].(map[string]any)["searchRequest"].(map[string]any)["searchCriteria"].(map[string]any)
	if got := sc["currency"]; got != "USD" {
		t.Errorf("searchCriteria.currency = %v, want USD", got)
	}
	pricing := vars["PricingSummaryRequest"].(map[string]any)["pricing"].(map[string]any)
	if got := pricing["currency"]; got != "USD" {
		t.Errorf("PricingSummaryRequest.pricing.currency = %v, want USD "+
			"(setting only searchCriteria.currency silently yields geo-default pricing)", got)
	}
}

func TestBuildCitySearchAppliesOccupancyAndDates(t *testing.T) {
	body, err := buildCitySearch(SearchOptions{
		CityID: 9395, CheckIn: "2026-12-24", Nights: 3, Rooms: 2,
		Adults: 4, Children: 1, ChildAges: []int{7},
		Currency: "SGD", Locale: "en-us", Origin: "US",
	})
	if err != nil {
		t.Fatalf("buildCitySearch() error = %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	csr := payload["variables"].(map[string]any)["CitySearchRequest"].(map[string]any)
	if got := csr["cityId"]; got != float64(9395) {
		t.Errorf("cityId = %v, want 9395", got)
	}
	sc := csr["searchRequest"].(map[string]any)["searchCriteria"].(map[string]any)
	for _, tc := range []struct {
		key  string
		want float64
	}{{"los", 3}, {"rooms", 2}, {"adults", 4}, {"children", 1}} {
		if got := sc[tc.key]; got != tc.want {
			t.Errorf("searchCriteria.%s = %v, want %v", tc.key, got, tc.want)
		}
	}
	if got := sc["localCheckInDate"]; got != "2026-12-24" {
		t.Errorf("localCheckInDate = %v, want 2026-12-24", got)
	}
}

// TestBuildCitySearchKeepsNonNullableIdentifiers pins the fix for a real
// failure: sending null for searchId made Agoda reject the whole operation
// with an opaque variable-type error.
func TestBuildCitySearchKeepsNonNullableIdentifiers(t *testing.T) {
	body, err := buildCitySearch(SearchOptions{
		CityID: 5085, CheckIn: "2026-10-15", Nights: 1, Rooms: 1,
		Adults: 2, Currency: "USD", Locale: "en-us", Origin: "US",
	})
	if err != nil {
		t.Fatalf("buildCitySearch() error = %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	vars := payload["variables"].(map[string]any)
	ctx := vars["CitySearchRequest"].(map[string]any)["searchRequest"].(map[string]any)["searchContext"].(map[string]any)
	if s, ok := ctx["searchId"].(string); !ok || s == "" {
		t.Errorf("searchContext.searchId = %v, want a non-empty string (null fails Agoda variable validation)", ctx["searchId"])
	}
	ci := vars["PricingSummaryRequest"].(map[string]any)["context"].(map[string]any)["clientInfo"].(map[string]any)
	if s, ok := ci["searchId"].(string); !ok || s == "" {
		t.Errorf("clientInfo.searchId = %v, want a non-empty string", ci["searchId"])
	}
}

func TestBuildCitySearchRejectsBadDate(t *testing.T) {
	if _, err := buildCitySearch(SearchOptions{CityID: 1, CheckIn: "15-10-2026", Nights: 1}); err == nil {
		t.Fatal("buildCitySearch() with a non-ISO date should error")
	}
}

// TestParseCitySearchComputesHiddenMarkup covers the headline behavior: both
// price bases are read and the gap between them is derived correctly.
func TestParseCitySearchComputesHiddenMarkup(t *testing.T) {
	raw := []byte(`{"data":{"citySearch":{"properties":[
      {"propertyId":42,
       "content":{"informationSummary":{"displayName":"Test Hotel","rating":4}},
       "pricing":{"offers":[{"roomOffers":[{"room":{"pricing":[{"price":{
          "perBook":{"exclusive":{"display":100},"inclusive":{"display":125}}
       }}]}}]}]}}
    ]}}}`)
	props, err := parseCitySearch(raw, SearchOptions{Nights: 2, Currency: "USD", CheckIn: "2026-10-15", Adults: 2, Rooms: 1})
	if err != nil {
		t.Fatalf("parseCitySearch() error = %v", err)
	}
	if len(props) != 1 {
		t.Fatalf("got %d properties, want 1", len(props))
	}
	p := props[0]
	if p.PriceAdvertised != 100 || p.PriceAllIn != 125 {
		t.Errorf("prices = (%v, %v), want (100, 125)", p.PriceAdvertised, p.PriceAllIn)
	}
	if p.HiddenAmount != 25 {
		t.Errorf("HiddenAmount = %v, want 25", p.HiddenAmount)
	}
	if p.HiddenPct != 25 {
		t.Errorf("HiddenPct = %v, want 25", p.HiddenPct)
	}
	if p.PerNightAllIn != 62.5 {
		t.Errorf("PerNightAllIn = %v, want 62.5 (125 over 2 nights)", p.PerNightAllIn)
	}
	if p.Name != "Test Hotel" {
		t.Errorf("Name = %q, want %q", p.Name, "Test Hotel")
	}
}

// TestParseCitySearchSurvivesMissingPricing ensures a sold-out property becomes
// a zero-priced row rather than dropping the whole response.
// TestParseCitySearchReadsCancellationFromPaymentNode pins a real bug: the
// extraction originally read pricing.isEasyCancel, which is a different flag and
// is false even for stays that genuinely are free-cancellation. Every property
// therefore reported free_cancellation=false. The truth lives at
// pricing.payment.cancellation.
func TestParseCitySearchReadsCancellationFromPaymentNode(t *testing.T) {
	raw := []byte(`{"data":{"citySearch":{"properties":[
      {"propertyId":1,
       "content":{"informationSummary":{"displayName":"Refundable Inn"}},
       "pricing":{
         "isEasyCancel":false,
         "payment":{"cancellation":{"cancellationType":"FreeCancellation","freeCancellationDate":"2026-10-11T00:00:00.000+07:00"}},
         "offers":[{"roomOffers":[{"room":{"pricing":[{"price":{
            "perBook":{"exclusive":{"display":100},"inclusive":{"display":120}}
         }}]}}]}]}}
    ]}}}`)
	props, err := parseCitySearch(raw, SearchOptions{Nights: 1})
	if err != nil {
		t.Fatalf("parseCitySearch() error = %v", err)
	}
	p := props[0]
	if !p.FreeCancellation {
		t.Error("FreeCancellation = false, want true (isEasyCancel must not be the source)")
	}
	if p.FreeCancellationUntil != "2026-10-11" {
		t.Errorf("FreeCancellationUntil = %q, want 2026-10-11", p.FreeCancellationUntil)
	}
}

// TestParseCitySearchNonRefundableStaysFalse is the negative half: a property
// without a FreeCancellation type must not be reported as refundable.
func TestParseCitySearchNonRefundableStaysFalse(t *testing.T) {
	raw := []byte(`{"data":{"citySearch":{"properties":[
      {"propertyId":2,
       "content":{"informationSummary":{"displayName":"Nonrefundable Inn"}},
       "pricing":{"payment":{"cancellation":{"cancellationType":"NonRefundable"}},
         "offers":[{"roomOffers":[{"room":{"pricing":[{"price":{
            "perBook":{"exclusive":{"display":80},"inclusive":{"display":90}}
         }}]}}]}]}}
    ]}}}`)
	props, err := parseCitySearch(raw, SearchOptions{Nights: 1})
	if err != nil {
		t.Fatalf("parseCitySearch() error = %v", err)
	}
	if props[0].FreeCancellation {
		t.Error("FreeCancellation = true for a NonRefundable stay, want false")
	}
	if props[0].FreeCancellationUntil != "" {
		t.Errorf("FreeCancellationUntil = %q, want empty for a NonRefundable stay", props[0].FreeCancellationUntil)
	}
}

func TestParseCitySearchSurvivesMissingPricing(t *testing.T) {
	raw := []byte(`{"data":{"citySearch":{"properties":[
      {"propertyId":7,"soldOut":true,
       "content":{"informationSummary":{"displayName":"Sold Out Inn"}}}
    ]}}}`)
	props, err := parseCitySearch(raw, SearchOptions{Nights: 1})
	if err != nil {
		t.Fatalf("parseCitySearch() error = %v", err)
	}
	if len(props) != 1 {
		t.Fatalf("got %d properties, want 1", len(props))
	}
	if !props[0].SoldOut {
		t.Error("SoldOut = false, want true")
	}
	if props[0].PriceAllIn != 0 {
		t.Errorf("PriceAllIn = %v, want 0 for a property with no pricing node", props[0].PriceAllIn)
	}
}

func TestParseCitySearchEmptyIsNotAnError(t *testing.T) {
	props, err := parseCitySearch([]byte(`{"data":{"citySearch":{"properties":[]}}}`), SearchOptions{Nights: 1})
	if err != nil {
		t.Fatalf("parseCitySearch() error = %v", err)
	}
	if len(props) != 0 {
		t.Fatalf("got %d properties, want 0", len(props))
	}
	if props == nil {
		t.Error("properties slice is nil; want an empty slice so JSON renders [] not null")
	}
}

// TestNumberAtAcceptsStringEncodedNumbers guards the silent-zero bug class:
// Agoda encodes some numerics as JSON strings, which unmarshal to 0 in a
// float64 field without error.
func TestNumberAtAcceptsStringEncodedNumbers(t *testing.T) {
	cases := []struct {
		name   string
		node   map[string]any
		want   float64
		wantOK bool
	}{
		{"json number", map[string]any{"v": 12.5}, 12.5, true},
		{"string number", map[string]any{"v": "12.5"}, 12.5, true},
		{"missing", map[string]any{}, 0, false},
		{"null", map[string]any{"v": nil}, 0, false},
		{"unparseable", map[string]any{"v": "abc"}, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := numberAt(tc.node, "v")
			if ok != tc.wantOK || got != tc.want {
				t.Errorf("numberAt() = (%v, %v), want (%v, %v)", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

// TestBookingURLPreservesStayParameters pins the deep-link contract.
//
// The original implementation built /partners/agoda.html?hotelId=..., which
// 404s to pagenotfound.html for every property. The working shape is
// /partners/partnersearch.aspx?hid=..., verified to return HTTP 200 with the
// stay parameters surviving the redirect. Asserting the parameter name here
// keeps a rename from silently reintroducing dead links.
func TestBookingURLPreservesStayParameters(t *testing.T) {
	got := bookingURL(123, SearchOptions{
		CheckIn: "2026-10-15", Nights: 2, Adults: 2, Rooms: 1, Currency: "USD",
	})
	for _, want := range []string{
		"partnersearch.aspx",
		"hid=123",
		"checkIn=2026-10-15",
		"los=2",
		"adults=2",
		"currencyCode=USD",
		"finalPriceView=1",
	} {
		if !contains(got, want) {
			t.Errorf("bookingURL() = %q, missing %q", got, want)
		}
	}
	if contains(got, "agoda.html") {
		t.Errorf("bookingURL() = %q, still uses the agoda.html path which 404s", got)
	}
	if bookingURL(0, SearchOptions{}) != "" {
		t.Error("bookingURL(0) should return an empty string")
	}
}

func TestNormalizeDateHandlesAgodaOffsetFormat(t *testing.T) {
	cases := map[string]string{
		"2026-09-01T07:00:00.000+07:00": "2026-09-01",
		"2026-09-01T00:00:00.000Z":      "2026-09-01",
		"2026-09-01":                    "2026-09-01",
	}
	for in, want := range cases {
		if got := normalizeDate(in); got != want {
			t.Errorf("normalizeDate(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNewRequestIDIsUniqueAndWellFormed(t *testing.T) {
	a, b := newRequestID(), newRequestID()
	if a == b {
		t.Error("newRequestID() returned duplicate ids")
	}
	if len(a) != 36 {
		t.Errorf("newRequestID() = %q, want 36 characters", a)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}

// TestBuildCitySearchWritesLocalPricingDates pins the most damaging bug found in
// this build. buildCitySearch originally set only pricing.checkIn/checkout,
// leaving pricing.localCheckInDate/localCheckoutDate pinned to the captured
// template's dates. Agoda keys pricing off the local pair, so every search
// silently returned prices for the template's date no matter what the caller
// asked for - and every mechanical gate still passed, because the response was
// well-formed and internally consistent.
//
// Asserting that a varied input actually reaches the request is the check that
// would have caught it.
func TestBuildCitySearchWritesLocalPricingDates(t *testing.T) {
	cases := []struct {
		checkIn      string
		nights       int
		wantCheckIn  string
		wantCheckout string
	}{
		{"2026-10-15", 2, "2026-10-15", "2026-10-17"},
		{"2026-11-20", 3, "2026-11-20", "2026-11-23"},
		{"2026-12-24", 1, "2026-12-24", "2026-12-25"},
	}
	for _, tc := range cases {
		t.Run(tc.checkIn, func(t *testing.T) {
			body, err := buildCitySearch(SearchOptions{
				CityID: 5085, CheckIn: tc.checkIn, Nights: tc.nights, Rooms: 1,
				Adults: 2, Currency: "USD", Locale: "en-us", Origin: "US",
			})
			if err != nil {
				t.Fatalf("buildCitySearch() error = %v", err)
			}
			var payload map[string]any
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			pricing := payload["variables"].(map[string]any)["PricingSummaryRequest"].(map[string]any)["pricing"].(map[string]any)
			if got := pricing["localCheckInDate"]; got != tc.wantCheckIn {
				t.Errorf("pricing.localCheckInDate = %v, want %v (pricing is keyed off this field)", got, tc.wantCheckIn)
			}
			if got := pricing["localCheckoutDate"]; got != tc.wantCheckout {
				t.Errorf("pricing.localCheckoutDate = %v, want %v", got, tc.wantCheckout)
			}
		})
	}
}

// TestBuildCitySearchDistinctDatesProduceDistinctRequests is the blunt version
// of the same guard: two different check-in dates must not serialize to the
// same request body.
func TestBuildCitySearchDistinctDatesProduceDistinctRequests(t *testing.T) {
	base := SearchOptions{CityID: 5085, Nights: 2, Rooms: 1, Adults: 2, Currency: "USD", Locale: "en-us", Origin: "US"}
	a := base
	a.CheckIn = "2026-10-15"
	b := base
	b.CheckIn = "2027-03-01"
	ab, err := buildCitySearch(a)
	if err != nil {
		t.Fatalf("buildCitySearch(a) error = %v", err)
	}
	bb, err := buildCitySearch(b)
	if err != nil {
		t.Fatalf("buildCitySearch(b) error = %v", err)
	}
	// Strip the per-call random ids and timestamp so only the dates differ.
	norm := func(raw []byte) string {
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		v := m["variables"].(map[string]any)
		sr := v["CitySearchRequest"].(map[string]any)["searchRequest"].(map[string]any)
		sr["searchCriteria"].(map[string]any)["bookingDate"] = ""
		sr["searchContext"].(map[string]any)["searchId"] = ""
		psr := v["PricingSummaryRequest"].(map[string]any)
		psr["context"].(map[string]any)["clientInfo"].(map[string]any)["searchId"] = ""
		psr["pricing"].(map[string]any)["bookingDate"] = ""
		out, _ := json.Marshal(m)
		return string(out)
	}
	if norm(ab) == norm(bb) {
		t.Fatal("two different check-in dates serialized to an identical request; the date is not reaching the payload")
	}
}

// TestFindPriceNodeIsDeterministic pins the fix for a P1 review finding.
//
// findPriceNode originally recursed over a map[string]any and returned the
// first node carrying perBook. Go randomizes map iteration order, so a property
// with several offers could yield a different price on each parse of the very
// same response - silently changing rankings and cheapest-property results
// between runs. Parsing the identical payload many times must be stable.
func TestFindPriceNodeIsDeterministic(t *testing.T) {
	raw := []byte(`{"data":{"citySearch":{"properties":[
      {"propertyId":1,
       "content":{"informationSummary":{"displayName":"Multi Offer Inn"}},
       "pricing":{
         "offerA":{"roomOffers":[{"room":{"pricing":[{"price":{
             "perBook":{"exclusive":{"display":900},"inclusive":{"display":1000}}}}]}}]},
         "offerB":{"roomOffers":[{"room":{"pricing":[{"price":{
             "perBook":{"exclusive":{"display":700},"inclusive":{"display":800}}}}]}}]},
         "offerC":{"roomOffers":[{"room":{"pricing":[{"price":{
             "perBook":{"exclusive":{"display":1100},"inclusive":{"display":1200}}}}]}}]}
       }}
    ]}}}`)
	var first float64
	for i := 0; i < 50; i++ {
		props, err := parseCitySearch(raw, SearchOptions{Nights: 1})
		if err != nil {
			t.Fatalf("parseCitySearch() error = %v", err)
		}
		got := props[0].PriceAllIn
		if i == 0 {
			first = got
			continue
		}
		if got != first {
			t.Fatalf("parse %d returned %v, first parse returned %v; offer selection is not deterministic", i, got, first)
		}
	}
	// Determinism alone is not enough: a search result should report the
	// cheapest bookable offer, not an arbitrary one.
	if first != 800 {
		t.Errorf("PriceAllIn = %v, want 800 (the cheapest of the three offers)", first)
	}
}

// TestFindPriceNodePrefersInclusiveOverCheaperExclusiveOnly pins a P1 review
// finding introduced by the determinism fix.
//
// nodeAllInPrice originally fell back to the exclusive (advertised) figure when
// a node had no inclusive price. Because selection picks the minimum, an
// exclusive-only offer at 700 would beat a genuine all-in offer at 800 - even
// though 700 is a pre-tax number and 800 is the real cost. Worse, the winning
// node carried no inclusive price, so the true all-in figure vanished from the
// output entirely. Ranking must compare like with like.
func TestFindPriceNodePrefersInclusiveOverCheaperExclusiveOnly(t *testing.T) {
	raw := []byte(`{"data":{"citySearch":{"properties":[
      {"propertyId":1,
       "content":{"informationSummary":{"displayName":"Mixed Basis Inn"}},
       "pricing":{
         "offerExclusiveOnly":{"roomOffers":[{"room":{"pricing":[{"price":{
             "perBook":{"exclusive":{"display":700}}}}]}}]},
         "offerWithInclusive":{"roomOffers":[{"room":{"pricing":[{"price":{
             "perBook":{"exclusive":{"display":750},"inclusive":{"display":800}}}}]}}]}
       }}
    ]}}}`)
	props, err := parseCitySearch(raw, SearchOptions{Nights: 1})
	if err != nil {
		t.Fatalf("parseCitySearch() error = %v", err)
	}
	p := props[0]
	if p.PriceAllIn != 800 {
		t.Errorf("PriceAllIn = %v, want 800; a cheaper exclusive-only offer must not win on a different price basis", p.PriceAllIn)
	}
	if p.PriceAdvertised != 750 {
		t.Errorf("PriceAdvertised = %v, want 750 (the advertised figure from the selected offer)", p.PriceAdvertised)
	}
}

// TestNodeAllInPriceRejectsExclusiveOnly is the unit-level half of the same guard.
func TestNodeAllInPriceRejectsExclusiveOnly(t *testing.T) {
	exclusiveOnly := map[string]any{"perBook": map[string]any{"exclusive": map[string]any{"display": 250.0}}}
	if _, ok := nodeAllInPrice(exclusiveOnly); ok {
		t.Error("nodeAllInPrice() accepted an exclusive-only node; ranking would then mix price bases")
	}
	withInclusive := map[string]any{"perBook": map[string]any{
		"exclusive": map[string]any{"display": 250.0},
		"inclusive": map[string]any{"display": 300.0},
	}}
	got, ok := nodeAllInPrice(withInclusive)
	if !ok || got != 300 {
		t.Errorf("nodeAllInPrice() = (%v, %v), want (300, true)", got, ok)
	}
	if _, ok := nodeAllInPrice(map[string]any{}); ok {
		t.Error("nodeAllInPrice() on a node without perBook should report not-ok")
	}
}
