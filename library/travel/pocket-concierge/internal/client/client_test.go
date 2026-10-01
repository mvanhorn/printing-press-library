package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/mvanhorn/printing-press-library/library/travel/pocket-concierge/internal/cliutil"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}
}
func TestFailuresNeverBecomeEmptySuccess(t *testing.T) {
	for _, tc := range []struct {
		name, body   string
		status, code int
	}{{"HTML", "<html>login</html>", 200, 7}, {"null", "{\"data\":null}", 200, 7}, {"partial errors", "{\"data\":{\"areas\":[]},\"errors\":[{}]}", 200, 7}, {"access", "denied", 403, 4}, {"not found endpoint", "not found", 404, 7}, {"too large", strings.Repeat("x", maxBody+1), 200, 7}} {
		t.Run(tc.name, func(t *testing.T) {
			c := New("", false, true)
			c.HTTP.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { return response(tc.status, tc.body), nil })
			var out any
			e := c.ReadQuery(context.Background(), "en", `query Filters { areas { id name } cuisines { id name } }`, nil, time.Hour, &out)
			if e == nil || ExitCode(e) != tc.code {
				t.Fatalf("%v code %d", e, ExitCode(e))
			}
		})
	}
}
func TestCacheFreshRefreshAndReadOnlyBoundary(t *testing.T) {
	c := New(t.TempDir(), false, false)
	calls := 0
	c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != Endpoint || r.Method != "POST" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Fatal("unexpected destination/auth")
		}
		b, _ := io.ReadAll(r.Body)
		var j map[string]any
		if json.Unmarshal(b, &j) != nil || !strings.HasPrefix(j["query"].(string), "query ") {
			t.Fatal("mutation boundary")
		}
		return response(200, `{"data":{"areas":[],"cuisines":[]}}`), nil
	})
	for i := 0; i < 2; i++ {
		if _, e := readFilters(c); e != nil {
			t.Fatal(e)
		}
	}
	if calls != 1 || c.Metrics.CacheHits != 1 {
		t.Fatalf("calls %d hits %d", calls, c.Metrics.CacheHits)
	}
	c.Refresh = true
	if _, e := readFilters(c); e != nil {
		t.Fatal(e)
	}
	if calls != 2 {
		t.Fatal("refresh did not request")
	}
	var out any
	if e := c.ReadQuery(context.Background(), "en", "mutation Create { reservationCreate { id } }", nil, 0, &out); ExitCode(e) != 2 || calls != 2 {
		t.Fatal("mutation accepted")
	}
	c.Refresh = false
	for i := 0; i < 2; i++ {
		if e := c.ReadQuery(context.Background(), "ja", `query Filters { areas { id name } cuisines { id name } }`, nil, 0, &out); e != nil {
			t.Fatal(e)
		}
	}
	if calls != 4 {
		t.Fatal("live inventory was cached")
	}
}
func TestRateLimitAndTimeoutAreTyped(t *testing.T) {
	c := New("", false, true)
	calls := 0
	c.HTTP.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		r := response(429, "slow down")
		r.Header.Set("Retry-After", "100")
		return r, nil
	})
	var out any
	e := c.ReadQuery(context.Background(), "en", `query Filters { areas { id name } cuisines { id name } }`, nil, 0, &out)
	var rate *cliutil.RateLimitError
	if !errors.As(e, &rate) || ExitCode(e) != 5 || calls != 1 {
		t.Fatal(e, calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e = c.ReadQuery(ctx, "en", `query Filters { areas { id name } cuisines { id name } }`, nil, 0, &out)
	if ExitCode(e) != 6 {
		t.Fatal(e)
	}
}

func readFilters(c *Client) (any, error) {
	var out any
	e := c.ReadQuery(context.Background(), "ja", `query Filters { areas { id name } cuisines { id name } }`, nil, 24*time.Hour, &out)
	return out, e
}
func TestCachePruningNeverDeletesOtherFiles(t *testing.T) {
	dir := t.TempDir()
	other := filepath.Join(dir, "notes.json")
	if e := os.WriteFile(other, bytes.Repeat([]byte("x"), maxCacheBytes), 0600); e != nil {
		t.Fatal(e)
	}
	c := New(dir, false, false)
	c.writeCache(filepath.Join(dir, "pocket-v1-"+strings.Repeat("a", 64)+".json"), cacheEntry{time.Now(), json.RawMessage(`{}`)})
	if _, e := os.Stat(other); e != nil {
		t.Fatal("unrelated JSON deleted", e)
	}
	for _, name := range []string{"notes.json", "pocket-v1-foo.json", strings.Repeat("a", 64) + ".json"} {
		if ownedCacheName(name) {
			t.Fatal("claimed unrelated cache file", name)
		}
	}
}

func TestMalformedCacheCannotContaminateLiveFallback(t *testing.T) {
	c := New(t.TempDir(), false, false)
	calls := 0
	c.HTTP.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return response(200, `{"data":{"name":"cached name","count":1}}`), nil
		}
		return response(200, `{"data":{"count":2}}`), nil
	})
	var out struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	query := `query Filters { areas { id name } cuisines { id name } }`
	if err := c.ReadQuery(context.Background(), "en", query, nil, time.Hour, &out); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(c.CacheDir, "pocket-v1-*.json"))
	if err != nil || len(files) != 1 {
		t.Fatalf("cache files: %v, error: %v", files, err)
	}
	b, err := json.Marshal(cacheEntry{time.Now(), json.RawMessage(`{"name":"stale cached name","count":"wrong type"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(files[0], b, 0600); err != nil {
		t.Fatal(err)
	}
	out.Name, out.Count = "", 0
	if err := c.ReadQuery(context.Background(), "en", query, nil, time.Hour, &out); err != nil {
		t.Fatal(err)
	}
	if out.Name != "" || out.Count != 2 || calls != 2 || c.Metrics.CacheHits != 0 {
		t.Fatalf("live fallback contaminated: %+v, calls=%d, hits=%d", out, calls, c.Metrics.CacheHits)
	}
}
