package traveloka

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"
)

func futureHotelQuery() Query {
	return Query{Kind: "hotel", Market: "SG", Locale: "en-SG", Currency: "SGD", PropertyID: "9000000001714", PropertyName: "The Berkeley Hotel Pratunam", CheckIn: "2099-01-06", CheckOut: "2099-01-08", Adults: 3, Children: 2, ChildAges: []int{8, 5}, Rooms: 2}
}
func futureFlightQuery() Query {
	return Query{Kind: "flight", Market: "SG", Locale: "en-SG", Currency: "SGD", Origin: "SIN", Destination: "CGK", Depart: "2099-11-20", Cabin: "ECONOMY", Adults: 1}
}
func TestSimulatedValidateShopper(t *testing.T) {
	for _, tt := range []struct {
		s    Shopper
		fail bool
	}{{Shopper{"SG", "en-SG", "SGD"}, false}, {Shopper{"US", "en-US", "USD"}, false}, {Shopper{}, true}, {Shopper{"SG", "en_SG", "SGD"}, true}, {Shopper{"sg", "en-SG", "SGD"}, true}, {Shopper{"SG", "en-SG", "sgd"}, true}} {
		if err := ValidateShopper(tt.s); (err != nil) != tt.fail {
			t.Fatalf("shopper validation: %v", err)
		}
	}
}
func TestSimulatedQueryContext(t *testing.T) {
	q := futureHotelQuery()
	other := q
	other.Limit, other.Offset, other.MaxCandidates = 3, 8, 50
	if !q.SameContext(other) || q.ContextKey() != other.ContextKey() {
		t.Fatal("output caps must not change comparison context")
	}
	for _, change := range []func(*Query){func(q *Query) { q.Currency = "USD" }, func(q *Query) { q.Rooms++ }, func(q *Query) { q.Adults++ }, func(q *Query) { q.Infants++ }, func(q *Query) { q.CheckOut = "2099-01-09" }, func(q *Query) { q.Market = "US" }, func(q *Query) { q.Locale = "en-US" }, func(q *Query) { q.PropertyName = "other" }, func(q *Query) { q.ChildAges = []int{5, 8} }} {
		other = q
		change(&other)
		if q.SameContext(other) {
			t.Fatal("lost real shopper/travel context")
		}
	}
	if q.Shopper() != (Shopper{"SG", "en-SG", "SGD"}) {
		t.Fatal("Shopper lost context")
	}
}
func TestSimulatedValidateQuery(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	for _, q := range []Query{futureHotelQuery(), futureFlightQuery()} {
		if err := ValidateQueryAt(q, now); err != nil {
			t.Fatal(err)
		}
		if err := ValidateQuery(q); err != nil {
			t.Fatal(err)
		}
	}
	failures := []func(*Query){func(q *Query) { q.Adults = 0 }, func(q *Query) { q.Children = -1 }, func(q *Query) { q.Rooms = 0 }, func(q *Query) { q.CheckIn = "2026-10-01" }, func(q *Query) { q.CheckIn = "2026-02-30" }, func(q *Query) { q.CheckOut = q.CheckIn }, func(q *Query) { q.CheckOut = "2099-01-05" }, func(q *Query) { q.ChildAges = []int{8} }, func(q *Query) { q.ChildAges = []int{8, -1} }, func(q *Query) { q.PropertyID = "" }, func(q *Query) { q.Kind = "booking" }, func(q *Query) { q.Limit = -1 }}
	for _, change := range failures {
		q := futureHotelQuery()
		change(&q)
		if err := ValidateQueryAt(q, now); err == nil {
			t.Fatalf("invalid query accepted: %+v", q)
		}
	}
	for _, change := range []func(*Query){func(q *Query) { q.ReturnDate = "2099-11-19" }, func(q *Query) { q.Origin = "SIN.CGK" }, func(q *Query) { q.Destination = "SIN" }, func(q *Query) { q.Cabin = "INVENTED" }} {
		q := futureFlightQuery()
		change(&q)
		if err := ValidateQueryAt(q, now); err == nil {
			t.Fatal("invalid flight accepted")
		}
	}
}
func TestSimulatedCanonicalURLs(t *testing.T) {
	q := futureHotelQuery()
	raw, err := HotelURL(q)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(raw)
	if u.Host != "www.traveloka.com" || u.Path != "/en-sg/hotel/detail" || u.Query().Get("spec") != "06-01-2099.08-01-2099.2.2.HOTEL.9000000001714.The Berkeley Hotel Pratunam.3" || u.Query().Get("childSpec") != "8,5" {
		t.Fatalf("wrong source hotel URL: %s", raw)
	}
	q.PropertyID = ""
	q.GeoID = "10000045"
	q.PropertyName = "Bangkok"
	raw, err = HotelURL(q)
	if err != nil {
		t.Fatal(err)
	}
	u, _ = url.Parse(raw)
	if u.Path != "/en-sg/hotel/search" || !strings.Contains(u.Query().Get("spec"), ".HOTEL_GEO.10000045.Bangkok.3") {
		t.Fatal(raw)
	}
	q.PropertyName = ""
	if _, err = HotelURL(q); err == nil {
		t.Fatal("missing source name must fail")
	}
	f := futureFlightQuery()
	raw, err = FlightURL(f)
	if err != nil {
		t.Fatal(err)
	}
	u, _ = url.Parse(raw)
	if u.Path != "/en-sg/flight/fullsearch" || u.Query().Get("dt") != "20-11-2099.NA" || u.Query().Get("ps") != "1.0.0" {
		t.Fatal(raw)
	}
	f.ReturnDate = "2099-11-27"
	f.Children = 1
	f.Infants = 1
	raw, err = FlightURL(f)
	if err != nil {
		t.Fatal(err)
	}
	u, _ = url.Parse(raw)
	if u.Path != "/en-sg/flight/fulltwosearch" || u.Query().Get("dt") != "20-11-2099.27-11-2099" || u.Query().Get("ps") != "1.1.1" {
		t.Fatal(raw)
	}
	if _, err = HotelURL(f); err == nil {
		t.Fatal("wrong kind hotel handoff")
	}
	if _, err = FlightURL(futureHotelQuery()); err == nil {
		t.Fatal("wrong kind flight handoff")
	}
}
func TestSimulatedEmptyJSONListsAndUnknowns(t *testing.T) {
	b, err := json.Marshal(Snapshot{Offers: []Offer{{Legs: []Leg{{}}}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"segments":[]`, `"warnings":[]`, `"child_ages":[]`, `"stops":null`, `"refundable":null`, `"total":null`} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("missing normalized %s", want)
		}
	}
	b, _ = json.Marshal(Snapshot{})
	if !strings.Contains(string(b), `"offers":[]`) {
		t.Fatal("empty offers must be array")
	}
}
func TestSimulatedAPIError(t *testing.T) {
	e := &APIError{Code: "ACCESS_BLOCKED", Message: "refresh", Status: 202}
	if e.Error() != "ACCESS_BLOCKED: refresh" {
		t.Fatal(e.Error())
	}
}

func TestSimulatedMatchingMarketAndSameDaySourcePolicy(t *testing.T) {
	if err := ValidateShopper(Shopper{"SG", "en-ID", "SGD"}); err == nil {
		t.Fatal("locale/market mismatch accepted")
	}
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	flight := futureFlightQuery()
	flight.Depart = "2026-10-02"
	if err := ValidateQueryAt(flight, now); err != nil {
		t.Fatal("source-supported same-day departure rejected")
	}
	hotel := futureHotelQuery()
	hotel.CheckIn = "2026-10-02"
	hotel.CheckOut = "2026-10-03"
	if err := ValidateQueryAt(hotel, now); err != nil {
		t.Fatal("same-day check-in rejected")
	}
	flight.Depart = "2026-10-01"
	if err := ValidateQueryAt(flight, now); err == nil {
		t.Fatal("past departure accepted")
	}
}
