package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/config"
)

func TestG5ReportsIncludeLaterPages(t *testing.T) {
	for _, report := range []string{"targeting", "revenue-per-email", "subject-engagement"} {
		t.Run(report, func(t *testing.T) {
			var bodies [][]byte
			var paths []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/vnd.api+json")
				if r.URL.Path == "/api/campaign-values-reports/" {
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
					}
					bodies = append(bodies, body)
					paths = append(paths, r.URL.RequestURI())
					if r.Method != http.MethodPost || r.Header.Get("revision") != "2026-10-15" || r.Header.Get("Authorization") != "Klaviyo-API-Key test-key" || r.Header.Get("Accept") != "application/vnd.api+json" || r.Header.Get("Content-Type") != "application/vnd.api+json" {
						t.Errorf("report request = %s, headers=%v", r.Method, r.Header)
					}
					if r.URL.Query().Get("page_cursor") == "page-2" {
						fmt.Fprint(w, `{"data":{"attributes":{"results":[{"groupings":{"campaign_id":"campaign-2"},"statistics":{"recipients":20,"conversion_value":80,"delivered":20,"opens_unique":8,"clicks_unique":2}}]}},"links":{"next":null}}`)
					} else {
						fmt.Fprintf(w, `{"data":{"type":"campaign-values-report","attributes":{"results":[{"groupings":{"campaign_id":"campaign-1"},"statistics":{"recipients":10,"conversion_value":10,"delivered":10,"opens_unique":1,"clicks_unique":1}}]}},"links":{"next":%q}}`, "http://"+r.Host+"/api/campaign-values-reports/?page_cursor=page-2")
					}
					return
				}
				fixPassAssertGET(t, r)
				switch r.URL.Path {
				case "/api/metrics":
					fmt.Fprint(w, `{"data":[{"id":"orders","attributes":{"name":"Placed Order","integration":{"name":"Shopify"}}},{"id":"received","attributes":{"name":"Received Email"}}],"links":{"next":null}}`)
				case "/api/campaign-messages":
					first, second := modelMessageFixture("campaign-1", "sent"), modelMessageFixture("campaign-2", "sent")
					anyPath(first, "attributes.definition").(map[string]any)["name"] = "First"
					anyPath(second, "attributes.definition").(map[string]any)["name"] = "Second"
					w.Write(modelCollectionFixture(first, second))
				case "/api/campaign-messages/campaign-1/campaign-audience":
					w.Write(modelAudienceFixture([]string{"segment-1"}, nil))
				case "/api/campaign-messages/campaign-2/campaign-audience":
					w.Write(modelAudienceFixture([]string{"segment-2"}, nil))
				case "/api/campaign-messages/campaign-1/campaign-variations", "/api/campaign-messages/campaign-2/campaign-variations":
					w.Write(campaignVariationTestPage("variation", "template", "Subject", "email"))
				case "/api/segments":
					fmt.Fprint(w, `{"data":[{"id":"segment-1","attributes":{"name":"First audience"}},{"id":"segment-2","attributes":{"name":"Second audience"}}],"links":{"next":null}}`)
				case "/api/lists":
					fmt.Fprint(w, `{"data":[],"links":{"next":null}}`)
				default:
					t.Errorf("unexpected request %s", r.URL.RequestURI())
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			fixPassClientEnvironment(t, server.URL)
			c := client.New(&config.Config{BaseURL: server.URL, KlaviyoApiKey: "test-key"}, time.Second, 0)
			c.NoCache = true
			since, until := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
			var wantBody string
			switch report {
			case "targeting":
				wantBody = `{"data":{"attributes":{"conversion_metric_id":"orders","statistics":["recipients","conversion_value"],"timeframe":{"key":"last_12_months"}},"type":"campaign-values-report"}}`
				cmd := newReportTargetingCmd(&rootFlags{asJSON: true, noCache: true})
				var output bytes.Buffer
				cmd.SetOut(&output)
				cmd.SetArgs([]string{"--conversion-metric-id", "orders"})
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
				var result map[string]any
				if err := json.Unmarshal(output.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				segments := mapSlice(result["segments"])
				if result["campaign_report_rows"] != float64(2) || len(segments) != 2 || result["segments_never_mailed"] != float64(0) || segments[0]["segment_id"] != "segment-2" || segments[0]["attributed_revenue"] != float64(80) {
					t.Fatalf("targeting omitted later campaign: %s", &output)
				}
			case "revenue-per-email":
				wantBody = `{"data":{"attributes":{"conversion_metric_id":"orders","statistics":["recipients","conversion_value"],"timeframe":{"end":"2026-11-09T00:00:00Z","start":"2026-11-01T00:00:00Z"}},"type":"campaign-values-report"}}`
				rows, err := campaignRevenuePerEmailRows(c, since, until)
				if err != nil {
					t.Fatal(err)
				}
				if len(rows) != 2 || rows[0]["campaign_id"] != "campaign-2" || rows[0]["emails_sent"] != int64(20) || rows[0]["revenue_per_email"] != float64(4) {
					t.Fatalf("revenue omitted later campaign: %#v", rows)
				}
			case "subject-engagement":
				wantBody = `{"data":{"attributes":{"conversion_metric_id":"received","statistics":["delivered","opens_unique","open_rate","clicks_unique","click_rate"],"timeframe":{"end":"2026-11-09T00:00:00Z","start":"2026-11-01T00:00:00Z"}},"type":"campaign-values-report"}}`
				rows, err := campaignSubjectEngagement(c, since, until)
				if err != nil {
					t.Fatal(err)
				}
				if len(rows) != 2 || rows["campaign-2"].OpenRate != 0.4 || rows["campaign-2"].ClickRate != 0.1 {
					t.Fatalf("engagement omitted later campaign: %#v", rows)
				}
			}
			if !reflect.DeepEqual(paths, []string{"/api/campaign-values-reports/", "/api/campaign-values-reports/?page_cursor=page-2"}) || len(bodies) != 2 {
				t.Fatalf("report requests = %v, bodies = %d", paths, len(bodies))
			}
			for _, body := range bodies {
				if string(body) != wantBody {
					t.Errorf("body = %s, want %s", body, wantBody)
				}
			}
		})
	}
}

func TestG5ReportPaginationWithoutHeaderOverrides(t *testing.T) {
	for _, test := range []struct {
		name      string
		path      string
		pages     []json.RawMessage
		wantError string
	}{
		{
			name: "merge results and preserve envelope",
			path: "/api/segment-values-reports/",
			pages: []json.RawMessage{
				json.RawMessage(`{"data":{"id":"report-1","attributes":{"results":[{"row":1}]}},"meta":{"retained":true},"links":{"self":"/api/segment-values-reports/","next":"/api/segment-values-reports/?page_cursor=next"}}`),
				json.RawMessage(`{"data":{"attributes":{"results":[{"row":2}]}},"links":{"next":null}}`),
			},
		},
		{
			name:      "repeated cursor",
			path:      "/api/segment-values-reports/?page_cursor=repeat",
			pages:     []json.RawMessage{json.RawMessage(`{"data":{"attributes":{"results":[]}},"links":{"next":"/api/segment-values-reports/?page_cursor=repeat"}}`)},
			wantError: `Klaviyo report pagination repeated page_cursor "repeat"`,
		},
		{
			name:      "different endpoint",
			path:      "/api/segment-values-reports/",
			pages:     []json.RawMessage{json.RawMessage(`{"data":{"attributes":{"results":[]}},"links":{"next":"/api/profiles/?page_cursor=next"}}`)},
			wantError: "Klaviyo report pagination returned an invalid next link",
		},
		{
			name: "missing results on later page",
			path: "/api/segment-values-reports/",
			pages: []json.RawMessage{
				json.RawMessage(`{"data":{"attributes":{"results":[]}},"links":{"next":"/api/segment-values-reports/?page_cursor=next"}}`),
				json.RawMessage(`{"data":{"attributes":{}},"links":{"next":null}}`),
			},
			wantError: "Klaviyo report response did not contain data.attributes.results",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := &reportTestClient{posts: test.pages}
			raw, status, err := postKlaviyoReport(c, test.path, map[string]any{"data": map[string]any{"type": "segment-values-report"}})
			if test.wantError != "" {
				if err == nil || err.Error() != test.wantError || raw != nil {
					t.Fatalf("response = %s, error = %v, want %s", raw, err, test.wantError)
				}
				return
			}
			if err != nil || status != 200 || len(c.posts) != 0 {
				t.Fatalf("status = %d, unread pages = %d, error = %v", status, len(c.posts), err)
			}
			rows, err := klaviyoReportRows(raw)
			var envelope map[string]any
			if jsonErr := json.Unmarshal(raw, &envelope); jsonErr != nil {
				t.Fatal(jsonErr)
			}
			if err != nil || len(rows) != 2 || rows[1]["row"] != float64(2) || anyPath(envelope, "data.id") != "report-1" || anyPath(envelope, "meta.retained") != true || anyPath(envelope, "links.self") != "/api/segment-values-reports/" || anyPath(envelope, "links.next") != nil {
				t.Fatalf("merged response = %s, rows error = %v", raw, err)
			}
		})
	}
}

func TestG5ReportRejectsRepeatedCursor(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"data":{"attributes":{"results":[]}},"links":{"next":"/api/flow-values-reports/?page_cursor=repeat"}}`)
	}))
	defer server.Close()
	c := client.New(&config.Config{BaseURL: server.URL, KlaviyoApiKey: "test-key"}, time.Second, 0)
	_, _, err := postKlaviyoReport(c, "/api/flow-values-reports/", map[string]any{"data": map[string]any{"type": "flow-values-report"}})
	if err == nil || err.Error() != `Klaviyo report pagination repeated page_cursor "repeat"` || calls != 2 {
		t.Fatalf("error = %v, requests = %d", err, calls)
	}
}

func TestG5QAGateChecksEveryEmailAndPage(t *testing.T) {
	for _, withHTML := range []bool{false, true} {
		t.Run(fmt.Sprintf("html-override-%t", withHTML), func(t *testing.T) {
			var paths []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fixPassAssertGET(t, r)
				paths = append(paths, r.URL.RequestURI())
				switch r.URL.RequestURI() {
				case "/api/campaigns/campaign-1":
					fmt.Fprint(w, `{"data":{"id":"campaign-1","type":"campaign"}}`)
				case "/api/campaigns/campaign-1/campaign-messages":
					fmt.Fprint(w, `{"data":[{"id":"message-1","attributes":{"definition":{"status":"draft"}}},{"id":"sms-1","attributes":{"definition":{"status":"draft"}}}],"links":{"next":"/api/campaigns/campaign-1/campaign-messages?page[cursor]=next"}}`)
				case "/api/campaigns/campaign-1/campaign-messages?page[cursor]=next":
					fmt.Fprint(w, `{"data":[{"id":"message-2","attributes":{"definition":{"status":"draft"}}},{"id":"message-3","attributes":{"definition":{"status":"draft"}}}],"links":{"next":null}}`)
				case "/api/campaign-messages/message-1/campaign-audience", "/api/campaign-messages/message-2/campaign-audience", "/api/campaign-messages/message-3/campaign-audience", "/api/campaign-messages/sms-1/campaign-audience":
					w.Write(modelAudienceFixture(nil, nil))
				case "/api/campaign-messages/message-1/campaign-variations", "/api/campaign-messages/message-2/campaign-variations", "/api/campaign-messages/message-3/campaign-variations", "/api/campaign-messages/sms-1/campaign-variations":
					id := strings.Split(r.URL.Path, "/")[3]
					channel := "email"
					if id == "sms-1" {
						channel = "sms"
					}
					w.Write(campaignVariationTestPage("variation-"+id, "template-"+id, "Subject", channel))
				case "/api/templates/template-message-1?additional-fields%5Btemplate%5D=definition&fields%5Btemplate%5D=definition%2Chtml", "/api/templates/template-message-3?additional-fields%5Btemplate%5D=definition&fields%5Btemplate%5D=definition%2Chtml":
					fmt.Fprint(w, `{"data":{"id":"valid","attributes":{"html":"<a href=\"{% unsubscribe_link %}\">Unsubscribe</a>"}}}`)
				case "/api/templates/template-message-2?additional-fields%5Btemplate%5D=definition&fields%5Btemplate%5D=definition%2Chtml":
					fmt.Fprint(w, `{"data":{"id":"broken","attributes":{"html":"<a href=\"{% unsubscribe %}\">Unsubscribe</a>"}}}`)
				default:
					t.Errorf("unexpected request %s", r.URL.RequestURI())
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			fixPassClientEnvironment(t, server.URL)
			cmd := newPlanQAGateCmd(&rootFlags{asJSON: true, noCache: true})
			args := []string{"--campaign-id", "campaign-1"}
			if withHTML {
				file := filepath.Join(t.TempDir(), "override.html")
				if err := os.WriteFile(file, []byte(`{% unsubscribe %}`), 0o600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--html", file)
			}
			cmd.SetArgs(args)
			var output, diagnostics bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&diagnostics)
			err := cmd.Execute()
			if err == nil || err.Error() != "campaign QA failed; see findings above" {
				t.Fatalf("error = %v, output = %s", err, &output)
			}
			var result struct {
				Verdict  string `json:"verdict"`
				Findings []struct {
					MessageID string `json:"message_id"`
					Check     string `json:"check"`
					Status    string `json:"status"`
					Detail    string `json:"detail"`
				} `json:"findings"`
			}
			if err := json.Unmarshal(output.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			checked := map[string]string{}
			for _, finding := range result.Findings {
				if finding.Check == "unsubscribe" && finding.MessageID != "" {
					checked[finding.MessageID] = finding.Status
					if finding.MessageID == "message-2" && finding.Detail != "The {% unsubscribe %} tag renders a complete link and cannot be used in href; use {% unsubscribe_link %} as the URL instead." {
						t.Errorf("failure detail = %q", finding.Detail)
					}
				}
			}
			if result.Verdict != "fail" || !reflect.DeepEqual(checked, map[string]string{"message-1": "pass", "message-2": "fail", "message-3": "pass"}) {
				t.Fatalf("campaign findings = %s", &output)
			}
			wantPaths := []string{"/api/campaigns/campaign-1", "/api/campaigns/campaign-1/campaign-messages", "/api/campaigns/campaign-1/campaign-messages?page[cursor]=next"}
			for _, id := range []string{"message-1", "sms-1", "message-2", "message-3"} {
				wantPaths = append(wantPaths, "/api/campaign-messages/"+id+"/campaign-audience", "/api/campaign-messages/"+id+"/campaign-variations")
			}
			for _, id := range []string{"message-1", "message-2", "message-3"} {
				wantPaths = append(wantPaths, "/api/templates/template-"+id+"?additional-fields%5Btemplate%5D=definition&fields%5Btemplate%5D=definition%2Chtml")
			}
			if !reflect.DeepEqual(paths, wantPaths) {
				t.Fatalf("requests = %v, want %v", paths, wantPaths)
			}
		})
	}
}
