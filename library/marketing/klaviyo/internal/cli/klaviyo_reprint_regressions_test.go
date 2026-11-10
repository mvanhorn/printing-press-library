// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

func TestKlaviyoReprintProbeRetriesCountForCLIAndMCP(t *testing.T) {
	for _, entrypoint := range []string{"CLI", "MCP"} {
		t.Run(entrypoint, func(t *testing.T) {
			profile := greptileG1Profile(t)
			previous, previousWait := registeredPlatformSource, waitForIdentityProbeRetry
			t.Cleanup(func() { registeredPlatformSource, waitForIdentityProbeRetry = previous, previousWait })
			calls := 0
			transport := klaviyoCloneTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				status := http.StatusServiceUnavailable
				if calls == 3 {
					status = http.StatusOK
				}
				return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"acct-a","attributes":{"contact_information":{"organization_name":"Acme"}}}]}`)), Request: r}, nil
			})
			registration := *previous
			registration.Adapter = klaviyoIdentityAdapter{httpClient: &http.Client{Transport: transport}}
			registeredPlatformSource = &registration
			var waits []time.Duration
			waitForIdentityProbeRetry = func(_ context.Context, wait time.Duration) error {
				waits = append(waits, wait)
				return nil
			}
			var session *platform.Session
			var err error
			if entrypoint == "CLI" {
				session, err = platform.PrepareProfileSource(profile.Name, "klaviyo-pp-cli", "klaviyo")
				if err == nil {
					err = verifyPlatformSession(context.Background(), &rootFlags{platformSession: session})
				}
			} else {
				t.Setenv(mcpBoundProfileEnv, profile.Name)
				session, err = VerifyMCPInvocation(context.Background())
			}
			if err != nil {
				t.Fatal(err)
			}
			defer session.ZeroCredentials()
			want := platform.RateLimitMetadata{Requests: 3, Retries: 2, WaitMS: 3000}
			if calls != 3 || !reflect.DeepEqual(waits, []time.Duration{time.Second, 2 * time.Second}) || !reflect.DeepEqual(session.RateLimitMetadata(), want) {
				t.Fatalf("calls=%d waits=%v metadata=%+v, want %+v", calls, waits, session.RateLimitMetadata(), want)
			}
		})
	}
}

func TestKlaviyoReprintFirstDefaultSyncIncludesCouponCodes(t *testing.T) {
	for _, concurrency := range []int{1, 4} {
		t.Run(fmt.Sprint(concurrency), func(t *testing.T) {
			profile := greptileG1Profile(t)
			previous := registeredPlatformSource
			t.Cleanup(func() { registeredPlatformSource = previous })
			var mu sync.Mutex
			couponsComplete := false
			filters := map[string]int{}
			transport := klaviyoCloneTransport(func(r *http.Request) (*http.Response, error) {
				mu.Lock()
				defer mu.Unlock()
				body := `{"data":[],"links":{"next":null}}`
				switch r.URL.Path {
				case "/api/accounts":
					body = `{"data":[{"id":"acct-a","attributes":{"contact_information":{"organization_name":"Acme"}}}],"links":{"next":null}}`
				case "/api/coupons":
					couponsComplete = true
					body = `{"data":[{"type":"coupon","id":"coupon-a"},{"type":"coupon","id":"coupon-b"}],"links":{"next":null}}`
				case "/api/coupon-codes":
					if !couponsComplete {
						t.Error("coupon codes requested before coupons completed")
					}
					filters[r.URL.Query().Get("filter")]++
				}
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/vnd.api+json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})
			registration := *previous
			registration.Adapter = klaviyoIdentityAdapter{httpClient: &http.Client{Transport: transport}}
			registration.BindClient = func(c *client.Client, session *platform.Session) error {
				if err := previous.BindClient(c, session); err != nil {
					return err
				}
				c.HTTPClient.Transport = transport
				for _, class := range []string{"*", "GET /api/accounts", "GET /api/profiles"} {
					if err := c.SetPlatformEndpointBudget(platform.EndpointBudget{Class: class, Steady: 10000, Interval: time.Second, Burst: 100, MaxAttempts: 1, RetryBudget: time.Second}); err != nil {
						return err
					}
				}
				return nil
			}
			registeredPlatformSource = &registration
			root := RootCmd()
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			root.SetArgs([]string{"sync", "--client-profile", profile.Name, "--concurrency", fmt.Sprint(concurrency), "--strict"})
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := root.ExecuteContext(ctx); err != nil {
				t.Fatal(err)
			}
			want := map[string]int{"equals(coupon.id,'coupon-a')": 1, "equals(coupon.id,'coupon-b')": 1}
			if !reflect.DeepEqual(filters, want) {
				t.Fatalf("coupon code requests=%v, want %v", filters, want)
			}
		})
	}
}
