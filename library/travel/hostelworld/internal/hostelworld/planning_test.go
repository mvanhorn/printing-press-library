package hostelworld

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

var fixtureNow = time.Date(2026, 10, 3, 6, 0, 0, 0, time.UTC)

func fixture(t *testing.T, name string) map[string]any {
	t.Helper()
	data, e := os.ReadFile("testdata/" + name + ".json")
	if e != nil {
		t.Fatal(e)
	}
	v, e := Decode(data)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func firstOffer(t *testing.T, v map[string]any) map[string]any {
	t.Helper()
	rows := v["offers"].([]any)
	if len(rows) == 0 {
		t.Fatal("missing offers")
	}
	return rows[0].(map[string]any)
}
func TestAvailabilityPriceUnits(t *testing.T) {
	for _, tc := range []struct {
		name, kind, basis, nightly, party string
		qty, guests                       int
	}{{"dorm", "dorm", "per_bed_per_stay", "per_bed_per_night", "33820.00", 2, 2}, {"private", "private", "per_room_per_stay", "per_occupancy_slot_per_night", "44808.48", 1, 2}, {"private", "private", "per_room_per_stay", "per_occupancy_slot_per_night", "89616.96", 2, 5}} {
		t.Run(tc.name+tc.party, func(t *testing.T) {
			q, e := NewQuery("2026-10-04", "2026-10-07", 0, tc.guests, fixtureNow)
			if e != nil {
				t.Fatal(e)
			}
			v, e := Availability(fixture(t, tc.name), fixture(t, "property"), q, tc.kind, false, fixtureNow)
			if e != nil {
				t.Fatal(e)
			}
			o := firstOffer(t, v)
			if object(o["source_stay_amount"])["unit"] != tc.basis || o["nightly_price_basis"] != tc.nightly || o["required_quantity"] != tc.qty || object(o["derived_party_estimate"])["value"] != tc.party {
				t.Fatalf("wrong quantities/units: %#v", o)
			}
			if tc.name == "dorm" && object(o["source_stay_amount"])["value"] != "16910.00" {
				t.Fatal("per-bed source stay amount was replaced with party amount")
			}
			if tc.name == "private" && o["guests_per_private_room"] == nil {
				t.Fatal("private occupancy was lost")
			}
		})
	}
}
func TestAvailabilityUnknownAndRestrictions(t *testing.T) {
	for _, tc := range []struct {
		name       string
		change     func(map[string]any)
		basis, fit string
	}{{"missing-night", func(v map[string]any) {
		r := object(array(object(v["rooms"])["dorms"])[0])
		r["priceBreakdown"] = array(r["priceBreakdown"])[1:]
	}, "unknown", "available"}, {"insufficient-beds", func(v map[string]any) {
		object(array(object(v["rooms"])["dorms"])[0])["totalBedsAvailable"] = json.Number("1")
	}, "per_bed_per_night", "insufficient_inventory"}, {"missing-inventory", func(v map[string]any) { object(array(object(v["rooms"])["dorms"])[0])["totalBedsAvailable"] = nil }, "per_bed_per_night", "unknown"}, {"rate-restriction", func(v map[string]any) {
		room := object(array(object(v["rooms"])["dorms"])[0])
		object(array(room["ratePlans"])[0])["rateRuleViolations"] = []any{"minimum stay"}
	}, "per_bed_per_night", "restricted"}} {
		t.Run(tc.name, func(t *testing.T) {
			v := fixture(t, "dorm")
			tc.change(v)
			q, _ := NewQuery("2026-10-04", "2026-10-07", 0, 2, fixtureNow)
			out, e := Availability(v, fixture(t, "property"), q, "dorm", false, fixtureNow)
			if e != nil {
				t.Fatal(e)
			}
			o := firstOffer(t, out)
			if o["nightly_price_basis"] != tc.basis || o["party_fit"] != tc.fit {
				t.Fatalf("wrong unknown/restriction: %#v", o)
			}
			if tc.fit != "available" && object(o["derived_party_estimate"]) != nil {
				t.Fatal("restricted or unknown inventory acquired party estimate")
			}
		})
	}
	for _, name := range []string{"missing-rooms", "missing-dorm-array", "missing-source-price"} {
		t.Run(name, func(t *testing.T) {
			v := fixture(t, "dorm")
			switch name {
			case "missing-rooms":
				delete(v, "rooms")
			case "missing-dorm-array":
				delete(object(v["rooms"]), "dorms")
			default:
				object(array(object(v["rooms"])["dorms"])[0])["totalPrice"] = nil
			}
			q, _ := NewQuery("2026-10-04", "2026-10-07", 0, 2, fixtureNow)
			if _, e := Availability(v, fixture(t, "property"), q, "all", false, fixtureNow); e == nil {
				t.Fatal("schema drift passed as unavailable")
			}
		})
	}
}
func TestCancellationStatus(t *testing.T) {
	for _, tc := range []struct {
		name, deadline, status string
		availabilityFlag       any
		rateFlag               any
		rateType, payment      string
	}{
		{"explicit-rate", "2026-10-03T23:59:59+09:00", "available", true, true, "STANDARD", "Fully refundable before the deadline"},
		{"availability-only", "2026-10-03T23:59:59+09:00", "unknown", true, nil, "STANDARD", "Deposit only"},
		{"actual-conditional-deposit", "2026-10-03T23:59:59+09:00", "conditional", true, nil, "STANDARD", "Your deposit will be non-refundable unless the Standard Flexible Booking option is available and you select it."},
		{"nonrefund-after-display-cap", "2026-10-03T23:59:59+09:00", "unavailable", true, true, "STANDARD", strings.Repeat("Policy detail. ", 200) + "Your deposit is non-refundable."},
		{"nonrefundable-payment-overrides-rate", "2026-10-03T23:59:59+09:00", "unavailable", true, true, "STANDARD", "Your deposit is non-refundable."},
		{"expired", "2026-10-03T00:00:00+09:00", "expired", true, true, "STANDARD", "Refundable"},
		{"missing-deadline", "", "unknown", true, true, "STANDARD", "Refundable"},
		{"unavailable", "", "unavailable", false, nil, "STANDARD", "Deposit only"},
		{"conflicting-source-flags", "2026-10-03T23:59:59+09:00", "unknown", false, true, "STANDARD", "Refundable"},
		{"nonrefund-rate", "2026-10-03T23:59:59+09:00", "unavailable", true, true, "NONREFUNDABLE", "Refundable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := fixture(t, "dorm")
			v["freeCancellationAvailable"] = tc.availabilityFlag
			v["freeCancellationAvailableUntil"] = tc.deadline
			for _, p := range array(object(array(object(v["rooms"])["dorms"])[0])["ratePlans"]) {
				plan := object(p)
				plan["ratePlanType"] = tc.rateType
				plan["freeCancellationAvailable"] = tc.rateFlag
				plan["paymentProcedure"] = map[string]any{"description": tc.payment}
			}
			q, _ := NewQuery("2026-10-04", "2026-10-07", 0, 2, fixtureNow)
			out, e := Availability(v, fixture(t, "property"), q, "all", false, fixtureNow)
			if e != nil {
				t.Fatal(e)
			}
			o := firstOffer(t, out)
			if o["free_cancellation_status"] != tc.status || o["free_cancellation_deadline_scope"] != "availability_response" || out["source_availability_free_cancellation_available"] != tc.availabilityFlag {
				t.Fatalf("wrong cancellation evidence: %#v", o)
			}
			out, e = Availability(v, fixture(t, "property"), q, "all", true, fixtureNow)
			if e != nil {
				t.Fatal(e)
			}
			if (len(out["offers"].([]any)) > 0) != (tc.status == "available") {
				t.Fatal("free cancellation filter guessed conditional/missing/expired terms")
			}
		})
	}
}

func TestQueryUsesUnknownSourceLocalCalendar(t *testing.T) {
	for _, tc := range []struct {
		now, start string
		ok         bool
	}{
		{"2026-10-03T00:30:00Z", "2026-10-02", true},
		{"2026-10-03T00:30:00Z", "2026-10-01", false},
		{"2026-10-03T20:30:00Z", "2026-10-02", false},
		{"2026-10-03T20:30:00Z", "2026-10-03", true},
	} {
		now, _ := time.Parse(time.RFC3339, tc.now)
		_, err := NewQuery(tc.start, "", 1, 2, now)
		if (err == nil) != tc.ok {
			t.Fatalf("UTC %s, source-local check-in %s: %v", tc.now, tc.start, err)
		}
	}
}

func TestQueryValidation(t *testing.T) {
	for _, tc := range []struct {
		start, end     string
		nights, guests int
		ok             bool
	}{{"2026-10-04", "2026-10-07", 0, 2, true}, {"2026-10-04", "", 3, 2, true}, {"bad", "", 3, 2, false}, {"2026-10-01", "", 3, 2, false}, {"2026-10-04", "2026-10-04", 0, 2, false}, {"2026-10-04", "2026-10-07", 2, 2, false}, {"2026-10-04", "", 31, 2, false}, {"2026-10-04", "", 3, 0, false}} {
		q, e := NewQuery(tc.start, tc.end, tc.nights, tc.guests, fixtureNow)
		if (e == nil) != tc.ok {
			t.Fatalf("query %v error=%v", tc, e)
		}
		if tc.ok {
			if q.Params()["guests"] != "2" || q.CheckOut != "2026-10-07" {
				t.Fatal("query dates/party were not serialized")
			}
		}
	}
}
func TestFactsAndHelpers(t *testing.T) {
	p, e := Property(fixture(t, "property"), fixtureNow)
	if e != nil {
		t.Fatal(e)
	}
	if p["id"] != "67481" || p["check_out"] != "11:00" || len(p["source_rules"].([]string)) == 0 {
		t.Fatal("missing property rules")
	}
	if _, e := Property(map[string]any{}, fixtureNow); e == nil {
		t.Fatal("property schema drift accepted")
	}
	for _, tc := range []struct {
		id string
		ok bool
	}{{"67481", true}, {"0", false}, {"../../1", false}, {"123456789012", false}} {
		if ValidID(tc.id) != tc.ok {
			t.Fatal("bad ID validation")
		}
	}
	for _, tc := range []struct {
		v    any
		want string
	}{{"8", "8"}, {json.Number("8"), "8"}, {float64(8), "8"}, {8, "8"}, {nil, ""}} {
		if Text(tc.v) != tc.want {
			t.Fatal("ID type normalization changed")
		}
	}
	if Clean("<b>Deposit only</b><br> T&amp;C", 100) != "Deposit only T&C" {
		t.Fatalf("bad HTML cleanup %q", Clean("<b>Deposit only</b><br> T&amp;C", 100))
	}
	if _, e := Decode([]byte("[]")); e == nil {
		t.Fatal("array accepted as object")
	}
	q, _ := NewQuery("2026-10-04", "", 3, 2, fixtureNow)
	u := BookingURL("67481", "Nui. Hostel & Bar Lounge", "Tokyo", q)
	if !strings.Contains(u, "guests=2") || !strings.Contains(u, "from=2026-10-04") || !strings.Contains(u, "to=2026-10-07") {
		t.Fatal("booking handoff lost query")
	}
	for _, tc := range []struct {
		a, b string
		want bool
	}{{"10.00", "20.00", true}, {"20.00", "10.00", false}} {
		a := map[string]any{"derived_party_estimate": map[string]any{"value": tc.a, "currency": "JPY"}}
		b := map[string]any{"derived_party_estimate": map[string]any{"value": tc.b, "currency": "JPY"}}
		if CompareAmounts(a, b) != tc.want {
			t.Fatal("party sorting changed")
		}
	}
}

func TestCanonicalListingRequiresAProviderRouteLabel(t *testing.T) {
	for _, tc := range []struct{ id, name, want string }{
		{"67481", "Nui. Hostel & Bar Lounge", "https://www.hostelworld.com/hostels/p/67481/nui-hostel-and-bar-lounge/"},
		{"15725", "Sakura Hostel Asakusa", "https://www.hostelworld.com/hostels/p/15725/sakura-hostel-asakusa/"},
	} {
		if got := canonical(tc.id, tc.name); got != tc.want {
			t.Fatalf("canonical=%s, want observed source URL %s", got, tc.want)
		}
	}
}
