package cli

import (
	"context"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-analytics/internal/ga4"
	"github.com/spf13/cobra"
)

func newChannelsCmd(flags *rootFlags) *cobra.Command {
	var start, end string
	var limit int
	c := &cobra.Command{Use: "channels", Short: "Sessions/users/conversions/revenue by default channel group", RunE: func(cmd *cobra.Command, args []string) error {
		req := reportRequest("sessions,totalUsers,conversions,totalRevenue", "sessionDefaultChannelGroup", start, end, limit)
		addOrder(&req, "-sessions")
		return novelReport(cmd, flags, req, "channels")
	}}
	// Keep these flags declared directly on the command rather than only through
	// dateLimitFlags so the library SKILL verifier can associate the documented
	// `channels --start/--end` examples with this Cobra command.
	c.Flags().StringVar(&start, "start", "30daysAgo", "Start date (YYYY-MM-DD or NdaysAgo)")
	c.Flags().StringVar(&end, "end", "yesterday", "End date")
	c.Flags().IntVar(&limit, "limit", 25, "Max rows")
	return c
}
func newSourcesCmd(flags *rootFlags) *cobra.Command {
	var start, end string
	var limit int
	c := &cobra.Command{Use: "sources", Short: "Source/medium acquisition breakdown with conversion rate", RunE: func(cmd *cobra.Command, args []string) error {
		req := reportRequest("sessions,totalUsers,conversions,totalRevenue", "sessionSourceMedium", start, end, limit)
		addOrder(&req, "-sessions")
		return novelReport(cmd, flags, req, "sources")
	}}
	dateLimitFlags(c, &start, &end, &limit)
	return c
}
func newTopPagesCmd(flags *rootFlags) *cobra.Command {
	var start, end string
	var limit int
	c := &cobra.Command{Use: "top-pages", Short: "Top landing pages by sessions, engagement, and conversions", RunE: func(cmd *cobra.Command, args []string) error {
		req := reportRequest("sessions,engagementRate,conversions,totalRevenue", "landingPagePlusQueryString", start, end, limit)
		addOrder(&req, "-sessions")
		return novelReport(cmd, flags, req, "top_pages")
	}}
	dateLimitFlags(c, &start, &end, &limit)
	return c
}
func newEventsCmd(flags *rootFlags) *cobra.Command      { return eventsCmd(flags, false) }
func newConversionsCmd(flags *rootFlags) *cobra.Command { return eventsCmd(flags, true) }
func eventsCmd(flags *rootFlags, conversions bool) *cobra.Command {
	var start, end string
	var limit int
	name, metric := "events", "eventCount"
	if conversions {
		name, metric = "conversions", "conversions"
	}
	c := &cobra.Command{Use: name, Short: "Key events / conversions over time with trend", RunE: func(cmd *cobra.Command, args []string) error {
		req := reportRequest(metric, "date,eventName", start, end, limit)
		if conversions {
			req.DimensionFilter = &ga4.FilterExpression{Filter: &ga4.Filter{FieldName: "isConversionEvent", StringFilter: &ga4.StringFilter{MatchType: "EXACT", Value: "true"}}}
		}
		return trendReport(cmd, flags, req, name, metric)
	}}
	dateLimitFlags(c, &start, &end, &limit)
	return c
}
func newRevenueCmd(flags *rootFlags) *cobra.Command {
	var start, end, by string
	var limit int
	c := &cobra.Command{Use: "revenue", Short: "Ecommerce revenue, AOV, and transactions by channel/source", RunE: func(cmd *cobra.Command, args []string) error {
		dim := "sessionDefaultChannelGroup"
		if by == "source" || by == "source-medium" {
			dim = "sessionSourceMedium"
		}
		req := reportRequest("purchaseRevenue,transactions,averagePurchaseRevenue,sessions", dim, start, end, limit)
		addOrder(&req, "-purchaseRevenue")
		return novelReport(cmd, flags, req, "revenue")
	}}
	dateLimitFlags(c, &start, &end, &limit)
	c.Flags().StringVar(&by, "by", "channel", "Breakdown: channel or source")
	return c
}
func newAudienceCmd(flags *rootFlags) *cobra.Command {
	var start, end string
	var limit int
	c := &cobra.Command{Use: "audience", Short: "Audience snapshot by country/device/new-vs-returning", RunE: func(cmd *cobra.Command, args []string) error {
		req := reportRequest("totalUsers,newUsers,sessions,engagementRate,conversions", "country,deviceCategory,newVsReturning", start, end, limit)
		addOrder(&req, "-totalUsers")
		return novelReport(cmd, flags, req, "audience")
	}}
	dateLimitFlags(c, &start, &end, &limit)
	return c
}
func newCohortCmd(flags *rootFlags) *cobra.Command {
	var start, end string
	var limit int
	c := &cobra.Command{Use: "cohort", Short: "Cheap retention proxy: users by first-user date and returning status", RunE: func(cmd *cobra.Command, args []string) error {
		req := reportRequest("totalUsers,sessions,engagementRate", "firstSessionDate,newVsReturning", start, end, limit)
		addOrder(&req, "firstSessionDate")
		return novelReport(cmd, flags, req, "cohort")
	}}
	dateLimitFlags(c, &start, &end, &limit)
	return c
}
func novelReport(cmd *cobra.Command, f *rootFlags, req ga4.RunReportRequest, name string) error {
	rows, raw, err := runReportRows(f, req)
	if err != nil {
		return err
	}
	out := map[string]any{"report": name, "property": configuredProperty(f), "rows": enrich(rows), "totals": flattenTotals(raw), "row_count": len(rows)}
	return output(cmd, f, out, renderRows(out))
}
func trendReport(cmd *cobra.Command, f *rootFlags, req ga4.RunReportRequest, name, metric string) error {
	rows, _, err := runReportRows(f, req)
	if err != nil {
		return err
	}
	out := map[string]any{"report": name, "property": configuredProperty(f), "metric": metric, "rows": enrich(rows), "trend": trend(rows, metric)}
	return output(cmd, f, out, renderRows(out))
}
func runReportRows(f *rootFlags, req ga4.RunReportRequest) ([]map[string]any, ga4.ReportResponse, error) {
	p, err := requireProperty(f)
	if err != nil {
		return nil, ga4.ReportResponse{}, err
	}
	cl, _, err := f.newClient()
	if err != nil {
		return nil, ga4.ReportResponse{}, err
	}
	raw, _, err := cl.RunReport(context.Background(), p, req)
	if err != nil {
		return nil, raw, err
	}
	return flattenRows(raw), raw, nil
}
