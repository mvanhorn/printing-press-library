package omakase

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/mvanhorn/printing-press-library/library/travel/omakase/internal/cliutil"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(strings.NewReader(body))}
}
func testClient(t *testing.T, f roundTrip) *Client {
	t.Helper()
	c := New(t.TempDir())
	c.Pace = 0
	c.HTTP = &http.Client{Transport: f}
	return c
}
func TestClientBoundedCacheFreshness(t *testing.T) {
	calls := 0
	c := testClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != "GET" || r.URL.Host != "omakase.in" || r.Header.Get("Accept") == "" {
			t.Fatal("request contract")
		}
		return response(200, detailHTML), nil
	})
	for i := 0; i < 2; i++ {
		d, e := c.Restaurant(context.Background(), "hc778124", "en")
		if e != nil || d.Evidence[0].Cached != (i == 1) {
			t.Fatalf("%+v %v", d, e)
		}
	}
	if calls != 1 {
		t.Fatal(calls)
	}
	c.MaxAge = time.Nanosecond
	c.Offline = true
	d, e := c.Restaurant(context.Background(), "hc778124", "en")
	if e != nil || !d.Evidence[0].Stale || calls != 1 {
		t.Fatal("offline freshness")
	}
	c.Offline = false
	if _, e = c.Restaurant(context.Background(), "hc778124", "en"); e != nil || calls != 2 {
		t.Fatal("stale online did not refresh")
	}
	for _, p := range []string{"https://elsewhere.test/en/r", "https://omakase.in/users/sign_in", "https://omakase.in/api/seat", "https://omakase.in/r/hc778124/reservations"} {
		if _, e = c.fetch(context.Background(), p); e == nil {
			t.Fatal(p)
		}
	}
}
func TestFailureStatesAndRetryBound(t *testing.T) {
	for _, status := range []int{403, 404, 429, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			c := testClient(t, func(r *http.Request) (*http.Response, error) { calls++; return response(status, "error"), nil })
			_, e := c.Restaurant(context.Background(), "hc778124", "en")
			if e == nil {
				t.Fatal("swallowed status")
			}
			want := 1
			if status == 429 || status >= 500 {
				want = 2
			}
			if calls != want {
				t.Fatal(calls)
			}
			if status == 429 {
				var rate *cliutil.RateLimitError
				if !errors.As(e, &rate) {
					t.Fatal("untyped throttling")
				}
			}
		})
	}
	c := testClient(t, func(r *http.Request) (*http.Response, error) { return response(200, "<h1>Log in</h1>"), nil })
	if _, e := c.Restaurant(context.Background(), "hc778124", "en"); e == nil {
		t.Fatal("cached malformed response")
	}
	c.Offline = true
	if _, e := c.Restaurant(context.Background(), "hc778124", "en"); e == nil {
		t.Fatal("malformed response cache leaked")
	}
}
func TestInventoryRefreshAtomicCoverage(t *testing.T) {
	c := testClient(t, func(r *http.Request) (*http.Response, error) { return response(200, pageHTML), nil })
	path := filepath.Join(t.TempDir(), "inventory.json")
	in, e := c.RefreshInventory(context.Background(), path, "en", 1)
	if e != nil || in.Complete || len(in.Results) != 1 || in.Pages != 1 {
		t.Fatalf("%+v %v", in, e)
	}
	old, _ := os.ReadFile(path)
	c.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) { return response(403, "error"), nil })
	if _, e = c.RefreshInventory(context.Background(), path, "en", 2); e == nil {
		t.Fatal("failed refresh accepted")
	}
	after, _ := os.ReadFile(path)
	if string(old) != string(after) {
		t.Fatal("failed refresh replaced snapshot")
	}
	if _, e = ReadInventory(path); e != nil {
		t.Fatal(e)
	}
}
func TestCataloguePremiumAndSizeLimits(t *testing.T) {
	c := testClient(t, func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "premium") {
			return response(200, `<h1>Premium</h1>4,980 Yen(tax inc.)/mo`), nil
		}
		return response(200, pageHTML), nil
	})
	p, _, e := c.Catalogue(context.Background(), "en", 2, "Sugita", "kanto", "sushi")
	if e != nil || len(p.Results) != 1 {
		t.Fatal(e)
	}
	if _, _, e = c.Premium(context.Background()); e != nil {
		t.Fatal(e)
	}
	if _, _, e = c.Catalogue(context.Background(), "en", 26, "", "", ""); e == nil {
		t.Fatal("unbounded page")
	}
	c.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		return response(200, strings.Repeat("x", maxBody+1)), nil
	})
	c.NoCache = true
	if _, e = c.Restaurant(context.Background(), "hc778124", "en"); e == nil {
		t.Fatal("unbounded body")
	}
}

func TestOldParserCacheIsRejected(t *testing.T) {
	calls := 0
	c := testClient(t, func(r *http.Request) (*http.Response, error) { calls++; return response(200, detailHTML), nil })
	raw := Origin + "/en/r/hc778124"
	old := cacheEntry{Version: documentCacheVersion - 1, URL: raw, FetchedAt: time.Now().UTC(), Data: json.RawMessage(`{"id":"hc778124","name":"pre-fix normalized result"}`)}
	b, _ := json.Marshal(old)
	if e := atomicWrite(c.key(raw), b); e != nil {
		t.Fatal(e)
	}
	c.Offline = true
	if _, e := c.Restaurant(context.Background(), "hc778124", "en"); e == nil {
		t.Fatal("accepted old parser cache offline")
	}
	c.Offline = false
	d, e := c.Restaurant(context.Background(), "hc778124", "en")
	if e != nil || calls != 1 || d.Name != "Test Sushi" || d.Evidence[0].Cached {
		t.Fatalf("%+v %v requests=%d", d, e, calls)
	}
	fresh, _ := os.ReadFile(c.key(raw))
	var ce cacheEntry
	_ = json.Unmarshal(fresh, &ce)
	if ce.Version != documentCacheVersion {
		t.Fatal("wrote legacy parser cache")
	}
}

func TestZeroMaxAgeReusesOldCacheAndHonorsRefresh(t *testing.T) {
	calls := 0
	c := testClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		return response(200, detailHTML), nil
	})
	raw := Origin + "/en/r/hc778124"
	entry := cacheEntry{Version: documentCacheVersion, URL: raw, FetchedAt: time.Now().Add(-48 * time.Hour), Data: json.RawMessage(`{"id":"hc778124","name":"cached source"}`)}
	b, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(c.key(raw), b); err != nil {
		t.Fatal(err)
	}
	c.MaxAge = 0
	d, err := c.Restaurant(context.Background(), "hc778124", "en")
	if err != nil || calls != 0 || d.Name != "cached source" || !d.Evidence[0].Cached || d.Evidence[0].Stale {
		t.Fatalf("zero age limit should reuse old cache: %+v err=%v requests=%d", d, err, calls)
	}
	c.Refresh = true
	d, err = c.Restaurant(context.Background(), "hc778124", "en")
	if err != nil || calls != 1 || d.Name != "Test Sushi" || d.Evidence[0].Cached {
		t.Fatalf("explicit refresh should still fetch: %+v err=%v requests=%d", d, err, calls)
	}
}
