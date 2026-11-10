// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

func TestKlaviyoReprintUnprotectedWritesDoNotReplay(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("PRINTING_PRESS_DOGFOOD", "")
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		for _, failure := range []string{"server", "transport"} {
			t.Run(method+"/"+failure, func(t *testing.T) {
				calls := 0
				c := New(&config.Config{BaseURL: "https://example.test"}, 10*time.Second, 0)
				c.HTTPClient.Transport = klaviyoRoundTripper(func(r *http.Request) (*http.Response, error) {
					calls++
					if failure == "transport" {
						return nil, io.EOF
					}
					return &http.Response{StatusCode: 500, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"errors":[{"detail":"ambiguous"}]}`)), Request: r}, nil
				})
				_, _, err := c.do(context.Background(), method, "/api/write", nil, nil, nil)
				if err == nil || calls != 1 {
					t.Fatalf("calls=%d error=%v; want one failed write", calls, err)
				}
			})
		}
	}
}

func TestKlaviyoReprintDeclaredBudgetControlsAutomaticFallback(t *testing.T) {
	for _, rate := range []float64{RateLimitAuto, 3, 0} {
		c := New(&config.Config{}, time.Second, rate)
		if err := c.BindPlatformSession(&platform.Session{GateOutcome: platform.GateVerified}); err != nil {
			t.Fatal(err)
		}
		if err := c.SetPlatformEndpointBudget(platform.EndpointBudget{Class: "*", Steady: 10000, Interval: time.Second, Burst: 100, MaxAttempts: 1, RetryBudget: time.Second}); err != nil {
			t.Fatal(err)
		}
		want := rate
		if rate == RateLimitAuto {
			want = 10000
		}
		if c.RateLimit() != want {
			t.Fatalf("rate=%v: effective rate=%v, want %v", rate, c.RateLimit(), want)
		}
		if err := c.SetPlatformEndpointBudget(platform.EndpointBudget{Class: "*", Steady: 1, Interval: time.Second, Burst: 1, MaxAttempts: 1, RetryBudget: time.Second}); err != nil {
			t.Fatal(err)
		}
		if rate == RateLimitAuto && c.RateLimit() != 2 {
			t.Fatal("lower endpoint budget changed the existing automatic fallback")
		}
		if c.cacheDir != "" {
			t.Fatal("cache-less session retained a legacy response cache")
		}
	}
}
