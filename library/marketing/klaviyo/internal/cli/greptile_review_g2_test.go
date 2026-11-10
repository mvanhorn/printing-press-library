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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/store"
	"github.com/spf13/cobra"
)

func g2Root(t *testing.T, handler http.HandlerFunc) (*cobra.Command, *rootFlags) {
	t.Helper()
	for _, name := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"} {
		t.Setenv(name, t.TempDir())
	}
	t.Setenv("PRINTING_PRESS_CLIENT_PROFILE", "")
	t.Setenv(mcpBoundProfileEnv, "")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("revision"); got != "2026-10-15" {
			t.Errorf("revision = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Klaviyo-API-Key fixture-key" {
			t.Errorf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	previous := registeredPlatformSource
	t.Cleanup(func() { registeredPlatformSource = previous })
	registeredPlatformSource = &platformSourceRegistration{
		Source: "klaviyo", Adapter: klaviyoIdentityAdapter{baseURL: server.URL, httpClient: server.Client()},
		BindClient: func(c *client.Client, _ *platform.Session) error {
			c.BaseURL = server.URL
			c.HTTPClient = server.Client()
			c.Config.KlaviyoApiKey = "fixture-key"
			c.NoCache = true
			return c.SetPlatformEndpointBudget(platform.EndpointBudget{Class: "*", Steady: 10000, Interval: time.Second, Burst: 10000, MaxAttempts: 1, RetryBudget: time.Second})
		},
	}
	profile := &platform.Profile{SchemaVersion: platform.ProfileSchemaVersion, Name: "g2-tenant", Sources: map[string]platform.SourceProfile{
		"klaviyo": {CredentialRef: "op://Test/Klaviyo/key", ExpectedAccountID: "acct-g2", ExpectedOrgName: "G2"},
	}}
	session, err := platform.PrepareProfileSourceFromConfig(profile, "klaviyo-pp-cli", "klaviyo")
	if err != nil {
		t.Fatal(err)
	}
	session.GateOutcome, session.GateCheckedAt = platform.GateVerified, time.Now().UTC()
	session.ObservedIdentity = map[string]string{"account_id": "acct-g2", "org_name": "G2"}
	session.CredentialFingerprint = "fixture-fingerprint"
	flags := &rootFlags{platformSession: session}
	return newRootCmd(flags), flags
}

func g2NoAPI(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected API request: %s %s", r.Method, r.URL.RequestURI())
		http.Error(w, "unexpected request", http.StatusBadRequest)
	}
}

func TestG2UsefulLocalReportsPassThroughRoot(t *testing.T) {
	for _, tc := range []struct {
		command string
		metrics map[string]float64
	}{
		{"flow-cannibalization", map[string]float64{"total_collisions": 16}},
		{"send-fatigue", map[string]float64{"fatigued_profiles": 1, "fatigued_percentage": 100}},
		{"cohort", map[string]float64{"profiles": 1, "retained": 1, "retention_rate": 1}},
		{"flow-decay", map[string]float64{"open_rate": 0.1, "previous_open_rate": 0.5, "decay": 0.8}},
	} {
		t.Run(tc.command, func(t *testing.T) {
			root, flags := g2Root(t, g2NoAPI(t))
			st, err := store.OpenVerifiedWithContext(context.Background(), flags.platformSession.Paths.DataFile, flags.platformSession)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			for i := range 8 {
				data, err := json.Marshal(map[string]any{
					"profile_id": "profile-1", "metric_name": "Received Email",
					"datetime":   now.Add(-10*24*time.Hour + time.Duration(i)*time.Minute).Format(time.RFC3339),
					"attributes": map[string]any{"properties": map[string]any{"flow_id": fmt.Sprintf("flow-%d", i%2)}},
				})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := st.DB().Exec(`INSERT INTO events (id, data) VALUES (?, ?)`, fmt.Sprintf("event-%d", i), string(data)); err != nil {
					t.Fatal(err)
				}
			}
			older, err := json.Marshal(map[string]any{"profile_id": "profile-1", "metric_name": "Received Email", "datetime": now.AddDate(0, -2, 0).Format(time.RFC3339)})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := st.DB().Exec(`INSERT INTO events (id, data) VALUES (?, ?)`, "older-event", string(older)); err != nil {
				t.Fatal(err)
			}
			if _, err := st.DB().Exec(`INSERT INTO flows (id, data) VALUES ('flow-1', '{"open_rate":0.1,"previous_open_rate":0.5}')`); err != nil {
				t.Fatal(err)
			}
			if err := st.Close(); err != nil {
				t.Fatal(err)
			}
			receiptPath := filepath.Join(t.TempDir(), "receipt.json")
			var output, diagnostics bytes.Buffer
			root.SetOut(&output)
			root.SetErr(&diagnostics)
			root.SetArgs([]string{tc.command, "--db", flags.platformSession.Paths.DataFile, "--json", "--receipt-file", receiptPath})
			err = root.Execute()
			finalizePlatformInvocation(flags, &err)
			if err != nil {
				t.Fatalf("useful report rejected: %v; stdout=%s stderr=%s", err, &output, &diagnostics)
			}
			if strings.Contains(output.String(), "all_business_metrics_zero_or_null") {
				t.Fatalf("useful report hidden: %s", &output)
			}
			if flags.platformAnalytics == nil || len(flags.platformAnalytics.BusinessMetrics) != len(tc.metrics) {
				t.Fatalf("missing reviewed declaration: %+v", flags.platformAnalytics)
			}
			for name, want := range tc.metrics {
				got := flags.platformAnalytics.BusinessMetrics[name]
				if got == nil || *got != want {
					t.Errorf("declared %s = %v, want %v", name, got, want)
				}
			}
			if tc.command == "send-fatigue" || tc.command == "flow-cannibalization" {
				var envelope struct {
					Meta platform.OutputMetadata `json:"meta"`
				}
				if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
					t.Fatal(err)
				}
				window := envelope.Meta.ResolvedWindow
				if window == nil || window.Requested.Days != 30 || window.End.Sub(window.Start) != 30*24*time.Hour {
					t.Fatalf("default event history window = %+v, want 30 days", window)
				}
				data, err := os.ReadFile(receiptPath)
				if err != nil {
					t.Fatal(err)
				}
				var receipt platform.Receipt
				if err := json.Unmarshal(data, &receipt); err != nil {
					t.Fatal(err)
				}
				if receipt.Status != "success" || !reflect.DeepEqual(receipt.Window, window) {
					t.Fatalf("receipt window = %+v; output window = %+v", receipt.Window, window)
				}
			}
		})
	}
}

func TestG2OptimalSendTimeUsesActualEngagementMetricThroughRoot(t *testing.T) {
	posts := 0
	root, flags := g2Root(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /api/metrics":
			if r.URL.Query().Encode() != "fields%5Bmetric%5D=name" {
				t.Errorf("metrics query = %q", r.URL.RawQuery)
			}
			fmt.Fprint(w, `{"data":[{"id":"opened","attributes":{"name":"Opened Email"}},{"id":"received","attributes":{"name":"Received Email"}}],"links":{"next":null}}`)
		case "POST /api/metric-aggregates":
			posts++
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				return
			}
			metric := "opened"
			count := 5
			if posts == 2 {
				metric, count = "received", 10
			}
			now := time.Now().UTC()
			want := map[string]any{"data": map[string]any{"type": "metric-aggregate", "attributes": map[string]any{
				"metric_id": metric, "measurements": []string{"count"}, "interval": "hour", "page_size": 500,
				"timezone": "UTC", "filter": []string{
					"greater-or-equal(datetime," + now.Add(-90*24*time.Hour).Format("2006-01-02") + "T00:00:00)",
					"less-than(datetime," + now.Format("2006-01-02") + "T23:59:59)",
				},
			}}}
			wantJSON, _ := json.Marshal(want)
			var normalized map[string]any
			if err := json.Unmarshal(wantJSON, &normalized); err != nil || !reflect.DeepEqual(body, normalized) {
				t.Errorf("aggregate body = %#v, want %s", body, wantJSON)
			}
			fmt.Fprintf(w, `{"data":{"attributes":{"dates":[%q],"data":[{"measurements":{"count":[%d]}}]}}}`, now.Add(-24*time.Hour).Truncate(time.Hour).Format(time.RFC3339), count)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.RequestURI())
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	})
	var output, diagnostics bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&diagnostics)
	root.SetArgs([]string{"optimal-send-time", "--timezone", "UTC", "--json"})
	err := root.Execute()
	finalizePlatformInvocation(flags, &err)
	if err != nil {
		t.Fatalf("engagement report rejected: %v; %s %s", err, &output, &diagnostics)
	}
	if posts != 2 || !strings.Contains(output.String(), `"engagement_rate":0.5`) {
		t.Fatalf("posts=%d report=%s", posts, &output)
	}
	metric := flags.platformAnalytics.BusinessMetrics["engagement_rate"]
	if metric == nil || *metric != 0.5 {
		t.Fatalf("declared engagement_rate = %v", metric)
	}
}

type g2TerminalBuffer struct {
	bytes.Buffer
	terminal bool
}

func (w *g2TerminalBuffer) IsTerminal() bool { return w.terminal }

func TestG2RootResourceOutputPreservesTerminalStatus(t *testing.T) {
	for _, tc := range []struct {
		name     string
		terminal bool
		json     bool
		receipt  bool
	}{
		{"terminal", true, false, false},
		{"terminal with receipt", true, false, true},
		{"pipe", false, false, false},
		{"explicit JSON", true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, flags := g2Root(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/api/metrics" {
					t.Errorf("resource request = %s %s", r.Method, r.URL.RequestURI())
				}
				fmt.Fprint(w, `{"data":[{"id":"metric-1","type":"metric","attributes":{"name":"Opened Email"}}],"links":{"next":null}}`)
			})
			output := &g2TerminalBuffer{terminal: tc.terminal}
			var diagnostics bytes.Buffer
			root.SetOut(output)
			root.SetErr(&diagnostics)
			args := []string{"metrics", "get", "--all", "--data-source", "live"}
			if tc.json {
				args = append(args, "--json")
			}
			if tc.receipt {
				args = append(args, "--receipt-file", filepath.Join(t.TempDir(), "receipt.json"))
			}
			root.SetArgs(args)
			err := root.Execute()
			finalizePlatformInvocation(flags, &err)
			if err != nil {
				t.Fatalf("resource command: %v; %s", err, &diagnostics)
			}
			wantJSON := !tc.terminal || tc.json
			if json.Valid(output.Bytes()) != wantJSON || !strings.Contains(output.String(), "metric-1") {
				t.Fatalf("JSON=%v, want %v; output=%s", json.Valid(output.Bytes()), wantJSON, output)
			}
			if !wantJSON && !strings.Contains(output.String(), "ID") {
				t.Fatalf("missing table header: %s", output)
			}
		})
	}
}

func TestG2CampaignSendRecognizesNonEmailAndRequiresReadableEvidence(t *testing.T) {
	page := string(campaignMessageTestPage("message-1", "Campaign", "draft", ""))
	mixed := string(modelCollectionFixture(modelMessageFixture("message-1", "draft"), modelMessageFixture("email-1", "draft")))
	email := string(campaignMessageTestPage("email-1", "Campaign", "draft", ""))
	for _, tc := range []struct {
		name, page, channel, html, wantErr string
		templateReads                      int
	}{
		{"sms", page, "sms", "", "", 0},
		{"push", page, "push", "", "", 0},
		{"whatsapp", page, "whatsapp", "", "", 0},
		{"mobile_push", page, "mobile_push", "", "", 0},
		{"mixed valid", mixed, "sms", "{% unsubscribe %}", "", 1},
		{"mixed invalid", mixed, "sms", "hello", "No Klaviyo unsubscribe tag found", 1},
		{"no messages", `{"data":[]}`, "", "", "has no readable messages", 0},
		{"missing data", `{}`, "", "", "campaign collection data is missing", 0},
		{"null data", `{"data":null}`, "", "", "campaign collection data is missing", 0},
		{"missing next page data", `{"data":[{"id":"message-1"}],"links":{"next":"/api/campaigns/campaign-1/campaign-messages?page[cursor]=next"}}`, "", "", "campaign collection data is missing", 0},
		{"unreadable data", `{"data":`, "", "", "unexpected end of JSON input", 0},
		{"missing ID", `{"data":[{"attributes":{"definition":{"status":"draft"}}}]}`, "", "", "resource ID is missing", 0},
		{"missing status", `{"data":[{"id":"message-1"}]}`, "sms", "", "status is missing", 0},
		{"missing channel", page, "", "", "channel is missing", 0},
		{"unknown channel", page, "fax", "", "unsupported channel", 0},
		{"missing email template", email, "email", "", "template ID is missing", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts, templateReads := 0, 0
			campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.Method + " " + r.URL.Path {
				case "GET /api/campaigns/campaign-1":
					fmt.Fprint(w, `{"data":{"id":"campaign-1","attributes":{"definition":{"name":"Campaign"}}}}`)
				case "GET /api/campaigns/campaign-1/campaign-messages":
					if r.URL.Query().Get("page[cursor]") == "next" {
						fmt.Fprint(w, `{}`)
						return
					}
					fmt.Fprint(w, tc.page)
				case "GET /api/campaign-messages/message-1/campaign-audience", "GET /api/campaign-messages/email-1/campaign-audience":
					w.Write(modelAudienceFixture(nil, nil))
				case "GET /api/campaign-messages/message-1/campaign-variations":
					w.Write(campaignVariationTestPage("variation-message-1", "template", "Subject", tc.channel))
				case "GET /api/campaign-messages/email-1/campaign-variations":
					template := "template"
					if tc.html == "" {
						template = ""
					}
					w.Write(campaignVariationTestPage("variation-email-1", template, "Subject", "email"))
				case "GET /api/templates/template":
					templateReads++
					fmt.Fprintf(w, `{"data":{"id":"template","attributes":{"html":%q}}}`, tc.html)
				case "POST /api/campaign-send-jobs":
					posts++
					body, err := io.ReadAll(r.Body)
					if err != nil || string(body) != `{"data":{"id":"campaign-1","type":"campaign-send-job"}}` {
						t.Errorf("send body=%s err=%v", body, err)
					}
					fmt.Fprint(w, `{"data":{"id":"send-1","type":"campaign-send-job"}}`)
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
					http.NotFound(w, r)
				}
			}))
			body, err := os.CreateTemp(t.TempDir(), "send-body-*.json")
			if err != nil {
				t.Fatal(err)
			}
			defer body.Close()
			if _, err := body.WriteString(`{"data":{"id":"campaign-1","type":"campaign-send-job"}}`); err != nil {
				t.Fatal(err)
			}
			if _, err := body.Seek(0, 0); err != nil {
				t.Fatal(err)
			}
			previousStdin := os.Stdin
			os.Stdin = body
			t.Cleanup(func() { os.Stdin = previousStdin })
			var output, diagnostics bytes.Buffer
			cmd := newCampaignSendJobsSendCampaignCmd(&rootFlags{asJSON: true, noCache: true})
			cmd.SetOut(&output)
			cmd.SetErr(&diagnostics)
			cmd.SetArgs([]string{"--stdin"})
			err = cmd.Execute()
			if tc.wantErr == "" {
				if err != nil || posts != 1 || !strings.Contains(output.String(), "send-1") {
					t.Fatalf("send err=%v posts=%d stdout=%s stderr=%s", err, posts, &output, &diagnostics)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantErr) || posts != 0 {
				t.Fatalf("send err=%v want %q posts=%d", err, tc.wantErr, posts)
			}
			if templateReads != tc.templateReads {
				t.Errorf("template reads=%d want=%d", templateReads, tc.templateReads)
			}
		})
	}
}

func TestG2ReportRevenueAcceptsOrdersWithZeroRevenue(t *testing.T) {
	posts := 0
	root, flags := g2Root(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /api/metrics":
			fmt.Fprint(w, `{"data":[{"id":"orders","attributes":{"name":"Placed Order"}}],"links":{"next":null}}`)
		case "POST /api/metric-aggregates":
			posts++
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				return
			}
			wantJSON := `{"data":{"type":"metric-aggregate","attributes":{"metric_id":"orders","measurements":["sum_value","count","unique"],"interval":"month","page_size":500,"by":["$attributed_channel"],"filter":["greater-or-equal(datetime,2026-10-01T00:00:00)","less-than(datetime,2026-10-31T23:59:59)"],"timezone":"US/Central"}}}`
			var want map[string]any
			if err := json.Unmarshal([]byte(wantJSON), &want); err != nil || !reflect.DeepEqual(body, want) {
				t.Errorf("revenue request = %#v, want %s", body, wantJSON)
			}
			fmt.Fprint(w, `{"data":{"attributes":{"dates":["2026-10-01"],"data":[{"dimensions":["email"],"measurements":{"sum_value":[0],"count":[3],"unique":[2]}}]}}}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.RequestURI())
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	})
	var output, diagnostics bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&diagnostics)
	root.SetArgs([]string{"report", "revenue", "--by", "channel", "--since", "2026-10-01", "--until", "2026-10-31", "--json"})
	err := root.Execute()
	finalizePlatformInvocation(flags, &err)
	if err != nil || posts != 1 {
		t.Fatalf("orders report rejected: %v; posts=%d stdout=%s stderr=%s", err, posts, &output, &diagnostics)
	}
	orders := flags.platformAnalytics.BusinessMetrics["orders"]
	if orders == nil || *orders != 3 {
		t.Fatalf("declared orders = %v", orders)
	}
}

func TestG2RemainingCorrectedSpecsMatchProducedFields(t *testing.T) {
	for _, tc := range []struct {
		command string
		payload any
		want    map[string]float64
	}{
		{"revenue-per-email", revenuePerEmailRows(map[string]float64{"flow-1": 0}, map[string]float64{"flow-1": 10}, "flow_id"), map[string]float64{"revenue": 0, "emails_sent": 10, "revenue_per_email": 0}},
		{"subject-line-analysis", subjectRows([]subjectEmail{{Subject: "Hello", OpenRate: 0.4, ClickRate: 0.1}}, 1), map[string]float64{"open_rate": 0.4, "click_rate": 0.1}},
	} {
		t.Run(tc.command, func(t *testing.T) {
			metrics := collectPlatformDeclaredMetrics(tc.payload, platformAnalyticsSpecs[tc.command].BusinessMetrics)
			if len(metrics) != len(tc.want) {
				t.Fatalf("declared metrics = %+v, want %+v", metrics, tc.want)
			}
			for name, want := range tc.want {
				if metrics[name] == nil || *metrics[name] != want {
					t.Errorf("%s metric %s = %v, want %v", tc.command, name, metrics[name], want)
				}
			}
		})
	}
}

func TestG2ReceiptLookbacksIgnoreCountingWindows(t *testing.T) {
	for _, makeCommand := range []func(*rootFlags) *cobra.Command{newSendFatigueCmd, newFlowCannibalizationCmd} {
		cmd := makeCommand(&rootFlags{})
		if err := cmd.ParseFlags([]string{"--last", "45d", "--window", "2h"}); err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
		request, exclusive, ok := platformWindowRequestFromFlags(cmd, now, time.UTC, "UTC")
		if !ok || !exclusive || request.Days != 45 || request.Start != nil || request.End != nil {
			t.Fatalf("%s event lookback = %+v", cmd.Name(), request)
		}
	}
}
