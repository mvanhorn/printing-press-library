// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"github.com/spf13/cobra"
)

var campaignReportDefaultStatistics = []string{
	"recipients", "delivered", "delivery_rate",
	"opens", "opens_unique", "open_rate",
	"clicks", "clicks_unique", "click_rate",
	"conversions", "conversion_uniques", "conversion_value",
	"unsubscribes", "unsubscribe_rate",
	"spam_complaints", "spam_complaint_rate",
	"bounced", "bounce_rate",
	"revenue_per_recipient", "average_order_value",
}

var segmentReportDefaultStatistics = []string{
	"members_added", "members_removed", "net_members_changed", "total_members",
}

var formReportDefaultStatistics = []string{
	"viewed_form", "viewed_form_uniques", "submits", "submit_rate",
}

type klaviyoReportRequest struct {
	Type               string
	Statistics         []string
	Timeframe          any
	ConversionMetricID string
	Interval           string
}

func buildKlaviyoReportBody(c flowClient, req klaviyoReportRequest, dryRun bool) (map[string]any, string, error) {
	statistics := nonEmptyStrings(req.Statistics)
	if len(statistics) == 0 {
		return nil, "", usageErr(fmt.Errorf("--statistics must include at least one statistic"))
	}
	if req.Timeframe == nil || fmt.Sprint(req.Timeframe) == "" {
		return nil, "", usageErr(fmt.Errorf("--timeframe is required"))
	}

	attrs := map[string]any{
		"statistics": statistics,
		"timeframe":  req.Timeframe,
	}
	metricID := strings.TrimSpace(req.ConversionMetricID)
	if reportRequiresConversionMetric(req.Type) {
		if metricID == "" {
			if dryRun {
				metricID = "<auto:shopify-placed-order>"
			} else {
				var err error
				metricID, err = resolveShopifyPlacedOrderMetricID(c)
				if err != nil {
					return nil, "", err
				}
			}
		}
		attrs["conversion_metric_id"] = metricID
	}
	if reportRequiresInterval(req.Type) {
		interval := strings.TrimSpace(req.Interval)
		if interval == "" {
			return nil, "", usageErr(fmt.Errorf("--interval is required for series reports"))
		}
		attrs["interval"] = interval
	}

	return map[string]any{
		"data": map[string]any{
			"type":       req.Type,
			"attributes": attrs,
		},
	}, metricID, nil
}

func reportRequiresConversionMetric(reportType string) bool {
	return reportType == "campaign-values-report" || reportType == "flow-values-report" || reportType == "flow-series-report"
}

func reportRequiresInterval(reportType string) bool {
	return strings.HasSuffix(reportType, "-series-report")
}

func nonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

type reportHeadersClient interface {
	PostWithHeaders(path string, body any, headers map[string]string) (json.RawMessage, int, error)
}

var waitForKlaviyoReportRetry = waitForIdentityProbeRetry

func postKlaviyoReport(c flowClient, path string, body any) (json.RawMessage, int, error) {
	// PATCH(greptile-review-g5): reporting pagination repeats the POST body on every cursor.
	ctx := context.Background()
	if sc, ok := c.(interface{ PlatformSession() *platform.Session }); ok {
		if session := sc.PlatformSession(); session != nil {
			ctx = session.Context()
		}
	}
	endpoint := path
	seen := map[string]bool{}
	if initial, err := url.Parse(path); err == nil {
		seen[initial.Query().Get("page_cursor")] = true
	}
	var combined map[string]any
	var results []json.RawMessage
	for {
		var raw json.RawMessage
		var status int
		var err error
		// PATCH(review-re-report-rate-limit-retries): read-only report POSTs can safely retry a 429.
		for retries := 0; ; retries++ {
			if hc, ok := c.(reportHeadersClient); ok {
				raw, status, err = hc.PostWithHeaders(endpoint, body, map[string]string{
					"Accept":       "application/vnd.api+json",
					"Content-Type": "application/vnd.api+json",
				})
			} else {
				raw, status, err = c.Post(endpoint, body)
			}
			if err == nil {
				break
			}
			var limited *platform.RateLimitedError
			if retries == 3 || !errors.As(err, &limited) {
				return nil, status, err
			}
			if err := waitForKlaviyoReportRetry(ctx, min(limited.RetryAfter, time.Minute)); err != nil {
				return nil, status, err
			}
		}
		var page struct {
			Data struct {
				Attributes struct {
					Results []json.RawMessage `json:"results"`
				} `json:"attributes"`
			} `json:"data"`
			Links struct {
				Next string `json:"next"`
			} `json:"links"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, status, fmt.Errorf("parsing Klaviyo report response: %w", err)
		}
		if combined == nil && page.Links.Next == "" {
			return raw, status, nil
		}
		if page.Data.Attributes.Results == nil {
			return nil, status, fmt.Errorf("Klaviyo report response did not contain data.attributes.results")
		}
		if combined == nil {
			if err := json.Unmarshal(raw, &combined); err != nil {
				return nil, status, err
			}
		}
		results = append(results, page.Data.Attributes.Results...)
		if page.Links.Next == "" {
			if results == nil {
				results = []json.RawMessage{}
			}
			combined["data"].(map[string]any)["attributes"].(map[string]any)["results"] = results
			combined["links"].(map[string]any)["next"] = nil
			merged, err := json.Marshal(combined)
			return merged, status, err
		}
		next, err := url.Parse(page.Links.Next)
		if err != nil || strings.TrimSuffix(next.Path, "/") != strings.TrimSuffix(strings.SplitN(path, "?", 2)[0], "/") || next.Query().Get("page_cursor") == "" {
			return nil, status, fmt.Errorf("Klaviyo report pagination returned an invalid next link")
		}
		cursor := next.Query().Get("page_cursor")
		if seen[cursor] {
			return nil, status, fmt.Errorf("Klaviyo report pagination repeated page_cursor %q", cursor)
		}
		seen[cursor] = true
		endpoint = next.RequestURI()
	}
}

func klaviyoReportRows(raw json.RawMessage) ([]map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parsing Klaviyo report response: %w", err)
	}
	rows := mapSlice(anyPath(payload, "data.attributes.results"))
	if rows == nil {
		return nil, fmt.Errorf("Klaviyo report response did not contain data.attributes.results")
	}
	return rows, nil
}

func resolveShopifyPlacedOrderMetricID(c flowClient) (string, error) {
	var nameMatches []string
	cursor := ""
	seen := map[string]bool{}
	for {
		params := map[string]string{"fields[metric]": "name,integration"}
		if cursor != "" {
			params["page[cursor]"] = cursor
		}
		resp, err := c.Get("/api/metrics", params)
		if err != nil {
			return "", classifyAPIError(err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(resp, &parsed); err != nil {
			return "", fmt.Errorf("parsing metrics response: %w", err)
		}
		for _, item := range mapSlice(parsed["data"]) {
			if !strings.EqualFold(stringFromMapPath(item, "attributes.name"), "Placed Order") {
				continue
			}
			id := fmt.Sprint(item["id"])
			if id == "" || id == "<nil>" {
				continue
			}
			nameMatches = append(nameMatches, id)
			if strings.EqualFold(stringFromMapPath(item, "attributes.integration.name"), "Shopify") {
				return id, nil
			}
		}
		next := ""
		if links, ok := parsed["links"].(map[string]any); ok {
			next = extractCursor(fmt.Sprint(links["next"]))
		}
		if next == "" || seen[next] {
			break
		}
		seen[next] = true
		cursor = next
	}
	if len(nameMatches) == 1 {
		return nameMatches[0], nil
	}
	if len(nameMatches) > 1 {
		return "", fmt.Errorf("multiple Placed Order metrics found but none identified as Shopify; pass --conversion-metric-id explicitly")
	}
	return "", fmt.Errorf("Shopify Placed Order metric not found; pass --conversion-metric-id explicitly")
}

func newReportTargetingCmd(flags *rootFlags) *cobra.Command {
	var timeframe, conversionMetricID string
	cmd := &cobra.Command{
		Use:         "targeting",
		Short:       "Summarize campaign sends and attributed revenue by included audience segment",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run": true, "timeframe": timeframe, "conversion_metric_id": firstNonEmptyString(conversionMetricID, "auto:shopify-placed-order"),
					"planned_steps": []string{"resolve_shopify_placed_order_metric", "query_campaign_values", "fetch_campaign_audiences", "fetch_segments", "join_targeting_performance"},
				}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body, metricID, err := buildKlaviyoReportBody(c, klaviyoReportRequest{
				Type: "campaign-values-report", Statistics: []string{"recipients", "conversion_value"},
				Timeframe: map[string]any{"key": timeframe}, ConversionMetricID: conversionMetricID,
			}, false)
			if err != nil {
				return err
			}
			raw, _, err := postKlaviyoReport(c, "/api/campaign-values-reports/", body)
			if err != nil {
				return classifyAPIError(err)
			}
			rows, err := klaviyoReportRows(raw)
			if err != nil {
				return err
			}
			result, err := campaignTargetingReport(c, rows)
			if err != nil {
				return err
			}
			result["timeframe"] = timeframe
			result["conversion_metric_id"] = metricID
			audiences := anySlice(result["audiences"])
			attributedRevenue, totalSends := 0.0, 0.0
			var newest time.Time
			for _, item := range audiences {
				row, _ := item.(map[string]any)
				attributedRevenue += anyFloat(row["attributed_revenue"])
				totalSends += anyFloat(row["total_sends"])
				if parsed := parseDate(fmt.Sprint(row["last_mailed_at"])); parsed.After(newest) {
					newest = parsed
				}
			}
			assignments := int64(anyFloat(result["campaign_audience_assignments_rows"]))
			declareReviewedPlatformAnalytics(cmd, flags,
				map[string]int64{"campaign_values": int64(len(rows)), "campaign_audience_assignments": assignments},
				map[string]int64{"segment_id": assignments},
				map[string]int64{"segment_id": 0},
				int64(len(audiences)),
				map[string]time.Time{"campaign_audience_assignments": newest},
				map[string]*float64{"attributed_revenue": &attributedRevenue, "total_sends": &totalSends},
			)
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&timeframe, "timeframe", "last_12_months", "Klaviyo predefined reporting timeframe")
	cmd.Flags().StringVar(&conversionMetricID, "conversion-metric-id", "", "Conversion metric ID (default: auto-resolve Shopify Placed Order)")
	return cmd
}

type targetingAggregate struct {
	ID                string
	Name              string
	Kind              string
	Campaigns         map[string]bool
	ExcludedCampaigns map[string]bool
	Sends             float64
	Revenue           float64
	LastMailed        time.Time
}

func campaignTargetingReport(c flowClient, reportRows []map[string]any) (map[string]any, error) {
	performance := map[string]map[string]float64{}
	for _, row := range reportRows {
		id := stringFromMapPath(row, "groupings.campaign_id")
		if id == "" {
			continue
		}
		if performance[id] == nil {
			performance[id] = map[string]float64{}
		}
		performance[id]["sends"] += anyFloat(anyPath(row, "statistics.recipients"))
		performance[id]["revenue"] += anyFloat(anyPath(row, "statistics.conversion_value"))
	}

	campaigns, err := fetchTargetingCampaigns(c)
	if err != nil {
		return nil, err
	}
	segments, err := fetchAllJSONAPI(c, "/api/segments", map[string]string{"fields[segment]": "name", "page[size]": "10"}, 0)
	if err != nil {
		return nil, err
	}
	// Klaviyo caps /api/lists at 10 rows per page (unlike campaigns and
	// several other collection endpoints). fetchAllJSONAPI follows pagination,
	// so the smaller endpoint-specific page size still returns every list.
	lists, err := fetchAllJSONAPI(c, "/api/lists", map[string]string{"fields[list]": "name", "page[size]": "10"}, 0)
	if err != nil {
		return nil, err
	}

	aggregates := map[string]*targetingAggregate{}
	audienceAssignments := 0
	for _, segment := range segments {
		id := fmt.Sprint(segment["id"])
		aggregates[id] = &targetingAggregate{ID: id, Name: stringFromMapPath(segment, "attributes.name"), Kind: "segment", Campaigns: map[string]bool{}, ExcludedCampaigns: map[string]bool{}}
	}
	for _, list := range lists {
		id := fmt.Sprint(list["id"])
		aggregates[id] = &targetingAggregate{ID: id, Name: stringFromMapPath(list, "attributes.name"), Kind: "list", Campaigns: map[string]bool{}, ExcludedCampaigns: map[string]bool{}}
	}
	for _, campaign := range campaigns {
		campaignID := fmt.Sprint(campaign["id"])
		stats, sent := performance[campaignID]
		if !sent {
			continue
		}
		sentAt := parseDate(firstNonEmptyString(stringFromMapPath(campaign, "attributes.send_time"), stringFromMapPath(campaign, "attributes.scheduled_at")))
		for _, segmentID := range campaignAudienceIDsByKind(campaign, "included") {
			audienceAssignments++
			agg := ensureTargetingAggregate(aggregates, segmentID)
			if !agg.Campaigns[campaignID] {
				agg.Campaigns[campaignID] = true
				agg.Sends += stats["sends"]
				agg.Revenue += stats["revenue"]
				if sentAt.After(agg.LastMailed) {
					agg.LastMailed = sentAt
				}
			}
		}
		for _, segmentID := range campaignAudienceIDsByKind(campaign, "excluded") {
			audienceAssignments++
			agg := ensureTargetingAggregate(aggregates, segmentID)
			agg.ExcludedCampaigns[campaignID] = true
		}
	}

	audiences := make([]map[string]any, 0, len(aggregates))
	rows := []map[string]any{}
	listRows := []map[string]any{}
	neverMailed := []map[string]any{}
	neverMailedLists := []map[string]any{}
	for _, agg := range aggregates {
		if len(agg.Campaigns) == 0 {
			if agg.Kind == "segment" {
				neverMailed = append(neverMailed, map[string]any{"segment_id": agg.ID, "segment_name": agg.Name})
			} else if agg.Kind == "list" {
				neverMailedLists = append(neverMailedLists, map[string]any{"list_id": agg.ID, "list_name": agg.Name})
			}
			continue
		}
		lastMailed := ""
		if !agg.LastMailed.IsZero() {
			lastMailed = agg.LastMailed.Format(time.RFC3339)
		}
		row := map[string]any{
			"audience_id": agg.ID, "audience_name": agg.Name, "audience_type": agg.Kind,
			"campaigns_sent": len(agg.Campaigns), "campaigns_excluded": len(agg.ExcludedCampaigns),
			"total_sends": int64(agg.Sends), "attributed_revenue": round2(agg.Revenue), "last_mailed_at": lastMailed,
		}
		audiences = append(audiences, row)
		if agg.Kind == "segment" {
			row["segment_id"], row["segment_name"] = agg.ID, agg.Name
			rows = append(rows, row)
		} else if agg.Kind == "list" {
			row["list_id"], row["list_name"] = agg.ID, agg.Name
			listRows = append(listRows, row)
		}
	}
	sortTargetingRows := func(items []map[string]any) {
		sort.Slice(items, func(i, j int) bool {
			if anyFloat(items[i]["attributed_revenue"]) == anyFloat(items[j]["attributed_revenue"]) {
				return fmt.Sprint(items[i]["audience_name"]) < fmt.Sprint(items[j]["audience_name"])
			}
			return anyFloat(items[i]["attributed_revenue"]) > anyFloat(items[j]["attributed_revenue"])
		})
	}
	sortTargetingRows(audiences)
	sortTargetingRows(rows)
	sortTargetingRows(listRows)
	sort.Slice(neverMailed, func(i, j int) bool {
		return fmt.Sprint(neverMailed[i]["segment_name"]) < fmt.Sprint(neverMailed[j]["segment_name"])
	})
	sort.Slice(neverMailedLists, func(i, j int) bool {
		return fmt.Sprint(neverMailedLists[i]["list_name"]) < fmt.Sprint(neverMailedLists[j]["list_name"])
	})
	return map[string]any{
		"audiences": audiences, "segments": rows, "lists": listRows,
		"never_mailed_segments": neverMailed, "never_mailed_lists": neverMailedLists,
		"campaign_report_rows": len(reportRows), "segments_mailed": len(rows), "segments_never_mailed": len(neverMailed),
		"campaign_audience_assignments_rows": audienceAssignments,
		"lists_mailed":                       len(listRows), "lists_never_mailed": len(neverMailedLists),
	}, nil
}

func campaignRevenuePerEmailRows(c flowClient, since, until time.Time) ([]map[string]any, error) {
	metricID, err := resolveShopifyPlacedOrderMetricID(c)
	if err != nil {
		return nil, err
	}
	body, _, err := buildKlaviyoReportBody(c, klaviyoReportRequest{
		Type: "campaign-values-report", Statistics: []string{"recipients", "conversion_value"},
		Timeframe:          map[string]any{"start": since.Format(time.RFC3339), "end": until.Format(time.RFC3339)},
		ConversionMetricID: metricID,
	}, false)
	if err != nil {
		return nil, err
	}
	raw, _, err := postKlaviyoReport(c, "/api/campaign-values-reports/", body)
	if err != nil {
		return nil, classifyAPIError(err)
	}
	reportRows, err := klaviyoReportRows(raw)
	if err != nil {
		return nil, err
	}
	campaigns, err := fetchTargetingCampaigns(c)
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	for _, campaign := range campaigns {
		names[fmt.Sprint(campaign["id"])] = stringFromMapPath(campaign, "attributes.name")
	}
	aggregates := map[string]map[string]float64{}
	for _, reportRow := range reportRows {
		id := stringFromMapPath(reportRow, "groupings.campaign_id")
		if id == "" {
			continue
		}
		if aggregates[id] == nil {
			aggregates[id] = map[string]float64{}
		}
		aggregates[id]["revenue"] += anyFloat(anyPath(reportRow, "statistics.conversion_value"))
		aggregates[id]["sends"] += anyFloat(anyPath(reportRow, "statistics.recipients"))
	}
	rows := make([]map[string]any, 0, len(aggregates))
	for id, values := range aggregates {
		name := firstNonEmptyString(names[id], id)
		rpe := rate(values["revenue"], values["sends"])
		rows = append(rows, map[string]any{
			"name": name, "campaign_id": id, "revenue": round2(values["revenue"]),
			"emails_sent": int64(values["sends"]), "revenue_per_email": round3(rpe),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		return anyFloat(rows[i]["revenue_per_email"]) > anyFloat(rows[j]["revenue_per_email"])
	})
	for i := range rows {
		rows[i]["rank"] = i + 1
	}
	return rows, nil
}

func fetchTargetingCampaigns(c flowClient) ([]map[string]any, error) {
	params := map[string]string{"fields[campaign]": "name,status,send_time,scheduled_at,audiences", "page[size]": "100"}
	filters := []string{`equals(messages.channel,'email')`, `and(equals(messages.channel,'email'),equals(archived,true))`}
	byID := map[string]map[string]any{}
	for _, filter := range filters {
		params["filter"] = filter
		items, err := fetchAllJSONAPI(c, "/api/campaigns", params, 0)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			byID[fmt.Sprint(item["id"])] = item
		}
	}
	out := make([]map[string]any, 0, len(byID))
	for _, item := range byID {
		out = append(out, item)
	}
	return out, nil
}

func campaignAudienceIDsByKind(campaign map[string]any, kind string) []string {
	var ids []string
	for _, value := range anySlice(anyPath(campaign, "attributes.audiences."+kind)) {
		switch typed := value.(type) {
		case string:
			ids = append(ids, typed)
		case map[string]any:
			if id := fmt.Sprint(typed["id"]); id != "" && id != "<nil>" {
				ids = append(ids, id)
			}
		}
	}
	return uniqueStrings(ids)
}

func ensureTargetingAggregate(aggregates map[string]*targetingAggregate, id string) *targetingAggregate {
	if aggregates[id] == nil {
		aggregates[id] = &targetingAggregate{ID: id, Name: "(unknown audience)", Kind: "unknown", Campaigns: map[string]bool{}, ExcludedCampaigns: map[string]bool{}}
	}
	return aggregates[id]
}
