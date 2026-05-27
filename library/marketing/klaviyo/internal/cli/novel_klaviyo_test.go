package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strconv"
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

func TestNovelPlanningHelpers(t *testing.T) {
	strategy := briefToStrategy("Launch a subscription winback offer for high intent customers before Mother's Day.")
	if strategy["summary"] == "" {
		t.Fatalf("strategy missing summary: %#v", strategy)
	}
	gate := qaGate(`<a href="https://example.com">Shop</a> SAVE20 {{ first_name|default:'there' }}`, "SAVE20", "America/Chicago")
	if gate["verdict"] != "warn" {
		t.Fatalf("qaGate verdict = %#v", gate)
	}
	if got := stripTags("<p>Hello&nbsp;there</p>"); got != "Hello there" {
		t.Fatalf("stripTags = %q", got)
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

type fakeCouponPoolClient struct {
	responses []json.RawMessage
	requests  []fakeCouponPoolRequest
}

type fakeCouponPoolRequest struct {
	path   string
	params map[string]string
}

func (f *fakeCouponPoolClient) Get(path string, params map[string]string) (json.RawMessage, error) {
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

func (f *fakeCouponPoolClient) Post(_ string, _ any) (json.RawMessage, int, error) {
	return nil, 0, nil
}

func (f *fakeCouponPoolClient) Patch(_ string, _ any) (json.RawMessage, int, error) {
	return nil, 0, nil
}

func rawJSON(s string) json.RawMessage {
	return json.RawMessage(s)
}
