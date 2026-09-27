package planner_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tablecheck/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tablecheck/internal/planner"
)

func TestCheckRequestedFalseIsUnavailableWithVenueAlternatives(t *testing.T) {
	f := defaultTransport(t, "menu-items.json", "calendar-party2.json")
	r, e := newFixtureClient(t, f, nil).Check(context.Background(), planner.CheckOptions{Venue: fixtureVenue, Date: "2026-09-30", Time: "18:00", Party: 2, IncludeUnavailable: true})
	if e != nil {
		t.Fatal(e)
	}
	row := firstCheck(t, r)
	if row["status"] != "unavailable" || row["scope"] != "venue" || row["requested_time"] != "18:00" || row["party"] != float64(2) {
		t.Fatalf("exact requested-time semantics lost: %v", row)
	}
	if !strings.Contains(jsonText(t, row["available_times"]), "17:30") {
		t.Fatalf("alternative venue time missing: %v", row)
	}
	if row["status"] == "sold_out" || row["status"] == "waitlist" {
		t.Fatalf("false flag/policy prose invented inventory status: %v", row)
	}
	entries := f.entries()
	if len(entries) != 2 {
		t.Fatalf("check expected venue plus one calendar, got %d", len(entries))
	}
	b := entries[1].Body
	if b["shop_id"] != fixtureID || b["num_people"] != float64(2) {
		t.Fatalf("explicit venue/party lost: %v", b)
	}
	start, err := time.Parse(time.RFC3339Nano, b["start_at"].(string))
	if err != nil {
		t.Fatal(err)
	}
	tokyo, _ := time.LoadLocation("Asia/Tokyo")
	if start.In(tokyo).Format("2006-01-02 15:04") != "2026-09-30 18:00" {
		t.Fatalf("start_at uses wrong local date/time: %s", start)
	}
}

func TestCalendarFalseMissingNullAndClosedRemainDistinct(t *testing.T) {
	for _, tc := range []struct{ date, clock, want string }{{"2026-09-30", "17:00", "unavailable"}, {"2026-09-30", "17:30", "unknown"}, {"2026-09-30", "18:00", "unknown"}, {"2026-10-01", "", "unknown"}, {"2026-10-02", "", "closed"}, {"2026-10-03", "", "unknown"}} {
		t.Run(tc.date+tc.clock, func(t *testing.T) {
			f := defaultTransport(t, "menu-items.json", "calendar-false-missing-null.json")
			r, e := newFixtureClient(t, f, nil).Check(context.Background(), planner.CheckOptions{Venue: fixtureVenue, Date: tc.date, Time: tc.clock, Party: 2, IncludeUnavailable: true})
			if r == nil {
				t.Fatalf("missing check row on uncertainty: %v", e)
			}
			row := firstCheck(t, r)
			if row["status"] != tc.want {
				t.Fatalf("status=%v want%s; source=%v error=%v", row["status"], tc.want, row, e)
			}
		})
	}
}

func TestCalendarMachineErrorsAndUnknownSchemaNeverLookLikeSoldOut(t *testing.T) {
	for _, name := range []string{"calendar-source-error.json", "calendar-source-unknown.json", "calendar-missing-schema.json", "calendar-invalid-zone.json"} {
		t.Run(name, func(t *testing.T) {
			f := defaultTransport(t, "menu-items.json", name)
			r, e := newFixtureClient(t, f, nil).Check(context.Background(), planner.CheckOptions{Venue: fixtureVenue, Date: "2026-09-30", Party: 2})
			if r == nil {
				t.Fatalf("must retain requested check evidence even on error: %v", e)
			}
			row := firstCheck(t, r)
			if row["status"] != "unknown" && row["status"] != "failed" {
				t.Fatalf("uncertain source manufactured inventory: %v error=%v", row, e)
			}
			if len(array(t, row["available_times"])) != 0 {
				t.Fatalf("uncertain response offered times: %v", row)
			}
			if name == "calendar-source-unknown.json" && !strings.Contains(jsonText(t, row["source_status"]), "vendor_future_status") {
				t.Fatalf("unknown source status discarded: %v", row)
			}
		})
	}
}

func TestCalendarUTCKeysAreConvertedAndFilteredByLocalDate(t *testing.T) {
	f := defaultTransport(t, "menu-items.json", "calendar-utc-boundaries.json")
	r, e := newFixtureClient(t, f, nil).Check(context.Background(), planner.CheckOptions{Venue: fixtureVenue, Date: "2026-09-30", Party: 2})
	if e != nil {
		t.Fatal(e)
	}
	row := firstCheck(t, r)
	times := array(t, row["available_times"])
	if len(times) != 1 || times[0] != "00:30" {
		t.Fatalf("UTC boundary or contradictory outer date leaked times: %v", row)
	}
	for _, s := range array(t, row["slots"]) {
		m := object(t, s)
		if strings.HasPrefix(m["starts_at"].(string), "2026-09-30T15:30") {
			t.Fatalf("next local-day slot leaked into Sep30: %v", m)
		}
	}
}

func TestScanReturnsEveryDateAndMakesOneCalendarPerCoveredVenue(t *testing.T) {
	f := defaultTransport(t, "menu-items.json", "calendar-party2.json")
	r, e := newFixtureClient(t, f, nil).Scan(context.Background(), planner.ScanOptions{Venues: []string{fixtureVenue}, From: "2026-09-28", To: "2026-10-03", Party: 2})
	if e != nil {
		t.Fatal(e)
	}
	checks := array(t, r["checks"])
	if len(checks) != 6 {
		t.Fatalf("scan omitted closed or unavailable days: %v", r)
	}
	dates := map[string]bool{}
	for _, x := range checks {
		m := object(t, x)
		dates[m["date"].(string)] = true
		if m["date"] == "2026-09-28" && m["status"] != "closed" {
			t.Errorf("closed date lost: %v", m)
		}
	}
	if len(dates) != 6 || len(f.entries()) != 2 || metaRequests(t, r) != 2 {
		t.Fatalf("covered scan fetched per day or dropped evidence: dates=%v calls=%d meta=%v", dates, len(f.entries()), r["meta"])
	}
}

func TestPartialScanRetainsFailedVenueRowsAndSuccessfulRows(t *testing.T) {
	f := defaultTransport(t, "menu-items.json", "calendar-party2.json")
	base := f.respond
	f.respond = func(r *http.Request, q recordedRequest, n int) (int, []byte) {
		if q.Path == "/v2/shops/broken-venue" {
			return 503, []byte(`{"error":"synthetic upstream unavailable"}`)
		}
		return base(r, q, n)
	}
	r, e := newFixtureClient(t, f, nil).Scan(context.Background(), planner.ScanOptions{Venues: []string{fixtureVenue, "broken-venue"}, From: "2026-09-30", To: "2026-10-01", Party: 2})
	var partial *planner.PartialError
	if !errors.As(e, &partial) {
		t.Fatalf("want typed partial failure with preserved results, got %T %v", e, e)
	}
	checks := array(t, r["checks"])
	if len(checks) != 4 {
		t.Fatalf("failed scan dates omitted: %v", r)
	}
	good, bad := 0, 0
	for _, x := range checks {
		m := object(t, x)
		switch m["slug"] {
		case fixtureVenue:
			good++
			if m["status"] != "available" {
				t.Errorf("successful venue lost: %v", m)
			}
		case "broken-venue":
			bad++
			if m["status"] != "failed" {
				t.Errorf("HTTP failure presented as inventory: %v", m)
			}
		default:
			t.Errorf("unexpected scan row: %v", m)
		}
	}
	if good != 2 || bad != 2 {
		t.Fatalf("partial rows wrong: good%d bad%d", good, bad)
	}
}

func TestCacheHitTTLRefreshAndPartyIsolation(t *testing.T) {
	f := defaultTransport(t, "menu-items.json", "calendar-party2.json")
	cache := t.TempDir()
	clock := time.Date(2026, 9, 27, 14, 0, 0, 0, time.UTC)
	client := func(refresh bool) *planner.Client {
		return newFixtureClient(t, f, func(o *planner.Options) {
			o.CacheDir = cache
			o.Now = func() time.Time { return clock }
			o.Refresh = refresh
		})
	}
	opt := planner.CheckOptions{Venue: fixtureVenue, Date: "2026-09-30", Party: 2}
	first, e := client(false).Check(context.Background(), opt)
	if e != nil {
		t.Fatal(e)
	}
	if metaRequests(t, first) != 2 {
		t.Fatalf("first check requests=%v", first["meta"])
	}
	clock = clock.Add(10 * time.Second)
	hit, e := client(false).Check(context.Background(), opt)
	if e != nil {
		t.Fatal(e)
	}
	if metaRequests(t, hit) != 0 || len(f.entries()) != 2 {
		t.Fatalf("cache hit made HTTP: %v calls%d", hit, len(f.entries()))
	}
	fresh := object(t, firstCheck(t, hit)["freshness"])
	if fresh["cache_hit"] != true {
		t.Fatalf("served cache not identified: %v", fresh)
	}
	firstFresh := object(t, firstCheck(t, first)["freshness"])
	if fresh["fetched_at"] != firstFresh["fetched_at"] {
		t.Fatalf("cached original observation time overwritten: before%v after%v", firstFresh, fresh)
	}
	clock = clock.Add(21 * time.Second)
	expired, e := client(false).Check(context.Background(), opt)
	if e != nil {
		t.Fatal(e)
	}
	if metaRequests(t, expired) != 1 {
		t.Fatalf("calendar TTL should refetch only calendar after31s: %v", expired["meta"])
	}
	refreshed, e := client(true).Check(context.Background(), opt)
	if e != nil {
		t.Fatal(e)
	}
	if metaRequests(t, refreshed) != 2 {
		t.Fatalf("refresh must bypass both local reads: %v", refreshed["meta"])
	}
	other := opt
	other.Party = 20
	_, e = client(false).Check(context.Background(), other)
	if e != nil {
		t.Fatal(e)
	}
	entries := f.entries()
	last := entries[len(entries)-1]
	if last.Body["num_people"] != float64(20) {
		t.Fatalf("party reused wrong cached calendar: %v", last)
	}
}

func TestFailedCalendarIsNotCachedAsInventory(t *testing.T) {
	f := defaultTransport(t, "menu-items.json", "calendar-source-error.json")
	cache := t.TempDir()
	newC := func() *planner.Client { return newFixtureClient(t, f, func(o *planner.Options) { o.CacheDir = cache }) }
	opt := planner.CheckOptions{Venue: fixtureVenue, Date: "2026-09-30", Party: 2}
	_, _ = newC().Check(context.Background(), opt)
	base := f.respond
	f.respond = func(r *http.Request, q recordedRequest, n int) (int, []byte) {
		if q.Path == "/v2/hub/availability_calendar_v2" {
			return 200, fixture(t, "calendar-party2.json")
		}
		return base(r, q, n)
	}
	r, e := newC().Check(context.Background(), opt)
	if e != nil {
		t.Fatal(e)
	}
	if firstCheck(t, r)["status"] != "available" || metaRequests(t, r) != 1 {
		t.Fatalf("failed calendar cached or venue cache lost: %v", r)
	}
}

func TestTransientRetryAndGlobalRequestBudgetCountActualAttempts(t *testing.T) {
	f := defaultTransport(t, "menu-items.json", "calendar-party2.json")
	base := f.respond
	f.respond = func(r *http.Request, q recordedRequest, n int) (int, []byte) {
		if n == 1 {
			return 503, []byte(`{"error":"retryable"}`)
		}
		return base(r, q, n)
	}
	c := newFixtureClient(t, f, func(o *planner.Options) { o.Retries = 1 })
	r, e := c.Search(context.Background(), planner.SearchOptions{Latitude: 35, Longitude: 139, Radius: 3000})
	if e != nil {
		t.Fatal(e)
	}
	if len(f.entries()) != 2 || metaRequests(t, r) != 2 {
		t.Fatalf("retry attempt metrics wrong: %d %v", len(f.entries()), r["meta"])
	}
	g := defaultTransport(t, "menu-items.json", "calendar-party2.json")
	g.respond = func(_ *http.Request, _ recordedRequest, _ int) (int, []byte) {
		return 503, []byte(`{"error":"always unavailable"}`)
	}
	d := newFixtureClient(t, g, func(o *planner.Options) { o.Retries = 1; o.MaxRequests = 1 })
	_, e = d.Search(context.Background(), planner.SearchOptions{Latitude: 35, Longitude: 139, Radius: 3000})
	if e == nil || len(g.entries()) != 1 {
		t.Fatalf("global request cap permits extra retry: error%v calls%d", e, len(g.entries()))
	}
}

func TestScanConcurrencyNeverExceedsTwoAndCancellationStopsWork(t *testing.T) {
	f := defaultTransport(t, "menu-items.json", "calendar-party2.json")
	base := f.respond
	var active, peak int32
	f.respond = func(r *http.Request, q recordedRequest, n int) (int, []byte) {
		a := atomic.AddInt32(&active, 1)
		for {
			p := atomic.LoadInt32(&peak)
			if a <= p || atomic.CompareAndSwapInt32(&peak, p, a) {
				break
			}
		}
		defer atomic.AddInt32(&active, -1)
		select {
		case <-r.Context().Done():
			return 499, []byte(`{"error":"cancelled"}`)
		case <-time.After(20 * time.Millisecond):
		}
		if strings.HasPrefix(q.Path, "/v2/shops/") {
			var v map[string]any
			_ = json.Unmarshal(fixture(t, "venue-v2.json"), &v)
			shop := v["shops"].([]any)[0].(map[string]any)
			slug := strings.TrimPrefix(q.Path, "/v2/shops/")
			shop["slug"] = slug
			shop["_id"] = "id-" + slug
			b, _ := json.Marshal(v)
			return 200, b
		}
		return base(r, q, n)
	}
	r, e := newFixtureClient(t, f, nil).Scan(context.Background(), planner.ScanOptions{Venues: []string{"one", "two", "three", "four", "five"}, From: "2026-09-30", To: "2026-10-01", Party: 2})
	if e != nil {
		t.Fatal(e)
	}
	if len(array(t, r["checks"])) != 10 {
		t.Fatalf("scan row count wrong: %v", r)
	}
	if p := atomic.LoadInt32(&peak); p < 1 || p > 2 {
		t.Fatalf("configured maximum concurrency2 actualpeak%d", p)
	}
	g := defaultTransport(t, "menu-items.json", "calendar-party2.json")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = newFixtureClient(t, g, nil).Scan(ctx, planner.ScanOptions{Venues: []string{"one", "two"}, From: "2026-09-30", To: "2026-10-01", Party: 2})
	if e == nil || len(g.entries()) != 0 {
		t.Fatalf("pre-cancelled command still performed HTTP: error%v calls%d", e, len(g.entries()))
	}
}

func TestDryRunPlanValidatesWithoutHTTPOrCacheWrites(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "must-not-exist")
	// The public planning plan deliberately does not construct a Client.
	r, e := planner.DryRunPlan("availability check", planner.CheckOptions{Venue: fixtureVenue, Date: "2026-09-30", Party: 2})
	if e != nil {
		t.Fatal(e)
	}
	if r["dry_run"] != true || r["request_plan"] == nil || metaRequests(t, r) != 0 {
		t.Fatalf("dry run manufactured API result: %v", r)
	}
	if _, e = os.Stat(cache); !errors.Is(e, os.ErrNotExist) {
		t.Fatalf("dry run touched cache: %v", e)
	}
	_, e = planner.DryRunPlan("availability check", planner.CheckOptions{Venue: fixtureVenue, Date: "2026-09-30", Party: 21})
	var invalid *planner.ValidationError
	if !errors.As(e, &invalid) {
		t.Fatalf("dry run skipped validation: %v", e)
	}
}

func TestRetryAfterLongDelayStopsWithoutEarlyRetry(t *testing.T) {
	f := defaultTransport(t, "menu-items.json", "calendar-party2.json")
	f.headers = http.Header{"Retry-After": []string{"10"}}
	f.respond = func(_ *http.Request, _ recordedRequest, _ int) (int, []byte) {
		return 429, []byte(`{"error":"slow down"}`)
	}
	started := time.Now()
	_, e := newFixtureClient(t, f, func(o *planner.Options) { o.Retries = 1 }).Search(context.Background(), planner.SearchOptions{Latitude: 35, Longitude: 139, Radius: 3000})
	var throttle *cliutil.RateLimitError
	if !errors.As(e, &throttle) {
		t.Fatalf("long Retry-After must return typed rate limit: %T %v", e, e)
	}
	if len(f.entries()) != 1 {
		t.Fatalf("Retry-After was shortened into an early retry: %d calls", len(f.entries()))
	}
	if time.Since(started) >= 2*time.Second {
		t.Fatalf("bounded command waited for unbounded Retry-After: %s", time.Since(started))
	}
}

func TestOversizedResponseIsRejectedAndNotCached(t *testing.T) {
	f := defaultTransport(t, "menu-items.json", "calendar-party2.json")
	f.respond = func(_ *http.Request, _ recordedRequest, _ int) (int, []byte) {
		return 200, []byte(strings.Repeat(" ", 5<<20))
	}
	cache := t.TempDir()
	_, e := newFixtureClient(t, f, func(o *planner.Options) { o.CacheDir = cache }).Search(context.Background(), planner.SearchOptions{Latitude: 35, Longitude: 139, Radius: 3000})
	if e == nil || len(f.entries()) != 1 {
		t.Fatalf("oversized response accepted or retried: error=%v calls=%d", e, len(f.entries()))
	}
	files, e := os.ReadDir(cache)
	if e != nil {
		t.Fatal(e)
	}
	if len(files) != 0 {
		t.Fatalf("oversized failure persisted as cache: %v", files)
	}
}

func TestAvailabilityReturnsVenueSummariesWithoutAdditionalHTTP(t *testing.T) {
	assertVenue := func(t *testing.T, v any) {
		t.Helper()
		m := object(t, v)
		if m["id"] != fixtureID || m["slug"] != fixtureVenue || m["name_ja"] != "SUSHI TOKYO 81" {
			t.Fatalf("venue identity/Japanese source label absent: %v", m)
		}
		if m["venue_url"] != "https://www.tablecheck.com/en/sushi-tokyo81" || m["booking_url"] != "https://www.tablecheck.com/en/sushi-tokyo81/reserve/landing" {
			t.Fatalf("canonical venue/booking URLs absent: %v", m)
		}
	}
	t.Run("check", func(t *testing.T) {
		f := defaultTransport(t, "menu-items.json", "calendar-party2.json")
		r, e := newFixtureClient(t, f, nil).Check(context.Background(), planner.CheckOptions{Venue: fixtureVenue, Date: "2026-09-30", Party: 2})
		if e != nil {
			t.Fatal(e)
		}
		assertVenue(t, r["venue"])
		if len(f.entries()) != 2 || metaRequests(t, r) != 2 {
			t.Fatalf("venue summary caused additional HTTP: calls%d meta%v", len(f.entries()), r["meta"])
		}
	})
	t.Run("scan", func(t *testing.T) {
		f := defaultTransport(t, "menu-items.json", "calendar-party2.json")
		r, e := newFixtureClient(t, f, nil).Scan(context.Background(), planner.ScanOptions{Venues: []string{fixtureVenue}, From: "2026-09-30", To: "2026-10-01", Party: 2})
		if e != nil {
			t.Fatal(e)
		}
		venues := array(t, r["venues"])
		if len(venues) != 1 || len(array(t, r["checks"])) != 2 {
			t.Fatalf("venue summary repeated per date or check rows lost: %v", r)
		}
		assertVenue(t, venues[0])
		if len(f.entries()) != 2 || metaRequests(t, r) != 2 {
			t.Fatalf("scan summaries caused additional HTTP: calls%d meta%v", len(f.entries()), r["meta"])
		}
	})
	t.Run("partial scan", func(t *testing.T) {
		f := defaultTransport(t, "menu-items.json", "calendar-party2.json")
		base := f.respond
		f.respond = func(r *http.Request, q recordedRequest, n int) (int, []byte) {
			if q.Path == "/v2/shops/broken-venue" {
				return 503, []byte(`{"error":"synthetic failure"}`)
			}
			return base(r, q, n)
		}
		r, e := newFixtureClient(t, f, nil).Scan(context.Background(), planner.ScanOptions{Venues: []string{fixtureVenue, "broken-venue"}, From: "2026-09-30", To: "2026-10-01", Party: 2})
		var partial *planner.PartialError
		if !errors.As(e, &partial) {
			t.Fatalf("expected preserved partial failure: %v", e)
		}
		venues := array(t, r["venues"])
		if len(venues) != 2 || len(array(t, r["checks"])) != 4 {
			t.Fatalf("partial summaries/checks omitted: %v", r)
		}
		for _, v := range venues {
			m := object(t, v)
			if m["slug"] == fixtureVenue {
				assertVenue(t, m)
			} else if m["slug"] == "broken-venue" {
				for _, k := range []string{"id", "name_ja", "venue_url", "booking_url"} {
					value, ok := m[k]
					if !ok || value != nil {
						t.Errorf("failed metadata must retain explicit null %s: %v", k, m)
					}
				}
			} else {
				t.Errorf("unexpected summary: %v", m)
			}
		}
		if len(f.entries()) != 3 || metaRequests(t, r) != 3 {
			t.Fatalf("partial summaries caused additional HTTP: calls%d meta%v", len(f.entries()), r["meta"])
		}
	})
}

func TestDefaultCalendarAnchorUsesDinnerWindowWithoutClaimingExactTimeOrFullDay(t *testing.T) {
	f := defaultTransport(t, "menu-items.json", "calendar-party2.json")
	cache := t.TempDir()
	client := func() *planner.Client { return newFixtureClient(t, f, func(o *planner.Options) { o.CacheDir = cache }) }
	assertWindow := func(t *testing.T, row map[string]any) {
		t.Helper()
		coverage := object(t, row["coverage"])
		if coverage["time_scope"] != "source_time_window" || coverage["full_day"] != false {
			t.Fatalf("limited source window presented as full-day inventory: %v", coverage)
		}
		if coverage["returned_time_from"] != "16:00" || coverage["returned_time_to"] != "20:00" {
			t.Fatalf("bounds must use all converted source slots, not only offered times: %v", coverage)
		}
	}
	opt := planner.CheckOptions{Venue: fixtureVenue, Date: "2026-09-30", Party: 2}
	first, e := client().Check(context.Background(), opt)
	if e != nil {
		t.Fatal(e)
	}
	row := firstCheck(t, first)
	if row["query_anchor_time"] != "18:00" || row["requested_time"] != nil || row["requested_time_available"] != nil || row["status"] != "available" {
		t.Fatalf("default anchor must retain any-offered-slot semantics: %v", row)
	}
	if !strings.Contains(jsonText(t, row["available_times"]), "17:30") {
		t.Fatalf("default source window missed dinner offer: %v", row)
	}
	assertWindow(t, row)
	entries := f.entries()
	if len(entries) != 2 {
		t.Fatalf("unexpected default read count: %d", len(entries))
	}
	start, e := time.Parse(time.RFC3339Nano, entries[1].Body["start_at"].(string))
	if e != nil {
		t.Fatal(e)
	}
	tokyo, _ := time.LoadLocation("Asia/Tokyo")
	if start.In(tokyo).Format("2006-01-02 15:04") != "2026-09-30 18:00" {
		t.Fatalf("omitted time used midnight rather than dinner anchor: %s", start)
	}
	opt.Time = "18:00"
	exact, e := client().Check(context.Background(), opt)
	if e != nil {
		t.Fatal(e)
	}
	exactRow := firstCheck(t, exact)
	if exactRow["status"] != "unavailable" || exactRow["requested_time"] != "18:00" || exactRow["requested_time_available"] != false {
		t.Fatalf("default cached body erased exact-time semantics: %v", exactRow)
	}
	if metaRequests(t, exact) != 0 || len(f.entries()) != 2 {
		t.Fatalf("default and explicit18 should share the source request cache: %v calls%d", exact["meta"], len(f.entries()))
	}
	assertWindow(t, exactRow)
	opt.Time = "17:30"
	explicit, e := client().Check(context.Background(), opt)
	if e != nil {
		t.Fatal(e)
	}
	explicitRow := firstCheck(t, explicit)
	if explicitRow["query_anchor_time"] != "17:30" || explicitRow["requested_time"] != "17:30" || explicitRow["requested_time_available"] != true {
		t.Fatalf("explicit time not preserved as source anchor and exact check: %v", explicitRow)
	}
	entries = f.entries()
	start, e = time.Parse(time.RFC3339Nano, entries[len(entries)-1].Body["start_at"].(string))
	if e != nil {
		t.Fatal(e)
	}
	if start.In(tokyo).Format("2006-01-02 15:04") != "2026-09-30 17:30" || metaRequests(t, explicit) != 1 {
		t.Fatalf("explicit anchor reused wrong body: start%s meta%v", start, explicit["meta"])
	}
}

func TestScanDefaultAnchorReportsSourceWindowForEveryLocalDate(t *testing.T) {
	f := defaultTransport(t, "menu-items.json", "calendar-party2.json")
	r, e := newFixtureClient(t, f, nil).Scan(context.Background(), planner.ScanOptions{Venues: []string{fixtureVenue}, From: "2026-09-30", To: "2026-10-01", Party: 2})
	if e != nil {
		t.Fatal(e)
	}
	checks := array(t, r["checks"])
	if len(checks) != 2 {
		t.Fatalf("scan lost requested local dates: %v", r)
	}
	for _, v := range checks {
		row := object(t, v)
		coverage := object(t, row["coverage"])
		if row["query_anchor_time"] != "18:00" || row["requested_time"] != nil || coverage["time_scope"] != "source_time_window" || coverage["full_day"] != false || coverage["returned_time_from"] != "16:00" || coverage["returned_time_to"] != "20:00" {
			t.Fatalf("scan misrepresented window/anchor: %v", row)
		}
	}
	entries := f.entries()
	if len(entries) != 2 || metaRequests(t, r) != 2 {
		t.Fatalf("window disclosure caused additional requests: %v calls%d", r["meta"], len(entries))
	}
	start, e := time.Parse(time.RFC3339Nano, entries[1].Body["start_at"].(string))
	if e != nil {
		t.Fatal(e)
	}
	tokyo, _ := time.LoadLocation("Asia/Tokyo")
	if start.In(tokyo).Format("2006-01-02 15:04") != "2026-09-30 18:00" {
		t.Fatalf("scan omitted time uses wrong source anchor: %s", start)
	}
}
