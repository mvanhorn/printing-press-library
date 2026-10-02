package jbo

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/net/html"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-bus-online/internal/cliutil"
)

func doc(t *testing.T, s string) *html.Node {
	t.Helper()
	d, e := html.Parse(strings.NewReader(s))
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func TestTimeAndAvailability(t *testing.T) {
	for _, tc := range []struct{ clock, want string }{{"23:25", "2026-10-10T23:25:00+09:00"}, {"24:07", "2026-10-11T00:07:00+09:00"}, {"25:00", "2026-10-11T01:00:00+09:00"}} {
		got, e := Timestamp("2026-10-10", tc.clock)
		if e != nil || got != tc.want {
			t.Fatalf("clock %s = %s, %v", tc.clock, got, e)
		}
	}
	if _, e := Timestamp("2026-10-10", "23:61"); e == nil {
		t.Fatal("invalid minute accepted")
	}
	if _, e := ParseDate("2026-02-30"); e == nil {
		t.Fatal("invalid date accepted")
	}
	for _, tc := range []struct{ raw, status string }{{"Not Available", "unavailable_unknown_reason"}, {"Sold out", "sold_out"}, {"Not on sale", "not_on_sale"}, {"more than 5 seats left", "available"}, {"1 seat left", "available"}, {"unknown", "unknown"}} {
		a := AvailabilityOf(tc.raw)
		if a.Status != tc.status {
			t.Fatalf("%s => %s", tc.raw, a.Status)
		}
	}
	a := AvailabilityOf("more than 5 seats left")
	if a.Exact != nil || a.LowerBound != 6 {
		t.Fatal("capped count asserted exact")
	}
	if PartyEvidence(a, 7, 0)["status"] != "unknown" {
		t.Fatal("lower bound treated as exact capacity")
	}
	if PartyEvidence(a, 4, 3)["status"] != "exceeds_transaction_limit" {
		t.Fatal("source transaction cap ignored")
	}
}
func TestInventoryAttributesDeduplicateAndDate(t *testing.T) {
	row := `<div class="cal_usedays" data-route="0001" data-depdate="20261010" data-deptime="2325" data-arrdate="20261011" data-arrtime="0705" data-coursecd="12200160001" data-updownflg="0"><div class="text_mobile">23:25</div><div class="text_mobile">07:05</div><div class="text_mobile">Hamamatsu Sta.</div><div class="text_mobile">To</div><div class="text_mobile">Tokyo DisneySea</div><div class="text_mobile">more than 5 seats left</div><div class="text_mobile">JPY 6,100～</div></div>`
	s, effective, _, e := ParseServices(doc(t, `<input id="SelectDate" value="10/10/2026">`+row+row))
	if e != nil || len(s) != 1 || effective != "2026-10-10" {
		t.Fatalf("parse %v %s %v", s, effective, e)
	}
	if s[0].Arrival != "2026-10-11T07:05:00+09:00" || s[0].FromFare != 6100 {
		t.Fatal(s[0])
	}
}
func TestStopIDsNamesAndOvernight(t *testing.T) {
	s := Service{DepDate: "2026-10-10", ArrDate: "2026-10-11"}
	st, e := ParseStops(doc(t, `<div class="row"><input name="DepBusStop" value="8,Tomei Kakegawa,25:00"><a href="https://maps.google.co.jp/maps?q=34,138">Map</a></div><input name="ArvBusStop" value="12,Tokyo DisneySea,07:05">`), s)
	if e != nil || len(st) != 2 || st[0].ID != "8" || st[0].Timestamp != "2026-10-11T01:00:00+09:00" || st[1].Timestamp != "2026-10-11T07:05:00+09:00" {
		t.Fatalf("%v %v", st, e)
	}
}
func TestReplaySessionAndDateSubstitution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("mutating method %s", r.Method)
		}
		if strings.Contains(r.URL.Path, "CourseSearch") {
			http.SetCookie(w, &http.Cookie{Name: "temporary", Value: "test", Path: "/"})
			w.Write([]byte(`<div id="122001600010"><h1>route</h1><a href="/en/Detail/12200160001/0/1/2/">Select</a><table class="time_table"><tr><th><span class="text_blk12">A</span></th><td>Dep. 23:25</td></tr><tr><th><span class="text_blk12">B</span></th><td>Arr. 07:05</td></tr></table></div>`))
			return
		}
		if _, e := r.Cookie("temporary"); e != nil {
			t.Error("ephemeral session missing")
		}
		w.Write([]byte(`<div class="text_strong">Bookable Days of Operation<br>10/2/2026 ～ 12/2/2026</div><input id="SelectDate" value="10/02/2026">`))
	}))
	defer server.Close()
	c, _ := New("en")
	c.Base = server.URL
	out, sv, _, e := c.Services(context.Background(), "12200160001", 0, "2026-09-30")
	if e != nil || len(sv) != 0 || out["status"] != "not_on_sale_or_outside_window" {
		t.Fatalf("%v %v", out, e)
	}
	if out["effective_source_date"] == out["requested_date"] {
		t.Fatal("source substitution hidden")
	}
}

func TestFareTableCapacityAndLabels(t *testing.T) {
	for _, tc := range []struct {
		name, source, status string
		exact, lower         any
	}{
		{"positive_count_may_be_capped", "Seat Availability : 6", "available", nil, 6},
		{"small_count_conservative", "Seat Availability : 2", "available", nil, 2},
		{"zero", "Seat Availability : 0", "sold_out", 0, nil},
		{"missing", "Please select a number of passengers", "unknown", nil, nil},
		{"malformed_large_count", "Seat Availability : 999999999999999999999999", "unknown", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := ParseFareAvailability(doc(t, tc.source))
			if a.Status != tc.status || a.Exact != tc.exact || a.LowerBound != tc.lower {
				t.Fatalf("capacity = %+v", a)
			}
		})
	}
	fares, limit, err := ParseFares(doc(t, `<input id="Fare_PassengerName0" value="Adult"><input id="Fare_Passenger0" value="6100"><input id="Fare_PassengerName1" value="Child(6-12)"><input id="Fare_Passenger1" value="3050"><div>Max number of tickets per transaction : 4</div>`))
	if err != nil || limit != 4 || fares["adult"].(map[string]any)["unit_jpy"] != 6100 || fares["child"].(map[string]any)["label"] != "Child(6-12)" {
		t.Fatalf("fare labels: %v, %d, %v", fares, limit, err)
	}
	if _, _, err := ParseFares(doc(t, `<input id="Fare_PassengerName0" value="Adult"><input id="Fare_Passenger0" value="broken">`)); err == nil {
		t.Fatal("malformed source fare silently became free")
	}
}

func TestQuoteUsesSelectedPairCapacityAndFirstAvailableService(t *testing.T) {
	for _, tc := range []struct {
		seats int
		want  string
	}{{0, "capacity_insufficient"}, {2, "unknown"}, {6, "capacity_sufficient"}} {
		t.Run(fmt.Sprint(tc.seats), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.Contains(r.URL.Path, "CourseSearch"):
					fmt.Fprint(w, `<div id="122001600010"><h1>Route</h1><a href="/en/Detail/12200160001/0/1/2/">Select</a></div>`)
				case strings.Contains(r.URL.Path, "SelectRoute"):
					if !strings.Contains(r.URL.Path, "/0001/") {
						t.Error("default did not select available service")
					}
					fmt.Fprint(w, `<input name="radioBtn" value="1/2/3/">`)
				case strings.Contains(r.URL.Path, "SelectFABN"):
					if r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
						t.Error("AJAX header missing")
					}
					fmt.Fprint(w, `<input name="DepBusStop" value="8,Tomei Kakegawa,25:00"><input name="ArvBusStop" value="9,Shinjuku,06:00">`)
				case strings.Contains(r.URL.Path, "GetFareTable"):
					if !strings.HasSuffix(r.URL.Path, "/8/9/0") {
						t.Error("wrong stop pair")
					}
					fmt.Fprintf(w, `<div>Seat Availability : %d</div><input id="Fare_PassengerName0" value="Adult"><input id="Fare_Passenger0" value="6100"><input id="Fare_PassengerName1" value="Child(6-12)"><input id="Fare_Passenger1" value="3050"><div>Max number of tickets per transaction : 4</div>`, tc.seats)
				default:
					fmt.Fprint(w, `<input id="SelectDate" value="10/10/2026">`)
					for _, row := range []struct{ id, availability string }{{"0000", "Sold out"}, {"0001", "more than 5 seats left"}} {
						fmt.Fprintf(w, `<div data-route="%s" data-depdate="20261010" data-deptime="2325" data-arrdate="20261011" data-arrtime="0705" data-coursecd="12200160001" data-updownflg="0">%s</div>`, row.id, row.availability)
					}
				}
			}))
			defer server.Close()
			c, _ := New("en")
			c.Base, c.limiter = server.URL, nil
			out, err := c.Quote(context.Background(), "12200160001", 0, "2026-10-10", "", "8", "9", 2, 1, 1)
			if err != nil {
				t.Fatal(err)
			}
			if out["party"].(map[string]any)["status"] != tc.want || out["estimated_total_jpy"] != 15250 || out["boarding"].(*Stop).Timestamp != "2026-10-11T01:00:00+09:00" {
				t.Fatalf("quote = %+v", out)
			}
			if out["cancellation_fee_conditions"] != nil {
				t.Fatal("missing cancellation fees invented")
			}
			if out["fare_basis"] != nil || out["headline_fare_basis"] == nil || out["price_basis"] == nil {
				t.Fatalf("headline and selected-pair fare scope ambiguous: %+v", out)
			}
			c2, _ := New("en")
			c2.Base, c2.limiter = server.URL, nil
			unavailable, err := c2.Quote(context.Background(), "12200160001", 0, "2026-10-10", "0000", "8", "9", 2, 1, 1)
			if err != nil || unavailable["party"].(map[string]any)["status"] != "unknown" {
				t.Fatalf("headline used as selected pair capacity: %v, %v", unavailable, err)
			}
			if unavailable["fare_basis"] != nil || unavailable["headline_fare_basis"] == nil {
				t.Fatalf("unavailable quote fare scope ambiguous: %+v", unavailable)
			}
		})
	}
}

func TestRateLimitIsTypedAndTimeoutPropagates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	c, _ := New("en")
	c.Base = server.URL
	_, _, err := c.Get(context.Background(), "/en/AllRouteList")
	var rateErr *cliutil.RateLimitError
	if !errors.As(err, &rateErr) || rateErr.RetryAfter.Seconds() != 3 {
		t.Fatalf("rate error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = c.Get(ctx, "/en/AllRouteList")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("timeout was swallowed: %v", err)
	}
}

func TestHTTP200MaintenanceIsAnOutageNotInventory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<div>We are sorry for the inconvenience. Our website is currently under maintenance from 10/02/2026 02:01 to 10/02/2026 04:59.</div>`)
	}))
	defer server.Close()
	c, _ := New("en")
	c.Base = server.URL
	_, err := c.Routes(context.Background(), "", 0, 20)
	var maintenance *MaintenanceError
	if !errors.As(err, &maintenance) || maintenance.StartJST != "2026-10-02T02:01:00+09:00" || maintenance.EndJST != "2026-10-02T04:59:00+09:00" {
		t.Fatalf("HTTP200 outage was misclassified: %v", err)
	}
}

func TestInventoryRowsMustMatchRequestedIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, route, date, dir string
		valid                  bool
	}{
		{"matching", "12200160001", "20261010", "0", true},
		{"wrong_day", "12200160001", "20261011", "0", false},
		{"wrong_route", "999", "20261010", "0", false},
		{"wrong_direction", "12200160001", "20261010", "1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "CourseSearch") {
					fmt.Fprint(w, `<div id="122001600010"><a href="/en/Detail/12200160001/0/1/2/">Select</a></div>`)
					return
				}
				fmt.Fprintf(w, `<input id="SelectDate" value="10/10/2026"><div data-route="0001" data-coursecd="%s" data-updownflg="%s" data-depdate="%s" data-deptime="2325" data-arrdate="20261012" data-arrtime="0705">more than 5 seats left</div>`, tc.route, tc.dir, tc.date)
			}))
			defer server.Close()
			c, _ := New("en", 0)
			c.Base = server.URL
			_, rows, _, err := c.Services(context.Background(), "12200160001", 0, "2026-10-10")
			if tc.valid && (err != nil || len(rows) != 1) {
				t.Fatalf("matching row rejected: %v", err)
			}
			if !tc.valid && (err == nil || len(rows) != 0 || !strings.Contains(err.Error(), "identity mismatch")) {
				t.Fatalf("wrong identity offered as requested-day inventory: %v %v", rows, err)
			}
		})
	}
}

func TestProviderRateLimitModes(t *testing.T) {
	for _, tc := range []struct {
		name            string
		input, expected float64
		disabled        bool
	}{
		{"auto", -1, 2, false},
		{"explicit", 0.5, 0.5, false},
		{"disabled", 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := New("en", tc.input)
			if err != nil || (c.limiter == nil) != tc.disabled {
				t.Fatalf("limiter mode: %+v %v", c, err)
			}
			if !tc.disabled && c.limiter.Rate() != tc.expected {
				t.Fatalf("requested rate %v became %v", tc.input, c.limiter.Rate())
			}
		})
	}
	for _, invalid := range []float64{math.NaN(), math.Inf(1)} {
		if _, err := New("en", invalid); err == nil {
			t.Fatal("nonfinite limiter accepted")
		}
	}
}

func TestOutsideWindowFallbackRowsStayNotOnSale(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "CourseSearch") {
			fmt.Fprint(w, `<div id="122001600010"><a href="/en/Detail/12200160001/0/1/2/">Select</a></div>`)
			return
		}
		fmt.Fprint(w, `<div class="text_strong">Bookable Days of Operation<br>10/2/2026 ～ 12/2/2026</div><input id="SelectDate" value="09/30/2026"><div data-route="0001" data-coursecd="12200160001" data-updownflg="0" data-depdate="20261002" data-deptime="2325" data-arrdate="20261003" data-arrtime="0705">more than 5 seats left</div>`)
	}))
	defer server.Close()
	c, _ := New("en", 0)
	c.Base = server.URL
	out, rows, _, err := c.Services(context.Background(), "12200160001", 0, "2026-09-30")
	if err != nil || len(rows) != 0 || out["status"] != "not_on_sale_or_outside_window" {
		t.Fatalf("outside-window fallback: %v %v", out, err)
	}
}

func TestDeclaredRatePacesRequestsAndObservesHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "headers") {
			w.Header().Set("X-Ratelimit-Remaining", "1")
			w.Header().Set("X-Ratelimit-Reset", "3")
		}
		fmt.Fprint(w, `<div>Public response</div>`)
	}))
	defer server.Close()
	c, _ := New("en", 0.5)
	c.Base = server.URL
	if _, _, err := c.Get(context.Background(), "/first"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, _, err := c.Get(ctx, "/second"); !errors.Is(err, context.DeadlineExceeded) || c.Requests != 1 {
		t.Fatalf("declared ceiling did not pace second request: %v, requests %d", err, c.Requests)
	}
	auto, _ := New("en", -1)
	auto.Base = server.URL
	if _, _, err := auto.Get(context.Background(), "/headers"); err != nil {
		t.Fatal(err)
	}
	if auto.limiter.Rate() <= 0 || auto.limiter.Rate() > 0.5 {
		t.Fatalf("server budget ignored: %v", auto.limiter.Rate())
	}
}
