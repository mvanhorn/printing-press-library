package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildKlaviyoCampaignReportBodyResolvesShopifyMetric(t *testing.T) {
	c := &reportTestClient{gets: []json.RawMessage{json.RawMessage(`{
		"data":[
			{"id":"other","attributes":{"name":"Placed Order","integration":{"name":"Custom"}}},
			{"id":"shopify","attributes":{"name":"Placed Order","integration":{"name":"Shopify"}}}
		],"links":{}}`)}}
	body, metricID, err := buildKlaviyoReportBody(c, klaviyoReportRequest{
		Type: "campaign-values-report", Statistics: []string{"recipients", "conversion_value"},
		Timeframe: map[string]any{"key": "last_12_months"},
	}, false)
	if err != nil {
		t.Fatalf("buildKlaviyoReportBody() error = %v", err)
	}
	if metricID != "shopify" {
		t.Fatalf("metric id = %q, want shopify", metricID)
	}
	attrs := anyPath(body, "data.attributes").(map[string]any)
	if attrs["conversion_metric_id"] != "shopify" {
		t.Fatalf("body attributes = %#v", attrs)
	}
	if got := anyPath(attrs, "timeframe.key"); got != "last_12_months" {
		t.Fatalf("timeframe key = %#v", got)
	}
}

func TestBuildKlaviyoSeriesReportBodyIncludesInterval(t *testing.T) {
	body, _, err := buildKlaviyoReportBody(&reportTestClient{}, klaviyoReportRequest{
		Type: "segment-series-report", Statistics: []string{"total_members"},
		Timeframe: map[string]any{"key": "last_30_days"}, Interval: "weekly",
	}, false)
	if err != nil {
		t.Fatalf("buildKlaviyoReportBody() error = %v", err)
	}
	if got := anyPath(body, "data.attributes.interval"); got != "weekly" {
		t.Fatalf("interval = %#v", got)
	}
	if got := anyPath(body, "data.attributes.conversion_metric_id"); got != nil {
		t.Fatalf("segment report unexpectedly included conversion metric: %#v", got)
	}
}

func TestBuildKlaviyoFlowSeriesReportBodyResolvesMetricAndIncludesInterval(t *testing.T) {
	c := &reportTestClient{gets: []json.RawMessage{json.RawMessage(`{
		"data":[{"id":"shopify","attributes":{"name":"Placed Order","integration":{"name":"Shopify"}}}],
		"links":{}
	}`)}}
	body, metricID, err := buildKlaviyoReportBody(c, klaviyoReportRequest{
		Type: "flow-series-report", Statistics: []string{"recipients", "open_rate"},
		Timeframe: map[string]any{"key": "last_12_months"}, Interval: "monthly",
	}, false)
	if err != nil {
		t.Fatalf("buildKlaviyoReportBody() error = %v", err)
	}
	if metricID != "shopify" {
		t.Fatalf("metric id = %q, want shopify", metricID)
	}
	attrs := anyPath(body, "data.attributes").(map[string]any)
	if attrs["conversion_metric_id"] != "shopify" || attrs["interval"] != "monthly" {
		t.Fatalf("body attributes = %#v", attrs)
	}
	if got := anyPath(attrs, "timeframe.key"); got != "last_12_months" {
		t.Fatalf("timeframe key = %#v", got)
	}
}

func TestFlowSeriesReportDryRunPrintsPreviewWithoutCredentials(t *testing.T) {
	flags := &rootFlags{asJSON: true, compact: true, dryRun: true}
	cmd := newFlowSeriesReportsPromotedCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--timeframe", "last_12_months", "--interval", "monthly"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run Execute() error = %v", err)
	}
	var preview map[string]any
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatalf("dry-run output = %q: %v", out.String(), err)
	}
	if preview["dry_run"] != true || anyPath(preview, "request.method") != "POST" || anyPath(preview, "request.path") != "/api/flow-series-reports/" {
		t.Fatalf("dry-run preview = %#v", preview)
	}
	if got := anyPath(preview, "request.body.data.attributes.conversion_metric_id"); got != "<auto:shopify-placed-order>" {
		t.Fatalf("dry-run conversion metric = %#v", got)
	}
	if got := anyPath(preview, "request.body.data.attributes.interval"); got != "monthly" {
		t.Fatalf("dry-run interval = %#v", got)
	}
}

func TestCampaignTargetingReportJoinsIncludedExcludedAndNeverMailedSegments(t *testing.T) {
	c := &reportTestClient{gets: []json.RawMessage{
		json.RawMessage(`{"data":[{"id":"campaign-1","attributes":{"name":"Launch","send_time":"2026-07-01T12:00:00Z","audiences":{"included":["segment-a","list-a"],"excluded":["segment-b"]}}}],"links":{}}`),
		json.RawMessage(`{"data":[],"links":{}}`),
		json.RawMessage(`{"data":[{"id":"segment-a","attributes":{"name":"VIP"}},{"id":"segment-b","attributes":{"name":"Recent Buyers"}},{"id":"segment-c","attributes":{"name":"Never Mailed"}}],"links":{}}`),
		json.RawMessage(`{"data":[{"id":"list-a","attributes":{"name":"Newsletter Subscribers"}},{"id":"list-b","attributes":{"name":"Dormant List"}}],"links":{}}`),
	}}
	reportRows := []map[string]any{{
		"groupings":  map[string]any{"campaign_id": "campaign-1"},
		"statistics": map[string]any{"recipients": 1200.0, "conversion_value": 3456.78},
	}}
	result, err := campaignTargetingReport(c, reportRows)
	if err != nil {
		t.Fatalf("campaignTargetingReport() error = %v", err)
	}
	rows := result["segments"].([]map[string]any)
	if len(rows) != 1 || rows[0]["segment_name"] != "VIP" || rows[0]["campaigns_sent"] != 1 {
		t.Fatalf("targeting rows = %#v", rows)
	}
	if rows[0]["total_sends"] != int64(1200) || rows[0]["attributed_revenue"] != 3456.78 {
		t.Fatalf("targeting performance = %#v", rows[0])
	}
	lists := result["lists"].([]map[string]any)
	if len(lists) != 1 || lists[0]["list_name"] != "Newsletter Subscribers" || lists[0]["audience_type"] != "list" {
		t.Fatalf("targeting lists = %#v", lists)
	}
	never := result["never_mailed_segments"].([]map[string]any)
	if len(never) != 2 {
		t.Fatalf("never mailed segments = %#v", never)
	}
	neverLists := result["never_mailed_lists"].([]map[string]any)
	if len(neverLists) != 1 || neverLists[0]["list_name"] != "Dormant List" {
		t.Fatalf("never mailed lists = %#v", neverLists)
	}
	var listPageSize string
	for _, request := range c.requests {
		if request.path == "/api/lists" {
			listPageSize = request.params["page[size]"]
		}
	}
	if listPageSize != "10" {
		t.Fatalf("list page size = %q, want Klaviyo endpoint maximum 10", listPageSize)
	}
}

func TestCampaignRevenuePerEmailUsesCampaignValuesReport(t *testing.T) {
	c := &reportTestClient{
		gets: []json.RawMessage{
			json.RawMessage(`{"data":[{"id":"shopify","attributes":{"name":"Placed Order","integration":{"name":"Shopify"}}}],"links":{}}`),
			json.RawMessage(`{"data":[{"id":"campaign-1","attributes":{"name":"Launch"}}],"links":{}}`),
			json.RawMessage(`{"data":[],"links":{}}`),
		},
		posts: []json.RawMessage{json.RawMessage(`{"data":{"attributes":{"results":[{"groupings":{"campaign_id":"campaign-1"},"statistics":{"recipients":1000,"conversion_value":2500}}]}}}`)},
	}
	rows, err := campaignRevenuePerEmailRows(c, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("campaignRevenuePerEmailRows() error = %v", err)
	}
	if len(rows) != 1 || rows[0]["campaign_id"] != "campaign-1" || rows[0]["name"] != "Launch" {
		t.Fatalf("campaign revenue rows = %#v", rows)
	}
	if rows[0]["revenue"] != 2500.0 || rows[0]["emails_sent"] != int64(1000) || rows[0]["revenue_per_email"] != 2.5 {
		t.Fatalf("campaign revenue values = %#v", rows[0])
	}
}

func TestDefaultDBPathUsesXDGAndAccountFingerprint(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", root)
	t.Setenv("HOME", t.TempDir())
	path, err := defaultDBPathForFingerprint("klaviyo-pp-cli", "abc123")
	if err != nil {
		t.Fatalf("defaultDBPathForFingerprint() error = %v", err)
	}
	want := filepath.Join(root, "klaviyo-pp-cli", "accounts", "abc123", "data.db")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}

func TestDefaultDBPathRefusesLegacyUnscopedStore(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("XDG_DATA_HOME", root)
	t.Setenv("HOME", home)
	legacy := filepath.Join(root, "klaviyo-pp-cli", "data.db")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("legacy"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := defaultDBPathForFingerprint("klaviyo-pp-cli", "abc123"); err == nil {
		t.Fatal("legacy unscoped database was silently ignored")
	}
}

type reportTestClient struct {
	gets     []json.RawMessage
	posts    []json.RawMessage
	requests []reportTestRequest
}

type reportTestRequest struct {
	path   string
	params map[string]string
}

func (c *reportTestClient) Get(path string, params map[string]string) (json.RawMessage, error) {
	copied := make(map[string]string, len(params))
	for key, value := range params {
		copied[key] = value
	}
	c.requests = append(c.requests, reportTestRequest{path: path, params: copied})
	if len(c.gets) == 0 {
		return json.RawMessage(`{"data":[],"links":{}}`), nil
	}
	response := c.gets[0]
	c.gets = c.gets[1:]
	return response, nil
}

func (c *reportTestClient) Post(_ string, _ any) (json.RawMessage, int, error) {
	if len(c.posts) > 0 {
		response := c.posts[0]
		c.posts = c.posts[1:]
		return response, 200, nil
	}
	return json.RawMessage(`{"data":{"attributes":{"results":[]}}}`), 200, nil
}

func (c *reportTestClient) Patch(_ string, _ any) (json.RawMessage, int, error) {
	return nil, 200, nil
}

func (c *reportTestClient) Delete(_ string) (json.RawMessage, int, error) {
	return nil, 204, nil
}
