package planner_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tablecheck/internal/planner"
)

const fixtureVenue = "sushi-tokyo81"
const fixtureID = "67e657634474874e35785280"

type recordedRequest struct {
	Method, Path string
	Query        url.Values
	Body         map[string]any
}

type fixtureTransport struct {
	mu       sync.Mutex
	requests []recordedRequest
	headers  http.Header
	respond  func(*http.Request, recordedRequest, int) (int, []byte)
}

func (f *fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
		return nil, errors.New("planning request unexpectedly carries credentials")
	}
	var body map[string]any
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
			return nil, err
		}
	}
	q := r.URL.Query()
	entry := recordedRequest{r.Method, r.URL.Path, q, body}
	f.mu.Lock()
	f.requests = append(f.requests, entry)
	n := len(f.requests)
	f.mu.Unlock()
	status, data := f.respond(r, entry, n)
	headers := f.headers.Clone()
	if headers == nil {
		headers = http.Header{}
	}
	headers.Set("Content-Type", "application/json")
	return &http.Response{StatusCode: status, Header: headers, Body: io.NopCloser(bytes.NewReader(data)), Request: r}, nil
}

func (f *fixtureTransport) entries() []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recordedRequest(nil), f.requests...)
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "planner", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func defaultTransport(t *testing.T, menu, calendar string) *fixtureTransport {
	t.Helper()
	f := &fixtureTransport{}
	f.respond = func(_ *http.Request, r recordedRequest, _ int) (int, []byte) {
		switch {
		case strings.HasPrefix(r.Path, "/v2/shops/"):
			return 200, fixture(t, "venue-v2.json")
		case r.Path == "/v2/hub/menu_items":
			return 200, fixture(t, menu)
		case r.Path == "/v2/hub/availability_calendar_v2":
			return 200, fixture(t, calendar)
		case r.Path == "/v2/shop_search":
			return 200, fixture(t, "search-filtered.json")
		case r.Path == "/v2/cuisines":
			return 200, fixture(t, "cuisines.json")
		default:
			t.Errorf("unexpected endpoint %s", r.Path)
			return 404, []byte(`{"error":"unexpected endpoint"}`)
		}
	}
	return f
}

func newFixtureClient(t *testing.T, f *fixtureTransport, change func(*planner.Options)) *planner.Client {
	t.Helper()
	o := planner.Options{BaseURL: "https://fixture.invalid", CacheDir: t.TempDir(), Timeout: 2 * time.Second, MaxRequests: 20, Concurrency: 2, Retries: 0, Now: func() time.Time { return time.Date(2026, 9, 27, 14, 0, 0, 0, time.UTC) }, HTTPClient: &http.Client{Transport: f}}
	if change != nil {
		change(&o)
	}
	c, err := planner.New(o)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func object(t *testing.T, v any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func array(t *testing.T, v any) []any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var a []any
	if err := json.Unmarshal(b, &a); err != nil {
		t.Fatalf("expected array: %s: %v", b, err)
	}
	return a
}

func firstCheck(t *testing.T, r planner.Result) map[string]any {
	t.Helper()
	checks := array(t, r["checks"])
	if len(checks) != 1 {
		t.Fatalf("want one check row, got %d: %#v", len(checks), r)
	}
	return object(t, checks[0])
}

func jsonText(t *testing.T, v any) string {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func number(t *testing.T, v any) int {
	t.Helper()
	n, ok := v.(float64)
	if !ok {
		t.Fatalf("want JSON number, got %T=%v", v, v)
	}
	return int(n)
}
func metaRequests(t *testing.T, r planner.Result) int {
	t.Helper()
	return number(t, object(t, r["meta"])["requests"])
}

func TestSearchSendsExplicitFiltersAndOpaqueCursorWithoutSpeculativePaging(t *testing.T) {
	f := defaultTransport(t, "menu-items.json", "calendar-party2.json")
	c := newFixtureClient(t, f, nil)
	cursor := "opaque+/cursor== with space"
	r, e := c.Search(context.Background(), planner.SearchOptions{Latitude: 35.681236, Longitude: 139.767125, Radius: 3000, Cuisine: "sushi", BudgetMin: "10000.00", BudgetMax: "20000.10", Date: "2026-09-30", Time: "18:00", Party: 2, Limit: 2, Cursor: cursor})
	if e != nil {
		t.Fatal(e)
	}
	entries := f.entries()
	if len(entries) != 1 {
		t.Fatalf("search made %d calls; want one search only", len(entries))
	}
	q := entries[0].Query
	for k, want := range map[string]string{"cuisines[]": "sushi", "budget_dinner_avg_min": "10000.00", "budget_dinner_avg_max": "20000.10", "date": "2026-09-30", "time": "18:00", "num_people": "2", "availability_mode": "same_meal_time", "availability_format": "datetime", "search_after": cursor, "per_page": "2", "include_ids": "true", "geo_distance": "3000"} {
		if q.Get(k) != want {
			t.Errorf("query %s=%q want %q", k, q.Get(k), want)
		}
	}
	if q.Get("name") != "" || q.Get("search_text") != "" {
		t.Errorf("unverified name search emitted: %v", q)
	}
	if entries[0].Path != "/v2/shop_search" || entries[0].Method != "GET" {
		t.Fatalf("wrong read endpoint: %+v", entries[0])
	}
	items := array(t, r["items"])
	if len(items) != 2 {
		t.Fatalf("want two items: %#v", r)
	}
	for _, x := range items {
		m := object(t, x)
		if m["id"] == nil || m["slug"] == nil {
			t.Errorf("stable identity missing: %v", m)
		}
	}
	p := object(t, r["pagination"])
	if p["next_cursor"] == nil || p["has_more"] != true {
		t.Errorf("cursor pagination lost: %v", p)
	}
	if metaRequests(t, r) != 1 {
		t.Fatalf("request metric lost: %v", r["meta"])
	}
}

func TestVenueUsesVenueGeocodeAndNotCallerLocation(t *testing.T) {
	f := defaultTransport(t, "menu-items.json", "calendar-party2.json")
	r, e := newFixtureClient(t, f, nil).Venue(context.Background(), fixtureVenue)
	if e != nil {
		t.Fatal(e)
	}
	v := object(t, r["venue"])
	if v["id"] != fixtureID || v["country"] != "JP" || v["time_zone"] != "Asia/Tokyo" {
		t.Fatalf("wrong venue identity/location: %v", v)
	}
	if strings.Contains(jsonText(t, v), "1.3153") || strings.Contains(jsonText(t, v), "103.9143") {
		t.Fatalf("caller geolocation leaked into venue: %v", v)
	}
	if !strings.Contains(jsonText(t, v), "35.626086") {
		t.Fatalf("venue geocode absent: %v", v)
	}
}

func TestCoursesPreserveDecimalConditionsAndUnknownBasis(t *testing.T) {
	f := defaultTransport(t, "menu-items.json", "calendar-party2.json")
	c := newFixtureClient(t, f, nil)
	r, e := c.Courses(context.Background(), planner.CourseOptions{Venue: fixtureVenue, Limit: 2})
	if e != nil {
		t.Fatal(e)
	}
	items := array(t, r["items"])
	if len(items) != 2 {
		t.Fatalf("want bounded two-course list: %v", r)
	}
	m := object(t, items[0])
	if m["price"] != "19800.0" || m["currency"] != "JPY" || m["tax_type"] != "included" || m["service_fee_type"] != "percent" {
		t.Fatalf("money metadata changed: %v", m)
	}
	if value, ok := m["price_basis"]; !ok || value != nil {
		t.Fatalf("unknown price basis must be explicit null: %v", m)
	}
	p := object(t, r["pagination"])
	if p["has_more"] != true || number(t, p["next_offset"]) != 2 {
		t.Fatalf("local menu pagination incorrect: %v", p)
	}
	if len(f.entries()) != 2 {
		t.Fatalf("courses must lazily resolve venue then menus, got %d calls", len(f.entries()))
	}
	d, e := c.Courses(context.Background(), planner.CourseOptions{Venue: fixtureVenue, CourseID: "68da546fcde865308c33e7f9"})
	if e != nil {
		t.Fatal(e)
	}
	course := object(t, d["course"])
	text := jsonText(t, course)
	for _, want := range []string{"19,800", "22,000", "10%", "postpay_required", "67eb4c81fc3dd992c5ec88ed", "min_time_cutoff_at"} {
		if !strings.Contains(text, want) {
			t.Errorf("detail lost source condition %q: %s", want, text)
		}
	}
	if _, ok := course["valid_date_ranges"]; !ok {
		t.Error("supported empty date rules omitted")
	}
	if course["price"] != "19800.0" {
		t.Fatalf("detail decimal changed: %v", course["price"])
	}
}

func TestCourseEdgesPreserveLargeDecimalAndMissingVersusEmptyRules(t *testing.T) {
	f := defaultTransport(t, "menu-items-edge.json", "calendar-party2.json")
	c := newFixtureClient(t, f, nil)
	for _, tc := range []struct {
		id, price string
		empty     bool
	}{{"synthetic-explicit-sold-out", "9007199254740993.10", true}, {"synthetic-future-status", "", false}} {
		r, e := c.Courses(context.Background(), planner.CourseOptions{Venue: fixtureVenue, CourseID: tc.id})
		if e != nil {
			t.Fatal(e)
		}
		m := object(t, r["course"])
		if tc.price != "" && m["price"] != tc.price {
			t.Errorf("precise decimal changed: %v", m["price"])
		}
		if tc.id == "synthetic-future-status" {
			if m["availability_status"] != "vendor_future_status" || m["price"] != nil || m["price_basis"] != nil {
				t.Errorf("unknown source evidence changed: %v", m)
			}
		}
		v, ok := m["valid_date_ranges"]
		if !ok {
			t.Error("supported rules field omitted")
		}
		if tc.empty && v == nil {
			t.Error("explicit empty rules collapsed into null")
		}
		if !tc.empty && v != nil {
			t.Errorf("missing rules invented: %#v", v)
		}
	}
}

func TestEmptyCoursesAreSuccessfulAndUnknownCourseIsTypedNotFound(t *testing.T) {
	f := defaultTransport(t, "menu-items-empty.json", "calendar-party2.json")
	c := newFixtureClient(t, f, nil)
	r, e := c.Courses(context.Background(), planner.CourseOptions{Venue: fixtureVenue})
	if e != nil {
		t.Fatal(e)
	}
	if len(array(t, r["items"])) != 0 {
		t.Fatalf("empty courses invented: %v", r)
	}
	_, e = c.Courses(context.Background(), planner.CourseOptions{Venue: fixtureVenue, CourseID: "missing"})
	var nf *planner.NotFoundError
	if !errors.As(e, &nf) {
		t.Fatalf("want typed not-found, got %T %v", e, e)
	}
}

func TestBookingHandoffPreservesCanonicalModeAndSourcePrefill(t *testing.T) {
	for _, tc := range []struct{ fixture, path string }{{"venue-v1.json", "/en/shops/sushi-tokyo81/reserve"}, {"venue-v2.json", "/en/sushi-tokyo81/reserve/landing"}} {
		t.Run(tc.fixture, func(t *testing.T) {
			f := defaultTransport(t, "menu-items.json", "calendar-party2.json")
			f.respond = func(_ *http.Request, _ recordedRequest, _ int) (int, []byte) { return 200, fixture(t, tc.fixture) }
			r, e := newFixtureClient(t, f, nil).BookingURL(context.Background(), planner.HandoffOptions{Venue: fixtureVenue, Date: "2026-09-30", Time: "18:00", Party: 2})
			if e != nil {
				t.Fatal(e)
			}
			var raw string
			for _, k := range []string{"booking_url", "url"} {
				if s, ok := r[k].(string); ok {
					raw = s
					break
				}
			}
			if raw == "" {
				t.Fatalf("handoff URL missing: %v", r)
			}
			u, e := url.Parse(raw)
			if e != nil {
				t.Fatal(e)
			}
			if u.Scheme != "https" || u.Host != "www.tablecheck.com" || u.Path != tc.path {
				t.Fatalf("wrong canonical URL: %s", raw)
			}
			for k, w := range map[string]string{"start_date": "2026-09-30", "start_time": "18:00", "num_people": "2"} {
				if u.Query().Get(k) != w {
					t.Errorf("handoff %s=%q want%q", k, u.Query().Get(k), w)
				}
			}
			if u.Query().Get("pax") != "" {
				t.Errorf("unobserved pax emitted: %s", raw)
			}
			if len(f.entries()) != 1 {
				t.Fatalf("handoff called more than venue read: %+v", f.entries())
			}
			venueResult, venueErr := newFixtureClient(t, f, nil).Venue(context.Background(), fixtureVenue)
			if venueErr != nil {
				t.Fatal(venueErr)
			}
			if object(t, venueResult["venue"])["venue_url"] != "https://www.tablecheck.com/en/sushi-tokyo81" {
				t.Fatalf("venue URL incorrectly follows booking mode: %v", venueResult)
			}
		})
	}
}

func TestInvalidPlannerInputsAreTypedAndPerformNoHTTP(t *testing.T) {
	f := defaultTransport(t, "menu-items.json", "calendar-party2.json")
	c := newFixtureClient(t, f, nil)
	badSearch := []planner.SearchOptions{{Latitude: math.NaN(), Longitude: 139, Radius: 3000}, {Latitude: 35, Longitude: math.Inf(1), Radius: 3000}, {Latitude: 35, Longitude: 139, Radius: 50001}, {Latitude: 35, Longitude: 139, Radius: 3000, BudgetMin: "20000", BudgetMax: "10000"}, {Latitude: 35, Longitude: 139, Radius: 3000, Time: "18:00"}, {Latitude: 35, Longitude: 139, Radius: 3000, Cursor: strings.Repeat("x", 8193)}}
	for _, o := range badSearch {
		_, e := c.Search(context.Background(), o)
		var v *planner.ValidationError
		if !errors.As(e, &v) {
			t.Errorf("want typed search validation error, got %v", e)
		}
	}
	for _, o := range []planner.CheckOptions{{Venue: fixtureVenue, Date: "2026-09-31", Party: 2}, {Venue: fixtureVenue, Date: "2026-09-30", Party: 0}, {Venue: fixtureVenue, Date: "2026-09-30", Party: 21}, {Venue: fixtureVenue, Date: "2026-09-30", Party: 2, Time: "24:00"}} {
		_, e := c.Check(context.Background(), o)
		var v *planner.ValidationError
		if !errors.As(e, &v) {
			t.Errorf("want typed check validation error, got %v", e)
		}
	}
	for _, o := range []planner.ScanOptions{{Venues: []string{fixtureVenue, fixtureVenue}, From: "2026-09-30", To: "2026-10-01", Party: 2}, {Venues: []string{fixtureVenue}, From: "2026-09-30", To: "2026-10-14", Party: 2}, {Venues: []string{"a", "b", "c", "d", "e", "f"}, From: "2026-09-30", To: "2026-10-01", Party: 2}} {
		_, e := c.Scan(context.Background(), o)
		var v *planner.ValidationError
		if !errors.As(e, &v) {
			t.Errorf("want typed scan validation error, got %v", e)
		}
	}
	if len(f.entries()) != 0 {
		t.Fatalf("invalid inputs caused %d HTTP calls", len(f.entries()))
	}
}
