package ikyu

import (
	"context"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/ikyu/internal/cliutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testClient(t *testing.T, h http.HandlerFunc, o Options) *Client {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	o.BaseURL = s.URL
	o.HTTPClient = s.Client()
	c, e := NewClient(o)
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func TestNewClientBounds(t *testing.T) {
	for _, o := range []Options{{MaxRequests: 21}, {Concurrency: 3}, {Retries: 3}, {MaxResponseBytes: 9 << 20}, {RequestTimeout: 16 * time.Second}, {BaseURL: "https://elsewhere.example"}} {
		if _, e := NewClient(o); e == nil {
			t.Fatalf("accepted bounds %+v", o)
		}
	}
}
func TestTransportRequestBudgetRedirectAndBytes(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/a" {
			http.Redirect(w, r, "/b", 302)
			return
		}
		fmt.Fprint(w, "body")
	}, Options{MaxRequests: 2})
	body, _, e := c.fetch(context.Background(), "GET", "/a", nil, staticTTL)
	if e != nil || string(body) != "body" {
		t.Fatalf("response %q %v", body, e)
	}
	if c.Stats().Requests != 2 || c.Stats().ResponseBytes < 4 {
		t.Fatalf("metrics %+v", c.Stats())
	}
	if _, _, e = c.fetch(context.Background(), "GET", "/b", nil, staticTTL); e == nil {
		t.Fatal("budget did not stop request")
	}
}
func TestTransportBodyBoundAnd403NoRetry(t *testing.T) {
	for _, status := range []int{200, 403} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				fmt.Fprint(w, "01234567890123456789012345")
			}, Options{MaxResponseBytes: 16})
			if _, _, e := c.fetch(context.Background(), "GET", "/", nil, staticTTL); e == nil {
				t.Fatal("unexpected success")
			}
			if c.Stats().Requests != 1 {
				t.Fatalf("unsafe retry %+v", c.Stats())
			}
		})
	}
}
func TestTransportRateLimitTyped(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(429) }, Options{MaxRequests: 3})
	_, _, err := c.fetch(context.Background(), "GET", "/", nil, staticTTL)
	var rate *cliutil.RateLimitError
	if !errors.As(err, &rate) {
		t.Fatalf("typed rate limit lost: %T %v", err, err)
	}
	if c.Stats().Requests != 3 {
		t.Fatalf("retries %+v", c.Stats())
	}
}
func TestCacheFreshStaleRefreshAndFuture(t *testing.T) {
	now := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	status := 200
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status); fmt.Fprint(w, "public") }, Options{CacheDir: t.TempDir(), AllowStale: true, Now: func() time.Time { return now }})
	_, first, e := c.fetch(context.Background(), "GET", "/", nil, availabilityTTL)
	if e != nil {
		t.Fatal(e)
	}
	_, warm, e := c.fetch(context.Background(), "GET", "/", nil, availabilityTTL)
	if e != nil || warm.Source != "cache" || c.Stats().Requests != 1 {
		t.Fatalf("warm cache %+v %v", warm, e)
	}
	now = now.Add(6 * time.Minute)
	status = 403
	_, stale, e := c.fetch(context.Background(), "GET", "/", nil, availabilityTTL)
	if e != nil || !stale.Stale || stale.SourceError == nil {
		t.Fatalf("stale provenance %+v %v", stale, e)
	}
	c.opts.Refresh = true
	if _, _, e = c.fetch(context.Background(), "GET", "/", nil, availabilityTTL); e == nil {
		t.Fatal("refresh returned stale")
	}
	c.opts.Refresh = false
	c.writeCache("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", cacheRecord{Body: []byte("future"), Version: SchemaVersion, FetchedAt: first.FetchedAt.Add(48 * time.Hour)})
	if _, ok := c.readCache("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); ok {
		t.Fatal("future cache accepted")
	}
}
func TestCacheBoundsPreserveUnrelatedFiles(t *testing.T) {
	dir := t.TempDir()
	c, e := NewClient(Options{CacheDir: dir})
	if e != nil {
		t.Fatal(e)
	}
	unrelated := filepath.Join(dir, "notes.json")
	if e = os.WriteFile(unrelated, []byte("keep"), 0600); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 257; i++ {
		c.writeCache(fmt.Sprintf("%064x", i), cacheRecord{FetchedAt: time.Now(), Version: SchemaVersion, Body: []byte("small")})
	}
	files, _ := os.ReadDir(dir)
	count := 0
	for _, f := range files {
		if cacheKeyFile(f.Name()) {
			count++
		}
	}
	if count != 256 {
		t.Fatalf("cache entries %d", count)
	}
	if _, e = os.Stat(unrelated); e != nil {
		t.Fatal("unrelated file removed")
	}
}
func TestGraphQLErrorsAreNotEmptySuccess(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":null,"errors":[{"message":"field removed"}]}`)
	}, Options{})
	var out any
	if _, e := c.query(context.Background(), "StayProperty", propertyQuery, map[string]string{"id": "00002889"}, &out, staticTTL); e == nil {
		t.Fatal("GraphQL error became success")
	}
}
