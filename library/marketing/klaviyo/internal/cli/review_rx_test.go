package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

type rxTransport func(*http.Request) (*http.Response, error)

func (f rxTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func rxHTTPFixture(t *testing.T, mode string, handler http.Handler) string {
	t.Helper()
	if mode == "server" {
		mock := httptest.NewServer(handler)
		t.Cleanup(mock.Close)
		return mock.URL
	}
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	http.DefaultTransport = rxTransport(func(r *http.Request) (*http.Response, error) {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, r)
		return response.Result(), nil
	})
	return "https://klaviyo.test"
}

func TestRXFlowPathAnalysisPropagatesAggregateFailure(t *testing.T) {
	t.Setenv("KLAVIYO_TIMEZONE", "UTC")
	for _, mode := range []string{"recorder", "server"} {
		for _, failAt := range []int{1, 2, 5} {
			t.Run(fmt.Sprintf("%s/aggregate-%d", mode, failAt), func(t *testing.T) {
				posts, flowReads, metricReads := 0, 0, 0
				now := time.Now().UTC() // the request declares timezone UTC, so its calendar days are UTC days
				const failure = `{"errors":[{"detail":"message aggregation unavailable"}]}`
				baseURL := rxHTTPFixture(t, mode, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					for key, want := range map[string]string{"revision": "2026-10-15", "Authorization": "Klaviyo-API-Key test-key"} {
						if got := r.Header.Get(key); got != want {
							t.Errorf("%s = %q, want %q", key, got, want)
						}
					}
					w.Header().Set("Content-Type", "application/vnd.api+json")
					switch r.Method + " " + r.URL.Path {
					case "GET /api/flows/flow-one":
						flowReads++
						if r.URL.RawQuery != "additional-fields%5Bflow%5D=definition" {
							t.Errorf("flow query = %q", r.URL.RawQuery)
						}
						_, _ = io.WriteString(w, `{"data":{"attributes":{"name":"Welcome","definition":{"actions":[{"id":"email-one","type":"send-email","data":{"message":{"name":"Welcome email"}}}]}}}}`)
					case "GET /api/metrics":
						metricReads++
						if r.URL.RawQuery != "fields%5Bmetric%5D=name" {
							t.Errorf("metrics query = %q", r.URL.RawQuery)
						}
						_, _ = io.WriteString(w, `{"data":[{"id":"sent","attributes":{"name":"Received Email"}},{"id":"opened","attributes":{"name":"Opened Email"}},{"id":"clicked","attributes":{"name":"Clicked Email"}},{"id":"orders","attributes":{"name":"Placed Order"}}],"links":{"next":null}}`)
					case "POST /api/metric-aggregates":
						posts++
						body, err := io.ReadAll(r.Body)
						var decoded map[string]any
						if err != nil || json.Unmarshal(body, &decoded) != nil {
							t.Errorf("invalid aggregate body %s, error = %v", body, err)
						}
						metric := anyPath(decoded, "data.attributes.metric_id")
						if metric != "sent" && metric != "opened" && metric != "clicked" && metric != "orders" {
							t.Errorf("unexpected metric = %v", metric)
						}
						measurement := "count"
						if posts == 5 {
							measurement = "sum_value"
							if metric != "orders" {
								t.Errorf("revenue metric = %v, want orders", metric)
							}
						}
						want := fmt.Sprintf(`{"data":{"attributes":{"by":["$message"],"filter":["greater-or-equal(datetime,%sT00:00:00)","less-than(datetime,%sT23:59:59)"],"interval":"day","measurements":[%q,"count"],"metric_id":%q,"page_size":500,"timezone":"UTC"},"type":"metric-aggregate"}}`, now.AddDate(0, 0, -30).Format("2006-01-02"), now.Format("2006-01-02"), measurement, metric)
						if string(body) != want || r.URL.RawQuery != "" || r.Header.Get("Content-Type") != "application/json" {
							t.Errorf("aggregate query = %q, content type = %q, body = %s; want %s", r.URL.RawQuery, r.Header.Get("Content-Type"), body, want)
						}
						if posts == failAt {
							w.WriteHeader(http.StatusInternalServerError)
							_, _ = io.WriteString(w, failure)
							return
						}
						_, _ = io.WriteString(w, `{"data":{"attributes":{"data":[{"dimensions":["email-one"],"measurements":{"count":[100],"sum_value":[250]}}]}}}`)
					default:
						t.Errorf("unexpected request: %s %s", r.Method, r.URL)
						http.NotFound(w, r)
					}
				}))
				fixPassClientEnvironment(t, baseURL)
				cmd := newFlowPathAnalysisCmd(&rootFlags{asJSON: true, noCache: true, rateLimit: 0})
				cmd.SilenceUsage = true
				var output bytes.Buffer
				cmd.SetOut(&output)
				cmd.SetErr(io.Discard)
				cmd.SetArgs([]string{"--id", "flow-one"})
				err := cmd.Execute()
				want := "POST /api/metric-aggregates returned HTTP 500: " + failure
				if err == nil || err.Error() != want || output.Len() != 0 || posts != failAt || flowReads != 1 || metricReads != failAt {
					t.Fatalf("error = %v, output = %q, aggregates = %d, flow reads = %d, metric reads = %d; want %q, no report, and stop at aggregate %d", err, output.String(), posts, flowReads, metricReads, want, failAt)
				}
			})
		}
	}
}

func TestRXRestoreRateLimitExhaustedNilSafe(t *testing.T) {
	for _, session := range []*platform.Session{nil, {}} {
		session.RestoreRateLimitExhausted(true)
		session.RestoreRateLimitExhausted(false)
		if got := session.RateLimitMetadata(); got != (platform.RateLimitMetadata{}) {
			t.Fatalf("uninitialized session metadata = %+v", got)
		}
	}
}

func TestRXReportRetriesRecordSessionAndReceiptTelemetry(t *testing.T) {
	for _, mode := range []string{"recorder", "server"} {
		for _, tc := range []struct {
			name               string
			failures           int
			retryAfter         string
			wantWait           time.Duration
			initiallyExhausted bool
		}{
			{"recovered", 1, "2", 2 * time.Second, false},
			{"full-retry-after", 1, "120", 2 * time.Minute, false},
			{"exhausted", 4, "2", 2 * time.Second, false},
			{"previously-exhausted", 1, "2", 2 * time.Second, true},
		} {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				const endpoint = "/api/campaign-values-reports/?page_cursor=page-2"
				const body = `{"data":{"type":"campaign-values-report","attributes":{"statistics":["recipients"],"timeframe":{"key":"last_7_days"}}}}`
				const response = `{"data":{"attributes":{"results":[{"recipients":100}]}},"links":{"next":null}}`
				calls := 0
				baseURL := rxHTTPFixture(t, mode, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if r.Method != http.MethodPost || r.URL.RequestURI() != endpoint {
						t.Errorf("request = %s %s, want POST %s", r.Method, r.URL, endpoint)
					}
					for key, want := range map[string]string{
						"Authorization": "Klaviyo-API-Key fixture-key", "revision": "2026-10-15",
						"Accept": "application/vnd.api+json", "Content-Type": "application/vnd.api+json",
						"Idempotency-Key": "", "X-Idempotency-Key": "",
					} {
						if got := r.Header.Get(key); got != want {
							t.Errorf("%s = %q, want %q", key, got, want)
						}
					}
					if got, err := io.ReadAll(r.Body); err != nil || string(got) != body {
						t.Errorf("body = %s, error = %v; want %s", got, err, body)
					}
					w.Header().Set("Content-Type", "application/vnd.api+json")
					if calls <= tc.failures {
						w.Header().Set("Retry-After", tc.retryAfter)
						w.Header().Set("X-Request-ID", "rx-report")
						w.WriteHeader(http.StatusTooManyRequests)
						return
					}
					_, _ = io.WriteString(w, response)
				}))
				for _, key := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"} {
					t.Setenv(key, t.TempDir())
				}
				session, err := platform.PrepareProfileSourceFromConfig(&platform.Profile{
					SchemaVersion: platform.ProfileSchemaVersion, Name: "review-rx",
					Sources: map[string]platform.SourceProfile{"klaviyo": {
						CredentialRef: "op://Test/Klaviyo/token", ExpectedAccountID: "acct-rx", ExpectedOrgName: "Test",
					}},
				}, "klaviyo-pp-cli", "klaviyo")
				if err != nil {
					t.Fatal(err)
				}
				session.GateOutcome = platform.GateVerified
				session.ObservedIdentity = map[string]string{"account_id": "acct-rx", "org_name": "Test"}
				session.CredentialFingerprint = "fixture-fingerprint"
				if tc.initiallyExhausted {
					session.RecordRateLimitExhausted()
				}
				c := client.New(&config.Config{BaseURL: baseURL, KlaviyoApiKey: "fixture-key"}, time.Second, 0)
				c.NoCache = true
				if err := c.BindPlatformSession(session); err != nil {
					t.Fatal(err)
				}
				if err := c.SetPlatformEndpointBudget(platform.EndpointBudget{
					Class: "POST /api/campaign-values-reports", Steady: 1000, Interval: time.Second,
					Burst: 10, MaxAttempts: 4, RetryBudget: time.Minute,
				}); err != nil {
					t.Fatal(err)
				}
				previousWait := waitForKlaviyoReportRetry
				t.Cleanup(func() { waitForKlaviyoReportRetry = previousWait })
				waits := 0
				waitForKlaviyoReportRetry = func(ctx context.Context, wait time.Duration) error {
					waits++
					metadata := session.RateLimitMetadata()
					if ctx != session.Context() || wait != tc.wantWait || metadata.Retries != waits || metadata.WaitMS < int64(waits)*tc.wantWait.Milliseconds() {
						t.Errorf("retry %d: wait = %v, metadata = %+v; want session context, wait %v and recorded retry", waits, wait, metadata, tc.wantWait)
					}
					return nil
				}
				raw, status, reportErr := postKlaviyoReport(c, endpoint, json.RawMessage(body))
				wantRetries, wantCalls := min(tc.failures, 3), min(tc.failures+1, 4)
				receiptStatus, exitCode := "success", 0
				if tc.failures == 4 {
					want := "rate limited: endpoint class POST /api/campaign-values-reports exhausted after 1 attempts; retry after 2s; request id rx-report"
					if reportErr == nil || reportErr.Error() != want || raw != nil || status != 429 || !session.RateLimitMetadata().Exhausted {
						t.Fatalf("response = %s, status = %d, error = %v; want exhausted error %q", raw, status, reportErr, want)
					}
					receiptStatus, exitCode = "error", 1
				} else if reportErr != nil || status != 200 || string(raw) != response {
					t.Fatalf("response = %s, status = %d, error = %v; want %s", raw, status, reportErr, response)
				}
				metadata := session.RateLimitMetadata()
				wantExhausted := tc.initiallyExhausted || tc.failures == 4
				if waits != wantRetries || calls != wantCalls || metadata.Requests != wantCalls || metadata.Retries != wantRetries || metadata.WaitMS < int64(wantRetries)*tc.wantWait.Milliseconds() || metadata.Exhausted != wantExhausted {
					t.Fatalf("calls = %d, waits = %d, metadata = %+v; want %d requests, %d retries, at least %v wait and exhausted=%t", calls, waits, metadata, wantCalls, wantRetries, time.Duration(wantRetries)*tc.wantWait, wantExhausted)
				}
				receiptFile := filepath.Join(t.TempDir(), "receipt.json")
				writer, err := platform.NewReceiptWriter(session, platform.ReceiptOptions{
					CLI: platform.ReceiptCLI{Name: "klaviyo-pp-cli"}, ReceiptFile: receiptFile,
				})
				if err != nil {
					t.Fatal(err)
				}
				if err := writer.Finalize(session.Context(), receiptStatus, exitCode, nil, session.OutputMetadata(), metadata); err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(receiptFile)
				if err != nil {
					t.Fatal(err)
				}
				var receipt platform.Receipt
				if err := json.Unmarshal(data, &receipt); err != nil || receipt.Status != receiptStatus || receipt.ExitCode != exitCode || receipt.RateLimit != metadata || receipt.RateLimit.Exhausted != wantExhausted {
					t.Fatalf("receipt = %s, error = %v; want status %s and rate metadata %+v", data, err, receiptStatus, metadata)
				}
			})
		}
	}
}
