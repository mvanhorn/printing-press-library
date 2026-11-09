// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

func TestReviewREReportPageRetries(t *testing.T) {
	const first = "/api/campaign-values-reports/"
	const second = first + "?page_cursor=page-2"
	const body = `{"data":{"attributes":{"conversion_metric_id":"orders","statistics":["recipients","conversion_value"],"timeframe":{"key":"last_7_days"}},"type":"campaign-values-report"}}`
	const failureBody = `{"errors":[{"detail":"query failed"}]}`
	const rateLimitError = "rate limited: endpoint class POST /api/campaign-values-reports exhausted after 1 attempts; retry after 1s; request id report-request"
	for _, test := range []struct {
		name           string
		firstFailures  int
		secondFailures int
		failureStatus  int
		retryAfter     string
		repeatCursor   bool
		wantPaths      []string
		wantWaits      []time.Duration
		wantError      string
	}{
		{
			name: "continuation recovers after one 429", secondFailures: 1,
			wantPaths: []string{first, second, second}, wantWaits: []time.Duration{time.Second},
		},
		{
			name: "continuation stops after four 429s", secondFailures: 4,
			wantPaths: []string{first, second, second, second, second},
			wantWaits: []time.Duration{time.Second, time.Second, time.Second}, wantError: rateLimitError,
		},
		{
			name: "wait is capped at sixty seconds", secondFailures: 1, retryAfter: "120",
			wantPaths: []string{first, second, second}, wantWaits: []time.Duration{time.Minute},
		},
		{
			name: "retry allowance resets for each page", firstFailures: 3, secondFailures: 3,
			wantPaths: []string{first, first, first, first, second, second, second, second},
			wantWaits: []time.Duration{time.Second, time.Second, time.Second, time.Second, time.Second, time.Second},
		},
		{
			name: "repeated cursor still rejected after retry", secondFailures: 1, repeatCursor: true,
			wantPaths: []string{first, second, second}, wantWaits: []time.Duration{time.Second},
			wantError: `Klaviyo report pagination repeated page_cursor "page-2"`,
		},
		{
			name: "bad request is not retried", secondFailures: 1, failureStatus: http.StatusBadRequest,
			wantPaths: []string{first, second},
			wantError: "POST " + second + " returned HTTP 400: " + failureBody,
		},
		{
			name: "server error is not replayed", secondFailures: 1, failureStatus: http.StatusInternalServerError,
			wantPaths: []string{first, second},
			wantError: "POST " + second + " returned HTTP 500: " + failureBody,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var paths []string
			counts := map[string]int{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				uri := r.URL.RequestURI()
				paths = append(paths, uri)
				counts[uri]++
				if r.Method != http.MethodPost || (uri != first && uri != second) {
					t.Errorf("request = %s %s, want report POST", r.Method, uri)
					http.NotFound(w, r)
					return
				}
				for name, want := range map[string]string{
					"Authorization": "Klaviyo-API-Key synthetic-key", "revision": "2026-04-15",
					"Accept": "application/vnd.api+json", "Content-Type": "application/vnd.api+json",
					"Idempotency-Key": "", "X-Idempotency-Key": "",
				} {
					if got := r.Header.Get(name); got != want {
						t.Errorf("%s = %q, want %q", name, got, want)
					}
				}
				if got, err := io.ReadAll(r.Body); err != nil || string(got) != body {
					t.Errorf("body = %s, error = %v, want %s", got, err, body)
				}
				w.Header().Set("Content-Type", "application/vnd.api+json")
				failures := test.firstFailures
				if uri == second {
					failures = test.secondFailures
				}
				if counts[uri] <= failures {
					status := test.failureStatus
					if status == 0 {
						status = http.StatusTooManyRequests
					}
					retryAfter := test.retryAfter
					if retryAfter == "" {
						retryAfter = "1"
					}
					w.Header().Set("Retry-After", retryAfter)
					w.Header().Set("X-Request-ID", "report-request")
					w.WriteHeader(status)
					fmt.Fprint(w, failureBody)
					return
				}
				if uri == first {
					fmt.Fprintf(w, `{"data":{"id":"report-1","type":"campaign-values-report","attributes":{"results":[{"row":1}]}},"links":{"next":%q}}`, "http://"+r.Host+second)
				} else if test.repeatCursor {
					fmt.Fprintf(w, `{"data":{"attributes":{"results":[{"row":2}]}},"links":{"next":%q}}`, second)
				} else {
					fmt.Fprint(w, `{"data":{"attributes":{"results":[{"row":2}]}},"links":{"next":null}}`)
				}
			}))
			defer server.Close()
			var waits []time.Duration
			previousWait, previousSource := waitForKlaviyoReportRetry, registeredPlatformSource
			t.Cleanup(func() {
				waitForKlaviyoReportRetry, registeredPlatformSource = previousWait, previousSource
			})
			waitForKlaviyoReportRetry = func(ctx context.Context, wait time.Duration) error {
				waits = append(waits, wait)
				return ctx.Err()
			}
			registeredPlatformSource = &platformSourceRegistration{
				BindClient: func(c *client.Client, _ *platform.Session) error {
					return reviewREConfigureClient(c, server)
				},
			}
			cmd := newCampaignValuesReportsPromotedCmd(&rootFlags{
				asJSON: true, noCache: true, timeout: time.Second,
				platformSession: reviewREReportSession(t, context.Background()),
			})
			cmd.SilenceUsage = true
			var output, stderr bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{"--statistics", "recipients,conversion_value", "--timeframe", "last_7_days", "--conversion-metric-id", "orders"})
			err := cmd.Execute()
			if !reflect.DeepEqual(paths, test.wantPaths) || !reflect.DeepEqual(waits, test.wantWaits) {
				t.Fatalf("requests = %v, waits = %v; want %v and %v", paths, waits, test.wantPaths, test.wantWaits)
			}
			if test.wantError != "" {
				if err == nil || err.Error() != test.wantError || output.Len() != 0 {
					t.Fatalf("error = %v, output = %q; want %q and no partial output", err, output.String(), test.wantError)
				}
				if test.wantError == rateLimitError {
					var apiError *client.APIError
					if !errors.As(err, &apiError) || apiError.StatusCode != 429 || apiError.Path != second || apiError.Body != failureBody {
						t.Fatalf("lost final page API error: %v", err)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var envelope map[string]any
			if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			wantRows := []any{map[string]any{"row": float64(1)}, map[string]any{"row": float64(2)}}
			if got := anyPath(envelope, "results.data.attributes.results"); !reflect.DeepEqual(got, wantRows) || anyPath(envelope, "results.data.id") != "report-1" {
				t.Fatalf("command lost or duplicated report rows: %s", output.String())
			}
		})
	}
}

func TestReviewREReportRetryHonorsSessionCancellation(t *testing.T) {
	const path = "/api/campaign-values-reports/"
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session := reviewREReportSession(t, ctx)
	c := client.New(&config.Config{}, time.Second, 0)
	if err := c.BindPlatformSession(session); err != nil {
		t.Fatal(err)
	}
	if err := reviewREConfigureClient(c, server); err != nil {
		t.Fatal(err)
	}
	previousWait := waitForKlaviyoReportRetry
	t.Cleanup(func() { waitForKlaviyoReportRetry = previousWait })
	waits := 0
	waitForKlaviyoReportRetry = func(gotCtx context.Context, wait time.Duration) error {
		waits++
		if gotCtx != session.Context() || wait != time.Minute {
			t.Errorf("retry context = %v, wait = %v; want session context and 60s", gotCtx, wait)
		}
		cancel()
		return previousWait(gotCtx, wait)
	}
	raw, status, err := postKlaviyoReport(c, path, json.RawMessage(`{"data":{"type":"campaign-values-report"}}`))
	if !errors.Is(err, context.Canceled) || err.Error() != "context canceled" || status != 429 || raw != nil || calls != 1 || waits != 1 {
		t.Fatalf("response = %s, status = %d, error = %v, requests = %d, waits = %d", raw, status, err, calls, waits)
	}
}

func reviewREReportSession(t *testing.T, ctx context.Context) *platform.Session {
	t.Helper()
	source := platform.SourceProfile{
		CredentialRef: "op://Test/Klaviyo/token", ExpectedAccountID: "acct-test", ExpectedOrgName: "Test",
	}
	session := &platform.Session{
		Profile: &platform.Profile{
			SchemaVersion: platform.ProfileSchemaVersion, Name: "review-re",
			Sources: map[string]platform.SourceProfile{"klaviyo": source},
		},
		ProfileName: "review-re", Source: "klaviyo", SourceProfile: source,
		ExpectedIdentity: source.ExpectedIdentity(), Paths: platform.Paths{StateDir: t.TempDir()},
	}
	if err := platform.VerifyPreparedProfileSource(ctx, session,
		platformCLIResolver{value: []byte("synthetic-key")},
		platformCLIAdapter{observed: platform.ObservedIdentity{AccountID: "acct-test", OrgName: "Test"}},
	); err != nil {
		t.Fatal(err)
	}
	return session
}

func reviewREConfigureClient(c *client.Client, server *httptest.Server) error {
	c.BaseURL, c.HTTPClient, c.Config.KlaviyoApiKey = server.URL, server.Client(), "synthetic-key"
	c.NoCache = true
	// Keep the production replay policy while eliminating proactive pacing in these retry tests.
	return c.SetPlatformEndpointBudget(platform.EndpointBudget{
		Class: "POST /api/campaign-values-reports", Steady: 1000, Interval: time.Second, Burst: 10,
		MaxAttempts: 4, RetryBudget: time.Minute,
	})
}
