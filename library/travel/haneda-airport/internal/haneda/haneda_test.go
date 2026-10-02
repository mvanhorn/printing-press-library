package haneda

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/haneda-airport/internal/cliutil"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func testNow() time.Time { return time.Date(2026, 10, 2, 23, 49, 40, 0, JST) }
func testQuery() Query {
	return Query{Kind: "international", Direction: "departure", Date: "2026-10-02", Limit: 20, MaxScan: 5000}
}
func sourceServer(t *testing.T, hook func(http.ResponseWriter, *http.Request) bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hook != nil && hook(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/en/app/api/v2/flight/search" {
			if r.Method != "POST" {
				t.Error("search must be a read-only POST")
			}
			var p map[string]any
			if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
				t.Error(err)
			}
			if p["searchDt"] != "20261002" {
				t.Errorf("wire search date = %v", p["searchDt"])
			}
			for _, k := range []string{"airportCodes", "airlineCodes", "status"} {
				if _, ok := p[k].([]any); !ok {
					t.Errorf("%s must be a JSON array", k)
				}
			}
			kind := "international"
			if p["flightType"] == float64(1) {
				kind = "domestic"
			}
			direction := "departures"
			if p["arrivalType"] == float64(2) {
				direction = "arrivals"
			}
			b := fixture(t, kind+"-"+direction)
			if p["exactMatch"] == true {
				if _, ok := p["arrivalType"]; ok {
					t.Error("detail must omit arrivalType")
				}
				var v RawBoard
				_ = json.Unmarshal(b, &v)
				if p["flightNumber"] == v.Flights[0].Airlines[0].Number {
					v.Flights = v.Flights[:1]
				} else {
					v.Flights = []RawFlight{}
				}
				n := len(v.Flights)
				v.Count = &n
				b, _ = json.Marshal(v)
			}
			_, _ = w.Write(b)
			return
		}
		if strings.Contains(r.URL.Path, "flight_status.json") {
			_, _ = w.Write(fixture(t, "disruption-summary"))
			return
		}
		kind := "international"
		if strings.Contains(r.URL.Path, "/dms/") {
			kind = "domestic"
		}
		name := ""
		switch {
		case strings.HasSuffix(r.URL.Path, "city_list_search.json"):
			name = kind + "-city"
		case strings.HasSuffix(r.URL.Path, "company_list_search.json"):
			name = kind + "-company"
		case strings.HasSuffix(r.URL.Path, "hdacfdsc.json"):
			name = "monthly-" + kind + "-departures"
		case strings.HasSuffix(r.URL.Path, "hdacfasc.json"):
			name = "monthly-" + kind + "-arrivals"
		}
		if name == "" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(fixture(t, name))
	}))
}

func TestNewClientAndBudgets(t *testing.T) {
	for _, tc := range []struct {
		origin string
		ok     bool
	}{{Origin, true}, {"http://127.0.0.1:8080", true}, {"https://user:secret@example.com", false}, {"https://example.com?token=secret", false}, {"file:///tmp/source", false}, {"not a URL", false}} {
		t.Run(tc.origin, func(t *testing.T) {
			c, err := NewClient(tc.origin, 0, 0)
			if (err == nil) != tc.ok {
				t.Fatalf("NewClient error %v", err)
			}
			if tc.ok && (c.HTTP.Timeout != 30*time.Second || c.Budget().MaxRequests != 20) {
				t.Fatal("command budget/default timeout missing")
			}
		})
	}
}
func TestNormalizeNumberAndIdentity(t *testing.T) {
	for _, tc := range []struct {
		in, want string
		ok       bool
	}{{"NH 0849", "NH0849", true}, {"6J-21", "6J21", true}, {"JL91", "JL91", true}, {"NH", "", false}, {"1234", "", false}, {"NH849;echo secret", "", false}} {
		n, err := NormalizeNumber(tc.in)
		if (err == nil) != tc.ok || n != tc.want {
			t.Errorf("NormalizeNumber(%q)=%q,%v", tc.in, n, err)
		}
	}
	for _, tc := range []struct {
		id string
		ok bool
	}{{"hnd:international:departure:20261002:NH849", true}, {"hnd:domestic:arrival:20261002:6J21", true}, {"hnd:international:departure:20260230:NH849", false}, {"hnd:int:departure:20261002:NH849", false}, {"NH849", false}} {
		kind, direction, date, number, err := ParseIdentity(tc.id)
		if (err == nil) != tc.ok {
			t.Errorf("ParseIdentity(%q): %v", tc.id, err)
		}
		if tc.ok && (kind == "" || direction == "" || date != "2026-10-02" || number == "") {
			t.Fatal("identity fields lost")
		}
	}
}
func TestValidateSourceDatesAndBounds(t *testing.T) {
	for _, tc := range []struct {
		name, kind, date string
		limit            int
		ok               bool
	}{{"today", "domestic", "2026-10-02", 20, true}, {"international yesterday", "international", "2026-10-01", 20, true}, {"domestic yesterday", "domestic", "2026-10-01", 20, false}, {"calendar maximum", "international", "2027-01-02", 20, true}, {"beyond maximum", "international", "2027-01-03", 20, false}, {"invalid calendar", "international", "2026-02-30", 20, false}, {"unbounded output", "international", "2026-10-02", 0, false}, {"excessive output", "international", "2026-10-02", 201, false}} {
		t.Run(tc.name, func(t *testing.T) {
			q := testQuery()
			q.Kind, q.Date, q.Limit = tc.kind, tc.date, tc.limit
			if err := ValidateQuery(q, testNow(), true); (err == nil) != tc.ok {
				t.Fatalf("date/budget validation %v", err)
			}
		})
	}
	q := testQuery()
	q.Date = "2020-01-01"
	if err := ValidateQuery(q, testNow(), false); err != nil {
		t.Fatal("offline history should remain queryable")
	}
	q.Destination = "---"
	if err := ValidateQuery(q, testNow(), false); err == nil {
		t.Fatal("punctuation must not match every airport")
	}
	q.Destination = ""
	for _, status := range []string{"canceled,", ",delayed", "canceled,-"} {
		q.Status = status
		if err := ValidateQuery(q, testNow(), false); err == nil {
			t.Fatalf("malformed status CSV %q must be rejected", status)
		}
	}
}

func TestFetchBoardFourScopesAndCodeshares(t *testing.T) {
	server := sourceServer(t, nil)
	defer server.Close()
	for _, kind := range []string{"domestic", "international"} {
		for _, direction := range []string{"departure", "arrival"} {
			t.Run(kind+" "+direction, func(t *testing.T) {
				c, _ := NewClient(server.URL, time.Second, 0)
				q := testQuery()
				q.Kind, q.Direction = kind, direction
				r, err := c.FetchBoard(context.Background(), q, testNow(), "")
				if err != nil {
					t.Fatal(err)
				}
				if len(r.Flights) == 0 || !r.Complete || r.Budget.Requests != 3 || r.Budget.ResponseBytes <= 0 {
					t.Fatalf("source scope not exercised: %+v", r.Budget)
				}
				for _, f := range r.Flights {
					if f.Kind != kind || f.Direction != direction || f.ActualAt != nil || f.OperatingFlight != nil || f.SourcePrimaryFlight != f.ListedFlights[0].Number {
						t.Fatal("group identity or unknown-time/operator semantics changed")
					}
				}
			})
		}
	}
	c, _ := NewClient(server.URL, time.Second, 0)
	q := testQuery()
	q.Direction = "both"
	q.Flight = "UA8003"
	r, err := c.FetchBoard(context.Background(), q, testNow(), "UA8003")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Flights) != 0 {
		t.Fatal("provider exact lookup is source-primary-only")
	}
	r, err = c.FetchBoard(context.Background(), q, testNow(), "")
	if err != nil {
		t.Fatal(err)
	}
	filtered := FilterBoard(r, q, testNow())
	if len(filtered.Flights) != 1 || filtered.Flights[0].SourcePrimaryFlight != "NH849" || filtered.Flights[0].ID != "hnd:international:departure:20261002:NH849" {
		t.Fatal("marketing lookup must not reorder the source primary")
	}
}
func TestExplicitRolloverAndFacilityRoles(t *testing.T) {
	var b RawBoard
	_ = json.Unmarshal(fixture(t, "international-departures"), &b)
	f, err := normalizeFlight(b.Flights[0], []Airport{{Code: "BKK", Name: "BANGKOK", NameJA: "バンコク"}}, nil, parseTimestamp(b.Date.Date))
	if err != nil {
		t.Fatal(err)
	}
	if f.ScheduledAt == nil || *f.ScheduledAt != "2026-10-02T00:05:00+09:00" || f.RevisedAt == nil || *f.RevisedAt != "2026-10-01T23:59:00+09:00" || f.TimeChangeMinutes == nil || *f.TimeChangeMinutes != -6 {
		t.Fatalf("explicit midnight dates were guessed: %+v", f)
	}
	changed := b.Flights[0]
	changed.Scheduled = "00:10"
	g, err := normalizeFlight(changed, nil, nil, nil)
	if err != nil || g.ID != f.ID {
		t.Fatal("scheduled-time changes must preserve stable group identity")
	}
	_ = json.Unmarshal(fixture(t, "domestic-arrivals"), &b)
	a, err := normalizeFlight(b.Flights[0], nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.ArrivalExits) == 0 || len(a.BoardingGates) != 0 {
		t.Fatal("arrival exit gate must not become a boarding gate")
	}
}

func TestCatalogAliasesAndSummary(t *testing.T) {
	server := sourceServer(t, nil)
	defer server.Close()
	c, _ := NewClient(server.URL, time.Second, 0)
	for _, kind := range []string{"domestic", "international"} {
		airports, err := c.FetchAirports(context.Background(), kind)
		if err != nil || len(airports) == 0 {
			t.Fatal(err)
		}
		airlines, err := c.FetchAirlines(context.Background(), kind)
		if err != nil || len(airlines) == 0 || airlines[0].NameJA == "" {
			t.Fatal("bilingual airline catalog missing")
		}
		if kind == "domestic" {
			a := findAirport("SPK", airports)
			if a == nil || a.Code != "CTS" || !strings.Contains(a.NameJA, "札幌") {
				t.Fatal("airportCode versus city search value collapsed")
			}
		}
	}
	s, err := c.FetchSummary(context.Background())
	if err != nil || s["gettingAt"] == nil {
		t.Fatal("source disruption timestamp missing")
	}
}
func TestFilterBoardBoundsNegativeAndUnknown(t *testing.T) {
	server := sourceServer(t, nil)
	defer server.Close()
	c, _ := NewClient(server.URL, time.Second, 0)
	q := testQuery()
	r, err := c.FetchBoard(context.Background(), q, testNow(), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, flight, airline, destination, status string
		want                                       int
	}{{"source primary", "NH849", "", "", "", 1}, {"marketing alias", "UA8003", "", "", "", 1}, {"Japanese airport", "", "", "バンコク", "", 1}, {"mismatching flight", "NH99999", "", "", "", 0}, {"mismatching airline", "", "imaginary-airline-xyz", "", "", 0}, {"blank category with known text", "", "", "", "unknown", 0}, {"final boarding text", "", "", "", "Final boarding call", 1}, {"source canceled", "", "", "", "cancelled", 1}} {
		t.Run(tc.name, func(t *testing.T) {
			v := q
			v.Flight, v.Airline, v.Destination, v.Status = tc.flight, tc.airline, tc.destination, tc.status
			got := FilterBoard(r, v, testNow())
			if got.TotalMatches != tc.want || len(got.Flights) != tc.want {
				t.Fatalf("wanted %d matching groups, got %d", tc.want, got.TotalMatches)
			}
		})
	}
	v := q
	v.Status = "ALL"
	if got := FilterBoard(r, v, testNow()); got.TotalMatches != len(r.Flights) {
		t.Fatal("case-insensitive ALL must retain the source scope")
	}
	v.Status = "canceled,"
	if got := FilterBoard(r, v, testNow()); got.TotalMatches != 1 {
		t.Fatal("empty status token must never match a blank category")
	}
	v.Status = ""
	v.Limit = 1
	page := FilterBoard(r, v, testNow())
	if len(page.Flights) != 1 || page.NextOffset == nil || *page.NextOffset != 1 {
		t.Fatal("local pagination is not bounded")
	}
	v.Offset = 999
	page = FilterBoard(r, v, testNow().Add(6*time.Minute))
	data, _ := json.Marshal(page)
	if len(page.Flights) != 0 || page.Flights == nil || !page.Stale || !strings.Contains(string(data), `"flights":[]`) {
		t.Fatal("empty/stale observation semantics missing")
	}
	v = q
	v.RolloverOnly = true
	page = FilterBoard(r, v, testNow())
	if page.TotalMatches < 1 || page.Flights[0].TimeChangeMinutes == nil {
		t.Fatal("midnight rollover inspection failed")
	}
	unknown := r
	unknown.Flights = append([]Flight{}, r.Flights...)
	unknown.Flights[0].Status = Status{}
	v = q
	v.Status = "unknown"
	if got := FilterBoard(unknown, v, testNow()); got.TotalMatches != 1 {
		t.Fatal("an actually blank status must remain unknown")
	}
}

func TestBodyContextAndRequestBudgets(t *testing.T) {
	server := sourceServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		fmt.Fprint(w, `{"padding":"`+strings.Repeat("x", int(maxBody))+`"}`)
		return true
	})
	defer server.Close()
	c, _ := NewClient(server.URL, time.Second, 0)
	if _, err := c.FetchAirports(context.Background(), "domestic"); err == nil || !strings.Contains(err.Error(), "budget") {
		t.Fatal("oversized provider body must fail explicitly")
	}
	c, _ = NewClient(server.URL, time.Second, 0)
	c.budget.Requests = maxRequests
	if _, err := c.FetchAirports(context.Background(), "domestic"); err == nil || c.Budget().Requests != maxRequests {
		t.Fatal("request budget must stop before another request")
	}
	small := sourceServer(t, nil)
	defer small.Close()
	c, _ = NewClient(small.URL, time.Second, 0)
	c.budget.ResponseBytes = maxTotal - 100
	if _, err := c.FetchAirports(context.Background(), "domestic"); err == nil || c.Budget().ResponseBytes > maxTotal+1 {
		t.Fatal("aggregate body budget was not enforced during reading")
	}
	blocked := sourceServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
		return true
	})
	defer blocked.Close()
	c, _ = NewClient(blocked.URL, time.Second, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := c.FetchAirports(ctx, "domestic"); err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("the command context must bound source requests")
	}
}

func TestSourceFailuresAreNeverEmptySuccess(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{{"rate limit", 429, `{}`}, {"maintenance", 503, `<html>maintenance</html>`}, {"HTML shell", 200, `<html>empty shell</html>`}, {"schema", 200, `{"en":{}}`}} {
		t.Run(tc.name, func(t *testing.T) {
			server := sourceServer(t, func(w http.ResponseWriter, r *http.Request) bool {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
				return true
			})
			defer server.Close()
			c, _ := NewClient(server.URL, time.Second, 0)
			_, err := c.FetchAirports(context.Background(), "domestic")
			if err == nil {
				t.Fatal("failed source must not become an empty successful catalog")
			}
			if tc.status == 429 {
				var rate *cliutil.RateLimitError
				if !errors.As(err, &rate) || c.Budget().Requests != 1 {
					t.Fatal("429 must be typed and not retried")
				}
			}
		})
	}
	server := sourceServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == "POST" {
			fmt.Fprint(w, `{"count":1,"date":{"date":"2026/10/02 23:49:40"},"flightlists":[]}`)
			return true
		}
		return false
	})
	defer server.Close()
	c, _ := NewClient(server.URL, time.Second, 0)
	if _, err := c.FetchBoard(context.Background(), testQuery(), testNow(), ""); err == nil {
		t.Fatal("source count mismatch must fail completeness")
	}
}

func TestPublishedSchedulePeriodWeekdayAndCoverage(t *testing.T) {
	for _, kind := range []string{"domestic", "international"} {
		for _, direction := range []string{"departure", "arrival"} {
			t.Run(kind+direction, func(t *testing.T) {
				server := sourceServer(t, nil)
				defer server.Close()
				c, _ := NewClient(server.URL, time.Second, 0)
				q := testQuery()
				q.Kind, q.Direction, q.Date = kind, direction, ""
				r, err := c.FetchSchedules(context.Background(), q, testNow())
				if err != nil || len(r.Schedules) == 0 {
					t.Fatal(err)
				}
				for _, s := range r.Schedules {
					if s.HanedaTimezone != "Asia/Tokyo" || s.OperatingFlight != nil || s.PeriodStart == "" || len(s.Weekdays) == 0 {
						t.Fatal("schedule period/time semantics incomplete")
					}
					if kind == "international" && s.OtherAirportTimezone != nil {
						t.Fatal("international remote timezone must not be guessed")
					}
				}
			})
		}
	}
	server := sourceServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if strings.HasSuffix(r.URL.Path, "hdacfdsc.json") {
			var f rawScheduleFeed
			_ = json.Unmarshal(fixture(t, "monthly-international-departures"), &f)
			f.Start, f.End = "2026/10/01", "2026/10/31"
			f.Flights = f.Flights[:1]
			f.Flights[0].Start, f.Flights[0].End = "2026/10/01", "2026/10/31"
			f.Flights[0].Days = map[string]string{"毎日": "0", "月曜日": "1"}
			_ = json.NewEncoder(w).Encode(f)
			return true
		}
		return false
	})
	defer server.Close()
	for _, tc := range []struct {
		date string
		want int
		ok   bool
	}{{"2026-10-05", 1, true}, {"2026-10-06", 0, true}, {"2027-01-01", 0, false}} {
		c, _ := NewClient(server.URL, time.Second, 0)
		q := testQuery()
		q.Date = tc.date
		r, err := c.FetchSchedules(context.Background(), q, testNow())
		if (err == nil) != tc.ok || (err == nil && len(r.Schedules) != tc.want) {
			t.Errorf("schedule %s got %d,%v", tc.date, len(r.Schedules), err)
		}
	}
}
