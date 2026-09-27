package ikyu

import (
	"testing"
	"time"
)

func pointer[T any](v T) *T { return &v }
func validStay() Stay {
	return Stay{CheckIn: "2026-10-18", CheckOut: "2026-10-19", Adults: 2, Rooms: 1}
}
func TestValidateStayAndCanonical(t *testing.T) {
	now := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		s    Stay
		bad  bool
	}{{"valid", validStay(), false}, {"calendar", Stay{CheckIn: "2026-13-40", CheckOut: "2026-13-41", Adults: 2, Rooms: 1}, true}, {"zero adults", Stay{CheckIn: "2026-10-18", CheckOut: "2026-10-19", Adults: 0, Rooms: 1}, true}, {"too far", Stay{CheckIn: "2027-10-19", CheckOut: "2027-10-20", Adults: 2, Rooms: 1}, true}, {"zero nights", Stay{CheckIn: "2026-10-18", CheckOut: "2026-10-18", Adults: 2, Rooms: 1}, true}, {"two rooms", Stay{CheckIn: "2026-10-18", CheckOut: "2026-10-19", Adults: 2, Rooms: 2}, false}} {
		t.Run(tc.name, func(t *testing.T) {
			e := ValidateStay(tc.s, now)
			if (e != nil) != tc.bad {
				t.Fatalf("validation %v", e)
			}
			_, e = CanonicalURL("00002889", "10193741", "11055986", &tc.s)
			if tc.name != "too far" && (e != nil) != tc.bad {
				t.Fatalf("canonical validation %v", e)
			}
		})
	}
	s := validStay()
	s.Rooms = 2
	s.Children[4] = 1
	u, e := CanonicalURL("00002889", "10193741", "11055986", &s)
	if e != nil || u != "https://www.ikyu.com/00002889/11055986/10193741/?cec=1&cid=20261018&cod=20261019&lc=1&ppc=2&rc=2" {
		t.Fatalf("canonical %s %v", u, e)
	}
}
func TestBathEvidenceDoesNotInferPropertyOnsen(t *testing.T) {
	for _, tc := range []struct {
		name         string
		a            []Attribute
		outdoor, hot *bool
	}{{"unknown", nil, nil, nil}, {"outdoor only", []Attribute{{Value: "18", Name: "露天風呂付"}}, pointer(true), nil}, {"hot outdoor", []Attribute{{Value: "16", Name: "温泉露天風呂付"}}, pointer(true), pointer(true)}, {"property hot spring code", []Attribute{{Value: "9", Name: "温泉"}}, nil, nil}} {
		t.Run(tc.name, func(t *testing.T) {
			b := bathEvidence(tc.a)
			equal := func(a, b *bool) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }
			if !equal(b.Outdoor, tc.outdoor) || !equal(b.HotSpring, tc.hot) {
				t.Fatalf("bad evidence %+v", b)
			}
		})
	}
}
func TestPriceExactYenFractionalRatesAndEcho(t *testing.T) {
	a := rawAmount{Amount: pointer(int64(30800)), BaseDiscountAmount: pointer(int64(30800)), DiscountAmount: pointer(int64(24640)), InstantPoint: pointer(int64(6160)), PointRate: pointer(2.5), Adults: pointer(2), Rooms: pointer(1), Nights: pointer(1), A: pointer(0), B: pointer(0), C: pointer(0), D: pointer(0), E: pointer(0), F: pointer(0), Details: []struct {
		Date string `json:"date"`
	}{{"2026-10-18"}}}
	p := normalizePrice(a)
	if *p.Amount != 30800 || *p.DiscountAmount != 24640 || *p.InstantPoint != 6160 || *p.PointRate != 2.5 || p.CheckoutConfirmedPayable != nil {
		t.Fatalf("price altered %+v", p)
	}
	if e := verifyAmount(a, validStay(), true); e != nil {
		t.Fatal(e)
	}
	a.Adults = pointer(3)
	if e := verifyAmount(a, validStay(), true); e == nil {
		t.Fatal("source adults normalization accepted")
	}
	a.Adults = pointer(2)
	a.Details[0].Date = "2026-10-20"
	if e := verifyAmount(a, validStay(), true); e == nil {
		t.Fatal("source date normalization accepted")
	}
}
func TestNormalizeIdentityAndRoomRange(t *testing.T) {
	if _, e := normalizeProperty(&rawProperty{ID: "00002889"}); e == nil {
		t.Fatal("missing name accepted")
	}
	r, e := normalizeRoom(rawRoom{ID: "10193741", Name: "room", MeterFrom: pointer(40.0), MeterTo: pointer(46.0), Presentation: "渓流を臨むテラス。"}, "00002889", nil)
	if e != nil || r.SizeMaxM2 == nil || *r.SizeMaxM2 != 46 || len(r.ViewEvidence) != 1 || r.Bath.HotSpring != nil {
		t.Fatalf("room evidence %+v %v", r, e)
	}
}
