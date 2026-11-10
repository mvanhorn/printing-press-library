// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
	"github.com/spf13/cobra"
)

type campaignABRequest struct {
	method, endpoint string
	body             map[string]any
}

type campaignABFixture struct {
	t                      *testing.T
	status, sendTime       string
	settings               any
	variations             []map[string]any
	rows                   []map[string]any
	metrics                []map[string]any
	metricReads            int
	writes                 []campaignABRequest
	reads                  int
	createAttempts         int
	failCreateAt, failCode int
	failPatch, failDelete  bool
	multi, corruptReadback bool
	reportNext             bool
}

func newCampaignABFixture(t *testing.T) *campaignABFixture {
	t.Helper()
	a := modelVariationFixture("variation-a", "acme-template", "email")
	anyPath(a, "attributes.definition").(map[string]any)["name"] = "Acme launch"
	details := anyPath(a, "attributes.definition.details").(map[string]any)
	details["cc_email"], details["bcc_email"] = "cc@example.com", "bcc@example.com"
	return &campaignABFixture{t: t, status: "draft", variations: []map[string]any{a}, rows: []map[string]any{}}
}

func (f *campaignABFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Header.Get("revision") != "2026-10-15" {
		f.t.Errorf("revision=%s", r.Header.Get("revision"))
	}
	endpoint := strings.TrimRight(r.URL.Path, "/")
	if r.Method == http.MethodGet {
		f.reads++
		m := modelMessageFixture(modelMessageID, f.status)
		definition := anyPath(m, "attributes.definition").(map[string]any)
		definition["ab_test"] = f.settings
		if f.sendTime != "" {
			definition["send_options"].(map[string]any)["send_time"] = f.sendTime
		}
		switch endpoint {
		case "/api/campaigns/" + modelContainerID:
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": modelContainerID}})
		case "/api/campaigns/" + modelContainerID + "/campaign-messages":
			messages := []map[string]any{m}
			if f.multi {
				messages = append(messages, modelMessageFixture("acme-message-2", "draft"))
			}
			w.Write(modelCollectionFixture(messages...))
		case "/api/campaign-messages/" + modelMessageID:
			json.NewEncoder(w).Encode(map[string]any{"data": m})
		case "/api/campaign-messages/" + modelMessageID + "/campaign-audience", "/api/campaign-messages/acme-message-2/campaign-audience":
			w.Write(modelAudienceFixture([]string{"acme-list"}, nil))
		case "/api/campaign-messages/" + modelMessageID + "/campaign-variations", "/api/campaign-messages/acme-message-2/campaign-variations":
			if f.corruptReadback && f.settings != nil && len(f.variations) > 1 {
				anyPath(f.variations[1], "attributes.definition.details").(map[string]any)["subject"] = "Unexpected subject"
			}
			w.Write(modelCollectionFixture(f.variations...))
		case "/api/metrics":
			f.metricReads++
			if f.metrics != nil {
				w.Write(modelCollectionFixture(f.metrics...))
				break
			}
			fmt.Fprint(w, `{"data":[{"id":"acme-order-metric","attributes":{"name":"Placed Order","integration":{"name":"Shopify"}}}],"links":{"next":null}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"errors":[{"detail":"not found"}]}`)
		}
		return
	}
	var body map[string]any
	if r.Method != http.MethodDelete {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			f.t.Fatal(err)
		}
	}
	f.writes = append(f.writes, campaignABRequest{r.Method, endpoint, body})
	switch {
	case r.Method == http.MethodPost && endpoint == "/api/campaign-variations":
		f.createAttempts++
		if f.createAttempts == f.failCreateAt {
			w.WriteHeader(f.failCode)
			fmt.Fprint(w, `{"errors":[{"detail":"create failed"}]}`)
			return
		}
		data := map[string]any{"id": fmt.Sprintf("variation-%c", 'a'+f.createAttempts), "type": "campaign-variation", "attributes": anyPath(body, "data.attributes")}
		f.variations = append(f.variations, data)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"data": data})
	case r.Method == http.MethodPatch && endpoint == "/api/campaign-messages/"+modelMessageID:
		if f.failPatch {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"errors":[{"detail":"settings rejected"}]}`)
			return
		}
		f.settings = anyPath(body, "data.attributes.definition.ab_test")
		fmt.Fprint(w, `{"data":{"id":"updated"}}`)
	case r.Method == http.MethodDelete && strings.HasPrefix(endpoint, "/api/campaign-variations/"):
		if f.failDelete {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"errors":[{"detail":"cleanup denied"}]}`)
			return
		}
		id := strings.TrimPrefix(endpoint, "/api/campaign-variations/")
		for i, variation := range f.variations {
			if variation["id"] == id {
				f.variations = append(f.variations[:i], f.variations[i+1:]...)
				break
			}
		}
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodPost && endpoint == "/api/campaign-values-reports":
		rows := f.rows
		var next any
		if f.reportNext {
			if r.URL.Query().Get("page_cursor") == "" {
				rows = rows[:1]
				next = "/api/campaign-values-reports/?page_cursor=last"
			} else {
				rows = rows[1:]
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"attributes": map[string]any{"results": rows}}, "links": map[string]any{"next": next}})
	default:
		f.t.Errorf("unexpected write: %s %s", r.Method, endpoint)
		w.WriteHeader(http.StatusBadRequest)
	}
}

func executeCampaignAB(t *testing.T, cmd *cobra.Command, args ...string) ([]byte, error) {
	t.Helper()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.Bytes(), err
}

func TestCampaignABCreateHappyPath(t *testing.T) {
	f := newCampaignABFixture(t)
	campaignFixtureClient(t, f)
	out, err := executeCampaignAB(t, newCampaignsABTestCreateCmd(&rootFlags{asJSON: true, rateLimit: 10000}),
		"--campaign-id", modelContainerID, "--subject", "Acme B, with a comma", "--subject", "Acme C", "--preview-text", "Preview B", "--preview-text", "", "--winner-metric", "orders", "--test-percent", "30", "--wait-hours", "8")
	if err != nil {
		t.Fatal(err)
	}
	var result campaignABCreateResult
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	if result.MessageID != modelMessageID || result.CampaignID != modelContainerID || len(result.Variations) != 3 || result.Variations[0].Created || !result.Variations[1].Created || !result.Variations[2].Created {
		t.Fatalf("result=%s", out)
	}
	if len(f.writes) != 3 {
		t.Fatalf("writes=%+v", f.writes)
	}
	for i, v := range result.Variations[1:] {
		request := f.writes[i]
		wantDetails := map[string]any{"channel": "email", "template_id": "acme-template", "from_email": "sender@example.com", "from_label": "Sender", "reply_to_email": "reply@example.com", "cc_email": "cc@example.com", "bcc_email": "bcc@example.com", "subject": v.Subject, "preview_text": v.PreviewText}
		want := jsonAPIBody("campaign-variation", map[string]any{"definition": map[string]any{"name": "Acme launch - " + v.Label, "details": wantDetails}}, map[string]any{"campaign-message": map[string]any{"data": map[string]any{"type": "campaign-message", "id": modelMessageID}}})
		if request.method != "POST" || request.endpoint != "/api/campaign-variations" || !reflect.DeepEqual(request.body, want) {
			t.Fatalf("POST %d=%+v want=%v", i, request, want)
		}
	}
	var expected map[string]any
	json.Unmarshal([]byte(fmt.Sprintf(`{"data":{"type":"campaign-message","id":%q,"attributes":{"definition":{"ab_test":{"winner_metric":"placed_orders_unique","winner_percentage":70,"winner_wait_hours":8}}}}}`, modelMessageID)), &expected)
	if f.writes[2].method != "PATCH" || !reflect.DeepEqual(f.writes[2].body, expected) {
		t.Fatalf("PATCH=%+v", f.writes[2])
	}
	if f.reads < 7 {
		t.Fatalf("no fresh read-back: reads=%d", f.reads)
	}
}

func TestCampaignABCreateRefusesUnsafeOrExistingTest(t *testing.T) {
	for _, kind := range []string{"scheduled", "draft with send time", "existing ab_test", "multiple variations", "no email", "no variations", "no template"} {
		t.Run(kind, func(t *testing.T) {
			f := newCampaignABFixture(t)
			switch kind {
			case "scheduled":
				f.status = "scheduled"
			case "draft with send time":
				f.sendTime = "2026-11-30T12:00:00Z"
			case "existing ab_test":
				f.settings = map[string]any{"winner_metric": "clicks_unique"}
			case "multiple variations":
				f.variations = append(f.variations, modelVariationFixture("other", "acme-template", "email"))
			case "no email":
				anyPath(f.variations[0], "attributes.definition.details").(map[string]any)["channel"] = "sms"
			case "no variations":
				f.variations = []map[string]any{}
			case "no template":
				anyPath(f.variations[0], "attributes.definition.details").(map[string]any)["template_id"] = ""
			}
			campaignFixtureClient(t, f)
			_, err := executeCampaignAB(t, newCampaignsABTestCreateCmd(&rootFlags{asJSON: true, rateLimit: 10000}), "--campaign-id", modelMessageID, "--subject", "Acme B")
			if err == nil || len(f.writes) != 0 {
				t.Fatalf("error=%v writes=%v", err, f.writes)
			}
			if kind == "existing ab_test" && !strings.Contains(err.Error(), "already has an A/B test") {
				t.Fatal(err)
			}
		})
	}
}

func TestCampaignABCreateRollsBackFailuresWithoutRetry(t *testing.T) {
	for _, kind := range []string{"patch", "cleanup failure", "ambiguous create", "readback mismatch"} {
		t.Run(kind, func(t *testing.T) {
			f := newCampaignABFixture(t)
			switch kind {
			case "patch", "cleanup failure":
				f.failPatch = true
				f.failDelete = kind == "cleanup failure"
			case "ambiguous create":
				f.failCreateAt, f.failCode = 2, http.StatusBadGateway
			case "readback mismatch":
				f.corruptReadback = true
			}
			campaignFixtureClient(t, f)
			if kind == "ambiguous create" {
				registerClientHook(func(c *client.Client) error {
					c.Config.Headers = map[string]string{"Idempotency-Key": "acme-test-key"}
					return nil
				})
			}
			_, err := executeCampaignAB(t, newCampaignsABTestCreateCmd(&rootFlags{asJSON: true, rateLimit: 10000}), "--campaign-id", modelMessageID, "--subject", "Acme B", "--subject", "Acme C")
			if err == nil || f.createAttempts != 2 {
				t.Fatalf("error=%v attempts=%d", err, f.createAttempts)
			}
			deleted := []string{}
			for _, request := range f.writes {
				if request.method == "DELETE" {
					deleted = append(deleted, strings.TrimPrefix(request.endpoint, "/api/campaign-variations/"))
				}
			}
			want := []string{"variation-b", "variation-c"}
			if kind == "ambiguous create" {
				want = []string{"variation-b"}
				if !strings.Contains(err.Error(), "outcome unknown") || !strings.Contains(err.Error(), "not retried") {
					t.Fatal(err)
				}
			}
			if kind == "readback mismatch" {
				want = []string{}
				if !strings.Contains(err.Error(), "read-back unverified") {
					t.Fatal(err)
				}
			}
			if !reflect.DeepEqual(deleted, want) {
				t.Fatalf("deleted=%v want=%v error=%v", deleted, want, err)
			}
			if kind == "cleanup failure" && (!strings.Contains(err.Error(), "could not roll back: [variation-b") || !strings.Contains(err.Error(), "variation-c")) {
				t.Fatal(err)
			}
			if kind == "patch" && (len(f.variations) != 1 || !strings.Contains(err.Error(), "rolled back variations: [variation-b, variation-c]")) {
				t.Fatalf("remaining=%v error=%v", f.variations, err)
			}
		})
	}
}

func TestCampaignABCreateValidatesFlagsAndSubjects(t *testing.T) {
	for _, args := range [][]string{
		{"--subject", "B", "--test-percent", "0"}, {"--subject", "B", "--test-percent", "101"},
		{"--subject", "B", "--wait-hours", "265"}, {"--subject", "B", "--wait-hours", "0"},
		{"--subject", ""}, {"--subject", "  "}, {"--subject", "line\nbreak"},
		{"--subject", "B", "--subject", "B"}, {"--subject", "A useful subject"},
		{"--subject", "B", "--subject", "C", "--preview-text", "Only one"},
		{"--subject", "B", "--winner-metric", "revenue"}, {},
		{"--subject", "B", "--subject", "C", "--subject", "D", "--subject", "E", "--subject", "F", "--subject", "G", "--subject", "H"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			f := newCampaignABFixture(t)
			campaignFixtureClient(t, f)
			args = append([]string{"--campaign-id", modelMessageID}, args...)
			_, err := executeCampaignAB(t, newCampaignsABTestCreateCmd(&rootFlags{asJSON: true, rateLimit: 10000}), args...)
			if err == nil || ExitCode(err) != 2 || len(f.writes) != 0 {
				t.Fatalf("error=%v writes=%v", err, f.writes)
			}
		})
	}
}

func TestCampaignABCreateReconcilesFailedSettingsPatch(t *testing.T) {
	for _, failure := range []string{"transport", "HTTP 502", "HTTP 202"} {
		for _, state := range []string{"requested", "null", "different", "unreadable"} {
			t.Run(failure+"/"+state, func(t *testing.T) {
				f := newCampaignABFixture(t)
				patches, freshReads := 0, 0
				campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodGet && r.URL.Path == "/api/campaign-messages/"+modelMessageID && patches > 0 {
						freshReads++
						if state == "unreadable" {
							w.WriteHeader(http.StatusForbidden)
							fmt.Fprint(w, `{"errors":[{"detail":"read denied"}]}`)
							return
						}
					}
					f.ServeHTTP(w, r)
				}))
				registerClientHook(func(c *client.Client) error {
					transport := c.HTTPClient.Transport
					c.HTTPClient.Transport = klaviyoCloneTransport(func(r *http.Request) (*http.Response, error) {
						response, err := transport.RoundTrip(r)
						if err != nil || r.Method != http.MethodPatch {
							return response, err
						}
						patches++
						switch state {
						case "null":
							f.settings = nil
						case "different":
							f.settings.(map[string]any)["winner_wait_hours"] = float64(8)
						}
						switch failure {
						case "transport":
							response.Body.Close()
							return nil, io.ErrUnexpectedEOF
						case "HTTP 502":
							response.StatusCode = http.StatusBadGateway
						case "HTTP 202":
							response.StatusCode = http.StatusAccepted
						}
						return response, nil
					})
					return nil
				})
				out, err := executeCampaignAB(t, newCampaignsABTestCreateCmd(&rootFlags{asJSON: true, rateLimit: 10000}),
					"--campaign-id", modelMessageID, "--subject", "Acme B", "--subject", "Acme C")
				if patches != 1 || freshReads == 0 {
					t.Fatalf("patches=%d fresh reads=%d error=%v", patches, freshReads, err)
				}
				switch state {
				case "requested":
					var result campaignABCreateResult
					if decodeErr := json.Unmarshal(out, &result); err != nil || decodeErr != nil || result.MessageID != modelMessageID || len(result.Variations) != 3 || len(f.writes) != 3 || freshReads < 2 {
						t.Fatalf("error=%v output=%s writes=%v fresh reads=%d", err, out, f.writes, freshReads)
					}
				case "null":
					if err == nil || len(f.variations) != 1 || !strings.Contains(err.Error(), "rolled back variations: [variation-b, variation-c]") {
						t.Fatalf("error=%v remaining variations=%v", err, f.variations)
					}
				default:
					if err == nil || ExitCode(err) == 0 || len(f.writes) != 3 || len(f.variations) != 3 || len(out) != 0 {
						t.Fatalf("error=%v output=%s writes=%v remaining variations=%v", err, out, f.writes, f.variations)
					}
					for _, detail := range []string{modelMessageID, "variation-b", "variation-c", "state needs inspection"} {
						if !strings.Contains(err.Error(), detail) {
							t.Fatalf("error missing %q: %v", detail, err)
						}
					}
				}
			})
		}
	}
}

func TestCampaignABContainerAmbiguityAndGlobalDryRun(t *testing.T) {
	for _, subcommand := range []string{"create", "results"} {
		t.Run(subcommand, func(t *testing.T) {
			f := newCampaignABFixture(t)
			f.multi = true
			campaignFixtureClient(t, f)
			flags := &rootFlags{asJSON: true, rateLimit: 10000}
			cmd := newCampaignsABTestResultsCmd(flags)
			args := []string{"--campaign-id", modelContainerID}
			if subcommand == "create" {
				cmd = newCampaignsABTestCreateCmd(flags)
				args = append(args, "--subject", "Acme B")
			}
			_, err := executeCampaignAB(t, cmd, args...)
			if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), modelMessageID) || !strings.Contains(err.Error(), "acme-message-2") || len(f.writes) != 0 {
				t.Fatalf("error=%v writes=%v", err, f.writes)
			}
		})
	}
	t.Run("global dry run", func(t *testing.T) {
		f := newCampaignABFixture(t)
		campaignFixtureClient(t, f)
		flags := &rootFlags{rateLimit: 10000}
		root := newRootCmd(flags)
		for _, name := range []string{"create", "results"} {
			cmd, _, err := root.Find([]string{"campaigns", "ab-test", name})
			if err != nil || cmd.Name() != name {
				t.Fatalf("registration %s: %v %v", name, cmd, err)
			}
		}
		root.PersistentPreRunE = nil
		out, err := executeCampaignAB(t, root, "campaigns", "ab-test", "create", "--campaign-id", modelMessageID, "--subject", "Acme B", "--dry-run", "--json")
		var result campaignABCreateResult
		if decodeErr := json.Unmarshal(out, &result); err != nil || decodeErr != nil || !result.DryRun || len(f.writes) != 0 || result.ABTest["winner_percentage"] != float64(80) || result.Variations[1].Created || result.Variations[1].PreviewText != "Preview" {
			t.Fatalf("error=%v output=%s writes=%v", err, out, f.writes)
		}
		human, err := executeCampaignAB(t, newCampaignsABTestCreateCmd(&rootFlags{dryRun: true, rateLimit: 10000}), "--campaign-id", modelMessageID, "--subject", "Acme B", "--winner-metric", "opens")
		if err != nil || !strings.Contains(string(human), "campaigns test-send --campaign-id "+modelMessageID) || len(f.writes) != 0 {
			t.Fatalf("error=%v output=%s", err, human)
		}
	})
}

func campaignABReportRow(id string, recipients, clicks, opens, orders, revenue float64) map[string]any {
	return map[string]any{"groupings": map[string]any{"campaign_id": modelContainerID, "campaign_message_id": modelMessageID, "variation": id, "variation_name": "Acme " + id},
		"statistics": map[string]any{"recipients": recipients, "clicks_unique": clicks, "click_rate": rate(clicks, recipients), "opens_unique": opens, "open_rate": rate(opens, recipients), "conversion_uniques": orders, "conversion_value": revenue, "conversion_rate": rate(orders, recipients)}}
}

func TestCampaignABResultsTooEarly(t *testing.T) {
	f := newCampaignABFixture(t)
	f.settings = map[string]any{"winner_metric": "clicks_unique", "winner_percentage": 80, "winner_wait_hours": 4}
	f.variations = append(f.variations, modelVariationFixture("variation-b", "acme-template", "email"))
	campaignFixtureClient(t, f)
	out, err := executeCampaignAB(t, newCampaignsABTestResultsCmd(&rootFlags{asJSON: true, rateLimit: 10000}), "--campaign-id", modelMessageID)
	var result campaignABResults
	if decodeErr := json.Unmarshal(out, &result); err != nil || decodeErr != nil || result.Verdict != "too_early" || result.Timeframe != nil || len(f.writes) != 0 {
		t.Fatalf("error=%v output=%s writes=%v", err, out, f.writes)
	}
}

func TestCampaignABResultsRejectsNonABMessagesBeforeReporting(t *testing.T) {
	for _, status := range []string{"draft", "sent"} {
		for _, kind := range []string{"no settings", "one email", "one email and SMS"} {
			t.Run(status+"/"+kind, func(t *testing.T) {
				f := newCampaignABFixture(t)
				f.status, f.sendTime = status, "invalid send time must not be inspected"
				want := "message " + modelMessageID + " is not an A/B test (only one email variation)"
				if kind == "no settings" {
					f.variations = append(f.variations, modelVariationFixture("variation-b", "acme-template", "email"))
					want = "message " + modelMessageID + " is not an A/B test (no ab_test settings)"
				} else {
					f.settings = map[string]any{"winner_metric": "clicks_unique"}
					if kind == "one email and SMS" {
						f.variations = append(f.variations, modelVariationFixture("variation-sms", "", "sms"))
					}
				}
				campaignFixtureClient(t, f)
				out, err := executeCampaignAB(t, newCampaignsABTestResultsCmd(&rootFlags{asJSON: true, rateLimit: 10000}), "--campaign-id", modelMessageID)
				if err == nil || ExitCode(err) != 2 || err.Error() != want || len(f.writes) != 0 || len(out) != 0 {
					t.Fatalf("error=%v want=%q output=%s writes=%v", err, want, out, f.writes)
				}
			})
		}
	}
}

func TestCampaignABResultsOnlyInfersCompletedRolloutWithUniqueRecipientLeader(t *testing.T) {
	for _, tc := range []struct {
		name, status, winner, note string
		elapsed                    time.Duration
		percentage                 int
		recipients                 []float64
	}{
		{"sending after wait", "sending", "", "still running", 5 * time.Hour, 80, []float64{1000, 500}},
		{"sent before wait", "sent", "", "still running", 4*time.Hour - time.Nanosecond, 80, []float64{1000, 500}},
		{"sent at wait", "sent", "variation-a", "", 4 * time.Hour, 80, []float64{1000, 500}},
		{"sent after wait", "sent", "variation-b", "", 5 * time.Hour, 80, []float64{500, 1000}},
		{"no rollout audience", "sent", "", "winner_percentage", 5 * time.Hour, 0, []float64{1000, 500}},
		{"tied counts", "sent", "", "counts are tied", 5 * time.Hour, 80, []float64{1000, 1000}},
		{"tied leaders", "sent", "", "counts are tied", 5 * time.Hour, 80, []float64{500, 1000, 1000}},
		{"zero counts", "sent", "", "counts are tied", 5 * time.Hour, 80, []float64{0, 0}},
		{"tied trailing counts", "sent", "variation-a", "", 5 * time.Hour, 80, []float64{1000, 500, 500}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCampaignABFixture(t)
			f.status, f.sendTime = tc.status, "2026-11-02T12:00:00Z"
			f.settings = map[string]any{"winner_metric": "clicks_unique", "winner_percentage": tc.percentage, "winner_wait_hours": 4}
			for i, recipients := range tc.recipients {
				id := fmt.Sprintf("variation-%c", 'a'+i)
				if i > 0 {
					f.variations = append(f.variations, modelVariationFixture(id, "acme-template", "email"))
				}
				f.rows = append(f.rows, campaignABReportRow(id, recipients, 0, 0, 0, 0))
			}
			previousNow := klaviyoWindowNow
			klaviyoWindowNow = func() time.Time { return time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC).Add(tc.elapsed) }
			t.Cleanup(func() { klaviyoWindowNow = previousNow })
			campaignFixtureClient(t, f)
			out, err := executeCampaignAB(t, newCampaignsABTestResultsCmd(&rootFlags{asJSON: true, rateLimit: 10000}), "--campaign-id", modelMessageID)
			var result campaignABResults
			if decodeErr := json.Unmarshal(out, &result); err != nil || decodeErr != nil {
				t.Fatalf("error=%v output=%s", err, out)
			}
			if tc.winner != "" {
				if result.KlaviyoWinnerInferred == nil || result.KlaviyoWinnerInferred.VariationID != tc.winner {
					t.Fatalf("expected winner %s: %s", tc.winner, out)
				}
			} else if result.KlaviyoWinnerInferred != nil || bytes.Contains(out, []byte(`"klaviyo_winner_inferred"`)) || !strings.Contains(strings.Join(result.Notes, " "), tc.note) {
				t.Fatalf("expected no inferred winner and note %q: %s", tc.note, out)
			}
		})
	}
}

func TestCampaignABResultsDoesNotInferWinnerWithNullWaitHours(t *testing.T) {
	f := newCampaignABFixture(t)
	f.status, f.sendTime = "sent", "2026-11-02T12:00:00Z"
	f.settings = map[string]any{"winner_metric": "clicks_unique", "winner_percentage": 80, "winner_wait_hours": nil}
	f.variations = append(f.variations, modelVariationFixture("variation-b", "acme-template", "email"))
	f.rows = []map[string]any{campaignABReportRow("variation-a", 1000, 0, 0, 0, 0), campaignABReportRow("variation-b", 500, 0, 0, 0, 0)}
	previousNow := klaviyoWindowNow
	klaviyoWindowNow = func() time.Time { return time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { klaviyoWindowNow = previousNow })
	campaignFixtureClient(t, f)
	out, err := executeCampaignAB(t, newCampaignsABTestResultsCmd(&rootFlags{asJSON: true, rateLimit: 10000}), "--campaign-id", modelMessageID)
	var result campaignABResults
	if decodeErr := json.Unmarshal(out, &result); err != nil || decodeErr != nil {
		t.Fatalf("error=%v output=%s", err, out)
	}
	if result.KlaviyoWinnerInferred != nil || bytes.Contains(out, []byte(`"klaviyo_winner_inferred"`)) || !strings.Contains(strings.Join(result.Notes, " "), "wait time is unknown") {
		t.Fatalf("expected no inferred winner and an unknown wait time note: %s", out)
	}
}

func TestCampaignABZTestFixedNumbers(t *testing.T) {
	// Hand calculation: pooled=.1, SE=sqrt(.1*.9*(1/1000+1/1000))=.0134164,
	// z=(.12-.08)/SE=2.981424; two-sided 2*(1-Phi(z))=.0028691 -> .0029.
	// For 60 vs 50: pooled=.055, SE=.01019559, z=.9808165; p=.3266833 -> .3267.
	// Uneven 6/25558 vs 1/4495: pooled=7/30053, SE=.0002468142, z=.04979755; p=.9602837 -> .9603.
	for _, tc := range []struct{ x1, n1, x2, n2, want float64 }{
		{120, 1000, 80, 1000, .0029}, {60, 1000, 50, 1000, .3267}, {6, 25558, 1, 4495, .9603},
		{0, 1000, 0, 1000, 1}, {1000, 1000, 1000, 1000, 1}, {0, 0, 50, 1000, 1},
	} {
		got := campaignABZTest(tc.x1, tc.n1, tc.x2, tc.n2)
		if math.Round(got*1e4)/1e4 != tc.want || campaignABZTest(tc.x2, tc.n2, tc.x1, tc.n1) != got {
			t.Errorf("%+v p=%.8f want %.4f", tc, got, tc.want)
		}
	}
}

func TestCampaignABResultsReportJoinAndInferredWinner(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(fmt.Sprintf("missing row=%t", missing), func(t *testing.T) {
			f := newCampaignABFixture(t)
			f.status, f.sendTime = "sent", "2026-11-02T01:00:00Z"
			f.settings = map[string]any{"winner_metric": "clicks_unique", "winner_percentage": 70, "winner_wait_hours": 4}
			f.variations = append(f.variations, modelVariationFixture("variation-b", "acme-template", "email"))
			f.rows = []map[string]any{campaignABReportRow("variation-b", 4495, 1, 100, 3, 90), campaignABReportRow("variation-a", 25558, 6, 200, 10, 300)}
			if missing {
				f.rows = f.rows[1:]
			} else {
				f.reportNext = true
			}
			t.Setenv("KLAVIYO_TIMEZONE", "America/Chicago")
			previousNow := klaviyoWindowNow
			klaviyoWindowNow = func() time.Time { return time.Date(2026, 11, 9, 18, 0, 0, 0, time.UTC) }
			t.Cleanup(func() { klaviyoWindowNow = previousNow })
			campaignFixtureClient(t, f)
			out, err := executeCampaignAB(t, newCampaignsABTestResultsCmd(&rootFlags{asJSON: true, rateLimit: 10000}), "--campaign-id", modelContainerID)
			if err != nil {
				t.Fatal(err)
			}
			var result campaignABResults
			if err := json.Unmarshal(out, &result); err != nil {
				t.Fatal(err)
			}
			if result.Verdict != "no_clear_winner" || result.Leader != "variation-a" || result.KlaviyoWinnerInferred == nil || result.KlaviyoWinnerInferred.VariationID != "variation-a" || result.KlaviyoWinnerInferred.Basis != "inferred from recipient counts" || len(result.Notes) == 0 {
				t.Fatalf("result=%s", out)
			}
			if result.Timeframe.Start.Format(time.RFC3339) != "2026-10-31T00:00:00-05:00" || result.Timeframe.End.Format(time.RFC3339) != "2026-11-09T12:00:00-06:00" {
				t.Fatalf("window=%+v", result.Timeframe)
			}
			if result.Variations[0].RevenuePerRecipient == nil || *result.Variations[0].RevenuePerRecipient != 300.0/25558 {
				t.Fatalf("revenue/recipient=%v", result.Variations[0])
			}
			if missing {
				zero := result.Variations[1]
				if zero.Recipients != 0 || zero.OpensUnique != 0 || zero.OpenRate != 0 || zero.ClicksUnique != 0 || zero.ClickRate != 0 || zero.Orders == nil || *zero.Orders != 0 || zero.Revenue == nil || *zero.Revenue != 0 || zero.RevenuePerRecipient == nil || *zero.RevenuePerRecipient != 0 || result.PValues[0].PValue != 1 {
					t.Fatalf("missing variation=%+v p=%v", zero, result.PValues)
				}
			} else if math.Round(result.PValues[0].PValue*1e4)/1e4 != .9603 {
				t.Fatalf("p=%v", result.PValues)
			}
			for _, request := range f.writes {
				if request.endpoint != "/api/campaign-values-reports" {
					t.Fatalf("unexpected write=%+v", request)
				}
				attrs := anyPath(request.body, "data.attributes").(map[string]any)
				wantGroups := []any{"campaign_id", "campaign_message_id", "variation", "variation_name"}
				wantStats := []any{"recipients", "opens_unique", "open_rate", "clicks_unique", "click_rate", "conversion_uniques", "conversion_value", "conversion_rate"}
				if attrs["conversion_metric_id"] != "acme-order-metric" || attrs["filter"] != fmt.Sprintf("equals(campaign_message_id,%q)", modelMessageID) || !reflect.DeepEqual(attrs["group_by"], wantGroups) || !reflect.DeepEqual(attrs["statistics"], wantStats) || stringFromMapPath(attrs, "timeframe.start") != "2026-10-31T00:00:00-05:00" || stringFromMapPath(attrs, "timeframe.end") != "2026-11-09T12:00:00-06:00" {
					t.Fatalf("report body=%v", request.body)
				}
			}
			wantReports := 2
			if missing {
				wantReports = 1
			}
			if len(f.writes) != wantReports {
				t.Fatalf("reports=%d want=%d", len(f.writes), wantReports)
			}
		})
	}
}

func TestCampaignABResultsMetricSelectionAndVerdicts(t *testing.T) {
	for _, metric := range []string{"clicks", "opens", "orders"} {
		t.Run(metric, func(t *testing.T) {
			f := newCampaignABFixture(t)
			f.status, f.sendTime = "sending", "2026-11-02T12:00:00Z"
			f.variations = append(f.variations, modelVariationFixture("variation-b", "acme-template", "email"), modelVariationFixture("variation-c", "acme-template", "email"))
			f.rows = []map[string]any{campaignABReportRow("variation-a", 1000, 80, 50, 80, 800), campaignABReportRow("variation-b", 1000, 120, 60, 150, 1200), campaignABReportRow("variation-c", 1000, 80, 50, 120, 800)}
			// Defaults to test's opens metric; explicit clicks/orders must override it.
			f.settings = map[string]any{"winner_metric": "opens_unique", "winner_percentage": 0}
			campaignFixtureClient(t, f)
			args := []string{"--campaign-id", modelMessageID}
			if metric != "opens" {
				args = append(args, "--metric", metric)
			}
			out, err := executeCampaignAB(t, newCampaignsABTestResultsCmd(&rootFlags{asJSON: true, rateLimit: 10000}), args...)
			var result campaignABResults
			if decodeErr := json.Unmarshal(out, &result); err != nil || decodeErr != nil {
				t.Fatalf("error=%v output=%s", err, out)
			}
			want := "clear_winner"
			if metric == "opens" {
				want = "no_clear_winner"
				if !strings.Contains(strings.Join(result.Notes, " "), "Apple Mail Privacy Protection") {
					t.Fatalf("notes=%v", result.Notes)
				}
			}
			if result.Metric != metric || result.Verdict != want || result.Leader != "variation-b" || len(result.PValues) != 2 || result.KlaviyoWinnerInferred != nil {
				t.Fatalf("result=%s", out)
			}
		})
	}
}

func TestCampaignABWindowCapsAndUsesCalendarDays(t *testing.T) {
	t.Setenv("KLAVIYO_TIMEZONE", "America/Chicago")
	previousNow := klaviyoWindowNow
	klaviyoWindowNow = func() time.Time { return time.Date(2026, 11, 9, 18, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { klaviyoWindowNow = previousNow })
	window, err := campaignABWindow("2024-01-01T12:00:00Z")
	if err != nil || !window.Clamped || window.Start.Hour() != 0 || window.Start.Minute() != 0 || window.End.Sub(window.Start) > klaviyoAnalyticsMaxWindow {
		t.Fatalf("window=%+v error=%v", window, err)
	}
	if _, err := campaignABWindow(""); err == nil {
		t.Fatal("accepted missing send time")
	}
}

func TestCampaignABResultsLabelsSurviveReorderedVariations(t *testing.T) {
	f := newCampaignABFixture(t)
	f.settings = map[string]any{"winner_metric": "clicks_unique"}
	a := f.variations[0]
	a["id"] = "variation-z"
	b := modelVariationFixture("variation-a", "acme-template", "email")
	c := modelVariationFixture("variation-b", "acme-template", "email")
	anyPath(a, "attributes.definition").(map[string]any)["name"] = "Acme launch - Styled"
	anyPath(b, "attributes.definition").(map[string]any)["name"] = "Acme launch - Plain Text"
	anyPath(c, "attributes.definition").(map[string]any)["name"] = "Acme launch - HTML"
	a["attributes"].(map[string]any)["created"] = "2026-10-14T17:53:08Z"
	b["attributes"].(map[string]any)["created"] = "2026-10-14T18:06:43Z"
	c["attributes"].(map[string]any)["created"] = "2026-10-14T18:06:43Z"
	f.variations = []map[string]any{c, a, b}
	campaignFixtureClient(t, f)
	out, err := executeCampaignAB(t, newCampaignsABTestResultsCmd(&rootFlags{asJSON: true, rateLimit: 10000}), "--campaign-id", modelMessageID)
	var result campaignABResults
	if decodeErr := json.Unmarshal(out, &result); err != nil || decodeErr != nil {
		t.Fatalf("error=%v output=%s", err, out)
	}
	for i, id := range []string{"variation-z", "variation-a", "variation-b"} {
		if result.Variations[i].VariationID != id || result.Variations[i].Label != string(rune('A'+i)) {
			t.Fatalf("labels=%+v", result.Variations)
		}
	}
}

func TestCampaignABResultsConversionMetricOverride(t *testing.T) {
	for _, metric := range []string{"clicks", "opens", "orders"} {
		for _, dryRun := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/dry-run=%t", metric, dryRun), func(t *testing.T) {
				f := newCampaignABFixture(t)
				f.status, f.sendTime = "sent", "2026-11-02T12:00:00Z"
				f.settings = map[string]any{"winner_metric": "placed_orders_unique", "winner_percentage": 0}
				f.variations = append(f.variations, modelVariationFixture("variation-b", "acme-template", "email"))
				f.rows = []map[string]any{campaignABReportRow("variation-a", 1000, 120, 200, 120, 600), campaignABReportRow("variation-b", 1000, 80, 100, 80, 400)}
				campaignFixtureClient(t, f)
				out, err := executeCampaignAB(t, newCampaignsABTestResultsCmd(&rootFlags{asJSON: true, dryRun: dryRun, rateLimit: 10000}),
					"--campaign-id", modelMessageID, "--metric", metric, "--conversion-metric-id", " acme-custom-order-metric ")
				if err != nil || f.metricReads != 0 {
					t.Fatalf("error=%v metric reads=%d output=%s", err, f.metricReads, out)
				}
				var body map[string]any
				if dryRun {
					var preview map[string]any
					if err := json.Unmarshal(out, &preview); err != nil {
						t.Fatal(err)
					}
					if preview["dry_run"] != true || len(f.writes) != 0 {
						t.Fatalf("output=%s writes=%v", out, f.writes)
					}
					body = preview["report_body"].(map[string]any)
				} else {
					if len(f.writes) != 1 {
						t.Fatalf("reports=%v", f.writes)
					}
					body = f.writes[0].body
					var result campaignABResults
					if err := json.Unmarshal(out, &result); err != nil {
						t.Fatal(err)
					}
					v := result.Variations[0]
					if result.Metric != metric || v.Orders == nil || *v.Orders != 120 || v.Revenue == nil || *v.Revenue != 600 || v.RevenuePerRecipient == nil || *v.RevenuePerRecipient != .6 {
						t.Fatalf("result=%s", out)
					}
					human, err := executeCampaignAB(t, &cobra.Command{RunE: func(cmd *cobra.Command, _ []string) error {
						return printCampaignABResults(cmd, &rootFlags{}, result)
					}})
					if err != nil || !strings.Contains(strings.Join(strings.Fields(string(human)), " "), "120 600.00 0.6000") {
						t.Fatalf("error=%v human output=%s", err, human)
					}
				}
				if anyPath(body, "data.attributes.conversion_metric_id") != "acme-custom-order-metric" {
					t.Fatalf("report body=%v", body)
				}
			})
		}
	}
}

func TestCampaignABResultsEngagementFallback(t *testing.T) {
	for _, tc := range []struct {
		name, metric, winnerMetric string
		ambiguous, missingRow      bool
	}{
		{name: "explicit clicks", metric: "clicks", winnerMetric: "placed_orders_unique"},
		{name: "explicit opens", metric: "opens", winnerMetric: "placed_orders_unique"},
		{name: "winner clicks", winnerMetric: "clicks_unique"},
		{name: "winner opens", winnerMetric: "opens_unique"},
		{name: "ambiguous orders and missing variation", winnerMetric: "clicks_unique", ambiguous: true, missingRow: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCampaignABFixture(t)
			f.status, f.sendTime = "sent", "2026-11-02T12:00:00Z"
			f.settings = map[string]any{"winner_metric": tc.winnerMetric, "winner_percentage": 0}
			f.variations = append(f.variations, modelVariationFixture("variation-b", "acme-template", "email"))
			f.metrics = []map[string]any{{"id": "acme-received-metric", "attributes": map[string]any{"name": "Received Email"}}}
			if tc.ambiguous {
				for _, id := range []string{"custom-orders-a", "custom-orders-b"} {
					f.metrics = append(f.metrics, map[string]any{"id": id, "attributes": map[string]any{"name": "Placed Order"}})
				}
			}
			f.rows = []map[string]any{campaignABReportRow("variation-a", 1000, 120, 200, 900, 9000)}
			if !tc.missingRow {
				f.rows = append(f.rows, campaignABReportRow("variation-b", 1000, 80, 100, 800, 8000))
			}
			campaignFixtureClient(t, f)
			args := []string{"--campaign-id", modelMessageID}
			if tc.metric != "" {
				args = append(args, "--metric", tc.metric)
			}
			out, err := executeCampaignAB(t, newCampaignsABTestResultsCmd(&rootFlags{asJSON: true, rateLimit: 10000}), args...)
			var result campaignABResults
			if decodeErr := json.Unmarshal(out, &result); err != nil || decodeErr != nil {
				t.Fatalf("error=%v output=%s", err, out)
			}
			if len(f.writes) != 1 || len(result.Variations) != 2 {
				t.Fatalf("reports=%v output=%s", f.writes, out)
			}
			attrs := anyPath(f.writes[0].body, "data.attributes")
			wantStats := []any{"recipients", "opens_unique", "open_rate", "clicks_unique", "click_rate"}
			if anyPath(attrs, "conversion_metric_id") != "acme-received-metric" || !reflect.DeepEqual(anyPath(attrs, "statistics"), wantStats) {
				t.Fatalf("report attributes=%v", attrs)
			}
			wantMetric := tc.metric
			if wantMetric == "" {
				wantMetric = "clicks"
				if tc.winnerMetric == "opens_unique" {
					wantMetric = "opens"
				}
			}
			wantVerdict := "clear_winner"
			if tc.missingRow {
				wantVerdict = "no_clear_winner"
			}
			if result.Metric != wantMetric || result.Verdict != wantVerdict || result.Leader != "variation-a" || len(result.PValues) != 1 || result.Variations[0].Recipients != 1000 || result.Variations[0].ClicksUnique != 120 || result.Variations[0].OpensUnique != 200 {
				t.Fatalf("result=%s", out)
			}
			var payload map[string]any
			if err := json.Unmarshal(out, &payload); err != nil {
				t.Fatal(err)
			}
			for _, variation := range mapSlice(payload["variations"]) {
				for _, field := range []string{"orders", "revenue", "revenue_per_recipient"} {
					if value, exists := variation[field]; !exists || value != nil {
						t.Fatalf("%s must be present and null: %s", field, out)
					}
				}
			}
			if !strings.Contains(strings.Join(result.Notes, " "), "Orders, revenue and revenue per recipient are unavailable") {
				t.Fatalf("missing conversion note: %s", out)
			}
			var human bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&human)
			if err := printCampaignABResults(cmd, &rootFlags{}, result); err != nil || strings.Count(human.String(), "unavailable") != 7 {
				t.Fatalf("error=%v human output=%s", err, human.String())
			}
		})
	}
}

func TestCampaignABResultsOrdersRequireConversionMetric(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprintf("explicit=%t", explicit), func(t *testing.T) {
			f := newCampaignABFixture(t)
			f.status, f.sendTime = "sent", "2026-11-02T12:00:00Z"
			f.settings = map[string]any{"winner_metric": "placed_orders_unique"}
			if explicit {
				f.settings = map[string]any{"winner_metric": "clicks_unique"}
			}
			f.variations = append(f.variations, modelVariationFixture("variation-b", "acme-template", "email"))
			f.metrics = []map[string]any{{"id": "acme-received-metric", "attributes": map[string]any{"name": "Received Email"}}}
			campaignFixtureClient(t, f)
			args := []string{"--campaign-id", modelMessageID}
			if explicit {
				args = append(args, "--metric", "orders")
			}
			out, err := executeCampaignAB(t, newCampaignsABTestResultsCmd(&rootFlags{asJSON: true, rateLimit: 10000}), args...)
			if err == nil || !strings.Contains(err.Error(), "--conversion-metric-id") || len(f.writes) != 0 || f.metricReads != 1 || len(out) != 0 {
				t.Fatalf("error=%v output=%s reports=%v metric reads=%d", err, out, f.writes, f.metricReads)
			}
		})
	}
}
