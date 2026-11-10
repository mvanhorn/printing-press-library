package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/store"
	"github.com/spf13/cobra"
)

func TestNovelCommandsRegistered(t *testing.T) {
	root := RootCmd()
	paths := [][]string{
		{"campaigns", "deploy"},
		{"campaigns", "image-swap"},
		{"flow-decay"},
		{"cohort"},
		{"attribution"},
		{"dedup"},
		{"reconcile"},
		{"flow-cannibalization"},
		{"send-fatigue"},
		{"subject-line-analysis"},
		{"optimal-send-time"},
		{"revenue-per-email"},
		{"segment-velocity"},
		{"flow-path-analysis"},
		{"campaign-time-decay"},
		{"list-quality-score"},
		{"content-fatigue"},
		{"plan", "brief-to-strategy"},
		{"plan", "qa-gate"},
		{"flows", "export"},
		{"flows", "clone"},
		{"flows", "deploy"},
		{"flows", "pause"},
		{"flows", "resume"},
		{"flows", "health"},
		{"report", "revenue"},
		{"report", "deliverability"},
		{"templates", "update-image"},
		{"coupons", "check-pools"},
		{"segments", "build"},
		{"segments", "overlap"},
		{"segments", "rfm"},
		{"flows", "audit"},
		{"campaigns", "schedule-conflicts"},
		{"profiles", "stats"},
		{"profiles", "top-spenders"},
		{"profiles", "never-purchased"},
		{"profiles", "churning"},
		{"profiles", "prune"},
		{"profiles", "export-suppressions"},
		{"report", "dashboard"},
		{"report", "open-rates"},
		{"report", "unsubscribes"},
		{"report", "spam-complaints"},
		{"report", "list-growth"},
		{"report", "domain-reputation"},
		{"report", "flow-funnel"},
		{"report", "flow-comparison"},
		{"report", "email-performance"},
		{"report", "forms"},
		{"report", "signup-sources"},
		{"report", "products"},
		{"report", "product-affinity"},
		{"report", "consent"},
		{"report", "targeting"},
		{"lists", "audit"},
		{"templates", "audit"},
		{"tags", "audit"},
		{"universal-content", "sync"},
	}
	for _, path := range paths {
		if findCommand(root, path) == nil {
			t.Fatalf("command %v not registered", path)
		}
	}
}

func TestPaginatedGetFollowsJSONAPINextLink(t *testing.T) {
	c := &fakePaginatedClient{responses: []json.RawMessage{
		rawJSON(`{"data":[{"id":"flow-1"}],"links":{"next":"https://example.test/api/flows?page%5Bcursor%5D=next-page"}}`),
		rawJSON(`{"data":[{"id":"flow-2"}],"links":{"next":null}}`),
	}}

	got, err := paginatedGet(context.Background(), c, "/api/flows", map[string]string{"page[size]": "50"}, nil, true, "page[cursor]", "", "page[size]", 50, "", "")
	if err != nil {
		t.Fatalf("paginatedGet() error = %v", err)
	}
	var items []map[string]any
	if err := json.Unmarshal(got, &items); err != nil {
		t.Fatalf("unmarshal paginated result: %v", err)
	}
	if len(items) != 2 || items[0]["id"] != "flow-1" || items[1]["id"] != "flow-2" {
		t.Fatalf("items = %#v", items)
	}
	if len(c.params) != 2 || c.params[1]["page[cursor]"] != "next-page" {
		t.Fatalf("request params = %#v", c.params)
	}
}

func TestPaginatedGetRejectsCursorCycles(t *testing.T) {
	c := &fakePaginatedClient{responses: []json.RawMessage{
		rawJSON(`{"data":[{"id":"flow-1"}],"links":{"next":"https://example.test/api/flows?page%5Bcursor%5D=A"}}`),
		rawJSON(`{"data":[{"id":"flow-2"}],"links":{"next":"https://example.test/api/flows?page%5Bcursor%5D=B"}}`),
		rawJSON(`{"data":[{"id":"flow-3"}],"links":{"next":"https://example.test/api/flows?page%5Bcursor%5D=A"}}`),
	}}

	if _, err := paginatedGet(context.Background(), c, "/api/flows", map[string]string{"page[size]": "50"}, nil, true, "page[cursor]", "", "page[size]", 50, "", ""); err == nil || !strings.Contains(err.Error(), "cursor") {
		t.Fatalf("expected cursor-cycle error, got %v", err)
	}
	if len(c.params) != 3 {
		t.Fatalf("request params = %#v", c.params)
	}
}

func TestAuditFetchersUseValidKlaviyoParametersAndAllPages(t *testing.T) {
	formsClient := &fakeCouponPoolClient{responses: []json.RawMessage{
		rawJSON(`{"data":[{"id":"form-1"}],"links":{"next":"https://example.test/api/forms?page%5Bcursor%5D=forms-2"}}`),
		rawJSON(`{"data":[{"id":"form-2"}],"links":{}}`),
	}}
	forms, err := fetchReportForms(formsClient)
	if err != nil {
		t.Fatalf("fetchReportForms() error = %v", err)
	}
	if len(forms) != 2 {
		t.Fatalf("forms = %#v", forms)
	}
	if got := formsClient.requests[0].params["fields[form]"]; got != "name,status,created_at,updated_at" {
		t.Fatalf("forms fields = %q", got)
	}
	if got := formsClient.requests[1].params["page[cursor]"]; got != "forms-2" {
		t.Fatalf("forms second-page cursor = %q", got)
	}

	templatesClient := &fakeCouponPoolClient{responses: []json.RawMessage{
		rawJSON(`{"data":[{"id":"template-1"}],"links":{}}`),
	}}
	if _, err := fetchTemplateAuditItems(templatesClient); err != nil {
		t.Fatalf("fetchTemplateAuditItems() error = %v", err)
	}
	if got := templatesClient.requests[0].params["page[size]"]; got != "10" {
		t.Fatalf("template page size = %q, want 10", got)
	}
	if got := templatesClient.requests[0].params["fields[template]"]; got != "name,updated" {
		t.Fatalf("template fields = %q", got)
	}
}

func TestListGrowthUsesCurrentKlaviyoMetricNames(t *testing.T) {
	want := []string{"Subscribed to List", "Unsubscribed from List", "Unsubscribed from Email Marketing"}
	if fmt.Sprint(listGrowthMetricNames) != fmt.Sprint(want) {
		t.Fatalf("listGrowthMetricNames = %#v, want %#v", listGrowthMetricNames, want)
	}
	rows := []map[string]any{
		{"metric": "Subscribed to List", "count": 120},
		{"metric": "Unsubscribed from List", "count": 25},
		{"metric": "Unsubscribed from Email Marketing", "count": 10},
	}
	report := buildListGrowthReport(rows, "90d")
	if got := report["net_list_growth"]; got != 95 {
		t.Fatalf("net list growth = %v, want 95", got)
	}
	if report["derived_metrics_complete"] != true {
		t.Fatalf("report = %#v", report)
	}
}

func TestListGrowthDoesNotDeriveFromMissingMetrics(t *testing.T) {
	rows := []map[string]any{
		{"metric": "Subscribed to List", "count": 120},
		{"metric": "Unsubscribed from List", "error": "metric not found"},
		{"metric": "Unsubscribed from Email Marketing", "count": 10},
	}
	report := buildListGrowthReport(rows, "90d")
	if report["net_list_growth"] != nil || report["list_unsubscriptions"] != nil {
		t.Fatalf("missing metric produced derived counts: %#v", report)
	}
	if report["derived_metrics_complete"] != false || report["warning"] == nil {
		t.Fatalf("missing metric should be explicit: %#v", report)
	}
}

func TestSubjectLineAnalysisProducesNonEmptyOutput(t *testing.T) {
	since := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	c := &fakeCouponPoolClient{
		responses: []json.RawMessage{
			rawJSON(`{"data":[{"id":"received-email","attributes":{"name":"Received Email","integration":{"name":"Klaviyo"}}}],"links":{}}`),
			campaignMessageTestPage("campaign-1", "Campaign One", "sent", "2026-06-15T12:00:00Z"),
			modelAudienceFixture([]string{"list"}, nil),
			campaignVariationTestPage("message-1", "campaign-template", "A useful subject", "email"),
		},
		postResponses: []json.RawMessage{rawJSON(`{"data":{"attributes":{"results":[{"groupings":{"campaign_id":"campaign-1"},"statistics":{"delivered":1000,"opens_unique":642,"open_rate":0.642,"clicks_unique":15,"click_rate":0.015}}]}}}`)},
	}
	result, err := subjectLineAnalysis(c, since, until, "campaign")
	if err != nil {
		t.Fatalf("subjectLineAnalysis() error = %v", err)
	}
	var out bytes.Buffer
	if err := printJSONFiltered(&out, result, &rootFlags{asJSON: true, compact: true}); err != nil {
		t.Fatalf("printJSONFiltered() error = %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("subject-line analysis produced empty stdout")
	}
	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("subject-line output is not JSON: %v", err)
	}
	if _, ok := decoded["total_emails_analyzed"]; !ok {
		t.Fatalf("subject-line output = %#v", decoded)
	}
	if decoded["total_emails_analyzed"] != float64(1) {
		t.Fatalf("subject-line output = %#v", decoded)
	}
	top := result["top_subjects"].([]map[string]any)
	if len(top) != 1 || top[0]["open_rate"] != 0.642 || top[0]["click_rate"] != 0.015 {
		t.Fatalf("subject-line rates = %#v", top)
	}
	if len(c.posts) != 1 || c.posts[0].path != "/api/campaign-values-reports/" {
		t.Fatalf("report requests = %#v", c.posts)
	}
	if got := anyPath(c.posts[0].body, "data.attributes.timeframe.start"); got != since.Format(time.RFC3339) {
		t.Fatalf("report timeframe start = %#v", got)
	}
	if got := anyPath(c.posts[0].body, "data.attributes.conversion_metric_id"); got != "received-email" {
		t.Fatalf("report conversion metric = %#v", got)
	}
	if len(c.requests) != 4 || !strings.Contains(c.requests[1].params["filter"], `any(id,["campaign-1"])`) {
		t.Fatalf("campaign requests = %#v", c.requests)
	}
}

func TestSubjectRowsPreservesLowNonzeroClickRates(t *testing.T) {
	rows := subjectRows([]subjectEmail{{
		Subject:    "Low-volume click",
		Source:     "Campaign",
		SourceType: "campaign",
		OpenRate:   0.0248,
		ClickRate:  0.00009,
	}}, 1)
	if len(rows) != 1 || rows[0]["open_rate"] != 0.0248 || rows[0]["click_rate"] != 0.0001 {
		t.Fatalf("subject rows = %#v", rows)
	}
}

func TestClampKlaviyoAnalyticsWindowKeepsRequestsInsideOneYear(t *testing.T) {
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	until := time.Date(2026, 7, 19, 18, 0, 0, 0, loc)
	requested365 := until.AddDate(0, 0, -365)
	clamped, changed := clampKlaviyoAnalyticsWindow(requested365, until)
	if !changed {
		t.Fatal("365-day calendar window was not clamped")
	}
	if got := until.Sub(clamped); got != klaviyoAnalyticsMaxWindow {
		t.Fatalf("clamped duration = %s, want %s", got, klaviyoAnalyticsMaxWindow)
	}

	requested360 := until.AddDate(0, 0, -360)
	unchanged, changed := clampKlaviyoAnalyticsWindow(requested360, until)
	if changed || !unchanged.Equal(requested360) {
		t.Fatalf("360-day window changed: start=%s clamped=%v", unchanged, changed)
	}
}

func TestCollectReferencedTemplateIDsUsesIncludedResources(t *testing.T) {
	c := &fakeCouponPoolClient{responses: []json.RawMessage{
		rawJSON(`{"data":[{"id":"flow-1"}],"included":[{"id":"action-1","attributes":{"definition":{"data":{"message":{"template_id":"flow-template"}}}}}],"links":{}}`),
		campaignMessageTestPage("campaign-1", "Launch", "sent", ""),
		modelAudienceFixture(nil, nil),
		campaignVariationTestPage("message-1", "campaign-template", "Subject", "email"),
	}}
	refs, err := collectReferencedTemplateIDs(c)
	if err != nil {
		t.Fatalf("collectReferencedTemplateIDs() error = %v", err)
	}
	if !refs["flow-template"] || !refs["campaign-template"] || len(refs) != 2 {
		t.Fatalf("refs = %#v", refs)
	}
	if len(c.requests) != 4 || c.requests[0].params["include"] != "flow-actions" || c.requests[1].path != "/api/campaign-messages" {
		t.Fatalf("requests = %#v", c.requests)
	}

}

func TestCollectSubjectMessagesPreservesFlowSubjectFallbacks(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{
			name:  "message subject",
			value: map[string]any{"data": map[string]any{"message": map[string]any{"subject": "Direct subject"}}},
			want:  "Direct subject",
		},
		{
			name:  "message definition content subject",
			value: map[string]any{"data": map[string]any{"message": map[string]any{"definition": map[string]any{"content": map[string]any{"subject": "Nested subject"}}}}},
			want:  "Nested subject",
		},
		{
			name:  "message name",
			value: map[string]any{"data": map[string]any{"message": map[string]any{"name": "Named message"}}},
			want:  "Named message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages := collectSubjectMessages(tt.value)
			if len(messages) != 1 || messages[0].Subject != tt.want {
				t.Fatalf("collectSubjectMessages() = %#v, want %q", messages, tt.want)
			}
		})
	}
}

func TestCollectSubjectEmailsUsesIncludedFlowMessageIDs(t *testing.T) {
	c := &fakeCouponPoolClient{responses: []json.RawMessage{
		rawJSON(`{"data":[{"id":"flow-1","attributes":{"name":"Welcome","status":"live"},"relationships":{"flow-actions":{"data":[{"id":"action-1"}]}}}],"included":[{"id":"action-1","attributes":{"definition":{"data":{"message":{"id":"message-1","subject_line":"Welcome home"}}}}}],"links":{}}`),
	}}
	emails, err := collectSubjectEmails(c, "flow", time.Time{}, time.Time{}, nil)
	if err != nil {
		t.Fatalf("collectSubjectEmails() error = %v", err)
	}
	if len(emails) != 1 || emails[0].ID != "message-1" || emails[0].Subject != "Welcome home" || emails[0].Source != "Welcome" {
		t.Fatalf("emails = %#v", emails)
	}
}

func TestCollectSubjectEmailsUsesCampaignForwardRelationship(t *testing.T) {
	since := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	c := &fakeCouponPoolClient{responses: []json.RawMessage{
		campaignMessageTestPage("campaign-1", "Launch", "sent", "2026-06-15T12:00:00Z"),
		modelAudienceFixture(nil, nil),
		campaignVariationTestPage("message-1", "campaign-template", "New collection", "email"),
	}}
	emails, err := collectSubjectEmails(c, "campaign", since, until, []string{"campaign-1"})
	if err != nil {
		t.Fatalf("collectSubjectEmails() error = %v", err)
	}
	if len(emails) != 1 || emails[0].ID != "message-1" || emails[0].Subject != "New collection" || emails[0].Source != "Launch" {
		t.Fatalf("emails = %#v", emails)
	}
}

func TestSegmentVelocityUsesProfileCountAdditionalFieldAndPacing(t *testing.T) {
	originalWait := waitForProfileCountRequest
	var waits []time.Duration
	waitForProfileCountRequest = func(_ context.Context, wait time.Duration) error {
		waits = append(waits, wait)
		return nil
	}
	t.Cleanup(func() { waitForProfileCountRequest = originalWait })

	db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "segment-velocity.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	first := &fakeCouponPoolClient{responses: []json.RawMessage{
		rawJSON(`{"data":{"id":"segment-1","attributes":{"name":"Engaged","profile_count":100}}}`),
		rawJSON(`{"data":{"id":"segment-2","attributes":{"name":"Customers","profile_count":200}}}`),
	}}
	if _, err := segmentVelocity(context.Background(), first, db, []string{"segment-1", "segment-2"}, "90d", "daily"); err != nil {
		t.Fatalf("first segmentVelocity() error = %v", err)
	}
	if len(first.requests) != 2 {
		t.Fatalf("requests = %#v", first.requests)
	}
	for _, request := range first.requests {
		if !strings.HasSuffix(request.path, "/") || request.params["additional-fields[segment]"] != "profile_count" || request.params["fields[segment]"] != "" {
			t.Fatalf("profile-count request = %#v", request)
		}
	}
	if len(waits) != 1 || waits[0] != profileCountRequestSpacing {
		t.Fatalf("pacing waits = %#v", waits)
	}

	yesterday := segmentSnapshotDate(time.Now().AddDate(0, 0, -1), "daily")
	if _, err := db.DB().Exec(`UPDATE segment_snapshots SET snapshot_date = ?`, yesterday); err != nil {
		t.Fatal(err)
	}
	second := &fakeCouponPoolClient{responses: []json.RawMessage{
		rawJSON(`{"data":{"id":"segment-1","attributes":{"name":"Engaged","profile_count":120}}}`),
		rawJSON(`{"data":{"id":"segment-2","attributes":{"name":"Customers","profile_count":180}}}`),
	}}
	result, err := segmentVelocity(context.Background(), second, db, []string{"segment-1", "segment-2"}, "90d", "daily")
	if err != nil {
		t.Fatalf("second segmentVelocity() error = %v", err)
	}
	rows := result["segments"].([]map[string]any)
	if len(rows) != 2 || rows[0]["current_size"] != 120 || rows[0]["change"] != 20 || rows[0]["direction"] != "growing" {
		t.Fatalf("velocity rows = %#v", rows)
	}
	if rows[1]["current_size"] != 180 || rows[1]["change"] != -20 || rows[1]["direction"] != "shrinking" {
		t.Fatalf("velocity rows = %#v", rows)
	}
	if len(waits) != 2 || waits[1] != profileCountRequestSpacing {
		t.Fatalf("pacing waits after second run = %#v", waits)
	}
}

func TestFetchAllJSONAPIWithIncludedRejectsRepeatedCursor(t *testing.T) {
	c := &fakeCouponPoolClient{responses: []json.RawMessage{
		rawJSON(`{"data":[{"id":"flow-1"}],"links":{"next":"https://example.test/api/flows?page%5Bcursor%5D=repeat"}}`),
		rawJSON(`{"data":[{"id":"flow-2"}],"links":{"next":"https://example.test/api/flows?page%5Bcursor%5D=repeat"}}`),
	}}
	if _, _, err := fetchAllJSONAPIWithIncluded(c, "/api/flows", map[string]string{"page[size]": "50"}); err == nil || !strings.Contains(err.Error(), "cursor") {
		t.Fatalf("expected repeated-cursor error, got %v", err)
	}
	if len(c.requests) != 2 {
		t.Fatalf("requests = %#v", c.requests)
	}
}

func TestDeliverabilityReportIncludesDenominatorCoverage(t *testing.T) {
	c := &fakeCouponPoolClient{
		responses: []json.RawMessage{
			metricListResponse("received", "Received Email"),
			metricListResponse("bounced", "Bounced Email"),
		},
		postResponses: []json.RawMessage{
			metricAggregateResponse("gmail.com", 7000),
			metricAggregateResponse("gmail.com", 70),
			metricAggregateResponse("", 1800000),
		},
	}
	report, err := buildDeliverabilityReport(c, time.Now().AddDate(0, 0, -30), time.Now())
	if err != nil {
		t.Fatalf("buildDeliverabilityReport() error = %v", err)
	}
	coverage := report["coverage"].(map[string]any)
	if coverage["account_received"] != 1800000 || coverage["domain_attributed_received"] != 7000 {
		t.Fatalf("coverage = %#v", coverage)
	}
	if coverage["complete"] != false || coverage["warning"] == nil {
		t.Fatalf("sparse coverage should be explicit: %#v", coverage)
	}
}

func TestNovelLocalAnalytics(t *testing.T) {
	rows := []resourceRow{
		{
			ID: "evt-1",
			Data: map[string]any{"data": map[string]any{"attributes": map[string]any{
				"datetime":    "2026-01-15T00:00:00Z",
				"metric_name": "Placed Order",
				"value":       125.0,
				"properties": map[string]any{
					"$attributed_flow":     "welcome",
					"$attributed_campaign": "spring",
					"utm_campaign":         "spring",
				},
			}, "relationships": map[string]any{"profile": map[string]any{"data": map[string]any{"id": "p1"}}}}},
		},
		{
			ID: "evt-2",
			Data: map[string]any{"data": map[string]any{"attributes": map[string]any{
				"datetime":    "2026-02-15T00:00:00Z",
				"metric_name": "Placed Order",
				"value":       75.0,
				"properties": map[string]any{
					"$attributed_flow":     "welcome",
					"$attributed_campaign": "spring",
					"utm_campaign":         "spring",
				},
			}, "relationships": map[string]any{"profile": map[string]any{"data": map[string]any{"id": "p1"}}}}},
		},
	}

	attr := attribution(rows, "Placed Order", "flow", "2026-01-01")
	if len(attr) != 1 || attr[0]["group"] != "welcome" || attr[0]["orders"] != 2 {
		t.Fatalf("attribution = %#v", attr)
	}
	cohorts := cohort(rows, "Placed Order", "month")
	if len(cohorts) != 1 || cohorts[0]["retained"] != 1 {
		t.Fatalf("cohort = %#v", cohorts)
	}
	rec := reconcile(rows, "spring", "2026-01-01")
	if rec["orders"] != 2 {
		t.Fatalf("reconcile = %#v", rec)
	}
}

func TestNovelDedupAndDecay(t *testing.T) {
	profiles := []resourceRow{
		{ID: "p1", Data: map[string]any{"data": map[string]any{"attributes": map[string]any{"email": "a@example.com", "phone": "+1555"}}}},
		{ID: "p2", Data: map[string]any{"data": map[string]any{"attributes": map[string]any{"email": "a@example.com", "phone": "+1666"}}}},
	}
	dupes := dedup(profiles, "email,phone")
	if len(dupes) != 1 || dupes[0]["field"] != "email" {
		t.Fatalf("dedup = %#v", dupes)
	}

	flows := []resourceRow{
		{ID: "f1", Data: map[string]any{"data": map[string]any{"attributes": map[string]any{"name": "Welcome", "open_rate": 0.20, "previous_open_rate": 0.40}}}},
	}
	decay := flowDecay(flows, 90, 0.15)
	if len(decay) != 1 || decay[0]["flagged"] != true {
		t.Fatalf("flowDecay = %#v", decay)
	}
}

func TestSendFatigueCountsAllOffendersBeforeTruncatingTopList(t *testing.T) {
	base := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	var rows []resourceRow
	for profile := 0; profile < 30; profile++ {
		profileID := "p" + strconv.Itoa(profile)
		for n := 0; n < 3; n++ {
			rows = append(rows, resourceRow{
				ID: profileID + "-" + strconv.Itoa(n),
				Data: map[string]any{"data": map[string]any{"attributes": map[string]any{
					"datetime":    base.Add(time.Duration(n) * time.Hour).Format(time.RFC3339),
					"metric_name": "Received Email",
					"email":       profileID + "@example.com",
				}, "relationships": map[string]any{"profile": map[string]any{"data": map[string]any{"id": profileID}}}}},
			})
		}
	}
	result := sendFatigue(rows, 3, 24*time.Hour, "24h", time.Time{})
	if result["fatigued_profiles"] != 30 {
		t.Fatalf("fatigued_profiles = %v, want 30", result["fatigued_profiles"])
	}
	if result["fatigued_percentage"] != 100.0 {
		t.Fatalf("fatigued_percentage = %v, want 100.0", result["fatigued_percentage"])
	}
	if offenders := result["top_offenders"].([]map[string]any); len(offenders) != 25 {
		t.Fatalf("len(top_offenders) = %d, want truncated top 25", len(offenders))
	}
}

func TestSegmentVelocityHonorsLastWindow(t *testing.T) {
	ctx := context.Background()
	db, err := store.OpenWithContext(ctx, filepath.Join(t.TempDir(), "klaviyo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	if _, err := db.DB().ExecContext(ctx, `CREATE TABLE IF NOT EXISTS segment_snapshots (segment_id TEXT NOT NULL, snapshot_date TEXT NOT NULL, name TEXT, count INTEGER NOT NULL, recorded_at DATETIME DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY(segment_id, snapshot_date))`); err != nil {
		t.Fatalf("create snapshots table: %v", err)
	}
	oldDate := segmentSnapshotDate(time.Now().AddDate(0, 0, -60), "daily")
	recentDate := segmentSnapshotDate(time.Now().AddDate(0, 0, -10), "daily")
	if _, err := db.DB().ExecContext(ctx, `INSERT INTO segment_snapshots(segment_id, snapshot_date, name, count) VALUES (?, ?, ?, ?), (?, ?, ?, ?)`, "seg-1", oldDate, "VIP", 100, "seg-1", recentDate, "VIP", 150); err != nil {
		t.Fatalf("seed snapshots: %v", err)
	}
	client := &fakeCouponPoolClient{responses: []json.RawMessage{rawJSON(`{"data":{"id":"seg-1","attributes":{"name":"VIP","profile_count":160}}}`)}}
	result, err := segmentVelocity(ctx, client, db, []string{"seg-1"}, "30d", "daily")
	if err != nil {
		t.Fatalf("segmentVelocity() error = %v", err)
	}
	segments := result["segments"].([]map[string]any)
	if len(segments) != 1 {
		t.Fatalf("segments = %#v, want one segment", segments)
	}
	trend := segments[0]["weekly_trend"].([]int)
	if len(trend) != 2 || trend[0] != 150 || trend[1] != 160 {
		t.Fatalf("trend = %#v, want recent/current only", trend)
	}
	if segments[0]["size_30d_ago"] != 150 || segments[0]["change"] != 10 {
		t.Fatalf("windowed row = %#v", segments[0])
	}
}

func TestSegmentBaselineKeyPreservesDurationUnit(t *testing.T) {
	if got := segmentBaselineKey("30d"); got != "size_30d_ago" {
		t.Fatalf("segmentBaselineKey(30d) = %q", got)
	}
	if got := segmentBaselineKey("168h"); got != "size_168h_ago" {
		t.Fatalf("segmentBaselineKey(168h) = %q", got)
	}
}

func TestCampaignEventMatchesSkipsUnattributedOrders(t *testing.T) {
	tests := []struct {
		name  string
		event novelEmailEvent
		want  bool
	}{
		{name: "matching id", event: novelEmailEvent{CampaignID: "camp-1"}, want: true},
		{name: "different id", event: novelEmailEvent{CampaignID: "camp-2"}, want: false},
		{name: "matching fallback name", event: novelEmailEvent{CampaignName: "Spring Sale"}, want: true},
		{name: "different fallback name", event: novelEmailEvent{CampaignName: "Summer Sale"}, want: false},
		{name: "unattributed", event: novelEmailEvent{}, want: false},
		{name: "empty target id", event: novelEmailEvent{CampaignID: "camp-1"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			campaignID := "camp-1"
			if tt.name == "empty target id" {
				campaignID = ""
			}
			if got := campaignEventMatches(tt.event, campaignID, "Spring Sale"); got != tt.want {
				t.Fatalf("campaignEventMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubjectCuriosityGapIsSpecific(t *testing.T) {
	if hasCuriosityGap("Use this code before midnight") {
		t.Fatal("generic 'this' subject should not count as curiosity gap")
	}
	if !hasCuriosityGap("Here's what your cart is missing") {
		t.Fatal("specific curiosity phrase should count")
	}
}

func TestFetchMetricEventsDoesNotRequestUnusedSideloads(t *testing.T) {
	client := &fakeCouponPoolClient{responses: []json.RawMessage{rawJSON(`{"data":[],"links":{}}`)}}
	_, err := fetchMetricEvents(client, "metric-1", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC), 10)
	if err != nil {
		t.Fatalf("fetchMetricEvents() error = %v", err)
	}
	if len(client.requests) != 1 {
		t.Fatalf("requests = %#v, want one request", client.requests)
	}
	if _, has := client.requests[0].params["include"]; has {
		t.Fatalf("fetchMetricEvents requested unused include param: %#v", client.requests[0].params)
	}
}

func TestListQualityScoreUsesSupportedListCollectionFields(t *testing.T) {
	originalWait := waitForProfileCountRequest
	waitForProfileCountRequest = func(_ context.Context, _ time.Duration) error { return nil }
	t.Cleanup(func() { waitForProfileCountRequest = originalWait })

	client := &fakeCouponPoolClient{responses: []json.RawMessage{
		rawJSON(`{"data":[{"id":"list-1","attributes":{"name":"Main"}}],"links":{}}`),
		rawJSON(`{"data":{"id":"list-1","attributes":{"name":"Main","profile_count":100}}}`),
		rawJSON(`{"data":[],"links":{}}`),
		rawJSON(`{"data":[],"links":{}}`),
		rawJSON(`{"data":[],"links":{}}`),
		rawJSON(`{"data":[],"links":{}}`),
		rawJSON(`{"data":[],"links":{}}`),
		rawJSON(`{"data":[],"links":{}}`),
		rawJSON(`{"data":[],"links":{}}`),
		rawJSON(`{"data":[],"links":{}}`),
	}}
	result, err := listQualityScore(context.Background(), client, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("listQualityScore() error = %v", err)
	}
	lists := result["lists"].([]map[string]any)
	if len(lists) != 1 || lists[0]["size"] != 100 {
		t.Fatalf("list quality rows = %#v", lists)
	}
	fields := client.requests[0].params["fields[list]"]
	if fields != "name,created,updated" {
		t.Fatalf("fields[list] = %q, want only fields supported by GET /api/lists", fields)
	}
	if got := client.requests[1]; got.path != "/api/lists/list-1/" || got.params["additional-fields[list]"] != "profile_count" {
		t.Fatalf("list profile-count request = %#v", got)
	}
}

func TestEnrichSegmentProfileCountsUsesPacedPerItemRequests(t *testing.T) {
	originalWait := waitForProfileCountRequest
	var waits []time.Duration
	waitForProfileCountRequest = func(_ context.Context, wait time.Duration) error {
		waits = append(waits, wait)
		return nil
	}
	t.Cleanup(func() { waitForProfileCountRequest = originalWait })

	client := &fakeCouponPoolClient{responses: []json.RawMessage{
		rawJSON(`{"data":{"id":"segment-1","attributes":{"profile_count":12}}}`),
		rawJSON(`{"data":{"id":"segment-2","attributes":{"profile_count":34}}}`),
	}}
	data, err := enrichSegmentProfileCounts(context.Background(), client, rawJSON(`[{"id":"segment-1","attributes":{"name":"One"}},{"id":"segment-2","attributes":{"name":"Two"}}]`))
	if err != nil {
		t.Fatalf("enrichSegmentProfileCounts() error = %v", err)
	}
	var segments []map[string]any
	if err := json.Unmarshal(data, &segments); err != nil {
		t.Fatal(err)
	}
	if got := anyInt(segments[0]["attributes"].(map[string]any)["profile_count"]); got != 12 {
		t.Fatalf("first profile_count = %d", got)
	}
	if got := anyInt(segments[1]["attributes"].(map[string]any)["profile_count"]); got != 34 {
		t.Fatalf("second profile_count = %d", got)
	}
	if len(waits) != 1 || waits[0] != profileCountRequestSpacing {
		t.Fatalf("pacing waits = %#v", waits)
	}
	for _, request := range client.requests {
		if !strings.HasSuffix(request.path, "/") || request.params["additional-fields[segment]"] != "profile_count" {
			t.Fatalf("profile-count request = %#v", request)
		}
	}
}

func TestEnrichSegmentProfileCountsPreservesJSONAPIEnvelope(t *testing.T) {
	client := &fakeCouponPoolClient{responses: []json.RawMessage{
		rawJSON(`{"data":{"id":"segment-1","attributes":{"profile_count":12}}}`),
	}}
	data, err := enrichSegmentProfileCounts(context.Background(), client, rawJSON(`{"data":[{"id":"segment-1","attributes":{"name":"One"}}],"links":{"next":null}}`))
	if err != nil {
		t.Fatalf("enrichSegmentProfileCounts() error = %v", err)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	var segments []map[string]any
	if err := json.Unmarshal(envelope["data"], &segments); err != nil {
		t.Fatal(err)
	}
	if got := anyInt(segments[0]["attributes"].(map[string]any)["profile_count"]); got != 12 {
		t.Fatalf("profile_count = %d", got)
	}
	if string(envelope["links"]) != `{"next":null}` {
		t.Fatalf("links = %s", envelope["links"])
	}
}

func TestCompactSegmentProfileCountsPreservesAgentArrayShape(t *testing.T) {
	t.Parallel()
	cases := map[string]json.RawMessage{
		"jsonapi envelope": rawJSON(`{"data":[{"id":"segment-1","type":"segment","attributes":{"name":"One","profile_count":12}}],"links":{"next":null}}`),
		"bare array":       rawJSON(`[{"id":"segment-1","type":"segment","attributes":{"name":"One","profile_count":12}}]`),
	}
	for name, data := range cases {
		data := data
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var segments []map[string]any
			if err := json.Unmarshal(compactSegmentProfileCounts(data), &segments); err != nil {
				t.Fatalf("compact result is not an array: %v", err)
			}
			if len(segments) != 1 {
				t.Fatalf("segments = %#v", segments)
			}
			if segments[0]["id"] != "segment-1" || segments[0]["type"] != "segment" {
				t.Fatalf("compact identity = %#v", segments[0])
			}
			if got := anyInt(segments[0]["profile_count"]); got != 12 {
				t.Fatalf("profile_count = %d", got)
			}
			attrs, ok := segments[0]["attributes"].(map[string]any)
			if !ok || attrs["name"] != "One" {
				t.Fatalf("compact item missing segment name: %#v", segments[0])
			}
			if _, ok := attrs["profile_count"]; ok {
				t.Fatalf("compact item duplicated profile_count in attributes: %#v", segments[0])
			}
		})
	}
}

func TestListGrowthRateCanFlagShrinkingLists(t *testing.T) {
	growth := listGrowthRate(90, 5, 15)
	if growth >= 0 {
		t.Fatalf("growth = %v, want negative", growth)
	}
	issues := listQualityIssues(0, 0, 0, growth)
	if !containsString(issues, "list is shrinking") {
		t.Fatalf("issues = %#v", issues)
	}
}

func TestContentFatigueMatchesOpensByMessageIdentity(t *testing.T) {
	base := time.Now().AddDate(0, 0, -20)
	var rows []resourceRow
	for i := 0; i < 10; i++ {
		messageID := "msg-" + strconv.Itoa(i)
		subject := "Message " + strconv.Itoa(i)
		receivedAt := base.Add(time.Duration(i) * 24 * time.Hour)
		rows = append(rows, emailEventRow("recv-"+strconv.Itoa(i), "p1", "Received Email", receivedAt, messageID, subject))
		if i < 2 {
			rows = append(rows, emailEventRow("open-"+strconv.Itoa(i), "p1", "Opened Email", receivedAt.Add(2*time.Hour), messageID, subject))
		}
	}
	result := contentFatigue(rows, time.Time{}, 10)
	if result["fatigued_profiles"] != 1 {
		t.Fatalf("fatigued_profiles = %v, want 1: %#v", result["fatigued_profiles"], result)
	}
	if categories := result["categories"].(map[string]int); categories["sudden_drop"] != 1 {
		t.Fatalf("categories = %#v, want one sudden_drop", categories)
	}
}

func TestNovelPlanningHelpers(t *testing.T) {
	strategy := briefToStrategy("Launch a subscription winback offer for high intent customers before Mother's Day.")
	if strategy["summary"] == "" {
		t.Fatalf("strategy missing summary: %#v", strategy)
	}
	gate := qaGate(`<a href="https://example.com">Shop</a> SAVE20 {{ first_name|default:'there' }} <p>{% unsubscribe %}</p>`, "SAVE20", "America/Chicago")
	if gate["verdict"] != "warn" {
		t.Fatalf("qaGate verdict = %#v", gate)
	}
	if got := stripTags("<p>Hello&nbsp;there</p>"); got != "Hello there" {
		t.Fatalf("stripTags = %q", got)
	}
}

func emailEventRow(id, profileID, metric string, when time.Time, messageID, subject string) resourceRow {
	return resourceRow{
		ID: id,
		Data: map[string]any{"data": map[string]any{"attributes": map[string]any{
			"datetime":    when.Format(time.RFC3339),
			"metric_name": metric,
			"properties": map[string]any{
				"message_id": messageID,
				"Subject":    subject,
			},
		}, "relationships": map[string]any{"profile": map[string]any{"data": map[string]any{"id": profileID}}}}},
	}
}

func findCommand(cmd *cobra.Command, path []string) *cobra.Command {
	for _, child := range cmd.Commands() {
		if child.Name() != path[0] {
			continue
		}
		if len(path) == 1 {
			return child
		}
		return findCommand(child, path[1:])
	}
	return nil
}

func TestTransformFlowIDs(t *testing.T) {
	tests := []struct {
		name  string
		in    map[string]any
		check func(t *testing.T, out map[string]any)
	}{
		{
			name: "linear chain",
			in: map[string]any{
				"entry_action_id": "100",
				"actions": []any{
					map[string]any{"id": "100", "type": "time-delay", "links": map[string]any{"next": "200"}},
					map[string]any{"id": "200", "type": "send-email", "links": map[string]any{"next": "300"}},
					map[string]any{"id": "300", "type": "send-email", "links": map[string]any{"next": nil}},
				},
			},
			check: func(t *testing.T, out map[string]any) {
				if out["entry_action_id"] != "tmp-1" {
					t.Fatalf("entry_action_id = %v, want tmp-1", out["entry_action_id"])
				}
				actions := out["actions"].([]any)
				a0 := actions[0].(map[string]any)
				if a0["temporary_id"] != "tmp-1" {
					t.Fatalf("action 0 temporary_id = %v", a0["temporary_id"])
				}
				if _, has := a0["id"]; has {
					t.Fatal("action 0 still has id")
				}
				if a0["links"].(map[string]any)["next"] != "tmp-2" {
					t.Fatalf("action 0 next = %v", a0["links"].(map[string]any)["next"])
				}
				a2 := actions[2].(map[string]any)
				if a2["links"].(map[string]any)["next"] != nil {
					t.Fatalf("action 2 next should be nil, got %v", a2["links"].(map[string]any)["next"])
				}
			},
		},
		{
			name: "conditional split",
			in: map[string]any{
				"entry_action_id": "10",
				"actions": []any{
					map[string]any{"id": "10", "type": "conditional-split", "links": map[string]any{"next_if_true": "20", "next_if_false": "30"}},
					map[string]any{"id": "20", "type": "send-email", "links": map[string]any{"next": nil}},
					map[string]any{"id": "30", "type": "send-sms", "links": map[string]any{"next": nil}},
				},
			},
			check: func(t *testing.T, out map[string]any) {
				a0 := out["actions"].([]any)[0].(map[string]any)
				links := a0["links"].(map[string]any)
				if links["next_if_true"] != "tmp-2" {
					t.Fatalf("next_if_true = %v", links["next_if_true"])
				}
				if links["next_if_false"] != "tmp-3" {
					t.Fatalf("next_if_false = %v", links["next_if_false"])
				}
			},
		},
		{
			name: "empty actions",
			in: map[string]any{
				"entry_action_id": "1",
				"actions":         []any{},
			},
			check: func(t *testing.T, out map[string]any) {
				if out["entry_action_id"] != "1" {
					t.Fatalf("entry_action_id should be unchanged for empty actions, got %v", out["entry_action_id"])
				}
			},
		},
		{
			name: "no actions key",
			in: map[string]any{
				"triggers": []any{},
			},
			check: func(t *testing.T, out map[string]any) {
				if out["triggers"] == nil {
					t.Fatal("triggers should be preserved")
				}
			},
		},
		{
			name: "does not mutate input",
			in: map[string]any{
				"entry_action_id": "A",
				"actions": []any{
					map[string]any{"id": "A", "type": "time-delay", "links": map[string]any{"next": nil}},
				},
			},
			check: func(t *testing.T, out map[string]any) {
				// out should have tmp-1, but we check the function returns a new map
				if out["entry_action_id"] != "tmp-1" {
					t.Fatalf("expected tmp-1, got %v", out["entry_action_id"])
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := transformFlowIDs(tt.in)
			tt.check(t, out)
		})
	}
}

func TestCleanFlowDefinition(t *testing.T) {
	def := map[string]any{
		"trigger_type": "Metric",
		"created":      "2024-01-01",
		"updated":      "2024-01-02",
		"triggers":     []any{map[string]any{"type": "metric"}},
		"actions": []any{
			map[string]any{
				"id": "1", "type": "send-email",
				"data": map[string]any{
					"message": map[string]any{"id": "MSG1", "subject_line": "Test"},
					"status":  "live",
				},
			},
		},
		"entry_action_id": "1",
	}
	out := cleanFlowDefinition(def)
	if _, has := out["trigger_type"]; has {
		t.Fatal("trigger_type should be removed")
	}
	if _, has := out["created"]; has {
		t.Fatal("created should be removed")
	}
	if _, has := out["updated"]; has {
		t.Fatal("updated should be removed")
	}
	a := out["actions"].([]any)[0].(map[string]any)
	msg := a["data"].(map[string]any)["message"].(map[string]any)
	if _, has := msg["id"]; has {
		t.Fatal("message id should be removed")
	}
	if msg["subject_line"] != "Test" {
		t.Fatal("subject_line should be preserved")
	}
}

func TestCouponCodeIsUsable(t *testing.T) {
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		code map[string]any
		want bool
	}{
		{
			name: "unassigned future expiration",
			code: map[string]any{"attributes": map[string]any{"status": "UNASSIGNED", "expires_at": "2026-06-01T00:00:00Z"}},
			want: true,
		},
		{
			name: "unassigned no expiration",
			code: map[string]any{"attributes": map[string]any{"status": "UNASSIGNED"}},
			want: true,
		},
		{
			name: "assigned",
			code: map[string]any{"attributes": map[string]any{"status": "ASSIGNED_TO_PROFILE", "expires_at": "2026-06-01T00:00:00Z"}},
			want: false,
		},
		{
			name: "expired",
			code: map[string]any{"attributes": map[string]any{"status": "UNASSIGNED", "expires_at": "2026-05-01T00:00:00Z"}},
			want: false,
		},
		{
			name: "bad expiration",
			code: map[string]any{"attributes": map[string]any{"status": "UNASSIGNED", "expires_at": "not-a-date"}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := couponCodeIsUsable(tt.code, now); got != tt.want {
				t.Fatalf("couponCodeIsUsable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckCouponPoolsPaginatesAndAlerts(t *testing.T) {
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	client := &fakeCouponPoolClient{responses: []json.RawMessage{
		rawJSON(`{
			"data": [
				{"id": "WELCOME10", "attributes": {"external_id": "WELCOME10", "description": "Welcome offer"}},
				{"id": "VIP20", "attributes": {"external_id": "VIP20", "description": "VIP offer"}}
			],
			"links": {}
		}`),
		rawJSON(`{
			"data": [
				{"id": "WELCOME10-A", "attributes": {"status": "UNASSIGNED", "expires_at": "2026-06-01T00:00:00Z"}},
				{"id": "WELCOME10-B", "attributes": {"status": "UNASSIGNED", "expires_at": "2026-05-01T00:00:00Z"}}
			],
			"links": {"next": "https://example.test/api/coupons/WELCOME10/coupon-codes?page%5Bcursor%5D=next-page"}
		}`),
		rawJSON(`{
			"data": [
				{"id": "WELCOME10-C", "attributes": {"status": "UNASSIGNED"}}
			],
			"links": {}
		}`),
		rawJSON(`{
			"data": [],
			"links": {}
		}`),
	}}

	report, err := checkCouponPools(client, 2, "", now)
	if err != nil {
		t.Fatalf("checkCouponPools() error = %v", err)
	}
	if report.TotalCoupons != 2 || report.AlertCount != 1 || report.OK {
		t.Fatalf("unexpected report summary: %#v", report)
	}
	if len(report.Pools) != 2 {
		t.Fatalf("len(report.Pools) = %d, want 2", len(report.Pools))
	}
	if report.Pools[0].CouponID != "VIP20" || !report.Pools[0].Alert || report.Pools[0].RemainingCodes != 0 {
		t.Fatalf("first pool should be low VIP20: %#v", report.Pools[0])
	}
	if report.Pools[1].CouponID != "WELCOME10" || report.Pools[1].Alert || report.Pools[1].RemainingCodes != 2 || report.Pools[1].ExpiredCodes != 1 || report.Pools[1].PagesScanned != 2 {
		t.Fatalf("second pool should be healthy WELCOME10: %#v", report.Pools[1])
	}
	if len(client.requests) != 4 {
		t.Fatalf("requests = %#v, want 4", client.requests)
	}
	if client.requests[1].params["filter"] != `equals(status,"UNASSIGNED")` {
		t.Fatalf("coupon code filter = %q", client.requests[1].params["filter"])
	}
	if client.requests[2].params["page[cursor]"] != "next-page" {
		t.Fatalf("cursor = %q, want next-page", client.requests[2].params["page[cursor]"])
	}
}

func TestSegmentBuildInterestFilters(t *testing.T) {
	if got := productSlug("Focus Timer"); got != "focus-timer" {
		t.Fatalf("productSlug() = %q", got)
	}
	contains := productMetricFilter("Timer", false)
	if contains[0].(map[string]any)["operator"] != "contains" {
		t.Fatalf("contains filter = %#v", contains)
	}
	exact := productMetricFilter("Timer", true)
	if exact[0].(map[string]any)["operator"] != "equals" {
		t.Fatalf("exact filter = %#v", exact)
	}
	clicked := clickedProductFilter("Focus Timer")
	if clicked[0].(map[string]any)["value"] != "/products/focus-timer" {
		t.Fatalf("clicked filter = %#v", clicked)
	}
}

func TestProductAffinityRowsUsesAnchorProduct(t *testing.T) {
	client := &fakeCouponPoolClient{responses: []json.RawMessage{rawJSON(`{"data":[
		{"id":"order-1","attributes":{"properties":{"ItemNames":["Focus Timer","Weekly Planner"]}}},
		{"id":"order-2","attributes":{"properties":{"Items":[{"ProductName":"Focus Timer"},{"ProductName":"Weekly Planner"},{"ProductName":"Pen Set"}]}}},
		{"id":"order-3","attributes":{"properties":{"ItemNames":["Notebook","Pen Set"]}}}
	]}`)}}
	rows, anchorOrders, err := productAffinityRows(client, "metric-1", "focus timer", time.Now().AddDate(-1, 0, 0), time.Now(), 10)
	if err != nil {
		t.Fatalf("productAffinityRows returned error: %v", err)
	}
	if anchorOrders != 2 {
		t.Fatalf("anchorOrders = %d, want 2", anchorOrders)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %#v", rows)
	}
	if rows[0]["name"] != "Weekly Planner" || anyInt(rows[0]["orders"]) != 2 {
		t.Fatalf("top affinity row = %#v", rows[0])
	}
	for _, row := range rows {
		if strings.Contains(strings.ToLower(fmt.Sprint(row["name"])), "focus timer") {
			t.Fatalf("anchor product should not be returned as an affinity row: %#v", row)
		}
	}
}

func TestGiftFollowupFlowRequiresAndUsesTriggerProduct(t *testing.T) {
	if _, _, err := buildGiftFollowupFlow(&fakeCouponPoolClient{}, "sender@example.com", "Brand", ""); err == nil || !strings.Contains(err.Error(), "--trigger-product") {
		t.Fatalf("expected missing trigger product error, got %v", err)
	}
	client := &fakeCouponPoolClient{responses: []json.RawMessage{rawJSON(`{"data":[{"id":"metric-1","attributes":{"name":"Placed Order"}}]}`)}}
	def, _, err := buildGiftFollowupFlow(client, "sender@example.com", "Brand", "Gift Box")
	if err != nil {
		t.Fatalf("buildGiftFollowupFlow returned error: %v", err)
	}
	triggers := def["triggers"].([]any)
	trigger := triggers[0].(map[string]any)
	filter := trigger["trigger_filter"].(map[string]any)
	conditions := filter["conditions"].([]any)
	condition := conditions[0].(map[string]any)
	if condition["value"] != "Gift Box" {
		t.Fatalf("trigger product = %#v", condition["value"])
	}
}

func TestScoreRFMProfilesUsesRecency(t *testing.T) {
	raw := rawJSON(`{"data":{"attributes":{"data":[
		{"dimensions":["recent"],"measurements":{"count":[5],"sum_value":[500]}},
		{"dimensions":["sleeping"],"measurements":{"count":[2],"sum_value":[50]}},
		{"dimensions":["lost"],"measurements":{"count":[1],"sum_value":[10]}}
	]}}}`)
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	scores := scoreRFMProfiles(raw, map[string]time.Time{
		"recent":   now.AddDate(0, 0, -10),
		"sleeping": now.AddDate(0, 0, -120),
		"lost":     now.AddDate(-2, 0, 0),
	}, now)
	byProfile := map[string]map[string]any{}
	for _, score := range scores {
		byProfile[fmt.Sprint(score["profile_id"])] = score
	}
	if anyInt(byProfile["recent"]["r_score"]) != 5 {
		t.Fatalf("recent score = %#v", byProfile["recent"])
	}
	if anyInt(byProfile["sleeping"]["r_score"]) != 3 {
		t.Fatalf("sleeping score = %#v", byProfile["sleeping"])
	}
	if anyInt(byProfile["lost"]["r_score"]) != 1 {
		t.Fatalf("lost score = %#v", byProfile["lost"])
	}
	if anyInt(byProfile["recent"]["m_score"]) != 5 {
		t.Fatalf("monetary score = %#v", byProfile["recent"])
	}
	if scoreBucket(1) != 2 {
		t.Fatalf("scoreBucket(1) = %d, want 2", scoreBucket(1))
	}
}

func TestRFMProfileBulkImportJobBodyUsesBulkShape(t *testing.T) {
	body := rfmProfileBulkImportJobBody([]map[string]any{{
		"profile_id": "profile-1",
		"r_score":    5,
		"f_score":    4,
		"m_score":    3,
	}})
	data := body["data"].(map[string]any)
	if data["type"] != "profile-bulk-import-job" {
		t.Fatalf("bulk import job type = %#v", data["type"])
	}
	attrs := data["attributes"].(map[string]any)
	profiles := attrs["profiles"].(map[string]any)
	items := profiles["data"].([]any)
	if len(items) != 1 {
		t.Fatalf("profiles data length = %d", len(items))
	}
	profile := items[0].(map[string]any)
	if profile["type"] != "profile" || profile["id"] != "profile-1" {
		t.Fatalf("profile resource = %#v", profile)
	}
	props := profile["attributes"].(map[string]any)["properties"].(map[string]any)
	if props["rfm_recency_score"] != 5 || props["rfm_frequency_score"] != 4 || props["rfm_monetary_score"] != 3 {
		t.Fatalf("rfm properties = %#v", props)
	}
}

func TestCampaignAudienceIDsUsesOnlyIncludedAudiences(t *testing.T) {
	ids := campaignAudienceIDs(map[string]any{"attributes": map[string]any{"audiences": map[string]any{
		"included": []any{"list-1", map[string]any{"id": "segment-1"}},
		"excluded": []any{"suppressed-list", map[string]any{"id": "segment-2"}},
	}}})
	if len(ids) != 2 || !containsString(ids, "list-1") || !containsString(ids, "segment-1") {
		t.Fatalf("included audience ids = %#v", ids)
	}
	if containsString(ids, "suppressed-list") || containsString(ids, "segment-2") {
		t.Fatalf("excluded audiences should not be returned: %#v", ids)
	}
}

func TestChurnCandidateRowsRequiresLapsedCadence(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	rows := []map[string]any{
		{"profile_id": "recent", "order_count": 4},
		{"profile_id": "lapsed", "order_count": 4},
		{"profile_id": "single", "order_count": 1},
	}
	flagged := churnCandidateRows(rows, map[string]time.Time{
		"recent": now.AddDate(0, 0, -200),
		"lapsed": now.AddDate(0, 0, -800),
		"single": now.AddDate(0, 0, -800),
	}, now)
	if len(flagged) != 1 || flagged[0]["profile_id"] != "lapsed" {
		t.Fatalf("flagged churn rows = %#v", flagged)
	}
}

func TestListSubscriberCountUsesListFields(t *testing.T) {
	count := listSubscriberCount(map[string]any{"attributes": map[string]any{"profile_count": 12000}})
	if count != 12000 {
		t.Fatalf("subscriber count = %d", count)
	}
	estimate := listSubscriberCount(map[string]any{"attributes": map[string]any{"profile_count_estimate": 15000}})
	if estimate != 15000 {
		t.Fatalf("subscriber count estimate = %d", estimate)
	}
}

func TestMetricAggregateBodyUsesConfigurableTimezone(t *testing.T) {
	t.Setenv("KLAVIYO_TIMEZONE", "Europe/London")
	body := metricAggregateBody("metric-1", []string{"count"}, nil, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC))
	attrs := body["data"].(map[string]any)["attributes"].(map[string]any)
	if attrs["timezone"] != "Europe/London" {
		t.Fatalf("timezone = %#v", attrs["timezone"])
	}
}

func TestAnnotateEngagementTrendsAddsTrendFields(t *testing.T) {
	rows := []map[string]any{{"name": "Welcome Flow"}}
	previous := []map[string]any{{"name": "Welcome Flow", "open_rate": 55.0, "click_rate": 8.0}}
	current := []map[string]any{{"name": "Welcome Flow", "open_rate": 44.0, "click_rate": 5.0}}
	annotateEngagementTrends(rows, previous, current)
	if rows[0]["trend_flag"] != "declining" {
		t.Fatalf("trend row = %#v", rows[0])
	}
	if anyFloat(rows[0]["open_rate_delta"]) != -11 {
		t.Fatalf("open delta = %#v", rows[0])
	}
}

func TestProfileSpendRowsLooksUpEmailsAfterLimit(t *testing.T) {
	client := &fakeCouponPoolClient{
		responses: []json.RawMessage{
			rawJSON(`{"data":[{"id":"metric-1","attributes":{"name":"Placed Order"}}]}`),
			rawJSON(`{"data":{"attributes":{"email":"top@example.com"}}}`),
		},
		postResponses: []json.RawMessage{rawJSON(`{"data":{"attributes":{"data":[
			{"dimensions":["low"],"measurements":{"count":[1],"sum_value":[10]}},
			{"dimensions":["top"],"measurements":{"count":[3],"sum_value":[300]}}
		]}}}`)},
	}
	rows, err := profileSpendRows(client, 1)
	if err != nil {
		t.Fatalf("profileSpendRows returned error: %v", err)
	}
	if len(rows) != 1 || rows[0]["profile_id"] != "top" || rows[0]["email"] != "top@example.com" {
		t.Fatalf("rows = %#v", rows)
	}
	var profileGets []string
	for _, request := range client.requests {
		if strings.HasPrefix(request.path, "/api/profiles/") {
			profileGets = append(profileGets, request.path)
		}
	}
	if len(profileGets) != 1 || profileGets[0] != "/api/profiles/top" {
		t.Fatalf("profile GETs = %#v", profileGets)
	}
}

func TestDeleteSegmentUsesEscapedSegmentPath(t *testing.T) {
	client := &fakeCouponPoolClient{}
	if err := deleteSegment(client, "segment/123"); err != nil {
		t.Fatalf("deleteSegment returned error: %v", err)
	}
	if len(client.deletes) != 1 || client.deletes[0] != "/api/segments/segment%2F123" {
		t.Fatalf("delete paths = %#v", client.deletes)
	}
}

type fakeCouponPoolClient struct {
	responses     []json.RawMessage
	postResponses []json.RawMessage
	requests      []fakeCouponPoolRequest
	posts         []fakeCouponPoolPost
	deletes       []string
}

type fakePaginatedClient struct {
	responses []json.RawMessage
	params    []map[string]string
}

func (f *fakePaginatedClient) GetWithHeaders(ctx context.Context, _ string, params map[string]string, _ map[string]string) (json.RawMessage, error) {
	copied := map[string]string{}
	for k, v := range params {
		copied[k] = v
	}
	f.params = append(f.params, copied)
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

type fakeCouponPoolRequest struct {
	path   string
	params map[string]string
}

type fakeCouponPoolPost struct {
	path string
	body any
}

func (f *fakeCouponPoolClient) Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error) {
	copied := map[string]string{}
	for k, v := range params {
		copied[k] = v
	}
	f.requests = append(f.requests, fakeCouponPoolRequest{path: path, params: copied})
	if len(f.responses) == 0 {
		return nil, nil
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

func (f *fakeCouponPoolClient) Post(ctx context.Context, path string, body any) (json.RawMessage, int, error) {
	f.posts = append(f.posts, fakeCouponPoolPost{path: path, body: body})
	if len(f.postResponses) > 0 {
		resp := f.postResponses[0]
		f.postResponses = f.postResponses[1:]
		return resp, 200, nil
	}
	return nil, 0, nil
}

func (f *fakeCouponPoolClient) Patch(ctx context.Context, _ string, _ any) (json.RawMessage, int, error) {
	return nil, 0, nil
}

func (f *fakeCouponPoolClient) Delete(ctx context.Context, path string) (json.RawMessage, int, error) {
	f.deletes = append(f.deletes, path)
	return nil, 204, nil
}

func rawJSON(s string) json.RawMessage {
	return json.RawMessage(s)
}

func metricListResponse(id, name string) json.RawMessage {
	return rawJSON(fmt.Sprintf(`{"data":[{"id":%q,"attributes":{"name":%q}}],"links":{}}`, id, name))
}

func metricAggregateResponse(dimension string, count float64) json.RawMessage {
	dimensions := "[]"
	if dimension != "" {
		dimensions = fmt.Sprintf(`[%q]`, dimension)
	}
	return rawJSON(fmt.Sprintf(`{"data":{"attributes":{"data":[{"dimensions":%s,"measurements":{"count":[%v]}}]}}}`, dimensions, count))
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func TestUnsubscribeCompliance(t *testing.T) {
	tests := []struct {
		name       string
		html       string
		wantStatus string
	}{
		{name: "full link tag", html: `<p>{% unsubscribe %}</p>`, wantStatus: "pass"},
		{name: "custom full link tag", html: `<p>{% unsubscribe 'Stop these emails' %}</p>`, wantStatus: "pass"},
		{name: "URL tag in href", html: `<a href="{% unsubscribe_link %}">Unsubscribe</a>`, wantStatus: "pass"},
		{name: "URL tag in single-quoted href", html: `<a href='{% unsubscribe_link %}'>Unsubscribe</a>`, wantStatus: "pass"},
		{name: "correct two-link footer", html: `<a href="{% unsubscribe_link %}">Unsubscribe</a><a href="{% manage_preferences_link %}">Manage Preferences</a>`, wantStatus: "pass"},
		{name: "compact Klaviyo tags", html: `<a href="{%unsubscribe_link%}">Unsubscribe</a><a href="{%manage_preferences_link%}">Manage Preferences</a>`, wantStatus: "pass"},
		{name: "nested preference property", html: `<p>{{ person.preferences }}</p><a href="{% unsubscribe_link %}">Unsubscribe</a>`, wantStatus: "fail"},
		{name: "nested unsubscribe URL", html: `<a href="{{ subscriber.unsubscribe_url }}{% unsubscribe_link %}">Unsubscribe</a>`, wantStatus: "fail"},
		{name: "Kit-only footer", html: `<a href="{{ unsubscribe_url }}">Unsubscribe</a><a href="{{ manage_preferences_url }}">Manage Preferences</a>`, wantStatus: "fail"},
		{name: "mixed footer", html: `<a href="{% unsubscribe_link %}">Unsubscribe</a><a href="{{ manage_preferences_url }}">Manage Preferences</a>`, wantStatus: "fail"},
		{name: "Kit tag hidden in comment", html: `<!-- {{ unsubscribe_url }} --><a href="{% unsubscribe_link %}">Unsubscribe</a>`, wantStatus: "fail"},
		{name: "preferences tag outside href", html: `<a href="{% unsubscribe_link %}">Unsubscribe</a><p>{% manage_preferences_link %}</p>`, wantStatus: "fail"},
		{name: "preferences tag on non-anchor", html: `<a href="{% unsubscribe_link %}">Unsubscribe</a><div href="{% manage_preferences_link %}">Manage Preferences</div>`, wantStatus: "fail"},
		{name: "wrong tag for preferences anchor", html: `<a href="{% unsubscribe_link %}">Unsubscribe</a><a href="{% unsubscribe_link %}">Manage Preferences</a>`, wantStatus: "fail"},
		{name: "URL tag in data href", html: `<a data-href="{% unsubscribe_link %}">Unsubscribe</a>`, wantStatus: "fail"},
		{name: "URL tag in aria label", html: `<a aria-label="href={% unsubscribe_link %}">Unsubscribe</a>`, wantStatus: "fail"},
		{name: "full link tag in href", html: `<a href="{% unsubscribe %}">Unsubscribe</a>`, wantStatus: "fail"},
		{name: "full link tag in unquoted href", html: `<a href={% unsubscribe %}>Unsubscribe</a>`, wantStatus: "fail"},
		{name: "full link tag in unrelated attribute", html: `<a title="{% unsubscribe %}" href="#">Unsubscribe</a>`, wantStatus: "fail"},
		{name: "hidden full link tag", html: `<div style="display:none">{% unsubscribe %}</div>`, wantStatus: "fail"},
		{name: "hidden attribute full link tag", html: `<div hidden>{% unsubscribe %}</div>`, wantStatus: "fail"},
		{name: "custom full link tag in href", html: `<a href="{% unsubscribe 'Stop' %}">Unsubscribe</a>`, wantStatus: "fail"},
		{name: "URL tag outside href", html: `<p>{% unsubscribe_link %}</p>`, wantStatus: "fail"},
		{name: "full link tag in comment", html: `<!-- {% unsubscribe %} -->`, wantStatus: "fail"},
		{name: "URL tag in non-rendered template", html: `<template><a href="{% unsubscribe_link %}">Unsubscribe</a></template>`, wantStatus: "fail"},
		{name: "hidden and visible tags", html: `<!-- {% unsubscribe %} --><a href="{% unsubscribe_link %}">Unsubscribe</a>`, wantStatus: "pass"},
		{name: "missing tag", html: `<p>Hello</p>`, wantStatus: "fail"},
		{name: "missing HTML evidence", wantStatus: "warn"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, _ := unsubscribeCompliance(tt.html)
			if gotStatus != tt.wantStatus {
				t.Fatalf("status = %q, want %q", gotStatus, tt.wantStatus)
			}
		})
	}
}

func TestQAGateUsesAssignedCampaignTemplate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/campaigns/campaign-1":
			fmt.Fprint(w, `{"data":{"id":"campaign-1","type":"campaign"}}`)
		case "/api/campaigns/campaign-1/campaign-messages":
			w.Write(campaignMessageTestPage("message-1", "Launch", "draft", ""))
		case "/api/campaign-messages/message-1/campaign-audience":
			w.Write(modelAudienceFixture(nil, nil))
		case "/api/campaign-messages/message-1/campaign-variations":
			w.Write(campaignVariationTestPage("variation-1", "template-1", "Subject", "email"))
		case "/api/templates/template-1":
			fmt.Fprint(w, `{"data":{"id":"template-1","type":"template","attributes":{"html":"<p>No unsubscribe link</p>"}}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("KLAVIYO_BASE_URL", server.URL)
	t.Setenv("KLAVIYO_API_KEY", "test-key")
	previousRegistration := registeredPlatformSource
	t.Cleanup(func() { registeredPlatformSource = previousRegistration })
	registeredPlatformSource = nil
	var output bytes.Buffer
	cmd := newPlanQAGateCmd(&rootFlags{asJSON: true})
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--campaign-id", "campaign-1"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected failed QA verdict to return an error")
	}
	var result map[string]any
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["verdict"] != "fail" {
		t.Fatalf("verdict = %v, want fail", result["verdict"])
	}
}

func TestQAGateWithoutAssignedTemplateReturnsChecklist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/campaigns/campaign-1":
			fmt.Fprint(w, `{"data":{"id":"campaign-1","type":"campaign"}}`)
		case "/api/campaigns/campaign-1/campaign-messages":
			w.Write(campaignMessageTestPage("message-1", "Launch", "draft", ""))
		case "/api/campaign-messages/message-1/campaign-audience":
			w.Write(modelAudienceFixture(nil, nil))
		case "/api/campaign-messages/message-1/campaign-variations":
			w.Write(campaignVariationTestPage("variation-1", "template-1", "Subject", "email"))
		case "/api/templates/template-1":
			http.Error(w, `{"errors":[{"detail":"not found"}]}`, http.StatusNotFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("KLAVIYO_BASE_URL", server.URL)
	t.Setenv("KLAVIYO_API_KEY", "test-key")
	previousRegistration := registeredPlatformSource
	t.Cleanup(func() { registeredPlatformSource = previousRegistration })
	registeredPlatformSource = nil
	var output bytes.Buffer
	cmd := newPlanQAGateCmd(&rootFlags{asJSON: true})
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--campaign-id", "campaign-1"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["verdict"] != "warn" {
		t.Fatalf("verdict = %v, want warn", result["verdict"])
	}
	if result["evidence"].(map[string]any)["template_status"] != "not_assigned" {
		t.Fatalf("evidence = %v", result["evidence"])
	}
}

func TestCampaignDeployRejectsBrokenUnsubscribeTag(t *testing.T) {
	cmd := newCampaignsDeployCmd(&rootFlags{})
	cmd.SetArgs([]string{
		"--template-html", `<a href="{% unsubscribe %}">Unsubscribe</a>`,
		"--campaign-name", "Compliance test",
		"--list-id", "list-1",
		"--subject", "Compliance test",
		"--from-email", "sender@example.com",
		"--from-label", "Sender",
	})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "campaign compliance preflight failed") {
		t.Fatalf("error = %v, want compliance preflight failure", err)
	}
}
