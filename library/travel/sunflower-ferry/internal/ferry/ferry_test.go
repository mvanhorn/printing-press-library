package ferry

import (
	"bytes"
	"context"
	"errors"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/sunflower-ferry/internal/cliutil"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/" + name)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestTimetablesPreserveRowspanAndDirections(t *testing.T) {
	for _, r := range Registry {
		for _, line := range []string{r.OutboundLine, r.InboundLine} {
			tt, e := ParseTimetable(fixture(t, r.ID+"-time.html"), r, line)
			if e != nil {
				t.Fatal(r.ID, line, e)
			}
			seen := map[int]bool{}
			for _, rule := range tt.Rules {
				if rule.ArrivalDayOffset != 1 {
					t.Fatal("lost explicit next-morning arrival")
				}
				for _, d := range rule.WeekdayNumbers {
					if seen[d] {
						t.Fatal("weekday overlap")
					}
					seen[d] = true
				}
			}
			if len(seen) != 7 {
				t.Fatal("weekday gap", seen)
			}
		}
	}
	r := Registry[2]
	out, e := ParseTimetable(fixture(t, "osaka-shibushi-time.html"), r, r.OutboundLine)
	if e != nil {
		t.Fatal(e)
	}
	if out.Rules[1].Departure != "17:55" || out.Rules[2].Departure != "17:55" || out.Rules[2].Arrival != "08:55" {
		t.Fatal("rowspan was lost", out.Rules)
	}
	in, e := ParseTimetable(fixture(t, "osaka-shibushi-time.html"), r, r.InboundLine)
	if e != nil {
		t.Fatal(e)
	}
	if in.Rules[1].Arrival != "07:50" || in.Rules[2].Departure != "18:30" || in.Rules[2].Arrival != "08:50" {
		t.Fatal("inbound Friday/Saturday changed", in.Rules)
	}
	if _, e := ParseTimetable([]byte(`<article id="article"><table><tr><th>Day of the week</th></tr></table></article>`), r, r.OutboundLine); e == nil {
		t.Fatal("missing rows must fail without panic")
	}
}
func TestSourceQuoteDatesPricesAndSnapshots(t *testing.T) {
	d, _ := ParseDate("2026-10-15")
	q, e := ParseQuote(fixture(t, "quote-osaka-beppu.html"), Registry[0], "21", d, Party{Adults: 1, Mode: "foot"})
	if e != nil {
		t.Fatal(e)
	}
	if q.Season != "A" || q.DiscountLabel != "Web DC(Partcal 5%)" || len(q.Sailings) != 1 {
		t.Fatalf("lost source season/discount/sailing: %+v", q)
	}
	s := q.Sailings[0]
	if s.Departure != "2026-10-15T19:05:00+09:00" || s.Arrival != "2026-10-16T06:55:00+09:00" || s.ArrivalDayOffset != 1 {
		t.Fatal("wrong overnight arrival", s)
	}
	found := false
	for _, f := range s.CabinFares {
		if f.Name == "Private bed room" {
			found = true
			if f.DisplayedPriceJPY != 14620 || f.Availability != "available_snapshot" || f.SourceClassCode != "30" {
				t.Fatal("quote arithmetic or status changed", f)
			}
		}
	}
	if !found {
		t.Fatal("source fare missing")
	}
	if q.InventoryGuaranteed {
		t.Fatal("must not guarantee inventory")
	}
	wrong, _ := ParseDate("2026-10-16")
	if _, e := ParseQuote(fixture(t, "quote-osaka-beppu.html"), Registry[0], "21", wrong, Party{Adults: 1, Mode: "foot"}); e == nil {
		t.Fatal("source date mismatch accepted")
	}
	if _, e := ParseQuote(fixture(t, "quote-osaka-beppu.html"), Registry[0], "22", d, Party{Adults: 1, Mode: "foot"}); e == nil {
		t.Fatal("source direction mismatch accepted")
	}
	if _, e := ParseQuote([]byte(`<h1>Error</h1><p>Time-out has occurred.</p>`), Registry[0], "21", d, Party{}); e == nil || !strings.Contains(e.Error(), "expired") {
		t.Fatal("timeout page must not become empty availability")
	}
}

func TestKobeInformationalLinksAndVehicleSnapshot(t *testing.T) {
	// Heading, source prices and vehicle symbol came from the independent live
	// 2026-10-15 two-adult/one-child <5m car lookup. This is a parser regression,
	// not a replacement for that live evidence.
	b := []byte(`<h2>Total Price</h2><table><tr><th>Season</th><th>Applicable discount</th></tr><tr><td>Ａ</td><td>Web DC(Partcal 5%)</td></tr></table>
<h2>10/15(Thu) Kobe 19:00 Departure &gt;&gt; Oita 06:20 Arrival （SUNFLOWER PEARL） <a href="#classes">About Classes</a> <a href="#ferries">About Ferries</a></h2>
<table><tr><th>Room type</th><th>Class</th><th>Image</th><th>Discounted Price</th><th>Vacant Seat/Room</th><th>Info</th></tr>
<tr><th>private</th><th>Deluxe room (2-4)</th><td></td><td>\85,440</td><td>○</td><td></td></tr>
<tr><th>dormitory</th><th>Tourist room</th><td></td><td>\45,590</td><td>3</td><td></td></tr></table>
<table><tr><th></th><th><p><strong>Vehicle</strong></p></th><th></th><th></th><th><p><strong>○</strong></p></th><th></th></tr></table>`)
	d, _ := ParseDate("2026-10-15")
	q, e := ParseQuote(b, Registry[1], "11", d, Party{Adults: 2, Children: 1, Mode: "car", CarCategory: "lt5m"})
	if e != nil {
		t.Fatal(e)
	}
	s := q.Sailings[0]
	if s.Ship != "SUNFLOWER PEARL" || s.Arrival != "2026-10-16T06:20:00+09:00" || len(s.CabinFares) != 2 || s.CabinFares[0].DisplayedPriceJPY != 85440 {
		t.Fatal("source heading or party fare lost", s)
	}
	if s.VehicleAvailability == nil || s.VehicleAvailability.Symbol != "○" || s.VehicleAvailability.Status != "available_snapshot" || s.VehicleAvailability.Guaranteed {
		t.Fatal("vehicle snapshot missing or guaranteed", s.VehicleAvailability)
	}
	// The live motorcycle result uses the same six-cell layout with Motorbike.
	bikeRow := bytes.ReplaceAll(b, []byte("Vehicle"), []byte("Motorbike"))
	bike, e := ParseQuote(bikeRow, Registry[1], "11", d, Party{Adults: 1, Mode: "bike", BikeCategory: "over750cc", Bikes: 1})
	if e != nil || bike.Sailings[0].VehicleAvailability == nil || bike.Sailings[0].VehicleAvailability.SourceLabel != "Motorbike" {
		t.Fatal("source motorcycle row lost", e, bike)
	}
}

func TestRouteSearchIncludesTerminalIDsAndJapanesePorts(t *testing.T) {
	for _, tc := range []struct{ query, route string }{{"osaka-terminal2", "osaka-shibushi"}, {"第1ターミナル", "osaka-beppu"}, {"大分港", "kobe-oita"}, {"12", "kobe-oita"}} {
		got := SearchRoutes(tc.query)
		if tc.route == "" {
			if len(got) != 0 {
				t.Fatal("unrelated route match", got)
			}
			continue
		}
		if len(got) != 1 || got[0].ID != tc.route {
			t.Fatal(tc.query, got)
		}
	}
}
func TestDaytimeArrivalAndYearRollover(t *testing.T) {
	b := string(fixture(t, "quote-osaka-beppu.html"))
	b = strings.ReplaceAll(b, "19:05", "09:15")
	b = strings.ReplaceAll(b, "06:55", "20:10")
	d, _ := ParseDate("2026-10-15")
	q, e := ParseQuote([]byte(b), Registry[0], "21", d, Party{Adults: 1, Mode: "foot"})
	if e != nil {
		t.Fatal(e)
	}
	if q.Sailings[0].Arrival != "2026-10-15T20:10:00+09:00" || q.Sailings[0].ArrivalDayOffset != 0 {
		t.Fatal("daytime arrival incorrectly rolled forward")
	}
	b = string(fixture(t, "quote-osaka-beppu.html"))
	b = strings.ReplaceAll(b, "10/15(Thu)", "12/31(Thu)")
	d, _ = ParseDate("2026-12-31")
	q, e = ParseQuote([]byte(b), Registry[0], "21", d, Party{Adults: 1, Mode: "foot"})
	if e != nil {
		t.Fatal(e)
	}
	if q.Sailings[0].Arrival != "2027-01-01T06:55:00+09:00" {
		t.Fatal("year rollover lost")
	}
}
func TestCalendarDirectionCoverageAndUnknowns(t *testing.T) {
	js := []byte(`const calendarEvents1=[{title:'A',start:'2026-12-31'},{title:'E',start:'2027-01-01'}]; const calendarEvents2=[{title:'B',start:'2026-12-31'}];`)
	from, _ := ParseDate("2026-12-31")
	to, _ := ParseDate("2027-01-02")
	a, e := ParseCalendar(js, "1", from, to)
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Dates) != 2 || !a.Dates[1].DaytimeCruise || a.Complete || len(a.MissingDates) != 1 || a.MissingDates[0] != "2027-01-02" {
		t.Fatal("calendar gaps/daytime were inferred away", a)
	}
	b, e := ParseCalendar(js, "2", from, to)
	if e != nil {
		t.Fatal(e)
	}
	if b.Dates[0].Band != "B" || len(b.Dates) != 1 {
		t.Fatal("direction arrays mixed", b)
	}
	if _, e := ParseCalendar(js, "4", from, to); e == nil {
		t.Fatal("missing array must fail")
	}
}
func TestCabinAndPortEvidence(t *testing.T) {
	for _, r := range Registry {
		rooms, e := ParseCabins(fixture(t, r.ID+"-cabin.html"))
		if e != nil {
			t.Fatal(r.ID, e)
		}
		if len(rooms) < 4 {
			t.Fatal("cabin scope incomplete", r.ID, len(rooms))
		}
		for _, room := range rooms {
			if room.OccupancySource == "" || room.ID == "" {
				t.Fatal("missing evidence", room)
			}
		}
		ports, e := ParsePorts(fixture(t, r.ID+"-boarding.html"), r)
		if e != nil {
			t.Fatal(r.ID, e)
		}
		if ports[0].Port.ID != r.Origin.ID || ports[1].Port.ID != r.Destination.ID || ports[0].Address == "" {
			t.Fatal("port identity lost")
		}
	}
	if Registry[0].Origin.ID == Registry[2].Origin.ID {
		t.Fatal("distinct Osaka terminals merged")
	}
}
func TestPrivateCabinIdentityDoesNotUseDescriptionSubstrings(t *testing.T) {
	rooms, err := ParseCabins(fixture(t, "osaka-beppu-cabin.html"))
	if err != nil {
		t.Fatal(err)
	}
	wanted := map[string]struct {
		category string
		max      int
	}{
		"Private single twin (4 rooms)":                  {"semi_private", 2},
		"Private twin (4 rooms)":                         {"semi_private", 2},
		"Private bed group (12 rooms in 3 compartments)": {"private", 4},
		"Private bed group barrier-free (1 room)":        {"private", 6},
	}
	for _, room := range rooms {
		if w, ok := wanted[room.Name]; ok {
			if room.Category != w.category || room.MaxOccupancy == nil || *room.MaxOccupancy != w.max {
				t.Fatalf("source privacy/capacity lost: %+v", room)
			}
			delete(wanted, room.Name)
		}
		if strings.HasPrefix(room.Name, "Private single (2 lots)") && (room.Category != "dormitory" || room.MaxOccupancy != nil) {
			t.Fatalf("shared section became private: %+v", room)
		}
	}
	if len(wanted) != 0 {
		t.Fatal("source room cases missing", wanted)
	}
	for _, route := range []string{"kobe-oita", "osaka-shibushi"} {
		rooms, err := ParseCabins(fixture(t, route+"-cabin.html"))
		if err != nil {
			t.Fatal(route, err)
		}
		shared := 0
		for _, room := range rooms {
			if strings.HasPrefix(room.Name, "Private") {
				shared++
				if room.Category != "dormitory" || room.MinOccupancy != nil || room.MaxOccupancy != nil {
					t.Fatalf("%s shared section became private: %+v", route, room)
				}
			}
		}
		if shared == 0 {
			t.Fatal("shared source cases missing", route)
		}
	}
}
func TestTerminalIdentitySurvivesReorderedSourceBoxes(t *testing.T) {
	box := func(p Terminal) string {
		return `<div class="tabBox"><h2>` + html.EscapeString(p.SourceName) + `</h2><table><tr><th>Address</th><td>` + html.EscapeString(p.Address) + `</td></tr></table></div>`
	}
	for _, r := range Registry {
		original, err := ParsePorts(fixture(t, r.ID+"-boarding.html"), r)
		if err != nil {
			t.Fatal(r.ID, err)
		}
		reversed := `<article>` + box(original[1]) + box(original[0]) + `</article>`
		got, err := ParsePorts([]byte(reversed), r)
		if err != nil || got[0].Port.ID != r.Origin.ID || got[0].Address != original[0].Address || got[1].Port.ID != r.Destination.ID || got[1].Address != original[1].Address {
			t.Fatal("source order changed identity", r.ID, err, got)
		}
		unknown := strings.ReplaceAll(reversed, html.EscapeString(original[0].SourceName), "Unidentified terminal")
		if _, err := ParsePorts([]byte(unknown), r); err == nil {
			t.Fatal("unidentified terminal guessed", r.ID)
		}
		if _, err := ParsePorts([]byte(`<article>`+box(original[0])+box(original[0])+`</article>`), r); err == nil {
			t.Fatal("duplicate terminal guessed", r.ID)
		}
		misleading := strings.ReplaceAll(reversed, html.EscapeString(original[0].SourceName), "Not "+html.EscapeString(original[0].SourceName))
		if _, err := ParsePorts([]byte(misleading), r); err == nil {
			t.Fatal("negated terminal guessed", r.ID)
		}
	}
	if terminalNameMatches("sunflower terminal (osaka) Terminal10", Registry[0].Origin) {
		t.Fatal("Terminal10 mistaken for Terminal1")
	}
	if terminalNameMatches("Not Oita Port", Registry[1].Destination) {
		t.Fatal("negative destination name accepted")
	}
}
func TestConditionsDeriveRefundMinimumAndBaggageDefinitions(t *testing.T) {
	en := fixture(t, "reservation.html")
	ja := fixture(t, "passenger-conditions.html")
	c, e := ParseConditions(en, ja)
	if e != nil {
		t.Fatal(e)
	}
	if *c.Cancellation[1].Percent != 10 || *c.Cancellation[1].MinimumJPY != 200 || *c.Cancellation[2].Percent != 30 {
		t.Fatal("normal-ticket cancellation bands wrong", c.Cancellation)
	}
	if c.Baggage["carried_item_max_weight_kg"] != 30 || c.Baggage["free_carried_baggage_aggregate_weight_kg"] != 20 {
		t.Fatal("carried item and aggregate free weight merged")
	}
	changed := []byte(strings.ReplaceAll(string(ja), "200円", "300円"))
	c, e = ParseConditions(en, changed)
	if e != nil {
		t.Fatal(e)
	}
	if *c.Cancellation[0].FixedJPY != 300 || *c.Cancellation[1].MinimumJPY != 300 {
		t.Fatal("hardcoded source refund fee")
	}
}
func TestValidationAndReservationBoundary(t *testing.T) {
	for _, p := range []Party{{Adults: 0, Mode: "foot"}, {Adults: 15, Mode: "foot"}, {Adults: 1, Mode: "car", CarCategory: "6m"}, {Adults: 1, Mode: "bike", BikeCategory: "bicycle", Bikes: 2}, {Adults: 1, Mode: "foot", CarCategory: "lt5m"}, {Adults: 1, Mode: "foot", PetCages: 3}} {
		if p.Validate() == nil {
			t.Fatal("invalid party accepted", p)
		}
	}
	for _, p := range []Party{{Adults: 1, Mode: "foot"}, {Adults: 2, Children: 1, Mode: "car", CarCategory: "lt5m"}, {Adults: 1, Mode: "bike", BikeCategory: "le125cc", Bikes: 1}} {
		if e := p.Validate(); e != nil {
			t.Fatal(e)
		}
	}
	for _, path := range []string{"/web/yoyaku/Reserve1020/MoveNext", "/web/yoyaku/Reserve0000/ReserveWithLogin", "/web/yoyaku/Reserve0000/Inquiry", "/web/yoyaku/Reserve1030/MovePrevious"} {
		u, _ := url.Parse(BookingBase + path)
		if allowedURL(u, "POST") {
			t.Fatal("reservation/account step accessible", path)
		}
	}
	for _, s := range []string{"http://booking.ferry-sunflower.co.jp/web/yoyaku/Reserve1030/MoveNext", "https://attacker.example/en/reservation/", "https://www.ferry-sunflower.co.jp/en/reservation/?token=x"} {
		u, _ := url.Parse(s)
		if allowedURL(u, "GET") {
			t.Fatal("unsafe URL allowed", s)
		}
	}
	c := New(2)
	if _, e := c.Quote(context.Background(), Registry[0], "21", time.Now().In(JST).AddDate(0, 0, 2).Format("2006-01-02"), Party{Adults: 0, Mode: "foot"}); e == nil || c.requests != 0 {
		t.Fatal("invalid input reached transport")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSourceMaintenanceStopsBeforeAnonymousPost(t *testing.T) {
	c := New(2)
	c.http.Transport = &boundedTransport{client: c, next: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.URL.String() != BookingURL {
			t.Fatal("maintenance reached a planning POST", r.Method, r.URL)
		}
		body := `<html><body><h1>Under maintenance.</h1><p>We are sorry, but reservation system is not available for maintenance.</p></body></html>`
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}
	q, err := c.Quote(context.Background(), Registry[0], "21", time.Now().In(JST).AddDate(0, 0, 2).Format("2006-01-02"), Party{Adults: 1, Mode: "foot"})
	if err == nil || !strings.Contains(err.Error(), "under maintenance") || c.requests != 1 || len(q.Sailings) != 0 {
		t.Fatal("maintenance response was mistaken for inventory or retried", err, c.requests, q)
	}
}

func TestRateLimitAndBodyCapsReturnErrors(t *testing.T) {
	c := New(2)
	c.http.Transport = &boundedTransport{client: c, next: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": []string{"2"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})}
	_, e := c.fetch(context.Background(), Registry[0].SourceURL, nil)
	var rate *cliutil.RateLimitError
	if !errors.As(e, &rate) {
		t.Fatal("throttle lost typed error", e)
	}
	c = New(2)
	c.http.Transport = &boundedTransport{client: c, next: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", maxBody+1))), Request: r}, nil
	})}
	if _, e := c.fetch(context.Background(), Registry[0].SourceURL, nil); e == nil {
		t.Fatal("oversized response accepted")
	}
}
