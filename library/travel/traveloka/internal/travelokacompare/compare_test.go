package travelokacompare

import (
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/traveloka"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// All data in this file is SIMULATED; no test is live inventory evidence.
func ptr[T any](v T) *T { return &v }
func money(raw string, scale int) *traveloka.Money {
	rendered, _ := traveloka.FormatMinorUnits(raw, scale)
	return &traveloka.Money{Currency: "SGD", MinorUnits: raw, Amount: rendered, Decimals: ptr(scale)}
}
func flight(id, amount string, stops, duration int) traveloka.Offer {
	return traveloka.Offer{ID: id, Kind: "flight", Price: traveloka.Price{Total: money(amount, 2), TaxInclusion: "unknown"}, Stops: ptr(stops), DurationMinutes: ptr(duration),
		Details: map[string]any{"price_basis": "party_trip_total"}, Legs: []traveloka.Leg{{Details: map[string]any{"id": "SIMULATED-leg"}, Segments: []traveloka.Segment{{Origin: "SIN", Destination: "CGK", DepartureDate: "2027-01-06", DepartureTime: "10:00", ArrivalDate: "2027-01-06", ArrivalTime: "11:00", DepartureUTCOffsetMinutes: ptr(480), ArrivalUTCOffsetMinutes: ptr(420), MarketingAirline: "TR", FlightNumber: "SIMULATED-100", Cabin: "ECONOMY"}}}}}
}
func flightSnapshot(offers ...traveloka.Offer) *traveloka.Snapshot {
	return &traveloka.Snapshot{ID: "SIMULATED-flight", Kind: "flights", RetrievedAt: "2026-10-02T00:00:00Z", Query: traveloka.Query{Kind: "flights", Market: "SG", Locale: "en-SG", Currency: "SGD", Origin: "SIN", Destination: "CGK", Depart: "2027-01-06", Cabin: "ECONOMY", Adults: 1}, Offers: offers}
}
func copyOffer(o traveloka.Offer) traveloka.Offer {
	b, _ := json.Marshal(o)
	var cloned traveloka.Offer
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	_ = dec.Decode(&cloned)
	return cloned
}
func offerIDs(offers []traveloka.Offer) []string {
	ids := []string{}
	for _, o := range offers {
		ids = append(ids, o.ID)
	}
	sort.Strings(ids)
	return ids
}

func TestExactTotalSimulated(t *testing.T) {
	cases := []struct {
		name          string
		m             *traveloka.Money
		want          string
		unknown, fail bool
	}{
		{"beyond_float_precision", money("9007199254740993", 2), "9007199254740993/100", false, false},
		{"scale_zero", money("123", 0), "123", false, false},
		{"missing_money", nil, "", true, false},
		{"unknown_scale", &traveloka.Money{Currency: "SGD", MinorUnits: "123"}, "", true, false},
		{"bad_currency", &traveloka.Money{Currency: "USD", MinorUnits: "123", Decimals: ptr(2)}, "", false, true},
		{"bad_units", &traveloka.Money{Currency: "SGD", MinorUnits: "1.5", Decimals: ptr(2)}, "", false, true},
		{"conflicting_amount", &traveloka.Money{Currency: "SGD", MinorUnits: "100", Decimals: ptr(2), Amount: "2.00"}, "", false, true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, e := ExactTotal(tt.m, "SGD")
			if (e != nil) != tt.fail {
				t.Fatalf("error=%v", e)
			}
			if tt.fail {
				return
			}
			if tt.unknown {
				if got != nil {
					t.Fatal("unknown invented amount")
				}
				return
			}
			if got.RatString() != tt.want {
				t.Fatalf("exact total=%s want %s", got.RatString(), tt.want)
			}
		})
	}
}
func TestDifferenceSimulated(t *testing.T) {
	cases := []struct {
		name          string
		before, after *traveloka.Money
		amount, units string
		scale         int
	}{
		{"large_precision", money("9007199254740992", 2), money("9007199254740993", 2), "0.01", "1", 2},
		{"different_source_scales", money("123", 2), money("1255", 3), "0.025", "25", 3},
		{"negative_difference", money("1500", 2), money("1000", 2), "-5.00", "-500", 2},
		{"unknown_price", nil, money("100", 2), "", "", 0},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			d, e := Difference(tt.before, tt.after, "SGD")
			if e != nil {
				t.Fatal(e)
			}
			if tt.before == nil {
				if d != nil {
					t.Fatal("unknown invented delta")
				}
				return
			}
			if d.Amount != tt.amount || d.MinorUnits != tt.units || *d.Decimals != tt.scale {
				t.Fatalf("wrong exact delta: %+v", d)
			}
		})
	}
}
func TestFrontierSimulated(t *testing.T) {
	missing := flight("unknown", "100", 0, 100)
	missing.DurationMinutes = nil
	cases := []struct {
		name               string
		offers             []traveloka.Offer
		ids                []string
		unknown, dominated int
	}{
		{"dominance", []traveloka.Offer{flight("winner", "10000", 0, 100), flight("dominated", "20000", 1, 200)}, []string{"winner"}, 0, 1},
		{"trade_off", []traveloka.Offer{flight("cheap_slow", "10000", 1, 200), flight("fast", "20000", 0, 100)}, []string{"cheap_slow", "fast"}, 0, 0},
		{"all_ties_retained", []traveloka.Offer{flight("tie1", "10000", 0, 100), flight("tie2", "10000", 0, 100)}, []string{"tie1", "tie2"}, 0, 0},
		{"unknown_separate", []traveloka.Offer{missing, flight("known", "20000", 0, 100)}, []string{"known"}, 1, 0},
		{"exact_beyond_float", []traveloka.Offer{flight("cheap", "9007199254740992", 0, 100), flight("expensive", "9007199254740993", 0, 100)}, []string{"cheap"}, 0, 1},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			r, e := Frontier(flightSnapshot(tt.offers...), 100, 1000)
			if e != nil {
				t.Fatal(e)
			}
			if !reflect.DeepEqual(offerIDs(r.Offers), tt.ids) || len(r.Unknown) != tt.unknown || r.Dominated != tt.dominated || r.Scanned != len(tt.offers) {
				t.Fatalf("frontier wrong: %+v", r)
			}
		})
	}
	r, e := Frontier(flightSnapshot(flight("a", "100", 0, 100), flight("b", "100", 0, 100), flight("c", "50", 0, 100)), 1, 2)
	if e != nil || !r.Truncated || r.Scanned != 2 || r.FrontierCount != 2 || len(r.Offers) != 1 {
		t.Fatalf("scan/output bounds wrong %+v %v", r, e)
	}
	bad := flight("bad", "100", 0, 100)
	bad.Price.Total.Currency = "USD"
	if _, e := Frontier(flightSnapshot(bad), 20, 500); e == nil {
		t.Fatal("currency mismatch accepted")
	}
}

func room(id string, refundable bool, raw string) traveloka.Offer {
	return traveloka.Offer{ID: id, Kind: "room", PropertyID: "SIMULATED-property", RoomID: "SIMULATED-room", RoomName: "Double", MealPlan: "Breakfast", Payment: "PAY_NOW", OccupancyMatch: ptr(true), Refundable: ptr(refundable), Price: traveloka.Price{Total: money(raw, 2), TaxInclusion: "inclusive"}, Cancellation: map[string]any{"timezone_known": false, "source_terms": "SIMULATED-exact-terms"}, Details: map[string]any{"price_basis": "party_stay_total", "rate": map[string]any{
		"maxOccupancy": json.Number("2"), "maxChildOccupancy": json.Number("0"), "numChargedRooms": json.Number("1"), "mealPlanDisplay": map[string]any{"display": "Breakfast"}, "isBreakfastIncluded": true, "displayNumBreakfastIncluded": "2", "ccGuaranteeRequirement": "NOT_REQUIRED", "bookingPolicy": map[string]any{"payment": "PAY_NOW"}, "paymentDisplay": "Pay now", "inventoryGroupId": "SIMULATED-group",
	}}}
}
func roomSnapshot(offers ...traveloka.Offer) *traveloka.Snapshot {
	return &traveloka.Snapshot{ID: "SIMULATED-room-snapshot", Kind: "rooms", RetrievedAt: "2026-10-02T00:00:00Z", Query: traveloka.Query{Kind: "rooms", Market: "SG", Locale: "en-SG", Currency: "SGD", PropertyID: "SIMULATED-property", CheckIn: "2027-01-06", CheckOut: "2027-01-08", Adults: 2, Rooms: 1, ChildAges: []int{}}, Offers: offers}
}
func TestFlexibilitySimulated(t *testing.T) {
	cases := []struct {
		name   string
		edit   func(*traveloka.Offer)
		pairs  int
		reason string
	}{
		{"exact_pair", func(o *traveloka.Offer) {}, 1, ""},
		{"unknown_refundable", func(o *traveloka.Offer) { o.Refundable = nil }, 0, "unknown_refundable"},
		{"meal_mismatch", func(o *traveloka.Offer) { o.MealPlan = "No breakfast" }, 0, "no_opposite"},
		{"payment_mismatch", func(o *traveloka.Offer) { o.Payment = "PAY_AT_PROPERTY" }, 0, "no_opposite"},
		{"credit_guarantee_mismatch", func(o *traveloka.Offer) { object(o.Details["rate"])["ccGuaranteeRequirement"] = "REQUIRED" }, 0, "no_opposite"},
		{"booking_terms_mismatch", func(o *traveloka.Offer) {
			object(o.Details["rate"])["bookingPolicy"] = map[string]any{"payment": "LATER"}
		}, 0, "no_opposite"},
		{"occupancy_mismatch", func(o *traveloka.Offer) { o.OccupancyMatch = ptr(false) }, 0, "mismatched_occupancy"},
		{"occupancy_unknown", func(o *traveloka.Offer) { o.OccupancyMatch = nil }, 0, "unknown_occupancy"},
		{"charged_party_mismatch", func(o *traveloka.Offer) { object(o.Details["rate"])["numChargedRooms"] = json.Number("2") }, 0, "mismatched_charged_rooms"},
		{"capacity_mismatch", func(o *traveloka.Offer) { object(o.Details["rate"])["maxOccupancy"] = json.Number("3") }, 0, "no_opposite"},
		{"child_capacity_mismatch", func(o *traveloka.Offer) { object(o.Details["rate"])["maxChildOccupancy"] = json.Number("1") }, 0, "no_opposite"},
		{"missing_payment_terms", func(o *traveloka.Offer) { delete(object(o.Details["rate"]), "bookingPolicy") }, 0, "unknown_bookingPolicy"},
		{"missing_capacity", func(o *traveloka.Offer) { delete(object(o.Details["rate"]), "maxOccupancy") }, 0, "unknown_maxOccupancy"},
		{"unknown_capacity_text", func(o *traveloka.Offer) { object(o.Details["rate"])["maxOccupancy"] = "UNKNOWN" }, 0, "unknown_maxOccupancy"},
		{"string_charged_rooms", func(o *traveloka.Offer) { object(o.Details["rate"])["numChargedRooms"] = "1" }, 1, ""},
		{"string_capacities", func(o *traveloka.Offer) {
			object(o.Details["rate"])["maxOccupancy"] = "2"
			object(o.Details["rate"])["maxChildOccupancy"] = "0"
		}, 1, ""},
		{"room_level_charged_rooms", func(o *traveloka.Offer) {
			delete(object(o.Details["rate"]), "numChargedRooms")
			o.Details["room"] = map[string]any{"numChargedRooms": "1"}
		}, 1, ""},
		{"absent_charged_rooms", func(o *traveloka.Offer) { delete(object(o.Details["rate"]), "numChargedRooms") }, 0, "unknown_numChargedRooms"},
		{"null_rate_charge_not_inherited", func(o *traveloka.Offer) {
			object(o.Details["rate"])["numChargedRooms"] = nil
			o.Details["room"] = map[string]any{"numChargedRooms": "1"}
		}, 0, "unknown_numChargedRooms"},
		{"unknown_total", func(o *traveloka.Offer) { o.Price.Total = nil }, 0, "unknown_exact_stay_total"},
		{"different_room", func(o *traveloka.Offer) { o.RoomID = "SIMULATED-other-room" }, 0, "no_opposite"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			a, b := room("flexible", true, "9007199254740993"), room("fixed", false, "9007199254740992")
			tt.edit(&b)
			r, e := Flexibility(roomSnapshot(a, b), 100, 1000)
			if e != nil {
				t.Fatal(e)
			}
			if r.PairCount != tt.pairs || len(r.Pairs) != tt.pairs || r.Scanned != 2 {
				t.Fatalf("pairing wrong: %+v", r)
			}
			if tt.pairs == 1 {
				p := r.Pairs[0]
				if p.Difference.Amount != "0.01" || p.Difference.MinorUnits != "1" || p.Refundable.Cancellation["timezone_known"] != false {
					t.Fatalf("lost exact difference or cancellation unknown: %+v", p)
				}
			} else {
				found := false
				for _, issue := range r.Unpaired {
					if strings.Contains(issue.Reason, tt.reason) {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing reason %s: %+v", tt.reason, r)
				}
			}
		})
	}
	allRefundable := roomSnapshot(room("one", true, "10000"), room("two", true, "12000"))
	r, e := Flexibility(allRefundable, 20, 500)
	if e != nil || r.PairCount != 0 || len(r.Unpaired) != 2 {
		t.Fatal("all-refundable rates invented an opposite policy")
	}
	badParty := roomSnapshot(room("one", true, "100"), room("two", false, "100"))
	badParty.Query.Children = 1
	r, e = Flexibility(badParty, 20, 500)
	if e != nil || r.PairCount != 0 || len(r.Unpaired) != 2 || r.Unpaired[0].Reason != "unknown_query_occupancy_or_stay" {
		t.Fatal("incomplete child party was inferred")
	}
}
func TestFlexibilityBoundsSimulated(t *testing.T) {
	s := roomSnapshot(room("a", true, "100"), room("b", true, "100"), room("c", false, "90"), room("d", false, "90"))
	r, e := Flexibility(s, 1, 500)
	if e != nil || r.PairCount != 4 || len(r.Pairs) != 1 || !r.Truncated {
		t.Fatalf("pair bounds lost: %+v %v", r, e)
	}
}
func TestFlexibilityChargedRoomIntegerFormsSimulated(t *testing.T) {
	cases := []struct {
		name  string
		value any
		valid bool
	}{
		{"source_string_two", "2", true}, {"source_json_number_two", json.Number("2"), true}, {"source_unknown", "UNKNOWN", false}, {"source_mismatch", "1", false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			a, b := room("SIMULATED-refundable", true, "101"), room("SIMULATED-nonrefundable", false, "100")
			object(a.Details["rate"])["numChargedRooms"] = json.Number("2")
			object(b.Details["rate"])["numChargedRooms"] = tt.value
			s := roomSnapshot(a, b)
			s.Query.Rooms = 2
			r, e := Flexibility(s, 20, 500)
			if e != nil {
				t.Fatal(e)
			}
			if (r.PairCount == 1) != tt.valid {
				t.Fatalf("exact string/number occupancy handling wrong: %+v", r)
			}
		})
	}
}

func TestDiffSimulated(t *testing.T) {
	cases := []struct {
		name                                                    string
		edit                                                    func(*traveloka.Offer)
		changes, notReturned, newReturned, unmatched, unchanged int
	}{
		{"price_change", func(o *traveloka.Offer) { o.Price.Total = money("10001", 2) }, 1, 0, 0, 0, 0},
		{"policy_change", func(o *traveloka.Offer) { o.Refundable = ptr(true) }, 1, 0, 0, 0, 0},
		{"baggage_change", func(o *traveloka.Offer) { o.Legs[0].Segments[0].CheckedBaggage = "SIMULATED-20kg" }, 1, 0, 0, 0, 0},
		{"unchanged_empty_diff", func(o *traveloka.Offer) {}, 0, 0, 0, 0, 1},
		{"new_id_not_guessed", func(o *traveloka.Offer) { o.ID = "SIMULATED-new-source-id" }, 0, 1, 1, 0, 0},
		{"missing_source_id", func(o *traveloka.Offer) { o.ID = "" }, 0, 1, 0, 1, 0},
		{"missing_leg_identity", func(o *traveloka.Offer) { delete(o.Legs[0].Details, "id") }, 0, 1, 0, 1, 0},
		{"missing_offset", func(o *traveloka.Offer) { o.Legs[0].Segments[0].DepartureUTCOffsetMinutes = nil }, 0, 1, 0, 1, 0},
		{"changed_scale_preserved", func(o *traveloka.Offer) { o.Price.Total = money("100000", 3) }, 1, 0, 0, 0, 0},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			a := flight("SIMULATED-source-id", "10000", 0, 100)
			b := copyOffer(a)
			tt.edit(&b)
			before, after := flightSnapshot(a), flightSnapshot(b)
			after.ID = "SIMULATED-later"
			after.RetrievedAt = "2026-10-02T01:00:00Z"
			r, e := Diff(before, after, 100, 1000)
			if e != nil {
				t.Fatal(e)
			}
			if len(r.Changes) != tt.changes || len(r.NotReturned) != tt.notReturned || len(r.NewReturned) != tt.newReturned || len(r.UnmatchedAfter) != tt.unmatched || r.Unchanged != tt.unchanged {
				t.Fatalf("diff wrong: %+v", r)
			}
			if tt.name == "price_change" && r.Changes[0].Difference.MinorUnits != "1" {
				t.Fatal("price change rounded")
			}
			if tt.name == "changed_scale_preserved" && (r.Changes[0].Difference.Amount != "0.000" || *r.Changes[0].After.Price.Total.Decimals != 3) {
				t.Fatal("source scale lost")
			}
			encoded, _ := json.Marshal(r)
			if strings.Contains(string(encoded), "sold out") {
				t.Fatal("availability invented")
			}
		})
	}
	a, b := room("SIMULATED-inventory-old", true, "100"), room("SIMULATED-inventory-new", true, "100")
	r, e := Diff(roomSnapshot(a), roomSnapshot(b), 20, 500)
	if e != nil || len(r.Changes) != 0 || len(r.NotReturned) != 1 || len(r.NewReturned) != 1 {
		t.Fatal("room UUID equivalence guessed from stable-looking group/name")
	}
	duplicate := flight("duplicate", "100", 0, 100)
	r, e = Diff(flightSnapshot(duplicate, duplicate), flightSnapshot(duplicate), 20, 500)
	if e != nil || r.Matched != 0 || len(r.UnmatchedBefore) != 2 || len(r.UnmatchedAfter) != 1 {
		t.Fatal("duplicate source identity collapsed")
	}
}
func TestDiffRejectsContextCurrencyUnitsSimulated(t *testing.T) {
	cases := []struct {
		name string
		edit func(*traveloka.Snapshot)
	}{
		{"different_party", func(s *traveloka.Snapshot) { s.Query.Adults = 2 }},
		{"different_currency", func(s *traveloka.Snapshot) { s.Query.Currency = "USD" }},
		{"offer_currency", func(s *traveloka.Snapshot) { s.Offers[0].Price.Total.Currency = "USD" }},
		{"different_price_unit", func(s *traveloka.Snapshot) { s.Offers[0].Details["price_basis"] = "per_passenger" }},
		{"invalid_scale", func(s *traveloka.Snapshot) { s.Offers[0].Price.Total.Decimals = ptr(19) }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			before, after := flightSnapshot(flight("a", "100", 0, 100)), flightSnapshot(flight("a", "100", 0, 100))
			tt.edit(after)
			if _, e := Diff(before, after, 20, 500); e == nil {
				t.Fatal("incompatible comparison accepted")
			}
		})
	}
}
func TestDiffMeaningfulPoliciesExcludeSupplierNoiseSimulated(t *testing.T) {
	cases := []struct {
		name    string
		edit    func(*traveloka.Offer)
		changed bool
	}{
		{"supplier_source_lut_only", func(o *traveloka.Offer) { object(o.Legs[0].Segments[0].CheckedBaggage)["source"] = "SIMULATED-LUT:999" }, false},
		{"baggage_icon_only", func(o *traveloka.Offer) {
			object(o.Legs[0].Segments[0].CheckedBaggage)["iconUrl"] = "https://www.traveloka.com/SIMULATED-new-icon.png"
		}, false},
		{"aircraft_details_only", func(o *traveloka.Offer) { o.Legs[0].Segments[0].Details["aircraftType"] = "SIMULATED-new-aircraft" }, false},
		{"quantity_change", func(o *traveloka.Offer) { object(o.Legs[0].Segments[0].CheckedBaggage)["quantity"] = "2" }, true},
		{"weight_change", func(o *traveloka.Offer) { object(o.Legs[0].Segments[0].CheckedBaggage)["weight"] = "25" }, true},
		{"restriction_change", func(o *traveloka.Offer) { o.Legs[0].Segments[0].Details["mayReCheckIn"] = true }, true},
		{"refund_restriction_change", func(o *traveloka.Offer) {
			route := o.Legs[0].FareRules["routes"].([]any)[0].(map[string]any)
			object(route["flightRefundInfo"])["refundableStatus"] = "YES"
		}, true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			a := flight("SIMULATED-source-id", "10000", 0, 100)
			a.Legs[0].Segments[0].CheckedBaggage = map[string]any{"quantity": "1", "weight": "20", "unitOfMeasure": "KG", "source": "SIMULATED-LUT:111", "iconUrl": "https://www.traveloka.com/SIMULATED-icon.png"}
			a.Legs[0].Segments[0].Details = map[string]any{"aircraftType": "SIMULATED-aircraft", "mayReCheckIn": false}
			a.Legs[0].FareRules = map[string]any{"routes": []any{map[string]any{"flightRefundInfo": map[string]any{"refundableStatus": "NO", "policyDetails": []any{}}}}}
			b := copyOffer(a)
			tt.edit(&b)
			r, e := Diff(flightSnapshot(a), flightSnapshot(b), 20, 500)
			if e != nil {
				t.Fatal(e)
			}
			if tt.changed {
				if len(r.Changes) != 1 || r.Changes[0].Fields["policies"].Before == nil {
					t.Fatalf("meaningful restriction/allowance change lost: %+v", r)
				}
			} else if len(r.Changes) != 0 || r.Unchanged != 1 {
				t.Fatalf("supplier/icon metadata mislabeled policy: %+v", r)
			}
			if tt.changed && r.Changes[0].Before.Legs[0].Segments[0].CheckedBaggage.(map[string]any)["source"] != "SIMULATED-LUT:111" {
				t.Fatal("full original offer was projected or altered")
			}
		})
	}
}

func TestDiffHotelCatalogPoliciesSimulated(t *testing.T) {
	cases := []struct {
		name string
		edit func(map[string]any)
	}{
		{"cancellation_availability", func(m map[string]any) { m["hasFreeCancellationRooms"] = false }},
		{"payment_types", func(m map[string]any) { m["availableRateTypes"] = []any{"PAY_AT_PROPERTY"} }},
		{"credit_guarantee", func(m map[string]any) { m["hotelRoomCCGuaranteeRequirementDisplay"] = "Credit card required" }},
		{"occupancy_match", func(m map[string]any) { m["matchSearchOccupancy"] = false }},
		{"charged_rooms", func(m map[string]any) { m["numChargedRooms"] = "2" }},
		{"known_to_null", func(m map[string]any) { m["hasFreeCancellationRooms"] = nil }},
		{"null_to_absent", func(m map[string]any) { delete(m, "hotelRoomCCGuaranteeRequirementDisplay") }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			a := traveloka.Offer{ID: "SIMULATED-property", Kind: "hotel", PropertyID: "SIMULATED-property", OccupancyMatch: ptr(true), Price: traveloka.Price{Total: money("21165", 2), TaxInclusion: "inclusive"}, Details: map[string]any{"price_basis": "party_stay_total", "inventory": map[string]any{
				"matchSearchOccupancy": true, "numChargedRooms": "1", "availableRateTypes": []any{"PAY_NOW"}, "hasFreeCancellationRooms": true, "hotelRoomCCGuaranteeRequirementDisplay": nil,
			}}}
			b := copyOffer(a)
			tt.edit(object(b.Details["inventory"]))
			before, after := roomSnapshot(a), roomSnapshot(b)
			before.Kind, before.Query.Kind = "hotels", "hotels"
			after.Kind, after.Query.Kind = "hotels", "hotels"
			r, e := Diff(before, after, 20, 500)
			if e != nil || r.Matched != 1 || r.ChangeCount != 1 || r.Unchanged != 0 || len(r.Changes) != 1 {
				t.Fatalf("catalog policy change lost: %+v %v", r, e)
			}
			change := r.Changes[0]
			if change.Fields["policies"].Before == nil || change.Difference == nil || change.Difference.Amount != "0.00" || change.Before.Refundable != nil || change.After.Refundable != nil {
				t.Fatalf("catalog change implied a price/refund change or lost original terms: %+v", change)
			}
			if canonical(change.Before.Details["inventory"]) != canonical(a.Details["inventory"]) || canonical(change.After.Details["inventory"]) != canonical(b.Details["inventory"]) {
				t.Fatal("original catalog terms changed during comparison")
			}
		})
	}
}

func TestDiffRoomIncludedMealTypesSimulated(t *testing.T) {
	for _, changed := range []bool{false, true} {
		name := "unchanged"
		if changed {
			name = "added_meal"
		}
		t.Run(name, func(t *testing.T) {
			a := room("SIMULATED-same-rate", true, "21165")
			object(a.Details["rate"])["includedMealTypes"] = []any{"BREAKFAST"}
			b := copyOffer(a)
			if changed {
				object(b.Details["rate"])["includedMealTypes"] = []any{"BREAKFAST", "DINNER"}
			}
			r, e := Diff(roomSnapshot(a), roomSnapshot(b), 20, 500)
			if e != nil || r.Matched != 1 {
				t.Fatalf("exact room inventory comparison failed: %+v %v", r, e)
			}
			if !changed {
				if r.ChangeCount != 0 || r.Unchanged != 1 {
					t.Fatalf("unchanged source meals became a policy change: %+v", r)
				}
				return
			}
			if r.ChangeCount != 1 || len(r.Changes) != 1 || r.Unchanged != 0 {
				t.Fatalf("meal-type-only policy change was lost: %+v", r)
			}
			change := r.Changes[0]
			if change.Fields["policies"].Before == nil || change.Difference == nil || change.Difference.Amount != "0.00" || change.Before.MealPlan != change.After.MealPlan {
				t.Fatal("meal-policy change altered labels or invented a price difference")
			}
			if canonical(object(change.Before.Details["rate"])["includedMealTypes"]) != canonical([]any{"BREAKFAST"}) || canonical(object(change.After.Details["rate"])["includedMealTypes"]) != canonical([]any{"BREAKFAST", "DINNER"}) {
				t.Fatal("original before/after included meal types were discarded")
			}
		})
	}
}

func TestDiffTaxInclusionCompatibilitySimulated(t *testing.T) {
	cases := []struct {
		name, before, after string
		comparable          bool
	}{
		{"exclusive_to_inclusive", "exclusive", "inclusive", false},
		{"inclusive_to_exclusive", "inclusive", "exclusive", false},
		{"unknown_to_inclusive", "unknown", "inclusive", false},
		{"inclusive_to_unknown", "inclusive", "unknown", false},
		{"absent_to_inclusive", "", "inclusive", false},
		{"same_inclusive", "inclusive", "inclusive", true},
		{"same_exclusive", "exclusive", "exclusive", true},
		{"same_source_unknown", "unknown", "unknown", true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			a, b := room("SIMULATED-rate", true, "17982"), room("SIMULATED-rate", true, "21165")
			a.Price.TaxInclusion, b.Price.TaxInclusion = tt.before, tt.after
			r, e := Diff(roomSnapshot(a), roomSnapshot(b), 20, 500)
			if e != nil || len(r.Changes) != 1 || r.Matched != 1 {
				t.Fatalf("price/basis change lost: %+v %v", r, e)
			}
			change := r.Changes[0]
			if change.Before.Price.TaxInclusion != tt.before || change.After.Price.TaxInclusion != tt.after || change.Fields["price"].Before == nil {
				t.Fatal("original differing tax bases were discarded")
			}
			if tt.comparable {
				if change.Difference == nil || change.Difference.Amount != "31.83" || change.DifferenceUnavailableReason != "" {
					t.Fatalf("comparable source totals lost their exact delta: %+v", change)
				}
			} else if change.Difference != nil || change.DifferenceUnavailableReason != "incompatible_tax_inclusion" {
				t.Fatalf("incomparable tax bases became a numeric price delta: %+v", change)
			}
		})
	}
}
