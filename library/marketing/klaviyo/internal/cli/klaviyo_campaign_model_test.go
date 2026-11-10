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
	"path/filepath"
	"strings"
	"testing"

	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/store"
)

const modelContainerID = "01J0000000CONTAINER0000001"
const modelMessageID = "01J0000000MESSAGE000000001"
const modelVariationID = "01J0000000VARIATION0000001"

func modelMessageFixture(id, status string) map[string]any {
	return map[string]any{"id": id, "type": "campaign-message", "attributes": map[string]any{"created": "2026-11-01T00:00:00Z", "updated": "2026-11-09T00:00:00Z", "definition": map[string]any{"name": "Launch " + id, "status": status, "send_options": map[string]any{"send_time": nil}}}, "relationships": map[string]any{"campaign": map[string]any{"data": map[string]any{"id": modelContainerID, "type": "campaign"}}}}
}

func modelAudienceFixture(included, excluded []string) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{"data": map[string]any{"id": "audience", "type": "campaign-audience", "attributes": map[string]any{"definition": map[string]any{"name": "VIP", "included": included, "excluded": excluded, "priority": 0}}}})
	return raw
}

func modelVariationFixture(id, template, channel string) map[string]any {
	return map[string]any{"id": id, "type": "campaign-variation", "attributes": map[string]any{"definition": map[string]any{"name": "Variation " + id, "details": map[string]any{"channel": channel, "template_id": template, "subject": "A useful subject", "preview_text": "Preview", "from_email": "sender@example.com", "from_label": "Sender", "reply_to_email": "reply@example.com"}}}}
}

func modelCollectionFixture(data ...map[string]any) json.RawMessage {
	if data == nil {
		data = []map[string]any{}
	}
	raw, _ := json.Marshal(map[string]any{"data": data, "links": map[string]any{"next": nil}})
	return raw
}

func campaignMessageTestPage(id, name, status, sendTime string) json.RawMessage {
	m := modelMessageFixture(id, status)
	definition := anyPath(m, "attributes.definition").(map[string]any)
	definition["name"] = name
	if sendTime != "" {
		definition["send_options"].(map[string]any)["send_time"] = sendTime
	}
	return modelCollectionFixture(m)
}

func campaignVariationTestPage(id, template, subject, channel string) json.RawMessage {
	v := modelVariationFixture(id, template, channel)
	anyPath(v, "attributes.definition.details").(map[string]any)["subject"] = subject
	return modelCollectionFixture(v)
}

type campaignModelFixture struct {
	t                                 *testing.T
	multi, paginate, assigned, unsafe bool
	variationPages, multipleEmail     bool
	html                              map[string]string
	requests                          []string
	bodies                            map[string]map[string]any
}

func (f *campaignModelFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.requests = append(f.requests, r.Method+" "+r.URL.RequestURI())
	w.Header().Set("Content-Type", "application/json")
	if r.Header.Get("revision") != "2026-10-15" {
		f.t.Errorf("revision=%s", r.Header.Get("revision"))
	}
	if strings.Contains(r.URL.Query().Get("filter"), "messages.channel") || r.URL.Query().Get("include") != "" {
		f.t.Errorf("unsupported campaign query %s", r.URL)
	}
	if r.Method != "GET" {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			f.t.Error(err)
		}
		f.bodies[r.URL.Path] = body
		switch strings.TrimRight(r.URL.Path, "/") {
		case "/api/templates":
			fmt.Fprint(w, `{"data":{"type":"template","id":"reusable"}}`)
		case "/api/campaigns", "/api/campaign-clone":
			fmt.Fprintf(w, `{"data":{"type":"campaign","id":%q}}`, modelContainerID)
		case "/api/campaign-message-assign-template":
			f.assigned = true
			fmt.Fprint(w, `{"data":{"type":"campaign-message","id":"assigned"}}`)
		default:
			fmt.Fprint(w, `{"data":{"id":"updated"}}`)
		}
		return
	}
	m := modelMessageFixture(modelMessageID, "draft")
	if f.assigned && f.unsafe {
		m = modelMessageFixture(modelMessageID, "scheduled")
	}
	m2 := modelMessageFixture("message-2", "sent")
	switch strings.TrimRight(r.URL.Path, "/") {
	case "/api/accounts":
		fmt.Fprint(w, `{"data":[{"attributes":{"timezone":"US/Eastern"}}]}`)
	case "/api/campaigns":
		fmt.Fprintf(w, `{"data":[{"type":"campaign","id":%q,"attributes":{"definition":{"name":"Container"}}}],"links":{"next":null}}`, modelContainerID)
	case "/api/campaigns/" + modelContainerID:
		fmt.Fprintf(w, `{"data":{"type":"campaign","id":%q,"attributes":{"definition":{"name":"Container"}}}}`, modelContainerID)
	case "/api/campaigns/" + modelContainerID + "/campaign-messages":
		if f.paginate && r.URL.Query().Get("page[cursor]") == "" {
			raw, _ := json.Marshal(map[string]any{"data": []map[string]any{m}, "links": map[string]any{"next": "https://a.klaviyo.com/api/campaigns/" + modelContainerID + "/campaign-messages/?page%5Bcursor%5D=next"}})
			w.Write(raw)
		} else if f.paginate {
			w.Write(modelCollectionFixture(m2))
		} else if f.multi {
			w.Write(modelCollectionFixture(m, m2))
		} else {
			w.Write(modelCollectionFixture(m))
		}
	case "/api/campaign-messages":
		w.Write(modelCollectionFixture(m))
	case "/api/campaign-messages/" + modelMessageID:
		json.NewEncoder(w).Encode(map[string]any{"data": m})
	case "/api/campaign-messages/" + modelMessageID + "/campaign-audience", "/api/campaign-messages/message-2/campaign-audience":
		w.Write(modelAudienceFixture([]string{"list", "segment"}, []string{"excluded"}))
	case "/api/campaign-messages/" + modelMessageID + "/campaign-variations":
		template := "TPL0001"
		if f.assigned {
			template = "clone-template"
		}
		if f.variationPages && r.URL.Query().Get("page[cursor]") == "" {
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{modelVariationFixture(modelVariationID, template, "email")}, "links": map[string]any{"next": "/api/campaign-messages/" + modelMessageID + "/campaign-variations?page%5Bcursor%5D=variation-next"}})
		} else if f.variationPages {
			w.Write(modelCollectionFixture(modelVariationFixture("variation-extra", "template-extra", "email")))
		} else if f.multipleEmail {
			w.Write(modelCollectionFixture(modelVariationFixture(modelVariationID, template, "email"), modelVariationFixture("variation-extra", "template-extra", "email")))
		} else {
			w.Write(modelCollectionFixture(modelVariationFixture(modelVariationID, template, "email")))
		}
	case "/api/campaign-messages/message-2/campaign-variations":
		w.Write(modelCollectionFixture(modelVariationFixture("variation-2", "template-2", "email"), modelVariationFixture("sms-variation", "", "sms")))
	case "/api/campaign-variations/" + modelVariationID:
		json.NewEncoder(w).Encode(map[string]any{"data": modelVariationFixture(modelVariationID, "TPL0001", "email")})
	default:
		if strings.HasPrefix(r.URL.Path, "/api/templates/") {
			id := strings.TrimPrefix(r.URL.Path, "/api/templates/")
			html := `<p>old.jpg</p>{% unsubscribe %}`
			if replacement, ok := f.html[id]; ok {
				html = replacement
			}
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"type": "template", "id": id, "attributes": map[string]any{"html": html}}})
		} else {
			w.WriteHeader(404)
			fmt.Fprint(w, `{"errors":[{"detail":"not found"}]}`)
		}
	}
}

// httptest recorders exercise HTTP wire requests without requiring a listening socket.
func campaignFixtureClient(t *testing.T, handler http.Handler) *client.Client {
	t.Helper()
	t.Setenv("KLAVIYO_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	t.Setenv("KLAVIYO_BASE_URL", "https://fixture.invalid")
	t.Setenv("KLAVIYO_API_KEY", "fixture-key")
	previous, hooks := registeredPlatformSource, clientHooks
	registeredPlatformSource = nil
	t.Cleanup(func() { registeredPlatformSource, clientHooks = previous, hooks })
	registerClientHook(func(c *client.Client) error {
		c.HTTPClient.Transport = klaviyoCloneTransport(func(r *http.Request) (*http.Response, error) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, r)
			response := recorder.Result()
			response.Request = r
			return response, nil
		})
		return nil
	})
	c, err := (&rootFlags{noCache: true, rateLimit: 10000}).newClient()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCampaignModelResolverContainerAndHistoricalMessage(t *testing.T) {
	f := &campaignModelFixture{t: t, multi: true, paginate: true, variationPages: true, bodies: map[string]map[string]any{}}
	c := campaignFixtureClient(t, f)
	model, err := resolveCampaign(c, modelContainerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Messages) != 2 || len(model.Messages[0].Variations) != 2 || model.Messages[0].Variations[0].TemplateID != "TPL0001" || model.Messages[1].Status != "sent" || len(model.Messages[1].Variations) != 2 {
		t.Fatalf("model=%+v", model)
	}
	if len(anyStringSlice(model.Messages[0].Audience["excluded"])) != 1 {
		t.Fatal("lost exclusions")
	}
	f.requests = nil
	model, err = resolveCampaign(c, modelMessageID)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Messages) != 1 || model.Messages[0].ID != modelMessageID || stringFromMapPath(model.Campaign, "attributes.definition.name") != "Container" {
		t.Fatalf("message scope=%+v", model)
	}
	for _, request := range f.requests {
		if strings.Contains(request, "message-2") || strings.Contains(request, "/campaigns/"+modelContainerID+"/campaign-messages") {
			t.Fatal("expanded historical message scope", request)
		}
	}
}

func TestCampaignModelRejectsBrokenPaginationAndDoesNotFallbackOnForbidden(t *testing.T) {
	for _, test := range []struct {
		name, next string
		code       int
	}{{"loop", "/api/campaigns/x/campaign-messages?page[cursor]=same", 200}, {"wrong relationship", "/api/templates", 200}, {"forbidden", "", 403}} {
		t.Run(test.name, func(t *testing.T) {
			count := 0
			c := campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count++
				w.Header().Set("Content-Type", "application/json")
				if test.code != 200 {
					w.WriteHeader(test.code)
					fmt.Fprint(w, `{"errors":[{"detail":"forbidden"}]}`)
					return
				}
				fmt.Fprintf(w, `{"data":[{"id":"m"}],"links":{"next":%q}}`, test.next)
			}))
			var err error
			if test.code == 403 {
				_, err = resolveCampaign(c, "x")
				if count != 1 {
					t.Fatal("403 triggered fallback")
				}
			} else {
				_, err = campaignCollection(c, "/api/campaigns/x/campaign-messages", nil)
			}
			if err == nil {
				t.Fatal("expected pagination/API error")
			}
		})
	}
}

func TestCampaignDeployOmnichannelBodyAndDraftVerification(t *testing.T) {
	for _, unsafe := range []bool{false, true} {
		t.Run(fmt.Sprint(unsafe), func(t *testing.T) {
			f := &campaignModelFixture{t: t, unsafe: unsafe, bodies: map[string]map[string]any{}}
			campaignFixtureClient(t, f)
			cmd := newCampaignsDeployCmd(&rootFlags{asJSON: true, noCache: true})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{"--template-html", `{% unsubscribe %}`, "--campaign-name", "Launch", "--list-id", "list", "--segment-id", "segment", "--exclude", "excluded", "--exclude", "excluded-segment", "--subject", "Subject", "--preview-text", "Preview", "--from-email", "sender@example.com", "--from-label", "Sender", "--reply-to", "reply@example.com"})
			err := cmd.Execute()
			if unsafe {
				if err == nil || !strings.Contains(err.Error(), "draft verification failed") {
					t.Fatalf("error=%v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			body := f.bodies["/api/campaigns"]
			if stringFromMapPath(body, "data.attributes.definition.send_settings.send_timezone") != "US/Eastern" || stringFromMapPath(body, "data.attributes.definition.send_settings.send_strategy") != "simple" {
				t.Fatalf("settings=%#v", body)
			}
			attrs := anyPath(body, "data.attributes").(map[string]any)
			audience := mapSlice(attrs["audiences"])[0]
			if len(anySlice(audience["included"])) != 2 || len(anySlice(audience["excluded"])) != 2 {
				t.Fatalf("audience=%#v", audience)
			}
			message := mapSlice(attrs["messages"])[0]
			variation := mapSlice(message["variations"])[0]
			details := anyPath(variation, "definition.details").(map[string]any)
			if _, exists := details["template_id"]; exists {
				t.Fatal("inline template_id is forbidden")
			}
			if _, exists := attrs["send_strategy"]; exists {
				t.Fatal("legacy scheduling body")
			}
			assign := f.bodies["/api/campaign-message-assign-template"]
			if stringFromMapPath(assign, "data.id") != modelVariationID || stringFromMapPath(assign, "data.relationships.template.data.id") != "reusable" {
				t.Fatalf("assignment=%#v", assign)
			}
			if !strings.Contains(out.String(), `"verified_draft": true`) || !strings.Contains(out.String(), "clone-template") {
				t.Fatalf("output=%s", out.String())
			}
			for _, req := range f.requests {
				if strings.Contains(req, "send-job") || strings.Contains(req, "schedule") {
					t.Fatal("deploy attempted scheduling", req)
				}
			}
		})
	}
}

func TestCampaignModelImageSwapAndQACheckEveryVariation(t *testing.T) {
	f := &campaignModelFixture{t: t, multi: true, multipleEmail: true, bodies: map[string]map[string]any{}}
	c := campaignFixtureClient(t, f)
	cmd := newCampaignsImageSwapCmd(&rootFlags{asJSON: true, noCache: true})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--campaign-id", modelContainerID, "--old-url", "old.jpg", "--new-url", "new.jpg"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if f.bodies["/api/templates/TPL0001"] == nil || f.bodies["/api/templates/template-2"] == nil || f.bodies["/api/templates/template-extra"] == nil {
		t.Fatal("did not patch every email template")
	}
	if _, err := swapCampaignTemplateImages(c, []string{"TPL0001", "template-2"}, "absent", "new"); err == nil {
		t.Fatal("expected no-match failure")
	}
	f.html = map[string]string{"template-2": "<p>No unsubscribe</p>"}
	qa := newPlanQAGateCmd(&rootFlags{asJSON: true, noCache: true})
	out.Reset()
	qa.SetOut(&out)
	qa.SetErr(io.Discard)
	qa.SetArgs([]string{"--campaign-id", modelContainerID})
	if err := qa.Execute(); err == nil {
		t.Fatal("QA must fail for the second message")
	}
	if !strings.Contains(out.String(), `"message_id": "message-2"`) || !strings.Contains(out.String(), `"subject": "A useful subject"`) {
		t.Fatalf("QA output=%s", out.String())
	}
	if err := requireCampaignCompliance(c, modelContainerID); err == nil || !strings.Contains(err.Error(), "message-2") {
		t.Fatalf("send guard=%v", err)
	}
}

func TestCampaignModelAssignTemplateMessageIDAndCloneReadback(t *testing.T) {
	f := &campaignModelFixture{t: t, bodies: map[string]map[string]any{}}
	campaignFixtureClient(t, f)
	cmd := newCampaignMessageAssignTemplatePromotedCmd(&rootFlags{asJSON: true, noCache: true})
	cmd.SetIn(strings.NewReader(fmt.Sprintf(`{"data":{"type":"campaign-message","id":%q,"relationships":{"template":{"data":{"type":"template","id":"reusable"}}}}}`, modelMessageID)))
	cmd.SetOut(io.Discard)
	cmd.SetArgs([]string{"--stdin"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if stringFromMapPath(f.bodies["/api/campaign-message-assign-template"], "data.id") != modelVariationID {
		t.Fatal("message ID was not resolved to variation")
	}
	f.multi = true
	clone := newCampaignClonePromotedCmd(&rootFlags{asJSON: true, noCache: true})
	var out, warnings bytes.Buffer
	clone.SetOut(&out)
	clone.SetErr(&warnings)
	clone.SetIn(strings.NewReader(`{"data":{"type":"campaign","id":"source"}}`))
	clone.SetArgs([]string{"--stdin"})
	if err := clone.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(warnings.String(), "NOT DRAFT") || !strings.Contains(out.String(), "campaign_messages") {
		t.Fatalf("clone stdout=%s warnings=%s", out.String(), warnings.String())
	}
}

func TestCampaignModelSyncAndHistoricalAnalytics(t *testing.T) {
	f := &campaignModelFixture{t: t, multi: true, bodies: map[string]map[string]any{}}
	c := campaignFixtureClient(t, f)
	db, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	legacy := json.RawMessage(fmt.Sprintf(`{"id":%q,"attributes":{"name":"Old","status":"Draft","send_time":null}}`, modelMessageID))
	if err := db.UpsertCampaigns(legacy); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertCampaigns(json.RawMessage(`{"id":"history-only","attributes":{"name":"History","status":"Sent"}}`)); err != nil {
		t.Fatal(err)
	}
	result := syncResource(context.Background(), c, db, "campaigns", "", true, 0, false, false, nil, io.Discard)
	if result.Err != nil || result.Warn != nil {
		t.Fatalf("sync=%+v", result)
	}
	messageResult := syncResource(context.Background(), c, db, "campaign-messages", "", true, 0, false, false, nil, io.Discard)
	if messageResult.Err != nil || messageResult.Warn != nil {
		t.Fatalf("message sync=%+v", messageResult)
	}
	rows, err := localCampaignAnalyticsRows(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows=%d want two messages plus preserved history", len(rows))
	}
	found := false
	for _, raw := range rows {
		var data map[string]any
		json.Unmarshal(raw, &data)
		if stringFromMapPath(data, "id") == modelMessageID {
			found = stringFromMapPath(data, "attributes.status") == "draft" && stringFromMapPath(data, "attributes.channel") == "email" && stringFromMapPath(data, "attributes.subject") == "A useful subject"
		}
	}
	if !found {
		t.Fatal("historical key did not acquire message-level analytics")
	}
	var out bytes.Buffer
	cmd := newAnalyticsCmd(&rootFlags{asJSON: true})
	cmd.SetOut(&out)
	cmd.SetContext(context.Background())
	if err := runGroupBy(cmd, &out, db, "campaigns", "status", 0, &rootFlags{asJSON: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "draft") || !strings.Contains(out.String(), "Sent") {
		t.Fatalf("analytics output=%s", out.String())
	}
}

func TestCampaignModelRejectsAmbiguousAssignAndScheduledDraft(t *testing.T) {
	f := &campaignModelFixture{t: t, multipleEmail: true, bodies: map[string]map[string]any{}}
	c := campaignFixtureClient(t, f)
	if _, err := resolveAssignVariation(c, modelMessageID); err == nil || !strings.Contains(err.Error(), "2 email variations") {
		t.Fatalf("ambiguous assign error=%v", err)
	}
	if id, err := resolveAssignVariation(c, modelVariationID); err != nil || id != modelVariationID {
		t.Fatalf("direct variation=%s err=%v", id, err)
	}
	for _, m := range []campaignModelMessage{{ID: "m", Status: "draft", SendTime: "2026-11-30T12:00:00Z"}, {ID: "m", Status: "sent"}, {ID: "m"}} {
		if err := verifyCampaignDraft(&campaignModel{Messages: []campaignModelMessage{m}}); err == nil {
			t.Fatalf("accepted unsafe draft %+v", m)
		}
	}
}

func TestCampaignModelDeployDryRunMakesNoRequests(t *testing.T) {
	f := &campaignModelFixture{t: t, bodies: map[string]map[string]any{}}
	campaignFixtureClient(t, f)
	cmd := newCampaignsDeployCmd(&rootFlags{asJSON: true, dryRun: true})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--template-html", "{% unsubscribe %}", "--campaign-name", "Draft", "--segment-id", "segment", "--subject", "Subject", "--from-email", "sender@example.com", "--from-label", "Sender"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(f.requests) != 0 || !strings.Contains(out.String(), "assign-template") {
		t.Fatalf("dry-run requests=%v output=%s", f.requests, &out)
	}
}

func TestCampaignModelScheduledConflictsUseMessageFilter(t *testing.T) {
	f := &campaignModelFixture{t: t, bodies: map[string]map[string]any{}}
	campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/campaign-messages" {
			f.ServeHTTP(w, r)
			return
		}
		if r.URL.Query().Get("filter") != "equals(status,'scheduled')" {
			t.Errorf("filter=%s", r.URL.RawQuery)
		}
		first, second := modelMessageFixture(modelMessageID, "scheduled"), modelMessageFixture("message-2", "scheduled")
		at := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
		anyPath(first, "attributes.definition.send_options").(map[string]any)["send_time"] = at
		anyPath(second, "attributes.definition.send_options").(map[string]any)["send_time"] = at
		w.Header().Set("Content-Type", "application/json")
		w.Write(modelCollectionFixture(first, second))
	}))
	cmd := newCampaignsScheduleConflictsCmd(&rootFlags{asJSON: true, noCache: true})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["conflict_count"] != float64(1) || !strings.Contains(out.String(), modelMessageID) {
		t.Fatalf("conflicts=%s", &out)
	}
}
