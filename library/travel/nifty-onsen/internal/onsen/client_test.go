package onsen

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/nifty-onsen/internal/cliutil"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(r *http.Request, status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}
}
func TestCacheSources(t *testing.T) {
	dir := t.TempDir()
	calls := 0
	c := New(Options{CacheDir: dir, Rate: 0, MaxAge: time.Hour})
	c.HTTP.Transport = transportFunc(func(r *http.Request) (*http.Response, error) { calls++; return response(r, 200, searchFixture), nil })
	o := SearchOptions{Region: "tokyo", Page: 1}
	for _, want := range []string{"miss", "hit"} {
		v, p, e := c.Search(context.Background(), o)
		if e != nil || p.Cache != want || len(v.Items) != 1 {
			t.Fatalf("%+v %v", p, e)
		}
	}
	if calls != 1 {
		t.Fatal(calls)
	}
	c.options.DataSource = "local"
	_, p, e := c.Search(context.Background(), o)
	if e != nil || p.Cache != "offline" {
		t.Fatal(p, e)
	}
	o.Page = 2
	if _, _, e = c.Search(context.Background(), o); e == nil {
		t.Fatal("missing cache accepted")
	}
	o.Page = 1
	c.options.NoCache = true
	if _, _, e = c.Search(context.Background(), o); e == nil {
		t.Fatal("no-cache local accepted")
	}
	c.options.NoCache = false
	c.options.DataSource = "live"
	if _, p, e = c.Search(context.Background(), o); e != nil || p.Cache != "miss" || calls != 2 {
		t.Fatal(p, e, calls)
	}
}
func TestStaleFallbackAndThrottle(t *testing.T) {
	for _, tc := range []struct {
		status   int
		fallback bool
	}{{503, true}, {429, false}} {
		c := New(Options{CacheDir: t.TempDir(), MaxAge: time.Nanosecond})
		link, _ := SearchURL(SearchOptions{Page: 1})
		v, _ := ParseSearch([]byte(searchFixture), link, 1)
		b := mustJSON(v)
		if e := c.saveCache(link, cacheRecord{cacheVersion, time.Now().Add(-time.Hour), link, b}); e != nil {
			t.Fatal(e)
		}
		c.HTTP.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
			v := response(r, tc.status, "")
			if tc.status == 429 {
				v.Header.Set("Retry-After", "20")
			}
			return v, nil
		})
		_, p, e := c.Search(context.Background(), SearchOptions{Page: 1})
		if tc.fallback {
			if e != nil || p.Cache != "fallback" || !p.Stale || len(p.Warnings) != 1 {
				t.Fatal(p, e)
			}
		} else {
			var rate *cliutil.RateLimitError
			if !errors.As(e, &rate) {
				t.Fatalf("throttle not typed: %v", e)
			}
		}
	}
}
func TestCacheBoundsAndInvalid(t *testing.T) {
	c := New(Options{CacheDir: t.TempDir()})
	for i := 0; i < 130; i++ {
		key := string(rune(i))
		if e := c.saveCache(key, cacheRecord{cacheVersion, time.Now(), Origin, []byte(`[]`)}); e != nil {
			t.Fatal(e)
		}
	}
	files, _ := os.ReadDir(c.options.CacheDir)
	if len(files) > 128 {
		t.Fatal(len(files))
	}
	p := c.cachePath("bad")
	if e := os.WriteFile(p, []byte("broken"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, ok := c.readCache("bad"); ok {
		t.Fatal("bad cache accepted")
	}
}
func TestRequestBounds(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
	}{{403, ""}, {200, strings.Repeat("a", maxResponse+1)}} {
		c := New(Options{})
		c.HTTP.Transport = transportFunc(func(r *http.Request) (*http.Response, error) { return response(r, tc.status, tc.body), nil })
		if _, _, e := c.fetch(context.Background(), "GET", Origin+"/", false); e == nil {
			t.Fatal("invalid response accepted")
		}
	}
}

func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }
