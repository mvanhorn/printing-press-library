package cli

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/consumer"
)

func TestRankSpots(t *testing.T) {
	spots := []consumer.SearchSpot{
		{FacilityID: 1, PriceCents: 900, DistanceMeters: 800, Available: true},
		{FacilityID: 2, PriceCents: 500, DistanceMeters: 100, Available: false}, // cheapest but not bookable
		{FacilityID: 3, PriceCents: 900, DistanceMeters: 200, Available: true},
		{FacilityID: 4, PriceCents: 1500, DistanceMeters: 50, Available: true},
	}
	got := rankSpots(spots, 0, 0)
	if len(got) != 3 || got[0].FacilityID != 3 || got[1].FacilityID != 1 {
		t.Fatalf("%+v", got)
	}
	if got := rankSpots(spots, 500, 1000); len(got) != 1 || got[0].FacilityID != 3 {
		t.Fatalf("limits: %+v", got)
	}
}

func TestCommuteWindowsStartTomorrowAndStayLocal(t *testing.T) {
	// A Friday: the series must not include today, because a window already underway
	// cannot be quoted usefully.
	from := time.Date(2027, 1, 15, 8, 0, 0, 0, time.UTC)
	got, err := commuteWindows(from, []string{"fri", "mon"}, "09:00", "17:00", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0][0] != "2027-01-18T09:00" || got[1][1] != "2027-01-22T17:00" {
		t.Fatalf("%v", got)
	}
	if _, err := commuteWindows(from, []string{"someday"}, "09:00", "17:00", 1); err == nil {
		t.Fatal("bad day accepted")
	}
}

func TestCommuteOvernightEndsNextDay(t *testing.T) {
	from := time.Date(2027, 1, 15, 8, 0, 0, 0, time.UTC)
	got, err := commuteWindows(from, []string{"mon"}, "22:00", "06:00", 1)
	if err != nil {
		t.Fatal(err)
	}
	if got[0][0] != "2027-01-18T22:00" || got[0][1] != "2027-01-19T06:00" {
		t.Fatalf("%v", got)
	}
}

func TestRankRegularsExcludesCancelled(t *testing.T) {
	rows := []json.RawMessage{
		json.RawMessage(`{"facility_id":1,"stall_name":"A","starts":"2027-01-01T00:00:00Z","price":100,"reservation_status":"valid"}`),
		json.RawMessage(`{"facility_id":1,"stall_name":"A","starts":"2027-02-01T00:00:00Z","price":200,"reservation_status":"valid"}`),
		json.RawMessage(`{"facility_id":2,"stall_name":"B","starts":"2027-03-01T00:00:00Z","price":300,"reservation_status":"cancelled"}`),
		json.RawMessage(`{"facility_id":3,"stall_name":"C","starts":"2026-01-01T00:00:00Z","price":50,"reservation_status":"valid"}`),
	}
	got := rankRegulars(rows)
	if len(got) != 2 || got[0].FacilityID != 1 || got[0].Bookings != 2 || got[0].LastPaidCents != 200 {
		t.Fatalf("%+v", got)
	}
}

func TestCountOverlaps(t *testing.T) {
	now := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	list := []consumer.Reservation{
		{Status: "valid", Starts: "2027-01-15T15:00:00Z", Ends: "2027-01-15T18:00:00Z"},
		{Status: "cancelled", Starts: "2027-01-15T15:00:00Z", Ends: "2027-01-15T18:00:00Z"},
		{Status: "valid", Starts: "2027-01-16T15:00:00Z", Ends: "2027-01-16T18:00:00Z"},
	}
	p := []consumer.SearchPeriod{{Starts: "2027-01-15T17:00:00Z", Ends: "2027-01-15T19:00:00Z"}}
	if n := countOverlaps(list, p, now); n != 1 {
		t.Fatalf("n = %d", n)
	}
}

func TestJWTExpiryReadsOnlyExp(t *testing.T) {
	// header.{"exp":1800000000}.sig
	if exp, ok := jwtExpiry("e30.eyJleHAiOjE4MDAwMDAwMDB9.c2ln"); !ok || exp.Unix() != 1800000000 {
		t.Fatalf("%v %v", exp, ok)
	}
	if _, ok := jwtExpiry("not-a-jwt"); ok {
		t.Fatal("accepted a non JWT")
	}
}

// The inventory host is derived from a generated map, so a regeneration that renames the
// map or changes the path must fail here rather than at the first real quote.
func TestInventoryBaseURLComesFromSpec(t *testing.T) {
	t.Setenv("SPOTHERO_CRAIG_BASE_URL", "")
	if got := inventoryBaseURL(); got != "https://api.spothero.com/v2" {
		t.Fatalf("inventoryBaseURL() = %q", got)
	}
}

// Text order puts 09:00-08:00 first, but it starts two hours after 10:00-05:00.
func TestSortReservationsComparesInstants(t *testing.T) {
	rs := []consumer.Reservation{{Starts: "2027-01-15T09:00:00-08:00"}, {Starts: "2027-01-15T10:00:00-05:00"}}
	sortReservationsByStart(rs)
	if rs[0].Starts != "2027-01-15T10:00:00-05:00" {
		t.Fatalf("soonest = %s", rs[0].Starts)
	}
}

// 00:30 Tuesday UTC is still Monday in Chicago, so tomorrow there is the coming Tuesday.
func TestCommuteUsesGarageCalendar(t *testing.T) {
	chicago, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Skip(err)
	}
	from := time.Date(2027, 1, 12, 0, 30, 0, 0, time.UTC).In(chicago)
	got, err := commuteWindows(from, []string{"tue"}, "09:00", "17:00", 1)
	if err != nil || len(got) != 1 || got[0][0] != "2027-01-12T09:00" {
		t.Fatalf("got %v, %v", got, err)
	}
}
