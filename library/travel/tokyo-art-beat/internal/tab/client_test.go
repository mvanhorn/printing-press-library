package tab

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/mvanhorn/printing-press-library/library/travel/tokyo-art-beat/internal/cliutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixtureClient(t *testing.T, h http.HandlerFunc, opt Options) *Client {
	t.Helper()
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	if opt.CacheDir == "" {
		opt.CacheDir = t.TempDir()
	}
	c := NewClient(opt)
	c.base = server.URL
	c.limiter = nil
	return c
}

const emptyFeed = `{"total":0,"skip":0,"limit":2,"items":[]}`

func TestQueryCacheAndOffline(t *testing.T) {
	hits := 0
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) { hits++; w.Write([]byte(emptyFeed)) }, Options{})
	for _, tt := range []struct {
		fresh, offline bool
		hits           int
	}{{false, false, 1}, {false, false, 1}, {true, false, 2}, {false, true, 2}} {
		c.opt.Fresh = tt.fresh
		c.opt.Offline = tt.offline
		f, e := c.Query(context.Background(), url.Values{"limit": {"2"}})
		if e != nil || f.Total != 0 || hits != tt.hits {
			t.Fatalf("%+v %v hits=%d", f, e, hits)
		}
	}
	if c.Summary().Requests != 2 || c.Summary().CacheHits != 2 {
		t.Fatal(c.Summary())
	}
	c.opt.Fresh = false
	c.opt.Offline = true
	if _, e := c.Query(context.Background(), url.Values{"limit": {"3"}}); Classify(e).Code != "cache_miss" {
		t.Fatal(e)
	}
	for _, entry := range mustReadDir(t, c.opt.CacheDir) {
		p := filepath.Join(c.opt.CacheDir, entry.Name())
		b, _ := os.ReadFile(p)
		var rec cacheRecord
		json.Unmarshal(b, &rec)
		rec.FetchedAt = time.Now().Add(-2 * time.Hour)
		b, _ = json.Marshal(rec)
		os.WriteFile(p, b, 0600)
	}
	if _, e := c.Query(context.Background(), url.Values{"limit": {"2"}}); e != nil || !c.Fetches[len(c.Fetches)-1].Stale {
		t.Fatal(e, c.Fetches)
	}
}
func mustReadDir(t *testing.T, path string) []os.DirEntry {
	t.Helper()
	v, e := os.ReadDir(path)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func TestQueryRejectsInvalidAndThrottled(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		status     int
		code       string
	}{{"malformed", "<html>", 200, "invalid_response"}, {"missing total", `{"items":[]}`, 200, "invalid_response"}, {"auth", "denied", 403, "http_error"}, {"rate", "", 429, "rate_limited"}} {
		t.Run(tt.name, func(t *testing.T) {
			c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tt.status); w.Write([]byte(tt.body)) }, Options{NoCache: true})
			_, e := c.Query(context.Background(), url.Values{})
			if e == nil {
				t.Fatal("success")
			}
			if tt.status == 429 {
				var rate *cliutil.RateLimitError
				if !errors.As(e, &rate) || Classify(e).Code != "rate_limited" {
					t.Fatal(e)
				}
			} else if tt.status == 200 && Classify(e).Code != tt.code {
				t.Fatal(e)
			}
		})
	}
}
func TestRequestBudgetAndTimeout(t *testing.T) {
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(emptyFeed)) }, Options{NoCache: true})
	c.Stats.Requests = maxRequests
	if _, e := c.Query(context.Background(), url.Values{}); Classify(e).Code != "request_budget" {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.Stats.Requests = 0
	if _, e := c.Query(ctx, url.Values{}); Classify(e).Code != "timeout" {
		t.Fatal(e)
	}
	if e := Fail("usage", "fix input", 2); e.Exit != 2 || e.Error() != "fix input" || Classify(e) != e {
		t.Fatal(e)
	}
}

func TestCacheEvictionOwnsOnlyHashedRecords(t *testing.T) {
	dir := t.TempDir()
	unrelated := filepath.Join(dir, "trip-notes.json")
	if err := os.WriteFile(unrelated, []byte(`{"keep":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-8 * 24 * time.Hour)
	if err := os.Chtimes(unrelated, old, old); err != nil {
		t.Fatal(err)
	}
	c := NewClient(Options{CacheDir: dir})
	path := filepath.Join(dir, cachePrefix+strings.Repeat("a", 64)+".json")
	c.saveCache(path, cacheRecord{FetchedAt: time.Now(), Data: json.RawMessage(emptyFeed)})
	if _, err := os.Stat(unrelated); err != nil {
		t.Fatalf("unrelated JSON removed: %v", err)
	}
	for _, tt := range []struct {
		name string
		want bool
	}{{cachePrefix + strings.Repeat("a", 64) + ".json", true}, {"notes.json", false}, {strings.Repeat("g", 64) + ".json", false}, {strings.Repeat("A", 64) + ".json", false}} {
		if ownedCacheFile(tt.name) != tt.want {
			t.Fatal(tt)
		}
	}
}
