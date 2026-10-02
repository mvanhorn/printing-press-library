package smartex

import (
	"math"
	"testing"
	"time"
)

func TestCalendarAndBoundaries(t *testing.T) {
	for _, tt := range []struct{ date, want string }{{"2026-05-31", "2026-05-01"}, {"2026-03-29", "2026-03-01"}, {"2028-03-29", "2028-02-29"}, {"2026-10-31", "2026-10-01"}, {"2026-12-31", "2026-12-01"}, {"2027-01-01", "2026-12-01"}} {
		d, _ := ParseDate(tt.date)
		if got := MonthBefore(d).Format("2006-01-02"); got != tt.want {
			t.Errorf("MonthBefore(%s)=%s want%s", tt.date, got, tt.want)
		}
	}
	now, _ := ParseNow("2026-10-01T00:00:00Z")
	w, e := BookingWindow("2026-10-31", "smart-ex", "09:00", now, 1, 0, false, "reserved")
	if e != nil || w.StandardSeatSales != "2026-10-01T10:00:00+09:00" || w.WindowStatus != "confirmation_processing_gap" {
		t.Fatalf("window=%+v error=%v", w, e)
	}
	if w.Confirmation != nil || len(w.ConfirmationSources) != 2 {
		t.Fatal("conflicting source times must remain explicit and actual confirmation unknown")
	}
	w, e = BookingWindow("2026-10-28", "hayatoku7", "", mustNow("2026-10-21T23:30:00+09:00"), 1, 0, false, "reserved")
	if e != nil || w.WindowStatus != "within_calendar_window" {
		t.Fatalf("deadline instant:%+v %v", w, e)
	}
	w, _ = BookingWindow("2026-10-28", "hayatoku7", "", mustNow("2026-10-21T23:30:01+09:00"), 1, 0, false, "reserved")
	if w.WindowStatus != "closed" {
		t.Fatal("after deadline must close")
	}
	for _, now := range []string{"2026-10-02T23:30:00+09:00", "2026-10-03T05:29:59+09:00"} {
		w, _ = BookingWindow("2026-10-28", "smart-ex", "", mustNow(now), 4, 0, false, "reserved")
		if !w.AfterHours || w.PartyFits != false || w.SeatMapAvailable {
			t.Fatal("afterhours restrictions missing")
		}
	}
	w, _ = BookingWindow("2026-10-28", "smart-ex", "", mustNow("2026-10-03T05:30:00+09:00"), 4, 0, true, "reserved")
	if w.AfterHours || w.PartyFits != true || w.OneYearRequestEligible != nil || w.Opens != nil || w.StandardSeatSales != "2026-09-28T10:00:00+09:00" {
		t.Fatalf("oversized window:%+v", w)
	}
	w, _ = BookingWindow("2028-02-29", "smart-ex", "", mustNow("2027-02-28T12:00:00+09:00"), 1, 0, false, "reserved")
	if w.Opens != nil {
		t.Fatal("undocumented Feb29 annual rule should be unknown")
	}
	if _, e = ParseDate("2026-02-29"); e == nil {
		t.Fatal("invalid calendar date accepted")
	}
	if _, e = ParseNow("2026-10-02T12:00:00"); e == nil {
		t.Fatal("offsetless now accepted")
	}
}
func mustNow(s string) time.Time {
	t, e := ParseNow(s)
	if e != nil {
		panic(e)
	}
	return t
}

func TestBaggageLimits(t *testing.T) {
	for _, tt := range []struct {
		height, weight  float64
		pieces          int
		class, decision string
		required        bool
	}{{20, 30, 2, "unreserved", "ordinary_baggage", false}, {20.01, 20, 1, "reserved", "reserve_oversized_baggage_area", true}, {20.01, 20, 1, "unreserved", "unreserved_class_incompatible", true}, {110, 20, 1, "green", "reserve_oversized_baggage_area", true}, {110.01, 20, 1, "reserved", "outside_normal_carry_on_limits", true}, {20, 30.01, 1, "reserved", "outside_normal_carry_on_limits", false}, {20, 20, 3, "reserved", "outside_normal_carry_on_limits", false}} {
		v, e := CheckBaggage(BaggageInput{Length: 80, Width: 60, Height: tt.height, Weight: tt.weight, Pieces: tt.pieces, Class: tt.class})
		if e != nil || v["decision"] != tt.decision || v["oversized_area_reservation_required"] != tt.required {
			t.Errorf("case%+v got%+v e%v", tt, v, e)
		}
	}
	if _, e := CheckBaggage(BaggageInput{Length: math.NaN(), Width: 10, Height: 10, Weight: 1, Pieces: 1, Class: "reserved"}); e == nil {
		t.Fatal("NaN accepted")
	}
	v, _ := CheckBaggage(BaggageInput{Length: 201, Width: 1, Height: 1, Weight: 1, Pieces: 1, Class: "reserved"})
	if v["within_normal_limits"] != false {
		t.Fatal("longest side limit ignored")
	}
	v, _ = CheckBaggage(BaggageInput{Length: 80, Width: 60, Height: 40, Weight: 10, Pieces: 1, Class: "unreserved", Special: "sports"})
	if v["decision"] != "special_equipment_requires_operator_confirmation" {
		t.Fatal("special equipment must not be autoapproved")
	}
}

func TestStationAndProductSemantics(t *testing.T) {
	a, e := Resolve("新大阪")
	b, e2 := Resolve("Shin Osaka")
	if e != nil || e2 != nil || a.ID != b.ID {
		t.Fatal("Japanese/English identity mismatch")
	}
	if _, e = Resolve("Osaka"); e == nil {
		t.Fatal("conventional Osaka silently converted")
	}
	if _, e = Resolve("仙台"); e == nil {
		t.Fatal("Tohoku Sendai included in Kyushu scope")
	}
	rows, total := Stations("ZZZ-not-a-station", 50)
	if rows == nil || total != 0 {
		t.Fatal("empty station result fabricated")
	}
	r, e := PlanRoute("Kagoshima-Chuo", "Tokyo", true)
	if e != nil || r.Direction != "eastbound" || len(r.Corridors) != 3 || len(r.Stations) != 46 || r.ThroughTrainConfirmed {
		t.Fatalf("route%+v %v", r, e)
	}
	p, e := CompareProducts("2026-10-28", "Tokyo", "Shin-Osaka", "unreserved", "nozomi", mustNow("2026-10-02T12:00:00+09:00"), 1, 0)
	if e != nil || len(p) != 6 {
		t.Fatal(e)
	}
	for _, v := range p {
		if v.ID == "family7" && len(v.ExclusionReasons) < 2 {
			t.Fatal("family party/train checks missing")
		}
		if v.ID == "hayatoku7" && v.Assessment != "excluded_by_checked_conditions" {
			t.Fatal("Tokaido Nozomi/unreserved Hayatoku7 incorrectly included")
		}
		if v.PriceJPY != nil {
			t.Fatal("unverified discount price invented")
		}
	}
	if _, e = ProductByID("roundtrip"); e == nil {
		t.Fatal("ended roundtrip offered")
	}
}

func TestTimetableExamplesKeepDepartureAndInventoryDistinct(t *testing.T) {
	m, e := BasicMatches("Tokyo", "Shin-Osaka", "nozomi", "", 5, false)
	if e != nil || len(m) != 1 || m[0].Number != 1 || m[0].Arrival != "08:22" || m[0].DurationMinutes != 142 || m[0].Inventory != nil || m[0].ServiceOnRequestedDate != nil {
		t.Fatalf("bad verified example%+v %v", m, e)
	}
	m, _ = BasicMatches("Tokyo", "Kyoto", "nozomi", "", 5, false)
	if len(m) != 1 || m[0].Arrival != nil || m[0].DestinationDeparture != "08:09" || m[0].DurationMinutes != nil {
		t.Fatal("departure-only row relabelled as arrival")
	}
	m, _ = BasicMatches("Tokyo", "Shin-Osaka", "tsubame", "", 5, false)
	if m == nil || len(m) != 0 {
		t.Fatal("invented Tsubame in Tokaido")
	}
	for _, s := range basicServices {
		previous := ""
		for _, stop := range s.Stops {
			if _, e := Resolve(stop.StationID); e != nil {
				t.Fatal(e)
			}
			for _, clock := range []string{stop.Arrival, stop.Departure} {
				if clock == "" {
					continue
				}
				if _, e := time.Parse("15:04", clock); e != nil {
					t.Fatal(e)
				}
				if clock < previous {
					t.Fatalf("nonmonotonic %s%d %+v", s.Train, s.Number, stop)
				}
				previous = clock
			}
		}
	}
}
