// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package parks

import (
	"errors"
	"os"
	"testing"
	"time"
)

func mustRead(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestExtractTicketPopup(t *testing.T) {
	cases := []struct {
		name    string
		page    string
		want    string
		wantErr bool
	}{
		{"object", "<script>\nvar ticketPopup = {\"20261114\":{}};\n</script>", `{"20261114":{}}`, false},
		{"empty month", "<script>\n  var ticketPopup = [];\n</script>", `[]`, false},
		{"missing", "<html>no calendar</html>", "", true},
		{"broken json", "<script>\nvar ticketPopup = {\"a\":;\n</script>", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExtractTicketPopup([]byte(tc.page))
			if tc.wantErr {
				if !errors.Is(err, ErrTDRStructure) {
					t.Fatalf("want ErrTDRStructure, got %v", err)
				}
				return
			}
			if err != nil || string(got) != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestParseTicketPopupFixture(t *testing.T) {
	ja, err := ParseTicketPopup(mustRead(t, "tdr-ticketpopup-ja.json"))
	if err != nil {
		t.Fatal(err)
	}
	en, err := ParseTicketPopup(mustRead(t, "tdr-ticketpopup-en.json"))
	if err != nil {
		t.Fatal(err)
	}
	ja.MergeEnglish(en)
	day := ja["2026-11-14"]["tds"]
	if day == nil {
		t.Fatal("2026-11-14 tds missing")
	}
	if day.Open != "09:00" || day.Close != "21:00" {
		t.Fatalf("hours = %s-%s", day.Open, day.Close)
	}
	if len(day.Tickets) == 0 || day.Tickets[0].ID != "T1" {
		t.Fatalf("tickets = %+v", day.Tickets)
	}
	t1 := day.Tickets[0]
	if t1.NameJA != "1デーパスポート" || t1.NameEN != "1-Day Passport" {
		t.Fatalf("names = %q / %q", t1.NameJA, t1.NameEN)
	}
	if t1.Status != StatusOnSale {
		t.Fatalf("T1 status = %s", t1.Status)
	}
	if t1.PricesYen["adult"] != 12400 || t1.PricesYen["junior"] != 10200 || t1.PricesYen["child"] != 5900 {
		t.Fatalf("prices = %v", t1.PricesYen)
	}
	var t2 *TDRTicket
	for i := range day.Tickets {
		if day.Tickets[i].ID == "T2" {
			t2 = &day.Tickets[i]
		}
	}
	if t2 == nil || t2.Status != StatusFewLeft {
		t.Fatalf("T2 = %+v", t2)
	}
}

func TestParseTicketPopupEmptyAndBroken(t *testing.T) {
	cal, err := ParseTicketPopup([]byte(`[]`))
	if err != nil || len(cal) != 0 {
		t.Fatalf("empty month: %v %v", cal, err)
	}
	for _, raw := range []string{`{"bad-date":{"tdl":{}}}`, `{"20261114":{"tdl":{"info":[1,2]}}}`, `"x"`} {
		if _, err := ParseTicketPopup([]byte(raw)); !errors.Is(err, ErrTDRStructure) {
			t.Fatalf("%s: want ErrTDRStructure, got %v", raw, err)
		}
	}
}

func TestDecodeStatus(t *testing.T) {
	cases := []struct {
		pass  bool
		class string
		want  string
	}{
		{false, "conversion", StatusOnSale},
		{false, "conversion is-few", StatusFewLeft},
		{false, "conversion is-none", StatusSoldOut},
		{false, "conversion is-notSales", StatusNotYetOnSale},
		{true, "conversion", StatusNoOnlineSale},
		{false, "something-new", StatusUnknown},
	}
	for _, tc := range cases {
		if got := DecodeStatus(tc.pass, tc.class); got != tc.want {
			t.Errorf("DecodeStatus(%v, %q) = %s, want %s", tc.pass, tc.class, got, tc.want)
		}
	}
}

func TestParsePrices(t *testing.T) {
	cases := []struct {
		text string
		want map[string]int
	}{
		{"大人：￥12,400　中人：￥10,200　小人：￥5,900", map[string]int{"adult": 12400, "junior": 10200, "child": 5900}},
		{"Adult：11,900 yen　Junior：9,800 yen　Child：5,900 yen", map[string]int{"adult": 11900, "junior": 9800, "child": 5900}},
		{"", map[string]int{}},
	}
	for _, tc := range cases {
		got := ParsePrices(tc.text)
		if len(got) != len(tc.want) {
			t.Fatalf("%q: got %v", tc.text, got)
		}
		for k, v := range tc.want {
			if got[k] != v {
				t.Fatalf("%q: %s = %d, want %d", tc.text, k, got[k], v)
			}
		}
	}
}

func TestSaleOpensAt(t *testing.T) {
	cases := []struct{ date, want string }{
		{"2026-12-20", "2026-10-20T14:00:00+09:00"},
		{"2027-01-15", "2026-11-15T14:00:00+09:00"},
		{"2026-12-31", "2026-10-31T14:00:00+09:00"}, // Oct 31 exists
		{"2027-04-30", "2027-03-01T14:00:00+09:00"}, // Feb 30 does not exist
	}
	for _, tc := range cases {
		d, _ := time.ParseInLocation("2006-01-02", tc.date, Tokyo)
		got := SaleOpensAt(d).Format(time.RFC3339)
		if got != tc.want {
			t.Errorf("SaleOpensAt(%s) = %s, want %s", tc.date, got, tc.want)
		}
	}
}

func TestMonthsBetween(t *testing.T) {
	a, _ := time.ParseInLocation("2006-01-02", "2026-11-28", Tokyo)
	b, _ := time.ParseInLocation("2006-01-02", "2027-01-03", Tokyo)
	got := MonthsBetween(a, b)
	want := []string{"202611", "202612", "202701"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v", got)
		}
	}
}

func TestParseQueueTimes(t *testing.T) {
	rides, err := ParseQueueTimes(mustRead(t, "queue-times-275.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rides) != 3 {
		t.Fatalf("rides = %d", len(rides))
	}
	var open int
	for _, r := range rides {
		if r.Land != "American Waterfront" {
			t.Fatalf("land = %q", r.Land)
		}
		if r.IsOpen {
			open++
			if r.ID != 8036 || r.WaitMinutes != 20 {
				t.Fatalf("open ride = %+v", r)
			}
		}
		if r.LastUpdated.IsZero() {
			t.Fatalf("last_updated not parsed: %+v", r)
		}
	}
	if open != 1 {
		t.Fatalf("open = %d", open)
	}
	top, err := ParseQueueTimes([]byte(`{"lands":[],"rides":[{"id":1,"name":"Coaster","is_open":true,"wait_time":45,"last_updated":"2026-10-09T01:00:00.000Z"}]}`))
	if err != nil || len(top) != 1 || top[0].Land != "" || top[0].WaitMinutes != 45 {
		t.Fatalf("top-level rides: %+v %v", top, err)
	}
	if _, err := ParseQueueTimes([]byte(`<html>`)); err == nil {
		t.Fatal("want error for non-JSON")
	}
}

func TestScheduleHoursFor(t *testing.T) {
	doc := []byte(`{"schedule":[
	 {"date":"2026-10-10","type":"OPERATING","openingTime":"2026-10-10T08:30:00+09:00","closingTime":"2026-10-10T21:00:00+09:00"},
	 {"date":"2026-10-10","type":"TICKETED_EVENT","description":"Early Park Admission","openingTime":"2026-10-10T08:00:00+09:00","closingTime":"2026-10-10T08:30:00+09:00"},
	 {"date":"2026-10-12","type":"INFO","openingTime":"2026-10-12T09:00:00+09:00","closingTime":"2026-10-12T19:00:00+09:00"}
	]}`)
	s, err := ParseSchedule(doc)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		date, open, close string
		extra             int
		reasonPrefix      string
	}{
		{"2026-10-10", "08:30", "21:00", 1, ""},
		{"2026-10-11", "", "", 0, "no ThemeParks.wiki schedule row"},
		{"2026-10-12", "", "", 1, "no OPERATING row"},
		{"2026-11-20", "", "", 0, "beyond third-party source horizon"},
		{"2026-10-01", "", "", 0, "before the third-party schedule window"},
	}
	for _, tc := range cases {
		open, close, extra, reason := s.HoursFor(tc.date)
		if open != tc.open || close != tc.close || len(extra) != tc.extra {
			t.Errorf("%s: got %s-%s extra=%d", tc.date, open, close, len(extra))
		}
		if len(reason) < len(tc.reasonPrefix) || reason[:len(tc.reasonPrefix)] != tc.reasonPrefix {
			t.Errorf("%s: reason %q, want prefix %q", tc.date, reason, tc.reasonPrefix)
		}
	}
	empty, _ := ParseSchedule([]byte(`{"schedule":[]}`))
	if _, _, _, r := empty.HoursFor("2026-10-10"); r == "" {
		t.Error("empty schedule must give a reason")
	}
}

func TestQuantile(t *testing.T) {
	cases := []struct {
		v    []float64
		q    float64
		want float64
	}{
		{[]float64{10}, 0.5, 10},
		{[]float64{10, 20}, 0.5, 15},
		{[]float64{40, 10, 30, 20}, 0.5, 25},
		{[]float64{10, 20, 30, 40}, 0.75, 32.5},
	}
	for _, tc := range cases {
		if got := Quantile(tc.v, tc.q); got != tc.want {
			t.Errorf("Quantile(%v, %v) = %v, want %v", tc.v, tc.q, got, tc.want)
		}
	}
}

func TestSummarize(t *testing.T) {
	at := func(s string) time.Time {
		v, err := time.ParseInLocation("2006-01-02 15:04", s, Tokyo)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	samples := []Sample{
		{RideID: 1, RideName: "A", IsOpen: true, WaitMinutes: 30, At: at("2026-10-03 10:05")}, // Sat
		{RideID: 1, RideName: "A", IsOpen: true, WaitMinutes: 50, At: at("2026-10-03 10:35")},
		{RideID: 1, RideName: "A", IsOpen: true, WaitMinutes: 40, At: at("2026-10-10 10:10")},
		{RideID: 1, RideName: "A", IsOpen: false, WaitMinutes: 0, At: at("2026-10-10 10:40")},
		{RideID: 2, RideName: "B", IsOpen: true, WaitMinutes: 5, At: at("2026-10-03 10:05")},
	}
	cells := Summarize(samples, 3)
	if len(cells) != 2 {
		t.Fatalf("cells = %+v", cells)
	}
	a := cells[0]
	if a.RideName != "A" || a.Weekday != "sat" || a.Hour != 10 || a.Status != "ok" {
		t.Fatalf("A = %+v", a)
	}
	if *a.Median != 40 || a.Samples != 3 || a.ClosedSamples != 1 || a.DistinctDays != 2 || a.FirstDate == nil || *a.FirstDate != "2026-10-03" || *a.LastDate != "2026-10-10" {
		t.Fatalf("A stats = %+v median=%v", a, *a.Median)
	}
	b := cells[1]
	if b.Status != "insufficient" || b.Median != nil || b.P75 != nil || b.Samples != 1 || b.FirstDate == nil {
		t.Fatalf("B = %+v", b)
	}
}

func TestParseWeekdays(t *testing.T) {
	cases := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"", 0, false},
		{"sat,sun", 2, false},
		{"weekend", 2, false},
		{"weekday", 5, false},
		{"Mon", 1, false},
		{"funday", 0, true},
	}
	for _, tc := range cases {
		got, err := ParseWeekdays(tc.in)
		if (err != nil) != tc.wantErr || len(got) != tc.want {
			t.Errorf("ParseWeekdays(%q) = %v, %v", tc.in, got, err)
		}
	}
}

func TestLookup(t *testing.T) {
	cases := map[string]string{"275": "tds", "tds": "tds", "Tokyo DisneySea": "tds", "usj": "usj", "284": "usj", "fujiq": "fujiq"}
	for in, want := range cases {
		p, ok := Lookup(in)
		if !ok || p.Key != want {
			t.Errorf("Lookup(%q) = %v, %v; want %s", in, p.Key, ok, want)
		}
	}
	if _, ok := Lookup("disneyland paris"); ok {
		t.Error("unknown park must not resolve")
	}
	if _, err := LookupList("tdl,nowhere", nil); err == nil {
		t.Error("LookupList must reject unknown parks")
	}
}

func TestHoursForSplitSessions(t *testing.T) {
	s, err := ParseSchedule([]byte(`{"schedule":[
	 {"date":"2026-10-10","type":"OPERATING","openingTime":"2026-10-10T17:00:00+09:00","closingTime":"2026-10-10T21:00:00+09:00"},
	 {"date":"2026-10-10","type":"OPERATING","openingTime":"2026-10-10T09:00:00+09:00","closingTime":"2026-10-10T15:00:00+09:00"}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	if open, close, _, reason := s.HoursFor("2026-10-10"); open != "09:00" || close != "21:00" || reason != "" {
		t.Fatalf("got %s-%s %q", open, close, reason)
	}
}

func TestMergeEnglishMatchesGroupAndID(t *testing.T) {
	ja := TDRCalendar{"2026-11-14": {"tds": {Tickets: []TDRTicket{
		{ID: "T1", GroupID: "G1", NameJA: "1デーパスポート"},
		{ID: "T1", GroupID: "G2", NameJA: "別グループ"},
	}}}}
	en := TDRCalendar{"2026-11-14": {"tds": {Tickets: []TDRTicket{
		{ID: "T1", GroupID: "G2", NameJA: "Other group"},
		{ID: "T1", GroupID: "G1", NameJA: "1-Day Passport"},
	}}}}
	ja.MergeEnglish(en)
	got := ja["2026-11-14"]["tds"].Tickets
	if got[0].NameEN != "1-Day Passport" || got[1].NameEN != "Other group" {
		t.Fatalf("names = %q, %q", got[0].NameEN, got[1].NameEN)
	}
}

func TestSortTicketIDs(t *testing.T) {
	got := sortTicketIDs([]string{"T10", "T2", "T1", "T8"})
	want := []string{"T1", "T2", "T8", "T10"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v", got)
		}
	}
}

func TestPadClock(t *testing.T) {
	cases := map[string]string{"9:00": "09:00", "21:00": "21:00", " 8:30 ": "08:30", "": "", "noon": "", "\x1b[2J": "", "25:00": "", "24:00": "24:00", "24:30": "", "9:5": ""}
	for in, want := range cases {
		if got := padClock(in); got != want {
			t.Errorf("padClock(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRemoteTextCleanedAtParse(t *testing.T) {
	rides, err := ParseQueueTimes([]byte(`{"lands":[{"name":"Land\u001b[31m","rides":[{"id":1,"name":"Big\nThunder &amp; Co","is_open":true,"wait_time":5,"last_updated":"2026-10-09T01:00:00.000Z"}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if rides[0].Name != "Big Thunder & Co" || rides[0].Land != "Land[31m" {
		t.Fatalf("got %q / %q", rides[0].Name, rides[0].Land)
	}
	s, err := ParseSchedule([]byte(`{"schedule":[{"date":"2026-10-10","type":"INFO","description":"Night\u0007 Show","openingTime":"2026-10-10T19:00:00+09:00","closingTime":"2026-10-10T20:00:00+09:00"}]}`))
	if err != nil || s.ByDate["2026-10-10"][0].Description != "Night Show" {
		t.Fatalf("schedule description not cleaned: %+v %v", s.ByDate, err)
	}
}
