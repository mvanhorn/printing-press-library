package pocket

import "testing"

func intp(i int) *int    { return &i }
func boolp(b bool) *bool { return &b }
func TestSourceIdentityAndBookingMode(t *testing.T) {
	for _, tc := range []struct {
		b    *bool
		want any
	}{{nil, nil}, {boolp(true), "instant_confirmation"}, {boolp(false), "reservation_request"}} {
		if got := BookingMode(tc.b); got != tc.want {
			t.Fatalf("mode %v", got)
		}
	}
	for _, id := range []string{"245672", "00012"} {
		if ValidateID(id) != nil {
			t.Fatal(id)
		}
		if URL(id, "en") != "https://www.pocket-concierge.jp/en/restaurants/"+id {
			t.Fatal("canonical URL changed identity")
		}
	}
	for _, id := range []string{"", "../x", "245672?", "1234567890123"} {
		if ValidateID(id) == nil {
			t.Fatal("accepted invalid ID", id)
		}
	}
}
func TestSessionStatesPartyAndMissingIdentity(t *testing.T) {
	course := &Course{ID: "182402", Name: "Monthly"}
	v := Venue{ID: "245672", RealTimeBooking: boolp(false)}
	for _, tc := range []struct {
		name     string
		s        Slot
		party    int
		want     any
		eligible bool
		error    bool
	}{{"request", Slot{Type: "ReservableAvailability", ID: ptr("6871760"), StartTime: "2026-10-05T18:00:00+09:00", MinPartySize: intp(2), MaxPartySize: intp(6), Course: course}, 2, "reservation_request", true, false}, {"waitlist", Slot{Type: "WaitlistableAvailability", StartTime: "2026-10-03T18:00:00+09:00", MinPartySize: intp(2), MaxPartySize: intp(6), Course: course}, 7, "waitlist", false, false}, {"missing bounds", Slot{Type: "WaitlistableAvailability", StartTime: "2026-10-03T18:00:00+09:00", Course: course}, 2, "waitlist", true, false}, {"missing id", Slot{Type: "ReservableAvailability", StartTime: "2026-10-03T18:00:00+09:00", Course: course}, 2, nil, true, true}, {"unknown union", Slot{Type: "OtherAvailability", StartTime: "2026-10-03T18:00:00+09:00", Course: course}, 2, nil, true, true}, {"missing course", Slot{Type: "WaitlistableAvailability", StartTime: "2026-10-03T18:00:00+09:00"}, 2, nil, true, true}} {
		t.Run(tc.name, func(t *testing.T) {
			ok, e := PartyEligible(tc.s, tc.party)
			if e != nil || ok != tc.eligible {
				t.Fatalf("party %v %v", ok, e)
			}
			got, e := SlotView(tc.s, v, nil, tc.party, "en")
			if (e != nil) != tc.error {
				t.Fatalf("error %v", e)
			}
			if e == nil {
				if got["status"] != tc.want {
					t.Fatal(got)
				}
				if tc.s.ID == nil && got["session_id"] != (*string)(nil) {
					t.Fatal("invented waitlist ID")
				}
				if tc.s.MinPartySize == nil && got["party_eligible"] != nil {
					t.Fatal("invented party suitability")
				}
			}
		})
	}
}
func TestPriceEvidenceAndCalendarValidation(t *testing.T) {
	c := Course{ID: "1", Name: "Omakase", CostPerGuest: intp(21000), FixedPrice: intp(1000), Summary: ptr("Tax and service charges are included.\nNo additional charge unless additional order is made.")}
	view := CourseView(c, nil, "245672", "en")
	p := view["price"].(map[string]any)
	if p["per_guest"] != c.CostPerGuest || p["fixed_per_group"] != c.FixedPrice || p["all_in_total"] != nil {
		t.Fatal(p)
	}
	if len(view["fee_statements"].([]map[string]string)) != 2 {
		t.Fatal("fee evidence lost")
	}
	for _, d := range []string{"2026-10-01", "2028-02-29"} {
		if ValidateDate(d) != nil {
			t.Fatal(d)
		}
	}
	for _, d := range []string{"2026-02-29", "2026-10-00", "", "2026-1-1"} {
		if ValidateDate(d) == nil {
			t.Fatal(d)
		}
	}
	if _, e := PartyEligible(Slot{MinPartySize: intp(4), MaxPartySize: intp(2)}, 2); e == nil {
		t.Fatal("accepted reversed bounds")
	}
}
func TestConditionsRequireExplicitEvidence(t *testing.T) {
	r := ConditionEvidence(nil, nil)
	if r["dietary"] != nil || r["children"] != nil || r["language"] != nil {
		t.Fatal("invented conditions")
	}
	r = ConditionEvidence(ptr("Children aged 12 and over.\nAllergies whenever possible.\nEnglish page does not guarantee service."), []string{"english-menu"})
	if len(r["children"].([]map[string]string)) != 1 || len(r["language"].([]map[string]string)) != 2 {
		t.Fatal(r)
	}
}
func TestSummaryUnknownCollectionsRemainNull(t *testing.T) {
	v := Venue{ID: "1", Name: "Example"}
	j := Summary(v, nil, "en")
	if j["cuisines"].([]Label) != nil || j["price_ranges"].([]map[string]any) != nil {
		t.Fatal("unknown source collections changed")
	}
	v.Cuisines = []Label{}
	v.PriceRanges = []PriceRange{}
	j = Summary(v, nil, "en")
	if j["cuisines"].([]Label) == nil || j["price_ranges"].([]map[string]any) == nil {
		t.Fatal("known empty source collections changed")
	}
}
func TestKnownPartyBoundCanRuleOutUnknownBound(t *testing.T) {
	for _, s := range []Slot{{MinPartySize: nil, MaxPartySize: intp(1)}, {MinPartySize: intp(3), MaxPartySize: nil}} {
		if ok, e := PartyEligible(s, 2); e != nil || ok {
			t.Fatal("known incompatible bound ignored", s, e)
		}
	}
	if ok, e := PartyEligible(Slot{MaxPartySize: intp(6)}, 2); e != nil || !ok {
		t.Fatal(e)
	}
}
