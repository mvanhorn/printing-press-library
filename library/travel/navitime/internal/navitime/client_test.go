package navitime

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/mvanhorn/printing-press-library/library/travel/navitime/internal/cliutil"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func mockClient(t *testing.T, opts Options, body []byte) *Client {
	t.Helper()
	c, e := NewClient(opts)
	if e != nil {
		t.Fatal(e)
	}
	c.limiter = nil
	c.now = func() time.Time { return time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC) }
	c.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Error("browser/API credentials were sent")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Set-Cookie": {"session=never-persist"}}, Body: io.NopCloser(strings.NewReader(string(body))), Request: r}, nil
	})}
	return c
}
func TestNormalizedCacheRefreshNoCacheAndSnapshots(t *testing.T) {
	dir := t.TempDir()
	c := mockClient(t, Options{CacheDir: dir}, fixture(t, "depart.html"))
	ctx := context.Background()
	result, e := c.Routes(ctx, testQuery())
	if e != nil {
		t.Fatal(e)
	}
	if c.Metrics().Requests != 1 || result.Meta.CacheHit {
		t.Fatal("first fetch/cache metrics")
	}
	cached, e := c.Routes(ctx, testQuery())
	if e != nil || !cached.Meta.CacheHit || c.Metrics().Requests != 1 {
		t.Fatal("cache did not remove request")
	}
	alias := Query{From: "00006668", To: "00001756", DepartAt: "2026-09-28T00:00:00Z"}
	aliased, e := c.Routes(ctx, alias)
	if e != nil || aliased.Query != alias || !aliased.Meta.CacheHit || c.Metrics().Requests != 1 {
		t.Fatal("equivalent cached query must retain the current caller's input")
	}
	detail, e := c.Show(ctx, result.Routes[0].ID)
	if e != nil || detail.Route == nil || len(detail.Route.Legs) == 0 || c.Metrics().Requests != 1 || !detail.StoredSnapshot {
		t.Fatal("detail re-fetched or lost source detail")
	}
	latest, e := c.Show(ctx, "latest")
	if e != nil || latest.Route.ID != result.Routes[0].ID {
		t.Fatal("latest snapshot mismatched")
	}
	catalog, e := c.Passes(ctx)
	if e != nil || len(catalog.Passes) != 53 || c.Metrics().Requests != 1 || catalog.Meta.DataKind != "pass_catalogue" {
		t.Fatal("catalog should reuse successful route response")
	}
	refresh := mockClient(t, Options{CacheDir: dir, Refresh: true}, fixture(t, "depart.html"))
	refreshed, e := refresh.Routes(ctx, testQuery())
	if e != nil || refreshed.Meta.CacheHit || refresh.Metrics().Requests != 1 || refresh.Metrics().CacheWrites == 0 {
		t.Fatal("refresh must bypass reads and write normalized response")
	}
	noCache := mockClient(t, Options{CacheDir: t.TempDir(), NoCache: true}, fixture(t, "depart.html"))
	for i := 0; i < 2; i++ {
		if _, e = noCache.Routes(ctx, testQuery()); e != nil {
			t.Fatal(e)
		}
	}
	if noCache.Metrics().Requests != 2 || noCache.Metrics().CacheHits != 0 || noCache.Metrics().CacheWrites != 0 {
		t.Fatal("no-cache must skip all reads/writes")
	}
	empty, e := noCache.Show(ctx, "latest")
	if e != nil || empty.Route != nil {
		t.Fatal("no-cache snapshot fabricated")
	}
	var notFound *NotFoundError
	if _, e = c.Show(ctx, "local_000000000000000000000000"); !errors.As(e, &notFound) {
		t.Fatal("missing snapshot needs typed not-found")
	}
	c.now = func() time.Time { return time.Date(2026, 9, 27, 12, 6, 0, 0, time.UTC) }
	if _, e = c.Routes(ctx, testQuery()); e != nil || c.Metrics().Requests != 2 {
		t.Fatal("route TTL did not expire")
	}
	entries, e := os.ReadDir(dir)
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		raw, _ := os.ReadFile(dir + "/" + entry.Name())
		if strings.Contains(string(raw), "never-persist") || strings.Contains(string(raw), "api_key") || strings.Contains(string(raw), "<script") {
			t.Fatal("cache persisted cookies/HTML/configuration instead of normalized public records")
		}
	}
	assertJSON(t, "cache_snapshot_metrics", c.Metrics())
}
func TestCachedRouteReordersLatestAndRestoresMissingDetail(t *testing.T) {
	dir := t.TempDir()
	firstClient := mockClient(t, Options{CacheDir: dir}, fixture(t, "depart.html"))
	secondClient := mockClient(t, Options{CacheDir: dir}, fixture(t, "arrive.html"))
	cachedClient := mockClient(t, Options{CacheDir: dir}, nil)
	clock := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	for _, client := range []*Client{firstClient, secondClient, cachedClient} {
		client.now = func() time.Time { return clock }
	}
	firstQuery := testQuery()
	secondQuery := Query{From: firstQuery.From, To: firstQuery.To, ArriveBy: "2026-09-28T12:00"}
	first, err := firstClient.Routes(context.Background(), firstQuery)
	if err != nil || len(first.Routes) == 0 {
		t.Fatalf("first route: %v", err)
	}
	second, err := secondClient.Routes(context.Background(), secondQuery)
	if err != nil || len(second.Routes) == 0 || second.Routes[0].ID == first.Routes[0].ID {
		t.Fatalf("second route did not create a distinct snapshot: %v", err)
	}
	params, _, err := queryParams(firstQuery)
	if err != nil {
		t.Fatal(err)
	}
	firstURL := strings.TrimRight(firstClient.BaseURL, "/") + "/en/area/jp/route/result/?" + params.Encode()
	responseCache := firstClient.cachePath("routes", firstURL)
	before, err := os.ReadFile(responseCache)
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range first.Routes {
		if err := os.Remove(firstClient.cachePath("snapshot", route.ID)); err != nil {
			t.Fatal(err)
		}
	}
	clock = clock.Add(2 * time.Minute)
	cached, err := cachedClient.Routes(context.Background(), firstQuery)
	if err != nil || !cached.Meta.CacheHit || firstClient.Metrics().Requests != 1 || secondClient.Metrics().Requests != 1 || cachedClient.Metrics().Requests != 0 {
		t.Fatalf("third route should reuse first response without HTTP: %v metrics=%+v", err, cachedClient.Metrics())
	}
	if !cached.Meta.FetchedAt.Equal(first.Meta.FetchedAt) || cached.Meta.AgeSeconds != 120 {
		t.Fatalf("cached source freshness was renewed: %+v", cached.Meta)
	}
	after, err := os.ReadFile(responseCache)
	if err != nil || string(after) != string(before) {
		t.Fatalf("cached query rewrote response TTL: %v", err)
	}
	for _, route := range first.Routes {
		detail, err := cachedClient.Show(context.Background(), route.ID)
		if err != nil || detail.Route == nil || detail.Route.ID != route.ID || !detail.Meta.FetchedAt.Equal(first.Meta.FetchedAt) {
			t.Fatalf("cached route has an unusable or renewed detail ID %s: %v", route.ID, err)
		}
	}
	latest, err := cachedClient.Show(context.Background(), "latest")
	if err != nil || latest.Route == nil || latest.Route.ID != first.Routes[0].ID {
		t.Fatalf("latest did not return restored first snapshot: %v", err)
	}
	if !latest.Meta.FetchedAt.Equal(first.Meta.FetchedAt) || cachedClient.Metrics().Requests != 0 {
		t.Fatalf("stored detail renewed source timestamp or issued HTTP: %+v", latest.Meta)
	}
}
func TestLookupBoundedDisplayRetainsAmbiguityAndCatalogRefresh(t *testing.T) {
	c := mockClient(t, Options{CacheDir: t.TempDir()}, fixture(t, "places-ja.json"))
	result, e := c.Places(context.Background(), "大久保", "station", 1)
	if e != nil {
		t.Fatal(e)
	}
	if len(result.Places) != 1 || !result.Ambiguous || result.SourceCandidateCount != 7 || result.SourceLimit < 2 || result.Meta.DataKind != "location_candidates" {
		t.Fatalf("bounded ambiguity: %+v", result)
	}
	if _, e = c.Places(context.Background(), "大久保", "station", 1); e != nil || c.Metrics().Requests != 1 {
		t.Fatal("lookup cache hit missing")
	}
	for _, tc := range []struct {
		q, kind string
		limit   int
	}{{"", "station", 5}, {"x", "bad", 5}, {"x", "all", 0}, {"x", "all", 11}} {
		if _, e = c.Places(context.Background(), tc.q, tc.kind, tc.limit); e == nil {
			t.Fatal("invalid lookup input accepted")
		}
	}
	catalogClient := mockClient(t, Options{CacheDir: t.TempDir()}, fixture(t, "depart.html"))
	catalog, e := catalogClient.Passes(context.Background())
	if e != nil || len(catalog.Passes) != 53 || catalogClient.Metrics().Requests != 1 {
		t.Fatal("reference catalog route failed")
	}
	verified := 0
	for _, p := range catalog.Passes {
		if p.LiveTested {
			verified++
			if p.ID != "japan_rail_pass" {
				t.Fatal("untested pass marked verified")
			}
		}
	}
	if verified != 1 {
		t.Fatal("representative verification missing")
	}
	catalogClient.opts.Refresh = true
	if _, e = catalogClient.Passes(context.Background()); e != nil || catalogClient.Metrics().Requests != 2 {
		t.Fatal("catalog refresh must make one reference-route GET")
	}
}
func TestAllLookupRequestsEnoughSourceNodesForBoundedDisplay(t *testing.T) {
	var candidates []json.RawMessage
	if err := json.Unmarshal(fixture(t, "places-ja.json"), &candidates); err != nil || len(candidates) < 7 {
		t.Fatalf("expected at least seven bilingual source candidates: %v", err)
	}
	var english []json.RawMessage
	if err := json.Unmarshal(fixture(t, "places-en.json"), &english); err != nil {
		t.Fatal(err)
	}
	spots := make([]json.RawMessage, 0, len(english))
	for _, raw := range english {
		var item struct {
			MainType string `json:"mainType"`
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			t.Fatal(err)
		}
		if item.MainType == "spot" {
			spots = append(spots, raw)
		}
	}
	if len(spots) < 3 {
		t.Fatal("expected at least three source spots")
	}
	for _, tc := range []struct {
		name, kind string
		limit      int
		wantCount  int
		wantNodes  int
	}{{"all limit one preserves ambiguity", "all", 1, 2, 10}, {"all limit five exposes more than three", "all", 5, 5, 10}, {"all limit ten exposes all seven", "all", 10, 7, 10}, {"spot retains its node cap", "spot", 5, 3, 3}} {
		t.Run(tc.name, func(t *testing.T) {
			c := mockClient(t, Options{NoCache: true}, nil)
			sourceCandidates := candidates
			query := "大久保"
			if tc.kind == "spot" {
				sourceCandidates = spots
				query = "Tokyo"
			}
			c.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				params := req.URL.Query()
				maxNodes, err := strconv.Atoi(params.Get("maxNodes"))
				if err != nil || maxNodes != tc.wantNodes {
					t.Errorf("source maxNodes=%q, want %d", params.Get("maxNodes"), tc.wantNodes)
				}
				sourceLimit, err := strconv.Atoi(params.Get("limit"))
				if err != nil || sourceLimit != max(2, tc.limit) {
					t.Errorf("source limit=%q, want %d", params.Get("limit"), max(2, tc.limit))
				}
				if tc.kind == "all" && params.Get("types") != "spot.station.airport.port" || tc.kind == "spot" && params.Get("types") != "spot" {
					t.Errorf("source candidate types=%q", params.Get("types"))
				}
				count := min(len(sourceCandidates), maxNodes, sourceLimit)
				body, err := json.Marshal(sourceCandidates[:count])
				if err != nil {
					return nil, err
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(body))), Request: req}, nil
			})
			result, err := c.Places(context.Background(), query, tc.kind, tc.limit)
			if err != nil || result.SourceCandidateCount != tc.wantCount || len(result.Places) != min(tc.limit, tc.wantCount) || result.SourceLimit != max(2, tc.limit) || !result.Ambiguous || c.Metrics().Requests != 1 {
				t.Fatalf("bounded source candidates: result=%+v err=%v metrics=%+v", result, err, c.Metrics())
			}
			if tc.limit == 1 && (len(result.Notes) < 2 || !strings.Contains(result.Notes[1], "truncated")) {
				t.Fatal("display truncation hid source ambiguity")
			}
			if tc.kind == "spot" {
				for _, place := range result.Places {
					if place.Kind != "spot" {
						t.Fatalf("spot source returned %s", place.Kind)
					}
				}
			}
		})
	}
}
func TestRetryRateLimitTimeoutAndBodyBound(t *testing.T) {
	t.Run("source wait exceeds bounded retry budget", func(t *testing.T) {
		c := mockClient(t, Options{NoCache: true}, nil)
		calls := 0
		c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": {"60"}}, Body: io.NopCloser(strings.NewReader("limited")), Request: r}, nil
		})
		_, err := c.fetch(context.Background(), sourceBase+"/public")
		var rate *cliutil.RateLimitError
		if !errors.As(err, &rate) || rate.RetryAfter != 60*time.Second || calls != 1 || c.Metrics().Retries != 0 {
			t.Fatalf("source throttle wait was shortened: %v calls=%d", err, calls)
		}
	})
	for _, tc := range []struct {
		name      string
		status    int
		wantRetry bool
	}{{"transient", 503, true}, {"permanent", 403, false}, {"rate-limit", 429, true}} {
		t.Run(tc.name, func(t *testing.T) {
			c := mockClient(t, Options{NoCache: true}, nil)
			calls := 0
			c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				status := tc.status
				if tc.status == 503 && calls > 1 {
					status = 200
				}
				return &http.Response{StatusCode: status, Header: http.Header{"Retry-After": {"1"}}, Body: io.NopCloser(strings.NewReader("[]")), Request: r}, nil
			})
			_, e := c.fetch(context.Background(), sourceBase+"/public")
			if tc.status == 429 {
				var rate *cliutil.RateLimitError
				if !errors.As(e, &rate) {
					t.Fatalf("throttle not typed: %v", e)
				}
			} else if tc.status == 503 && e != nil {
				t.Fatal(e)
			} else if tc.status == 403 && e == nil {
				t.Fatal("permanent error hidden")
			}
			want := 1
			if tc.wantRetry {
				want = 2
			}
			if calls != want || c.Metrics().Requests != want {
				t.Fatalf("bounded requests %d want %d", calls, want)
			}
		})
	}
	c := mockClient(t, Options{NoCache: true, Timeout: 10 * time.Millisecond}, nil)
	c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })
	start := time.Now()
	if _, e := c.fetch(context.Background(), sourceBase+"/slow"); e == nil || time.Since(start) > time.Second {
		t.Fatal("request timeout not bounded")
	}
	c = mockClient(t, Options{NoCache: true}, nil)
	c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", maxBodyBytes+1))), Request: r}, nil
	})
	if _, e := c.fetch(context.Background(), sourceBase+"/large"); e == nil {
		t.Fatal("oversize response accepted")
	}
}
func TestEmptySnapshotMetadataAndDisabledReads(t *testing.T) {
	c := mockClient(t, Options{NoCache: true}, nil)
	result, e := c.Show(context.Background(), "latest")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(result)
	for _, expected := range []string{`"source_url":null`, `"fetched_at":null`, `"age_seconds":null`, `"live_status_available":false`, `"seat_availability_available":false`, `"route":null`} {
		if !strings.Contains(string(b), expected) {
			t.Fatalf("missing explicit empty snapshot field %s", expected)
		}
	}
	if len(result.Notes) == 0 || !strings.Contains(result.Notes[0], "disabled by no-cache") {
		t.Fatal("disabled reads misreported as missing on disk")
	}
}
func TestOneInflightAndTransportDefaults(t *testing.T) {
	for _, tc := range []struct{ supplied, want time.Duration }{{0, 15 * time.Second}, {-time.Second, 15 * time.Second}, {time.Second, time.Second}, {60 * time.Second, 60 * time.Second}, {2 * time.Hour, 2 * time.Hour}} {
		c, e := NewClient(Options{Timeout: tc.supplied, NoCache: true})
		if e != nil {
			t.Fatal(e)
		}
		if c.HTTP.Jar != nil || c.opts.Timeout != tc.want || c.HTTP.Timeout != tc.want {
			t.Fatalf("timeout %s became provider=%s transport=%s, want %s", tc.supplied, c.opts.Timeout, c.HTTP.Timeout, tc.want)
		}
		req, _ := http.NewRequest("GET", sourceBase, nil)
		if c.HTTP.CheckRedirect(req, nil) != http.ErrUseLastResponse {
			t.Fatal("provider followed redirects")
		}
	}
	c := mockClient(t, Options{NoCache: true}, nil)
	var active, maxActive atomic.Int32
	c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		n := active.Add(1)
		for {
			old := maxActive.Load()
			if n <= old || maxActive.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		active.Add(-1)
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("[]")), Request: r}, nil
	})
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, e := c.fetch(context.Background(), sourceBase+"/public"); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	if maxActive.Load() != 1 {
		t.Fatal("more than one in-flight request")
	}
}
func TestExplicitLongTimeoutSharesOneContextBudgetAcrossRetry(t *testing.T) {
	c, err := NewClient(Options{NoCache: true, Timeout: 60 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	c.limiter = nil
	var deadline time.Time
	calls := 0
	c.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		requestDeadline, ok := req.Context().Deadline()
		if !ok || time.Until(requestDeadline) < 45*time.Second {
			t.Errorf("explicit 60-second context budget was shortened: deadline=%s present=%t", requestDeadline, ok)
		}
		if calls == 1 {
			deadline = requestDeadline
			return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": {"0"}}, Body: io.NopCloser(strings.NewReader("limited")), Request: req}, nil
		}
		if !requestDeadline.Equal(deadline) {
			t.Errorf("retry reset the request deadline: first=%s second=%s", deadline, requestDeadline)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("ok")), Request: req}, nil
	})
	body, err := c.fetch(context.Background(), sourceBase+"/public")
	if err != nil || string(body) != "ok" || calls != 2 || c.Metrics().Retries != 1 {
		t.Fatalf("shared-budget retry failed: body=%q err=%v calls=%d metrics=%+v", body, err, calls, c.Metrics())
	}
}
func TestMissingFareAndUnknownWalkRemainNull(t *testing.T) {
	source := string(fixture(t, "depart.html"))
	source = strings.ReplaceAll(source, `class="basic-fare js-route-fare"`, `class="unavailable-fare"`)
	source = strings.ReplaceAll(source, ">Nozomi</span>", ">Kanazawa Mystery</span>")
	r, _, e := parseRoutes([]byte(source), testQuery(), sourceBase)
	if e != nil {
		t.Fatal(e)
	}
	if r[0].Fare.TotalJPY != nil || r[0].WalkingMeters != nil || r[0].Legs[0].Kind != "unknown" {
		t.Fatal("missing fare/unclassified walking became zero or rail")
	}
	b, e := json.Marshal(r[0])
	if e != nil || !strings.Contains(string(b), `"total_jpy":null`) || !strings.Contains(string(b), `"walking_meters":null`) {
		t.Fatal("unknown metrics not explicit null")
	}
}
func TestLiveSmoke(t *testing.T) {
	if os.Getenv("NAVITIME_LIVE_SMOKE") != "1" {
		t.Skip("bounded public live smoke is opt-in")
	}
	c, e := NewClient(Options{NoCache: true})
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	places, e := c.Places(ctx, "Tokyo", "station", 3)
	if e != nil || len(places.Places) == 0 {
		t.Fatalf("live candidates: %v", e)
	}
	date := time.Now().In(japan).AddDate(0, 0, 1).Format("2006-01-02")
	q := Query{From: "station:00006668", To: "station:00001756", DepartAt: date + "T09:00"}
	result, e := c.Routes(ctx, q)
	if e != nil || len(result.Routes) == 0 {
		t.Fatalf("live route normalization: %v", e)
	}
	assertJSON(t, "live_provider", map[string]any{"candidate_count": len(places.Places), "route_count": len(result.Routes), "first_route": summary(result.Routes[0]), "metrics": c.Metrics()})
}
func TestProviderNegotiatesOnlyVerifiedResponseTypes(t *testing.T) {
	for _, tc := range []struct{ name, path, accept string }{
		{"route", "/en/area/jp/route/result/?start=00006668&goal=00001756", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"},
		{"autocomplete", "/en/async/route/autocomplete?word=Tokyo", "application/json"},
		{"unrelated", "/public", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := mockClient(t, Options{NoCache: true}, nil)
			c.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Header.Get("Accept") != tc.accept {
					t.Errorf("Accept=%q, want %q", req.Header.Get("Accept"), tc.accept)
				}
				if req.Header.Get("Cookie") != "" || req.Header.Get("Authorization") != "" {
					t.Error("credentials sent with public negotiation")
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("[]")), Request: req}, nil
			})
			if _, err := c.fetch(context.Background(), sourceBase+tc.path); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestLiveOctoberFirstNegotiation(t *testing.T) {
	if os.Getenv("NAVITIME_OCT1_SMOKE") != "1" {
		t.Skip("exact two-query public smoke is opt-in")
	}
	c, err := NewClient(Options{NoCache: true})
	if err != nil {
		t.Fatal(err)
	}
	base := c.HTTP.Transport
	statuses := []int{}
	sent := 0
	c.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if sent >= 2 {
			return nil, errors.New("two-GET smoke request budget exhausted")
		}
		sent++
		resp, err := base.RoundTrip(req)
		if resp != nil {
			statuses = append(statuses, resp.StatusCode)
		}
		return resp, err
	})
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	queries := []Query{{From: "station:00006668", To: "station:00001756", DepartAt: "2026-10-01T09:00"}, {From: "station:00006668", To: "station:00001756", ArriveBy: "2026-10-01T12:00"}}
	for _, q := range queries {
		before := c.Metrics()
		statusStart := len(statuses)
		result, err := c.Routes(ctx, q)
		after := c.Metrics()
		record := map[string]any{"query": q, "statuses": append([]int{}, statuses[statusStart:]...), "requests": after.Requests - before.Requests, "retries": after.Retries - before.Retries, "bytes_received": after.BytesReceived - before.BytesReceived, "cache_hit": result.Meta.CacheHit, "normalization_succeeded": err == nil, "route_count": len(result.Routes)}
		if err != nil {
			record["error"] = err.Error()
			t.Error(err)
		} else if len(result.Routes) == 0 {
			t.Error("successful response has no normalized alternatives")
		} else {
			record["first_route"] = summary(result.Routes[0])
		}
		assertJSON(t, "oct1_negotiation", record)
	}
	assertJSON(t, "oct1_totals", map[string]any{"actual_gets": sent, "metrics": c.Metrics()})
}
