// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package limousine

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, name string) map[string]any {
	t.Helper()
	b, e := os.ReadFile("testdata/" + name)
	if e != nil {
		t.Fatal(e)
	}
	d, e := DecodeSvelte(b)
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func scheduleFixture(t *testing.T) Schedule {
	t.Helper()
	s, e := ParseSchedule(fixture(t, "timetable-detail-data.json"), "Haneda-Narita", "2026-10-03", "from-airport", "https://www.limousinebus.co.jp/en/timetable/detail/Haneda-Narita/?dir=1&d=2026-10-03")
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestDatedTerminalPairAndFare(t *testing.T) {
	s := scheduleFixture(t)
	if len(s.Stations) != 6 || len(s.Journeys) != 28 {
		t.Fatalf("unexpected source columns/trips %d/%d", len(s.Stations), len(s.Journeys))
	}
	j, e := FilterJourneys(s, "HanedaAirportTerminal3", "NaritaAirportTerminal1", "08:00")
	if e != nil {
		t.Fatal(e)
	}
	if len(j) == 0 {
		t.Fatal("filtered journeys empty")
	}
	for _, r := range j {
		if r.Origin.Departure.Time < "08:00" || r.Origin.StopID != "HanedaAirportTerminal3" || r.Destination.StopID != "NaritaAirportTerminal1" {
			t.Fatalf("wrong selected pair: %+v", r)
		}
	}
	if j[0].Origin.Departure.DateTime != "2026-10-03T08:25:00+09:00" || *j[0].ScheduledDurationMinutes != 95 {
		t.Fatalf("wrong first selected trip: %+v", j[0])
	}
	quote, e := Quote(s, "HanedaAirportTerminal3", "NaritaAirportTerminal1", 2, 1, "")
	if e != nil {
		t.Fatal(e)
	}
	if quote.TotalJPY == nil || *quote.TotalJPY != 9000 || *quote.AdultJPY != 3600 || *quote.ChildJPY != 1800 {
		t.Fatalf("wrong party fare %+v", quote)
	}
	b, _ := json.Marshal(s)
	for _, secret := range []string{"normal_seat_remains", "seat_remains", "reservBtn", "Available", "reservation"} {
		if strings.Contains(string(b), secret) {
			t.Fatalf("inventory leaked: %s", secret)
		}
	}
}
func TestStopRolesMissingAndDateMismatch(t *testing.T) {
	s := scheduleFixture(t)
	if _, e := FilterJourneys(s, "NaritaAirportTerminal1", "HanedaAirportTerminal3", ""); e == nil {
		t.Fatal("reversed stop roles accepted")
	}
	if _, e := FilterJourneys(s, "", "", "08:00"); e == nil {
		t.Fatal("ambiguous departure cutoff accepted")
	}
	if _, e := FilterJourneys(s, "Missing", "NaritaAirportTerminal1", ""); e == nil {
		t.Fatal("unknown stop accepted")
	}
	if _, e := Quote(s, "HanedaAirportTerminal3", "NaritaAirportTerminal1", -1, 1, ""); e == nil {
		t.Fatal("negative party accepted")
	}
	if _, e := ParseSchedule(fixture(t, "timetable-detail-data.json"), "Haneda-Narita", "2026-10-04", "from-airport", ""); e == nil {
		t.Fatal("wrong returned service date accepted")
	}
}
func TestClockRolloverAndUnknowns(t *testing.T) {
	x, e := ClockFromSource("2420", "0", "2026-10-03", 23*60+55)
	if e != nil || x.DateTime != "2026-10-04T00:20:00+09:00" || x.DayOffset != 1 || x.RolloverInferred {
		t.Fatalf("extended hour %+v %v", x, e)
	}
	x, e = ClockFromSource("20", "0", "2026-10-03", 23*60+55)
	if e != nil || x.DayOffset != 1 || !x.RolloverInferred {
		t.Fatalf("wrapped hour %+v %v", x, e)
	}
	for _, c := range []struct{ raw, kind string }{{"-1", "-2"}, {"1200", "3"}, {"", "0"}} {
		x, e = ClockFromSource(c.raw, c.kind, "2026-10-03", -1)
		if e != nil || x != nil {
			t.Fatalf("unknown time guessed: %+v %v", x, e)
		}
	}
	for _, raw := range []string{"1260", "4800", "x", "99999"} {
		if _, e = ClockFromSource(raw, "0", "2026-10-03", -1); e == nil {
			t.Fatalf("invalid clock accepted %s", raw)
		}
	}
	if _, e = ClockFromSource("1159", "0", "2026-10-03", 12*60); e == nil {
		t.Fatal("small backwards clock treated as overnight")
	}
	day, e := ServiceDate("", time.Date(2026, 10, 2, 16, 0, 0, 0, time.UTC))
	if e != nil || day != "2026-10-03" {
		t.Fatalf("wrong JST default %s %v", day, e)
	}
}
func TestFareVariesByTripAndUnknown(t *testing.T) {
	s := scheduleFixture(t)
	sourceFare := s.Journeys[0].Calls[5].Fare
	n := 4000
	s.Journeys[0].Calls[5].Fare.AdultJPY = &n
	if _, e := Quote(s, "HanedaAirportTerminal3", "NaritaAirportTerminal1", 1, 0, ""); e == nil {
		t.Fatal("different trip fares silently merged")
	}
	q, e := Quote(s, "HanedaAirportTerminal3", "NaritaAirportTerminal1", 1, 0, s.Journeys[0].ID)
	if e != nil || q.TotalJPY == nil || *q.TotalJPY != 4000 {
		t.Fatalf("selected fare %+v %v", q, e)
	}
	s.Journeys[0].Calls[5].Fare = sourceFare
	s.Journeys[0].Calls[5].Fare.AdultJPY = nil
	q, e = Quote(s, "HanedaAirportTerminal3", "NaritaAirportTerminal1", 1, 0, s.Journeys[0].ID)
	if e != nil || q.TotalJPY != nil || q.QuoteState != "unknown" {
		t.Fatalf("unknown fare became quote %+v %v", q, e)
	}
}
func TestCurrentEstimatesPreserveUnavailable(t *testing.T) {
	rows, scanned, e := Durations(fixture(t, "realtime-data.json"), "", "", " ")
	_ = scanned
	if e != nil {
		t.Fatal(e)
	}
	if len(rows) != 0 {
		t.Fatal("mismatched direction did not filter")
	}
	rows, _, e = Durations(fixture(t, "realtime-data.json"), "Shinjuku", "haneda", "")
	if e != nil || len(rows) != 2 {
		t.Fatalf("current Shinjuku %d %v", len(rows), e)
	}
	for _, r := range rows {
		if r.CurrentMinutes == nil || r.StandardMinutes == nil || r.DifferenceMinutes == nil || *r.DifferenceMinutes != *r.CurrentMinutes-*r.StandardMinutes || r.SourceDateKnown || r.ArrivalGuaranteed {
			t.Fatalf("bad evidence %+v", r)
		}
	}
	for _, value := range []string{"Adjusting...", "Retrieving data...", "--", "unexpected"} {
		if n, state := Minutes(value); n != nil || state == "estimated" {
			t.Fatalf("unavailable %q became numeric", value)
		}
	}
	rows, _, e = Durations(fixture(t, "realtime-data.json"), "not-a-route", "", "")
	if e != nil || len(rows) != 0 {
		t.Fatalf("negative search %v %v", rows, e)
	}
}
func TestRouteAndStopSourceIdentity(t *testing.T) {
	routes, _, e := Routes(fixture(t, "routes-data.json"), "Shinjuku", "haneda", false)
	if e != nil || len(routes) < 1 {
		t.Fatal(e)
	}
	for _, r := range routes {
		if r.Airport != "haneda" || !strings.Contains(r.ID, "Shinjuku") {
			t.Fatalf("wrong route %+v", r)
		}
	}
	detail, e := Detail(fixture(t, "stop-haneda3-data.json"), 3)
	if e != nil || detail.Stop.ID != "HanedaAirportTerminal3" || detail.Stop.JapaneseName != "羽田空港第３ターミナル" || detail.PickupLat == nil || len(detail.Connections) != 3 || len(detail.MapURLs) == 0 {
		t.Fatalf("stop %+v %v", detail, e)
	}
}
func TestGuideFactsAndStreamedData(t *testing.T) {
	facts, e := Conditions(fixture(t, "baggage-data.json"), "baggage")
	if e != nil {
		t.Fatal(e)
	}
	found := map[string]any{}
	for _, f := range facts {
		found[f.Key] = f.Value
	}
	if found["checked_bag_count"] != 2 || found["checked_bag_weight"] != 30 {
		t.Fatalf("wrong luggage facts %+v", facts)
	}
	facts, e = Conditions(fixture(t, "boarding-data.json"), "boarding")
	if e != nil {
		t.Fatal(e)
	}
	keys := map[string]bool{}
	for _, f := range facts {
		keys[f.Key] = true
	}
	if !keys["preschool_seats"] || !keys["child_category"] || !keys["seated_only"] {
		t.Fatalf("missing boarding rules %+v", facts)
	}
	if CleanHTML("<div>Visible<!--hidden--><script>evil</script><style>ignore</style> fact</div>") != "Visible fact" {
		t.Fatal("HTML comments or scripts leaked")
	}
}
func TestSvelteRejectsCorruptOrErrorResponse(t *testing.T) {
	for _, body := range []string{`{"type":"data","nodes":[{"type":"data","data":[{"x":55}]}]}`, `{"type":"data","nodes":[{"type":"error","error":{"message":"Internal Error"}}]}`, `{"type":"data","nodes":[{"type":"data","data":[{"x":0}]}]}`, `<html>challenge</html>`} {
		if _, e := DecodeSvelte([]byte(body)); e == nil {
			t.Fatalf("corrupt accepted %s", body)
		}
	}
}

func TestProvenanceEnvelopeHasExactlyTwoKeys(t *testing.T) {
	b, e := json.Marshal(Envelope{Meta: Meta{Source: "live", ObservedAt: "2026-10-02T15:25:38Z"}, Results: []string{"trip"}, Stations: []Station{{ID: "HanedaAirportTerminal3"}}, Details: map[string]any{"selected_date_jst": "2026-10-03"}})
	if e != nil {
		t.Fatal(e)
	}
	var out map[string]any
	if e = json.Unmarshal(b, &out); e != nil {
		t.Fatal(e)
	}
	if len(out) != 2 || len(A(out["results"])) != 1 || len(A(M(out["meta"])["stations"])) != 1 || M(out["meta"])["context"] == nil {
		t.Fatalf("wrong agent envelope %s", b)
	}
}
