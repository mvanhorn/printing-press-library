package cli

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-analytics/internal/ga4"
	"github.com/spf13/cobra"
)

func reportFlags(c *cobra.Command, metrics, dims, start, end *string, limit *int) {
	c.Flags().StringVar(metrics, "metrics", "sessions,totalUsers,conversions,totalRevenue", "Comma-separated metrics")
	c.Flags().StringVar(dims, "dimensions", "date", "Comma-separated dimensions")
	dateLimitFlags(c, start, end, limit)
}
func dateLimitFlags(c *cobra.Command, start, end *string, limit *int) {
	c.Flags().StringVar(start, "start", "30daysAgo", "Start date (YYYY-MM-DD or NdaysAgo)")
	c.Flags().StringVar(end, "end", "yesterday", "End date")
	c.Flags().IntVar(limit, "limit", 25, "Max rows")
}
func splitCSV(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
func splitDefault(s, d string) []string {
	if strings.TrimSpace(s) == "" {
		s = d
	}
	return splitCSV(s)
}
func metricNames(xs []string) []ga4.Metric {
	out := []ga4.Metric{}
	for _, x := range xs {
		out = append(out, ga4.Metric{Name: x})
	}
	return out
}
func dimensionNames(xs []string) []ga4.Dimension {
	out := []ga4.Dimension{}
	for _, x := range xs {
		out = append(out, ga4.Dimension{Name: x})
	}
	return out
}
func reportRequest(metrics, dims, start, end string, limit int) ga4.RunReportRequest {
	if start == "" {
		start = "30daysAgo"
	}
	if end == "" {
		end = "yesterday"
	}
	if limit <= 0 {
		limit = 25
	}
	return ga4.RunReportRequest{DateRanges: []ga4.DateRange{{StartDate: start, EndDate: end}}, Metrics: metricNames(splitCSV(metrics)), Dimensions: dimensionNames(splitCSV(dims)), Limit: strconv.Itoa(limit)}
}
func realtimeRequest(metrics, dims string, limit int) ga4.RunRealtimeReportRequest {
	if limit <= 0 {
		limit = 10
	}
	return ga4.RunRealtimeReportRequest{Metrics: metricNames(splitDefault(metrics, "activeUsers")), Dimensions: dimensionNames(splitCSV(dims)), Limit: strconv.Itoa(limit)}
}
func compatibilityRequest(metrics, dims string) ga4.CheckCompatibilityRequest {
	return ga4.CheckCompatibilityRequest{Metrics: metricNames(splitCSV(metrics)), Dimensions: dimensionNames(splitCSV(dims)), CompatibilityFilter: "COMPATIBLE"}
}
func addRawDimensionFilter(req *ga4.RunReportRequest, raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var js json.RawMessage
	if err := json.Unmarshal([]byte(raw), &js); err != nil {
		return err
	}
	req.DimensionFilter = &ga4.FilterExpression{Raw: js}
	return nil
}
func addOrder(req *ga4.RunReportRequest, order string) {
	if order == "" {
		return
	}
	desc := strings.HasPrefix(order, "-")
	name := strings.TrimPrefix(order, "-")
	for _, dim := range req.Dimensions {
		if dim.Name == name {
			req.OrderBys = []ga4.OrderBy{{Desc: desc, Dimension: &ga4.DimensionOrderBy{DimensionName: name}}}
			return
		}
	}
	req.OrderBys = []ga4.OrderBy{{Desc: desc, Metric: &ga4.MetricOrderBy{MetricName: name}}}
}
func funnelRequest(steps, start, end string) ga4.RunFunnelReportRequest {
	fs := []ga4.FunnelStep{}
	for _, s := range splitCSV(steps) {
		fs = append(fs, ga4.FunnelStep{Name: s, FilterExpression: &ga4.FunnelFilterExpression{FunnelEventFilter: &ga4.FunnelEventFilter{EventName: s}}})
	}
	return ga4.RunFunnelReportRequest{DateRanges: []ga4.DateRange{{StartDate: start, EndDate: end}}, Funnel: ga4.Funnel{Steps: fs}}
}
