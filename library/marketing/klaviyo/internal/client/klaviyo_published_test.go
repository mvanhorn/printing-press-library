// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"context"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetRetriesRateLimitUsingRetryAfter(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"segment-1","attributes":{"profile_count":42}}}`))
	}))
	defer server.Close()

	c := New(&config.Config{BaseURL: server.URL, KlaviyoApiKey: "test-key"}, 5*time.Second, 1000)
	c.NoCache = true
	c.platformBudgets = map[string]platform.EndpointBudget{
		"*": {Class: "*", Steady: 2, Interval: time.Second, Burst: 2, MaxAttempts: 4, RetryBudget: time.Minute},
	}
	started := time.Now()
	if _, err := c.Get(context.Background(), "/api/segments/segment-1/", map[string]string{"additional-fields[segment]": "profile_count"}); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("request count = %d, want 2", got)
	}
	if elapsed := time.Since(started); elapsed < time.Second {
		t.Fatalf("Retry-After was not honored; elapsed = %s", elapsed)
	}
}
