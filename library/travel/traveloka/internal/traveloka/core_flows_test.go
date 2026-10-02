package traveloka

// These fixtures simulate source HTTP contracts. They are not live Traveloka evidence.
import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

func simulatedCoreClient(t *testing.T) *Client {
	t.Helper()
	c, _ := testHTTPClient(t)
	for path := range operations {
		c.session.Profiles[path] = requestProfile{Headers: map[string]string{}, Body: map[string]any{"data": map[string]any{"tripType": "ONE_WAY", "searchId": "stale-id", "journeys": []any{map[string]any{"originCode": "OLD", "destinationCode": "OLD", "departureDate": "2000-01-01"}}, "selectedFlights": []any{"stale-selection"}, "selectedFlightsContext": map[string]any{"stale-selection": map[string]any{}}, "additionalData": map[string]any{"searchSource": "ONE_WAY"}, "numSeats": map[string]any{"numAdults": 99}, "query": "Singapore", "filterSortRequestSpec": map[string]any{}, "contexts": map[string]any{}, "sentinel": "not-public"}}}
	}
	return c
}
func coreRequestData(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	b, e := io.ReadAll(r.Body)
	if e != nil {
		t.Fatal(e)
	}
	var p map[string]any
	if e = decodeJSON(b, &p); e != nil {
		t.Fatal(e)
	}
	d := sourceObject(p["data"])
	if d == nil {
		t.Fatal("missing request data")
	}
	for k, want := range map[string]string{"Origin": origin, "Sec-Fetch-Site": "same-origin", "Sec-Fetch-Mode": "cors", "Sec-Fetch-Dest": "empty", "Tv-Country": "SG", "Tv-Language": "en_SG", "Tv-Currency": "SGD", "X-Route-Prefix": "en-sg", "X-Domain": operations[r.URL.Path]} {
		if r.Header.Get(k) != want {
			t.Fatalf("source header %s differs", k)
		}
	}
	if r.URL.Host != "www.traveloka.com" || r.Method != "POST" {
		t.Fatal("unpinned source request")
	}
	return d
}
func coreResponse(t *testing.T, r *http.Request, data any) *http.Response {
	t.Helper()
	b, e := json.Marshal(map[string]any{"data": data})
	if e != nil {
		t.Fatal(e)
	}
	return simulatedResponse(r, 200, string(b), http.Header{})
}
func coreFlightQuery() Query {
	date := time.Now().AddDate(0, 3, 0).Format("2006-01-02")
	return Query{Kind: "flights", Market: "SG", Locale: "en-SG", Currency: "SGD", Origin: "SIN", Destination: "CGK", Depart: date, Cabin: "ECONOMY", Adults: 2, Children: 1, Infants: 1, Limit: 2, MaxCandidates: 2}
}
func coreHotelQuery(kind string) Query {
	return Query{Kind: kind, Market: "SG", Locale: "en-SG", Currency: "SGD", GeoID: "10000045", PropertyName: "Bangkok", CheckIn: time.Now().AddDate(0, 3, 0).Format("2006-01-02"), CheckOut: time.Now().AddDate(0, 3, 2).Format("2006-01-02"), Adults: 3, Children: 2, ChildAges: []int{8, 5}, Rooms: 2, Limit: 2, Offset: 4}
}
func coreMoney(amount string) map[string]any {
	return map[string]any{"amount": amount, "currency": "SGD", "nullOrEmpty": false}
}
func coreDisplay(amount string) map[string]any {
	return map[string]any{"currencyValue": coreMoney(amount), "numOfDecimalPoint": "2"}
}
func coreFinalPrice(total, nightly string) map[string]any {
	return map[string]any{"totalPriceRateDisplay": map[string]any{"inclusiveFinalPrice": coreMoney(total), "exclusiveFinalPrice": coreMoney("40000"), "totalFare": coreMoney(total), "baseFare": coreMoney("40000"), "taxes": coreMoney("7181"), "fees": coreMoney("0"), "numOfDecimalPoint": "2"}, "perRoomPerNightDisplay": map[string]any{"inclusiveFinalPrice": coreMoney(nightly), "totalFare": coreMoney(nightly), "numOfDecimalPoint": "2"}}
}
func coreFlightRow(id, originCode, destination, amount string) map[string]any {
	return map[string]any{"id": id, "flightMetadata": map[string]any{"totalNumStop": "1", "tripDuration": "180", "isRefundable": false, "isReschedulable": true, "totalCombinedPrice": coreDisplay(amount)}, "fare": map[string]any{"display": coreDisplay(amount), "displayType": "ADT"}, "connectingFlightRoutes": []any{map[string]any{"flightRefundInfo": map[string]any{"refundableStatus": "NO", "refundInfoDetail": "source fare refund rules"}, "flightRescheduleInfo": map[string]any{"rescheduleStatus": "YES", "policyDetails": []any{"source fare change fee"}}, "segments": []any{map[string]any{"departureAirport": originCode, "arrivalAirport": "KUL", "airlineCode": "SQ", "brandCode": "SQ", "operatingAirlineCode": "MI", "flightNumber": "SQ-123", "seatClass": "ECONOMY", "departureDate": map[string]any{"year": "2027", "month": "1", "day": "6"}, "arrivalDate": map[string]any{"year": "2027", "month": "1", "day": "6"}, "departureTime": map[string]any{"hour": "8", "minute": "5"}, "arrivalTime": map[string]any{"hour": "9", "minute": "10"}, "tzDepartureMinuteOffset": "480", "tzArrivalMinuteOffset": "480", "durationMinutes": "65", "facilities": map[string]any{"baggage": map[string]any{"weight": "20", "quantity": "1"}, "cabinBaggage": map[string]any{"weight": "7"}}}, map[string]any{"departureAirport": "KUL", "arrivalAirport": destination, "airlineCode": "SQ", "operatingAirlineCode": "MI", "flightNumber": "SQ-456", "seatClass": "ECONOMY", "departureDate": "2027-01-06", "arrivalDate": "2027-01-06", "departureTime": "10:05", "arrivalTime": "11:00", "tzDepartureMinuteOffset": "480", "tzArrivalMinuteOffset": "420", "durationMinutes": "115"}}}}}
}
func coreFlightResult(rows []any, complete bool) map[string]any {
	return map[string]any{"searchResults": rows, "meta": map[string]any{"searchCompleted": complete, "refreshDelayMillisecond": "0", "defaultSortType": "DIRECT_FLIGHT_FIRST", "expiryTimeStamp": "1900000000000"}}
}

func TestSimulatedCoreOneWayIncrementalPrefetch(t *testing.T) {
	c := simulatedCoreClient(t)
	q := coreFlightQuery()
	q.MaxCandidates = 1
	calls := []string{}
	searchID := ""
	polls := 0
	c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		d := coreRequestData(t, r)
		calls = append(calls, r.URL.Path)
		id := sourceString(d["searchId"])
		if searchID == "" {
			searchID = id
		}
		if id != searchID || r.Header.Get("Fpr-Search-Id") != searchID || !regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$`).MatchString(id) {
			t.Fatal("fresh UUID or matching search header missing")
		}
		switch r.URL.Path {
		case flightInitialPath:
			if d["tripType"] != "ONE_WAY" || d["inventoryPricingDisplayType"] != "INDEPENDENT" || len(sourceList(d["journeys"])) != 1 || len(sourceList(d["selectedFlights"])) != 0 || len(sourceObject(d["selectedFlightsContext"])) != 0 {
				t.Fatal("stale flight template not cleared")
			}
			seats := sourceObject(d["numSeats"])
			if sourceString(seats["numAdults"]) != "2" || sourceString(seats["numChildren"]) != "1" || sourceString(seats["numInfants"]) != "1" {
				t.Fatal("wrong explicit passengers")
			}
			j := sourceObject(sourceList(d["journeys"])[0])
			if j["originCode"] != "SIN" || j["destinationCode"] != "CGK" || j["departureDate"] != q.Depart || d["seatPublishedClass"] != q.Cabin {
				t.Fatal("wrong explicit route/date/cabin")
			}
			return coreResponse(t, r, coreFlightResult([]any{coreFlightRow("out", "SIN", "CGK", "10000")}, false)), nil
		case flightPollPath:
			polls++
			return coreResponse(t, r, coreFlightResult([]any{}, polls == 2)), nil
		case flightPrefetchPath:
			if d["isPrefetch"] != true || d["isBreakSmartCombo"] != false || !reflect.DeepEqual(sourceList(d["journeyIds"]), []any{"out"}) {
				t.Fatal("not exact read-only source prefetch")
			}
			if sourceObject(sourceObject(d["journeyContextData"])["out"])["isBaggageFilterEnabled"] != false {
				t.Fatal("missing prefetch selection context")
			}
			return coreResponse(t, r, map[string]any{"totalPrice": coreDisplay("32789"), "displayedPricePerPax": coreDisplay("10930"), "displayType": "ADT"}), nil
		}
		t.Fatal("unexpected source path")
		return nil, nil
	}))
	s, e := c.SearchFlights(context.Background(), q)
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Offers) != 1 || s.Status != "success" || !s.SearchComplete || polls != 2 {
		t.Fatal("empty incremental polls erased initial inventory")
	}
	o := s.Offers[0]
	if o.Price.Total.Amount != "327.89" || o.Price.PerPassenger.Amount != "109.30" {
		t.Fatal("source prefetch total/per-passenger not authoritative")
	}
	if len(o.Legs) != 1 || len(o.Legs[0].Segments) != 2 || *o.Stops != 1 || *o.DurationMinutes != 180 {
		t.Fatal("lost connecting itinerary/dimensions")
	}
	seg := o.Legs[0].Segments[0]
	if seg.DepartureDate != "2027-01-06" || seg.DepartureTime != "08:05" || *seg.DepartureUTCOffsetMinutes != 480 || seg.MarketingAirline != "SQ" || seg.OperatingAirline != "MI" || seg.FlightNumber != "SQ-123" || sourceObject(seg.CheckedBaggage)["weight"] != "20" || sourceObject(seg.CabinBaggage)["weight"] != "7" {
		t.Fatal("lost segment source fields")
	}
	if o.Legs[0].Segments[1].ArrivalUTCOffsetMinutes == nil || *o.Legs[0].Segments[1].ArrivalUTCOffsetMinutes != 420 || o.Legs[0].Segments[1].CheckedBaggage != nil {
		t.Fatal("offset or unknown baggage incorrect")
	}
	if o.Refundable == nil || *o.Refundable || o.Reschedulable == nil || !*o.Reschedulable || len(sourceList(o.Legs[0].FareRules["routes"])) != 1 {
		t.Fatal("fare-specific policies lost")
	}
	want, _ := FlightURL(q)
	if o.BookingURL != want || !s.Indicative || s.Freshness != "fresh_retrieval" {
		t.Fatal("canonical handoff/fresh quote markers missing")
	}
	ts, e := time.Parse(time.RFC3339Nano, s.RetrievedAt)
	if e != nil || time.Since(ts) > time.Minute {
		t.Fatal("retrieval timestamp is not current")
	}
	if len(calls) != 4 {
		t.Fatal("wrong source call flow")
	}
}
func TestSimulatedCoreReturnReconstructsBothJourneysAndCombinedTotal(t *testing.T) {
	c := simulatedCoreClient(t)
	q := coreFlightQuery()
	q.ReturnDate = time.Now().AddDate(0, 3, 7).Format("2006-01-02")
	q.Limit = 1
	q.MaxCandidates = 1
	sid := ""
	returned := false
	c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		d := coreRequestData(t, r)
		if sid == "" {
			sid = sourceString(d["searchId"])
		}
		if sourceString(d["searchId"]) != sid || r.Header.Get("Fpr-Search-Id") != sid {
			t.Fatal("round-trip changed fresh search identity")
		}
		switch r.URL.Path {
		case flightInitialPath:
			js := sourceList(d["journeys"])
			if d["tripType"] != "ROUND_TRIP" || d["inventoryPricingDisplayType"] != "COMBINED" || sourceObject(d["additionalData"])["searchSource"] != "ROUNDTRIP" || len(js) != 2 {
				t.Fatal("one-way captured template not reconstructed")
			}
			j := sourceObject(js[1])
			if j["originCode"] != "CGK" || j["destinationCode"] != "SIN" || j["departureDate"] != q.ReturnDate || len(sourceList(d["selectedFlights"])) != 0 {
				t.Fatal("wrong return journey or stale selection")
			}
			return coreResponse(t, r, coreFlightResult([]any{coreFlightRow("out", "SIN", "CGK", "12000")}, true)), nil
		case flightPollPath:
			returned = true
			if sourceString(d["journeyIndex"]) != "1" || !reflect.DeepEqual(sourceList(d["selectedFlights"]), []any{"out"}) || sourceObject(sourceObject(d["selectedFlightsContext"])["out"])["isBaggageFilterEnabled"] != false || len(sourceList(d["journeys"])) != 2 {
				t.Fatal("return poll lacks exact outbound context")
			}
			return coreResponse(t, r, coreFlightResult([]any{coreFlightRow("ret", "CGK", "SIN", "0")}, true)), nil
		case flightPrefetchPath:
			if !returned || !reflect.DeepEqual(sourceList(d["journeyIds"]), []any{"out", "ret"}) {
				t.Fatal("prefetch did not use exact two-journey combination")
			}
			return coreResponse(t, r, map[string]any{"totalPrice": coreDisplay("34567"), "displayedPricePerPax": coreDisplay("11522")}), nil
		}
		t.Fatal("unexpected return source path")
		return nil, nil
	}))
	s, e := c.SearchFlights(context.Background(), q)
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Offers) != 1 || len(s.Offers[0].Legs) != 2 || s.Offers[0].Price.Total.Amount != "345.67" || s.Offers[0].Price.PerPassenger.Amount != "115.22" || s.Offers[0].Legs[1].Segments[1].Destination != "SIN" {
		t.Fatal("return delta was used as total or a leg was omitted")
	}
}
func TestSimulatedCoreFlightCompletionAndCandidateCaps(t *testing.T) {
	for _, complete := range []bool{true, false} {
		t.Run(map[bool]string{true: "completed empty", false: "incomplete empty"}[complete], func(t *testing.T) {
			c := simulatedCoreClient(t)
			calls := 0
			c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				coreRequestData(t, r)
				return coreResponse(t, r, coreFlightResult([]any{}, complete)), nil
			}))
			s, e := c.SearchFlights(context.Background(), coreFlightQuery())
			if e != nil {
				t.Fatal(e)
			}
			want := "incomplete"
			if complete {
				want = "no_inventory"
			}
			if s.Status != want || s.SearchComplete != complete || len(s.Offers) != 0 {
				t.Fatal("empty completion state collapsed")
			}
			if !complete && (calls != 13 || len(s.Warnings) == 0) {
				t.Fatal("incomplete source search was not bounded/labeled")
			}
		})
	}
	c := simulatedCoreClient(t)
	q := coreFlightQuery()
	q.MaxCandidates = 2
	q.Limit = 1
	prefetches := 0
	c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		coreRequestData(t, r)
		if r.URL.Path == flightInitialPath {
			return coreResponse(t, r, coreFlightResult([]any{coreFlightRow("a", "SIN", "CGK", "40000"), coreFlightRow("b", "SIN", "CGK", "10000"), coreFlightRow("c", "SIN", "CGK", "20000")}, true)), nil
		}
		prefetches++
		return coreResponse(t, r, map[string]any{"totalPrice": coreDisplay("90000")}), nil
	}))
	s, e := c.SearchFlights(context.Background(), q)
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Offers) != 1 || s.Offers[0].ID != "b" || prefetches != 1 || s.Coverage["outbound_observed"] != 3 || s.Coverage["outbound_retained"] != 2 || s.Coverage["outbound_candidates_truncated"] != true {
		t.Fatal("candidate bound and output limit conflated")
	}
}
func TestSimulatedCoreFlightPartialAndAllPrefetchErrors(t *testing.T) {
	for _, all := range []bool{false, true} {
		t.Run(map[bool]string{false: "partial", true: "all"}[all], func(t *testing.T) {
			c := simulatedCoreClient(t)
			q := coreFlightQuery()
			c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
				d := coreRequestData(t, r)
				if r.URL.Path == flightInitialPath {
					return coreResponse(t, r, coreFlightResult([]any{coreFlightRow("a", "SIN", "CGK", "10000"), coreFlightRow("b", "SIN", "CGK", "20000")}, true)), nil
				}
				if all || sourceList(d["journeyIds"])[0] == "a" {
					return simulatedResponse(r, 403, "", http.Header{}), nil
				}
				return coreResponse(t, r, map[string]any{"totalPrice": coreDisplay("23456")}), nil
			}))
			s, e := c.SearchFlights(context.Background(), q)
			if all {
				var ae *APIError
				if s != nil || !errors.As(e, &ae) || ae.Code != "ACCESS_BLOCKED" {
					t.Fatal("all prefetch access failures became inventory")
				}
				return
			}
			if e != nil || len(s.Offers) != 1 || s.Offers[0].ID != "b" || s.Offers[0].Price.Total.Amount != "234.56" || s.Coverage["prefetch_failed"] != 1 || len(s.Warnings) == 0 {
				t.Fatal("partial prefetch failure became phantom zero quote or lost coverage")
			}
		})
	}
}

func TestSimulatedCoreResolverRankedGroupsAndAmbiguity(t *testing.T) {
	c := simulatedCoreClient(t)
	c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		d := coreRequestData(t, r)
		if d["query"] != "Singapore" {
			t.Fatal("query not explicit")
		}
		if r.URL.Path == airportSearchPath {
			return coreResponse(t, r, map[string]any{"sections": []any{map[string]any{"type": "POPULAR", "results": []any{map[string]any{"code": "SIN", "type": "AIRPORT", "displayData": map[string]any{"title": "Singapore popular"}}}}, map[string]any{"type": "SEARCH", "results": []any{map[string]any{"code": "SINA", "entityId": "SINA", "iataCode": "SIN", "areaCode": "SINA", "type": "CITY", "country": "Singapore", "displayData": map[string]any{"title": "Singapore"}}, map[string]any{"code": "SIN", "entityId": "SIN", "iataCode": "SIN", "type": "AIRPORT", "country": "Singapore", "location": "Singapore", "geoLocation": map[string]any{"lat": "1.36", "lon": "103.99"}, "displayData": map[string]any{"title": "Changi International Airport", "description": "Singapore"}}, map[string]any{"code": "DPS", "type": "AIRPORT", "displayData": map[string]any{"title": "Bali"}}}}}, "directory": []any{map[string]any{"code": "BAD", "type": "AIRPORT", "displayData": map[string]any{"title": "Singapore directory"}}}}), nil
		}
		if sourceObject(sourceObject(d["experimentContext"])["mapParamKeyToVariant"])["varAutocompleteLogic"] != "control" {
			t.Fatal("resolver source experiment missing")
		}
		return coreResponse(t, r, map[string]any{"autoCompleteContent": map[string]any{"rows": []any{}}, "geoRegionContent": map[string]any{"rows": []any{map[string]any{"id": "107493", "type": "GEO_REGION", "name": "Singapore", "displayName": "Singapore", "country": "SG", "geoLocation": map[string]any{"lat": "1.29", "lon": "103.84"}}}}, "hotelContent": map[string]any{"rows": []any{map[string]any{"id": "1635", "type": "HOTEL", "name": "M Hotel Singapore", "displayName": "M Hotel Singapore", "geoId": "107493"}, map[string]any{"id": "irrelevant", "type": "HOTEL", "name": "Bali hotel"}}}, "frequentlyBooked": map[string]any{"rows": []any{map[string]any{"id": "POPULAR", "type": "HOTEL", "name": "Singapore"}}}}), nil
	}))
	r, e := c.Resolve(context.Background(), "Singapore", "all", Shopper{"SG", "en-SG", "SGD"}, 3)
	if e != nil {
		t.Fatal(e)
	}
	if !r.Ambiguous || len(r.Locations) != 3 || r.Coverage["matched"] != 4 || r.Coverage["truncated"] != true {
		t.Fatal("source ambiguity/bounds lost")
	}
	if r.Locations[0].ID != "SINA" || r.Locations[0].Type != "CITY" || r.Locations[1].ID != "SIN" || r.Locations[1].Coordinates["lat"] != "1.36" || r.Locations[2].Namespace != "hotel" {
		t.Fatal("source IDs/namespaces/coordinates lost")
	}
	for _, l := range r.Locations {
		if l.ID == "BAD" || l.ID == "DPS" || l.ID == "POPULAR" {
			t.Fatal("directory/popular/unrelated data returned as query match")
		}
	}
}
func TestSimulatedCoreHotelPageExactPriceAndParty(t *testing.T) {
	c := simulatedCoreClient(t)
	q := coreHotelQuery("hotels")
	c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		d := coreRequestData(t, r)
		assertHotelParty(t, d, q)
		if r.URL.Path != hotelSearchPath || d["sourceType"] != "HOTEL_GEO" || d["geoId"] != q.GeoID {
			t.Fatal("wrong hotel geo request")
		}
		f := sourceObject(d["filterSortRequestSpec"])
		if sourceString(f["skip"]) != "4" || sourceString(f["top"]) != "2" || f["basicSortType"] != "POPULARITY" {
			t.Fatal("wrong source pagination/order")
		}
		u, _ := HotelURL(q)
		if sourceObject(d["contexts"])["searchURL"] != u {
			t.Fatal("canonical search context missing")
		}
		card := func(id string, match any) any {
			return map[string]any{"displayType": "INVENTORY", "contentType": "HOTEL", "data": map[string]any{"id": id, "name": "Source Hotel " + id, "reviewSummary": map[string]any{"ugcReviewSummary": map[string]any{"userRating": "8.9"}}, "region": "Pratunam", "hotelFeatures": []any{"Pool"}, "hotelInventorySummary": map[string]any{"matchSearchOccupancy": match, "numChargedRooms": "2", "finalPrice": coreFinalPrice("47181", "11795"), "hasFreeCancellationRooms": true}}}
		}
		return coreResponse(t, r, map[string]any{"numOfHotels": "50", "numOfOriginalHotelCard": "2", "entries": []any{map[string]any{"displayType": "GENERIC_CARD", "contentType": "HOTEL", "data": map[string]any{"id": "banner"}}, card("one", true), map[string]any{"displayType": "CURATION_LIST", "contentType": "HOTEL", "data": map[string]any{"id": "curation"}}, card("two", false)}}), nil
	}))
	s, e := c.SearchHotels(context.Background(), q)
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Offers) != 2 || s.Status != "success" || s.Coverage["next_offset"] != 6 || s.Coverage["scanned_entries"] != 4 || s.Coverage["source_total"] != "50" {
		t.Fatal("mixed entries/pagination/source total incorrect")
	}
	o := s.Offers[0]
	if o.Price.Total.Amount != "471.81" || o.Price.PerRoomPerNight.Amount != "117.95" || o.Price.Taxes.Amount != "71.81" || o.Price.Fees.Amount != "0.00" || o.Price.TaxInclusion != "inclusive" {
		t.Fatal("source stay/nightly tax rounding corrupted")
	}
	if o.Refundable != nil || o.OccupancyMatch == nil || !*o.OccupancyMatch || s.Offers[1].OccupancyMatch == nil || *s.Offers[1].OccupancyMatch {
		t.Fatal("property-wide policy inferred or mismatch filtered")
	}
	dq := q
	dq.PropertyID = o.PropertyID
	dq.PropertyName = o.PropertyName
	want, _ := HotelURL(dq)
	if o.BookingURL != want || !strings.Contains(want, "childSpec=8%2C5") || o.Details["num_charged_rooms"] != "2" {
		t.Fatal("exact property handoff/charged rooms missing")
	}
}
func assertHotelParty(t *testing.T, d map[string]any, q Query) {
	t.Helper()
	for k, want := range map[string]string{"numAdults": "3", "numChildren": "2", "numInfants": "0", "numRooms": "2", "numOfNights": "2"} {
		if sourceString(d[k]) != want {
			t.Fatalf("wrong hotel party %s", k)
		}
	}
	if !reflect.DeepEqual(sourceList(d["childAges"]), []any{json.Number("8"), json.Number("5")}) || !reflect.DeepEqual(sourceObject(d["checkInDate"]), dateObject(q.CheckIn)) || !reflect.DeepEqual(sourceObject(d["checkOutDate"]), dateObject(q.CheckOut)) {
		t.Fatal("hotel date objects/ages wrong")
	}
	if d["currency"] != "SGD" || !reflect.DeepEqual(sourceList(d["rateTypes"]), []any{"PAY_NOW", "PAY_AT_PROPERTY"}) || sourceObject(d["ccGuaranteeOptions"])["ccInfoPreferences"] == nil {
		t.Fatal("source rate/CC/currency context missing")
	}
}
func TestSimulatedCoreRoomRatePoliciesDifferAndUnknownsRemain(t *testing.T) {
	c := simulatedCoreClient(t)
	q := coreHotelQuery("rooms")
	q.PropertyID = "9000000001714"
	q.PropertyName = "Source Property"
	q.Limit = 3
	q.Offset = 0
	c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		d := coreRequestData(t, r)
		assertHotelParty(t, d, q)
		if r.URL.Path != hotelRoomsPath || d["hotelId"] != q.PropertyID || d["isReschedule"] != false || d["prefetch"] != false {
			t.Fatal("wrong read-only room request")
		}
		want, _ := HotelURL(q)
		if sourceObject(d["contexts"])["hotelDetailURL"] != want {
			t.Fatal("wrong property context URL")
		}
		rate := func(id, meal, payment string, refundable any, match any) any {
			return map[string]any{"hotelRoomInventoryId": id, "inventoryName": "Source " + id, "inventoryGroupId": "group-" + id, "maxOccupancy": "3", "maxChildOccupancy": "2", "numRemainingRooms": "4", "isBreakfastIncluded": meal == "Breakfast", "mealPlanDisplay": map[string]any{"displayMealPlanIncluded": meal}, "rateType": payment, "ccGuaranteeRequirement": map[string]any{"required": payment == "PAY_AT_PROPERTY"}, "bookingPolicy": map[string]any{"label": payment}, "isRefundable": refundable, "matchSearchOccupancy": match, "finalPrice": coreFinalPrice("47181", "11795"), "inventoryRateKey": "must-not-persist", "roomCancellationPolicy": map[string]any{"freeCancel": refundable, "cancellationPolicyLabel": "terms for " + id, "cancellationPolicyInfos": []any{map[string]any{"appliedEndDate": map[string]any{"monthDayYear": map[string]any{"year": "2027", "month": "1", "day": "3"}, "hourMinute": map[string]any{"hour": "22", "minute": "58"}}, "policyInfoDetail": map[string]any{"description": "source fee", "fee": coreDisplay("10000")}}}}}
		}
		return coreResponse(t, r, map[string]any{"status": "SUCCESS", "recommendedEntries": []any{map[string]any{"hotelRoomId": "room1", "name": "Twin", "hotelBedType": []any{"2 single beds"}, "hotelRoomSizeDisplay": map[string]any{"value": "35", "unit": "m2"}, "baseOccupancy": "2", "matchSearchOccupancy": true, "numChargedRooms": "2", "hotelRoomInventoryList": []any{rate("free", "Breakfast", "PAY_AT_PROPERTY", true, true), rate("fixed", "Room only", "PAY_NOW", false, false), rate("unknown", "Half board", "PAY_NOW", nil, nil)}}}}), nil
	}))
	s, e := c.HotelRooms(context.Background(), q)
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Offers) != 3 || s.Coverage["scanned_rate_plans"] != 3 {
		t.Fatal("source rate plans not preserved")
	}
	a, b, u := s.Offers[0], s.Offers[1], s.Offers[2]
	if a.MealPlan == b.MealPlan || a.Payment == b.Payment || a.Refundable == nil || !*a.Refundable || b.Refundable == nil || *b.Refundable || b.OccupancyMatch == nil || *b.OccupancyMatch || u.Refundable != nil || u.OccupancyMatch != nil {
		t.Fatal("rate policy/occupancy difference or unknown collapsed")
	}
	if a.Cancellation["timezone_known"] != false || len(sourceList(a.Cancellation["cancellationPolicyInfos"])) != 1 || a.RoomID != "room1" || a.RoomName != "Twin" || sourceObject(a.Details["rate"])["numRemainingRooms"] != "4" || sourceObject(a.Details["rate"])["ccGuaranteeRequirement"] == nil {
		t.Fatal("source cancellation ranges/remaining rooms/CC terms lost")
	}
	encoded, _ := json.Marshal(s)
	if strings.Contains(string(encoded), "inventoryRateKey") || strings.Contains(string(encoded), "must-not-persist") {
		t.Fatal("opaque rate key escaped public snapshot")
	}
}
func TestSimulatedCoreMissingHotelNameUsesSourceIDMatch(t *testing.T) {
	for _, property := range []bool{false, true} {
		t.Run(map[bool]string{false: "destination", true: "property"}[property], func(t *testing.T) {
			c := simulatedCoreClient(t)
			q := coreHotelQuery("hotels")
			q.PropertyName = ""
			id, name, kind := "10000045", "Bangkok", "GEO_REGION"
			if property {
				q.Kind = "rooms"
				q.PropertyID = "9000000001714"
				id, name, kind = q.PropertyID, "The Berkeley Hotel Pratunam", "HOTEL"
			}
			resolves := 0
			c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
				d := coreRequestData(t, r)
				if r.URL.Path == hotelAutocompletePath {
					resolves++
					return coreResponse(t, r, map[string]any{"autoCompleteContent": map[string]any{"rows": []any{}}, "hotelContent": map[string]any{"rows": []any{map[string]any{"id": "wrong", "type": kind, "displayName": "Wrong Name"}, map[string]any{"id": id, "type": kind, "displayName": name}}}}), nil
				}
				contexts := sourceObject(d["contexts"])
				key := "searchURL"
				if property {
					key = "hotelDetailURL"
				}
				link, e := url.Parse(sourceString(contexts[key]))
				if e != nil || !strings.Contains(link.Query().Get("spec"), "."+name+".") {
					t.Fatal("canonical URL name was guessed instead of source matched")
				}
				if property {
					return coreResponse(t, r, map[string]any{"status": "SUCCESS", "recommendedEntries": []any{}}), nil
				}
				return coreResponse(t, r, map[string]any{"numOfHotels": "0", "entries": []any{}}), nil
			}))
			var s *Snapshot
			var e error
			if property {
				s, e = c.HotelRooms(context.Background(), q)
			} else {
				s, e = c.SearchHotels(context.Background(), q)
			}
			if e != nil {
				t.Fatal(e)
			}
			if resolves != 1 || s.Query.PropertyName != name || s.Status != "no_inventory" {
				t.Fatal("actual source name/empty completion lost")
			}
		})
	}
}

func TestSimulatedCoreInvalidInputRejectedBeforeHTTP(t *testing.T) {
	qf, qh, qr := coreFlightQuery(), coreHotelQuery("hotels"), coreHotelQuery("rooms")
	qr.PropertyID = "p"
	qr.PropertyName = "P"
	cases := []struct {
		name  string
		query Query
		call  string
	}{{"wrong flight kind", qh, "flights"}, {"wrong hotels kind", qf, "hotels"}, {"missing rooms property", coreHotelQuery("rooms"), "rooms"}, {"invalid flight date", func() Query { q := qf; q.Depart = "2027-02-30"; return q }(), "flights"}, {"negative age", func() Query { q := qh; q.ChildAges = []int{8, -1}; return q }(), "hotels"}, {"missing child age", func() Query { q := qr; q.ChildAges = []int{8}; return q }(), "rooms"}, {"reversed stay", func() Query { q := qh; q.CheckOut = q.CheckIn; return q }(), "hotels"}, {"zero rooms", func() Query { q := qr; q.Rooms = 0; return q }(), "rooms"}, {"hotel missing geographic ID", func() Query { q := qh; q.GeoID = ""; return q }(), "hotels"}}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			c := simulatedCoreClient(t)
			c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
				t.Fatal("invalid input reached source")
				return nil, nil
			}))
			var e error
			switch tt.call {
			case "flights":
				_, e = c.SearchFlights(context.Background(), tt.query)
			case "hotels":
				_, e = c.SearchHotels(context.Background(), tt.query)
			case "rooms":
				_, e = c.HotelRooms(context.Background(), tt.query)
			}
			var ae *APIError
			if !errors.As(e, &ae) || ae.Code != "INVALID_INPUT" {
				t.Fatalf("wrong invalid input classification: %v", e)
			}
		})
	}
}
func TestSimulatedCoreSourceFailuresAndCurrencyMismatch(t *testing.T) {
	for _, tt := range []struct {
		name, body, code string
		status           int
	}{{"access", "", "ACCESS_BLOCKED", 202}, {"auth", "", "AUTH_REQUIRED", 401}, {"malformed", "[]", "MALFORMED_RESPONSE", 200}, {"source error", `{"data":{"status":"ERROR","message":"do not treat as empty"}}`, "UPSTREAM_ERROR", 200}, {"source error object", `{"error":{"code":"broken"},"data":{"entries":[]}}`, "UPSTREAM_ERROR", 200}, {"throttle", "", "RATE_LIMITED", 429}} {
		t.Run(tt.name, func(t *testing.T) {
			c := simulatedCoreClient(t)
			c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
				return simulatedResponse(r, tt.status, tt.body, http.Header{"Retry-After": []string{"2"}}), nil
			}))
			_, e := c.SearchHotels(context.Background(), coreHotelQuery("hotels"))
			var ae *APIError
			if !errors.As(e, &ae) || ae.Code != tt.code {
				t.Fatalf("source failure did not propagate: %v", e)
			}
		})
	}
	for _, kind := range []string{"flights", "hotels", "rooms"} {
		t.Run(kind+" currency", func(t *testing.T) {
			c := simulatedCoreClient(t)
			c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
				if kind == "flights" {
					row := coreFlightRow("a", "SIN", "CGK", "10000")
					sourceObject(sourceObject(sourceObject(row["flightMetadata"])["totalCombinedPrice"])["currencyValue"])["currency"] = "USD"
					return coreResponse(t, r, coreFlightResult([]any{row}, true)), nil
				}
				price := coreFinalPrice("47181", "11795")
				sourceObject(sourceObject(price["totalPriceRateDisplay"])["inclusiveFinalPrice"])["currency"] = "USD"
				if kind == "hotels" {
					return coreResponse(t, r, map[string]any{"numOfHotels": "1", "entries": []any{map[string]any{"displayType": "INVENTORY", "contentType": "HOTEL", "data": map[string]any{"id": "p", "name": "Property", "hotelInventorySummary": map[string]any{"finalPrice": price}}}}}), nil
				}
				return coreResponse(t, r, map[string]any{"status": "SUCCESS", "recommendedEntries": []any{map[string]any{"hotelRoomId": "r", "hotelRoomInventoryList": []any{map[string]any{"hotelRoomInventoryId": "rate", "finalPrice": price}}}}}), nil
			}))
			var e error
			switch kind {
			case "flights":
				_, e = c.SearchFlights(context.Background(), coreFlightQuery())
			case "hotels":
				_, e = c.SearchHotels(context.Background(), coreHotelQuery("hotels"))
			case "rooms":
				q := coreHotelQuery("rooms")
				q.PropertyID = "p"
				q.PropertyName = "Property"
				_, e = c.HotelRooms(context.Background(), q)
			}
			var ae *APIError
			if !errors.As(e, &ae) || ae.Code != "CURRENCY_MISMATCH" {
				t.Fatalf("currency mismatch silently relabeled: %v", e)
			}
		})
	}
}

func TestSimulatedRoundTripEmptyCandidateBoundIsIncomplete(t *testing.T) {
	c := simulatedCoreClient(t)
	q := coreFlightQuery()
	q.ReturnDate = time.Now().AddDate(0, 3, 5).Format("2006-01-02")
	q.MaxCandidates = 1
	c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		data := coreRequestData(t, r)
		if r.URL.Path == flightInitialPath {
			return coreResponse(t, r, coreFlightResult([]any{coreFlightRow("cheap", "SIN", "CGK", "10000"), coreFlightRow("has-return", "SIN", "CGK", "20000")}, true)), nil
		}
		if sourceInt(data["journeyIndex"]) == nil {
			t.Fatal("missing return journey index")
		}
		if sourceList(data["selectedFlights"])[0] != "cheap" {
			t.Fatal("candidate bound exceeded")
		}
		return coreResponse(t, r, coreFlightResult([]any{}, true)), nil
	}))
	s, err := c.SearchFlights(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != "incomplete" || s.SearchComplete || len(s.Offers) != 0 || s.Coverage["outbound_candidates_truncated"] != true {
		t.Fatalf("bounded empty result overstated inventory: %#v", s)
	}
	if len(s.Warnings) == 0 {
		t.Fatal("missing bounded coverage warning")
	}
}
