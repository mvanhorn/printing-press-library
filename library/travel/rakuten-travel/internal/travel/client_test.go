package travel

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/cliutil"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Header: http.Header{"Content-Type": {"text/html; charset=utf-8"}}, Body: io.NopCloser(strings.NewReader(body))}
}
func clientFor(t *testing.T, handler transportFunc, cfg Config) *Client {
	t.Helper()
	cfg.HTTPClient = &http.Client{Transport: handler}
	cfg.MinInterval = -1
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC) }
	}
	c, e := NewClient(cfg)
	if e != nil {
		t.Fatal(e)
	}
	return c
}

const areaBody = `<html><a href="/yado/tokyo/map.html">東京都</a></html>`

func TestQueryValidationBeforeIO(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*OfferQuery)
	}{{"bad ID", func(q *OfferQuery) { q.HotelID = "../../etc" }}, {"invalid calendar date", func(q *OfferQuery) { q.Checkin = "2026-02-30" }}, {"equal dates", func(q *OfferQuery) { q.Checkout = q.Checkin }}, {"reversed dates", func(q *OfferQuery) { q.Checkout = "2026-11-07" }}, {"29 nights", func(q *OfferQuery) { q.Checkout = "2026-12-07" }}, {"past JST day", func(q *OfferQuery) { q.Checkin = "2026-09-30"; q.Checkout = "2026-10-01" }}, {"far future", func(q *OfferQuery) { q.Checkin = "2027-12-01"; q.Checkout = "2027-12-02" }}, {"no adult", func(q *OfferQuery) { q.AdultsPerRoom = 0 }}, {"too many rooms", func(q *OfferQuery) { q.Rooms = 11 }}, {"negative child", func(q *OfferQuery) { q.Children.InfantNone = -1 }}, {"large child", func(q *OfferQuery) { q.Children.Upper = 11 }}, {"negative offset", func(q *OfferQuery) { q.Offset = -1 }}, {"large limit", func(q *OfferQuery) { q.Limit = 101 }}, {"large page", func(q *OfferQuery) { q.Page = 101 }}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "must-not-exist")
			calls := 0
			c := clientFor(t, func(*http.Request) (*http.Response, error) { calls++; return nil, errors.New("unexpected I/O") }, Config{CacheDir: dir})
			q := sampleQuery()
			tc.mutate(&q)
			_, e := c.Offers(context.Background(), q)
			assertKind(t, e, "unsupported_query")
			if calls != 0 {
				t.Fatal("invalid query reached transport")
			}
			if _, e := os.Stat(dir); !os.IsNotExist(e) {
				t.Fatal("invalid query touched cache")
			}
		})
	}
	for _, q := range []HotelQuery{{}, {Query: "x", Area: "tokyo/E"}, {Area: "../etc"}, {Query: strings.Repeat("x", 101)}, {Query: "x", Offset: -1}} {
		if e := q.Validate(); e == nil {
			t.Fatalf("invalid hotel query accepted %#v", q)
		}
	}
	for _, q := range []HotelQuery{{Query: "品川"}, {Area: "tokyo/E", Page: 2}} {
		if e := q.Validate(); e != nil {
			t.Fatal(e)
		}
	}
	for _, id := range []string{"51870", "001"} {
		if ValidateHotelID(id) != nil {
			t.Fatal(id)
		}
	}
	for _, id := range []string{"", "x", "1/2"} {
		if ValidateHotelID(id) == nil {
			t.Fatal(id)
		}
	}
	for _, p := range []string{"", "tokyo"} {
		if ValidateAreaParent(p) != nil {
			t.Fatal(p)
		}
	}
	for _, p := range []string{"../tokyo", "tokyo/E", "TOKYO"} {
		if ValidateAreaParent(p) == nil {
			t.Fatal(p)
		}
	}
	q := sampleQuery()
	q.Checkin = time.Now().In(jst).AddDate(0, 0, 1).Format("2006-01-02")
	q.Checkout = time.Now().In(jst).AddDate(0, 0, 2).Format("2006-01-02")
	if q.Validate() != nil {
		t.Fatal("valid explicit future dates rejected")
	}
}
func TestClientConfigBounds(t *testing.T) {
	for _, cfg := range []Config{{InventoryTTL: 61 * time.Second}, {InventoryTTL: -time.Second}, {MaxRequests: 101}, {MaxRequests: -1}, {Timeout: -time.Second}, {CacheMaxEntries: 257}, {CacheMaxBytes: 65 << 20}} {
		if _, e := NewClient(cfg); e == nil {
			t.Fatalf("bad config accepted %#v", cfg)
		}
	}
	c, e := NewClient(Config{MinInterval: time.Nanosecond})
	if e != nil || c.config.MinInterval < time.Second || c.config.MaxRequests != 12 || c.config.InventoryTTL != 0 {
		t.Fatalf("defaults %#v %v", c, e)
	}
}
func TestNetworkErrorClassificationAndAttemptBudget(t *testing.T) {
	cases := []struct {
		name       string
		code       int
		body, kind string
		attempts   int
	}{{"404", 404, "not found", "not_found", 1}, {"401", 401, "login", "unauthorized", 1}, {"403", 403, "denied", "forbidden", 1}, {"429", 429, "throttled", "rate_limited", 3}, {"503", 503, "unavailable", "upstream_error", 3}, {"200 challenge", 200, "<title>Just a moment</title>", "challenge", 1}, {"empty document", 200, "", "parse_error", 1}, {"unknown layout", 200, "<html>error</html>", "parse_error", 1}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			c := clientFor(t, func(*http.Request) (*http.Response, error) { calls++; return response(tc.code, tc.body), nil }, Config{})
			_, e := c.Areas(context.Background(), "")
			assertKind(t, e, tc.kind)
			if calls != tc.attempts || c.Stats().Requests != calls {
				t.Fatalf("attempt count %d %#v", calls, c.Stats())
			}
			if tc.code == 429 {
				var rate *cliutil.RateLimitError
				if !errors.As(e, &rate) {
					t.Fatal("429 lost typed rate error")
				}
			}
		})
	}
	t.Run("transport errors retry twice", func(t *testing.T) {
		calls := 0
		c := clientFor(t, func(*http.Request) (*http.Response, error) { calls++; return nil, errors.New("connection reset") }, Config{})
		_, e := c.Areas(context.Background(), "")
		assertKind(t, e, "network_error")
		if calls != 3 || c.Stats().Retries != 2 {
			t.Fatalf("retry stats %#v", c.Stats())
		}
	})
	t.Run("attempt hard cap", func(t *testing.T) {
		calls := 0
		c := clientFor(t, func(*http.Request) (*http.Response, error) { calls++; return response(503, "busy"), nil }, Config{MaxRequests: 2})
		_, e := c.Areas(context.Background(), "")
		assertKind(t, e, "request_budget")
		if calls != 2 {
			t.Fatalf("budget issued %d attempts", calls)
		}
	})
	t.Run("context timeout", func(t *testing.T) {
		c := clientFor(t, func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() }, Config{Timeout: 15 * time.Millisecond})
		_, e := c.Areas(context.Background(), "")
		assertKind(t, e, "timeout")
		if !errors.Is(e, context.DeadlineExceeded) || c.Stats().Requests != 1 {
			t.Fatal(e, c.Stats())
		}
	})
	t.Run("source retry-after never shortened", func(t *testing.T) {
		calls := 0
		c := clientFor(t, func(*http.Request) (*http.Response, error) {
			calls++
			r := response(429, "busy")
			r.Header.Set("Retry-After", "120")
			return r, nil
		}, Config{Timeout: 20 * time.Millisecond})
		_, e := c.Areas(context.Background(), "")
		assertKind(t, e, "rate_limited")
		var rate *cliutil.RateLimitError
		if !errors.As(e, &rate) || rate.RetryAfter != 120*time.Second || calls != 1 {
			t.Fatalf("source delay lost %v %d", e, calls)
		}
	})
	t.Run("body cap", func(t *testing.T) {
		c := clientFor(t, func(*http.Request) (*http.Response, error) {
			return response(200, strings.Repeat("x", int(maxBodyBytes)+1)), nil
		}, Config{})
		_, e := c.Areas(context.Background(), "")
		assertKind(t, e, "body_too_large")
		if c.Stats().Bytes != maxBodyBytes+1 {
			t.Fatal("incorrect byte count")
		}
	})
}
func TestRedirectRestrictionsAndNoCredentials(t *testing.T) {
	for _, target := range []string{"http://travel.rakuten.co.jp/yado/japan.html", "https://evil.example/", "https://user:pass@travel.rakuten.co.jp/", "https://travel.rakuten.co.jp:444/"} {
		t.Run(target, func(t *testing.T) {
			calls := 0
			c := clientFor(t, func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != "GET" || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
					t.Fatal("credentials/mutation")
				}
				x := response(302, "")
				x.Header.Set("Location", target)
				return x, nil
			}, Config{})
			_, e := c.Areas(context.Background(), "")
			assertKind(t, e, "unsafe_redirect")
			if calls != 1 {
				t.Fatal(calls)
			}
		})
	}
	calls := 0
	c := clientFor(t, func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			x := response(302, "")
			x.Header.Set("Location", "https://travel.rakuten.co.jp/yado/japan.html?public=1")
			return x, nil
		}
		return response(200, areaBody), nil
	}, Config{MaxRequests: 1})
	_, e := c.Areas(context.Background(), "")
	assertKind(t, e, "request_budget")
	if calls != 1 {
		t.Fatal("redirect escaped attempt cap")
	}
}
func TestCharsetAndWireParty(t *testing.T) {
	t.Run("Windows-31j", func(t *testing.T) {
		s := `<html><input name="f_query" value="品川"><div id="result"><div class="hotelBox"><h2><a href="https://travel.rakuten.co.jp/HOTEL/1/1.html">品川ホテル</a></h2></div></div></html>`
		encoded, _, e := transform.String(japanese.ShiftJIS.NewEncoder(), s)
		if e != nil {
			t.Fatal(e)
		}
		c := clientFor(t, func(r *http.Request) (*http.Response, error) {
			if r.URL.Query().Get("f_query") != "品川" {
				t.Fatal(r.URL)
			}
			x := response(200, encoded)
			x.Header.Set("Content-Type", "text/html; charset=Windows-31j")
			return x, nil
		}, Config{})
		r, e := c.SearchHotels(context.Background(), HotelQuery{Query: "品川"})
		if e != nil || len(r.Hotels) != 1 || *r.Hotels[0].Name != "品川ホテル" {
			t.Fatalf("decoded %#v %v", r, e)
		}
	})
	t.Run("six counts remain per room", func(t *testing.T) {
		q := sampleQuery()
		q.Rooms = 2
		q.Children = Children{1, 2, 3, 4, 5, 6}
		c := clientFor(t, func(r *http.Request) (*http.Response, error) {
			v := r.URL.Query()
			if v.Get("f_heya_su") != "2" || v.Get("f_otona_su") != "2" || v.Get("f_s1") != "1" || v.Get("f_y4") != "6" {
				t.Fatalf("party was multiplied %s", r.URL)
			}
			return response(200, string(offerFixture(q, []fixtureRoom{{plan: "1", room: "r-", total: 20720}}, false))), nil
		}, Config{})
		r, e := c.Offers(context.Background(), q)
		if e != nil || r.Offers[0].Price.PerRoomStayJPY != 20720 {
			t.Fatal(e, r)
		}
	})
	t.Run("area page2 host/path", func(t *testing.T) {
		c := clientFor(t, func(r *http.Request) (*http.Response, error) {
			if r.URL.String() != "https://search.travel.rakuten.co.jp/ds/yado/tokyo/E-p2" {
				t.Fatal(r.URL)
			}
			return response(200, `<h2><a href="https://travel.rakuten.co.jp/HOTEL/1/1.html">Hotel</a></h2>`), nil
		}, Config{})
		_, e := c.SearchHotels(context.Background(), HotelQuery{Area: "tokyo/E", Page: 2})
		if e != nil {
			t.Fatal(e)
		}
	})
}
func TestPersistentCacheBoundsFreshnessAndBypass(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	calls := 0
	handler := transportFunc(func(*http.Request) (*http.Response, error) { calls++; return response(200, areaBody), nil })
	cfg := Config{CacheDir: dir, Now: func() time.Time { return now }}
	c := clientFor(t, handler, cfg)
	r, e := c.Areas(context.Background(), "")
	if e != nil || r.Source.CacheState != "miss" {
		t.Fatal(e, r)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatal(entries)
	}
	info, _ := entries[0].Info()
	if info.Mode().Perm() != 0600 {
		t.Fatal("nonprivate entry")
	}
	info, _ = os.Stat(dir)
	if info.Mode().Perm() != 0700 {
		t.Fatal("nonprivate directory")
	}
	now = now.Add(time.Hour)
	c = clientFor(t, handler, cfg)
	r, e = c.Areas(context.Background(), "")
	if e != nil || r.Source.CacheState != "hit" || r.Source.CacheAgeSeconds != 3600 || calls != 1 || c.Stats().Requests != 0 || c.Stats().CacheHits != 1 {
		t.Fatal(e, r, calls, c.Stats())
	}
	old, _ := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	cfg.NoCache = true
	c = clientFor(t, handler, cfg)
	_, e = c.Areas(context.Background(), "")
	after, _ := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if e != nil || calls != 2 || string(old) != string(after) {
		t.Fatal("no-cache read/wrote persistent data", e)
	}
	cfg.NoCache = false
	cfg.Refresh = true
	c = clientFor(t, handler, cfg)
	r, e = c.Areas(context.Background(), "")
	if e != nil || r.Source.CacheState != "refresh" || calls != 3 {
		t.Fatal("refresh did not replace", e, r)
	}
	cfg.Refresh = false
	now = now.Add(25 * time.Hour)
	c = clientFor(t, func(*http.Request) (*http.Response, error) { return response(503, "unavailable"), nil }, cfg)
	_, e = c.Areas(context.Background(), "")
	assertKind(t, e, "upstream_error")
	t.Run("bounded eviction", func(t *testing.T) {
		cfg := Config{CacheDir: t.TempDir(), CacheMaxEntries: 2, CacheMaxBytes: 1300}
		c := clientFor(t, handler, cfg)
		for i := 0; i < 8; i++ {
			d := doc([]byte(areaBody), "https://travel.rakuten.co.jp/yado/"+integer(i)+".html")
			if e := c.save(d, false); e != nil {
				t.Fatal(e)
			}
		}
		entries, _ := os.ReadDir(cfg.CacheDir)
		var bytes int64
		for _, e := range entries {
			info, _ := e.Info()
			bytes += info.Size()
		}
		if len(entries) > 2 || bytes > 1300 {
			t.Fatalf("cache unbounded %d %d", len(entries), bytes)
		}
	})
}
func TestInventoryCacheAndInvocationSnapshot(t *testing.T) {
	q := sampleQuery()
	q.Limit = 1
	body := string(offerFixture(q, []fixtureRoom{{plan: "1", room: "r-", total: 20720}, {plan: "2", room: "s-", total: 22000}}, false))
	calls := 0
	handler := transportFunc(func(*http.Request) (*http.Response, error) { calls++; return response(200, body), nil })
	dir := t.TempDir()
	cfg := Config{CacheDir: dir, NoCache: true, Refresh: true}
	c := clientFor(t, handler, cfg)
	_, e := c.Offers(context.Background(), q)
	if e != nil {
		t.Fatal(e)
	}
	q.Offset = 1
	r, e := c.Offers(context.Background(), q)
	if e != nil || r.Offers[0].PlanID != "2" || r.Source.CacheState != "invocation_snapshot" || calls != 1 || c.Stats().Requests != 1 {
		t.Fatal(e, r, calls)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("no-cache wrote inventory")
	}
	q.Offset = 0
	cfg.NoCache = false
	cfg.Refresh = false
	c = clientFor(t, handler, cfg)
	_, e = c.Offers(context.Background(), q)
	if e != nil {
		t.Fatal(e)
	}
	entries, _ = os.ReadDir(dir)
	if len(entries) != 0 || calls != 2 {
		t.Fatal("default inventory cached")
	}
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	cfg.InventoryTTL = 30 * time.Second
	cfg.Now = func() time.Time { return now }
	c = clientFor(t, handler, cfg)
	_, e = c.Offers(context.Background(), q)
	if e != nil {
		t.Fatal(e)
	}
	now = now.Add(15 * time.Second)
	c = clientFor(t, handler, cfg)
	r, e = c.Offers(context.Background(), q)
	if e != nil || r.Source.CacheState != "hit" || r.Source.CacheTTLSeconds != 30 || calls != 3 {
		t.Fatal(e, r, calls)
	}
	now = now.Add(16 * time.Second)
	c = clientFor(t, func(*http.Request) (*http.Response, error) { return response(503, "busy"), nil }, cfg)
	_, e = c.Offers(context.Background(), q)
	assertKind(t, e, "upstream_error")
}
func TestProductionPacingFloor(t *testing.T) {
	cfg := Config{HTTPClient: &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) { return response(200, areaBody), nil })}, MinInterval: time.Second}
	c, e := NewClient(cfg)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 12; i++ {
		c.limiter.OnSuccess()
	}
	start := time.Now()
	for i := 0; i < 2; i++ {
		if _, e = c.Areas(context.Background(), ""); e != nil {
			t.Fatal(e)
		}
	}
	if time.Since(start) < time.Second {
		t.Fatal("adaptive limiter violated fixed pacing floor")
	}
}

func TestLargeNumericRetryAfterRemainsDeltaSeconds(t *testing.T) {
	for _, header := range []string{"1700000000", "9999999999999999999999999999999999999"} {
		t.Run(header, func(t *testing.T) {
			calls := 0
			c := clientFor(t, func(*http.Request) (*http.Response, error) {
				calls++
				r := response(429, "throttled")
				r.Header.Set("Retry-After", header)
				return r, nil
			}, Config{Timeout: time.Second})
			_, e := c.Areas(context.Background(), "")
			assertKind(t, e, "rate_limited")
			var rate *cliutil.RateLimitError
			if !errors.As(e, &rate) || rate.RetryAfter < time.Hour || calls != 1 {
				t.Fatalf("numeric source delay was shortened %v, calls %d", e, calls)
			}
		})
	}
}
