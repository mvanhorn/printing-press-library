package cli

import (
	"sort"

	"github.com/spf13/cobra"
)

func newCompareCmd(flags *rootFlags) *cobra.Command {
	var start, end, prevStart, prevEnd, period, metrics, dims string
	var limit int
	c := &cobra.Command{Use: "compare", Short: "Period-over-period deltas and percent change without doing two calls manually", RunE: func(cmd *cobra.Command, args []string) error {
		if start == "" || end == "" {
			start, end = "7daysAgo", "yesterday"
		}
		if prevStart == "" || prevEnd == "" {
			prevStart, prevEnd = inferPrevious(start, end, period)
		}
		dimList := splitCSV(dims)
		reqA := reportRequest(metrics, dims, start, end, limit)
		reqB := reportRequest(metrics, dims, prevStart, prevEnd, limit)
		rowsA, rawA, err := runReportRows(flags, reqA)
		if err != nil {
			return err
		}
		rowsB, _, err := runReportRows(flags, reqB)
		if err != nil {
			return err
		}
		out := compareRows(rowsA, rowsB, dimList, splitCSV(metrics))
		out["period_a"] = map[string]string{"start": start, "end": end}
		out["period_b"] = map[string]string{"start": prevStart, "end": prevEnd}
		out["property"] = configuredProperty(flags)
		out["raw_row_count"] = len(rawA.Rows)
		return output(cmd, flags, out, renderCompare(out))
	}}
	c.Flags().StringVar(&start, "start", "", "Current period start")
	c.Flags().StringVar(&end, "end", "", "Current period end")
	c.Flags().StringVar(&prevStart, "previous-start", "", "Previous period start")
	c.Flags().StringVar(&prevEnd, "previous-end", "", "Previous period end")
	c.Flags().StringVar(&period, "period", "wow", "If previous dates absent: wow, mom, or trailing")
	c.Flags().StringVar(&metrics, "metrics", "sessions,totalUsers,conversions,totalRevenue", "Metrics")
	c.Flags().StringVar(&metrics, "metric", "sessions,totalUsers,conversions,totalRevenue", "Alias for --metrics")
	c.Flags().StringVar(&dims, "dimensions", "", "Optional dimensions to compare by")
	c.Flags().IntVar(&limit, "limit", 25, "Rows per report")
	return c
}
func newWhatsChangedCmd(flags *rootFlags) *cobra.Command {
	var start, end, prevStart, prevEnd, period, metrics, dims string
	var limit int
	c := &cobra.Command{Use: "whats-changed", Short: "Scan key metrics for notable spikes/drops vs trailing period", RunE: func(cmd *cobra.Command, args []string) error {
		if dims == "" {
			dims = "sessionDefaultChannelGroup,sessionSourceMedium,landingPagePlusQueryString"
		}
		if prevStart == "" || prevEnd == "" {
			prevStart, prevEnd = inferPrevious(start, end, period)
		}
		movers := []map[string]any{}
		for _, dim := range splitCSV(dims) {
			reqA := reportRequest(metrics, dim, start, end, limit)
			reqB := reportRequest(metrics, dim, prevStart, prevEnd, limit)
			a, _, err := runReportRows(flags, reqA)
			if err != nil {
				return err
			}
			b, _, err := runReportRows(flags, reqB)
			if err != nil {
				return err
			}
			cmp := compareRows(a, b, []string{dim}, splitCSV(metrics))
			for _, r := range cmp["rows"].([]map[string]any) {
				r["dimension"] = dim
				movers = append(movers, r)
			}
		}
		sort.Slice(movers, func(i, j int) bool {
			return abs(toFloat(movers[i]["largest_pct_change"])) > abs(toFloat(movers[j]["largest_pct_change"]))
		})
		if len(movers) > limit {
			movers = movers[:limit]
		}
		out := map[string]any{"property": configuredProperty(flags), "period": map[string]string{"start": start, "end": end}, "previous": map[string]string{"start": prevStart, "end": prevEnd}, "movers": movers}
		return output(cmd, flags, out, renderMovers(out))
	}}
	c.Flags().StringVar(&start, "start", "7daysAgo", "Current start")
	c.Flags().StringVar(&end, "end", "yesterday", "Current end")
	c.Flags().StringVar(&prevStart, "previous-start", "", "Previous start")
	c.Flags().StringVar(&prevEnd, "previous-end", "", "Previous end")
	c.Flags().StringVar(&period, "period", "trailing", "wow/mom/trailing")
	c.Flags().StringVar(&metrics, "metrics", "sessions,totalUsers,conversions,totalRevenue", "Metrics")
	c.Flags().StringVar(&dims, "dimensions", "", "Dimensions to scan")
	c.Flags().IntVar(&limit, "limit", 20, "Movers")
	return c
}
