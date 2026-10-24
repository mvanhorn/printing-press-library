// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package appstore

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil"
)

// rewriteTransport sends every request to the test server while keeping the
// original Apple host in a header, so handlers can dispatch by host.
type rewriteTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (rt rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Header.Set("X-Test-Orig-Host", req.URL.Host)
	r.URL.Scheme = rt.target.Scheme
	r.URL.Host = rt.target.Host
	r.Host = rt.target.Host
	return rt.base.RoundTrip(r)
}

type recordedRequest struct {
	Host   string
	Path   string
	Query  url.Values
	Header http.Header
}

type fakeApple struct {
	mu       sync.Mutex
	requests []recordedRequest
	handler  func(w http.ResponseWriter, r *http.Request, n int)
}

func (f *fakeApple) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

func (f *fakeApple) last() recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[len(f.requests)-1]
}

type captureRecorder struct {
	mu   sync.Mutex
	rows []Capture
}

func (c *captureRecorder) RecordCapture(_ context.Context, row Capture) error {
	c.mu.Lock()
	c.rows = append(c.rows, row)
	c.mu.Unlock()
	return nil
}

func (c *captureRecorder) all() []Capture {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Capture(nil), c.rows...)
}

func newTestClient(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, n int), mutate func(*Options)) (*Client, *fakeApple, *captureRecorder) {
	t.Helper()
	fake := &fakeApple{handler: handler}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fake.mu.Lock()
		fake.requests = append(fake.requests, recordedRequest{Host: r.Header.Get("X-Test-Orig-Host"), Path: r.URL.Path, Query: r.URL.Query(), Header: r.Header.Clone()})
		n := len(fake.requests)
		fake.mu.Unlock()
		fake.handler(w, r, n)
	}))
	t.Cleanup(srv.Close)
	target, _ := url.Parse(srv.URL)
	rec := &captureRecorder{}
	fast := map[RateClass]float64{}
	for class := range DefaultRates {
		fast[class] = 1000
	}
	opts := Options{
		Country:    "us",
		HTTPClient: &http.Client{Transport: rewriteTransport{target: target, base: http.DefaultTransport}},
		Rates:      fast,
		Jitter:     -1,
		RetryDelay: 10 * time.Millisecond,
		CacheDir:   t.TempDir(),
		Capture:    rec,
	}
	if mutate != nil {
		mutate(&opts)
	}
	c, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	return c, fake, rec
}

const lookupOneBody = `{"resultCount":1,"results":[{"wrapperType":"software","kind":"software","trackId":324684580,"trackName":"Spotify: Music and Podcasts","bundleId":"com.spotify.client","version":"9.1.92","currency":"USD","price":0,"userRatingCount":42703709,"averageUserRating":4.77,"trackViewUrl":"https://apps.apple.com/us/app/spotify-music-and-podcasts/id324684580?uo=4"}]}`

func TestClientThrottleStopsRun(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusForbidden} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			c, fake, rec := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
				w.Header().Set("Retry-After", "30")
				w.WriteHeader(status)
				_, _ = fmt.Fprint(w, "Rate limit has been exceeded")
			}, nil)
			_, err := c.Lookup(context.Background(), LookupQuery{IDs: []string{"324684580"}})
			var rl *cliutil.RateLimitError
			if !errors.As(err, &rl) {
				t.Fatalf("want *cliutil.RateLimitError, got %T %v", err, err)
			}
			// The error names Apple's Retry-After but asks the caller to wait
			// for the governor's hold, which is at least ThrottleHold: a retry
			// after Apple's 30 s would only be refused by the governor.
			if rl.RetryAfter < ThrottleHold-time.Second || rl.RetryAfter > ThrottleHold+time.Second ||
				!strings.Contains(rl.Body, fmt.Sprintf("HTTP %d", status)) || !strings.Contains(rl.Body, "Retry-After 30s") ||
				!strings.Contains(rl.Body, "holds every request to itunes.apple.com until") {
				t.Fatalf("rate limit error = %+v", rl)
			}
			if fake.count() != 1 {
				t.Fatalf("requests = %d, want 1 (no retry loop)", fake.count())
			}
			// The governor refuses every later request without touching the network.
			_, err = c.ReviewsRSS(context.Background(), "us", "324684580", 1, "recent")
			if !errors.As(err, &rl) {
				t.Fatalf("second call: want RateLimitError, got %v", err)
			}
			if fake.count() != 1 {
				t.Fatalf("governor let a request through: %d", fake.count())
			}
			if c.Tripped() == nil {
				t.Fatal("Tripped() = nil after a throttle")
			}
			if c.LimiterRate(RateITunes) >= 1000 {
				t.Fatalf("limiter not slowed by OnRateLimit: %v", c.LimiterRate(RateITunes))
			}
			rows := rec.all()
			if len(rows) != 1 || rows[0].Status != status || rows[0].Outcome != OutcomeFailed || rows[0].Error == "" {
				t.Fatalf("capture rows = %+v", rows)
			}
		})
	}
}

func TestClientRetriesServerErrorOnce(t *testing.T) {
	t.Run("recovers", func(t *testing.T) {
		c, fake, rec := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
			if n == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_, _ = fmt.Fprint(w, lookupOneBody)
		}, nil)
		resp, err := c.Lookup(context.Background(), LookupQuery{IDs: []string{"324684580"}})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Results) != 1 || fake.count() != 2 {
			t.Fatalf("results %d requests %d", len(resp.Results), fake.count())
		}
		rows := rec.all()
		if len(rows) != 2 || rows[0].Outcome != OutcomeFailed || rows[1].Outcome != OutcomeOK || rows[1].EntryCount != 1 {
			t.Fatalf("capture rows = %+v", rows)
		}
	})
	t.Run("gives up after one retry", func(t *testing.T) {
		c, fake, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
			w.WriteHeader(http.StatusBadGateway)
		}, nil)
		_, err := c.Lookup(context.Background(), LookupQuery{IDs: []string{"324684580"}})
		var he *HTTPError
		if !errors.As(err, &he) || he.Status != http.StatusBadGateway {
			t.Fatalf("want HTTPError 502, got %v", err)
		}
		if fake.count() != 2 {
			t.Fatalf("requests = %d, want 2", fake.count())
		}
		if c.Tripped() != nil {
			t.Fatal("a 5xx must not trip the governor")
		}
	})
}

func TestClientNotFound(t *testing.T) {
	c, fake, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
		http.NotFound(w, r)
	}, nil)
	_, err := c.ProductPage(context.Background(), ProductPageQuery{AppID: "1"})
	if !IsNotFound(err) {
		t.Fatalf("want NotFoundError, got %T %v", err, err)
	}
	if fake.count() != 1 {
		t.Fatalf("requests = %d", fake.count())
	}
}

func TestClientHeaders(t *testing.T) {
	c, fake, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
		w.Header().Set("Content-Type", "text/xml")
		_, _ = fmt.Fprint(w, string(readFixtureBytes(t, "hints_insta_us.plist")))
	}, func(o *Options) { o.Lang = "en-GB"; o.Country = "pk" })
	res, err := c.Hints(context.Background(), "", "insta")
	if err != nil {
		t.Fatal(err)
	}
	if res.Country != "pk" || len(res.Suggestions) < 5 {
		t.Fatalf("hints = %+v", res)
	}
	req := fake.last()
	if got := req.Header.Get("User-Agent"); got != DefaultUserAgent {
		t.Fatalf("User-Agent = %q, want the Chrome UA", got)
	}
	if !strings.Contains(DefaultUserAgent, "Chrome/131.0.0.0") {
		t.Fatalf("default UA is not Chrome 131: %s", DefaultUserAgent)
	}
	if got := req.Header.Get("Accept-Language"); got != "en-GB" {
		t.Fatalf("Accept-Language = %q", got)
	}
	if got := req.Header.Get("X-Apple-Store-Front"); got != "143477,12" {
		t.Fatalf("X-Apple-Store-Front = %q, want 143477,12", got)
	}
	if req.Host != "search.itunes.apple.com" || req.Query.Get("clientApplication") != "Software" || req.Query.Get("term") != "insta" {
		t.Fatalf("hints request = %+v", req)
	}
}

func readFixtureBytes(t *testing.T, name string) []byte { return readFixture(t, name) }

func TestClientCache(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request, n int) {
		w.Header().Set("Cache-Control", "max-age=86400")
		_, _ = fmt.Fprint(w, lookupOneBody)
	}
	t.Run("honoured", func(t *testing.T) {
		c, fake, rec := newTestClient(t, handler, nil)
		for i := 0; i < 2; i++ {
			resp, err := c.Lookup(context.Background(), LookupQuery{IDs: []string{"324684580"}})
			if err != nil || len(resp.Results) != 1 {
				t.Fatalf("call %d: %v", i, err)
			}
			if i == 1 && !resp.Meta[0].FromCache {
				t.Fatal("second call was not served from the cache")
			}
		}
		if fake.count() != 1 {
			t.Fatalf("requests = %d, want 1", fake.count())
		}
		if len(rec.all()) != 1 {
			t.Fatalf("cache hits must not be logged as captures: %d rows", len(rec.all()))
		}
	})
	t.Run("bypassed by NoCache", func(t *testing.T) {
		c, fake, _ := newTestClient(t, handler, func(o *Options) { o.NoCache = true })
		for i := 0; i < 2; i++ {
			if _, err := c.Lookup(context.Background(), LookupQuery{IDs: []string{"324684580"}}); err != nil {
				t.Fatal(err)
			}
		}
		if fake.count() != 2 {
			t.Fatalf("requests = %d, want 2", fake.count())
		}
	})
	t.Run("shared cache dir across clients", func(t *testing.T) {
		dir := t.TempDir()
		c1, fake1, _ := newTestClient(t, handler, func(o *Options) { o.CacheDir = dir })
		if _, err := c1.Lookup(context.Background(), LookupQuery{IDs: []string{"324684580"}}); err != nil {
			t.Fatal(err)
		}
		c2, fake2, _ := newTestClient(t, handler, func(o *Options) { o.CacheDir = dir })
		resp, err := c2.Lookup(context.Background(), LookupQuery{IDs: []string{"324684580"}})
		if err != nil || !resp.Meta[0].FromCache || fake1.count() != 1 || fake2.count() != 0 {
			t.Fatalf("err %v fromCache %v requests %d/%d", err, resp != nil && resp.Meta[0].FromCache, fake1.count(), fake2.count())
		}
	})
	t.Run("no-store and empty feeds are not cached", func(t *testing.T) {
		c, fake, rec := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
			if strings.Contains(r.URL.Path, "customerreviews") {
				_, _ = fmt.Fprint(w, `{"feed":{"updated":{"label":"2026-10-23T05:00:00-07:00"}}}`)
				return
			}
			w.Header().Set("Cache-Control", "max-age=0, no-cache, no-store")
			_, _ = fmt.Fprint(w, lookupOneBody)
		}, nil)
		for i := 0; i < 2; i++ {
			feed, err := c.ReviewsRSS(context.Background(), "us", "324684580", 1, "recent")
			if err != nil {
				t.Fatal(err)
			}
			if feed.Outcome != OutcomeVerifiedEmpty || len(feed.Reviews) != 0 {
				t.Fatalf("empty feed outcome = %s", feed.Outcome)
			}
			if _, err := c.Lookup(context.Background(), LookupQuery{IDs: []string{"324684580"}}); err != nil {
				t.Fatal(err)
			}
		}
		if fake.count() != 4 {
			t.Fatalf("requests = %d, want 4 (nothing cached)", fake.count())
		}
		var empty int
		for _, r := range rec.all() {
			if r.Outcome == OutcomeVerifiedEmpty {
				empty++
			}
		}
		if empty != 2 {
			t.Fatalf("verified-empty capture rows = %d, want 2", empty)
		}
	})
}

func TestCacheTTL(t *testing.T) {
	cases := []struct {
		header string
		want   time.Duration
	}{
		{"", DefaultPageTTL},
		{"public, max-age=900", 900 * time.Second},
		// Lookup and Search advertise up to a day; every response feeds a
		// time series, so the cache keeps them at most 15 minutes.
		{"max-age=39136", MaxCacheTTL},
		{"max-age=86400", MaxCacheTTL},
		{"max-age=999999", MaxCacheTTL},
		{"no-transform, max-age=600", 600 * time.Second},
		{"max-age=0, no-cache, no-store", 0},
		{"private", 0},
	}
	for _, tc := range cases {
		if got := cacheTTL(tc.header); got != tc.want {
			t.Errorf("cacheTTL(%q) = %v, want %v", tc.header, got, tc.want)
		}
	}
	if MaxCacheTTL != 15*time.Minute {
		t.Fatalf("MaxCacheTTL = %v, want 15m (snapshot reads must not be served stale for a day)", MaxCacheTTL)
	}
}

func TestClassifyURL(t *testing.T) {
	cases := []struct {
		url   string
		class RateClass
		err   bool
	}{
		{"https://itunes.apple.com/search?term=spotify&media=software", RateSearchAPI, false},
		{"https://itunes.apple.com/lookup?id=324684580", RateITunes, false},
		{"https://itunes.apple.com/us/rss/topfreeapplications/limit=100/json", RateITunes, false},
		{"https://itunes.apple.com/us/rss/customerreviews/page=1/id=1/sortby=mostrecent/json", RateITunes, false},
		{"https://apps.apple.com/us/app/id324684580", RatePages, false},
		{"https://apps.apple.com/us/iphone/charts/36?chart=top-free", RatePages, false},
		{"https://apps.apple.com/api/apps/v1/catalog/us/apps/1/reviews", RateStoreAPI, false},
		{"https://search.itunes.apple.com/WebObjects/MZSearchHints.woa/wa/hints?term=a", RateHints, false},
		{"https://example.com/lookup", "", true},
		{"http://itunes.apple.com/lookup", "", true},
		{"https://amp-api-edge.apps.apple.com/v1/catalog/us/apps/1", "", true},
	}
	for _, tc := range cases {
		u, _ := url.Parse(tc.url)
		got, err := ClassifyURL(u)
		if (err != nil) != tc.err || got != tc.class {
			t.Errorf("ClassifyURL(%s) = %q, %v; want %q, err=%v", tc.url, got, err, tc.class, tc.err)
		}
	}
	// Pages go one per 20 s (2026-10-24: at 1/s Apple throttled 2 of 6
	// uncached page fetches).
	if DefaultRates[RateSearchAPI] != 0.3 || DefaultRates[RateStoreAPI] != 0.5 || DefaultRates[RatePages] != 1.0/20 || DefaultRates[RateITunes] != 1 || DefaultRates[RateHints] != 1 {
		t.Fatalf("default rates changed: %v", DefaultRates)
	}
}

func TestClientLimiterPerHost(t *testing.T) {
	c, _, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/search"):
			_, _ = fmt.Fprint(w, lookupOneBody)
		case strings.HasPrefix(r.URL.Path, "/lookup"):
			_, _ = fmt.Fprint(w, lookupOneBody)
		case strings.Contains(r.URL.Path, "/charts/"):
			_, _ = w.Write(readFixture(t, "charts_us_iphone_36.html"))
		default:
			_, _ = w.Write(readFixture(t, "hints_insta_us.plist"))
		}
	}, nil)
	ctx := context.Background()
	if _, err := c.Search(ctx, SearchQuery{Term: "spotify", Limit: 5}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Lookup(ctx, LookupQuery{IDs: []string{"324684580"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ChartsPage(ctx, ChartsPageQuery{GenreID: "36"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Hints(ctx, "us", "insta"); err != nil {
		t.Fatal(err)
	}
	stats := c.Stats()
	want := map[RateClass]int{RateSearchAPI: 1, RateITunes: 1, RatePages: 1, RateHints: 1}
	for class, n := range want {
		if stats[class] != n {
			t.Fatalf("stats[%s] = %d, want %d (all %v)", class, stats[class], n, stats)
		}
	}
	if c.Requests() != 4 {
		t.Fatalf("requests = %d", c.Requests())
	}
}

func TestClientPacing(t *testing.T) {
	c, fake, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
		_, _ = fmt.Fprint(w, lookupOneBody)
	}, func(o *Options) {
		o.NoCache = true
		o.Rates = map[RateClass]float64{RateITunes: 20} // 50 ms interval
	})
	start := time.Now()
	for i := 0; i < 4; i++ {
		if _, err := c.Lookup(context.Background(), LookupQuery{IDs: []string{"324684580"}}); err != nil {
			t.Fatal(err)
		}
	}
	elapsed := time.Since(start)
	if fake.count() != 4 {
		t.Fatalf("requests = %d", fake.count())
	}
	if elapsed < 140*time.Millisecond {
		t.Fatalf("4 requests at 20/s took %v, want >= 150ms of pacing", elapsed)
	}
}

// sendTimes is a RoundTripper that records when each request is sent.
type sendTimes struct {
	mu    sync.Mutex
	times []time.Time
	base  http.RoundTripper
}

func (s *sendTimes) RoundTrip(r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	s.times = append(s.times, time.Now())
	s.mu.Unlock()
	return s.base.RoundTrip(r)
}

func (s *sendTimes) all() []time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]time.Time(nil), s.times...)
}

// Jitter must survive the limiter: with responses far faster than the
// interval, consecutive request gaps still vary by more than ~10% of the
// interval, and no gap is shorter than the interval (the rate is a ceiling).
func TestClientJitterSpreadsRequestGaps(t *testing.T) {
	// Send times are taken in the client transport, so connection setup and
	// server scheduling do not blur the gaps.
	sends := &sendTimes{}
	c, _, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
		_, _ = fmt.Fprint(w, lookupOneBody)
	}, func(o *Options) {
		o.NoCache = true
		o.Rates = map[RateClass]float64{RateITunes: 20} // 50 ms interval
		o.Jitter = 0.2
		sends.base = o.HTTPClient.Transport
		o.HTTPClient = &http.Client{Transport: sends}
	})
	for i := 0; i < 20; i++ {
		if _, err := c.Lookup(context.Background(), LookupQuery{IDs: []string{"324684580"}}); err != nil {
			t.Fatal(err)
		}
	}
	interval := 50 * time.Millisecond
	minGap, maxGap := time.Hour, time.Duration(0)
	arrivals := sends.all()
	if len(arrivals) != 20 {
		t.Fatalf("requests = %d, want 20", len(arrivals))
	}
	for i := 1; i < len(arrivals); i++ {
		gap := arrivals[i].Sub(arrivals[i-1])
		if gap < minGap {
			minGap = gap
		}
		if gap > maxGap {
			maxGap = gap
		}
	}
	// Send times include goroutine scheduling noise (heavy under parallel
	// package tests), so the ceiling check allows 10%; the spread check is
	// the regression signal (a fixed rhythm spreads by about 2 ms).
	if minGap < interval*9/10 {
		t.Fatalf("shortest gap %v is below the %v interval: the rate is no longer a ceiling", minGap, interval)
	}
	if spread := maxGap - minGap; spread < interval/10 {
		t.Fatalf("gap spread %v (min %v, max %v) is under 10%% of the %v interval: the jitter was absorbed and requests keep a fixed rhythm", spread, minGap, maxGap, interval)
	}
}

func TestClientRedirects(t *testing.T) {
	t.Run("follows apple.com redirects and caches both URLs", func(t *testing.T) {
		page := readFixture(t, "product_spotify_us.html")
		c, fake, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
			if r.URL.Path == "/us/app/id324684580" {
				w.Header().Set("Location", "https://apps.apple.com/us/app/spotify-music-and-podcasts/id324684580")
				w.Header().Set("Cache-Control", "public, max-age=900")
				w.WriteHeader(http.StatusMovedPermanently)
				return
			}
			w.Header().Set("Cache-Control", "public, max-age=900")
			_, _ = w.Write(page)
		}, nil)
		p, err := c.ProductPage(context.Background(), ProductPageQuery{AppID: "324684580"})
		if err != nil {
			t.Fatal(err)
		}
		if p.AdamID != "324684580" || fake.count() != 2 {
			t.Fatalf("adamId %s requests %d", p.AdamID, fake.count())
		}
		if _, err := c.ProductPage(context.Background(), ProductPageQuery{URL: "https://apps.apple.com/us/app/spotify-music-and-podcasts/id324684580?uo=4"}); err != nil {
			t.Fatal(err)
		}
		if _, err := c.ProductPage(context.Background(), ProductPageQuery{AppID: "324684580"}); err != nil {
			t.Fatal(err)
		}
		if fake.count() != 2 {
			t.Fatalf("requests = %d, want 2 (both URLs cached)", fake.count())
		}
	})
	t.Run("refuses redirects off Apple's hosts", func(t *testing.T) {
		c, _, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
			w.Header().Set("Location", "https://evil.example.com/x")
			w.WriteHeader(http.StatusFound)
		}, nil)
		_, err := c.ProductPage(context.Background(), ProductPageQuery{AppID: "324684580"})
		if err == nil || !strings.Contains(err.Error(), "refusing") {
			t.Fatalf("want a refusal, got %v", err)
		}
	})
}

func TestClientLookupBatching(t *testing.T) {
	ids := make([]string, 0, 160)
	for i := 0; i < 160; i++ {
		ids = append(ids, fmt.Sprint(1000000+i))
	}
	c, fake, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
		req := strings.Split(r.URL.Query().Get("id"), ",")
		parts := make([]string, 0, len(req))
		// Answer in reverse order and drop every 50th id.
		for i := len(req) - 1; i >= 0; i-- {
			var id int
			_, _ = fmt.Sscan(req[i], &id)
			if id%50 == 0 {
				continue
			}
			parts = append(parts, fmt.Sprintf(`{"wrapperType":"software","trackId":%d,"trackName":"App %d"}`, id, id))
		}
		_, _ = fmt.Fprintf(w, `{"resultCount":%d,"results":[%s]}`, len(parts), strings.Join(parts, ","))
	}, nil)
	resp, err := c.Lookup(context.Background(), LookupQuery{IDs: append(ids, "1000001")})
	if err != nil {
		t.Fatal(err)
	}
	if fake.count() != 2 || resp.Requests != 2 {
		t.Fatalf("requests = %d (%d), want 2 batches", fake.count(), resp.Requests)
	}
	if n := len(strings.Split(fake.requests[0].Query.Get("id"), ",")); n != LookupBatchSize {
		t.Fatalf("first batch = %d ids, want %d", n, LookupBatchSize)
	}
	if len(resp.Missing) != 4 || resp.Missing[0] != "1000000" {
		t.Fatalf("missing = %v", resp.Missing)
	}
	if len(resp.Results)+len(resp.Missing) != 160 {
		t.Fatalf("results %d + missing %d != 160", len(resp.Results), len(resp.Missing))
	}
	for i := 1; i < len(resp.Results); i++ {
		if resp.Results[i-1].TrackID >= resp.Results[i].TrackID {
			t.Fatalf("results not in input order at %d", i)
		}
	}
	if _, err := c.Lookup(context.Background(), LookupQuery{IDs: []string{"324684580", "com.bad"}}); err == nil {
		t.Fatal("a non-numeric id must be rejected before the request (Apple fails the whole batch)")
	}
}

// A batch that fails after earlier batches completed must not drop them:
// Lookup returns the completed batches together with the error, and the ids
// it never asked about are unresolved, not missing.
func TestClientLookupKeepsCompletedBatchesOnFailure(t *testing.T) {
	ids := make([]string, 0, 200)
	for i := 0; i < 200; i++ {
		ids = append(ids, fmt.Sprint(2000000+i))
	}
	answer := func(w http.ResponseWriter, r *http.Request) {
		req := strings.Split(r.URL.Query().Get("id"), ",")
		parts := make([]string, 0, len(req))
		for _, id := range req[1:] { // the first id of each batch is unknown to Apple
			parts = append(parts, fmt.Sprintf(`{"wrapperType":"software","trackId":%s,"trackName":"App %s"}`, id, id))
		}
		_, _ = fmt.Fprintf(w, `{"resultCount":%d,"results":[%s]}`, len(parts), strings.Join(parts, ","))
	}
	for _, tc := range []struct {
		name     string
		status   int
		throttle bool
	}{{"server error", http.StatusInternalServerError, false}, {"throttle", http.StatusTooManyRequests, true}} {
		t.Run(tc.name, func(t *testing.T) {
			c, _, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
				if n == 1 {
					answer(w, r)
					return
				}
				w.WriteHeader(tc.status)
			}, nil)
			resp, err := c.Lookup(context.Background(), LookupQuery{IDs: ids})
			if err == nil {
				t.Fatal("the failed second batch must surface its error")
			}
			if _, isThrottle := AsRateLimit(err); isThrottle != tc.throttle {
				t.Fatalf("error = %v, throttle %v want %v", err, isThrottle, tc.throttle)
			}
			if resp == nil || len(resp.Results) != LookupBatchSize-1 || resp.Requests != 1 {
				t.Fatalf("completed batch dropped: resp = %+v", resp)
			}
			if resp.Results[0].AppID() != ids[1] || resp.Results[len(resp.Results)-1].AppID() != ids[LookupBatchSize-1] {
				t.Fatalf("results not the first batch in input order: %s .. %s", resp.Results[0].AppID(), resp.Results[len(resp.Results)-1].AppID())
			}
			if len(resp.Missing) != 1 || resp.Missing[0] != ids[0] {
				t.Fatalf("missing = %v, want only %s (ids never asked are not missing)", resp.Missing, ids[0])
			}
			if len(resp.Unresolved) != 200-LookupBatchSize || resp.Unresolved[0] != ids[LookupBatchSize] {
				t.Fatalf("unresolved = %d ids starting %v", len(resp.Unresolved), resp.Unresolved[:1])
			}
		})
	}
	t.Run("bundle ids", func(t *testing.T) {
		c, _, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
			if n == 1 {
				_, _ = fmt.Fprint(w, lookupOneBody)
				return
			}
			w.WriteHeader(http.StatusBadGateway)
		}, nil)
		resp, err := c.Lookup(context.Background(), LookupQuery{BundleIDs: []string{"com.spotify.client", "com.example.two", "com.example.three"}})
		if err == nil || resp == nil || len(resp.Results) != 1 || strings.Join(resp.Unresolved, ",") != "com.example.two,com.example.three" {
			t.Fatalf("bundle lookup partial failure = %+v, %v", resp, err)
		}
	})
}

func TestClientDeveloperLookup(t *testing.T) {
	c, fake, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
		_, _ = w.Write(readFixture(t, "lookup_developer_spotify_us.json"))
	}, nil)
	resp, err := c.Lookup(context.Background(), LookupQuery{ArtistID: "324684583"})
	if err != nil {
		t.Fatal(err)
	}
	q := fake.last().Query
	if q.Get("id") != "324684583" || q.Get("entity") != "software" || q.Get("limit") != "200" {
		t.Fatalf("developer lookup query = %v", q)
	}
	if resp.Artist == nil || resp.Artist.ArtistName != "Spotify" || len(resp.Results) != 4 {
		t.Fatalf("artist %+v results %d", resp.Artist, len(resp.Results))
	}
	for _, r := range resp.Results {
		if r.WrapperType != "software" {
			t.Fatalf("artist row leaked into results: %+v", r)
		}
	}
}

func TestClientFeedsAndPages(t *testing.T) {
	c, fake, rec := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
		switch {
		case strings.Contains(r.URL.Path, "customerreviews"):
			_, _ = w.Write(readFixture(t, "reviews_rss_spotify_us_p1_recent.json"))
		case strings.Contains(r.URL.Path, "topgrossingapplications"):
			_, _ = w.Write(readFixture(t, "rss_topgrossing_us_6014.json"))
		case strings.Contains(r.URL.Path, "/today"):
			_, _ = w.Write(readFixture(t, "today_us_iphone.html"))
		case strings.Contains(r.URL.Path, "/developer/"):
			_, _ = w.Write(readFixture(t, "developer_spotify_us.html"))
		case r.URL.Query().Get("see-all") != "":
			_, _ = w.Write(readFixture(t, "seeall_reviews_spotify_us.html"))
		default:
			http.NotFound(w, r)
		}
	}, nil)
	ctx := context.Background()
	feed, err := c.ReviewsRSS(ctx, "us", "id324684580", 1, "recent")
	if err != nil || len(feed.Reviews) != 50 || feed.Sort != SortMostRecent || feed.Oldest == "" {
		t.Fatalf("reviews: %v %+v", err, feed)
	}
	if p := fake.last().Path; p != "/us/rss/customerreviews/page=1/id=324684580/sortby=mostrecent/json" {
		t.Fatalf("reviews path = %s", p)
	}
	chart, err := c.ChartRSS(ctx, "us", "topgrossingapplications", "6014", 200)
	if err != nil || len(chart.Entries) != 100 || chart.Limit != 100 {
		t.Fatalf("chart rss: %v", err)
	}
	if p := fake.last().Path; p != "/us/rss/topgrossingapplications/limit=100/genre=6014/json" {
		t.Fatalf("chart path = %s", p)
	}
	today, err := c.TodayPage(ctx, "us", "iphone")
	if err != nil || len(today.Stories) != 32 {
		t.Fatalf("today: %v", err)
	}
	dev, err := c.DeveloperPage(ctx, "us", "324684583", "")
	if err != nil || len(dev.Shelves) == 0 {
		t.Fatalf("developer: %v", err)
	}
	sa, err := c.SeeAllPage(ctx, "us", "324684580", "reviews", "")
	if err != nil || len(sa.Reviews) != 10 {
		t.Fatalf("see-all: %v", err)
	}
	if _, err := c.ReviewsRSS(ctx, "us", "324684580", 11, "recent"); err == nil {
		t.Fatal("page 11 must be rejected locally (Apple answers 400)")
	}
	if _, err := c.ChartRSS(ctx, "us", "topsecretapps", "", 10); err == nil {
		t.Fatal("unknown feed must be rejected")
	}
	if _, err := c.PageOfKind(ctx, "https://apps.apple.com/us/developer/id324684583", KindChartsPage); err == nil {
		t.Fatal("kind mismatch must be an error")
	}
	var ok int
	for _, r := range rec.all() {
		if r.Outcome == OutcomeOK && r.ParserVersion == ParserVersion {
			ok++
		}
	}
	if ok < 5 {
		t.Fatalf("ok capture rows = %d", ok)
	}
}

func TestClientOptInSurfaces(t *testing.T) {
	c, fake, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
		_, _ = w.Write(readFixture(t, "api_reviews_candycrush_us.json"))
	}, nil)
	ctx := context.Background()
	if _, err := c.APIReviews(ctx, "us", "553834731", 0, 20, "recent"); !errors.Is(err, ErrOptInRequired) {
		t.Fatalf("want ErrOptInRequired, got %v", err)
	}
	if _, err := c.WebSearch(ctx, "us", "iphone", "spotify"); !errors.Is(err, ErrOptInRequired) {
		t.Fatalf("want ErrOptInRequired, got %v", err)
	}
	if _, err := c.Page(ctx, "https://apps.apple.com/api/apps/v1/catalog/us/apps/1"); err == nil {
		t.Fatal("raw /api fetch must be refused without opt-in")
	}
	if fake.count() != 0 {
		t.Fatalf("opt-in surfaces sent %d requests before opt-in", fake.count())
	}
	c.EnableOptInSurfaces()
	page, err := c.APIReviews(ctx, "us", "553834731", 10, 50, "helpful")
	if err != nil || len(page.Reviews) != 10 {
		t.Fatalf("api reviews: %v", err)
	}
	req := fake.last()
	if req.Host != "apps.apple.com" || req.Path != "/api/apps/v1/catalog/us/apps/553834731/reviews" {
		t.Fatalf("api request = %+v", req)
	}
	if req.Header.Get("Authorization") != "Bearer" && req.Header.Get("Authorization") != "Bearer " {
		t.Fatalf("Authorization = %q", req.Header.Get("Authorization"))
	}
	if req.Header.Get("Origin") != "https://apps.apple.com" || req.Query.Get("limit") != "20" || req.Query.Get("offset") != "10" || req.Query.Get("sort") != "helpful" {
		t.Fatalf("api headers/query = %v %v", req.Header, req.Query)
	}
	if c.Stats()[RateStoreAPI] != 1 {
		t.Fatalf("api request not paced in the apps-api class: %v", c.Stats())
	}
}

func TestNewRejectsUnknownCountry(t *testing.T) {
	if _, err := New(Options{Country: "zz"}); err == nil {
		t.Fatal("unknown storefront accepted")
	}
	c, err := New(Options{Country: "UK", CacheDir: t.TempDir()})
	if err != nil || c.Country() != "gb" {
		t.Fatalf("alias uk: %v %s", err, c.Country())
	}
}

// A 404 is "not found" only for an id-addressed apps.apple.com resource.
// Surfaces that answer unknown ids with an empty 200, or that are not
// id-addressed, report a 404 as an API failure (*HTTPError), e.g. the
// transient /search 404 in the research error catalogue.
func TestClient404TypingDependsOnTheSurface(t *testing.T) {
	ctx := context.Background()
	newClient := func(t *testing.T) *Client {
		c, _, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
			http.NotFound(w, r)
		}, func(o *Options) { o.NoCache = true })
		c.EnableOptInSurfaces()
		return c
	}
	failures := map[string]func(c *Client) error{
		"search":      func(c *Client) error { _, err := c.Search(ctx, SearchQuery{Term: "spotify"}); return err },
		"lookup":      func(c *Client) error { _, err := c.Lookup(ctx, LookupQuery{IDs: []string{"324684580"}}); return err },
		"chart rss":   func(c *Client) error { _, err := c.ChartRSS(ctx, "us", "topfreeapplications", "", 100); return err },
		"reviews rss": func(c *Client) error { _, err := c.ReviewsRSS(ctx, "us", "324684580", 1, "recent"); return err },
		"hints":       func(c *Client) error { _, err := c.Hints(ctx, "us", "insta"); return err },
		"chart page": func(c *Client) error {
			_, err := c.ChartsPage(ctx, ChartsPageQuery{Platform: "iphone", Chart: "top-free"})
			return err
		},
		"today page": func(c *Client) error { _, err := c.TodayPage(ctx, "us", "iphone"); return err },
	}
	for name, call := range failures {
		t.Run(name, func(t *testing.T) {
			err := call(newClient(t))
			var httpErr *HTTPError
			if IsNotFound(err) || !errors.As(err, &httpErr) || httpErr.Status != http.StatusNotFound {
				t.Fatalf("%s 404 = %T %v, want *HTTPError with status 404", name, err, err)
			}
		})
	}
	missing := map[string]func(c *Client) error{
		"product page":   func(c *Client) error { _, err := c.ProductPage(ctx, ProductPageQuery{AppID: "1"}); return err },
		"developer page": func(c *Client) error { _, err := c.DeveloperPage(ctx, "us", "1", ""); return err },
		"see-all page":   func(c *Client) error { _, err := c.SeeAllPage(ctx, "us", "1", "reviews", ""); return err },
		"api app":        func(c *Client) error { _, err := c.APIApp(ctx, "us", "1", APIAppOptions{}); return err },
		"api reviews":    func(c *Client) error { _, err := c.APIReviews(ctx, "us", "1", 0, 20, "recent"); return err },
	}
	for name, call := range missing {
		t.Run(name, func(t *testing.T) {
			if err := call(newClient(t)); !IsNotFound(err) {
				t.Fatalf("%s 404 = %T %v, want NotFoundError", name, err, err)
			}
		})
	}
}

// The same app given as "id324684580", " 324684580 " and "324684580" is one
// id: it is sent once and returned once.
func TestClientLookupNormalizesIDsBeforeDedupe(t *testing.T) {
	c, fake, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
		_, _ = fmt.Fprint(w, lookupOneBody)
	}, func(o *Options) { o.NoCache = true })
	resp, err := c.Lookup(context.Background(), LookupQuery{IDs: []string{"id324684580", " 324684580 ", "324684580", "ID324684580"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := fake.last().Query.Get("id"); got != "324684580" {
		t.Fatalf("sent id=%q, want each app once", got)
	}
	if len(resp.Results) != 1 || len(resp.Missing) != 0 {
		t.Fatalf("results %d missing %v, want one result", len(resp.Results), resp.Missing)
	}
	if _, err := c.Lookup(context.Background(), LookupQuery{IDs: []string{"idabc"}}); err == nil || !strings.Contains(err.Error(), "idabc") {
		t.Fatalf("a bad id must be rejected with the value given: %v", err)
	}
}

// --lang selects the language of apps.apple.com pages with ?l= (Apple's page
// CDN ignores Accept-Language), which also keeps a page cached in one
// language from being served for another.
func TestClientLangSelectsThePageLanguage(t *testing.T) {
	charts := readFixture(t, "charts_us_iphone_36.html")
	cacheDir := t.TempDir()
	fakes := make([]*fakeApple, 0, 3)
	newClient := func(lang string) *Client {
		c, fake, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
			w.Header().Set("Cache-Control", "public, max-age=900")
			_, _ = w.Write(charts)
		}, func(o *Options) { o.Lang = lang; o.CacheDir = cacheDir })
		fakes = append(fakes, fake)
		return c
	}
	ja := newClient("ja")
	if _, err := ja.ChartsPage(context.Background(), ChartsPageQuery{Platform: "iphone", Chart: "top-free"}); err != nil {
		t.Fatal(err)
	}
	def := newClient("")
	page, err := def.ChartsPage(context.Background(), ChartsPageQuery{Platform: "iphone", Chart: "top-free"})
	if err != nil {
		t.Fatal(err)
	}
	if page.FromCache {
		t.Fatal("a page cached for --lang ja was served to a client without --lang")
	}
	// A second ja client is served from the ja cache entry.
	again := newClient("ja")
	if p, err := again.ChartsPage(context.Background(), ChartsPageQuery{Platform: "iphone", Chart: "top-free"}); err != nil || !p.FromCache {
		t.Fatalf("same-language rerun: fromCache %v err %v", p != nil && p.FromCache, err)
	}
	if fakes[0].count() != 1 || fakes[1].count() != 1 || fakes[2].count() != 0 {
		t.Fatalf("requests ja %d, default %d, ja again %d; want 1, 1, 0", fakes[0].count(), fakes[1].count(), fakes[2].count())
	}
	jaReq, defReq := fakes[0].last(), fakes[1].last()
	if jaReq.Query.Get("l") != "ja" || jaReq.Header.Get("Accept-Language") != "ja" {
		t.Fatalf("ja page request: l=%q Accept-Language=%q, want ja for both", jaReq.Query.Get("l"), jaReq.Header.Get("Accept-Language"))
	}
	if defReq.Query.Get("l") != "" || defReq.Header.Get("Accept-Language") != "" {
		t.Fatalf("default page request carries a language: l=%q Accept-Language=%q", defReq.Query.Get("l"), defReq.Header.Get("Accept-Language"))
	}
	if got := withPageLang("https://apps.apple.com/us/app/id1?l=en-GB", "ja"); got != "https://apps.apple.com/us/app/id1?l=en-GB" {
		t.Fatalf("an explicit l= must be kept: %s", got)
	}
	if got := withPageLang("https://itunes.apple.com/lookup?id=1", "ja"); strings.Contains(got, "l=ja") {
		t.Fatalf("non-page URLs get no l=: %s", got)
	}
}

// Results carry when Apple produced the data, not only when it was fetched:
// the Lookup/Search edge counts max-age down from 86,400 s instead of
// sending Age (max-age=39985 is a copy 46,415 s old), pages send Age, and a
// cached result keeps both.
func TestResultsCarryDataAge(t *testing.T) {
	now := time.Date(2026, 10, 23, 19, 28, 54, 0, time.UTC)
	clock := func() time.Time { return now }
	page := readFixture(t, "product_spotify_us.html")
	c, _, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
		switch {
		case r.URL.Path == "/lookup" && r.URL.Query().Get("id") == "324684580":
			w.Header().Set("Cache-Control", "max-age=39985")
			_, _ = fmt.Fprint(w, lookupOneBody)
		case r.URL.Path == "/lookup":
			w.Header().Set("Cache-Control", "max-age=86400")
			_, _ = fmt.Fprint(w, `{"resultCount":1,"results":[{"wrapperType":"software","trackId":553834731,"trackName":"Candy Crush Saga"}]}`)
		default:
			w.Header().Set("Cache-Control", "public, max-age=900")
			w.Header().Set("Age", "332")
			_, _ = w.Write(page)
		}
	}, func(o *Options) { o.Now = clock })
	ctx := context.Background()

	stale, err := c.Lookup(ctx, LookupQuery{IDs: []string{"324684580"}})
	if err != nil {
		t.Fatal(err)
	}
	wantStale := now.Add(-(86400 - 39985) * time.Second)
	m := stale.Meta[0]
	if m.MaxAge != 39985*time.Second || m.Age != 0 || !m.DataAsOf.Equal(wantStale) || !stale.Results[0].DataAsOf.Equal(wantStale) {
		t.Fatalf("stale Lookup copy: maxAge %v age %v dataAsOf %v / result %v, want %v", m.MaxAge, m.Age, m.DataAsOf, stale.Results[0].DataAsOf, wantStale)
	}
	fresh, err := c.Lookup(ctx, LookupQuery{IDs: []string{"553834731"}})
	if err != nil {
		t.Fatal(err)
	}
	if !fresh.Results[0].DataAsOf.Equal(now) {
		t.Fatalf("a fresh Lookup copy (max-age=86400) dates from its fetch: %v, want %v", fresh.Results[0].DataAsOf, now)
	}

	p, err := c.ProductPage(ctx, ProductPageQuery{URL: "https://apps.apple.com/us/app/spotify-music-and-podcasts/id324684580"})
	if err != nil {
		t.Fatal(err)
	}
	if !p.DataAsOf.Equal(now.Add(-332 * time.Second)) {
		t.Fatalf("page with Age 332: dataAsOf %v, want %v", p.DataAsOf, now.Add(-332*time.Second))
	}

	// Ten minutes later both come from the local cache with their data time.
	now = now.Add(10 * time.Minute)
	again, err := c.Lookup(ctx, LookupQuery{IDs: []string{"324684580"}})
	if err != nil || !again.FromCache || !again.Results[0].DataAsOf.Equal(wantStale) {
		t.Fatalf("cached Lookup: fromCache %v dataAsOf %v, want %v (%v)", again != nil && again.FromCache, again.Results[0].DataAsOf, wantStale, err)
	}
	cached, err := c.ProductPage(ctx, ProductPageQuery{URL: "https://apps.apple.com/us/app/spotify-music-and-podcasts/id324684580"})
	if err != nil || !cached.FromCache || !cached.DataAsOf.Equal(p.DataAsOf) {
		t.Fatalf("cached page: fromCache %v dataAsOf %v, want %v (%v)", cached != nil && cached.FromCache, cached.DataAsOf, p.DataAsOf, err)
	}
}
