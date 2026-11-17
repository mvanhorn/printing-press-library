// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/config"
)

func TestUncacheableRequest(t *testing.T) {
	cases := []struct {
		path   string
		params map[string]string
		want   bool
	}{
		{"/v1/jobs/abc123", nil, true},
		{"/v1/jobs", nil, true},
		{"/v1/google/search", map[string]string{"query": "x", "is_async": "true"}, true},
		{"/v1/google/search", map[string]string{"query": "x", "is_async": "1"}, true},
		{"/v1/google/search?query=x&is_async=true", nil, true},
		{"/v1/google/search", map[string]string{"query": "x", "is_async": "false"}, false},
		{"/v1/google/search", map[string]string{"query": "x"}, false},
		{"/v1/jobsearch", nil, false},
	}
	for _, tc := range cases {
		if got := uncacheableRequest(tc.path, tc.params); got != tc.want {
			t.Errorf("uncacheableRequest(%q, %v) = %v, want %v", tc.path, tc.params, got, tc.want)
		}
	}
}

// Repeated job reads and async submissions must reach the API every time,
// while an ordinary search is still served from the cache.
func TestResponseCacheSkipsAsyncAndJobs(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"queued"}`))
	}))
	t.Cleanup(server.Close)

	c := New(&config.Config{BaseURL: server.URL}, time.Second, 0)
	c.HTTPClient = server.Client()
	c.cacheDir = t.TempDir()
	ctx := context.Background()

	count := func(path string, params map[string]string) int32 {
		before := hits.Load()
		for i := 0; i < 2; i++ {
			if _, err := c.Get(ctx, path, params); err != nil {
				t.Fatalf("Get %s: %v", path, err)
			}
		}
		return hits.Load() - before
	}
	if n := count("/v1/jobs/abc123", nil); n != 2 {
		t.Errorf("job read reached the API %d times in 2 calls, want 2", n)
	}
	if n := count("/v1/google/search", map[string]string{"query": "x", "is_async": "true"}); n != 2 {
		t.Errorf("async search reached the API %d times in 2 calls, want 2", n)
	}
	if n := count("/v1/google/search", map[string]string{"query": "x"}); n != 1 {
		t.Errorf("sync search reached the API %d times in 2 calls, want 1 (cached)", n)
	}
}
