// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/store"
	"github.com/spf13/cobra"
)

func TestCampaignReviewLegacyCreateAndStdin(t *testing.T) {
	for _, stdin := range []bool{false, true} {
		t.Run(fmt.Sprint(stdin), func(t *testing.T) {
			var posted map[string]any
			campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != "/api/campaigns" || r.Header.Get("revision") != "2026-10-15" {
					t.Errorf("request=%s %s", r.Method, r.URL)
				}
				dec := json.NewDecoder(r.Body)
				dec.UseNumber()
				if err := dec.Decode(&posted); err != nil {
					t.Fatal(err)
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"data":{"id":"new-campaign","type":"campaign"}}`)
			}))
			cmd := newCampaignsCreateCmd(&rootFlags{asJSON: true, noCache: true})
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			audience := `{"included":["list","segment"],"excluded":["excluded"]}`
			if stdin {
				original := `{"data":{"type":"campaign","attributes":{"definition":{"name":"Modern","send_settings":{"send_strategy":"simple","send_timezone":"US/Eastern"}},"audiences":[{"temporary_id":"aud","included":["list"],"excluded":[]}],"messages":[{"audience_temporary_id":"aud","name":"Modern message","variations":[{"definition":{"details":{"channel":"email","subject":"Subject"}}}]}]}},"meta":{"large":9007199254740993}}`
				body, err := os.CreateTemp(t.TempDir(), "create-stdin-*.json")
				if err != nil {
					t.Fatal(err)
				}
				defer body.Close()
				if _, err := body.WriteString(original); err != nil {
					t.Fatal(err)
				}
				if _, err := body.Seek(0, 0); err != nil {
					t.Fatal(err)
				}
				previous := os.Stdin
				os.Stdin = body
				defer func() { os.Stdin = previous }()
				cmd.SetArgs([]string{"--stdin"})
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
				var want map[string]any
				dec := json.NewDecoder(strings.NewReader(original))
				dec.UseNumber()
				if err := dec.Decode(&want); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(posted, want) {
					t.Fatalf("stdin changed: %#v", posted)
				}
			} else {
				cmd.SetArgs([]string{"--data-type", "campaign", "--data-attributes-name", "Legacy", "--data-attributes-audiences", audience, "--data-attributes-campaign-messages", `{"data":[{"type":"campaign-message","attributes":{"definition":{"channel":"email","content":{"subject":"Subject","preview_text":"Preview","from_email":"sender@example.com","from_label":"Sender","reply_to_email":"reply@example.com"}}}}]}`})
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
				attrs := anyPath(posted, "data.attributes").(map[string]any)
				for _, legacy := range []string{"name", "campaign-messages", "send_options", "send_strategy", "tracking_options"} {
					if _, exists := attrs[legacy]; exists {
						t.Fatalf("legacy field %s escaped translation", legacy)
					}
				}
				audiences, messages := mapSlice(attrs["audiences"]), mapSlice(attrs["messages"])
				if stringFromMapPath(posted, "data.attributes.definition.name") != "Legacy" || len(audiences) != 1 || len(messages) != 1 || audiences[0]["temporary_id"] != messages[0]["audience_temporary_id"] {
					t.Fatalf("body=%#v", posted)
				}
				details := anyPath(mapSlice(messages[0]["variations"])[0], "definition.details")
				if anyPath(details, "subject") != "Subject" || anyPath(details, "channel") != "email" || len(anySlice(audiences[0]["excluded"])) != 1 {
					t.Fatalf("body=%#v", posted)
				}
			}
		})
	}
}

func TestCampaignReviewCreateRejectsUnmappableFlagsAndAmbiguousLegacyBody(t *testing.T) {
	for _, flag := range []string{"send-options", "send-strategy", "tracking-options"} {
		cmd := newCampaignsCreateCmd(&rootFlags{asJSON: true})
		cmd.SetErr(io.Discard)
		cmd.SetOut(io.Discard)
		cmd.SetArgs([]string{"--data-attributes-" + flag, `{}`})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unknown flag") {
			t.Fatalf("flag %s error=%v", flag, err)
		}
		if !strings.Contains(cmd.Long, "--data-attributes-"+flag) {
			t.Fatalf("removed flag %s undocumented", flag)
		}
	}
	for _, body := range []string{
		`{"data":{"attributes":{"name":"A","definition":{"name":"B"}}}}`,
		`{"data":{"attributes":{"name":"A","audiences":{"included":[42]},"campaign-messages":{"data":[]}}}}`,
		`{"data":{"attributes":{"name":"A","audiences":{"included":["list"]},"messages":[],"campaign-messages":{"data":[]}}}}`,
		`{"data":{"attributes":{"name":"A","audiences":{"included":["list"]},"campaign-messages":{"data":[{"attributes":{"definition":{"channel":"email","send_time":"later"}}}]}}}}`,
	} {
		var decoded map[string]any
		if err := json.Unmarshal([]byte(body), &decoded); err != nil {
			t.Fatal(err)
		}
		if err := translateLegacyCampaignCreate(decoded); err == nil {
			t.Fatalf("accepted ambiguous body %s", body)
		}
	}
}

func TestCampaignReviewDeployPrevalidationAndCleanup(t *testing.T) {
	for _, tc := range []struct {
		name, flag, value string
		cleanupFails      bool
	}{
		{name: "campaign failure"}, {name: "cleanup failure", cleanupFails: true},
		{name: "bad timezone", flag: "--send-timezone", value: "Not/AZone"},
		{name: "bad sender", flag: "--from-email", value: "invalid"},
		{name: "bad reply", flag: "--reply-to", value: "invalid"},
		{name: "blank name", flag: "--campaign-name", value: "   "},
		{name: "existing message", flag: "--message-id", value: "old-message"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := []string{}
			campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r.Method+" "+r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				switch r.Method + " " + r.URL.Path {
				case "POST /api/templates":
					fmt.Fprint(w, `{"data":{"type":"template","id":"created-template"}}`)
				case "POST /api/campaigns":
					w.WriteHeader(400)
					fmt.Fprint(w, `{"errors":[{"detail":"campaign rejected"}]}`)
				case "DELETE /api/templates/created-template":
					if tc.cleanupFails {
						w.WriteHeader(403)
						fmt.Fprint(w, `{"errors":[{"detail":"cleanup denied"}]}`)
					} else {
						w.WriteHeader(204)
					}
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
					http.NotFound(w, r)
				}
			}))
			args := []string{"--template-html", "{% unsubscribe %}", "--campaign-name", "Draft", "--subject", "Subject", "--from-email", "sender@example.com", "--from-label", "Sender", "--reply-to", "reply@example.com", "--segment-id", "segment", "--send-timezone", "US/Eastern"}
			if tc.flag != "" {
				args = append(args, tc.flag, tc.value)
			}
			cmd := newCampaignsDeployCmd(&rootFlags{asJSON: true, noCache: true})
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(args)
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected validation or campaign failure")
			}
			if tc.flag != "" {
				if len(requests) != 0 {
					t.Fatalf("failed validation created remote artifacts: %v", requests)
				}
				return
			}
			if !reflect.DeepEqual(requests, []string{"POST /api/templates", "POST /api/campaigns", "DELETE /api/templates/created-template"}) {
				t.Fatalf("cleanup requests=%v", requests)
			}
			if tc.cleanupFails {
				if !strings.Contains(err.Error(), "cleanup of created template created-template also failed") {
					t.Fatalf("cleanup error=%v", err)
				}
			} else if !strings.Contains(err.Error(), "created template created-template was deleted") {
				t.Fatalf("cleanup success error=%v", err)
			}
		})
	}
}

func TestCampaignReviewGeneratedGetIDHintsWithoutFallback(t *testing.T) {
	calls := 0
	campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" || r.URL.Path != "/api/campaigns/"+modelMessageID || r.Header.Get("revision") != "2026-10-15" {
			t.Errorf("generated request changed: %s %s", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		fmt.Fprint(w, `{"errors":[{"detail":"not found"}]}`)
	}))
	cmd := newCampaignsGetIdCmd(&rootFlags{asJSON: true, noCache: true, dataSource: "live"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{modelMessageID})
	err := cmd.Execute()
	const hint = "IDs from before revision 2026-10-15 are campaign message IDs now: try `campaign-messages get-id <id>` or `campaigns get-id <container id>`"
	if err == nil || !strings.Contains(err.Error(), hint) || ExitCode(err) != 3 || calls != 1 {
		t.Fatalf("error=%v calls=%d", err, calls)
	}
}

func TestCampaignReviewTemplatePaginationAndCampaignDiscovery(t *testing.T) {
	f := &campaignModelFixture{t: t, bodies: map[string]map[string]any{}}
	cursors := []string{}
	campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimRight(r.URL.Path, "/") != "/api/templates" {
			f.ServeHTTP(w, r)
			return
		}
		if r.URL.Query().Get("page[size]") != "10" {
			t.Errorf("template page size=%s", r.URL.RawQuery)
		}
		cursor := r.URL.Query().Get("page[cursor]")
		cursors = append(cursors, cursor)
		w.Header().Set("Content-Type", "application/json")
		if cursor == "" {
			fmt.Fprint(w, `{"data":[{"id":"first","type":"template","attributes":{"name":"First","html":"No matching URL"}}],"links":{"next":"https://a.klaviyo.com/api/templates/?page%5Bcursor%5D=last&page%5Bsize%5D=10"}}`)
			return
		}
		fmt.Fprint(w, `{"data":[{"id":"last","type":"template","attributes":{"name":"Last","html":"old.jpg"}}],"links":{"next":null}}`)
	}))
	cmd := newTemplatesUpdateImageCmd(&rootFlags{asJSON: true, noCache: true})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--old-url", "old.jpg", "--new-url", "new.jpg", "--include-campaigns"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cursors, []string{"", "last"}) || f.bodies["/api/templates/last"] == nil || f.bodies["/api/templates/TPL0001"] == nil {
		t.Fatalf("pages=%v patches=%v output=%s", cursors, f.bodies, &out)
	}
}

func TestCampaignReviewSinceSyncKeepsMessageAnalytics(t *testing.T) {
	f := &campaignModelFixture{t: t, bodies: map[string]map[string]any{}}
	c := campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/campaigns" && r.URL.Query().Get("filter") != "greater-or-equal(updated_at,2026-11-01T00:00:00Z)" {
			t.Errorf("campaign since query=%s", r.URL.RawQuery)
		}
		f.ServeHTTP(w, r)
	}))
	db, err := store.Open(filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	res := syncResource(context.Background(), c, db, "campaigns", "2026-11-01T00:00:00Z", false, 0, false, false, nil, io.Discard)
	if res.Err != nil || res.Warn != nil {
		t.Fatalf("sync=%+v", res)
	}
	rows, err := localCampaignAnalyticsRows(context.Background(), db)
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%v error=%v", rows, err)
	}
	var row map[string]any
	if err := json.Unmarshal(rows[0], &row); err != nil {
		t.Fatal(err)
	}
	if stringFromMapPath(row, "id") != modelMessageID || stringFromMapPath(row, "attributes.subject") != "A useful subject" || stringFromMapPath(row, "attributes.status") != "draft" {
		t.Fatalf("analytics=%v", row)
	}
	found := map[string]bool{}
	for _, resource := range defaultSyncResources() {
		found[resource] = true
	}
	if !found["campaigns"] || !found["campaign-messages"] {
		t.Fatal("default sync omitted campaign resources")
	}
}

func TestCampaignReviewCollectionCommandsUseMessageModel(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(*rootFlags) *cobra.Command
		args  []string
		want  string
	}{
		{"targeting", newReportTargetingCmd, []string{"--conversion-metric-id", "orders"}, "VIP"},
		{"revenue campaign", newRevenuePerEmailCmd, []string{"--by", "campaign"}, modelMessageID},
		{"revenue all", newRevenuePerEmailCmd, []string{"--by", "all"}, modelMessageID},
		{"subject campaign", newSubjectLineAnalysisCmd, []string{"--source", "campaign"}, "A useful subject"},
		{"subject all", newSubjectLineAnalysisCmd, []string{"--source", "all"}, "A useful subject"},
		{"template audit", newTemplatesAuditCmd, nil, "issue_count"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &campaignModelFixture{t: t, bodies: map[string]map[string]any{}}
			campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/metrics":
					fmt.Fprint(w, `{"data":[{"id":"orders","attributes":{"name":"Placed Order","integration":{"name":"Shopify"}}},{"id":"received","attributes":{"name":"Received Email"}},{"id":"opened","attributes":{"name":"Opened Email"}},{"id":"clicked","attributes":{"name":"Clicked Email"}}],"links":{"next":null}}`)
				case "/api/campaign-messages":
					m := modelMessageFixture(modelMessageID, "sent")
					anyPath(m, "attributes.definition.send_options").(map[string]any)["send_time"] = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
					w.Write(modelCollectionFixture(m))
				case "/api/campaign-values-reports/":
					fmt.Fprintf(w, `{"data":{"attributes":{"results":[{"groupings":{"campaign_id":%q},"statistics":{"recipients":100,"conversion_value":500,"delivered":100,"opens_unique":50,"clicks_unique":10}}]}},"links":{"next":null}}`, modelMessageID)
				case "/api/metric-aggregates":
					fmt.Fprint(w, `{"data":{"attributes":{"data":[]}}}`)
				case "/api/segments":
					fmt.Fprint(w, `{"data":[{"id":"segment","attributes":{"name":"VIP"}}],"links":{"next":null}}`)
				case "/api/lists":
					if r.URL.Query().Get("page[size]") != "10" {
						t.Errorf("lists page size=%s", r.URL.RawQuery)
					}
					fmt.Fprint(w, `{"data":[{"id":"list","attributes":{"name":"List"}}],"links":{"next":null}}`)
				case "/api/flows":
					fmt.Fprint(w, `{"data":[],"included":[],"links":{"next":null}}`)
				case "/api/templates":
					if r.URL.Query().Get("page[size]") != "10" {
						t.Errorf("templates page size=%s", r.URL.RawQuery)
					}
					fmt.Fprint(w, `{"data":[],"links":{"next":null}}`)
				default:
					f.ServeHTTP(w, r)
				}
			}))
			cmd := tc.build(&rootFlags{asJSON: true, noCache: true})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Fatalf("%s output=%s", tc.name, &out)
			}
		})
	}
}

func TestCampaignReviewHistoricalIDScopesQAImageCloneAndGuard(t *testing.T) {
	for _, operation := range []string{"qa", "image", "clone", "guard"} {
		t.Run(operation, func(t *testing.T) {
			f := &campaignModelFixture{t: t, multi: true, html: map[string]string{"template-2": "No unsubscribe"}, bodies: map[string]map[string]any{}}
			c := campaignFixtureClient(t, f)
			var out bytes.Buffer
			switch operation {
			case "qa":
				cmd := newPlanQAGateCmd(&rootFlags{asJSON: true, noCache: true})
				cmd.SetOut(&out)
				cmd.SetErr(io.Discard)
				cmd.SetArgs([]string{"--campaign-id", modelMessageID})
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
			case "image":
				cmd := newCampaignsImageSwapCmd(&rootFlags{asJSON: true, noCache: true})
				cmd.SetOut(&out)
				cmd.SetArgs([]string{"--campaign-id", modelMessageID, "--old-url", "old.jpg", "--new-url", "new.jpg"})
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
				if f.bodies["/api/templates/TPL0001"] == nil || f.bodies["/api/templates/template-2"] != nil {
					t.Fatalf("patched outside message scope: %v", f.bodies)
				}
			case "clone":
				raw := json.RawMessage(fmt.Sprintf(`{"data":{"id":%q,"type":"campaign-message"}}`, modelMessageID))
				cmd := &cobra.Command{}
				cmd.SetErr(io.Discard)
				result, err := campaignCloneReadback(cmd, c, raw)
				if err != nil {
					t.Fatal(err)
				}
				out.Write(result)
			case "guard":
				if err := requireCampaignCompliance(c, modelMessageID); err != nil {
					t.Fatal(err)
				}
			}
			for _, request := range f.requests {
				if strings.Contains(request, "message-2") || strings.Contains(request, "/api/campaigns/"+modelContainerID+"/campaign-messages") {
					t.Fatalf("%s expanded historical ID scope: %s", operation, request)
				}
			}
			if strings.Contains(out.String(), "message-2") {
				t.Fatalf("%s output expanded scope: %s", operation, &out)
			}
		})
	}
}

func TestCampaignReviewHistoricalIDTimeDecayReadsMessageTimestamp(t *testing.T) {
	f := &campaignModelFixture{t: t, multi: true, bodies: map[string]map[string]any{}}
	sentAt := time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Second)
	c := campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("fields[campaign]") != "" {
			t.Errorf("time-decay requested old campaign fields: %s", r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/campaign-messages/" + modelMessageID:
			m := modelMessageFixture(modelMessageID, "sent")
			anyPath(m, "attributes.definition.send_options").(map[string]any)["send_time"] = sentAt.Format(time.RFC3339)
			json.NewEncoder(w).Encode(map[string]any{"data": m})
		case "/api/metrics":
			fmt.Fprint(w, `{"data":[{"id":"orders","attributes":{"name":"Placed Order"}}],"links":{"next":null}}`)
		case "/api/events":
			fmt.Fprintf(w, `{"data":[{"id":"order","attributes":{"datetime":%q,"value":25,"properties":{"$attributed_campaign_id":%q}}}],"links":{"next":null}}`, sentAt.Add(2*time.Hour).Format(time.RFC3339), modelMessageID)
		default:
			f.ServeHTTP(w, r)
		}
	}))
	result, err := campaignTimeDecay(c, modelMessageID)
	if err != nil {
		t.Fatal(err)
	}
	if result["campaign_id"] != modelMessageID || result["sent_at"] != sentAt.Format(time.RFC3339) || result["total_conversions"] != 1 || result["total_revenue"] != float64(25) {
		t.Fatalf("decay=%v", result)
	}
	for _, request := range f.requests {
		if strings.Contains(request, "message-2") {
			t.Fatalf("time decay expanded historical ID scope: %s", request)
		}
	}
}

func TestCampaignReviewQAAndGuardAllowNonEmailVariations(t *testing.T) {
	for _, channel := range []string{"sms", "push", "mobile_push", "whatsapp"} {
		t.Run(channel, func(t *testing.T) {
			f := &campaignModelFixture{t: t, bodies: map[string]map[string]any{}}
			c := campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/campaign-messages/"+modelMessageID+"/campaign-variations" {
					w.Header().Set("Content-Type", "application/json")
					w.Write(campaignVariationTestPage("non-email", "", "", channel))
					return
				}
				if strings.HasPrefix(r.URL.Path, "/api/templates/") {
					t.Error("non-email variation demanded an email template")
				}
				f.ServeHTTP(w, r)
			}))
			cmd := newPlanQAGateCmd(&rootFlags{asJSON: true, noCache: true})
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{"--campaign-id", modelMessageID})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if err := requireCampaignCompliance(c, modelMessageID); err != nil {
				t.Fatal(err)
			}
		})
	}
}
