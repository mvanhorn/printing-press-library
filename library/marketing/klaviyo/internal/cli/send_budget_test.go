// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/config"
)

const sendBudgetBillingFixture = `{"data":[
	{"id":"sms_credits","attributes":{"usage_used":23.6766,"usage_max":135,"unit":"USD"}},
	{"id":"email_sends","attributes":{"usage_used":415344.0,"usage_max":1450000,"unit":"messages","period_start":"2026-10-19T17:01:00Z","period_end":"2026-11-19T17:01:00Z","as_of":"2026-11-09T20:55:09.350757Z"}}
],"links":{"next":null}}`

type sendBudgetTestTransport func(*http.Request) (*http.Response, error)

func (f sendBudgetTestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func sendBudgetTestClient(t *testing.T, handler http.HandlerFunc) *client.Client {
	t.Helper()
	c := client.New(&config.Config{BaseURL: "https://a.klaviyo.com", KlaviyoApiKey: "synthetic"}, time.Second, 0)
	c.HTTPClient.Transport = sendBudgetTestTransport(func(r *http.Request) (*http.Response, error) {
		if err := r.Context().Err(); err != nil {
			return nil, err
		}
		w := httptest.NewRecorder()
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("revision") != "2026-10-15" {
			t.Errorf("revision = %q", r.Header.Get("revision"))
		}
		handler(w, r)
		if err := r.Context().Err(); err != nil {
			return nil, err
		}
		response := w.Result()
		response.Request = r
		return response, nil
	})
	c.NoCache = true
	return c
}

func sendBudgetTestUsage(t *testing.T, c *client.Client) sendBudgetUsage {
	t.Helper()
	usage, err := readSendBudgetUsage(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	return usage
}

func TestSendBudgetBillingParse(t *testing.T) {
	c := sendBudgetTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/billing-usage" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		fmt.Fprint(w, sendBudgetBillingFixture)
	})
	usage := sendBudgetTestUsage(t, c)
	if *usage.Used != 415344 || *usage.Cap != 1450000 || usage.Start.Day() != 19 || usage.End.Month() != time.November || usage.AsOf.Nanosecond() != 350757000 {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestSendBudgetBillingRejectsMissingCapAndInvalidPeriod(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"missing row", `{"data":[{"id":"active_profiles"}]}`, "plan has no email cap via API"},
		{"missing cap", strings.Replace(sendBudgetBillingFixture, `"usage_max":1450000`, `"usage_max":null`, 1), "plan has no email cap via API"},
		{"missing used", strings.Replace(sendBudgetBillingFixture, `"usage_used":415344.0`, `"usage_used":null`, 1), "invalid email_sends"},
		{"invalid period", strings.Replace(sendBudgetBillingFixture, "2026-11-19T17:01:00Z", "2026-10-18T17:01:00Z", 1), "invalid email_sends"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := sendBudgetTestClient(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tc.body) })
			_, err := readSendBudgetUsage(context.Background(), c)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestSendBudgetScheduledPaginationChannelsAndRefresh(t *testing.T) {
	var mu sync.Mutex
	var requests []string
	polls, estimateReads := 0, 0
	c := sendBudgetTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		requests = append(requests, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/api/billing-usage":
			fmt.Fprint(w, sendBudgetBillingFixture)
		case "/api/campaign-messages":
			if r.URL.Query().Get("filter") != "equals(status,'scheduled')" {
				t.Errorf("filter = %q", r.URL.Query().Get("filter"))
			}
			if r.URL.Query().Get("page[cursor]") == "second" {
				fmt.Fprint(w, `{"data":[{"id":"bfcm","attributes":{"definition":{"name":"BFCM email","send_options":{"send_time":"2026-11-19T17:01:00Z"}}}},{"id":"sms","attributes":{"definition":{"name":"SMS","send_options":{"send_time":"2026-11-10T17:01:00Z"}}}}],"links":{"next":null}}`)
			} else {
				fmt.Fprint(w, `{"data":[{"id":"current","attributes":{"definition":{"name":"Current email","send_options":{"send_time":"2026-11-19T17:00:59Z"}}}}],"links":{"next":"https://a.klaviyo.com/api/campaign-messages?page%5Bcursor%5D=second"}}`)
			}
		case "/api/campaign-messages/current/campaign-variations":
			// Email is on the second variation page; count the message only once.
			if r.URL.Query().Get("page[cursor]") == "email" {
				fmt.Fprint(w, `{"data":[{"attributes":{"definition":{"details":{"channel":"email"}}}},{"attributes":{"definition":{"details":{"channel":"email"}}}}],"links":{"next":null}}`)
			} else {
				fmt.Fprint(w, `{"data":[{"attributes":{"definition":{"details":{"channel":"sms"}}}}],"links":{"next":"?page%5Bcursor%5D=email"}}`)
			}
		case "/api/campaign-messages/bfcm/campaign-variations":
			fmt.Fprint(w, `{"data":[{"attributes":{"definition":{"details":{"channel":"email"}}}}],"links":{"next":null}}`)
		case "/api/campaign-messages/sms/campaign-variations":
			fmt.Fprint(w, `{"data":[{"attributes":{"definition":{"details":{"channel":"sms"}}}}],"links":{"next":null}}`)
		case "/api/campaign-messages/current/campaign", "/api/campaign-messages/bfcm/campaign":
			fmt.Fprint(w, `{"data":{"attributes":{"name":"Campaign container"}}}`)
		case "/api/campaign-recipient-estimations/current":
			fmt.Fprint(w, `{"data":{"attributes":{"estimated_recipient_count":12000}}}`)
		case "/api/campaign-recipient-estimations/bfcm":
			estimateReads++
			if estimateReads == 1 {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprint(w, `{"errors":[{"detail":"No results or results were out of date. Schedule a new estimation..."}]}`)
			} else {
				fmt.Fprint(w, `{"data":{"attributes":{"estimated_recipient_count":500000}}}`)
			}
		case "/api/campaign-recipient-estimation-jobs":
			var body struct {
				Data struct{ ID, Type string } `json:"data"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || r.Method != http.MethodPost || body.Data.ID != "bfcm" || body.Data.Type != "campaign-recipient-estimation-job" {
				t.Errorf("job body = %+v, method = %s, error = %v", body, r.Method, err)
			}
			w.WriteHeader(http.StatusAccepted)
			fmt.Fprint(w, `{"data":{"id":"job-bfcm"}}`)
		case "/api/campaign-recipient-estimation-jobs/job-bfcm":
			polls++
			status := "processing"
			if polls == 2 {
				status = "complete"
			}
			fmt.Fprintf(w, `{"data":{"attributes":{"status":%q}}}`, status)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	})
	report, err := collectSendBudget(context.Background(), c, false, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Messages) != 2 || report.Current.Scheduled != 12000 || report.Current.Committed != 427344 || report.Current.RemainingAfterScheduled != 1022656 || report.Current.Verdict != "ok" {
		t.Fatalf("report = %+v", report)
	}
	if report.Messages[0].CampaignName != "Campaign container" || report.Messages[0].MessageName != "Current email" || report.Messages[1].Period != 1 || report.Messages[1].Estimate == nil || *report.Messages[1].Estimate != 500000 || len(report.Later) != 1 || report.Later[0].Scheduled != 500000 {
		t.Fatalf("messages = %+v, later = %+v", report.Messages, report.Later)
	}
	mu.Lock()
	defer mu.Unlock()
	if polls != 2 || estimateReads != 2 {
		t.Fatalf("polls=%d estimate reads=%d", polls, estimateReads)
	}
	var refreshSequence []string
	for _, request := range requests {
		if strings.Contains(request, "estimations/bfcm") || strings.Contains(request, "estimation-jobs") {
			refreshSequence = append(refreshSequence, request)
		}
	}
	want := []string{"GET /api/campaign-recipient-estimations/bfcm", "POST /api/campaign-recipient-estimation-jobs", "GET /api/campaign-recipient-estimation-jobs/job-bfcm", "GET /api/campaign-recipient-estimation-jobs/job-bfcm", "GET /api/campaign-recipient-estimations/bfcm"}
	if !reflect.DeepEqual(refreshSequence, want) {
		t.Fatalf("refresh sequence = %v", refreshSequence)
	}
}

func TestSendBudgetMissingEstimatesRemainUnknown(t *testing.T) {
	for _, mode := range []string{"no-refresh", "processing timeout", "complete but missing"} {
		t.Run(mode, func(t *testing.T) {
			var mu sync.Mutex
			posts, polls := 0, 0
			c := sendBudgetTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				switch r.URL.Path {
				case "/api/billing-usage":
					fmt.Fprint(w, sendBudgetBillingFixture)
				case "/api/campaign-messages":
					fmt.Fprint(w, `{"data":[{"id":"missing","attributes":{"definition":{"name":"Missing estimate","send_options":{"send_time":"2026-11-10T17:01:00Z"}}}},{"id":"zero","attributes":{"definition":{"name":"Zero estimate","send_options":{"send_time":"2026-11-10T17:02:00Z"}}}}],"links":{"next":null}}`)
				case "/api/campaign-messages/missing/campaign-variations", "/api/campaign-messages/zero/campaign-variations":
					fmt.Fprint(w, `{"data":[{"attributes":{"definition":{"details":{"channel":"email"}}}}]}`)
				case "/api/campaign-messages/missing/campaign", "/api/campaign-messages/zero/campaign":
					fmt.Fprint(w, `{"data":{"attributes":{"definition":{"name":"Container"}}}}`)
				case "/api/campaign-recipient-estimations/missing":
					w.WriteHeader(http.StatusNotFound)
					fmt.Fprint(w, `{"errors":[{"detail":"No results or results were out of date"}]}`)
				case "/api/campaign-recipient-estimations/zero":
					fmt.Fprint(w, `{"data":{"attributes":{"estimated_recipient_count":0}}}`)
				case "/api/campaign-recipient-estimation-jobs":
					posts++
					fmt.Fprint(w, `{"data":{"id":"missing-job"}}`)
				case "/api/campaign-recipient-estimation-jobs/missing-job":
					polls++
					status := "processing"
					if mode == "complete but missing" {
						status = "complete"
					}
					fmt.Fprintf(w, `{"data":{"attributes":{"status":%q}}}`, status)
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
					http.NotFound(w, r)
				}
			})
			started := time.Now()
			report, err := collectSendBudget(context.Background(), c, mode == "no-refresh", 80*time.Millisecond)
			if err != nil {
				t.Fatal(err)
			}
			if report.Current.UnknownCount != 1 || report.Current.Scheduled != 0 || report.Messages[0].Estimate != nil || report.Messages[0].EstimateStatus != "no estimate" || report.Messages[1].Estimate == nil || *report.Messages[1].Estimate != 0 {
				t.Fatalf("report = %+v", report)
			}
			if time.Since(started) > time.Second {
				t.Fatal("refresh exceeded total wait budget")
			}
			var output bytes.Buffer
			if err := printSendBudget(&output, report); err != nil || !strings.Contains(output.String(), "known estimates only") || !strings.Contains(output.String(), "1 scheduled message(s) have no estimate") {
				t.Fatalf("output = %s, error = %v", output.String(), err)
			}
			raw, err := json.Marshal(report)
			if err != nil || !bytes.Contains(raw, []byte(`"estimate":null`)) || !bytes.Contains(raw, []byte(`"estimate":0`)) {
				t.Fatalf("JSON loses unknown vs zero: %s, error = %v", raw, err)
			}
			mu.Lock()
			defer mu.Unlock()
			if mode == "no-refresh" && (posts != 0 || polls != 0) {
				t.Fatalf("--no-refresh sent %d POSTs and %d polls", posts, polls)
			}
			if mode != "no-refresh" && (posts != 1 || polls == 0) {
				t.Fatalf("refresh sent %d POSTs and %d polls", posts, polls)
			}
		})
	}
}

func TestSendBudgetPeriodBoundariesAndVerdicts(t *testing.T) {
	c := sendBudgetTestClient(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, sendBudgetBillingFixture) })
	usage := sendBudgetTestUsage(t, c)
	estimate := int64(100)
	times := []time.Time{usage.Start, usage.End.Add(-time.Nanosecond), usage.End, time.Date(2026, 11, 27, 8, 0, 0, 0, time.UTC), usage.End.AddDate(0, 1, 0)}
	var messages []sendBudgetMessage
	for i, sendTime := range times {
		messages = append(messages, sendBudgetMessage{ID: fmt.Sprint(i), SendTime: sendTime, Estimate: &estimate})
	}
	report, err := buildSendBudgetReport(usage, messages)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []int{0, 0, 1, 1, 2} {
		if report.Messages[i].Period != want {
			t.Fatalf("message %d period = %d, want %d", i, report.Messages[i].Period, want)
		}
	}
	if report.Current.Scheduled != 200 || len(report.Later) != 2 || report.Later[0].Scheduled != 200 || report.Later[1].Scheduled != 100 {
		t.Fatalf("buckets = %+v", report)
	}
	anchor := time.Date(2026, 12, 31, 17, 1, 0, 0, time.UTC)
	for i, want := range []string{"2027-01-31", "2027-02-28", "2027-03-31"} {
		if got := sendBudgetMonthBoundary(anchor, i+1).Format("2006-01-02"); got != want {
			t.Fatalf("boundary %d = %s, want %s", i+1, got, want)
		}
	}
	for _, tc := range []struct {
		name      string
		used      float64
		scheduled int64
		verdict   string
		over      float64
		paceOver  float64
	}{
		{"over beats pace risk", 800, 300, "over", 100, 600},
		{"at risk", 600, 100, "at risk", 0, 200},
		{"ok", 400, 100, "ok", 0, 0},
		{"exact cap is ok", 500, 500, "ok", 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cap := float64(1000)
			usage.Cap, usage.Used = &cap, &tc.used
			usage.AsOf = usage.Start.Add(usage.End.Sub(usage.Start) / 2)
			report, err := buildSendBudgetReport(usage, []sendBudgetMessage{{SendTime: usage.AsOf, Estimate: &tc.scheduled}})
			if err != nil {
				t.Fatal(err)
			}
			got := report.Current
			if got.Verdict != tc.verdict || got.OverBy != tc.over || got.PaceOverBy != tc.paceOver || got.Committed != tc.used+float64(tc.scheduled) || got.PaceProjection == nil || *got.PaceProjection != tc.used*2 {
				t.Fatalf("current = %+v", got)
			}
		})
	}
}

func TestSendBudgetRefreshOneSharedDeadlineAndCancellation(t *testing.T) {
	var mu sync.Mutex
	posts := 0
	c := sendBudgetTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			mu.Lock()
			posts++
			mu.Unlock()
			var body struct{ Data struct{ ID string } }
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			fmt.Fprintf(w, `{"data":{"id":%q}}`, body.Data.ID)
			return
		}
		<-r.Context().Done()
	})
	messages := []sendBudgetMessage{{ID: "one"}, {ID: "two"}}
	started := time.Now()
	if err := refreshSendBudgetEstimates(context.Background(), c, messages, 80*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if time.Since(started) > time.Second || messages[0].Estimate != nil || messages[1].Estimate != nil {
		t.Fatal("deadline lost or unknown estimates converted to counts")
	}
	mu.Lock()
	gotPosts := posts
	mu.Unlock()
	if gotPosts != 2 {
		t.Fatalf("only started %d of 2 jobs before polling", gotPosts)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := refreshSendBudgetEstimates(ctx, c, messages, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error = %v", err)
	}
}

func TestSendBudgetPaginationRejectsIncompleteWalk(t *testing.T) {
	for _, next := range []string{"?page%5Bcursor%5D=repeat", "?page=2"} {
		t.Run(next, func(t *testing.T) {
			c := sendBudgetTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"data":[{"id":"one"}],"links":{"next":%q}}`, next)
			})
			if _, err := sendBudgetCollection(context.Background(), c, "/api/campaign-messages", nil); err == nil || !strings.Contains(err.Error(), "pagination cursor") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestSendBudgetCommandRegistrationAndOutput(t *testing.T) {
	root := RootCmd()
	command, _, err := root.Find([]string{"send-budget"})
	if err != nil || command == root || command.Name() != "send-budget" {
		t.Fatalf("command = %v, error = %v", command, err)
	}
	if command.Flags().Lookup("wait").DefValue != "2m0s" || command.Flags().Lookup("no-refresh") == nil {
		t.Fatal("missing flags or incorrect wait default")
	}
	c := sendBudgetTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("--no-refresh issued %s", r.Method)
		}
		if r.URL.Path == "/api/billing-usage" {
			fmt.Fprint(w, sendBudgetBillingFixture)
		} else {
			fmt.Fprint(w, `{"data":[],"links":{"next":null}}`)
		}
	})
	t.Setenv("KLAVIYO_BASE_URL", c.BaseURL)
	t.Setenv("KLAVIYO_API_KEY", "synthetic")
	t.Setenv("KLAVIYO_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	previousRegistration, previousHooks := registeredPlatformSource, clientHooks
	registeredPlatformSource = nil
	t.Cleanup(func() { registeredPlatformSource, clientHooks = previousRegistration, previousHooks })
	registerClientHook(func(created *client.Client) error {
		created.HTTPClient.Transport = c.HTTPClient.Transport
		return nil
	})
	for _, asJSON := range []bool{false, true} {
		var output bytes.Buffer
		command := newSendBudgetCmd(&rootFlags{asJSON: asJSON, timeout: time.Second})
		command.SetOut(&output)
		command.SetArgs([]string{"--no-refresh"})
		if err := command.Execute(); err != nil {
			t.Fatal(err)
		}
		if asJSON {
			var report sendBudgetReport
			if err := json.Unmarshal(output.Bytes(), &report); err != nil || report.Current.Used != 415344 || report.Current.Cap != 1450000 || report.Current.Verdict != "ok" {
				t.Fatalf("JSON output = %s, error = %v", output.String(), err)
			}
		} else if !strings.Contains(output.String(), "415,344 of 1,450,000 sent") || !strings.Contains(output.String(), "On track: 1,034,656 left") || !strings.Contains(output.String(), "includes flows and past campaigns") {
			t.Fatalf("human output = %s", output.String())
		}
	}
}
