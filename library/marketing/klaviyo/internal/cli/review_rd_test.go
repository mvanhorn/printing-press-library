// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/store"
	"github.com/spf13/cobra"
)

func TestReviewRDLocalAnalyticsValidationAndWriter(t *testing.T) {
	for _, jsonOutput := range []bool{false, true} {
		for _, mode := range []string{"summary", "count", "group"} {
			for _, populated := range []bool{false, true} {
				t.Run(fmt.Sprintf("json=%v/%s/populated=%v", jsonOutput, mode, populated), func(t *testing.T) {
					session := platformCommandTestSession(t)
					dbPath := filepath.Join(t.TempDir(), "local.db")
					db, err := store.OpenVerifiedWithContext(context.Background(), dbPath, session)
					if err != nil {
						t.Fatal(err)
					}
					if populated {
						if err := db.Upsert("events", "one", json.RawMessage(`{"id":"one","channel":"email"}`)); err != nil {
							t.Fatal(err)
						}
					}
					if err := db.Close(); err != nil {
						t.Fatal(err)
					}
					receiptPath := filepath.Join(t.TempDir(), "receipt.json")
					writer, err := platform.NewReceiptWriter(session, platform.ReceiptOptions{CLI: platform.ReceiptCLI{Name: "klaviyo-pp-cli", Version: "test"}, ReceiptFile: receiptPath})
					if err != nil {
						t.Fatal(err)
					}
					flags := &rootFlags{asJSON: jsonOutput, platformSession: session, receiptWriter: writer}
					root := &cobra.Command{Use: "klaviyo-pp-cli", SilenceErrors: true, SilenceUsage: true}
					root.AddCommand(newAnalyticsCmd(flags))
					wrapPlatformCommandOutputs(root, flags)
					var out, diagnostics bytes.Buffer
					flags.platformMetadataWriter = &diagnostics
					root.SetOut(writer.CaptureWriter("stdout", &out))
					root.SetErr(&diagnostics)
					args := []string{"analytics", "--db", dbPath}
					if mode != "summary" {
						args = append(args, "--type", "events")
					}
					if mode == "group" {
						args = append(args, "--group-by", "channel")
					}
					root.SetArgs(args)
					err = root.Execute()
					finalizePlatformInvocation(flags, &err)
					status := "success"
					if !populated {
						if err == nil || err.Error() != "analytics result rejected: required_input_below_minimum" {
							t.Fatalf("error = %v; want exact minimum-input rejection", err)
						}
						status = "error"
						if strings.Contains(out.String(), `"count": 0`) || strings.Contains(out.String(), "0 records") {
							t.Fatalf("rejected result was emitted: %s", out.String())
						}
						if jsonOutput && !strings.Contains(out.String(), `"reason":"required_input_below_minimum"`) {
							t.Fatalf("missing structured failure: %s", out.String())
						}
					} else if err != nil || out.Len() == 0 {
						t.Fatalf("populated output=%q error=%v; want output through Cobra writer", out.String(), err)
					}
					raw, err := os.ReadFile(receiptPath)
					if err != nil {
						t.Fatal(err)
					}
					var receipt platform.Receipt
					if err := json.Unmarshal(raw, &receipt); err != nil || receipt.Status != status {
						t.Fatalf("receipt status=%q error=%v; want %q", receipt.Status, err, status)
					}
					if !populated && receipt.ExitCode == 0 {
						t.Fatal("rejected analytics must produce a failure receipt")
					}
				})
			}
		}
	}
}

func TestReviewRDLocalAnalyticsRejectsBeforeOutputWithoutWrapper(t *testing.T) {
	flags := &rootFlags{asJSON: true, platformSession: platformCommandTestSession(t)}
	cmd := newAnalyticsCmd(flags)
	cmd.SilenceUsage = true
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--db", filepath.Join(t.TempDir(), "empty.db"), "--type", "events"})
	err := cmd.Execute()
	if err == nil || err.Error() != "analytics result rejected: required_input_below_minimum" || out.Len() != 0 {
		t.Fatalf("output=%q error=%v; want rejection before any result", out.String(), err)
	}
}

func TestReviewRDProviderClampScope(t *testing.T) {
	for _, name := range []string{"send-fatigue", "flow-cannibalization", "content-fatigue", "segment-velocity", "forms", "signup-sources", "subject-line-analysis", "open-rates"} {
		t.Run(name, func(t *testing.T) {
			flags := &rootFlags{platformSession: platformCommandTestSession(t)}
			cmd := &cobra.Command{Use: name}
			cmd.Flags().String("last", "730d", "lookback")
			var warnings bytes.Buffer
			cmd.SetErr(&warnings)
			if err := adoptPlatformCommandWindow(cmd, flags); err != nil {
				t.Fatal(err)
			}
			window := flags.platformSession.OutputMetadata().ResolvedWindow
			wantClamp := name == "subject-line-analysis" || name == "open-rates"
			wantDays := 730
			if wantClamp {
				wantDays = 364
			}
			if window == nil || window.Clamped != wantClamp || window.Requested.Days != 730 || window.End.Sub(window.Start) != time.Duration(wantDays)*24*time.Hour {
				t.Fatalf("window=%+v; want %d days clamped=%v", window, wantDays, wantClamp)
			}
			if !wantClamp && (window.ProviderMaximum != "" || warnings.Len() != 0) {
				t.Fatalf("unrelated provider limit leaked into window=%+v warnings=%q", window, warnings.String())
			}
		})
	}
}

func TestReviewRDCampaignOnlyAnalysisDoesNotUseAggregateClamp(t *testing.T) {
	for _, name := range []string{"subject-line-analysis", "revenue-per-email"} {
		t.Run(name, func(t *testing.T) {
			flags := &rootFlags{platformSession: platformCommandTestSession(t)}
			cmd := &cobra.Command{Use: name}
			cmd.Flags().String("last", "730d", "lookback")
			cmd.Flags().String("source", "campaign", "source")
			cmd.Flags().String("by", "campaign", "scope")
			if err := adoptPlatformCommandWindow(cmd, flags); err != nil {
				t.Fatal(err)
			}
			window := flags.platformSession.OutputMetadata().ResolvedWindow
			if window == nil || window.Clamped || window.ProviderMaximum != "" || window.End.Sub(window.Start) != 730*24*time.Hour {
				t.Fatalf("campaign-only report must not inherit metric-aggregate limit: %+v", window)
			}
		})
	}
}

func TestReviewRDClientAndWindowUsesResolvedBounds(t *testing.T) {
	previous := registeredPlatformSource
	registeredPlatformSource = nil
	t.Cleanup(func() { registeredPlatformSource = previous })
	session := platformCommandTestSession(t)
	start := time.Date(2025, 11, 10, 12, 0, 0, 0, time.UTC)
	end := start.Add(364 * 24 * time.Hour)
	session.SetResolvedWindow(platform.ResolvedWindow{Start: start, End: end, Clamped: true})
	_, since, until, err := clientAndWindow(&rootFlags{platformSession: session}, "730d")
	if err != nil || !since.Equal(start) || !until.Equal(end) {
		t.Fatalf("window=%s..%s error=%v; want exact resolved bounds %s..%s", since, until, err, start, end)
	}
}

func TestReviewRDRevenueRequestUsesClampedWindow(t *testing.T) {
	start := time.Date(2025, 11, 10, 12, 30, 0, 0, time.UTC)
	end := start.Add(364 * 24 * time.Hour)
	posts := 0
	flowLookups := 0
	_, flags := g2Root(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /api/metrics":
			fmt.Fprint(w, `{"data":[{"id":"orders","attributes":{"name":"Placed Order"}}],"links":{"next":null}}`)
		case "GET /api/flows":
			flowLookups++
			if !reflect.DeepEqual(r.URL.Query(), url.Values{"fields[flow]": {"name"}}) {
				t.Errorf("flow-name lookup query=%v; want fields[flow]=name", r.URL.Query())
			}
			fmt.Fprint(w, `{"data":[{"id":"flow-one","type":"flow","attributes":{"name":"Welcome"}}],"links":{"next":null}}`)
		case "POST /api/metric-aggregates":
			posts++
			var got, want any
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Error(err)
			}
			wantJSON := `{"data":{"type":"metric-aggregate","attributes":{"metric_id":"orders","measurements":["sum_value","count","unique"],"interval":"month","page_size":500,"by":["$attributed_flow"],"filter":["greater-or-equal(datetime,2025-11-10T12:30:00)","less-than(datetime,2026-11-09T12:30:00)"],"timezone":"UTC"}}}`
			if err := json.Unmarshal([]byte(wantJSON), &want); err != nil {
				t.Error(err)
			}
			if !reflect.DeepEqual(got, want) || len(r.URL.Query()) != 0 {
				t.Errorf("query=%v body=%v; want exact clamped body %v", r.URL.Query(), got, want)
			}
			fmt.Fprint(w, `{"data":{"attributes":{"dates":["2026-11-09"],"data":[{"dimensions":["flow-one"],"measurements":{"sum_value":[12],"count":[2],"unique":[1]}}]}}}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.RequestURI())
			http.NotFound(w, r)
		}
	})
	flags.asJSON = true
	flags.platformSession.SetResolvedWindow(platform.ResolvedWindow{Start: start, End: end, Timezone: "UTC", Clamped: true})
	cmd := newReportRevenueCmd(flags)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--since", "2024-01-01", "--until", "2026-11-09"})
	if err := cmd.Execute(); err != nil || posts != 1 || flowLookups != 1 {
		t.Fatalf("posts=%d flow lookups=%d error=%v; want one aggregate using resolved bounds and one flow-name lookup", posts, flowLookups, err)
	}
}

func TestReviewRDMCPWindowDoesNotAssumeAggregateEndpoint(t *testing.T) {
	session := platformCommandTestSession(t)
	if err := AdoptMCPOutputSemantics(session, map[string]any{"last": "730d"}); err != nil {
		t.Fatal(err)
	}
	window := session.OutputMetadata().ResolvedWindow
	if window == nil || window.Clamped || window.ProviderMaximum != "" || window.End.Sub(window.Start) != 730*24*time.Hour {
		t.Fatalf("generic MCP arguments cannot select an endpoint-specific limit: %+v", window)
	}
}

func TestReviewRDSendFatigueUsesResolvedBounds(t *testing.T) {
	session := platformCommandTestSession(t)
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	session.SetResolvedWindow(platform.ResolvedWindow{Start: start, End: end, StartInclusive: true, EndExclusive: true, Timezone: "UTC", Clamped: true})
	dbPath := filepath.Join(t.TempDir(), "events.db")
	db, err := store.OpenVerifiedWithContext(context.Background(), dbPath, session)
	if err != nil {
		t.Fatal(err)
	}
	for i, at := range []time.Time{start.Add(-time.Second), start, end.Add(-time.Second), end, end.Add(time.Hour)} {
		body, err := json.Marshal(map[string]any{"attributes": map[string]any{"metric_name": "Received Email", "profile_id": "profile-one", "datetime": at.Format(time.RFC3339)}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB().Exec(`INSERT INTO events (id, data, synced_at) VALUES (?, ?, ?)`, fmt.Sprint(i), string(body), time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	cmd := newSendFatigueCmd(&rootFlags{asJSON: true, platformSession: session})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--db", dbPath, "--last", "730d", "--threshold", "1"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(platformTestOutputPayload(t, out.Bytes()), &result); err != nil {
		t.Fatal(err)
	}
	if got := anyPath(result, "top_offenders"); got == nil {
		t.Fatalf("missing offenders: %s", out.String())
	}
	offenders := result["top_offenders"].([]any)
	if len(offenders) != 1 || anyPath(offenders[0], "emails_received") != float64(2) {
		t.Fatalf("offenders=%v; only start-inclusive/end-exclusive events should count", offenders)
	}
}

func TestReviewRDSubjectAnalysisPropagatesFlowAggregateFailure(t *testing.T) {
	t.Setenv("KLAVIYO_TIMEZONE", "UTC")
	for metricIndex, failedMetric := range []string{"received", "opened", "clicked"} {
		for dimensionIndex, failedDimension := range []string{"$message", "$flow"} {
			t.Run(failedMetric+"/"+failedDimension, func(t *testing.T) {
				posts := 0
				campaignReports := 0
				aggregateFailed := false
				now := time.Now().UTC()
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/vnd.api+json")
					if r.Header.Get("revision") != "2026-04-15" || r.Header.Get("Authorization") != "Klaviyo-API-Key test-key" {
						t.Error("request must retain revision and synthetic auth headers")
					}
					switch r.Method + " " + r.URL.Path {
					case "GET /api/metrics":
						fmt.Fprint(w, `{"data":[{"id":"received","attributes":{"name":"Received Email"}},{"id":"opened","attributes":{"name":"Opened Email"}},{"id":"clicked","attributes":{"name":"Clicked Email"}}],"links":{"next":null}}`)
					case "POST /api/campaign-values-reports/":
						campaignReports++
						fmt.Fprint(w, `{"data":{"attributes":{"results":[{"groupings":{"campaign_id":"campaign-one"},"statistics":{"delivered":100,"opens_unique":50,"clicks_unique":10}}]}}}`)
					case "GET /api/flows":
						fmt.Fprint(w, `{"data":[{"id":"flow-one","type":"flow","attributes":{"name":"Welcome","status":"live"},"relationships":{"flow-actions":{"data":[{"id":"action-one","type":"flow-action"}]}}}],"included":[{"id":"action-one","type":"flow-action","attributes":{"definition":{"data":{"message":{"id":"flow-message","subject_line":"Welcome!"}}}},"relationships":{"flow":{"data":{"id":"flow-one","type":"flow"}}}}],"links":{"next":null}}`)
					case "GET /api/campaigns":
						fmt.Fprintf(w, `{"data":[{"id":"campaign-one","attributes":{"name":"Launch","status":"sent","send_time":%q},"relationships":{"campaign-messages":{"data":[{"id":"campaign-message"}]}}}],"included":[{"id":"campaign-message","attributes":{"definition":{"content":{"subject":"Launch!"}}}}],"links":{"next":null}}`, time.Now().Add(-time.Hour).UTC().Format(time.RFC3339))
					case "POST /api/metric-aggregates":
						posts++
						var body map[string]any
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Error(err)
						}
						metric := anyPath(body, "data.attributes.metric_id")
						by := anyPath(body, "data.attributes.by").([]any)[0]
						wantBody := map[string]any{"data": map[string]any{"type": "metric-aggregate", "attributes": map[string]any{
							"metric_id": metric, "measurements": []any{"count", "count"}, "by": []any{by},
							"interval": "day", "page_size": float64(500), "timezone": "UTC",
							"filter": []any{"greater-or-equal(datetime," + now.AddDate(0, 0, -90).Format("2006-01-02") + "T00:00:00)", "less-than(datetime," + now.Format("2006-01-02") + "T23:59:59)"},
						}}}
						if !reflect.DeepEqual(body, wantBody) || len(r.URL.Query()) != 0 || r.Header.Get("Content-Type") != "application/json" {
							t.Errorf("aggregate request query=%v body=%v; want exact JSON:API body %v", r.URL.Query(), body, wantBody)
						}
						if metric == failedMetric && by == failedDimension {
							aggregateFailed = true
							w.WriteHeader(http.StatusInternalServerError)
							fmt.Fprint(w, `{"errors":[{"detail":"flow aggregation unavailable"}]}`)
							return
						}
						fmt.Fprint(w, `{"data":{"attributes":{"data":[{"dimensions":["flow-message"],"measurements":{"count":[100]}}]}}}`)
					default:
						t.Errorf("unexpected request: %s %s", r.Method, r.URL.RequestURI())
						http.NotFound(w, r)
					}
				}))
				defer server.Close()
				fixPassClientEnvironment(t, server.URL)
				cmd := newSubjectLineAnalysisCmd(&rootFlags{asJSON: true, noCache: true, rateLimit: 0})
				cmd.SilenceUsage = true
				var out bytes.Buffer
				cmd.SetOut(&out)
				cmd.SetErr(io.Discard)
				cmd.SetArgs([]string{"--source", "all"})
				err := cmd.Execute()
				want := `POST /api/metric-aggregates returned HTTP 500: {"errors":[{"detail":"flow aggregation unavailable"}]}`
				wantPosts := dimensionIndex*3 + metricIndex + 1
				if err == nil || err.Error() != want || out.Len() != 0 || !aggregateFailed || campaignReports != 1 || posts != wantPosts {
					t.Fatalf("output=%q error=%v posts=%d want=%d campaign reports=%d aggregate failed=%v; want exact aggregation error with no successful result", out.String(), err, posts, wantPosts, campaignReports, aggregateFailed)
				}
			})
		}
	}
}

type reviewRDMetricFailureClient struct {
	*reportTestClient
}

func (c *reviewRDMetricFailureClient) Post(_ string, _ any) (json.RawMessage, int, error) {
	return nil, 500, &client.APIError{Method: "POST", Path: "/api/metric-aggregates", StatusCode: 500, Body: `{"errors":[{"detail":"flow aggregation unavailable"}]}`}
}

func TestReviewRDFlowAggregationFailureCannotBecomeMetrics(t *testing.T) {
	c := &reviewRDMetricFailureClient{&reportTestClient{gets: []json.RawMessage{
		metricListResponse("received", "Received Email"), metricListResponse("opened", "Opened Email"), metricListResponse("clicked", "Clicked Email"),
	}}}
	emails := []subjectEmail{{ID: "flow-message", SourceType: "flow", OpenRate: 0.5}}
	err := applyFlowSubjectEngagement(c, emails, time.Now().AddDate(0, 0, -30), time.Now())
	want := `POST /api/metric-aggregates returned HTTP 500: {"errors":[{"detail":"flow aggregation unavailable"}]}`
	if err == nil || err.Error() != want || emails[0].OpenRate != 0.5 {
		t.Fatalf("error=%v emails=%v; want exact upstream error and unchanged measurements", err, emails)
	}
}

func TestReviewRDQAGateRequestsSupportedTemplateFields(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixPassAssertGET(t, r)
		paths = append(paths, r.URL.RequestURI())
		switch r.URL.Path {
		case "/api/campaigns/campaign-one":
			fmt.Fprint(w, `{"data":{"id":"campaign-one"}}`)
		case "/api/campaigns/campaign-one/campaign-messages":
			fmt.Fprint(w, `{"data":[{"id":"message-one","attributes":{"definition":{"channel":"email"}}}]}`)
		case "/api/campaign-messages/message-one/template":
			if !reflect.DeepEqual(r.URL.Query(), url.Values{"fields[template]": {"html"}}) {
				t.Errorf("query=%v; want only supported html field", r.URL.Query())
			}
			fmt.Fprint(w, `{"data":{"id":"template-one","attributes":{"html":"<p>No unsubscribe link</p>"}}}`)
		default:
			t.Errorf("unexpected path %s", r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	fixPassClientEnvironment(t, server.URL)
	cmd := newPlanQAGateCmd(&rootFlags{asJSON: true, noCache: true})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--campaign-id", "campaign-one"})
	err := cmd.Execute()
	if err == nil || err.Error() != "campaign QA failed; see findings above" || len(paths) != 3 {
		t.Fatalf("paths=%v error=%v; want fetched HTML to fail QA", paths, err)
	}
}

func TestReviewRDTemplateAuditResolvesReferencesWithSupportedFields(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixPassAssertGET(t, r)
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/api/flows":
			fmt.Fprint(w, `{"data":[],"included":[{"attributes":{"definition":{"data":{"message":{"template_id":"flow-template"}}}}}],"links":{"next":null}}`)
		case "/api/campaigns":
			want := url.Values{"fields[campaign]": {"name,status"}, "fields[campaign-message]": {"definition"}, "filter": {`equals(messages.channel,"email")`}, "include": {"campaign-messages"}, "page[size]": {"100"}}
			if !reflect.DeepEqual(r.URL.Query(), want) {
				t.Errorf("query=%v; want %v", r.URL.Query(), want)
			}
			fmt.Fprint(w, `{"data":[],"included":[{"id":"message-one","type":"campaign-message","attributes":{"definition":{"channel":"email"}}}],"links":{"next":null}}`)
		case "/api/campaign-messages/message-one/template":
			if !reflect.DeepEqual(r.URL.Query(), url.Values{"fields[template]": {"id"}}) {
				t.Errorf("template query=%v; want id only", r.URL.Query())
			}
			fmt.Fprint(w, `{"data":{"id":"campaign-template","type":"template"}}`)
		default:
			t.Errorf("unexpected path %s", r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c := client.New(&config.Config{BaseURL: server.URL, KlaviyoApiKey: "test-key"}, 5*time.Second, 0)
	c.NoCache = true
	refs, err := collectReferencedTemplateIDs(c)
	if err != nil || !reflect.DeepEqual(refs, map[string]bool{"flow-template": true, "campaign-template": true}) {
		t.Fatalf("refs=%v error=%v; want both referenced templates", refs, err)
	}
	if !reflect.DeepEqual(paths, []string{"/api/flows", "/api/campaigns", "/api/campaign-messages/message-one/template"}) {
		t.Fatalf("paths=%v", paths)
	}
}

func TestReviewRDMessageTemplateInspectionUsesSupportedFields(t *testing.T) {
	for _, scope := range []string{"flow", "campaign"} {
		t.Run(scope, func(t *testing.T) {
			var paths []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fixPassAssertGET(t, r)
				paths = append(paths, r.URL.Path)
				want := url.Values{"page[size]": {"50"}}
				switch r.URL.Path {
				case "/api/flows":
					want.Set("fields[flow]", "name")
					fmt.Fprint(w, `{"data":[{"id":"parent-one"}],"links":{"next":null}}`)
				case "/api/campaigns":
					want.Set("fields[campaign]", "name,status")
					fmt.Fprint(w, `{"data":[{"id":"parent-one"}],"links":{"next":null}}`)
				case "/api/flows/parent-one/flow-actions":
					want.Set("fields[flow-action]", "id")
					fmt.Fprint(w, `{"data":[{"id":"action-one"}],"links":{"next":null}}`)
				case "/api/flow-actions/action-one/flow-messages":
					want.Set("fields[flow-message]", "id")
					fmt.Fprint(w, `{"data":[{"id":"message-one"}],"links":{"next":null}}`)
				case "/api/campaigns/parent-one/campaign-messages":
					want.Set("fields[campaign-message]", "id")
					fmt.Fprint(w, `{"data":[{"id":"message-one"}],"links":{"next":null}}`)
				case "/api/flow-messages/message-one/template", "/api/campaign-messages/message-one/template":
					want = url.Values{"fields[template]": {"name,html"}}
					fmt.Fprint(w, `{"data":{"id":"template-one","attributes":{"name":"Welcome","html":"<p>Welcome</p>"}}}`)
				default:
					t.Errorf("unexpected request %s", r.URL.RequestURI())
					http.NotFound(w, r)
				}
				if !reflect.DeepEqual(r.URL.Query(), want) {
					t.Errorf("query=%v; want %v", r.URL.Query(), want)
				}
			}))
			defer server.Close()
			c := client.New(&config.Config{BaseURL: server.URL, KlaviyoApiKey: "test-key"}, 5*time.Second, 0)
			c.NoCache = true
			rows, err := collectMessageTemplates(c, scope)
			if err != nil || len(rows) != 1 || rows[0]["id"] != "template-one" || rows[0]["message_id"] != "message-one" || rows[0]["parent_id"] != "parent-one" {
				t.Fatalf("templates=%v error=%v", rows, err)
			}
			wantPaths := []string{"/api/campaigns", "/api/campaigns/parent-one/campaign-messages", "/api/campaign-messages/message-one/template"}
			if scope == "flow" {
				wantPaths = []string{"/api/flows", "/api/flows/parent-one/flow-actions", "/api/flow-actions/action-one/flow-messages", "/api/flow-messages/message-one/template"}
			}
			if !reflect.DeepEqual(paths, wantPaths) {
				t.Fatalf("paths=%v; want %v", paths, wantPaths)
			}
		})
	}
}
