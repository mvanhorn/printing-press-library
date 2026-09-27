package jalan

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCacheFreshnessExactQueryAndObservationTime(t *testing.T) {
	calls := 0
	c := testClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		return httpResult(200, searchHTML(r, 0, 2, 2, false)), nil
	})
	now := testNow
	c.now = func() time.Time { return now }
	q := datedQuery()
	q.Destination = "Hakone"
	first, err := c.Search(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	observed := first.Meta["observed_at"]
	if first.Meta["cache_status"] != "live" || first.Meta["upstream_requests"] != 1 {
		t.Fatal(first.Meta)
	}
	now = now.Add(30 * time.Second)
	c.options.MaxAge = time.Minute
	reused, err := c.Search(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || reused.Meta["upstream_requests"] != 0 || reused.Meta["cache_status"] != "hit" || reused.Meta["observed_at"] != observed || reused.Meta["cache_age_ms"] != int64(30000) {
		t.Fatalf("cache falsely refreshed: calls=%d meta=%v", calls, reused.Meta)
	}
	q.Adults = 3
	different, err := c.Search(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || different.Meta["cache_status"] != "live" {
		t.Fatal("different occupancy reused original cache")
	}
	q.Adults = 2
	c.options.Refresh = true
	refreshed, err := c.Search(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 || refreshed.Meta["cache_status"] != "live" || refreshed.Meta["observed_at"] == observed {
		t.Fatal("refresh did not observe live source")
	}
	c.options.Refresh = false
	c.options.MaxAge = 5 * time.Minute
	now = now.Add(5*time.Minute + time.Second)
	expired, err := c.Search(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 4 || expired.Meta["cache_status"] != "live" {
		t.Fatal("expired cache reused inventory")
	}
	c.options.MaxAge = 0
	_, err = c.Search(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 5 {
		t.Fatal("default freshness performed a cached read")
	}
	for _, observation := range expired.Meta["observations"].([]observation) {
		path, err := c.cachePath(observation.URL)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("cache mode=%o", info.Mode().Perm())
		}
	}
}
func TestCacheRefreshDisableAndClockRollback(t *testing.T) {
	c := testClient(t, func(*http.Request) (*http.Response, error) { return httpResult(200, "fresh"), nil })
	c.options.MaxAge = time.Minute
	sourceURL := "https://www.jalan.net/one?adultNum=2"
	if err := c.writeCache(cacheEntry{Version: cacheVersion, URL: sourceURL, ObservedAt: testNow.Add(time.Minute), Body: "future"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.readCache(sourceURL); ok {
		t.Fatal("future observation accepted after clock rollback")
	}
	c.options.DisableCache = true
	disabledURL := "https://www.jalan.net/two"
	if err := c.writeCache(cacheEntry{Version: cacheVersion, URL: disabledURL, ObservedAt: testNow, Body: "should not persist"}); err != nil {
		t.Fatal(err)
	}
	path, _ := c.cachePath(disabledURL)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("disabled cache persisted response")
	}
	if _, ok := c.readCache(sourceURL); ok {
		t.Fatal("disabled cache performed read")
	}
	c.options.DisableCache = false
	if err := c.writeCache(cacheEntry{Version: cacheVersion, URL: sourceURL, ObservedAt: testNow, Body: "stored"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.readCache(strings.Replace(sourceURL, "2", "3", 1)); ok {
		t.Fatal("cache reused different exact URL")
	}
}
func TestInvalidCacheAgeRejectsBeforeNetwork(t *testing.T) {
	calls := 0
	c := NewClient(Options{MaxAge: 6 * time.Minute, CacheDir: t.TempDir()})
	c.HTTP.Transport = transportFunc(func(*http.Request) (*http.Response, error) { calls++; return httpResult(200, "unused"), nil })
	_, err := c.Property(context.Background(), "385995")
	if err == nil || calls != 0 {
		t.Fatalf("invalid age executed request: %v calls=%d", err, calls)
	}
}

func TestCachedLogicalSlicesShareOriginalSourceObservation(t *testing.T) {
	calls := 0
	c := testClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		// A second live fetch would reorder the same native source page.
		return httpResult(200, searchHTML(r, (calls-1)*100, 30, 90, true)), nil
	})
	c.options.MaxAge = 5 * time.Minute
	now := testNow
	c.now = func() time.Time { return now }
	q := datedQuery()
	q.Destination = "Hakone"
	first, err := c.Search(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(30 * time.Second)
	q.Page = 2
	second, err := c.Search(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || second.Meta["upstream_requests"] != 0 || second.Meta["cache_status"] != "hit" {
		t.Fatalf("logical slice fetched a different observation: calls=%d meta=%v", calls, second.Meta)
	}
	if first.Meta["observed_at"] != second.Meta["observed_at"] || first.Meta["source_url"] != second.Meta["source_url"] {
		t.Fatal("logical page changed its source observation")
	}
	for index, item := range second.Results {
		if item.(Property).ID != strconv.Itoa(100005+index) {
			t.Fatalf("cached second slice skipped an item: %v", second.Results)
		}
	}
	q.Page = 1
	q.Limit = 10
	wider, err := c.Search(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || wider.Meta["observed_at"] != first.Meta["observed_at"] || len(wider.Results) != 10 {
		t.Fatal("limit change did not slice the same cached source observation")
	}
	if !strings.Contains(second.Meta["coverage"].(string), "not exhaustive") || !strings.Contains(second.Pagination["next_action"].(string), "--max-age 5m") {
		t.Fatalf("pagination coverage guidance missing: %v %v", second.Meta, second.Pagination)
	}
	warnings := second.Meta["warnings"].([]string)
	if len(warnings) == 0 || !strings.Contains(warnings[0], "does not create a snapshot across native pages") {
		t.Fatalf("snapshot limitation missing: %v", warnings)
	}
}
