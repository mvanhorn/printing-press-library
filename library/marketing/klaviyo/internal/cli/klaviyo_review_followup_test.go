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
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/store"
	"github.com/spf13/cobra"
)

func reviewFollowupRecordingRoot(t *testing.T, handler http.HandlerFunc) (*cobra.Command, *rootFlags) {
	t.Helper()
	root, flags := rcOfflineSyncRoot(t)
	registeredPlatformSource.BindClient = func(c *client.Client, _ *platform.Session) error {
		c.BaseURL, c.Config.KlaviyoApiKey, c.NoCache = "https://klaviyo.test", "fixture-key", true
		c.HTTPClient.Transport = klaviyoCloneTransport(func(r *http.Request) (*http.Response, error) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, r)
			return response.Result(), nil
		})
		return c.SetPlatformEndpointBudget(platform.EndpointBudget{Class: "*", Steady: 10000, Interval: time.Second, Burst: 100, MaxAttempts: 1, RetryBudget: time.Second})
	}
	return root, flags
}

func TestReviewFollowupOptimalSendTimeJSONStaysCompact(t *testing.T) {
	posts := 0
	root, flags := reviewFollowupRecordingRoot(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /api/metrics":
			fmt.Fprint(w, `{"data":[{"id":"opened","attributes":{"name":"Opened Email"}},{"id":"received","attributes":{"name":"Received Email"}}],"links":{"next":null}}`)
		case "POST /api/metric-aggregates":
			posts++
			count := 5
			if posts == 2 {
				count = 10
			}
			fmt.Fprintf(w, `{"data":{"attributes":{"dates":[%q],"data":[{"measurements":{"count":[%d]}}]}}}`, time.Now().UTC().Add(-24*time.Hour).Truncate(time.Hour).Format(time.RFC3339), count)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL)
		}
	})
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"optimal-send-time", "--timezone", "UTC", "--json"})
	err := root.Execute()
	finalizePlatformInvocation(flags, &err)
	if err != nil || posts != 2 || !strings.Contains(output.String(), `"engagement_rate":0.5`) {
		t.Fatalf("posts=%d error=%v output=%s", posts, err, &output)
	}
}

func TestReviewFollowupSyncCapsResumeEmptyAndShortPages(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(fmt.Sprint(empty), func(t *testing.T) {
			var cursors []string
			root, flags := reviewFollowupRecordingRoot(t, func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("page[size]"); got != "100" {
					t.Errorf("page size = %q, want published size 100", got)
				}
				cursor := r.URL.Query().Get("page[cursor]")
				cursors = append(cursors, cursor)
				if cursor == "" {
					rows := `[{"id":"p1","type":"profile"}]`
					if empty {
						rows = `[]`
					}
					fmt.Fprintf(w, `{"data":%s,"links":{"next":"/api/profiles?page%%5Bcursor%%5D=page2"}}`, rows)
				} else {
					fmt.Fprint(w, `{"data":[{"id":"p2","type":"profile"}],"links":{"next":null}}`)
				}
			})
			for run := 0; run < 2; run++ {
				if run > 0 {
					session, err := platform.PrepareProfileSourceFromConfig(flags.platformSession.Profile, "klaviyo-pp-cli", "klaviyo")
					if err != nil {
						t.Fatal(err)
					}
					session.GateOutcome = platform.GateVerified
					session.ObservedIdentity = flags.platformSession.ObservedIdentity
					flags = &rootFlags{platformSession: session}
					root = newRootCmd(flags)
				}
				root.SetOut(io.Discard)
				root.SetErr(io.Discard)
				root.SetArgs([]string{"sync", "--resources", "profiles", "--max-pages", "1"})
				if err := root.Execute(); err != nil {
					t.Fatal(err)
				}
				if len(cursors) != run+1 {
					t.Fatalf("run %d fetched beyond its one-page cap: %v", run, cursors)
				}
				db, err := store.OpenVerifiedWithContext(context.Background(), flags.platformSession.Paths.DataFile, flags.platformSession)
				if err != nil {
					t.Fatal(err)
				}
				cursor, _, count, err := db.GetSyncState("profiles")
				db.Close()
				wantCursor, wantCount := "", 1
				if run == 0 {
					wantCursor = "page2"
					if empty {
						wantCount = 0
					}
				}
				if err != nil || cursor != wantCursor || count != wantCount || flags.platformSession.OutputMetadata().Truncated != (run == 0) {
					t.Fatalf("run=%d cursor=%q count=%d metadata=%+v error=%v", run, cursor, count, flags.platformSession.OutputMetadata(), err)
				}
			}
			if len(cursors) != 2 || cursors[0] != "" || cursors[1] != "page2" {
				t.Fatalf("cursors=%v", cursors)
			}
		})
	}
}

func TestReviewFollowupFilteredFullSyncKeepsCheckpoint(t *testing.T) {
	root, flags := reviewFollowupRecordingRoot(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Query().Get("filter"), "greater-than(updated,") || r.URL.Query().Get("page[cursor]") != "" {
			t.Fatalf("query=%v", r.URL.Query())
		}
		fmt.Fprint(w, `{"data":[{"id":"filtered","type":"profile"}],"links":{"next":"/api/profiles?page%5Bcursor%5D=filtered-next"}}`)
	})
	db, err := store.OpenVerifiedWithContext(context.Background(), flags.platformSession.Paths.DataFile, flags.platformSession)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSyncState("profiles", "unfiltered-next", 42); err != nil {
		t.Fatal(err)
	}
	_, savedAt, _, _ := db.GetSyncState("profiles")
	db.Close()
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"sync", "--resources", "profiles", "--full", "--since", "1d", "--max-pages", "1"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	db, err = store.OpenVerifiedWithContext(context.Background(), flags.platformSession.Paths.DataFile, flags.platformSession)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cursor, at, count, err := db.GetSyncState("profiles")
	metadata, _ := json.Marshal(flags.platformSession.OutputMetadata())
	if err != nil || cursor != "unfiltered-next" || count != 42 || !at.Equal(savedAt) || strings.Contains(string(metadata), "filtered-next") {
		t.Fatalf("cursor=%q count=%d at=%v metadata=%s error=%v", cursor, count, at, metadata, err)
	}
}

func TestReviewFollowupReportContinuationRetainsFinalAPIError(t *testing.T) {
	const first = "/api/campaign-values-reports/"
	const second = first + "?page_cursor=page-2"
	const failureBody = `{"errors":[{"detail":"query failed"}]}`
	calls := 0
	_, flags := reviewFollowupRecordingRoot(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		want := second
		if calls == 1 {
			want = first
		}
		if r.Method != http.MethodPost || r.URL.RequestURI() != want {
			t.Errorf("request %d = %s %s, want POST %s", calls, r.Method, r.URL.RequestURI(), want)
		}
		if calls == 1 {
			fmt.Fprintf(w, `{"data":{"attributes":{"results":[{"row":1}]}},"links":{"next":%q}}`, second)
			return
		}
		w.Header().Set("Retry-After", "1")
		w.Header().Set("X-Request-ID", "report-request")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, failureBody)
	})
	bindClient := registeredPlatformSource.BindClient
	registeredPlatformSource.BindClient = func(c *client.Client, session *platform.Session) error {
		if err := bindClient(c, session); err != nil {
			return err
		}
		return c.SetPlatformEndpointBudget(platform.EndpointBudget{
			Class: "POST /api/campaign-values-reports", Steady: 1000, Interval: time.Second,
			Burst: 10, MaxAttempts: 4, RetryBudget: time.Minute,
		})
	}
	waits := 0
	previousWait := waitForKlaviyoReportRetry
	t.Cleanup(func() { waitForKlaviyoReportRetry = previousWait })
	waitForKlaviyoReportRetry = func(ctx context.Context, wait time.Duration) error {
		waits++
		if ctx != flags.platformSession.Context() || wait != time.Second {
			t.Errorf("retry context = %v, wait = %v", ctx, wait)
		}
		return ctx.Err()
	}
	flags.asJSON, flags.noCache, flags.timeout = true, true, time.Second
	cmd := newCampaignValuesReportsPromotedCmd(flags)
	cmd.SilenceUsage = true
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--statistics", "recipients,conversion_value", "--timeframe", "last_7_days", "--conversion-metric-id", "orders"})
	err := cmd.Execute()
	wantError := "rate limited: endpoint class POST /api/campaign-values-reports exhausted after 1 attempts; retry after 1s; request id report-request"
	if err == nil || err.Error() != wantError || calls != 5 || waits != 3 {
		t.Fatalf("error = %v, requests = %d, waits = %d; want %q, 5 requests and 3 waits", err, calls, waits, wantError)
	}
	var apiError *client.APIError
	if !errors.As(err, &apiError) || apiError.StatusCode != 429 || apiError.Path != second || apiError.Body != failureBody {
		t.Fatalf("lost final page API error: %v", err)
	}
	var envelope map[string]any
	if json.Unmarshal(output.Bytes(), &envelope) != nil || len(envelope) != 2 || envelope["error"] != wantError || envelope["code"] != float64(7) {
		t.Fatalf("expected only JSON error envelope, got %s", &output)
	}
}
