package tenki

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func cacheClient(t *testing.T, cfg Config, status int, body string, fail error) *Client {
	t.Helper()
	cfg.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if fail != nil {
			return nil, fail
		}
		return response(r, status, body), nil
	})
	c := NewClient(cfg)
	c.limiter = nil
	return c
}

func TestCacheWarmRefreshLocalAndExplicitStale(t *testing.T) {
	ctx := context.Background()
	now := snapshotNow
	dir := t.TempDir()
	body := fixture(t, "daily-20260927.html")
	cfg := Config{CacheDir: dir, Now: func() time.Time { return now }}
	c := cacheClient(t, cfg, 200, body, nil)
	first, e := c.Daily(ctx, chiyoda)
	if e != nil {
		t.Fatal(e)
	}
	second, e := c.Daily(ctx, chiyoda)
	if e != nil {
		t.Fatal(e)
	}
	if first.Source.FromCache || !second.Source.FromCache || c.Metrics().HTTPRequests != 1 || c.Metrics().CacheHits != 1 || c.Metrics().ResponseBytes != int64(len(body)) {
		t.Fatalf("cold/warm metadata %+v %+v %+v", first.Source, second.Source, c.Metrics())
	}
	fresh := cacheClient(t, cfg, 500, "failure", nil)
	disk, e := fresh.Daily(ctx, chiyoda)
	if e != nil {
		t.Fatal(e)
	}
	if !disk.Source.FromCache || fresh.Metrics().HTTPRequests != 0 {
		t.Fatal("fresh disk cache made HTTP request")
	}
	cfg.Refresh = true
	refresh := cacheClient(t, cfg, 200, body, nil)
	refreshed, e := refresh.Daily(ctx, chiyoda)
	if e != nil {
		t.Fatal(e)
	}
	if refreshed.Source.FromCache || refresh.Metrics().HTTPRequests != 1 {
		t.Fatal("refresh did not bypass initial disk cache")
	}
	if _, e = refresh.Daily(ctx, chiyoda); e != nil {
		t.Fatal(e)
	}
	if refresh.Metrics().HTTPRequests != 1 {
		t.Fatal("per-run refresh dedup failed")
	}
	now = now.Add(3 * time.Hour)
	cfg.Refresh = false
	cfg.Local = true
	local := cacheClient(t, cfg, 200, body, nil)
	if _, e = local.Daily(ctx, chiyoda); e == nil {
		t.Fatal("expired local cache silently returned")
	}
	if local.Metrics().HTTPRequests != 0 {
		t.Fatal("local issued HTTP")
	}
	cfg.AllowStale = true
	stale := cacheClient(t, cfg, 200, body, nil)
	sr, e := stale.Daily(ctx, chiyoda)
	if e != nil {
		t.Fatal(e)
	}
	if !sr.Source.CacheStale || !sr.Source.SourceStale || sr.Source.Freshness != "stale" || !sr.Source.FromCache {
		t.Fatalf("explicit stale result %+v", sr.Source)
	}
	cfg.Local = false
	cfg.AllowStale = false
	broken := cacheClient(t, cfg, 0, "", errors.New("transport failed"))
	if _, e = broken.Daily(ctx, chiyoda); e == nil {
		t.Fatal("transport failed but stale fallback silent")
	}
	if broken.Metrics().HTTPRequests != 1 {
		t.Fatal("failed HTTP attempt not counted")
	}
	cfg.AllowStale = true
	fallback := cacheClient(t, cfg, 0, "", errors.New("transport failed"))
	fr, e := fallback.Daily(ctx, chiyoda)
	if e != nil {
		t.Fatal(e)
	}
	if !fr.Source.CacheStale || fallback.Metrics().HTTPRequests != 1 || fallback.Metrics().CacheHits != 1 {
		t.Fatal("explicit fallback missing stale/metrics evidence")
	}
}

func TestFutureCacheAndEncodedBoundary(t *testing.T) {
	now := snapshotNow
	c := cacheClient(t, Config{CacheDir: t.TempDir(), Local: true, AllowStale: true, Now: func() time.Time { return now }}, 200, "", nil)
	if e := c.writeCache(cacheEntry{URL: chiyoda, FetchedAt: now.Add(time.Hour), Body: "future"}); e != nil {
		t.Fatal(e)
	}
	if _, _, e := c.fetch(context.Background(), chiyoda, forecastTTL); e == nil {
		t.Fatal("future-dated local cache accepted despite invalid chronology")
	}
	// Worst-case JSON escape growth is bounded independently of raw HTML size.
	body := strings.Repeat("\x01", maxBodyBytes-10)
	if e := c.writeCache(cacheEntry{URL: chiyoda, FetchedAt: now, Body: body}); e != nil {
		t.Fatal(e)
	}
	entry, ok := c.readCache(chiyoda)
	if !ok || len(entry.Body) != len(body) {
		t.Fatal("valid near-limit escaped cache rejected")
	}
}

func TestHTTPDenialRateLimitAndBounds(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
		rate   bool
	}{{"denied", 403, "CloudFront denied", false}, {"notfound", 404, "missing product", false}, {"throttled", 429, "too many requests", true}, {"oversized", 200, strings.Repeat("x", maxBodyBytes+1), false}} {
		t.Run(test.name, func(t *testing.T) {
			c := cacheClient(t, Config{}, test.status, test.body, nil)
			_, _, e := c.fetch(context.Background(), chiyoda, forecastTTL)
			if e == nil {
				t.Fatal("HTTP failure silently successful")
			}
			var limited *RateLimitError
			if errors.As(e, &limited) != test.rate {
				t.Fatalf("typed429 %T %v", e, e)
			}
			if c.Metrics().HTTPRequests != 1 {
				t.Fatal("HTTP request not counted")
			}
			if test.status == 403 && !strings.Contains(e.Error(), "HTTP 403") {
				t.Fatal("denial status erased")
			}
		})
	}
	for _, cfg := range []Config{{}, {RateLimit: 100}, {RateLimit: 0.25}} {
		c := NewClient(cfg)
		if c.limiter.Rate() > 1 || c.limiter.Rate() <= 0 {
			t.Fatal("default ceiling violated")
		}
		if cfg.RateLimit == 0.25 && c.limiter.Rate() != 0.25 {
			t.Fatal("lower politeness rate ignored")
		}
	}
}

func TestSameHostRedirectPolicyAndAttemptMetrics(t *testing.T) {
	for _, test := range []struct {
		name         string
		location     string
		wantRequests int
		wantErr      bool
	}{{"samehost", chiyoda + "10days.html", 2, false}, {"otherhost", "https://example.com/", 1, true}, {"otherpath", "https://tenki.jp/docs/rule/", 1, true}} {
		t.Run(test.name, func(t *testing.T) {
			c := NewClient(Config{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() == chiyoda {
					return &http.Response{StatusCode: 302, Header: http.Header{"Location": {test.location}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
				}
				return response(r, 200, "<html>source</html>"), nil
			})})
			c.limiter = nil
			_, _, e := c.fetch(context.Background(), chiyoda, forecastTTL)
			if (e != nil) != test.wantErr {
				t.Fatalf("redirect policy error=%v", e)
			}
			if c.Metrics().HTTPRequests != test.wantRequests {
				t.Fatalf("redirect requests=%d want %d", c.Metrics().HTTPRequests, test.wantRequests)
			}
		})
	}
}

func TestLocalHTTPServerCacheAnd429(t *testing.T) {
	// The production URL validator remains strict. Only the injected test
	// transport routes approved tenki.jp requests to this local server.
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if strings.HasSuffix(r.URL.Path, "1hour.html") {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(429)
			_, _ = io.WriteString(w, "throttled")
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, fixture(t, "daily-20260927.html"))
	}))
	defer server.Close()
	endpoint, _ := url.Parse(server.URL)
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		clone := r.Clone(r.Context())
		u := *r.URL
		u.Scheme, u.Host = endpoint.Scheme, endpoint.Host
		clone.URL = &u
		return http.DefaultTransport.RoundTrip(clone)
	})
	c := NewClient(Config{CacheDir: t.TempDir(), Now: func() time.Time { return snapshotNow }, Transport: transport})
	c.limiter = nil
	if _, e := c.Daily(context.Background(), chiyoda); e != nil {
		t.Fatal(e)
	}
	if _, e := c.Daily(context.Background(), chiyoda); e != nil {
		t.Fatal(e)
	}
	_, e := c.Hourly(context.Background(), chiyoda)
	var rate *RateLimitError
	if !errors.As(e, &rate) || rate.RetryAfter != 2*time.Second {
		t.Fatalf("local429 typed evidence %v", e)
	}
	if requests != 2 || c.Metrics().HTTPRequests != 2 || c.Metrics().CacheHits != 1 {
		t.Fatalf("local server/metrics requests=%d metrics=%+v", requests, c.Metrics())
	}
}
